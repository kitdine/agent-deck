package desktop

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/store"
)

func projectionSQL(t *testing.T, core *store.Store, query string, args ...any) {
	t.Helper()
	if _, err := core.DB.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

func projectionSeed(t *testing.T, core *store.Store, at time.Time) {
	t.Helper()
	recordOfficial(t, core, "codex", "claude")
	saveQuotaSettings(t, core, func(s *quota.Settings) { s.ProbeEnabled = true })
	date := at.Format(time.RFC3339Nano)
	projectionSQL(t, core, `UPDATE quota_cycle SET seq=4,revalidate_seq=1`)
	projectionSQL(t, core, `INSERT INTO quota_envelopes_v2(client,source,observed_at,plan,cycle_seq,account_id,reset_allowance_reason) VALUES('codex','codex',?,'pro',4,'private-account-sentinel','not_reported')`, date)
	recordQuotaWindow(t, core, quota.Observation{Client: quota.ClientClaude, Source: quota.SourceClaudeStatusLine, WindowKey: "five_hour", WindowMinutes: 300, ObservedAt: at, UsedPercent: 32, ResetsAt: at.Add(time.Hour)})
}

func seedClaudeProjectionIdentity(t *testing.T, core *store.Store, at time.Time) {
	t.Helper()
	projectionSQL(t, core, `INSERT INTO quota_identities(client,attempt_cycle_seq,attempted_at,success_cycle_seq,success_state,success_observed_at,email_masked,org_kind,org_name,plan_sources_json)
	VALUES('claude',4,?,4,'identified',?,'a***e@example.test','organization','Fixture organization','[{},{"value":"pro"},{}]')`, at.Format(time.RFC3339Nano), at.Format(time.RFC3339Nano))
}

func TestSubscriptionProjectionPinnedBeforeClock(t *testing.T) {
	core := openSubscriptionStore(t)
	ctx := context.Background()
	at := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	projectionSeed(t, core, at)
	seedClaudeProjectionIdentity(t, core, at)
	// This clock runs after the transaction's first read. Commit invalidation
	// and a gate change before the projector reads either client's facts.
	s := Service{Now: func() time.Time {
		projectionSQL(t, core, `BEGIN IMMEDIATE;
			DELETE FROM quota_windows_v2 WHERE client='claude';
			INSERT INTO quota_envelopes_v2(client,context_generation,invalidated_at,invalidated_reason,capture_gate) VALUES('claude',1,'2026-11-18T12:00:00Z','context_changed','blocked');
			UPDATE quota_cycle SET revalidate_seq=seq;
			UPDATE settings SET value='off' WHERE key='quota.probe'; COMMIT`)
		return at
	}}
	before, err := s.BuildSubscription(ctx, core, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	c, x := subscriptionFor(t, before, "claude"), subscriptionFor(t, before, "codex")
	if !before.Reading || before.Projection == nil || c.QuotaContext != nil || len(c.Windows) != 1 || c.Identity == nil || c.Identity.State != "identified" || x.Identity != nil || x.PlanDisplay == nil || x.PlanDisplay.State != quota.PlanRawOnly {
		t.Fatal("mixed snapshots", before)
	}
	after, err := (Service{}).BuildSubscription(ctx, core, at)
	if err != nil || after.Reading || subscriptionFor(t, after, "codex").Identity != nil || after.Projection == nil || after.Projection.Revision <= before.Projection.Revision || after.Projection.CoreEpoch != before.Projection.CoreEpoch || after.Projection.Boot != before.Projection.Boot || after.Projection.Mono < before.Projection.Mono {
		t.Fatal("incorrect next projection", after, err)
	}
	// Only the ordering key is machine-specific. Freeze exact wire output
	// for the Swift consumer with a deterministic synthetic key.
	before.Projection = &SubscriptionProjection{CoreEpoch: 11, Revision: 12, Boot: "fixture-boot", Mono: 13}
	got, err := json.MarshalIndent(before, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "subscription-identity-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(want)) != string(got) {
		t.Fatalf("fixture differs:\n%s", got)
	}
	for _, forbidden := range []string{"private-account-sentinel", "private-key-sentinel", "context_generation", "association_cycle_seq", "baseline_"} {
		if strings.Contains(string(got), forbidden) {
			t.Fatal("private value or field", forbidden)
		}
	}
}

func TestSubscriptionProjectionExpiryAndFailure(t *testing.T) {
	core := openSubscriptionStore(t)
	at := time.Now().UTC().Truncate(time.Second)
	projectionSeed(t, core, at)
	seedClaudeProjectionIdentity(t, core, at)
	read := func(now time.Time) SubscriptionClient {
		t.Helper()
		sub, err := (Service{}).BuildSubscription(context.Background(), core, now)
		if err != nil {
			t.Fatal(err)
		}
		return subscriptionFor(t, sub, "claude")
	}
	until := at.Add(10 * time.Minute)
	for _, tc := range []struct {
		now     time.Time
		current bool
	}{{at.Add(-1), false}, {at, true}, {until, true}, {until.Add(1), false}} {
		i := read(tc.now).Identity
		if (i.Freshness == "current") != tc.current || (i.CurrentUntil != nil) != tc.current {
			t.Fatal(i)
		}
		if tc.current && *i.CurrentUntil != until.Format(time.RFC3339Nano) {
			t.Fatal("wrong expiry")
		}
	}
	projectionSQL(t, core, `UPDATE quota_cycle SET revalidate_seq=4`)
	if i := read(at).Identity; i.Freshness != "last_read" || i.CurrentUntil != nil {
		t.Fatal("revalidation floor accepted an old success", i)
	}
	projectionSQL(t, core, `UPDATE quota_cycle SET revalidate_seq=1`)
	projectionSQL(t, core, `UPDATE quota_identities SET attempt_cycle_seq=5,failure_cycle_seq=5,attempt_failure='probe_failed',attempted_at=?`, until.Format(time.RFC3339Nano))
	c := read(until)
	i := c.Identity
	if i.State != "unknown" || *i.UnknownReason != "probe_failed" || i.Freshness != "last_read" || i.CurrentUntil != nil || i.EmailMasked == nil || i.QuotaAssociation != nil || *i.FailureAt != until.Format(time.RFC3339Nano) {
		t.Fatal("wrong last-read state", i)
	}
}

func TestSubscriptionServingOverlay(t *testing.T) {
	s, core, _ := newServingFixture(t)
	now := s.now()
	projectionSeed(t, core, now)
	ctx := context.Background()
	req := Request{WireVersion: 1, RecentLimit: DefaultRecentLimit}
	bootstrap, err := s.ReadOrBootstrapServingSnapshot(ctx, req)
	if err != nil || !bootstrap.Payload.Data.Subscription.Available || bootstrap.Payload.Data.Subscription.Projection == nil {
		t.Fatal("missing bootstrap subscription", err)
	}
	pub, err := s.PublishServingSnapshot(ctx, req, true)
	if err != nil {
		t.Fatal(err)
	}
	projectionSQL(t, core, `DELETE FROM quota_windows_v2 WHERE client='claude';
	INSERT INTO quota_envelopes_v2(client,context_generation,invalidated_at,invalidated_reason,capture_gate,failure,failure_detail,failure_observed_at)
	VALUES('claude',2,'2026-10-12T12:00:00Z','context_changed','blocked','parse_failed','shape_invalid','2026-10-12T12:01:00Z')`)
	for _, read := range []func() (ServingSnapshot, error){func() (ServingSnapshot, error) { return s.ReadServingSnapshot(ctx) }, func() (ServingSnapshot, error) { return s.ReadOrBootstrapServingSnapshot(ctx, req) }} {
		recovered, err := read()
		if err != nil {
			t.Fatal(err)
		}
		c := subscriptionFor(t, recovered.Payload.Data.Subscription, "claude")
		if recovered.Header.PublicationID != pub.Header.PublicationID || recovered.Header.Freshness != "stale" || c.QuotaContext == nil || c.QuotaContext.Reason != "context_changed" || c.FailureDetail == nil || *c.FailureDetail != "shape_invalid" || len(c.Windows) != 0 {
			t.Fatal("returned cached subscription", c)
		}
	}
	projectionSQL(t, core, `UPDATE quota_envelopes_v2 SET confirmed_generation=context_generation,invalidated_at='',invalidated_reason='',capture_gate='open',failure='',failure_detail='',source='claude_prose',observed_at='2026-10-12T12:03:00Z' WHERE client='claude';
	INSERT INTO quota_windows_v2(client,window_key,source,observed_at,used_percent,resets_at,context_generation) VALUES('claude','five_hour','claude_prose','2026-10-12T12:03:00Z',11,'2026-10-12T13:00:00Z',2)`)
	reconfirmed, err := s.ReadServingSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c := subscriptionFor(t, reconfirmed.Payload.Data.Subscription, "claude")
	if c.QuotaContext != nil || c.FailureDetail != nil || len(c.Windows) != 1 || c.Windows[0].UsedPercent != 11 || c.AttributionConfirmed {
		t.Fatal(c)
	}
	projectionSQL(t, core, `UPDATE settings SET value='broken' WHERE key='quota.probe'`)
	invalid, err := s.ReadOrBootstrapServingSnapshot(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if invalid.Payload.Data.Subscription.Available || len(invalid.Payload.Data.Subscription.Clients) != 0 || invalid.Payload.Data.Subscription.Projection != nil {
		t.Fatal("reused cached proof on read failure")
	}
	// All other payload fields retain their cached values.
	invalid.Payload.Data.Subscription = pub.Payload.Data.Subscription
	a, _ := json.Marshal(invalid.Payload)
	b, _ := json.Marshal(pub.Payload)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("failed projection changed the rest of the envelope")
	}
}

func TestSubscriptionLegacyProjection(t *testing.T) {
	for _, version := range []int{27, 28, 29, 30, 35} {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			s, core, _ := newServingFixture(t)
			at := s.now()
			projectionSeed(t, core, at)
			projectionSQL(t, core, `ALTER TABLE quota_envelopes_v2 RENAME TO quota_envelopes;
				ALTER TABLE quota_windows_v2 RENAME TO quota_windows;
				INSERT INTO quota_envelopes(client,failure,failure_at) VALUES('claude','parse_failed','2026-10-12T10:00:00Z')`)
			if version < 30 {
				projectionSQL(t, core, "ALTER TABLE quota_envelopes DROP COLUMN failure_observed_at")
			}
			if version < 28 {
				projectionSQL(t, core, "ALTER TABLE quota_envelopes DROP COLUMN backoff_until")
			}
			projectionSQL(t, core, "UPDATE schema_metadata SET version=?", version)
			ro, err := store.OpenReadOnly(context.Background(), s.StateRoot)
			if err != nil {
				t.Fatal(err)
			}
			defer ro.Close()
			sub, err := s.BuildSubscription(context.Background(), ro, at)
			if err != nil {
				t.Fatal(err)
			}
			c, x := subscriptionFor(t, sub, "claude"), subscriptionFor(t, sub, "codex")
			if c.ObservedAt == nil || *c.ObservedAt != "2026-10-12T10:00:00Z" {
				t.Fatalf("schema %d should display legacy failure_at, got %v", version, c.ObservedAt)
			}
			if c.Identity != nil || c.QuotaContext != nil || c.FailureDetail != nil || !reasonIs(c.Failure, quota.ReasonParseFailed) || len(c.Windows) != 0 || x.Plan == nil || *x.Plan != "pro" || x.Identity != nil || x.PlanDisplay == nil || x.PlanDisplay.Name == nil || *x.PlanDisplay.Name != "pro" {
				t.Fatal("legacy projection or unknown failure order", version, c, x)
			}
		})
	}
}
