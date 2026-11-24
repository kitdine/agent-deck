// Package cjsiledger prepares private Codex usage proposals. It has no
// database dependency; store can consume its result without importing usage.
package cjsiledger

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
)

const Parser = 9

// Basis comes from the coordinator's snapshot. InputDigest identifies its
// selected sources, inventory, coverage, old-side facts and dependencies.
// A proposal is scoped to one canonical execution thread.
type Basis struct {
	Ticket, Attempt, BaseTicket, CoreEpoch int64
	InputDigest                            [32]byte
}

type ProposalStamp struct {
	Basis Basis
	Diff  [32]byte
}

// SourceStamp identifies the selected name without retaining a path. Prefix
// includes file identity, generation, parser, line policy, full digest, read
// end and complete end. Epoch comes from the independent current selection.
type SourceStamp struct {
	Name   [32]byte
	Epoch  int64
	Prefix ingest.CodexPrefixEvidence
}

type Reference struct {
	Source SourceStamp
	Span   ingest.CodexSpan // half-open, in the authenticated complete prefix
}

type DetailMask uint8

const (
	CachedKnown DetailMask = 1 << iota
	WriteKnown
	ReasoningKnown
)

// Input and Output are always present on a member. Bits distinguish an
// explicit auxiliary zero from a dimension the observation did not supply.
type Counts struct {
	Input, Output, Cached, Write, Reasoning int64
	Known                                   DetailMask
}

type MemberKind uint8

const (
	Response MemberKind = iota + 1
	LegacyCall
	TokenInterval
)

// Key is a typed tuple; delimiters inside an original ID are not structural.
type Key struct {
	Kind       MemberKind
	Thread, ID string
}

// CounterRange describes cumulative boundaries, not a second call. Time
// bounds are original source instants, inclusive; never split a crossing
// interval proportionally. Zero times are unknown.
type CounterRange struct {
	Epoch            [32]byte
	Lower, Upper     Counts
	LowerAt, UpperAt time.Time
}

type Member struct {
	Key           Key
	Tokens        Counts
	TurnID, Model string
	TurnIndex     int
	At            time.Time // request time; zero on a token-only member
	Interval      CounterRange
	Proof         []Reference // proven local aliases; endpoints for intervals
	ExactRun      int64       // zero unless a verified historical range qualifies
}

type State uint8

const (
	Unassessed State = iota
	Quarantined
	NoLocalCall
	LocalCalls
)

type Gap uint32

const (
	UnverifiedSource Gap = 1 << iota
	IdentityConflict
	IncompletePrefix
	InvalidMeter
	ConflictingResponse
	PossibleOverlap
	OldSideHeld
	UnallocatedTokens
)

// Only LocalCalls supplies Task 4 with takeover membership. Pending means
// no new takeover/settlement may be inferred here. Partial is independent:
// a confirmed call can coexist with incomplete coverage. UnresolvedOverlap
// concerns still-outstanding metering, not a fully replayed rejected call.
// Neither absence nor rejection is an instruction to restore or delete OTel.
type Eligibility struct {
	Stamp             ProposalStamp
	Thread            string
	State             State
	Pending, Partial  bool
	UnresolvedOverlap bool
	Gaps              Gap
	Sources           []SourceStamp
	Members           []Member
}

var ErrProposalMismatch = errors.New("Codex usage proposal does not match selected inputs")

// Proposal has no useful zero value. Its accessor never consults live tables
// or a JSONL header. Callers compare an independently selected stamp.
type Proposal struct {
	view  Eligibility
	delta Delta
}

func (p Proposal) Stamp() ProposalStamp { return p.view.Stamp }

func (p Proposal) Eligibility(current ProposalStamp, thread string) (Eligibility, error) {
	v := p.view
	if thread == "" || thread != v.Thread || current.Diff == ([32]byte{}) || current != v.Stamp {
		return Eligibility{}, ErrProposalMismatch
	}
	v.Sources = slices.Clone(v.Sources)
	v.Members = slices.Clone(v.Members)
	for i := range v.Members {
		v.Members[i].Proof = slices.Clone(v.Members[i].Proof)
	}
	return v, nil
}

// Retractions carry the old row precondition, not merely a source/thread.
// A held old side is neither retracted nor inserted a second time.
type OldRow struct {
	EventKey string
	RowHash  [32]byte
}

type Delta struct {
	Retract []OldRow
	Hold    []OldRow
}

func (p Proposal) Changes(current ProposalStamp, thread string) (Delta, error) {
	if current.Diff == ([32]byte{}) || current != p.view.Stamp || thread == "" || thread != p.view.Thread {
		return Delta{}, ErrProposalMismatch
	}
	return Delta{slices.Clone(p.delta.Retract), slices.Clone(p.delta.Hold)}, nil
}

func seal(v Eligibility, d Delta) (Proposal, error) {
	b := v.Stamp.Basis
	if b.Ticket <= 0 || b.Attempt <= 0 || b.BaseTicket < 0 || b.BaseTicket >= b.Ticket || b.CoreEpoch <= 0 ||
		b.InputDigest == ([32]byte{}) || v.Thread == "" {
		return Proposal{}, ErrProposalMismatch
	}
	v.Pending = v.State != LocalCalls
	v.Partial = v.Gaps != 0 || v.Pending
	body, err := json.Marshal(struct {
		View  Eligibility
		Delta Delta
	}{v, d})
	if err != nil {
		return Proposal{}, err
	}
	v.Stamp.Diff = sha256.Sum256(body)
	return Proposal{view: v, delta: d}, nil
}
