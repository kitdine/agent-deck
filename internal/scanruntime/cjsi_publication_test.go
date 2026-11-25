package scanruntime

import (
	"context"
	"errors"
	"testing"

	"github.com/kitdine/agent-deck/internal/store"
)

type scopePlanner struct{ combined, usage int }

func (p *scopePlanner) Combined(context.Context, CJSIRoundRequest) (CJSIPublicationPlan, store.CJSIRuntimeResult, error) {
	p.combined++
	return CJSIPublicationPlan{}, store.CJSIRuntimeResult{}, store.ErrTelemetrySourceIdentityPending
}
func (p *scopePlanner) UsageOnly(context.Context, CJSIRoundRequest) (store.CJSIRuntimeResult, error) {
	p.usage++
	return store.CJSIRuntimeResult{}, nil
}
func TestCJSIPublicationLifecycle(t *testing.T) {
	p := &scopePlanner{}
	if _, err := dispatchCJSIRound(t.Context(), p, ScopeUsage, CJSIRoundRequest{}); err != nil || p.usage != 1 || p.combined != 0 {
		t.Fatalf("usage opened combined planning: %v", err)
	}
	s := server{}
	r, _, err := s.selectRoundLocked("home", ScopeUsage)
	if err != nil || r.managedScope != ScopeUsage {
		t.Fatal(err)
	}
	same, _, err := s.selectRoundLocked("home", ScopeSession)
	if err != nil || same != r || r.managedScope != ScopeBoth {
		t.Fatal("round lost combined admission", err)
	}
	if _, err = dispatchCJSIRound(t.Context(), p, r.managedScope, CJSIRoundRequest{}); !errors.Is(err, store.ErrTelemetrySourceIdentityPending) || p.combined != 1 {
		t.Fatal("held combined plan", err)
	}
}
