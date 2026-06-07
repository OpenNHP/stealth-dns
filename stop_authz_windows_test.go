//go:build windows

package main

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

// TestOwnersMatch covers the security-critical decision via injected lookups,
// so the same-owner, different-owner, nil-SID, and fail-closed branches run
// with no elevation and no filesystem.
func TestOwnersMatch(t *testing.T) {
	localSystem, err := windows.StringToSid("S-1-5-18")
	if err != nil {
		t.Fatalf("building LocalSystem SID: %v", err)
	}
	otherUser, err := windows.StringToSid("S-1-5-21-1-2-3-1001")
	if err != nil {
		t.Fatalf("building other-user SID: %v", err)
	}
	sid := func(s *windows.SID) func() (*windows.SID, error) {
		return func() (*windows.SID, error) { return s, nil }
	}
	boom := func() (*windows.SID, error) { return nil, errors.New("lookup failed") }

	tests := []struct {
		name      string
		file, dir func() (*windows.SID, error)
		want      bool
	}{
		{"same owner -> authorized", sid(localSystem), sid(localSystem), true},
		{"different owner -> rejected", sid(localSystem), sid(otherUser), false},
		{"nil file SID -> fail closed", sid(nil), sid(localSystem), false},
		{"nil dir SID -> fail closed", sid(localSystem), sid(nil), false},
		{"file lookup error -> fail closed", boom, sid(localSystem), false},
		{"dir lookup error -> fail closed", sid(localSystem), boom, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ownersMatch(tc.file, tc.dir); got != tc.want {
				t.Fatalf("ownersMatch = %v, want %v", got, tc.want)
			}
		})
	}
}
