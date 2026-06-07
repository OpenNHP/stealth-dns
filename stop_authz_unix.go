//go:build linux || darwin

package main

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// stopFileAuthorized reports whether the stop signal file is honored: it is
// honored only when its owner UID matches the install directory's owner UID
// (the threat model and rationale live at the call site in main.go).
//
// We Lstat the sentinel so the owner check inspects the entry itself and does
// not follow a symlink to a dir-owner-owned target — a strict improvement over
// Stat, though not full forgery resistance (a hard link aliases its inode's
// owner; see the world-writable-dir caveat at the call site). The directory is
// the trust anchor, so it is Stat'd normally. Any lookup error fails closed.
func stopFileAuthorized(stopFilePath string) bool {
	return ownersMatch(
		func() (uint32, error) { return uidOf(os.Lstat(stopFilePath)) },
		func() (uint32, error) { return uidOf(os.Stat(filepath.Dir(stopFilePath))) },
	)
}

// ownersMatch is the pure authorization decision: fail closed on either lookup
// error, otherwise the two owner UIDs must be equal. Injecting the lookups
// keeps the syscall plumbing thin and lets tests exercise the same-owner,
// different-owner, and fail-closed branches without root.
func ownersMatch(fileOwner, dirOwner func() (uint32, error)) bool {
	fuid, err := fileOwner()
	if err != nil {
		return false
	}
	duid, err := dirOwner()
	if err != nil {
		return false
	}
	return fuid == duid
}

// uidOf extracts the owner UID from an os.Stat/os.Lstat result, propagating any
// stat error and failing on platforms without a *syscall.Stat_t backing.
func uidOf(fi os.FileInfo, err error) (uint32, error) {
	if err != nil {
		return 0, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, errNoStatT
	}
	return st.Uid, nil
}

var errNoStatT = errors.New("stop_authz: FileInfo.Sys() is not *syscall.Stat_t")
