package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"stealthdns-ui/version"

	"github.com/pelletier/go-toml/v2"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App application structure
type App struct {
	ctx           context.Context
	process       *exec.Cmd
	processLock   sync.Mutex
	running       bool
	autoRestart   bool
	manualStopped bool // Flag to mark manual stop, prevent auto-restart
	restartCount  int
	maxRestarts   int
	lastRestartAt time.Time
	exePath       string
	configPath    string
	serverPath    string
	trayManager   *TrayManager
	// DNS restore related
	originalDNS   []string // Save original DNS settings
	interfaceName string   // Network interface name
	dhcpEnabled   bool     // Whether DHCP is enabled
	dnsBackedUp   bool     // Whether DNS is backed up
}

// SetTrayManager sets the tray manager
func (a *App) SetTrayManager(tm *TrayManager) {
	a.trayManager = tm
}

// NewApp creates a new application instance
func NewApp() *App {
	return &App{
		autoRestart: true,
		maxRestarts: 5,
	}
}

// startup is called when the application starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Get working directory and resource directory
	workDir, resourceDir := a.findWorkDir(ctx)
	wailsRuntime.LogInfo(ctx, "Working directory: "+workDir)
	wailsRuntime.LogInfo(ctx, "Resource directory: "+resourceDir)

	// Set paths
	if goruntime.GOOS == "windows" {
		a.exePath = filepath.Join(workDir, "stealth-dns.exe")
	} else {
		a.exePath = filepath.Join(workDir, "stealth-dns")
	}
	a.configPath = filepath.Join(resourceDir, "etc", "config.toml")
	a.serverPath = filepath.Join(resourceDir, "etc", "server.toml")

	wailsRuntime.LogInfo(ctx, "DNS executable path: "+a.exePath)
	wailsRuntime.LogInfo(ctx, "StealthDNS UI started")
}

// findWorkDir finds the correct working directory and resource directory
// Returns (workDir, resourceDir) - workDir is where CLI executable is, resourceDir is where etc/ is
func (a *App) findWorkDir(ctx context.Context) (string, string) {
	// Get executable path
	exePath, err := os.Executable()
	if err != nil {
		wailsRuntime.LogError(ctx, "Failed to get executable path: "+err.Error())
		// Fall back to current working directory
		if cwd, err := os.Getwd(); err == nil {
			return cwd, cwd
		}
		return ".", "."
	}

	// Resolve symbolic links
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		wailsRuntime.LogWarning(ctx, "Failed to resolve symbolic link: "+err.Error())
	}

	exeDir := filepath.Dir(exePath)
	wailsRuntime.LogDebug(ctx, "Executable directory: "+exeDir)

	// Special handling for macOS .app bundle
	// CLI and resources (etc/, sdk/) are all in Contents/MacOS/
	if goruntime.GOOS == "darwin" {
		if filepath.Base(exeDir) == "MacOS" {
			contentsDir := filepath.Dir(exeDir)
			if filepath.Base(contentsDir) == "Contents" {
				appDir := filepath.Dir(contentsDir)
				if filepath.Ext(appDir) == ".app" {
					wailsRuntime.LogInfo(ctx, "Detected macOS .app bundle")
					wailsRuntime.LogInfo(ctx, "  Working directory: "+exeDir)
					// Both CLI and resources are in MacOS folder
					return exeDir, exeDir
				}
			}
		}
	}

	// Check if stealth-dns executable exists in current directory
	stealthDNSName := "stealth-dns"
	if goruntime.GOOS == "windows" {
		stealthDNSName = "stealth-dns.exe"
	}

	// First check executable directory
	if _, err := os.Stat(filepath.Join(exeDir, stealthDNSName)); err == nil {
		return exeDir, exeDir
	}

	// Check current working directory
	if cwd, err := os.Getwd(); err == nil {
		if _, err := os.Stat(filepath.Join(cwd, stealthDNSName)); err == nil {
			wailsRuntime.LogInfo(ctx, "Found stealth-dns in current working directory: "+cwd)
			return cwd, cwd
		}
	}

	// Check parent directory of executable (for development scenarios)
	parentDir := filepath.Dir(exeDir)
	if _, err := os.Stat(filepath.Join(parentDir, stealthDNSName)); err == nil {
		wailsRuntime.LogInfo(ctx, "Found stealth-dns in parent directory: "+parentDir)
		return parentDir, parentDir
	}

	// Fall back to executable directory
	wailsRuntime.LogWarning(ctx, "stealth-dns not found, using default directory: "+exeDir)
	return exeDir, exeDir
}

// shutdown is called when the application closes
func (a *App) shutdown(ctx context.Context) {
	a.StopDNS()
	wailsRuntime.LogInfo(ctx, "StealthDNS UI closed")
}

// onDomReady is called when the DOM is ready
func (a *App) onDomReady(ctx context.Context) {
	// Initialization after DOM is ready
	wailsRuntime.LogInfo(ctx, "DOM ready")
}

// ServiceStatus service status
type ServiceStatus struct {
	Running      bool   `json:"running"`
	RestartCount int    `json:"restartCount"`
	Message      string `json:"message"`
}

// checkStealthDNSProcess checks if stealth-dns process is actually running
// It excludes launcher processes (osascript, pkexec, etc.) and only checks for the actual stealth-dns process
func (a *App) checkStealthDNSProcess() bool {
	switch goruntime.GOOS {
	case "darwin", "linux":
		// Check if stealth-dns process is running
		// Exclude launcher processes like osascript, pkexec, sudo, etc.
		psCmd := exec.Command("ps", "aux")
		output, err := psCmd.Output()
		if err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(output)))
			for scanner.Scan() {
				line := scanner.Text()
				// Check if line contains "stealth-dns" and "run"
				if strings.Contains(line, "stealth-dns") && strings.Contains(line, "run") {
					// Exclude launcher processes
					// Launcher processes will have "osascript", "pkexec", "sudo", etc. as the process name
					// The actual stealth-dns process will have "stealth-dns" as the process name or in the command path
					fields := strings.Fields(line)
					if len(fields) >= 11 {
						// In "ps aux" output, fields are: USER, PID, %CPU, %MEM, VSZ, RSS, TT, STAT, START, TIME, COMMAND...
						// The COMMAND field starts at index 10 (11th field)
						processName := fields[10] // First part of COMMAND (executable name)

						// Skip if this is a launcher process
						if strings.Contains(processName, "osascript") ||
							strings.Contains(processName, "pkexec") ||
							strings.Contains(processName, "sudo") ||
							strings.Contains(processName, "zenity") {
							continue // Skip launcher processes
						}

						// Check if this is the actual stealth-dns process
						// The process name should be "stealth-dns" or the command should contain "/stealth-dns"
						command := strings.Join(fields[10:], " ")
						if processName == "stealth-dns" || strings.Contains(command, "/stealth-dns") {
							return true
						}
					}
				}
			}
		}
		return false
	case "windows":
		// Check if stealth-dns.exe process is running
		cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq stealth-dns.exe", "/NH")
		hideWindow(cmd)
		output, err := cmd.Output()
		if err == nil {
			return strings.Contains(string(output), "stealth-dns.exe")
		}
		return false
	default:
		return false
	}
}

// GetStatus gets DNS service status
// It checks both the internal state (a.running) and the actual process state
// to ensure accuracy, especially on macOS where a.process points to osascript
// rather than the actual stealth-dns child process
func (a *App) GetStatus() ServiceStatus {
	a.processLock.Lock()
	defer a.processLock.Unlock()

	// If we're in a starting state (launcher started but actual process not confirmed yet),
	// check if the actual stealth-dns process is running
	// Only update to running if we're in starting state AND process is confirmed running
	isStarting := a.process != nil && !a.running

	if isStarting {
		// We're in starting state, check if actual process is running
		actualRunning := a.checkStealthDNSProcess()
		if actualRunning {
			// Process confirmed running, update state
			wailsRuntime.LogInfo(a.ctx, "Process confirmed running, updating status from starting to running")
			a.running = true
			if a.trayManager != nil {
				a.trayManager.UpdateStatus(true)
			}
			// Emit status event to notify UI that process is now running
			wailsRuntime.EventsEmit(a.ctx, "dns:status", ServiceStatus{
				Running:      true,
				RestartCount: a.restartCount,
				Message:      "status_started",
			})
		}
		// If not running yet, stay in starting state (don't update a.running)
	} else {
		// Not in starting state, sync with actual process state (for normal status checks)
		actualRunning := a.checkStealthDNSProcess()
		if actualRunning != a.running {
			if actualRunning {
				wailsRuntime.LogInfo(a.ctx, fmt.Sprintf("Process detected as running, updating status from %v to %v", a.running, actualRunning))
			} else {
				wailsRuntime.LogInfo(a.ctx, fmt.Sprintf("State mismatch detected: internal=%v, actual=%v, syncing...", a.running, actualRunning))
			}
			a.running = actualRunning

			// If process just started, update tray status and emit event
			if actualRunning {
				if a.trayManager != nil {
					a.trayManager.UpdateStatus(true)
				}
				// Emit status event to notify UI that process is now running
				wailsRuntime.EventsEmit(a.ctx, "dns:status", ServiceStatus{
					Running:      true,
					RestartCount: a.restartCount,
					Message:      "status_started",
				})
			}
		}
	}

	return ServiceStatus{
		Running:      a.running,
		RestartCount: a.restartCount,
		Message:      a.getStatusMessage(),
	}
}

// IsProcessRunning checks if stealth-dns process is actually running
// Deprecated: Use GetStatus() instead, which includes process state checking
func (a *App) IsProcessRunning() bool {
	return a.checkStealthDNSProcess()
}

func (a *App) getStatusMessage() string {
	if a.running {
		return "status_running"
	}
	// Check if we're in a starting state (process launcher started but actual process not detected yet)
	// This is indicated by a.process != nil but a.running == false
	if a.process != nil && !a.running {
		return "status_starting"
	}
	return "status_stopped"
}

// StartDNS starts DNS service
func (a *App) StartDNS() error {
	a.processLock.Lock()
	defer a.processLock.Unlock()

	if a.running {
		return fmt.Errorf("service is already running")
	}

	// Check if executable exists
	if _, err := os.Stat(a.exePath); os.IsNotExist(err) {
		return fmt.Errorf("stealth-dns executable not found: %s", a.exePath)
	}

	// Log current permission status
	if a.isRunningAsAdmin() {
		wailsRuntime.LogInfo(a.ctx, "UI is running with admin privileges")
	} else {
		wailsRuntime.LogInfo(a.ctx, "UI is not running with admin privileges, will request elevation to start DNS service")
	}

	// Backup current DNS settings (Windows only)
	a.backupWindowsDNS()

	// Reset manual stop flag and restart count
	a.manualStopped = false
	a.restartCount = 0

	return a.startProcess()
}

// startProcess internal method to start process
func (a *App) startProcess() error {
	var cmd *exec.Cmd

	switch goruntime.GOOS {
	case "darwin":
		// macOS: Use osascript to request admin privileges
		cmd = a.createMacOSElevatedCommand()
	case "linux":
		// Linux: Prefer pkexec (graphical), otherwise use sudo
		cmd = a.createLinuxElevatedCommand()
	case "windows":
		// Windows: Direct run, requires UI to be started with admin privileges
		// Or use specific elevation method
		cmd = a.createWindowsElevatedCommand()
	default:
		cmd = exec.Command(a.exePath, "run")
	}

	cmd.Dir = filepath.Dir(a.exePath)

	// Set stdout and stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	wailsRuntime.LogInfo(a.ctx, fmt.Sprintf("Start command: %s %v", cmd.Path, cmd.Args))

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start DNS service: %v", err)
	}

	a.process = cmd
	// Don't set a.running = true immediately
	// Let GetStatus() detect the actual process state and update it
	// This ensures the UI shows "starting..." until the process is actually running
	a.running = false

	// Start monitoring goroutine
	go a.monitorProcess()

	wailsRuntime.LogInfo(a.ctx, "DNS service launcher started, waiting for actual process to start...")
	// Send "starting" status instead of "started"
	// The UI will automatically update to "running" when GetStatus() detects the actual process
	wailsRuntime.EventsEmit(a.ctx, "dns:status", ServiceStatus{
		Running:      false,
		RestartCount: a.restartCount,
		Message:      "status_starting",
	})

	// Don't update tray status yet, wait for actual process to start
	// if a.trayManager != nil {
	// 	a.trayManager.UpdateStatus(true)
	// }

	return nil
}

// createMacOSElevatedCommand creates macOS elevated command
func (a *App) createMacOSElevatedCommand() *exec.Cmd {
	// Use osascript to call AppleScript for admin privileges
	script := fmt.Sprintf(`do shell script "%s run" with administrator privileges`, a.exePath)
	return exec.Command("osascript", "-e", script)
}

// createLinuxElevatedCommand creates Linux elevated command
func (a *App) createLinuxElevatedCommand() *exec.Cmd {
	// Prefer pkexec (graphical PolicyKit authentication)
	if _, err := exec.LookPath("pkexec"); err == nil {
		wailsRuntime.LogInfo(a.ctx, "Using pkexec for elevation")
		return exec.Command("pkexec", a.exePath, "run")
	}

	// Check for graphical sudo tools
	// gksudo (GNOME), kdesudo (KDE), lxsudo (LXDE)
	graphicalSudos := []string{"gksudo", "kdesudo", "lxsudo"}
	for _, sudoTool := range graphicalSudos {
		if _, err := exec.LookPath(sudoTool); err == nil {
			wailsRuntime.LogInfo(a.ctx, "Using "+sudoTool+" for elevation")
			return exec.Command(sudoTool, a.exePath, "run")
		}
	}

	// Check for zenity, can be used with sudo
	if _, err := exec.LookPath("zenity"); err == nil {
		wailsRuntime.LogInfo(a.ctx, "Using zenity + sudo for elevation")
		// Use zenity to show password dialog, then pipe to sudo
		script := fmt.Sprintf(`zenity --password --title="StealthDNS requires admin privileges" | sudo -S "%s" run`, a.exePath)
		return exec.Command("bash", "-c", script)
	}

	// Fall back to terminal sudo (requires terminal environment)
	wailsRuntime.LogWarning(a.ctx, "No graphical elevation tool found, using sudo")
	return exec.Command("sudo", a.exePath, "run")
}

// createWindowsElevatedCommand creates Windows elevated command
func (a *App) createWindowsElevatedCommand() *exec.Cmd {
	// Windows uses PowerShell Start-Process -Verb RunAs to trigger UAC
	// This will show UAC elevation dialog
	// Note: Use -Wait to properly monitor process status
	// Use -PassThru to get process object
	// Use -WindowStyle Hidden to hide the launched process window
	// Use -WorkingDirectory to ensure correct working directory for signal file detection
	workDir := filepath.Dir(a.exePath)
	psScript := fmt.Sprintf(
		`$p = Start-Process -FilePath '%s' -ArgumentList 'run' -WorkingDirectory '%s' -Verb RunAs -PassThru -WindowStyle Hidden; $p.WaitForExit(); exit $p.ExitCode`,
		a.exePath, workDir,
	)
	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-Command", psScript)
	// Hide PowerShell window itself
	hideWindow(cmd)
	return cmd
}

// isRunningAsAdmin checks if running with admin privileges (for logging only)
func (a *App) isRunningAsAdmin() bool {
	switch goruntime.GOOS {
	case "windows":
		// Try to open a path that requires admin privileges
		_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
		return err == nil
	case "darwin", "linux":
		return os.Geteuid() == 0
	default:
		return false
	}
}

// monitorProcess monitors process status
func (a *App) monitorProcess() {
	if a.process == nil {
		return
	}

	err := a.process.Wait()

	a.processLock.Lock()
	a.running = false
	a.process = nil

	// If manually stopped, don't auto-restart
	if a.manualStopped {
		wailsRuntime.LogInfo(a.ctx, "Service manually stopped, not auto-restarting")
		// Update tray status
		if a.trayManager != nil {
			a.trayManager.UpdateStatus(false)
		}
		a.processLock.Unlock()
		wailsRuntime.EventsEmit(a.ctx, "dns:status", ServiceStatus{
			Running:      false,
			RestartCount: a.restartCount,
			Message:      "status_stopped",
		})
		return
	}

	// Check if auto-restart is needed (only on abnormal exit)
	if a.autoRestart && err != nil {
		// Check restart frequency limit
		if time.Since(a.lastRestartAt) < 10*time.Second {
			a.restartCount++
		} else {
			a.restartCount = 1
		}

		if a.restartCount <= a.maxRestarts {
			a.lastRestartAt = time.Now()
			wailsRuntime.LogWarning(a.ctx, fmt.Sprintf("DNS service exited abnormally, attempting restart (%d/%d)", a.restartCount, a.maxRestarts))
			a.processLock.Unlock()

			// Wait one second before restart
			time.Sleep(time.Second)

			a.processLock.Lock()
			if !a.running && !a.manualStopped {
				a.startProcess()
			}
			a.processLock.Unlock()
			return
		} else {
			wailsRuntime.LogError(a.ctx, "DNS service restarted too many times, stopping auto-restart")
		}
	}

	// Update tray status
	if a.trayManager != nil {
		a.trayManager.UpdateStatus(false)
	}

	a.processLock.Unlock()

	wailsRuntime.EventsEmit(a.ctx, "dns:status", ServiceStatus{
		Running:      false,
		RestartCount: a.restartCount,
		Message:      "status_stopped",
	})
}

// StopDNS stops DNS service
func (a *App) StopDNS() error {
	a.processLock.Lock()
	defer a.processLock.Unlock()

	wailsRuntime.LogInfo(a.ctx, "Starting to stop DNS service...")

	// Mark as manually stopped to prevent auto-restart
	a.manualStopped = true

	// Record whether UI needs to restore DNS (only needed when graceful shutdown fails)
	needRestoreDNS := false

	// First try to kill the actual stealth-dns process (by process name)
	// Since we use elevation tools to start, a.process points to the elevation tool process
	gracefulShutdown, err := a.killStealthDNSProcess()
	if err != nil {
		wailsRuntime.LogWarning(a.ctx, "Failed to stop service by process name: "+err.Error())
		needRestoreDNS = true // Stop failed, UI needs to restore DNS
	} else {
		wailsRuntime.LogInfo(a.ctx, "stealth-dns process stopped")
		// On Windows, if graceful shutdown via signal file was used, stealth-dns will restore DNS itself
		// If taskkill force kill was used, UI needs to restore DNS
		if goruntime.GOOS == "windows" {
			if gracefulShutdown {
				wailsRuntime.LogInfo(a.ctx, "Graceful shutdown - stealth-dns restored DNS itself")
				needRestoreDNS = false
			} else {
				wailsRuntime.LogInfo(a.ctx, "Force kill used - UI needs to restore DNS")
				needRestoreDNS = true
			}
		}
	}

	// If there's a launcher process (elevation tool), also try to close it
	if a.process != nil {
		if goruntime.GOOS == "windows" {
			a.process.Process.Kill()
		} else {
			a.process.Process.Signal(os.Interrupt)
			// Wait for process to exit
			done := make(chan error)
			go func() {
				done <- a.process.Wait()
			}()

			select {
			case <-done:
			case <-time.After(3 * time.Second):
				a.process.Process.Kill()
			}
		}
	}

	a.running = false
	a.process = nil

	// Wait a short time to ensure process is completely stopped
	if goruntime.GOOS == "windows" {
		wailsRuntime.LogInfo(a.ctx, "Waiting for process to completely stop...")
		time.Sleep(1 * time.Second)

		// Restore original DNS settings (as backup measure)
		if needRestoreDNS && a.dnsBackedUp {
			wailsRuntime.LogInfo(a.ctx, "Preparing to restore DNS settings (UI backup restore)...")
			a.restoreWindowsDNS()
		} else {
			wailsRuntime.LogInfo(a.ctx, "stealth-dns should have restored DNS settings itself")
		}
	}

	wailsRuntime.LogInfo(a.ctx, "DNS service manually stopped")
	wailsRuntime.EventsEmit(a.ctx, "dns:status", ServiceStatus{
		Running:      false,
		RestartCount: a.restartCount,
		Message:      "status_stopped",
	})

	// Update tray status
	if a.trayManager != nil {
		a.trayManager.UpdateStatus(false)
	}

	return nil
}

// killMacOSProcess attempts to kill stealth-dns process on macOS
// First tries to kill by PID without admin privileges, falls back to admin killall if needed
func (a *App) killMacOSProcess() error {
	// First try to find and kill by PID (no admin needed if process is owned by current user)
	// Try to find the process using ps
	psCmd := exec.Command("ps", "aux")
	output, err := psCmd.Output()
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "stealth-dns") && strings.Contains(line, "run") {
				// Extract PID (second field)
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					pid := fields[1]
					// Try to send SIGTERM signal (graceful shutdown)
					killCmd := exec.Command("kill", "-TERM", pid)
					if err := killCmd.Run(); err == nil {
						wailsRuntime.LogInfo(a.ctx, "Sent SIGTERM to stealth-dns process (PID: "+pid+")")
						// Wait a bit to see if process exits
						time.Sleep(1 * time.Second)
						// Check if process still exists
						checkCmd := exec.Command("ps", "-p", pid)
						if err := checkCmd.Run(); err != nil {
							// Process exited, success
							wailsRuntime.LogInfo(a.ctx, "stealth-dns process stopped gracefully")
							return nil
						}
						// Process still running, try SIGKILL
						killCmd = exec.Command("kill", "-KILL", pid)
						if err := killCmd.Run(); err == nil {
							wailsRuntime.LogInfo(a.ctx, "Sent SIGKILL to stealth-dns process (PID: "+pid+")")
							return nil
						}
					}
				}
			}
		}
	}

	// Fallback: Use osascript to execute killall with admin privileges (only if PID method failed)
	wailsRuntime.LogInfo(a.ctx, "PID method failed, using osascript with admin privileges to stop stealth-dns")
	script := `do shell script "killall stealth-dns 2>/dev/null || true" with administrator privileges`
	cmd := exec.Command("osascript", "-e", script)
	output, err = cmd.CombinedOutput()
	if err != nil {
		wailsRuntime.LogDebug(a.ctx, fmt.Sprintf("osascript killall output: %s", string(output)))
		return err
	}

	wailsRuntime.LogInfo(a.ctx, "stealth-dns process stopped via osascript")
	return nil
}

// killStealthDNSProcess kills stealth-dns process by process name
// Returns (gracefulShutdown bool, err error) - gracefulShutdown is true if signal file method was used
func (a *App) killStealthDNSProcess() (bool, error) {
	var cmd *exec.Cmd

	switch goruntime.GOOS {
	case "darwin":
		// macOS: First try signal file method (no admin needed)
		stopFilePath := filepath.Join(filepath.Dir(a.exePath), ".stealth-dns-stop")
		wailsRuntime.LogInfo(a.ctx, "Creating stop signal file: "+stopFilePath)

		stopFile, err := os.Create(stopFilePath)
		if err != nil {
			wailsRuntime.LogWarning(a.ctx, "Failed to create stop signal file: "+err.Error())
			// Fall back to kill method
			return false, a.killMacOSProcess()
		}
		stopFile.Close()
		wailsRuntime.LogInfo(a.ctx, "Stop signal file created, waiting for stealth-dns graceful shutdown...")

		// Wait for process to exit (max 5 seconds)
		for i := 0; i < 10; i++ {
			time.Sleep(500 * time.Millisecond)
			// Check if process is still running
			psCmd := exec.Command("ps", "aux")
			output, err := psCmd.Output()
			if err == nil {
				if !strings.Contains(string(output), "stealth-dns") || !strings.Contains(string(output), "run") {
					wailsRuntime.LogInfo(a.ctx, "stealth-dns process gracefully shut down")
					os.Remove(stopFilePath) // Clean up signal file
					return true, nil        // Graceful shutdown
				}
			}
		}
		wailsRuntime.LogWarning(a.ctx, "Timeout waiting, stealth-dns did not respond to stop signal")
		os.Remove(stopFilePath) // Clean up signal file

		// Fall back to kill method if signal file didn't work
		wailsRuntime.LogInfo(a.ctx, "Signal file method failed, falling back to kill method")
		return false, a.killMacOSProcess()

	case "linux":
		// Linux: Use pkexec or sudo to execute killall/pkill
		if _, err := exec.LookPath("pkexec"); err == nil {
			// Use pkexec for elevated killall
			cmd = exec.Command("pkexec", "killall", "stealth-dns")
			wailsRuntime.LogInfo(a.ctx, "Using pkexec to stop stealth-dns process")
		} else if _, err := exec.LookPath("zenity"); err == nil {
			// Use zenity + sudo
			script := `zenity --password --title="StealthDNS requires admin privileges" | sudo -S killall stealth-dns 2>/dev/null || true`
			cmd = exec.Command("bash", "-c", script)
			wailsRuntime.LogInfo(a.ctx, "Using zenity + sudo to stop stealth-dns process")
		} else {
			// Fall back to sudo
			cmd = exec.Command("sudo", "killall", "stealth-dns")
			wailsRuntime.LogInfo(a.ctx, "Using sudo to stop stealth-dns process")
		}

	case "windows":
		// Windows: Use signal file for graceful shutdown of stealth-dns
		wailsRuntime.LogInfo(a.ctx, "Attempting to stop stealth-dns.exe process...")

		// First check if process is even running
		checkCmd := exec.Command("tasklist", "/FI", "IMAGENAME eq stealth-dns.exe", "/NH")
		hideWindow(checkCmd)
		checkOutput, _ := checkCmd.Output()
		if !strings.Contains(string(checkOutput), "stealth-dns.exe") {
			wailsRuntime.LogInfo(a.ctx, "stealth-dns.exe is not running")
			return true, nil // Not running, treat as graceful (no DNS restore needed)
		}

		// Method 1: Create stop signal file for graceful shutdown (NO ADMIN NEEDED)
		// The signal file must be in the same directory as stealth-dns.exe
		// stealth-dns uses os.Executable() to determine its directory
		stopFilePath := filepath.Join(filepath.Dir(a.exePath), ".stealth-dns-stop")
		debugLogPath := filepath.Join(filepath.Dir(a.exePath), "logs", "stop-debug.log")

		// Create debug log
		debugLog := func(msg string) {
			f, _ := os.OpenFile(debugLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if f != nil {
				f.WriteString(time.Now().Format("15:04:05") + " " + msg + "\n")
				f.Close()
			}
			wailsRuntime.LogInfo(a.ctx, msg)
		}

		debugLog(fmt.Sprintf("UI exePath: %s", a.exePath))
		debugLog(fmt.Sprintf("Creating stop signal file: %s", stopFilePath))

		stopFile, err := os.Create(stopFilePath)
		if err != nil {
			debugLog("ERROR: Failed to create stop signal file: " + err.Error())
		} else {
			stopFile.Close()

			// Verify the file was created
			if _, statErr := os.Stat(stopFilePath); statErr != nil {
				debugLog("ERROR: Signal file verification failed: " + statErr.Error())
			} else {
				debugLog("Signal file created and verified OK")
			}

			// Wait for process to exit (max 8 seconds - longer wait time)
			for i := 0; i < 16; i++ {
				time.Sleep(500 * time.Millisecond)
				// Check if process is still running
				checkCmd := exec.Command("tasklist", "/FI", "IMAGENAME eq stealth-dns.exe", "/NH")
				hideWindow(checkCmd)
				output, _ := checkCmd.Output()
				outputStr := string(output)
				if !strings.Contains(outputStr, "stealth-dns.exe") {
					debugLog("SUCCESS: stealth-dns.exe gracefully shut down via signal file")
					os.Remove(stopFilePath) // Clean up signal file
					return true, nil        // Graceful shutdown - stealth-dns restored DNS itself
				}
				// Log every 2 seconds
				if i%4 == 3 {
					debugLog(fmt.Sprintf("Still waiting... (%d/16), process still running", i+1))
				}
			}
			debugLog("TIMEOUT: stealth-dns did not respond to stop signal after 8 seconds")
			// Check if signal file still exists
			if _, statErr := os.Stat(stopFilePath); statErr == nil {
				debugLog("Signal file still exists - stealth-dns is not monitoring the correct path")
			} else {
				debugLog("Signal file was deleted but process didn't exit")
			}
			os.Remove(stopFilePath) // Clean up signal file
		}

		// Method 2: Try taskkill without admin (may work if we have same user session)
		wailsRuntime.LogInfo(a.ctx, "Attempting to terminate with taskkill (no admin)...")
		cmd = exec.Command("taskkill", "/F", "/IM", "stealth-dns.exe")
		hideWindow(cmd)
		output, err := cmd.CombinedOutput()
		if err == nil {
			wailsRuntime.LogInfo(a.ctx, "taskkill execution successful (no admin needed)")
			return false, nil // Force kill - UI needs to restore DNS
		}
		wailsRuntime.LogDebug(a.ctx, fmt.Sprintf("taskkill without admin failed: %v, output: %s", err, string(output)))

		// Check if process stopped anyway
		checkCmd = exec.Command("tasklist", "/FI", "IMAGENAME eq stealth-dns.exe", "/NH")
		hideWindow(checkCmd)
		checkOutput, _ = checkCmd.Output()
		if !strings.Contains(string(checkOutput), "stealth-dns.exe") {
			wailsRuntime.LogInfo(a.ctx, "stealth-dns.exe stopped")
			return false, nil // Force kill - UI needs to restore DNS
		}

		// Method 3: Only use admin privileges as last resort
		wailsRuntime.LogWarning(a.ctx, "Process still running, requesting admin privileges...")
		tmpFile, err := os.CreateTemp("", "stop_dns_*.bat")
		if err != nil {
			return false, fmt.Errorf("failed to create temp file: %v", err)
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)

		// Write stop command
		tmpFile.WriteString("@echo off\r\n")
		tmpFile.WriteString("taskkill /F /IM stealth-dns.exe\r\n")
		tmpFile.Close()

		// Use PowerShell for elevated execution
		psScript := fmt.Sprintf(`Start-Process -FilePath '%s' -Verb RunAs -Wait -WindowStyle Hidden`, tmpPath)
		cmd = exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psScript)
		hideWindow(cmd)
		output, err = cmd.CombinedOutput()
		if err != nil {
			wailsRuntime.LogWarning(a.ctx, fmt.Sprintf("Elevated taskkill failed: %v, output: %s", err, string(output)))
			return false, err
		}
		wailsRuntime.LogInfo(a.ctx, "stealth-dns.exe process stopped via elevated method")
		return false, nil // Force kill - UI needs to restore DNS

	default:
		return false, fmt.Errorf("unsupported operating system: %s", goruntime.GOOS)
	}

	// Execute command (only reached for Linux)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If process doesn't exist, it's not an error
		wailsRuntime.LogDebug(a.ctx, fmt.Sprintf("Stop process command output: %s", string(output)))
		return false, err
	}

	wailsRuntime.LogInfo(a.ctx, "stealth-dns process stopped")
	return false, nil // Linux uses kill, not graceful shutdown
}

// RestartDNS restarts DNS service
func (a *App) RestartDNS() error {
	if err := a.StopDNS(); err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)
	return a.StartDNS()
}

// backupWindowsDNS backs up Windows DNS settings
func (a *App) backupWindowsDNS() {
	if goruntime.GOOS != "windows" {
		return
	}

	wailsRuntime.LogInfo(a.ctx, "Starting to backup Windows DNS settings...")

	// Method 1: Use PowerShell to get primary network interface name
	cmd := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command",
		`Get-NetRoute -DestinationPrefix '0.0.0.0/0' | Sort-Object RouteMetric | Select-Object -First 1 -ExpandProperty InterfaceAlias`)
	hideWindow(cmd)
	output, err := cmd.Output()

	if err != nil {
		wailsRuntime.LogWarning(a.ctx, "PowerShell failed to get network interface, trying netsh: "+err.Error())
		// Method 2: Use netsh to get interface name
		cmd = exec.Command("netsh", "interface", "show", "interface")
		hideWindow(cmd)
		output, err = cmd.Output()
		if err != nil {
			wailsRuntime.LogWarning(a.ctx, "netsh also failed: "+err.Error())
			return
		}
		// Parse netsh output to find connected interface
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Connected") {
				fields := strings.Fields(line)
				if len(fields) >= 4 {
					a.interfaceName = fields[len(fields)-1]
					break
				}
			}
		}
	} else {
		a.interfaceName = strings.TrimSpace(string(output))
	}

	if a.interfaceName == "" {
		wailsRuntime.LogWarning(a.ctx, "No active network interface found")
		return
	}
	wailsRuntime.LogInfo(a.ctx, "Primary network interface: "+a.interfaceName)

	// Use netsh to get current DNS settings (more reliable)
	cmd = exec.Command("netsh", "interface", "ip", "show", "dns", a.interfaceName)
	hideWindow(cmd)
	output, err = cmd.Output()
	if err != nil {
		wailsRuntime.LogWarning(a.ctx, "Failed to get current DNS settings: "+err.Error())
		// Try PowerShell method
		cmd = exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command",
			fmt.Sprintf(`(Get-DnsClientServerAddress -InterfaceAlias '%s' -AddressFamily IPv4).ServerAddresses -join ','`, a.interfaceName))
		hideWindow(cmd)
		output, err = cmd.Output()
		if err != nil {
			wailsRuntime.LogWarning(a.ctx, "PowerShell get DNS also failed: "+err.Error())
			return
		}
		dnsStr := strings.TrimSpace(string(output))
		if dnsStr != "" {
			a.originalDNS = strings.Split(dnsStr, ",")
		}
	} else {
		// Parse netsh output
		outputStr := string(output)
		wailsRuntime.LogDebug(a.ctx, "netsh DNS output: "+outputStr)

		// Check if DHCP is enabled
		if strings.Contains(outputStr, "DHCP") {
			a.dhcpEnabled = true
		}

		// Extract DNS addresses
		lines := strings.Split(outputStr, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Match IP address format
			if matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+`, line); matched {
				// Extract first IP address
				parts := strings.Fields(line)
				if len(parts) > 0 {
					ip := parts[0]
					if ip != "127.0.0.1" {
						a.originalDNS = append(a.originalDNS, ip)
					}
				}
			}
		}
	}

	// Filter out 127.0.0.1 (if StealthDNS was already set)
	filtered := make([]string, 0)
	for _, dns := range a.originalDNS {
		dns = strings.TrimSpace(dns)
		if dns != "" && dns != "127.0.0.1" {
			filtered = append(filtered, dns)
		}
	}
	a.originalDNS = filtered

	if len(a.originalDNS) > 0 {
		wailsRuntime.LogInfo(a.ctx, "Backed up DNS settings: "+strings.Join(a.originalDNS, ", "))
		a.dnsBackedUp = true
	} else {
		wailsRuntime.LogInfo(a.ctx, "No static DNS settings found, marking as DHCP mode")
		a.dhcpEnabled = true
		a.dnsBackedUp = true
	}

	wailsRuntime.LogInfo(a.ctx, fmt.Sprintf("DNS backup complete: Interface=%s, DHCP=%v, DNS=%v",
		a.interfaceName, a.dhcpEnabled, a.originalDNS))
}

// restoreWindowsDNS restores Windows DNS settings
func (a *App) restoreWindowsDNS() {
	if goruntime.GOOS != "windows" {
		return
	}

	if !a.dnsBackedUp {
		wailsRuntime.LogInfo(a.ctx, "No backed up DNS settings, skipping restore")
		return
	}

	if a.interfaceName == "" {
		wailsRuntime.LogWarning(a.ctx, "Unknown network interface, cannot restore DNS")
		return
	}

	wailsRuntime.LogInfo(a.ctx, fmt.Sprintf("Restoring DNS settings... Interface: %s, DHCP: %v, Original DNS: %v",
		a.interfaceName, a.dhcpEnabled, a.originalDNS))

	// Method 1: First try direct netsh command execution (if has permission)
	var success bool

	if a.dhcpEnabled || len(a.originalDNS) == 0 {
		// Restore to DHCP mode
		cmd := exec.Command("netsh", "interface", "ipv4", "set", "dnsservers",
			fmt.Sprintf("name=%s", a.interfaceName), "source=dhcp")
		hideWindow(cmd)
		output, err := cmd.CombinedOutput()
		if err == nil {
			wailsRuntime.LogInfo(a.ctx, "Restored to DHCP DNS mode (direct execution)")
			success = true
		} else {
			wailsRuntime.LogDebug(a.ctx, fmt.Sprintf("Direct execution failed: %v, output: %s", err, string(output)))
		}
	} else {
		// Restore to static DNS - first clear
		cmd := exec.Command("netsh", "interface", "ip", "delete", "dns",
			fmt.Sprintf("name=%s", a.interfaceName), "all")
		hideWindow(cmd)
		cmd.Run()

		// Set primary DNS
		cmd = exec.Command("netsh", "interface", "ipv4", "set", "dns",
			fmt.Sprintf("name=%s", a.interfaceName), "static", a.originalDNS[0], "primary")
		hideWindow(cmd)
		output, err := cmd.CombinedOutput()
		if err == nil {
			success = true
			// Add alternate DNS
			for i := 1; i < len(a.originalDNS); i++ {
				cmd = exec.Command("netsh", "interface", "ipv4", "add", "dns",
					fmt.Sprintf("name=%s", a.interfaceName), a.originalDNS[i], fmt.Sprintf("index=%d", i+1))
				hideWindow(cmd)
				cmd.Run()
			}
			wailsRuntime.LogInfo(a.ctx, "Restored DNS settings (direct execution): "+strings.Join(a.originalDNS, ", "))
		} else {
			wailsRuntime.LogDebug(a.ctx, fmt.Sprintf("Direct execution failed: %v, output: %s", err, string(output)))
		}
	}

	// Method 2: If direct execution fails, use elevated execution
	if !success {
		wailsRuntime.LogInfo(a.ctx, "Attempting to restore DNS with elevation...")

		// Create temp batch file
		tmpFile, err := os.CreateTemp("", "restore_dns_*.bat")
		if err != nil {
			wailsRuntime.LogWarning(a.ctx, "Failed to create temp script file: "+err.Error())
			a.dnsBackedUp = false
			return
		}
		tmpPath := tmpFile.Name()

		// Write restore commands
		tmpFile.WriteString("@echo off\r\n")
		if a.dhcpEnabled || len(a.originalDNS) == 0 {
			tmpFile.WriteString(fmt.Sprintf("netsh interface ipv4 set dnsservers name=\"%s\" source=dhcp\r\n", a.interfaceName))
		} else {
			tmpFile.WriteString(fmt.Sprintf("netsh interface ip delete dns name=\"%s\" all\r\n", a.interfaceName))
			tmpFile.WriteString(fmt.Sprintf("netsh interface ipv4 set dns name=\"%s\" static %s primary\r\n", a.interfaceName, a.originalDNS[0]))
			for i := 1; i < len(a.originalDNS); i++ {
				tmpFile.WriteString(fmt.Sprintf("netsh interface ipv4 add dns name=\"%s\" %s index=%d\r\n", a.interfaceName, a.originalDNS[i], i+1))
			}
		}
		tmpFile.Close()

		// Use PowerShell to execute batch file with elevation
		psScript := fmt.Sprintf(`Start-Process -FilePath '%s' -Verb RunAs -Wait -WindowStyle Hidden`, tmpPath)
		cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psScript)
		hideWindow(cmd)

		output, err := cmd.CombinedOutput()
		os.Remove(tmpPath) // Clean up temp file

		if err != nil {
			wailsRuntime.LogWarning(a.ctx, fmt.Sprintf("Elevated DNS restore failed: %v, output: %s", err, string(output)))
		} else {
			if a.dhcpEnabled || len(a.originalDNS) == 0 {
				wailsRuntime.LogInfo(a.ctx, "Restored to DHCP DNS mode (elevated execution)")
			} else {
				wailsRuntime.LogInfo(a.ctx, "Restored DNS settings (elevated execution): "+strings.Join(a.originalDNS, ", "))
			}
		}
	}

	a.dnsBackedUp = false
}

// SetAutoRestart sets auto-restart
func (a *App) SetAutoRestart(enabled bool) {
	a.processLock.Lock()
	defer a.processLock.Unlock()
	a.autoRestart = enabled
}

// GetAutoRestart gets auto-restart status
func (a *App) GetAutoRestart() bool {
	a.processLock.Lock()
	defer a.processLock.Unlock()
	return a.autoRestart
}

// ClientConfig client configuration
type ClientConfig struct {
	PrivateKeyBase64    string `json:"privateKeyBase64" toml:"PrivateKeyBase64"`
	DefaultCipherScheme int    `json:"defaultCipherScheme" toml:"DefaultCipherScheme"`
	UserId              string `json:"userId" toml:"UserId"`
	OrganizationId      string `json:"organizationId" toml:"OrganizationId"`
	LogLevel            int    `json:"logLevel" toml:"LogLevel"`
}

// ServerConfig server configuration
type ServerConfig struct {
	Hostname     string `json:"hostname" toml:"Hostname"`
	Ip           string `json:"ip" toml:"Ip"`
	Port         int    `json:"port" toml:"Port"`
	PubKeyBase64 string `json:"pubKeyBase64" toml:"PubKeyBase64"`
	ExpireTime   int64  `json:"expireTime" toml:"ExpireTime"`
}

// ServerConfigFile server configuration file structure
type ServerConfigFile struct {
	Servers []ServerConfig `toml:"Servers"`
}

// GetClientConfig gets client configuration
func (a *App) GetClientConfig() (ClientConfig, error) {
	var config ClientConfig

	data, err := os.ReadFile(a.configPath)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %v", err)
	}

	err = toml.Unmarshal(data, &config)
	if err != nil {
		return config, fmt.Errorf("failed to parse config file: %v", err)
	}

	return config, nil
}

// SaveClientConfig saves client configuration
func (a *App) SaveClientConfig(config ClientConfig) error {
	// Read original file to preserve comments and other fields
	originalData, err := os.ReadFile(a.configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	// If file exists, try to preserve UserData section
	var userData map[string]interface{}
	if len(originalData) > 0 {
		var fullConfig struct {
			ClientConfig
			UserData map[string]interface{} `toml:"UserData"`
		}
		toml.Unmarshal(originalData, &fullConfig)
		userData = fullConfig.UserData
	}

	// Build complete configuration
	fullConfig := struct {
		ClientConfig
		UserData map[string]interface{} `toml:"UserData,omitempty"`
	}{
		ClientConfig: config,
		UserData:     userData,
	}

	data, err := toml.Marshal(fullConfig)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %v", err)
	}

	// Add comment header
	header := `# NHP-Agent base config
# field with (-) does not support dynamic update

# PrivateKeyBase64 (-): agent private key in base64 format.
# DefaultCipherScheme: 0: gmsm, 1: curve25519.
# UserId: specify the user id this agent represents.
# OrganizationId: specify the organization id this agent represents.
# LogLevel: 0: silent, 1: error, 2: info, 3: audit, 4: debug, 5: trace.
`
	finalData := header + string(data)

	err = os.WriteFile(a.configPath, []byte(finalData), 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// GetServerConfig gets server configuration
func (a *App) GetServerConfig() ([]ServerConfig, error) {
	data, err := os.ReadFile(a.serverPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read server config file: %v", err)
	}

	var configFile ServerConfigFile
	err = toml.Unmarshal(data, &configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server config file: %v", err)
	}

	return configFile.Servers, nil
}

// SaveServerConfig saves server configuration
func (a *App) SaveServerConfig(servers []ServerConfig) error {
	configFile := ServerConfigFile{
		Servers: servers,
	}

	data, err := toml.Marshal(configFile)
	if err != nil {
		return fmt.Errorf("failed to serialize server config: %v", err)
	}

	// Add comment header
	header := `# list the server peers for the agent under [[Servers]] table

# Hostname: the domain of the server peer. If specified, it overrides the "Ip" field with its first resolved address.
# Ip: specify the ip address of the server peer
# Port: specify the port number of this server peer is listening
# PubKeyBase64: public key of the server peer in base64 format
# ExpireTime (epoch timestamp in seconds): peer key validation will fail when it expires.
`
	finalData := header + string(data)

	err = os.WriteFile(a.serverPath, []byte(finalData), 0644)
	if err != nil {
		return fmt.Errorf("failed to write server config file: %v", err)
	}

	return nil
}

// MinimizeToTray minimizes to system tray
func (a *App) MinimizeToTray() {
	wailsRuntime.WindowHide(a.ctx)
}

// ShowWindow shows window
func (a *App) ShowWindow() {
	wailsRuntime.WindowShow(a.ctx)
}

// Quit quits the application
func (a *App) Quit() {
	a.StopDNS()
	wailsRuntime.Quit(a.ctx)
}

// SystemDNSInfo system DNS info
type SystemDNSInfo struct {
	DNSServers    []string `json:"dnsServers"`
	ListenPort    int      `json:"listenPort"`
	IsProxyActive bool     `json:"isProxyActive"`
}

// GetSystemDNS gets system DNS configuration
func (a *App) GetSystemDNS() SystemDNSInfo {
	info := SystemDNSInfo{
		DNSServers:    []string{},
		ListenPort:    53,
		IsProxyActive: a.running,
	}

	switch goruntime.GOOS {
	case "darwin":
		info.DNSServers = a.getMacOSDNS()
	case "linux":
		info.DNSServers = a.getLinuxDNS()
	case "windows":
		info.DNSServers = a.getWindowsDNS()
	}

	// If no DNS obtained, return default value
	if len(info.DNSServers) == 0 {
		info.DNSServers = []string{"Unknown"}
	}

	return info
}

// getMacOSDNS gets macOS system DNS
func (a *App) getMacOSDNS() []string {
	var dnsServers []string

	// First, check if DNS is from DHCP by using networksetup
	// This only shows manually configured DNS, so if it returns empty/error, it's DHCP
	isDHCP := false
	interfaceName := a.getMacOSInterfaceName()
	if interfaceName != "" {
		cmd := exec.Command("networksetup", "-getdnsservers", interfaceName)
		output, err := cmd.Output()
		if err != nil || strings.Contains(strings.TrimSpace(string(output)), "aren't any DNS Servers set") {
			isDHCP = true
		}
	}

	// Use scutil --dns to get actual DNS configuration (includes DHCP-assigned DNS)
	cmd := exec.Command("scutil", "--dns")
	output, err := cmd.Output()
	if err != nil {
		wailsRuntime.LogWarning(a.ctx, "Failed to get macOS DNS: "+err.Error())
		return dnsServers
	}

	lines := strings.Split(string(output), "\n")
	seen := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "nameserver[") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				dns := strings.TrimSpace(parts[1])
				if dns != "" && !seen[dns] {
					seen[dns] = true
					// If DHCP mode and not 127.0.0.1 (StealthDNS local proxy), append "DHCP"
					// 127.0.0.1 should always be displayed but never marked as DHCP
					if isDHCP && dns != "127.0.0.1" {
						dnsServers = append(dnsServers, dns+" (DHCP)")
					} else {
						dnsServers = append(dnsServers, dns)
					}
				}
			}
		}
	}

	return dnsServers
}

// getMacOSInterfaceName gets the active network interface name on macOS
func (a *App) getMacOSInterfaceName() string {
	cmd := exec.Command("networksetup", "-listallnetworkservices")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		service := strings.TrimSpace(scanner.Text())
		if service == "" || strings.HasPrefix(service, "*") {
			continue
		}
		// Check if this service is active
		cmd := exec.Command("networksetup", "-getinfo", service)
		infoOutput, err := cmd.Output()
		if err == nil {
			infoStr := string(infoOutput)
			if strings.Contains(infoStr, "IP address:") && !strings.Contains(infoStr, "IP address: 0.0.0.0") {
				return service
			}
		}
	}

	return ""
}

// getLinuxDNS gets Linux system DNS
func (a *App) getLinuxDNS() []string {
	var dnsServers []string

	// First try systemd-resolve
	cmd := exec.Command("systemd-resolve", "--status")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		seen := make(map[string]bool)

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "DNS Servers:") || strings.HasPrefix(line, "DNS Server:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) >= 2 {
					dns := strings.TrimSpace(parts[1])
					if dns != "" && !seen[dns] {
						seen[dns] = true
						dnsServers = append(dnsServers, dns)
					}
				}
			}
		}

		if len(dnsServers) > 0 {
			return dnsServers
		}
	}

	// Fall back to reading /etc/resolv.conf
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		wailsRuntime.LogWarning(a.ctx, "Failed to get Linux DNS: "+err.Error())
		return dnsServers
	}

	lines := strings.Split(string(data), "\n")
	seen := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "nameserver") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				dns := parts[1]
				if !seen[dns] {
					seen[dns] = true
					dnsServers = append(dnsServers, dns)
				}
			}
		}
	}

	return dnsServers
}

// getWindowsDNS gets Windows system DNS
func (a *App) getWindowsDNS() []string {
	var dnsServers []string

	// First check if DNS is from DHCP using netsh (check all interfaces)
	isDHCP := false
	checkCmd := exec.Command("netsh", "interface", "ip", "show", "dns")
	hideWindow(checkCmd)
	checkOutput, err := checkCmd.Output()
	if err == nil && strings.Contains(string(checkOutput), "DHCP") {
		isDHCP = true
	}

	// Use PowerShell to get DNS configuration (hidden window)
	cmd := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command",
		"Get-DnsClientServerAddress -AddressFamily IPv4 | Select-Object -ExpandProperty ServerAddresses | Select-Object -Unique")
	hideWindow(cmd)
	output, err := cmd.Output()
	if err != nil {
		// Fall back to netsh - parse the output we already have
		if checkOutput != nil {
			lines := strings.Split(string(checkOutput), "\n")
			seen := make(map[string]bool)

			for _, line := range lines {
				line = strings.TrimSpace(line)
				// Match IP address format
				if matched, _ := regexp.MatchString(`^\d+\.\d+\.\d+\.\d+`, line); matched {
					// Extract first IP address from line
					parts := strings.Fields(line)
					if len(parts) > 0 {
						ip := parts[0]
						if !seen[ip] {
							seen[ip] = true
							// If DHCP mode and not 127.0.0.1 (StealthDNS local proxy), append "DHCP"
							if isDHCP && ip != "127.0.0.1" {
								dnsServers = append(dnsServers, ip+" (DHCP)")
							} else {
								dnsServers = append(dnsServers, ip)
							}
						}
					}
				}
			}
		}
		return dnsServers
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	seen := make(map[string]bool)

	for _, line := range lines {
		dns := strings.TrimSpace(line)
		if dns != "" && !seen[dns] {
			seen[dns] = true
			// If DHCP mode and not 127.0.0.1 (StealthDNS local proxy), append "DHCP"
			if isDHCP && dns != "127.0.0.1" {
				dnsServers = append(dnsServers, dns+" (DHCP)")
			} else {
				dnsServers = append(dnsServers, dns)
			}
		}
	}

	return dnsServers
}

// LogEntry log entry
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Raw       string `json:"raw"`
}

// LogFile log file info
type LogFile struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

// GetLogFiles gets log file list (returns only today's logs)
func (a *App) GetLogFiles() ([]LogFile, error) {
	logsDir := filepath.Join(filepath.Dir(a.exePath), "logs")

	// Check if directory exists
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		return []LogFile{}, nil
	}

	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read log directory: %v", err)
	}

	// Get today's date (midnight)
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var files []LogFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Only show .log files
		if !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Only show log files modified today
		if info.ModTime().Before(todayStart) {
			continue
		}

		files = append(files, LogFile{
			Name:     entry.Name(),
			Size:     info.Size(),
			Modified: info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	// Sort by modification time descending (newest first)
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].Modified < files[j].Modified {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	return files, nil
}

// GetLogContent gets log file content
func (a *App) GetLogContent(filename string, lines int) ([]LogEntry, error) {
	// Security check: prevent path traversal attack
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return nil, fmt.Errorf("invalid filename")
	}

	logsDir := filepath.Join(filepath.Dir(a.exePath), "logs")
	logPath := filepath.Join(logsDir, filename)

	// Check if file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return []LogEntry{}, nil
	}

	// Read file
	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read log file: %v", err)
	}

	// Split by lines
	content := string(data)
	allLines := strings.Split(content, "\n")

	// Get last N lines
	startIdx := 0
	if lines > 0 && len(allLines) > lines {
		startIdx = len(allLines) - lines
	}

	var entries []LogEntry
	for i := startIdx; i < len(allLines); i++ {
		line := strings.TrimSpace(allLines[i])
		if line == "" {
			continue
		}

		entry := a.parseLogLine(line)
		entries = append(entries, entry)
	}

	return entries, nil
}

// parseLogLine parses log line
func (a *App) parseLogLine(line string) LogEntry {
	entry := LogEntry{
		Raw: line,
	}

	// Try to parse common log format: [time] [level] message
	// Or: time level message
	parts := strings.SplitN(line, " ", 4)
	if len(parts) >= 3 {
		// Check if it's timestamp format
		if len(parts[0]) >= 8 && (strings.Contains(parts[0], ":") || strings.Contains(parts[0], "-")) {
			entry.Timestamp = parts[0]
			if len(parts) >= 2 {
				// Check if second part is part of timestamp
				if strings.Contains(parts[1], ":") && !strings.Contains(parts[0], ":") {
					entry.Timestamp = parts[0] + " " + parts[1]
					if len(parts) >= 3 {
						entry.Level = strings.Trim(parts[2], "[]")
						if len(parts) >= 4 {
							entry.Message = parts[3]
						}
					}
				} else {
					entry.Level = strings.Trim(parts[1], "[]")
					if len(parts) >= 3 {
						entry.Message = strings.Join(parts[2:], " ")
					}
				}
			}
		}
	}

	// If parsing fails, use entire line as message
	if entry.Message == "" {
		entry.Message = line
	}

	// Identify log level
	upperLine := strings.ToUpper(line)
	if strings.Contains(upperLine, "ERROR") || strings.Contains(upperLine, "ERR") {
		entry.Level = "ERROR"
	} else if strings.Contains(upperLine, "WARN") {
		entry.Level = "WARN"
	} else if strings.Contains(upperLine, "DEBUG") {
		entry.Level = "DEBUG"
	} else if strings.Contains(upperLine, "INFO") {
		entry.Level = "INFO"
	} else if strings.Contains(upperLine, "TRACE") {
		entry.Level = "TRACE"
	}

	return entry
}

// WatchLogFile watches log file changes (returns new content)
func (a *App) WatchLogFile(filename string, lastSize int64) ([]LogEntry, int64, error) {
	// Security check
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return nil, 0, fmt.Errorf("invalid filename")
	}

	logsDir := filepath.Join(filepath.Dir(a.exePath), "logs")
	logPath := filepath.Join(logsDir, filename)

	// Get file info
	info, err := os.Stat(logPath)
	if err != nil {
		return []LogEntry{}, 0, nil
	}

	currentSize := info.Size()

	// If file hasn't changed
	if currentSize <= lastSize {
		return []LogEntry{}, currentSize, nil
	}

	// Read new portion
	file, err := os.Open(logPath)
	if err != nil {
		return nil, lastSize, err
	}
	defer file.Close()

	// Seek to last read position
	_, err = file.Seek(lastSize, 0)
	if err != nil {
		return nil, lastSize, err
	}

	// Read new content
	newData := make([]byte, currentSize-lastSize)
	_, err = file.Read(newData)
	if err != nil {
		return nil, lastSize, err
	}

	// Parse new lines
	lines := strings.Split(string(newData), "\n")
	var entries []LogEntry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entries = append(entries, a.parseLogLine(line))
	}

	return entries, currentSize, nil
}

// ClearLogFile clears log file
func (a *App) ClearLogFile(filename string) error {
	// Security check
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return fmt.Errorf("invalid filename")
	}

	logsDir := filepath.Join(filepath.Dir(a.exePath), "logs")
	logPath := filepath.Join(logsDir, filename)

	// Clear file
	return os.WriteFile(logPath, []byte{}, 0644)
}

// GetVersion returns the application version
func (a *App) GetVersion() string {
	if version.BuildNumber != "" {
		return version.Version + "+" + version.BuildNumber
	}
	return version.Version
}

// VersionInfo contains detailed version information
type VersionInfo struct {
	Version     string `json:"version"`
	BuildNumber string `json:"buildNumber"`
	CommitID    string `json:"commitId"`
	BuildTime   string `json:"buildTime"`
}

// GetVersionInfo returns detailed version information
func (a *App) GetVersionInfo() VersionInfo {
	return VersionInfo{
		Version:     version.Version,
		BuildNumber: version.BuildNumber,
		CommitID:    version.CommitID,
		BuildTime:   version.BuildTime,
	}
}
