package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/kitdine/agent-deck/internal/scanruntime"
)

func TestScanCommandReturnsOneCompletedSharedRound(t *testing.T) {
	state := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	exit := execute([]string{"--state-dir", state, "--format", "json", "scan"}, bytes.NewReader(nil), &stdout, &stderr)
	if exit != 0 || stderr.Len() != 0 {
		t.Fatalf("scan exit = %d, stderr = %q", exit, stderr.String())
	}
	var envelope struct {
		Command string             `json:"command"`
		Data    scanruntime.Result `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode scan envelope: %v\n%s", err, stdout.String())
	}
	if envelope.Command != "scan" || envelope.Data.RoundID == "" {
		t.Fatalf("scan envelope = %#v", envelope)
	}
	if envelope.Data.Usage.State != "completed" || envelope.Data.Session.State != "completed" {
		t.Fatalf("scan terminal states = usage=%#v session=%#v", envelope.Data.Usage, envelope.Data.Session)
	}
}

func TestScanCommandRejectsUnsupportedScopeBeforeWorkerLaunch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := execute([]string{"--state-dir", t.TempDir(), "scan", "--scope", "extension"}, bytes.NewReader(nil), &stdout, &stderr)
	if exit == 0 {
		t.Fatal("scan accepted unsupported scope")
	}
	if !bytes.Contains(stderr.Bytes(), []byte("invalid scan scope")) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
