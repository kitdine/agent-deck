package cjsiledger

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"slices"
	"strings"

	"github.com/kitdine/agent-deck/internal/ingest"
)

// OldImage is the whole stored event, including both possible run witnesses.
type OldImage struct {
	EventKey, Client, SessionID, EventID, At, Model, Path                            string
	Offset, Input, Output, Cached, Write, CacheRead, CacheCreation, Write5m, Write1h int64
	TurnIndex, RunID, Binding                                                        sql.NullInt64
}

func (i OldImage) Precondition() OldRow {
	b, _ := json.Marshal(i)
	return OldRow{i.EventKey, sha256.Sum256(b)}
}

type Anchor struct {
	Original Reference
	Through  [32]byte
}
type Prior struct {
	Image  OldImage
	Anchor Anchor
}
type RunRange struct {
	// OriginalPath and the anchor pair are selected from the original valid
	// completed run, after the source-range generation has been verified.
	OriginalPath, StartHash, EndHash string
	RunID                            int64
	Completed, Exact                 bool
	Source                           SourceStamp
	Bytes                            ingest.CodexSpan
}
type History struct {
	Committed            []CommittedCall
	Assessed, NoSafeSide bool
	Rows                 []OldRow
	Evidence             []Prior
	Runs                 []RunRange
}
type legacyFact struct {
	image   OldImage
	span    ingest.CodexSpan
	through [32]byte
}

func (f legacyFact) matches(p Prior) bool {
	a := p.Anchor
	i := p.Image
	// A renamed or equivalent copy must reproduce every byte through the old
	// observation. A same-name rewrite cannot qualify by sharing its offset.
	if a.Original.Source.Name != sha256.Sum256([]byte(i.Path)) || a.Original.Span != f.span ||
		a.Through != f.through || a.Through == ([32]byte{}) || a.Original.Source.Prefix.Progress.CompleteEnd < f.span.End ||
		a.Original.Source.Prefix.Version.Parser != Parser {
		return false
	}
	observed := f.image
	observed.Path = i.Path
	observed.RunID, observed.Binding = i.RunID, i.Binding
	return observed == i
}
func reconcile(b Basis, thread string, h History, sources []Source, members []Member) ([]Member, Delta, Gap, error) {
	rows := slices.Clone(h.Rows)
	slices.SortFunc(rows, func(a, b OldRow) int { return strings.Compare(a.EventKey, b.EventKey) })
	evidence := map[string]Prior{}
	for _, p := range h.Evidence {
		if _, ok := evidence[p.Image.EventKey]; ok {
			return nil, Delta{}, 0, ErrProposalMismatch
		}
		evidence[p.Image.EventKey] = p
	}
	accepted := map[string]CommittedCall{}
	for _, c := range h.Committed {
		if _, dup := accepted[c.Prior.EventKey]; dup {
			return nil, Delta{}, 0, ErrProposalMismatch
		}
		accepted[c.Prior.EventKey] = c
	}
	held := !h.Assessed
	var d Delta
	var gaps Gap
	for i, row := range rows {
		if row.EventKey == "" || row.RowHash == ([32]byte{}) || i > 0 && rows[i-1].EventKey == row.EventKey {
			return nil, d, 0, ErrProposalMismatch
		}
		if c, ok := accepted[row.EventKey]; ok {
			delete(accepted, row.EventKey)
			keep, proven := reconcileCommitted(c, b, thread, row, members, sources)
			if proven {
				for j, m := range members {
					if m.Key == c.Key {
						if _, compatible := commonCounts(c.Tokens, m.Tokens); !compatible {
							// A stale response image does not win a disagreement by arriving first.
							members = slices.Delete(members, j, j+1)
							gaps |= ConflictingResponse
						}
						break
					}
				}
				if !keep {
					d.Retract = append(d.Retract, row)
				}
				delete(evidence, row.EventKey)
				continue
			}
		}
		p, ok := evidence[row.EventKey]
		if !ok {
			held = true
			continue
		}
		if p.Image.Precondition() != row {
			return nil, d, 0, ErrProposalMismatch
		}
		delete(evidence, row.EventKey)
		matched := false
		for _, s := range sources {
			for _, f := range s.old {
				if f.matches(p) {
					matched = true
					break
				}
			}
		}
		if !matched {
			held = true
			continue
		}
		// A legacy call already represented by its original key stays in place.
		unchanged := false
		for _, m := range members {
			if m.Key.Kind == LegacyCall && m.Key.ID == row.EventKey && m.Key.Thread == p.Image.SessionID && m.TurnID == p.Image.EventID {
				unchanged = true
				break
			}
		}
		if !unchanged {
			d.Retract = append(d.Retract, row)
		}
	}
	if len(accepted) != 0 || len(evidence) != 0 {
		return nil, d, 0, ErrProposalMismatch
	}
	if held {
		g := OldSideHeld
		if h.NoSafeSide {
			g |= UnsafeOldSide
		}
		return nil, Delta{Hold: rows}, g, nil
	}
	for j := range members {
		if members[j].Key.Kind == TokenInterval {
			continue
		}
		members[j].ExactRun = exactRunForProof(members[j].Proof, h.Runs)
	}
	return members, d, gaps, nil
}

// Every eligible run intersecting any local alias votes on the run ID.
// At least one complete, verified range must also cover its whole record.
func exactRunForProof(refs []Reference, runs []RunRange) int64 {
	var id int64
	covered := false
	for _, ref := range refs {
		for _, run := range runs {
			span := run.Bytes
			if run.RunID <= 0 || !run.Completed || !run.Exact || run.Source != ref.Source || span.Start < 0 || span.Start >= span.End || span.End <= ref.Span.Start || ref.Span.End <= span.Start {
				continue
			}
			if id != 0 && id != run.RunID {
				return 0
			}
			id = run.RunID
			if span.Start <= ref.Span.Start && ref.Span.End <= span.End && span.End <= ref.Source.Prefix.Progress.CompleteEnd {
				covered = true
			}
		}
	}
	if covered {
		return id
	}
	return 0
}
