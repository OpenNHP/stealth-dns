//go:build !windows

package main

import (
	"os"
	"syscall"
)

// restrictTokenToDirOwner chowns the stop-token file to the owner of the
// install directory. The daemon runs as root while the UI runs as the logged-in
// user that owns the install directory; chowning lets that user read the token
// (0600) while other local users cannot. nhp#1150 item 1.
func restrictTokenToDirOwner(tokenPath, dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		// Unknown stat backing; leave the file root-owned 0600.
		return nil
	}
	return os.Chown(tokenPath, int(st.Uid), int(st.Gid))
}
