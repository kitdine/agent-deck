//go:build darwin

package quota

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// DefaultNotifyTimeout bounds one notification delivery.
const DefaultNotifyTimeout = 5 * time.Second

// notifierCommandArgs runs a fixed AppleScript that displays argv's two
// items. The title and body travel as process arguments, never spliced into
// the script text, so notification content cannot change what runs.
// Package-level so tests substitute a fake process.
var notifierCommandArgs = []string{
	"/usr/bin/osascript",
	"-e", "on run argv",
	"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
	"-e", "end run",
}

// DefaultNotifier is the platform's notification delivery.
func DefaultNotifier() Notifier { return OSANotifier{} }

// OSANotifier delivers C10 notifications through macOS Notification Centre
// via osascript. It reads no credential and opens no network connection (C0).
type OSANotifier struct {
	Timeout time.Duration
}

func (n OSANotifier) Notify(ctx context.Context, note Notification) error {
	timeout := n.Timeout
	if timeout <= 0 {
		timeout = DefaultNotifyTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := append(append([]string{}, notifierCommandArgs[1:]...), note.Title(), note.Body())
	if err := exec.CommandContext(ctx, notifierCommandArgs[0], args...).Run(); err != nil {
		return fmt.Errorf("deliver quota notification: %w", err)
	}
	return nil
}
