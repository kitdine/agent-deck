//go:build !darwin

package platform

import (
	"context"
	"testing"
)

func TestMachineIdentityUnsupportedPlatform(t *testing.T) {
	identity, err := MachineIdentity(context.Background())
	if identity != "" {
		t.Fatalf("MachineIdentity() identity = %q, want empty", identity)
	}
	if err != ErrMachineIdentityUnavailable {
		t.Fatalf("MachineIdentity() error = %v, want exact ErrMachineIdentityUnavailable sentinel", err)
	}
}
