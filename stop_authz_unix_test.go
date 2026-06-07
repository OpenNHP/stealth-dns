//go:build linux || darwin

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestStopFileAuthorized_RealFS covers the syscall leaf without requiring root:
// a freshly created file lives in a temp dir owned by the same (test) user,
// mirroring the legitimate per-user deployment where the UI owns both the
// install directory and the sentinel it drops. The cross-owner branch needs a
// second UID (root), so it is covered purely via TestOwnersMatch below.
func TestStopFileAuthorized_RealFS(t *testing.T) {
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

// TestOwnersMatch covers the security-critical decision via injected lookups,
// so the same-owner, different-owner, and fail-closed branches run with no root
// and no filesystem.
func TestOwnersMatch(t *testing.T) {
	const owner uint32 = 1000
	uid := func(u uint32) func() (uint32, error) {
		return func() (uint32, error) { return u, nil }
	}
	boom := func() (uint32, error) { return 0, errors.New("lookup failed") }

	tests := []struct {
		name      string
		file, dir func() (uint32, error)
		want      bool
	}{
		{"same owner -> authorized", uid(owner), uid(owner), true},
		{"different owner -> rejected", uid(owner), uid(owner + 1), false},
		{"file lookup error -> fail closed", boom, uid(owner), false},
		{"dir lookup error -> fail closed", uid(owner), boom, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ownersMatch(tc.file, tc.dir); got != tc.want {
				t.Fatalf("ownersMatch = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestOwnersMatch_ShortCircuitsOnFileError asserts a failing file lookup stops
// before the dir lookup runs (the fail-closed branch must not depend on the
// second lookup).
func TestOwnersMatch_ShortCircuitsOnFileError(t *testing.T) {
	dirCalled := false
	got := ownersMatch(
		func() (uint32, error) { return 0, errors.New("file lookup failed") },
		func() (uint32, error) { dirCalled = true; return 1000, nil },
	)
	if got {
		t.Fatalf("expected fail closed on file lookup error")
	}
	if dirCalled {
		t.Fatalf("dir lookup ran after file lookup error; should short-circuit")
	}
}

// TestUIDOf checks the stat-result adapter: propagate stat errors and return
// the owner UID otherwise.
func TestUIDOf(t *testing.T) {
	dir := t.TempDir()
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat temp dir: %v", err)
	}
	uid, err := uidOf(fi, nil)
	if err != nil {
		t.Fatalf("uidOf on valid FileInfo: %v", err)
	}
	if uid != uint32(os.Getuid()) {
		t.Fatalf("uidOf = %d, want current uid %d", uid, os.Getuid())
	}

	statErr := errors.New("stat failed")
	if _, err := uidOf(nil, statErr); !errors.Is(err, statErr) {
		t.Fatalf("uidOf must propagate stat error, got %v", err)
	}
}
