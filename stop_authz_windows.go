//go:build windows

package main

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

// stopFileAuthorized reports whether the stop signal file at stopFilePath
// exists and was created by a principal trusted to stop the daemon.
//
// The shutdown IPC is an unprotected location: any local process that can
// write to the daemon's directory could otherwise drop the sentinel file and
// stop the daemon — a local denial of service. We bind the right to stop the
// daemon to ownership of the install directory: the file is only honored when
// its owner SID matches the owner SID of the directory it lives in. A file
// created by any other local user is owned by that user's SID and is rejected.
//
// Note: this check is conservative — it can only ever over-reject, never
// accept an untrusted writer's file. Under UAC, files created by an elevated
// process may be owned by the Administrators group SID rather than the user
// SID, so in some elevated-UI configurations the legitimate file may be
// rejected and Stop falls back to the existing taskkill/elevation path. The
// exact owner-SID behavior under UAC has not been validated at runtime; treat
// this path as best-effort hardening that never weakens security.
//
// As on Unix, there is a TOCTOU window before the caller acts on the file and
// the check is moot if the install directory is world-writable; and an NTFS
// hard link (mklink /H, which needs no privilege) aliases its target's owner
// SID, so a hard link to a dir-owner-owned file would still pass. All of these
// require the attacker to already have write access to the install directory,
// whose worst case is the local DoS this guard bounds; the real mitigation is
// keeping the install dir non-world-writable.
func stopFileAuthorized(stopFilePath string) bool {
	fileOwner, err := ownerSID(stopFilePath)
	if err != nil {
		return false
	}
	dirOwner, err := ownerSID(filepath.Dir(stopFilePath))
	if err != nil {
		return false
	}
	return fileOwner.Equals(dirOwner)
}

// ownerSID returns the owner SID of the filesystem object at path.
func ownerSID(path string) (*windows.SID, error) {
	sd, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION,
	)
	if err != nil {
		return nil, err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return nil, err
	}
	return owner, nil
}
