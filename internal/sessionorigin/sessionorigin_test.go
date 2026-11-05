package sessionorigin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
)

type fakeStore struct {
	preference    store.OriginPreference
	preferenceErr error
	record        store.OriginOverride
	recordFound   bool
	recordErr     error
	applyErr      error
	applyReceipt  store.OriginReceipt
	applied       []store.OriginMutation
	receipt       store.OriginReceipt
	receiptFound  bool
}

func (f *fakeStore) OriginPreference(context.Context) (store.OriginPreference, error) {
	return f.preference, f.preferenceErr
}
func (f *fakeStore) OriginOverrideRecord(context.Context, string, string) (store.OriginOverride, bool, error) {
	return f.record, f.recordFound, f.recordErr
}
func (f *fakeStore) OriginOverrides(context.Context, int, int) ([]store.OriginOverride, int, error) {
	return nil, 0, nil
}
func (f *fakeStore) OriginReceipt(context.Context, string) (store.OriginReceipt, bool, error) {
	return f.receipt, f.receiptFound, nil
}
func (f *fakeStore) ApplyOriginMutation(_ context.Context, m store.OriginMutation) (store.OriginReceipt, error) {
	f.applied = append(f.applied, m)
	return f.applyReceipt, f.applyErr
}

func TestDefaultScopeConfirmationContract(t *testing.T) {
	ctx := context.Background()
	absent := Service{Store: &fakeStore{}}.DefaultScope(ctx)
	if !absent.Confirmed || *absent.Value != ScopeWork || *absent.Revision != 0 || absent.Reason != nil {
		t.Fatalf("missing configuration = %+v; want confirmed work/0", absent)
	}
	present := Service{Store: &fakeStore{preference: store.OriginPreference{Scope: ScopeAll, Revision: 3, Present: true}}}.DefaultScope(ctx)
	if !present.Confirmed || *present.Value != ScopeAll || *present.Revision != 3 {
		t.Fatalf("stored default = %+v", present)
	}
	failed := Service{Store: &fakeStore{preferenceErr: errors.New("io")}}.DefaultScope(ctx)
	if failed.Confirmed || failed.Value != nil || failed.Revision != nil || failed.Reason == nil || *failed.Reason != CodeReadFailed {
		t.Fatalf("read failure = %+v; want unconfirmed null", failed)
	}
}

func TestClearReadCASNeverTreatsReadFailureAsNoOverride(t *testing.T) {
	fake := &fakeStore{recordErr: errors.New("io")}
	_, err := Service{Store: fake}.ClearOverride(context.Background(), OverrideClearRequest{Client: "codex", SessionID: "s"})
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != CodeReadFailed || len(fake.applied) != 0 {
		t.Fatalf("err=%v applied=%d; want read failure and no write", err, len(fake.applied))
	}
	if _, err = (Service{Store: fake}).ClearOverride(context.Background(), OverrideClearRequest{SessionID: "s"}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("clear without client err = %v", err)
	}
}

func TestSetWithoutVerifiedBindingIsIdentityUnavailable(t *testing.T) {
	fake := &fakeStore{}
	_, err := Service{Store: fake}.SetOverride(context.Background(), OverrideSetRequest{Client: "claude", SessionID: "s", Value: ClassAuxiliary})
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != CodeIdentityUnavailable || len(fake.applied) != 0 {
		t.Fatalf("err=%v applied=%d", err, len(fake.applied))
	}
}

func TestWriteErrorMapping(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		err  error
		code string
	}{
		{&store.OriginConflictError{Expected: 1, Current: 2}, CodeConflict},
		{store.ErrOriginRequestMismatch, CodeInvalidRequest},
		{&store.OriginCommitUnknownError{Err: errors.New("lost")}, CodeWriteUnknown},
		{errors.New("disk full"), CodeWriteFailed},
	} {
		fake := &fakeStore{applyErr: test.err}
		_, err := Service{Store: fake}.SetDefaultScope(ctx, ScopeRequest{RequestID: "req-1", Value: ScopeAll})
		var typed *Error
		if !errors.As(err, &typed) || typed.Code != test.code || typed.RequestID != "req-1" {
			t.Fatalf("%v mapped to %v", test.err, err)
		}
		if test.code == CodeConflict && (typed.CurrentRevision == nil || *typed.CurrentRevision != 2) {
			t.Fatal("conflict lost the current revision")
		}
	}
	fake := &fakeStore{applyErr: store.ErrOriginSecureFilesFailed, applyReceipt: store.OriginReceipt{RequestID: "req-2", Kind: store.OriginMutationScopeSet, Outcome: OutcomeSaved, Revision: 1}}
	receipt, err := Service{Store: fake}.SetDefaultScope(ctx, ScopeRequest{RequestID: "req-2", Value: ScopeAll})
	if err != nil || receipt.Outcome != OutcomeSaved || len(receipt.Warnings) != 1 || receipt.Warnings[0] != WarningPermissions {
		t.Fatalf("post-commit permission failure = %+v, %v; want saved + warning", receipt, err)
	}
}

func TestRequestIDValidationAndGeneration(t *testing.T) {
	fake := &fakeStore{}
	service := Service{Store: fake, NewRequestID: func() (string, error) { return "req-generated", nil }}
	if _, err := service.SetDefaultScope(context.Background(), ScopeRequest{Value: ScopeWork}); err != nil {
		t.Fatal(err)
	}
	if fake.applied[0].RequestID != "req-generated" || fake.applied[0].PayloadHash == "" {
		t.Fatalf("generated request = %+v", fake.applied[0])
	}
	if _, err := service.SetDefaultScope(context.Background(), ScopeRequest{RequestID: "../etc", Value: ScopeWork}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("path-like request id err = %v", err)
	}
	if id, err := NewRequestID(); err != nil || !ValidRequestID(id) {
		t.Fatalf("NewRequestID = %q, %v", id, err)
	}
}

func TestPayloadHashCoversExplicitExpectedRevisionOnly(t *testing.T) {
	m := store.OriginMutation{Kind: store.OriginMutationOverrideClear, Client: "codex", SessionID: "s"}
	four, five := int64(4), int64(5)
	if PayloadHash(m, &four) == PayloadHash(m, &five) {
		t.Fatal("explicit expected revision is not part of the payload")
	}
	m.ExpectedRevision = 4
	implicit := PayloadHash(m, nil)
	m.ExpectedRevision = 5
	if PayloadHash(m, nil) != implicit {
		t.Fatal("implicit read-CAS revision changed the payload")
	}
	other := m
	other.SessionID = "t"
	if PayloadHash(other, nil) == implicit {
		t.Fatal("target is not part of the payload")
	}
}

func TestListOverridesBounds(t *testing.T) {
	service := Service{Store: &fakeStore{}}
	page, err := service.ListOverrides(context.Background(), 1, 0)
	if err != nil || page.Limit != DefaultOverrideLimit || page.Items == nil {
		t.Fatalf("default page = %+v, %v", page, err)
	}
	for _, bad := range [][2]int{{0, 20}, {1, 201}, {1, -1}} {
		if _, err := service.ListOverrides(context.Background(), bad[0], bad[1]); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("page=%d limit=%d err=%v", bad[0], bad[1], err)
		}
	}
}

func TestReasonCodeEnumIsClosed(t *testing.T) {
	codes := ReasonCodes()
	if len(codes) != 7 {
		t.Fatalf("reason codes = %v", codes)
	}
	for _, code := range codes {
		if _, ok := AutomaticForReason(code); !ok {
			t.Fatalf("%s has no automatic class", code)
		}
	}
	if class, _ := AutomaticForReason(ReasonMaintenanceOrigin); class != ClassAuxiliary {
		t.Fatal("maintenance_origin must be the only auxiliary reason")
	}
	if _, ok := AutomaticForReason("origin_missing"); ok {
		t.Fatal("menubar-internal key accepted as a reason code")
	}
}

// Core-only clear chain through the service against a real isolated store.
func TestServiceClearAgainstCoreOnly(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "state")
	core, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer core.Close()
	service := Service{Store: core, Now: func() time.Time { return time.Unix(10, 0) }}
	if _, err = service.SetOverride(ctx, OverrideSetRequest{RequestID: "set-1", Client: "codex", SessionID: "archived-1", Value: ClassAuxiliary, Binding: &store.OriginBinding{Digest: "birth"}}); err != nil {
		t.Fatal(err)
	}
	result, err := service.ClearOverride(ctx, OverrideClearRequest{RequestID: "clear-1", Client: "codex", SessionID: "archived-1"})
	if err != nil || result.Outcome != OutcomeSaved || result.Revision != 2 || result.Override != nil ||
		result.Automatic != nil || result.Effective != nil || result.ClassificationAvailable || result.ClassificationReason != ReasonNotQueried ||
		result.Target == nil || result.Target.Client != "codex" {
		t.Fatalf("clear = %+v, %v", result, err)
	}
	// Same request, implicit read-CAS: the revision moved, but the receipt replays.
	again, err := service.ClearOverride(ctx, OverrideClearRequest{RequestID: "clear-1", Client: "codex", SessionID: "archived-1"})
	if err != nil || !again.Replayed || again.Revision != 2 {
		t.Fatalf("implicit retry = %+v, %v", again, err)
	}
	stale := int64(1)
	if _, err = service.ClearOverride(ctx, OverrideClearRequest{RequestID: "clear-2", Client: "codex", SessionID: "archived-1", ExpectedRevision: &stale}); err == nil {
		t.Fatal("stale explicit clear succeeded")
	}
	known, err := service.ReadReceipt(ctx, "clear-1")
	if err != nil || !known.Known || known.Receipt.Outcome != OutcomeSaved || known.Receipt.Revision != 2 {
		t.Fatalf("read-back = %+v, %v", known, err)
	}
	unknown, err := service.ReadReceipt(ctx, "never-sent")
	if err != nil || unknown.Known || unknown.Receipt.Outcome != OutcomeUnknown {
		t.Fatalf("unknown read-back = %+v, %v", unknown, err)
	}
	page, err := service.ListOverrides(ctx, 1, 0)
	if err != nil || page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("tombstone listed: %+v, %v", page, err)
	}
	// Clear is core-only: no session index was created or opened.
	if _, err = os.Stat(filepath.Join(root, "sessions.sqlite3")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("clear touched the session index: %v", err)
	}
	scope := service.DefaultScope(ctx)
	if !scope.Confirmed || *scope.Value != ScopeWork || *scope.Revision != 0 {
		t.Fatalf("default = %+v", scope)
	}
}
