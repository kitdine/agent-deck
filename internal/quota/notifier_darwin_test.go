//go:build darwin

package quota

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withFakeNotifier(t *testing.T, script string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-osascript.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	previous := notifierCommandArgs
	notifierCommandArgs = []string{"/bin/sh", path}
	t.Cleanup(func() { notifierCommandArgs = previous })
}

func TestOSANotifierPassesTitleAndBodyAsSeparateArguments(t *testing.T) {
	out := filepath.Join(t.TempDir(), "args")
	withFakeNotifier(t, `for arg in "$@"; do printf '%s\n' "$arg" >> `+shellQuotePath(out)+`; done`)

	note := Notification{Kind: AlertThreshold, Client: ClientClaude, Window: `5-hour "window" $(touch pwned)`, UsedPercent: 91, Threshold: 90}
	if err := (OSANotifier{}).Notify(context.Background(), note); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	want := []string{note.Title(), note.Body()}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("arguments = %q, want %q verbatim", got, want)
	}
}

func TestOSANotifierReportsDeliveryFailure(t *testing.T) {
	withFakeNotifier(t, "exit 1\n")
	if err := (OSANotifier{}).Notify(context.Background(), Notification{Kind: AlertReset, Client: ClientCodex, Window: "5-hour window"}); err == nil {
		t.Fatal("Notify returned nil for a failing delivery process")
	}
}

func TestOSANotifierScriptDoesNotEmbedNotificationText(t *testing.T) {
	// Content reaches osascript only as argv; the script statements are fixed.
	for _, arg := range notifierCommandArgs {
		if strings.Contains(arg, "%") {
			t.Fatalf("notifier command argument %q looks like a format template", arg)
		}
	}
	if !strings.Contains(strings.Join(notifierCommandArgs, " "), "item 1 of argv") {
		t.Fatalf("notifier script = %q, want title and body read from argv", notifierCommandArgs)
	}
}
