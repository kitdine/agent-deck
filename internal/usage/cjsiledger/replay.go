package cjsiledger

import (
	"bytes"
	"cmp"
	"context"
	"slices"

	"github.com/kitdine/agent-deck/internal/ingest"
)

const UnsafeOldSide Gap = 1 << 8

type ReplayInput struct {
	Basis    Basis
	Thread   string
	Expected []SourceStamp
	Sources  []Source
	History  History
	Hints    Gap
}

func Replay(ctx context.Context, in ReplayInput) (Proposal, error) {
	if err := ctx.Err(); err != nil {
		return Proposal{}, err
	}
	if in.Thread == "" || ingest.CodexLegacySessionID(in.Thread) != in.Thread || in.Hints & ^(UnverifiedSource|IdentityConflict) != 0 {
		return Proposal{}, ErrProposalMismatch
	}
	expected := map[[32]byte]SourceStamp{}
	for _, st := range in.Expected {
		if _, dup := expected[st.Name]; dup {
			return Proposal{}, ErrProposalMismatch
		}
		expected[st.Name] = st
	}
	sources := slices.Clone(in.Sources)
	slices.SortFunc(sources, func(a, b Source) int { return bytes.Compare(a.stamp.Name[:], b.stamp.Name[:]) })
	v := Eligibility{Thread: in.Thread, Stamp: ProposalStamp{Basis: in.Basis}, Gaps: in.Hints}
	for _, s := range sources {
		st, ok := expected[s.stamp.Name]
		if !ok || st != s.stamp || s.thread != in.Thread {
			return Proposal{}, ErrProposalMismatch
		}
		delete(expected, st.Name)
		v.Sources = append(v.Sources, st)
		v.Gaps |= s.gaps
	}
	if len(expected) != 0 {
		return Proposal{}, ErrProposalMismatch
	}
	m, g, err := classify(ctx, sources)
	if err != nil {
		return Proposal{}, err
	}
	v.Gaps |= g
	m, d, g, err := reconcile(in.Basis, in.Thread, in.History, sources, m)
	if err != nil {
		return Proposal{}, err
	}
	v.Gaps |= g
	hasCall := slices.ContainsFunc(m, func(m Member) bool { return m.Key.Kind != TokenInterval })
	if hasCall {
		v.State = LocalCalls
		v.Members = m
	} else if len(sources) > 0 {
		v.State = NoLocalCall
	} else if v.Gaps&IdentityConflict != 0 {
		v.State = Quarantined
	}
	v.UnresolvedOverlap = v.Gaps&(OldSideHeld|PossibleOverlap|UnverifiedSource|IdentityConflict|IncompletePrefix) != 0 || !hasCall && v.Gaps&UnallocatedTokens != 0
	if err := ctx.Err(); err != nil {
		return Proposal{}, err
	}
	return seal(v, d)
}

func compareKey(a, b Key) int {
	return cmp.Or(cmp.Compare(a.Thread, b.Thread), cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.ID, b.ID))
}
