package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/output"
)

func TestDesktopSnapshotMissingStateReturnsStablePartialEnvelope(t *testing.T) {
	state := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	exit := execute([]string{
		"--state-dir", state, "--format", "json",
		"desktop", "snapshot", "--wire-version", "1", "--recent-limit", "3",
	}, bytes.NewReader(nil), &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("desktop snapshot exit = %d, stderr = %s", exit, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("desktop snapshot stderr = %q", stderr.String())
	}
	var envelope struct {
		SchemaVersion int              `json:"schema_version"`
		Command       string           `json:"command"`
		Data          desktop.Snapshot `json:"data"`
		Warnings      []string         `json:"warnings"`
		Partial       bool             `json:"partial"`
		Error         *output.Error    `json:"error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("Unmarshal stdout: %v\n%s", err, stdout.String())
	}
	if envelope.SchemaVersion != output.SchemaVersion || envelope.Command != "desktop.snapshot" || envelope.Data.WireVersion != desktop.WireVersion {
		t.Fatalf("envelope identity = %#v", envelope)
	}
	if !envelope.Partial || envelope.Error != nil {
		t.Fatalf("partial envelope = %#v", envelope)
	}
	wantWarnings := []string{"provider_unavailable", "sessions_unavailable", "usage_unavailable"}
	if !reflect.DeepEqual(envelope.Warnings, wantWarnings) {
		t.Fatalf("warnings = %#v, want %#v", envelope.Warnings, wantWarnings)
	}
	if envelope.Data.Provider.Routes == nil || envelope.Data.Sessions.Items == nil || envelope.Data.Usage.Warnings == nil {
		t.Fatalf("empty collections must encode as arrays: %#v", envelope.Data)
	}
	for _, name := range []string{"agentdeck.sqlite3", "sessions.sqlite3"} {
		if _, err := os.Stat(filepath.Join(state, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s after desktop snapshot: %v", name, err)
		}
	}
}

func TestDesktopSnapshotInputErrorsHaveStableJSONCodesAndExitTwo(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		code string
	}{
		{name: "unsupported wire", args: []string{"--wire-version", "2"}, code: desktop.ErrUnsupportedWireVersion.Error()},
		{name: "zero limit", args: []string{"--recent-limit", "0"}, code: desktop.ErrInvalidRecentLimit.Error()},
		{name: "large limit", args: []string{"--recent-limit", "21"}, code: desktop.ErrInvalidRecentLimit.Error()},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := []string{"--state-dir", t.TempDir(), "--format", "json", "desktop", "snapshot"}
			args = append(args, test.args...)
			exit := execute(args, bytes.NewReader(nil), &stdout, &stderr)
			if exit != 2 {
				t.Fatalf("exit = %d, want 2; stderr = %s", exit, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q", stdout.String())
			}
			var envelope output.Envelope
			if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
				t.Fatalf("Unmarshal stderr: %v\n%s", err, stderr.String())
			}
			if envelope.Command != "desktop.snapshot" || envelope.Error == nil || envelope.Error.Code != test.code {
				t.Fatalf("error envelope = %#v", envelope)
			}
		})
	}
}

func TestDesktopSnapshotRejectsTextOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := execute([]string{"--state-dir", t.TempDir(), "desktop", "snapshot"}, bytes.NewReader(nil), &stdout, &stderr)
	if exit != 2 || stderr.String() != "desktop snapshot requires --format json\n" {
		t.Fatalf("text invocation = exit %d, stdout %q, stderr %q", exit, stdout.String(), stderr.String())
	}
}
