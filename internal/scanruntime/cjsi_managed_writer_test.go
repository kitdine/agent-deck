//go:build cjsi_adapter_fixture

package scanruntime

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/kitdine/agent-deck/internal/store"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestManagedWorkerJournalHandoff(t *testing.T) {
	root := t.TempDir()
	j, err := openReceiptJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	r := scanReceipt{ID: "accepted", StateID: "root", Home: "/fixture", Scope: ScopeBoth, RoundID: "original", Observation: 7}
	if err := j.accept(r); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(j.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := moveManagedJournal(root, j.path); err != nil {
		t.Fatal(err)
	}
	if _, err := openReceiptJournal(root); err == nil {
		t.Fatal("old Serve could proceed to socket deletion")
	}
	after, err := os.ReadFile(filepath.Join(j.path, receiptJournalFilename))
	if err != nil || string(before) != string(after) {
		t.Fatalf("journal handoff changed original receipts: %v", err)
	}
	moved, err := openReceiptJournal(j.path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := moved.get(r.ID)
	if !ok || got.RoundID != r.RoundID || got.Observation != r.Observation || got.State != receiptAccepted {
		t.Fatalf("lost recoverable round: %+v", got)
	}
}

func TestManagedWorkerSocketOwnership(t *testing.T) {
	dir, err := os.MkdirTemp("", "cjsi-uds-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "w")
	old, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	old.SetUnlinkOnClose(false)
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := removeStaleWorkerSocket(path); err == nil {
		t.Fatal("deleted live worker endpoint")
	}
	// Preserve a hard link to the old inode across removal. This makes
	// the successor-identity check independent of inode reuse.
	if err := os.Link(path, filepath.Join(dir, "prior")); err != nil {
		t.Fatal(err)
	}
	old.Close()
	if err := removeStaleWorkerSocket(path); err != nil {
		t.Fatal(err)
	}
	// Keep a reference to another entry so inode reuse cannot make it
	// indistinguishable from a removed socket in this cleanup control.
	other := filepath.Join(dir, "owner")
	if err := os.WriteFile(other, []byte("successor"), 0600); err != nil {
		t.Fatal(err)
	}
	owner, err := os.Lstat(other)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(other, path); err != nil {
		t.Fatal(err)
	}
	removeWorkerEntry(path, info)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		t.Fatal("cleanup removed another owner's entry")
	}
	removeWorkerEntry(path, owner)
}

func TestManagedWorkerPendingOverSocket(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	legacy, err := store.Open(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	index, err := store.OpenSessions(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := index.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.InstallTask5Fixture(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".codex", "sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "source.jsonl"), []byte(`{"type":"session_meta","payload":{"id":"00000000-0000-4000-8000-000000000002"}}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Persist a real legacy journal first; the elected managed worker must
	// retain its terminal receipt and admit new requests on the same wire.
	_, sid, err := prepareStateRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	j, err := openReceiptJournal(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.accept(scanReceipt{ID: "earlier", StateID: sid, Home: home, Scope: ScopeBoth, RoundID: "old-round", Observation: 5}); err != nil {
		t.Fatal(err)
	}
	if err := j.terminal("old-round", Result{RoundID: "old-round", Usage: UsageResult{State: "completed"}, Session: SessionResult{State: "completed"}}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	var workers sync.WaitGroup
	client := Client{StateRoot: root, Home: home, RequestID: "earlier", Launch: func(context.Context, string) error {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if e := Serve(ctx, root); e != nil {
				t.Errorf("Serve: %v", e)
			}
		}()
		return nil
	}}
	defer func() { cancel(); workers.Wait() }()
	bounded, stop := context.WithTimeout(t.Context(), time.Minute)
	defer stop()
	old, err := client.Request(bounded, ScopeBoth)
	if err != nil || old.RoundID != "old-round" {
		t.Fatalf("lost prior receipt: %+v %v", old, err)
	}
	endpoint, e := socketPath(root, sid)
	if e != nil {
		t.Fatal(e)
	}
	conn, e := dial(endpoint)
	if e != nil {
		t.Fatal(e)
	}
	if e = json.NewEncoder(conn).Encode(wireRequest{Protocol: ProtocolVersion, StateID: sid, Request: "wrong-root", Home: home, Scope: ScopeBoth, PhysicalRoot: "other"}); e != nil {
		t.Fatal(e)
	}
	var rejected wireResponse
	if e = json.NewDecoder(conn).Decode(&rejected); e != nil || rejected.Type != "rejected" {
		t.Fatalf("foreign physical root: %+v %v", rejected, e)
	}
	conn.Close()
	client.RequestID = "missing-authority"
	result, err := client.Request(t.Context(), ScopeBoth)
	if err != nil || result.Usage.ErrorCode != "source_identity_pending" || result.Session.ErrorCode != "source_identity_pending" {
		t.Fatalf("unqualified round: %+v %v", result, err)
	}
	release, err := AcquireMaintenance(t.Context(), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	replayed, err := client.Request(t.Context(), ScopeBoth)
	if err != nil || replayed.RoundID != result.RoundID {
		t.Fatalf("socket/journal owner changed: %+v %v", replayed, err)
	}
}

// CJSI-WRITER-R1-F1: the identity handed to the runner is the held directory,
// not the journal bytes. Replacing the entry with an identical v1 journal in a
// new directory supersedes the election.
func TestManagedWorkerElectionRejectsReplacedJournalDirectory(t *testing.T) {
	root := t.TempDir()
	legacy, err := store.Open(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	index, err := store.OpenSessions(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := index.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.InstallTask5Fixture(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	m, err := store.ResolveCJSIRuntimeRoot(t.Context(), root)
	if err != nil || m == nil {
		t.Fatalf("managed root: %+v %v", m, err)
	}
	e, j, err := electManagedWorker(root, *m)
	if err != nil {
		t.Fatal(err)
	}
	defer e.dir.Close()
	if err := j.accept(scanReceipt{ID: "accepted", StateID: "root", Home: "/fixture", Scope: ScopeBoth, RoundID: "r", Observation: 1}); err != nil {
		t.Fatal(err)
	}
	r := &round{managed: m, election: e}
	if r.election.check() != nil {
		t.Fatal("fresh election rejected")
	}
	active := filepath.Join(root, receiptJournalFilename)
	receipts, err := os.ReadFile(filepath.Join(active, receiptJournalFilename))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(active, active+".prior"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(active, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(active, receiptJournalFilename), receipts, 0600); err != nil {
		t.Fatal(err)
	}
	if err := r.election.check(); !errors.Is(err, store.ErrLockLost) {
		t.Fatalf("superseded election still current: %v", err)
	}
}
