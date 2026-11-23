package store

import (
	"crypto/sha256"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/ingest"
)

func TestCJSICheckpointTypedReadback(t *testing.T) {
	input := `{"timestamp":"2026-01-02T09:00:00+01:00","type":"session_meta","payload":{"session_id":"a/b","cwd":"/PATH_SENTINEL"}}` + "\n" +
		`{"type":"token_usage_record","payload":{"thread_id":"00000000-0000-4000-8000-000000000001","turn_id":"turn","response_id":"r","usage":{"input_tokens":0,"output_tokens":1,"cached_input_tokens":"invalid"}}}` + "\n" +
		`{"type":"response_item","payload":{"type":"function_call","call_id":"call","name":"exec_command","arguments":"ARG_SENTINEL"}}` + "\n" +
		`{"type":"response_item","payload":{"type":"message","role":"user","content":"TEXT_SENTINEL"}}` + "\n" +
		strings.Repeat("OVERSIZE_SENTINEL", 20) + "\n" + `{"type":"PARTIAL_SENTINEL"`
	db, path, file, b := newCJSIWorksetFixture(t, input)
	sink, step, err := runCJSIWorksetReplay(t, db, file, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Seal(t.Context(), step); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db = openCJSIWorksetFixture(t, path)
	reader, err := openCJSICheckpointRecords(t.Context(), db, b)
	if err != nil || reader.Prefix() != step.Prefix {
		t.Fatalf("readback did not recover the sealed prefix: %v", err)
	}
	seen := 0
	_, err = ingest.CaptureCodexPrefix(t.Context(), file, file.Name(), b.CaptureEnd, b.Version.MaxLineBytes, false, func(want ingest.CodexRecord, _ *ingest.CodexVisible) error {
		got, evidence, err := reader.Next(t.Context())
		if err != nil {
			return err
		}
		span := want.Span
		if got != want || evidence.Span != span || evidence.Digest != sha256.Sum256([]byte(input[span.Start:span.End])) {
			t.Fatalf("typed row %d differs from the verified lexical observation", seen)
		}
		seen++
		return nil
	})
	if err != nil || seen != 5 {
		t.Fatalf("readback did not cover five complete observations: %d, %v", seen, err)
	}
	if _, _, err := reader.Next(t.Context()); !errors.Is(err, io.EOF) {
		t.Fatalf("partial suffix changed EOF: %v", err)
	}
}

func TestCJSICheckpointTypedReadRejectsDamagedRows(t *testing.T) {
	const line = "{\"type\":\"synthetic\"}\n"
	for _, tc := range []struct {
		name, sql, value string
	}{
		{"malformed JSON", "metadata=?", "{"},
		{"unknown private field", "metadata=?", `{"content":"PRIVATE_SENTINEL"}`},
		{"repeated field", "metadata=?", `{"legacy_id":"a/b","legacy_id":"a/b"}`},
		{"noncanonical time", "metadata=?", `{"at":"2026-01-02T09:00:00+01:00"}`},
		{"invalid count state", "metadata=?", `{"usage":{"state":"valid","input":{"state":"unknown","value":0}}}`},
		{"oversized metadata", "metadata=?", strings.Repeat("x", 8193)},
		{"invalid indexed field", "turn_id=?", "/PATH_SENTINEL"},
		{"non-text indexed field", "turn_id=CAST(? AS BLOB)", "turn"},
		{"unknown record kind", "kind=?", "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _, file, b := newCJSIWorksetFixture(t, line+line)
			sink, step, err := runCJSIWorksetReplay(t, db, file, b, nil)
			if err != nil || sink.Seal(t.Context(), step) != nil {
				t.Fatal("fixture seal failed")
			}
			reader, err := openCJSICheckpointRecords(t.Context(), db, b)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := reader.Next(t.Context()); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec("UPDATE cjsi_checkpoint_records SET "+tc.sql+" WHERE start_byte>0", tc.value); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				record, evidence, err := reader.Next(t.Context())
				if !errors.Is(err, errCJSICheckpointWorkset) || record != (ingest.CodexRecord{}) ||
					evidence != (ingest.CodexCheckpointEvidence{}) {
					t.Fatal("damaged row leaked, advanced the cursor, or became EOF")
				}
			}
		})
	}
}

func TestCJSICheckpointDraftRecovery(t *testing.T) {
	const line = "{\"type\":\"synthetic\"}\n"
	t.Run("restart discovery rollback and retry", func(t *testing.T) {
		db, path, file, b := newCJSIWorksetFixture(t, line)
		_, step, err := runCJSIWorksetReplay(t, db, file, b, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`CREATE TRIGGER fixture_cleanup_failure BEFORE DELETE ON cjsi_checkpoint_records
			BEGIN SELECT RAISE(ABORT,'fixture cleanup failure'); END`); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		db = openCJSIWorksetFixture(t, path)
		owner := cjsiCheckpointDraftOwner{Round: b.Round, Identity: b.Identity, Version: b.Version}
		draft, found, err := findCJSICheckpointDraft(t.Context(), db, owner)
		if err != nil || !found || draft.binding != b || draft.completeEnd != step.Progress.CompleteEnd {
			t.Fatalf("exact owner could not recover draft watermark: %+v, %t, %v", draft, found, err)
		}
		for _, other := range []cjsiCheckpointDraftOwner{
			{Round: b.Round + 1, Identity: b.Identity, Version: b.Version},
			{Round: b.Round, Identity: "99:99", Version: b.Version},
			{Round: b.Round, Identity: b.Identity, Version: ingest.CodexCheckpointVersion{
				Generation: b.Version.Generation + 1, Parser: b.Version.Parser, MaxLineBytes: b.Version.MaxLineBytes}},
		} {
			if _, found, err := findCJSICheckpointDraft(t.Context(), db, other); err != nil || found {
				t.Fatal("another owner could see the draft")
			}
		}
		wrong := draft
		wrong.binding.Attempt++
		if err := discardCJSICheckpointDraft(t.Context(), db, wrong); !errors.Is(err, errCJSICheckpointWorkset) {
			t.Fatal("discard accepted wrong attempt")
		}
		if err := discardCJSICheckpointDraft(t.Context(), db, draft); err == nil {
			t.Fatal("cleanup failure did not reach caller")
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		db = openCJSIWorksetFixture(t, path)
		again, found, err := findCJSICheckpointDraft(t.Context(), db, owner)
		if err != nil || !found || again != draft {
			t.Fatal("cleanup failure lost the original draft watermark")
		}
		if _, err := openCJSICheckpointRecords(t.Context(), db, b); !errors.Is(err, errCJSICheckpointWorkset) {
			t.Fatal("failed cleanup made a draft readable")
		}
		if _, err := db.Exec("DROP TRIGGER fixture_cleanup_failure"); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := discardCJSICheckpointDraft(t.Context(), db, draft); err != nil {
				t.Fatal(err)
			}
		}
		if _, found, err := findCJSICheckpointDraft(t.Context(), db, owner); err != nil || found {
			t.Fatal("discarded draft still enumerated")
		}
		var state string
		var watermark, count int64
		if err := db.QueryRow(`SELECT state,complete_end,(SELECT count(*) FROM cjsi_checkpoint_records)
			FROM cjsi_checkpoint_attempts`).Scan(&state, &watermark, &count); err != nil ||
			state != "aborted" || watermark != draft.completeEnd || count != 0 {
			t.Fatal("cleanup did not preserve a row-free tombstone")
		}
		if _, err := beginCJSICheckpointWorkset(t.Context(), db, b); err == nil {
			t.Fatal("old attempt key can be reused after recovery")
		}
	})

	for _, sealFirst := range []bool{true, false} {
		name := "cleanup wins"
		if sealFirst {
			name = "seal wins"
		}
		t.Run(name, func(t *testing.T) {
			db, _, file, b := newCJSIWorksetFixture(t, line+line)
			sink, err := beginCJSICheckpointWorkset(t.Context(), db, b)
			if err != nil {
				t.Fatal(err)
			}
			replay, err := ingest.BeginCodexCheckpointReplay(t.Context(), file, file.Name(), b.CaptureEnd, b.Version, nil, sink)
			if err != nil {
				t.Fatal(err)
			}
			defer replay.Abort()
			owner := cjsiCheckpointDraftOwner{Round: b.Round, Identity: b.Identity, Version: b.Version}
			step, err := replay.Step(t.Context(), int64(len(line)))
			if err != nil || step.Done {
				t.Fatal("fixture did not stop after first row")
			}
			stale, found, err := findCJSICheckpointDraft(t.Context(), db, owner)
			if err != nil || !found {
				t.Fatal("first draft observation failed")
			}
			step, err = replay.Step(t.Context(), int64(len(line)))
			if err != nil || !step.Done {
				t.Fatal("fixture did not finish replay")
			}
			if err := discardCJSICheckpointDraft(t.Context(), db, stale); !errors.Is(err, errCJSICheckpointWorkset) {
				t.Fatal("stale cleanup won after watermark advanced")
			}
			draft, found, err := findCJSICheckpointDraft(t.Context(), db, owner)
			if err != nil || !found {
				t.Fatal("latest draft observation failed")
			}
			if sealFirst {
				if err := sink.Seal(t.Context(), step); err != nil {
					t.Fatal(err)
				}
				if err := discardCJSICheckpointDraft(t.Context(), db, draft); !errors.Is(err, errCJSICheckpointWorkset) {
					t.Fatal("cleanup overrode the seal")
				}
				if _, err := openCJSICheckpointRecords(t.Context(), db, b); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := discardCJSICheckpointDraft(t.Context(), db, draft); err != nil {
					t.Fatal(err)
				}
				if err := sink.Seal(t.Context(), step); !errors.Is(err, errCJSICheckpointWorkset) {
					t.Fatal("seal resurrected an aborted draft")
				}
			}
		})
	}
}
