package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/OpenNHP/StealthDNS/cert"
	"github.com/OpenNHP/StealthDNS/dns"
	"github.com/OpenNHP/StealthDNS/version"
	"github.com/urfave/cli/v2"
)

// Run the program as an administrator to automatically set up the DNS proxy.
// On Linux/MacOS, the program needs to be started with the root account or
// sudo command to ensure it can properly listen on port 53.
func main() {
	app := cli.NewApp()
	app.Name = "StealthDNS"
	app.Usage = "local DNS proxy service"
	// Version is set at build time via ldflags
	if version.BuildNumber != "" {
		app.Version = version.Version + "+" + version.BuildNumber
	} else {
		app.Version = version.Version
	}

	app.Action = func(c *cli.Context) error {
		return runApp()
	}

	runCmd := &cli.Command{
		Name:    "run",
		Aliases: []string{"r"},
		Usage:   "create and run local DNS proxy service",
		Action: func(c *cli.Context) error {
			return runApp()
		},
	}

	certInstallCmd := &cli.Command{
		Name:    "install-root-ca",
		Usage:   "install root CA",
		Aliases: []string{"i"},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "ensure-file",
				Aliases: []string{"e"},
				Usage:   "if true, create the rootCA file if it does not exist; if false, fail or skip when missing.",
				Value:   true,
			},
		},
		Action: func(c *cli.Context) error {
			ensureFile := c.Bool("ensure-file")
			return cert.Install(ensureFile)
		},
	}

	certUninstallCmd := &cli.Command{
		Name:    "uninstall-root-ca",
		Usage:   "uninstall root CA",
		Aliases: []string{"u"},
		Action: func(c *cli.Context) error {
			return cert.Uninstall()
		},
	}

	certCreateCmd := &cli.Command{
		Name:    "create-cert",
		Aliases: []string{"c"},
		Usage:   "create a certificate from a CSR file or a domain name.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "csr-file",
				Aliases: []string{"f"},
				Usage:   "create a certificate from a CSR file; specify the path to the CSR file.",
			},
			&cli.StringFlag{
				Name:    "domain-name",
				Aliases: []string{"d"},
				Usage:   "create a certificate using a domain name; please specify the domain name.",
			},
		},
		Action: func(c *cli.Context) error {
			csrFile := c.String("csr-file")
			domainName := c.String("domain-name")
			if csrFile == "" && domainName == "" {
				return fmt.Errorf("--csr-file and --domain-name cannot both be empty")
			}
			if csrFile != "" && domainName != "" {
				return fmt.Errorf("--csr-file and --domain-name are mutually exclusive; please specify only one")
			}
			return cert.CreateCert(csrFile, domainName)
		},
	}

	app.Commands = []*cli.Command{
		runCmd,
		certInstallCmd,
		certUninstallCmd,
		certCreateCmd,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func runApp() error {
	log.Println("Stealth DNS starting")
	if !isAdminPermission() {
		log.Println("Insufficient privileges detected. This application must be executed with administrator/root permissions. Please relaunch with elevated rights.")
		if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
			time.Sleep(3 * time.Second)
			os.Exit(0)
		}
	}

	exeFilePath, err := os.Executable()
	if err != nil {
		return err
	}
	exeDirPath := filepath.Dir(exeFilePath)

	// Clean up any residual stop signal file
	stopFilePath := filepath.Join(exeDirPath, ".stealth-dns-stop")
	log.Printf("StealthDNS exe path: %s\n", exeFilePath)
	log.Printf("StealthDNS stop signal file path: %s\n", stopFilePath)
	os.Remove(stopFilePath)

	err = cert.Install(false)
	if err != nil {
		log.Printf("Installation of the root certificate [rootCA.pem] failed: %v\n", err)
	}

	p := &dns.ProxyService{}
	err = p.Start(exeDirPath, 4)
	if err != nil {
		return err
	}

	// Create stop channel
	stopCh := make(chan struct{}, 1)

	// Listen for system signals
	termCh := make(chan os.Signal, 1)
	signal.Notify(termCh, syscall.SIGTERM, os.Interrupt, syscall.SIGABRT)

	// Listen for stop signal file (used by UI to send stop request without admin privileges)
	// Works on both Windows and macOS/Linux
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if _, err := os.Stat(stopFilePath); err == nil {
					log.Println("Stop signal file detected, gracefully shutting down...")
					os.Remove(stopFilePath)
					stopCh <- struct{}{}
					return
				}
			case <-stopCh:
				return
			}
		}
	}()

	// Wait for termination signal
	select {
	case <-termCh:
		log.Println("Received system termination signal")
	case <-stopCh:
		log.Println("Received stop request")
	}

	p.Stop()
	return nil
}

func isAdminPermission() bool {
	switch runtime.GOOS {
	case "windows":
		_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
		if err == nil {
			return true
		} else {
			return false
		}
	case "darwin":
		return os.Geteuid() == 0
	case "linux":
		return os.Geteuid() == 0
	default:
		log.Println(runtime.GOOS, " operating system is not supported; unable to create a DNS handler.")
		return false
	}
}
