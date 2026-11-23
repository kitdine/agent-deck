package store

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/ingest"
)

func openCJSIWorksetFixture(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := openWritableSQL(path+"?"+writablePragmas, "core")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newCJSIWorksetFixture(t *testing.T, input string) (*sql.DB, string, *os.File, cjsiCheckpointBinding) {
	t.Helper()
	root := t.TempDir()
	dbPath := filepath.Join(root, "workset.sqlite")
	db := openCJSIWorksetFixture(t, dbPath)
	if _, err := db.ExecContext(t.Context(), cjsiCheckpointWorksetSchema); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(filepath.Join(root, "synthetic-source.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	if _, err := file.WriteString(input); err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	identity, _, stable := ingest.FileGeneration(info)
	if !stable {
		t.Fatal("fixture lacks file identity")
	}
	b := cjsiCheckpointBinding{Round: 1, Attempt: 1, Identity: identity, CaptureEnd: int64(len(input)),
		Version: ingest.CodexCheckpointVersion{Generation: 7, Parser: 9, MaxLineBytes: 256}}
	return db, dbPath, file, b
}

func runCJSIWorksetReplay(t *testing.T, db *sql.DB, file *os.File, b cjsiCheckpointBinding, resume *ingest.CodexCheckpointResume) (*cjsiCheckpointWorkset, ingest.CodexCheckpointStep, error) {
	t.Helper()
	sink, err := beginCJSICheckpointWorkset(t.Context(), db, b)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := ingest.BeginCodexCheckpointReplay(t.Context(), file, file.Name(), b.CaptureEnd, b.Version, resume, sink)
	if err != nil {
		sink.Abort()
		return sink, ingest.CodexCheckpointStep{}, err
	}
	defer replay.Abort()
	var readEnd int64
	const budget = 97
	for {
		step, err := replay.Step(t.Context(), budget)
		if err != nil {
			return sink, step, err
		}
		if step.Progress.ReadEnd-readEnd > budget {
			t.Fatal("replay exceeded source budget")
		}
		if step.Done {
			return sink, step, nil
		}
		if _, err := openCJSICheckpointResume(t.Context(), db, b); !errors.Is(err, errCJSICheckpointWorkset) {
			t.Fatalf("draft became a resume source: %v", err)
		}
		readEnd = step.Progress.ReadEnd
	}
}

func TestCJSICheckpointWorkset(t *testing.T) {
	t.Run("legacy ID replays", func(t *testing.T) {
		input := "{\"type\":\"session_meta\",\"payload\":{\"session_id\":\"a/b\"}}\n"
		db, _, file, b := newCJSIWorksetFixture(t, input)
		sink, step, err := runCJSIWorksetReplay(t, db, file, b, nil)
		if err != nil {
			t.Fatalf("ingest-accepted legacy ID prevented replay: %v", err)
		}
		if err := sink.Seal(t.Context(), step); err != nil {
			t.Fatal(err)
		}
		var encoded []byte
		if err := db.QueryRowContext(t.Context(), "SELECT metadata FROM cjsi_checkpoint_records").Scan(&encoded); err != nil {
			t.Fatal(err)
		}
		var meta cjsiCheckpointMetadata
		if err := json.Unmarshal(encoded, &meta); err != nil || meta.LegacyID != "a/b" {
			t.Fatalf("legacy ID did not survive staging: %+v: %v", meta, err)
		}
		resume, err := openCJSICheckpointResume(t.Context(), db, b)
		if err != nil {
			t.Fatal(err)
		}
		b.Attempt++
		if _, _, err := runCJSIWorksetReplay(t, db, file, b, resume); err != nil {
			t.Fatalf("sealed legacy ID could not replay: %v", err)
		}
	})

	t.Run("canonical file identity", func(t *testing.T) {
		_, _, _, b := newCJSIWorksetFixture(t, "")
		if !b.valid() {
			t.Fatal("native file identity failed validation")
		}
		for _, identity := range []string{"+1:2", "01:2", "-0:2", "1:02"} {
			b.Identity = identity
			if b.valid() {
				t.Errorf("accepted noncanonical identity %q", identity)
			}
		}
	})

	t.Run("seal reopen ordered resume and extend", func(t *testing.T) {
		prefix := "{\"type\":\"session_meta\",\"payload\":{\"id\":\"00000000-0000-4000-8000-000000000001\"}}\n" +
			strings.Repeat("{\"type\":\"synthetic\"}\n", 256)
		raw := prefix + "{\"type\":\"synthetic\",\"ignored\":\"PARTIAL_SENTINEL\""
		db, path, file, b := newCJSIWorksetFixture(t, raw)
		sink, step, err := runCJSIWorksetReplay(t, db, file, b, nil)
		if err != nil || step.Progress.CompleteEnd != int64(len(prefix)) || !step.Progress.Partial {
			t.Fatalf("replay boundary: %+v, %v", step.Progress, err)
		}
		if _, err := openCJSICheckpointResume(t.Context(), db, b); !errors.Is(err, errCJSICheckpointWorkset) {
			t.Fatal("terminal replay became visible before seal")
		}
		bad := step
		bad.Prefix.CompleteEnd--
		if err := sink.Seal(t.Context(), bad); !errors.Is(err, errCJSICheckpointWorkset) {
			t.Fatal("seal accepted mismatched replay boundary")
		}
		for range 2 {
			if err := sink.Seal(t.Context(), step); err != nil {
				t.Fatal(err)
			}
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		db = openCJSIWorksetFixture(t, path)
		resume, err := openCJSICheckpointResume(t.Context(), db, b)
		if err != nil || resume.Prefix != step.Prefix {
			t.Fatalf("sealed resume did not survive reopen: %v", err)
		}
		next := b
		next.Attempt++
		next.CaptureEnd += 2
		if _, err := file.WriteAt([]byte("}\n"), b.CaptureEnd); err != nil {
			t.Fatal(err)
		}
		nextSink, extended, err := runCJSIWorksetReplay(t, db, file, next, resume)
		if err != nil || extended.Progress.Partial || extended.Progress.CompleteEnd != next.CaptureEnd {
			t.Fatalf("append resume failed: %+v, %v", extended.Progress, err)
		}
		if err := nextSink.Seal(t.Context(), extended); err != nil {
			t.Fatal(err)
		}
		resume, err = openCJSICheckpointResume(t.Context(), db, next)
		if err != nil {
			t.Fatal(err)
		}
		var cursor int64
		for count := 0; ; count++ {
			row, err := resume.Next(t.Context())
			if errors.Is(err, io.EOF) {
				if cursor != next.CaptureEnd || count != 258 {
					t.Fatalf("premature EOF: offset=%d, rows=%d", cursor, count)
				}
				break
			}
			if err != nil || row.Span.Start != cursor {
				t.Fatalf("resume order: %v", err)
			}
			cursor = row.Span.End
		}
		for _, mutate := range []func(*cjsiCheckpointBinding){
			func(v *cjsiCheckpointBinding) { v.Round++ },
			func(v *cjsiCheckpointBinding) { v.Attempt++ },
			func(v *cjsiCheckpointBinding) { v.Version.Generation++ },
			func(v *cjsiCheckpointBinding) { v.Version.Parser++ },
			func(v *cjsiCheckpointBinding) { v.Version.MaxLineBytes++ },
			func(v *cjsiCheckpointBinding) { v.Identity = "9:9" },
			func(v *cjsiCheckpointBinding) { v.CaptureEnd++ },
		} {
			bad := next
			mutate(&bad)
			if _, err := openCJSICheckpointResume(t.Context(), db, bad); !errors.Is(err, errCJSICheckpointWorkset) {
				t.Fatal("cross-binding read succeeded")
			}
		}
	})

	t.Run("abandoned draft stays closed and abort releases round", func(t *testing.T) {
		db, path, file, b := newCJSIWorksetFixture(t, "{\"type\":\"synthetic\"}\n")
		sink, step, err := runCJSIWorksetReplay(t, db, file, b, nil)
		if err != nil {
			t.Fatal(err)
		}
		next := b
		next.Attempt++
		if _, err := beginCJSICheckpointWorkset(t.Context(), db, next); err == nil {
			t.Fatal("overlapping draft admitted")
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		sink.Abort() // cleanup cannot write on the old handle
		db = openCJSIWorksetFixture(t, path)
		if _, err := openCJSICheckpointResume(t.Context(), db, b); !errors.Is(err, errCJSICheckpointWorkset) {
			t.Fatal("abandoned draft became selectable after reopen")
		}
		if err := discardCJSICheckpointWorkset(t.Context(), db, b); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.QueryRow("SELECT count(*) FROM cjsi_checkpoint_records").Scan(&count); err != nil || count != 0 {
			t.Fatal("recovery retained rows for an aborted attempt")
		}
		if _, err := beginCJSICheckpointWorkset(t.Context(), db, b); err == nil {
			t.Fatal("aborted attempt ID was reused")
		}
		if err := sink.Seal(t.Context(), step); !errors.Is(err, errCJSICheckpointWorkset) {
			t.Fatal("stale sink sealed after abort")
		}
		retry, step, err := runCJSIWorksetReplay(t, db, file, next, nil)
		if err != nil || retry.Seal(t.Context(), step) != nil {
			t.Fatalf("fresh retry failed: %v", err)
		}
		retry.Abort()
		if _, err := openCJSICheckpointResume(t.Context(), db, next); err != nil {
			t.Fatal("abort mutated sealed evidence")
		}
	})

	t.Run("revalidation rejects bad digest and aborts new rows", func(t *testing.T) {
		const line = "{\"type\":\"synthetic\"}\n"
		db, path, file, b := newCJSIWorksetFixture(t, line+line)
		sink, step, err := runCJSIWorksetReplay(t, db, file, b, nil)
		if err != nil || sink.Seal(t.Context(), step) != nil {
			t.Fatal("setup seal failed")
		}
		// Simulate damaged old evidence without changing the source.
		if _, err := db.Exec(`UPDATE cjsi_checkpoint_records SET digest=zeroblob(32) WHERE start_byte=?`, len(line)); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		db = openCJSIWorksetFixture(t, path)
		resume, err := openCJSICheckpointResume(t.Context(), db, b)
		if err != nil {
			t.Fatal(err)
		}
		next := b
		next.Attempt++
		_, failed, err := runCJSIWorksetReplay(t, db, file, next, resume)
		if !errors.Is(err, ingest.ErrSourceChanged) || failed.Done {
			t.Fatal("bad old digest passed ingest replay")
		}
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM cjsi_checkpoint_records WHERE attempt_id=?`, next.Attempt).Scan(&count); err != nil || count != 0 {
			t.Fatal("failed attempt exposed already staged rows")
		}
		if _, err := openCJSICheckpointResume(t.Context(), db, next); !errors.Is(err, errCJSICheckpointWorkset) {
			t.Fatal("failed attempt was selectable")
		}
	})

	t.Run("failed row insert rolls back its watermark", func(t *testing.T) {
		const line = "{\"type\":\"synthetic\"}\n"
		db, _, file, b := newCJSIWorksetFixture(t, line+line)
		if _, err := db.Exec(`CREATE TRIGGER fixture_reject_second BEFORE INSERT ON cjsi_checkpoint_records
			WHEN NEW.start_byte>0 BEGIN SELECT RAISE(ABORT,'fixture reject'); END`); err != nil {
			t.Fatal(err)
		}
		_, failed, err := runCJSIWorksetReplay(t, db, file, b, nil)
		if err == nil || failed.Done {
			t.Fatal("failed insert minted a terminal step")
		}
		var completeEnd, count int
		var state string
		if err := db.QueryRow(`SELECT a.complete_end,a.state,(SELECT count(*) FROM cjsi_checkpoint_records)
			FROM cjsi_checkpoint_attempts a`).Scan(&completeEnd, &state, &count); err != nil ||
			completeEnd != len(line) || state != "aborted" || count != 0 {
			t.Fatalf("failed insert left progress or selectable rows: %d, %s, %d, %v", completeEnd, state, count, err)
		}
	})

	t.Run("EOF requires a sealed contiguous prefix", func(t *testing.T) {
		const line = "{\"type\":\"synthetic\"}\n"
		for _, corruption := range []string{"missing", "overlap", "extra", "lost seal"} {
			t.Run(corruption, func(t *testing.T) {
				db, _, file, b := newCJSIWorksetFixture(t, line+line)
				sink, step, err := runCJSIWorksetReplay(t, db, file, b, nil)
				if err != nil || sink.Seal(t.Context(), step) != nil {
					t.Fatal("setup seal failed")
				}
				resume, err := openCJSICheckpointResume(t.Context(), db, b)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := resume.Next(t.Context()); err != nil {
					t.Fatal(err)
				}
				switch corruption {
				case "missing":
					_, err = db.Exec(`DELETE FROM cjsi_checkpoint_records WHERE start_byte>0`)
				case "overlap":
					_, err = db.Exec(`UPDATE cjsi_checkpoint_records SET start_byte=start_byte-1 WHERE start_byte>0`)
				case "extra":
					if _, err = resume.Next(t.Context()); err != nil {
						t.Fatal(err)
					}
					_, err = db.Exec(`INSERT INTO cjsi_checkpoint_records SELECT round_id,attempt_id,end_byte,end_byte+1,
						digest,kind,thread_id,turn_id,call_id,metadata FROM cjsi_checkpoint_records WHERE start_byte>0`)
				case "lost seal":
					if _, err = resume.Next(t.Context()); err != nil {
						t.Fatal(err)
					}
					_, err = db.Exec(`UPDATE cjsi_checkpoint_attempts SET state='aborted'`)
				}
				if err != nil {
					t.Fatal(err)
				}
				if _, err := resume.Next(t.Context()); !errors.Is(err, errCJSICheckpointWorkset) {
					t.Fatalf("corrupt evidence was accepted or treated as EOF: %v", err)
				}
			})
		}
	})

	t.Run("durable observations omit private content", func(t *testing.T) {
		const id = "00000000-0000-4000-8000-000000000001"
		input := `{"type":"session_meta","payload":{"id":"` + id + `","cwd":"/PATH_SENTINEL"}}` + "\n" +
			`{"type":"response_item","payload":{"type":"message","role":"user","content":"TEXT_SENTINEL"}}` + "\n" +
			`{"type":"response_item","payload":{"type":"function_call","name":"exec_command","call_id":"call","arguments":"INPUT_SENTINEL"}}` + "\n" +
			`{"type":"response_item","payload":{"type":"function_call_output","call_id":"call","output":"OUTPUT_SENTINEL"}}` + "\n" +
			`{"type":"token_usage_record","payload":{"thread_id":"` + id + `","turn_id":"turn","response_id":"r","usage":{"input_tokens":0,"output_tokens":1}}}` + "\n" +
			`{"type":"synthetic","credential":"CREDENTIAL_SENTINEL"`
		db, path, file, b := newCJSIWorksetFixture(t, input)
		sink, step, err := runCJSIWorksetReplay(t, db, file, b, nil)
		if err != nil || sink.Seal(t.Context(), step) != nil {
			t.Fatal("setup seal failed")
		}
		var raw string
		if err := db.QueryRow(`SELECT metadata FROM cjsi_checkpoint_records WHERE kind='request_usage' AND turn_id='turn'`).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var meta cjsiCheckpointMetadata
		if err := json.Unmarshal([]byte(raw), &meta); err != nil || meta.Usage == nil || meta.Usage.Input == nil ||
			meta.Usage.Input.State != "valid" || meta.Usage.Input.Value != 0 || meta.Usage.Cached != nil || meta.ResponseID != "r" {
			t.Fatal("explicit zero, absent field, or approved identity was lost")
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"PATH_SENTINEL", "TEXT_SENTINEL", "INPUT_SENTINEL", "OUTPUT_SENTINEL", "CREDENTIAL_SENTINEL", file.Name()} {
			if bytes.Contains(content, []byte(forbidden)) {
				t.Fatal("private fixture content reached the durable workset")
			}
		}
	})
}
