//go:build linux || darwin

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestStopFileAuthorized exercises the owner-authorization helper without
// requiring root: a freshly created file lives in a temp dir owned by the same
// (test) user, mirroring the legitimate per-user deployment where the UI owns
// both the install directory and the sentinel it drops.
func TestStopFileAuthorized(t *testing.T) {
	dir := t.TempDir()
	stopFilePath := filepath.Join(dir, ".stealth-dns-stop")

	// Missing file must fail closed (no shutdown on stat error).
	if stopFileAuthorized(stopFilePath) {
		t.Fatalf("expected unauthorized for missing stop file %q", stopFilePath)
	}

	// A file owned by the same UID as its directory is authorized.
	if err := os.WriteFile(stopFilePath, nil, 0o600); err != nil {
		t.Fatalf("creating stop file: %v", err)
	}
	if !stopFileAuthorized(stopFilePath) {
		t.Fatalf("expected authorized for same-owner stop file %q", stopFilePath)
	}
}
