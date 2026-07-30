//go:build unix

package shellconfig

import (
	"io/fs"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestOwnedByCurrentUserChecksUnixUID(t *testing.T) {
	currentUID := uint32(os.Geteuid())
	differentUID := currentUID + 1

	tests := []struct {
		name string
		sys  any
		want bool
	}{
		{name: "current UID", sys: &syscall.Stat_t{Uid: currentUID}, want: true},
		{name: "different UID", sys: &syscall.Stat_t{Uid: differentUID}, want: false},
		{name: "missing stat", sys: nil, want: false},
		{name: "unexpected stat type", sys: struct{}{}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := unixOwnerFileInfo{sys: test.sys}
			if got := ownedByCurrentUser(info); got != test.want {
				t.Fatalf("ownedByCurrentUser() = %v, want %v", got, test.want)
			}
		})
	}
}

type unixOwnerFileInfo struct {
	sys any
}

func (unixOwnerFileInfo) Name() string       { return "startup" }
func (unixOwnerFileInfo) Size() int64        { return 0 }
func (unixOwnerFileInfo) Mode() fs.FileMode  { return 0o600 }
func (unixOwnerFileInfo) ModTime() time.Time { return time.Time{} }
func (unixOwnerFileInfo) IsDir() bool        { return false }
func (info unixOwnerFileInfo) Sys() any      { return info.sys }
