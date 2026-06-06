//go:build windows

package main

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

// stopFileAuthorized reports whether the stop signal file is honored: it is
// honored only when its owner SID matches the install directory's owner SID
// (the threat model and rationale live at the call site in main.go).
//
// This check is conservative — it can only ever over-reject, never accept an
// untrusted writer's file. Under UAC, files created by an elevated process may
// be owned by the Administrators group SID rather than the user SID, so in some
// elevated-UI configurations the legitimate file may be rejected and Stop falls
// back to the existing taskkill/elevation path. The exact owner-SID behavior
// under UAC has not been validated at runtime; treat this path as best-effort
// hardening that never weakens security. An NTFS hard link (mklink /H, no
// privilege) aliases its target's owner SID, so it shares the world-writable-
// dir caveat at the call site. Any lookup error fails closed.
func stopFileAuthorized(stopFilePath string) bool {
	return ownersMatch(
		func() (*windows.SID, error) { return ownerSID(stopFilePath) },
		func() (*windows.SID, error) { return ownerSID(filepath.Dir(stopFilePath)) },
	)
}

// ownersMatch is the pure authorization decision: fail closed on either lookup
// error or a nil SID, otherwise the two owner SIDs must be equal. Injecting the
// lookups keeps the syscall plumbing thin and lets tests exercise the
// same-owner, different-owner, and fail-closed branches without elevation.
func ownersMatch(fileOwner, dirOwner func() (*windows.SID, error)) bool {
	fsid, err := fileOwner()
	if err != nil || fsid == nil {
		return false
	}
	dsid, err := dirOwner()
	if err != nil || dsid == nil {
		return false
	}
	return fsid.Equals(dsid)
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
