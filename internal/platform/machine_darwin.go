//go:build darwin

package platform

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
)

var platformUUIDPattern = regexp.MustCompile(`"IOPlatformUUID"\s*=\s*"([^"]+)"`)

func machineIdentity(ctx context.Context) (string, error) {
	output, err := exec.CommandContext(ctx, "/usr/sbin/ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrMachineIdentityUnavailable, err)
	}
	matches := platformUUIDPattern.FindSubmatch(output)
	if len(matches) != 2 || len(matches[1]) == 0 {
		return "", ErrMachineIdentityUnavailable
	}
	return string(matches[1]), nil
}
