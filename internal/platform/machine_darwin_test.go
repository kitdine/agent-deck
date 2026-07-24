//go:build darwin

package platform

import (
	"context"
	"errors"
	"testing"
)

func TestPlatformUUIDPattern(t *testing.T) {
	output := []byte(`    "IOPlatformUUID" = "00000000-1111-2222-3333-444444444444"`)
	matches := platformUUIDPattern.FindSubmatch(output)
	if len(matches) != 2 || string(matches[1]) != "00000000-1111-2222-3333-444444444444" {
		t.Fatalf("platform UUID matches = %q", matches)
	}
}

func TestMachineIdentityWithCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	identity, err := MachineIdentity(ctx)
	if identity != "" {
		t.Fatalf("MachineIdentity() identity = %q, want empty", identity)
	}
	if !errors.Is(err, ErrMachineIdentityUnavailable) {
		t.Fatalf("MachineIdentity() error = %v, want ErrMachineIdentityUnavailable", err)
	}
}
