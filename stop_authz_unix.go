//go:build linux || darwin

package main

import (
	"os"
	"path/filepath"
	"syscall"
)

// stopFileAuthorized reports whether the stop signal file at stopFilePath
// exists and was created by a principal trusted to stop the daemon.
//
// The shutdown IPC is an unprotected location: any local process that can
// write to the daemon's directory could otherwise drop the sentinel file and
// stop the (root-owned) daemon — a local denial of service. We bind the right
// to stop the daemon to ownership of the install directory: the file is only
// honored when its owner UID matches the owner UID of the directory it lives
// in. The legitimate stopper is the unprivileged UI running as the desktop
// user who owns the install directory, so its file is honored; a file touched
// by any other local user is owned by that user and is rejected.
//
// (A root attacker could forge ownership, but root can already signal the
// daemon directly, so this changes nothing about the threat model.)
//
// Caveats, acceptable for this local-only, low-severity threat model:
//   - We Lstat the sentinel so the owner check inspects the entry itself and
//     does not follow a symlink to a dir-owner-owned target. This is a strict
//     improvement over Stat but is not full forgery resistance: a hard link
//     aliases its inode's owner, so a hard link to a dir-owner-owned file in a
//     world-writable install dir would still pass. The directory is the trust
//     anchor, so it is Stat'd normally.
//   - There is a TOCTOU window between this check and acting on the file in
//     the caller, and ownership gating is moot if the install directory is
//     itself world-writable. The symlink, hard-link, and TOCTOU cases all
//     require the attacker to already have write access to the install
//     directory, whose worst case is exactly the local DoS this guard bounds;
//     the real mitigation is keeping the install dir non-world-writable.
func stopFileAuthorized(stopFilePath string) bool {
	fi, err := os.Lstat(stopFilePath)
	if err != nil {
		return false
	}
	fst, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}

	di, err := os.Stat(filepath.Dir(stopFilePath))
	if err != nil {
		return false
	}
	dst, ok := di.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}

	return fst.Uid == dst.Uid
}
