package store

import (
	"database/sql"
	"errors"
	"maps"
	"reflect"
	"testing"
	"time"

	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

func checkEffectRows[T comparable](t *testing.T, before, remove, insert, after []T) {
	t.Helper()
	counts := func(rows []T) map[T]int {
		m := map[T]int{}
		for _, r := range rows {
			m[r]++
		}
		return m
	}
	want := counts(before)
	for _, r := range remove {
		if want[r] == 0 {
			t.Fatalf("removed absent row: %+v", r)
		}
		want[r]--
		if want[r] == 0 {
			delete(want, r)
		}
	}
	for _, r := range insert {
		want[r]++
	}
	if !maps.Equal(want, counts(after)) {
		t.Fatalf("projected rows differ from applied rows: projected=%+v actual=%+v", want, after)
	}
}

func TestCJSIOTelReportEffects(t *testing.T) {
	for _, tc := range []struct {
		name    string
		invalid string
		tokens  int64
	}{{"restore", "", 12}, {"invalidate", "'first','middle','last'", 0}, {"partial", "'first','last'", 4}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			_, st := openCJSI37TestStore(t, false)
			db := st.DB
			clock := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
			take := receiptProposal(t, 5, receiptHeader+receiptTurn+receiptMeter("request", 17), 0)
			release := receiptProposal(t, 7, receiptHeader, 0)
			f := receiptSQLiteFixture{thread: receiptThread, stamp: take.Stamp(), revision: 1, attributions: map[string]CJSIOTelAttribution{}}
			for i, id := range []string{"first", "middle", "last"} {
				at := clock.AddDate(0, -1, i).Format(time.RFC3339Nano)
				o := receiptObservation(id, receiptThread, at, int64(i+2))
				if _, err := st.ApplyTelemetryObservations(ctx, []TelemetryObservation{o}, nil, nil); err != nil {
					t.Fatal(err)
				}
				f.attributions["otel:"+id] = makeTestAttribution("p", "exact", "1", id)
			}
			ownershipExec(t, db, "INSERT INTO inventory_runs(inventory_id,epoch,captured_at,source_count,state) VALUES('synthetic',1,'2026-01-04T00:00:00Z',0,'complete')")
			if _, err := st.SettleTelemetry(ctx, clock, time.Minute); err != nil {
				t.Fatal(err)
			}
			ownershipExec(t, db, `INSERT INTO usage_runs(id,client,provider,multiplier,started_at) VALUES(41,'codex','p','1','2025-12-01T00:00:00Z'); INSERT INTO usage_run_bindings VALUES('otel:middle',41)`)
			check := func(p ledger.Proposal, withdraw func(*sql.Tx) error) {
				t.Helper()
				before := receiptSide(t, db, receiptThread)
				tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
				c, err := PrepareCJSIOTel(ctx, tx, receiptThread, p, f)
				if err != nil || c == nil {
					t.Fatalf("prepare: %v", err)
				}
				var n0, n1 int64
				if err := tx.QueryRow("SELECT total_changes()").Scan(&n0); err != nil {
					t.Fatal(err)
				}
				stale := f
				stale.revision++
				if _, err := c.ReportEffects(ctx, tx, stale, receiptThread, p.Stamp()); !errors.Is(err, ErrCJSIOTelPrecondition) {
					t.Fatalf("stale guard accepted: %v", err)
				}
				e, err := c.ReportEffects(ctx, tx, f, receiptThread, p.Stamp())
				if err != nil {
					t.Fatal(err)
				}
				if err := tx.QueryRow("SELECT total_changes()").Scan(&n1); err != nil || n1 != n0 {
					t.Fatal("effect reader wrote", err)
				}
				if err := tx.Rollback(); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(before, receiptSide(t, db, receiptThread)) {
					t.Fatal("effect read changed rows")
				}
				w, err := db.BeginTx(ctx, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer w.Rollback()
				if err := stageCJSIProposalRows(ctx, w, receiptThread, p, true); err != nil {
					t.Fatal(err)
				}
				if err := c.Apply(ctx, w, f, withdraw); err != nil {
					t.Fatal(err)
				}
				if err := w.Commit(); err != nil {
					t.Fatal(err)
				}
				after := receiptSide(t, db, receiptThread)
				checkEffectRows(t, before.Events, e.Remove.Events, e.Insert.Events, after.Events)
				checkEffectRows(t, before.Bindings, e.Remove.Bindings, e.Insert.Bindings, after.Bindings)
				checkEffectRows(t, before.Sessions, e.Remove.Sessions, e.Insert.Sessions, after.Sessions)
				if e.Restoring {
					var n int64
					for _, v := range e.Insert.Events {
						n += v.Input + v.Output
					}
					if n != tc.tokens {
						t.Fatalf("restored tokens %d want %d", n, tc.tokens)
					}
				}
			}
			check(take, nil)
			if tc.invalid != "" {
				ownershipExec(t, db, "UPDATE telemetry_observations SET state='invalid',reason='revalidated' WHERE observation_key IN ("+tc.invalid+")")
			}
			receiptPublish(t, db, receiptThread, "jsonl:request", "2026-01-02T10:00:00Z", 5, 10)
			f.stamp = release.Stamp()
			f.release = &CJSIOTelRelease{Thread: receiptThread, Number: 1, Support: take.Stamp(), Current: f.stamp, Removed: receiptPeriodDB(t, db, receiptThread).Requests, InventoryAt: clock}
			check(release, func(tx *sql.Tx) error {
				_, err := tx.Exec("DELETE FROM usage_events WHERE event_key='jsonl:request'")
				return err
			})
		})
	}
}

func TestCJSIOTelVerifyReportVacatedRejectsNilView(t *testing.T) {
	change := &CJSIOTelChange{restoring: true}
	if err := change.VerifyReportVacated(t.Context(), nil); !errors.Is(err, ErrCJSIOTelPrecondition) {
		t.Fatalf("nil report view: got %v, want %v", err, ErrCJSIOTelPrecondition)
	}
}
