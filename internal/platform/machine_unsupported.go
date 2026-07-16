//go:build !darwin

package platform

import "context"

func machineIdentity(context.Context) (string, error) {
	return "", ErrMachineIdentityUnavailable
}
