package usage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

func TestCJSIProjectedCodec(t *testing.T) {
	price := modelPrice{Provider: "openai", Prices: map[string]string{"input": "1000000"}}
	v1 := projectedPriceVersion(8, 11, price, true)
	if v1 != "ec9ad31774f5b2d055bfc40cd79ab797d7d43e06b5827b8a828b7b750022fd89" {
		t.Fatalf("legacy version %s", v1)
	}
	encode := func(rows []projectedEvent, binding string) []byte {
		t.Helper()
		payload, err := encodeProjectedEvents(rows, binding)
		if err != nil {
			t.Fatal(err)
		}
		return payload
	}
	if string(encode([]projectedEvent{}, "")) != `{"events":[],"checksum":"4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945"}` {
		t.Fatal("legacy empty payload changed")
	}
	for _, client := range []string{"claude", "codex"} {
		p := store.UsagePartition{Client: client, Model: "", Day: "2026-01-02", Revision: 4, ActiveRevision: 4, Version: v1}
		row := projectedEvent{Event: Event{Key: "r", Client: client, EventAt: "2026-01-02T01:00:00Z", Tokens: map[string]int64{"input_tokens": 7}}}
		rows := []projectedEvent{row}
		data, err := json.Marshal(rows)
		if err != nil {
			t.Fatal(err)
		}
		checksum := sha256.Sum256(data)
		p.Payload, err = json.Marshal(projectedEventPartition{Events: rows, Checksum: hex.EncodeToString(checksum[:])})
		if err != nil || !reflect.DeepEqual(p.Payload, encode(rows, "")) || !reflect.DeepEqual(cachedProjectedEvents(p, v1, ""), rows) {
			t.Fatal("legacy positive", client, err)
		}
		for _, binding := range []string{"", "v2:fixture"} {
			version := v1
			if binding != "" {
				version = binding
			}
			p.Version = version
			p.Payload = encode(nil, binding)
			if cachedProjectedEvents(p, version, binding) != nil {
				t.Fatal("null rows became available")
			}
			p.Payload = encode([]projectedEvent{}, binding)
			if got := cachedProjectedEvents(p, version, binding); got == nil || len(got) != 0 {
				t.Fatal("lost explicit empty")
			}
		}
	}
}

func TestCJSIProjectedEventPartition(t *testing.T) {
	ctx := t.Context()
	db := newStampFixture(t)
	err := stampFixtureTransaction(t, db, func(tx *sql.Tx) error {
		day := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
		p := store.UsagePartition{Client: "codex", Model: "M", Day: day.Format(time.DateOnly), Revision: 5, ActiveRevision: 5}
		price := func(unit string) *readPriceResolver {
			return &readPriceResolver{current: day.Add(time.Hour), byModel: map[string][]readPriceRow{
				"openai\x00" + readPriceModelKey("codex", "M"): {{price: modelPrice{Prices: map[string]string{"input": unit}}}},
			}}
		}
		view := cjsiProjectionView{head: intervalStampHead{11, 1}, publisher: "fixture-publisher/1", parser: ledger.Parser, priceRevision: 3, prices: price("1000000")}
		event := storedEvent{
			Event: Event{
				Key: "R", Client: p.Client, SessionID: "A", Model: p.Model, EventAt: day.Format(time.RFC3339Nano),
				SourcePath: "private-fixture", SourceOffset: 8, Tokens: map[string]int64{"input_tokens": 10},
			},
			runID: sql.NullInt64{Int64: 1, Valid: true}, runExact: sql.NullInt64{Int64: 1, Valid: true},
			runProvider: sql.NullString{String: "Q", Valid: true}, runMultiplier: sql.NullString{String: "2", Valid: true},
		}
		facts := []storedEvent{event}
		reads := 0
		reader := func(_ context.Context, borrowed *sql.Tx, client, model string, from, to time.Time) ([]storedEvent, error) {
			if borrowed != tx || client != "codex" || model != "M" || !from.Equal(day) || !to.Equal(day.AddDate(0, 0, 1)) {
				t.Fatal("wrong admitted ledger scope")
			}
			reads++
			return facts, nil
		}
		load := func(v cjsiProjectionView, part store.UsagePartition, wantReads int) cjsiRequestPartition {
			t.Helper()
			before := reads
			result, err := readCJSIRequestPartition(ctx, tx, v, part, reader)
			if err != nil || reads-before != wantReads || result.events == nil {
				t.Fatalf("reads=%d want=%d err=%v", reads-before, wantReads, err)
			}
			return result
		}
		cold := load(view, p, 1)
		p.Version, p.Payload = cold.version, cold.payload
		same := func(r cjsiRequestPartition) {
			t.Helper()
			if !reflect.DeepEqual(r, cold) {
				t.Fatal("reconstruction differs", r)
			}
		}
		same(load(view, p, 0))

		// R8-F2: non-canonical JSON with valid checksum (extra warnings:[] or
		// unknown JSON field) MUST miss and rebuild canonical bytes from ledger.
		withEmptyWarnings := bytes.Replace(cold.payload, []byte(`"unpriced_components":[]`), []byte(`"unpriced_components":[],"warnings":[]`), 1)
		if bytes.Equal(withEmptyWarnings, cold.payload) {
			t.Fatal("failed to inject warnings:[]")
		}
		nonCanon1 := p
		nonCanon1.Payload = withEmptyWarnings
		same(load(view, nonCanon1, 1))

		withUnknownField := bytes.Replace(cold.payload, []byte(`"events":[`), []byte(`"extra_unknown":1,"events":[`), 1)
		if bytes.Equal(withUnknownField, cold.payload) {
			t.Fatal("failed to inject unknown JSON field")
		}
		nonCanon2 := p
		nonCanon2.Payload = withUnknownField
		same(load(view, nonCanon2, 1))

		// Corrupt, stale, mismatched model, or v1 payload mislabeled v2 all trigger rebuild.
		e := cold.events[0]
		for _, mutate := range []func(*store.UsagePartition){
			func(x *store.UsagePartition) { x.Dirty = true },
			func(x *store.UsagePartition) { x.ActiveRevision-- },
			func(x *store.UsagePartition) { x.Version = "old-version" },
			func(x *store.UsagePartition) { x.Payload = []byte("{") },
			func(x *store.UsagePartition) { x.Payload = []byte(`{"events":[],"checksum":"wrong"}`) },
			func(x *store.UsagePartition) { x.Payload, _ = encodeProjectedEvents(cold.events, "") },
			func(x *store.UsagePartition) {
				bad := e
				bad.Event.Model = "N"
				x.Payload, _ = encodeProjectedEvents([]projectedEvent{bad}, cold.version)
			},
		} {
			bad := p
			mutate(&bad)
			same(load(view, bad, 1))
		}

		// Interval add & delete-last-interval invalidate the v2 version key.
		publish := func(next int64, at time.Time, model string, remove bool) {
			t.Helper()
			view.head = stampFixtureHead(t, tx, view.head.ticket, next)
			if err := writeIntervalStamps(ctx, tx, view.head, stampFixturePlan(t, at, at, model, remove)); err != nil {
				t.Fatal(err)
			}
		}
		publish(2, day.AddDate(0, 0, 1), "M", false)
		same(load(view, p, 0))
		publish(3, day, "M", false)
		added := load(view, p, 1)
		if added.version == cold.version || !reflect.DeepEqual(added.events, cold.events) {
			t.Fatal("interval failed to invalidate or mutated request list")
		}
		p.Version, p.Payload = added.version, added.payload
		publish(4, day, "M", true)
		deleted := load(view, p, 1)
		if deleted.version == added.version {
			t.Fatal("deleting last interval resurrected stale version")
		}
		if _, err := readCJSIRequestPartition(ctx, tx, view, p, func(context.Context, *sql.Tx, string, string, time.Time, time.Time) ([]storedEvent, error) {
			return nil, errors.New("fixture failed")
		}); err == nil {
			t.Fatal("failed reader did not propagate error")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
