package usage

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/activity"
	"github.com/kitdine/agent-deck/internal/store"
)

func coldPublicationFixture() ([]Event, []activity.Record) {
	var events []Event
	var records []activity.Record
	for i := 0; i < 101; i++ {
		events = append(events, Event{Key: fmt.Sprintf("event-%03d", i), Client: "codex", SessionID: "s", EventID: fmt.Sprint(i), EventAt: "2026-08-13T02:00:00+01:00", Model: "gpt-5", SourcePath: "source", SourceOffset: int64(i), TurnIndex: i, Tokens: map[string]int64{"input_tokens": int64(i)}})
		key := fmt.Sprintf("tool-%03d", i)
		start := activity.Record{Key: key, Client: "codex", SessionID: "s", Model: "gpt-5", Tool: "Read", StartedAt: "2026-08-13T01:00:00Z", SourcePath: "source", SourceOffset: int64(i), TurnIndex: i, ToolKind: "read", CommandRead: true, Files: []activity.File{{PathDigest: "digest", BaseName: "old"}, {PathDigest: "digest", BaseName: "new", Wrote: true}}}
		completion := activity.Record{Key: key, CompletedAt: "2026-08-13T01:00:03Z", Status: "completed", SourcePath: "source"}
		if i == 1 {
			completion.CompletedAt = "2026-08-12T01:00:00Z" // negative duration -> NULL
		}
		if i == 2 {
			completion.CompletedAt = "invalid" // unknown duration -> NULL
		}
		records = append(records, completion, start, completion) // early completion is ignored
	}
	duplicate := events[0]
	duplicate.Tokens = map[string]int64{"input_tokens": 999}
	events = append(events, duplicate)
	duplicate.SourceOffset = 200 // offset-only update does not count as a change
	events = append(events, duplicate)
	duplicate.Model = "gpt-5.4"
	events = append(events, duplicate)
	records = append(records,
		activity.Record{Key: "tool-000", Client: "codex", SessionID: "s", Tool: "Write", StartedAt: "2026-08-13T02:00:00Z", SourcePath: "source", ToolKind: "edit"},
		activity.Record{Key: "tool-000", CompletedAt: "2026-08-13T02:00:01Z", Status: "failed", SourcePath: "source"},
		activity.Record{Key: "unknown", CompletedAt: "2026-08-13T02:00:01Z", Status: "completed", SourcePath: "source"})
	return events, records
}

func TestColdPublicationMatchesSequentialUpserts(t *testing.T) {
	ctx := context.Background()
	events, records := coldPublicationFixture()
	var snapshots []map[string][][]any
	for _, cold := range []bool{false, true} {
		db, err := store.Open(ctx, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		tx, err := db.DB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		imported, updated := 0, 0
		if cold {
			handled, added, changed, err := publishColdSource(ctx, tx, "source", events, records)
			if err != nil || !handled {
				t.Fatalf("cold handled=%v: %v", handled, err)
			}
			imported, updated = added, changed
		} else {
			p, err := prepareIngestion(ctx, tx)
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			for _, e := range events {
				added, changed, err := upsertTx(ctx, p, e)
				if err != nil {
					t.Fatal(err)
				}
				if added {
					imported++
				} else if changed {
					updated++
				}
			}
			for _, r := range records {
				if err := upsertToolActivityTx(ctx, p, r); err != nil {
					t.Fatal(err)
				}
			}
		}
		if imported != 101 || updated != 2 {
			t.Fatalf("cold=%v counts=%d/%d", cold, imported, updated)
		}
		snapshots = append(snapshots, coldPublicationRows(t, tx))
	}
	if !reflect.DeepEqual(snapshots[0], snapshots[1]) {
		t.Fatal("cold final rows differ from sequential reference")
	}
}

func coldPublicationRows(t *testing.T, tx *sql.Tx) map[string][][]any {
	t.Helper()
	result := make(map[string][][]any)
	for _, table := range []string{"usage_events", "usage_tool_calls", "usage_tool_files"} {
		rows, err := tx.Query("SELECT * FROM " + table + " ORDER BY 1,2")
		if err != nil {
			t.Fatal(err)
		}
		columns, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for i := range values {
				pointers[i] = &values[i]
			}
			if err := rows.Scan(pointers...); err != nil {
				t.Fatal(err)
			}
			result[table] = append(result[table], values)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
	}
	return result
}

func TestColdPublicationFallsBackBeforeWritingExistingKeys(t *testing.T) {
	ctx := context.Background()
	for _, existing := range []string{"event", "tool", "completion"} {
		t.Run(existing, func(t *testing.T) {
			db, err := store.Open(ctx, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			tx, err := db.DB.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			events, records := coldPublicationFixture()
			if existing == "event" {
				_, _, err = upsertTx(ctx, tx, events[0])
			} else {
				err = upsertToolActivityTx(ctx, tx, records[1])
				if existing == "completion" {
					records = records[:1]
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			before := coldPublicationRows(t, tx)
			if handled, _, _, err := publishColdSource(ctx, tx, "source", events, records); err != nil || handled {
				t.Fatalf("existing key accepted=%v: %v", handled, err)
			}
			if !reflect.DeepEqual(before, coldPublicationRows(t, tx)) {
				t.Fatal("fallback wrote a partial batch")
			}
		})
	}
}

func TestColdPublicationFailureRollsBackAllBatches(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.DB.Exec(`CREATE TRIGGER reject_last_file BEFORE INSERT ON usage_tool_files WHEN NEW.activity_key='tool-100' BEGIN SELECT RAISE(ABORT,'reject'); END`); err != nil {
		t.Fatal(err)
	}
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	events, records := coldPublicationFixture()
	if handled, _, _, err := publishColdSource(ctx, tx, "source", events, records); !handled || err == nil {
		t.Fatalf("injected failure ignored: %v, %v", handled, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.DB.QueryRow(`SELECT (SELECT count(*) FROM usage_events)+(SELECT count(*) FROM usage_tool_calls)+(SELECT count(*) FROM usage_tool_files)+(SELECT count(*) FROM usage_source_files)`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial publication count=%d: %v", count, err)
	}
}

func TestColdSQLBatchBoundsStringBytes(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec("CREATE TEMP TABLE batch_values(value TEXT)"); err != nil {
		t.Fatal(err)
	}
	b := coldSQLBatch{tx: tx, columns: 1, prefix: "INSERT INTO batch_values VALUES"}
	for _, size := range []int{700_000, 700_000, 2 << 20} {
		if err := b.add(ctx, strings.Repeat("x", size)); err != nil || b.rows != 1 {
			t.Fatalf("byte bound not honored: rows=%d, %v", b.rows, err)
		}
	}
	if err := b.flush(ctx); err != nil {
		t.Fatal(err)
	}
	var count, total int
	if err := tx.QueryRow("SELECT count(*),sum(length(value)) FROM batch_values").Scan(&count, &total); err != nil || count != 3 || total != 1_400_000+(2<<20) {
		t.Fatalf("oversized value lost: %d/%d, %v", count, total, err)
	}
}
