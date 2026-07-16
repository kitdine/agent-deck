package platform

import (
	"context"
	"errors"
)

var ErrMachineIdentityUnavailable = errors.New("machine_identity_unavailable")

func MachineIdentity(ctx context.Context) (string, error) {
	return machineIdentity(ctx)
}
