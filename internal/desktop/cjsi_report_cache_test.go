package desktop

import (
	"bytes"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

func TestUnit2aReportCacheReuse(t *testing.T) {
	ctx := t.Context()
	st, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.DB.Exec(`CREATE TABLE cjsi_publication_head(singleton INTEGER PRIMARY KEY,core_epoch INTEGER,ticket INTEGER); INSERT INTO cjsi_publication_head VALUES(1,11,7);` + desktopStampDDL); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 1, 2, 10, 25, 0, 0, time.UTC)
	input := CJSIReportCacheInputs{CoreEpoch: 11, Ticket: 7, Database: "db", Publisher: "renderer", Schema: 37, UsageParser: 1, SessionParser: 1, Facts: [32]byte{1}, Sessions: [32]byte{2}, Protection: [32]byte{3}, Coverage: [32]byte{4}, Prices: [32]byte{5}, Routes: [32]byte{6}, Location: time.UTC, ValidUntil: start.Add(time.Hour)}
	build := func(now time.Time, in CJSIReportCacheInputs, prior []byte) (CJSIReportCache, error) {
		tx, err := st.DB.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		var cache CJSIReportCache
		err = usage.WithPublishedReports(ctx, tx, usage.ReportSelection{CoreEpoch: 11, Ticket: 7, Clock: now, Groups: []usage.ReportGroup{}}, func(r *usage.PublishedReports) error {
			cache, err = PrepareCJSIReportCache(ctx, r, in, prior)
			return err
		})
		return cache, err
	}
	cache, err := build(start, input, nil)
	if err != nil {
		t.Fatal(err)
	}
	original, err := cache.Encode()
	if err != nil {
		t.Fatal(err)
	}
	later := start.Add(time.Minute)
	cache, err = build(later, input, original)
	if err != nil {
		t.Fatal(err)
	}
	got, err := cache.Encode()
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("same window did not reuse original bytes: %v", err)
	}
	for _, alter := range []func(*CJSIReportCacheInputs){
		func(i *CJSIReportCacheInputs) { i.Facts[0]++ }, func(i *CJSIReportCacheInputs) { i.Sessions[0]++ },
		func(i *CJSIReportCacheInputs) { i.Coverage[0]++ }, func(i *CJSIReportCacheInputs) { i.Protection[0]++ },
		func(i *CJSIReportCacheInputs) { i.Prices[0]++ }, func(i *CJSIReportCacheInputs) { i.Routes[0]++ },
		func(i *CJSIReportCacheInputs) { i.ValidUntil = later.Add(time.Second) },
	} {
		in := input
		alter(&in)
		value, err := build(later, in, original)
		if err != nil {
			t.Fatal(err)
		}
		buf, err := value.Encode()
		if err != nil || bytes.Equal(buf, original) {
			t.Fatal("stale cache survived changed dependency")
		}
	}
	input.Ticket = 8
	if _, err := build(later, input, original); err == nil {
		t.Fatal("cache accepted a different report ticket")
	}
}
