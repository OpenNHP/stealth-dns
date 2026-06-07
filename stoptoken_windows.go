//go:build windows

package main

// restrictTokenToDirOwner is a best-effort no-op on Windows: the stop-token
// file inherits the install directory's ACLs. Tightening to a specific SID
// would require the golang.org/x/sys/windows security APIs; the token still
// removes the previous "any file presence stops the daemon" behavior, since a
// caller must now read and echo the token to request a shutdown. nhp#1150 item 1.
func restrictTokenToDirOwner(tokenPath, dir string) error {
	_ = tokenPath
	_ = dir
	return nil
}
