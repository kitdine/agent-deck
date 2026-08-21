package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

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

func TestDesktopRefreshIndexesRunsIndependentIncrementalScansInParallel(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	completed := make(chan desktopIndexRefreshResult, 1)
	scan := func(name string) desktopIndexScan {
		return func() (any, error) {
			started <- name
			<-release
			return map[string]int{"changed": 1}, nil
		}
	}
	go func() {
		completed <- runDesktopIndexScans(scan("usage"), scan("sessions"))
	}()

	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case name := <-started:
			seen[name] = true
		case <-time.After(time.Second):
			t.Fatal("incremental scans did not start in parallel")
		}
	}
	close(release)
	result := <-completed
	if !result.Usage.Success || !result.Sessions.Success {
		t.Fatalf("parallel result = %#v", result)
	}
}

func TestDesktopRefreshIndexesReturnsBothIncrementalDomainResults(t *testing.T) {
	state := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	exit := execute([]string{
		"--state-dir", state, "--format", "json", "desktop", "refresh-indexes",
	}, bytes.NewReader(nil), &stdout, &stderr)
	if exit != 0 || stderr.Len() != 0 {
		t.Fatalf("desktop refresh-indexes = exit %d, stderr %q", exit, stderr.String())
	}
	var envelope struct {
		Command  string                    `json:"command"`
		Data     desktopIndexRefreshResult `json:"data"`
		Partial  bool                      `json:"partial"`
		Warnings []string                  `json:"warnings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("Unmarshal stdout: %v", err)
	}
	if envelope.Command != "desktop.refresh-indexes" || envelope.Partial || len(envelope.Warnings) != 0 {
		t.Fatalf("envelope = %#v", envelope)
	}
	if !envelope.Data.Usage.Success || !envelope.Data.Sessions.Success {
		t.Fatalf("domain results = %#v", envelope.Data)
	}
}

func TestDesktopSnapshotStreamUsesBoundedOrderedIntegrityCheckedChunks(t *testing.T) {
	state := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	exit := execute([]string{
		"--state-dir", state, "--format", "json",
		"desktop", "snapshot", "--wire-version", "1", "--recent-limit", "3", "--stream",
	}, bytes.NewReader(nil), &stdout, &stderr)
	if exit != 0 || stderr.Len() != 0 {
		t.Fatalf("streamed desktop snapshot = exit %d, stderr %q", exit, stderr.String())
	}

	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	var rebuilt []byte
	var wantCount, wantBytes int
	var wantDigest string
	for index := 0; decoder.More(); index++ {
		var frame desktopSnapshotChunkEnvelope
		if err := decoder.Decode(&frame); err != nil {
			t.Fatalf("Decode frame %d: %v", index, err)
		}
		if frame.SchemaVersion != output.SchemaVersion || frame.Command != "desktop.snapshot.chunk" || frame.Data.Index != index {
			t.Fatalf("frame %d identity = %#v", index, frame)
		}
		if len(frame.Data.Payload) > base64.StdEncoding.EncodedLen(desktopSnapshotChunkBytes) {
			t.Fatalf("frame %d payload = %d encoded bytes", index, len(frame.Data.Payload))
		}
		chunk, err := base64.StdEncoding.DecodeString(frame.Data.Payload)
		if err != nil {
			t.Fatalf("Decode payload %d: %v", index, err)
		}
		rebuilt = append(rebuilt, chunk...)
		wantCount, wantBytes, wantDigest = frame.Data.Count, frame.Data.TotalBytes, frame.Data.SHA256
	}
	if wantCount == 0 || wantCount != (len(rebuilt)+desktopSnapshotChunkBytes-1)/desktopSnapshotChunkBytes {
		t.Fatalf("chunk count = %d for %d bytes", wantCount, len(rebuilt))
	}
	if wantBytes != len(rebuilt) {
		t.Fatalf("total bytes = %d, rebuilt = %d", wantBytes, len(rebuilt))
	}
	digest := sha256.Sum256(rebuilt)
	if hex.EncodeToString(digest[:]) != wantDigest {
		t.Fatalf("stream digest mismatch")
	}
	var envelope output.Envelope
	if err := json.Unmarshal(rebuilt, &envelope); err != nil {
		t.Fatalf("Unmarshal rebuilt envelope: %v", err)
	}
	if envelope.Command != "desktop.snapshot" || !envelope.Partial {
		t.Fatalf("rebuilt envelope = %#v", envelope)
	}
}

func TestDesktopSnapshotStreamNormalizesEmptyWarningsToArray(t *testing.T) {
	var streamed bytes.Buffer
	if err := writeDesktopSnapshotStream(&streamed, desktop.Result{Snapshot: desktop.Snapshot{WireVersion: desktop.WireVersion}}); err != nil {
		t.Fatalf("writeDesktopSnapshotStream: %v", err)
	}
	var frame desktopSnapshotChunkEnvelope
	if err := json.Unmarshal(streamed.Bytes(), &frame); err != nil {
		t.Fatalf("Unmarshal frame: %v", err)
	}
	payload, err := base64.StdEncoding.DecodeString(frame.Data.Payload)
	if err != nil {
		t.Fatalf("Decode payload: %v", err)
	}
	var envelope struct {
		Warnings json.RawMessage `json:"warnings"`
	}
	if err = json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("Unmarshal payload: %v", err)
	}
	if string(envelope.Warnings) != "[]" {
		t.Fatalf("warnings = %s, want []", envelope.Warnings)
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
