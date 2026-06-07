package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// writeStopRequest asks the running (root) stealth-dns daemon to shut down
// without an admin prompt. The daemon publishes a per-run secret token next to
// its executable; this reads that token and writes it into the stop-request
// file atomically (temp file + rename). The daemon ignores any stop-request
// file that does not carry the current token, so a local process that cannot
// read the token cannot trigger a shutdown. nhp#1150 item 1.
//
// It returns the stop-file path and an error. On error the caller should fall
// back to the privileged stop path (the token may be unreadable, e.g. the
// daemon was installed in a root-only directory).
func writeStopRequest(exeDir string) (string, error) {
	stopFilePath := filepath.Join(exeDir, ".stealth-dns-stop")
	tokenPath := filepath.Join(exeDir, ".stealth-dns-stop-token")

	raw, err := os.ReadFile(tokenPath)
	if err != nil {
		return stopFilePath, fmt.Errorf("read stop token: %w", err)
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return stopFilePath, fmt.Errorf("stop token is empty")
	}

	// Write atomically so the daemon's poller never reads a half-written token.
	tmpPath := stopFilePath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(token), 0o600); err != nil {
		return stopFilePath, fmt.Errorf("write stop request: %w", err)
	}
	if err := os.Rename(tmpPath, stopFilePath); err != nil {
		os.Remove(tmpPath)
		return stopFilePath, fmt.Errorf("commit stop request: %w", err)
	}
	return stopFilePath, nil
}
