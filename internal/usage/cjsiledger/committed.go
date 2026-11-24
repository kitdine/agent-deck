package cjsiledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// EventKey is the usage_events key for a metered call. An interval cannot
// acquire one. JSON's typed array encoding preserves the original ID bytes.
func (k Key) EventKey() string {
	switch k.Kind {
	case LegacyCall:
		return k.ID
	case Response:
		b, _ := json.Marshal([3]string{"codex", k.Thread, k.ID})
		h := sha256.Sum256(b)
		return "codex-response:" + hex.EncodeToString(h[:])
	}
	return ""
}

// CommittedCall is supplied by the coordinator's current selected core
// snapshot. Its explicit member and old public image prove the identity of
// an otherwise unreplayable call. An arbitrary orphan admits no such link.
type CommittedCall struct {
	Selection ProposalStamp
	Key       Key
	Prior     OldRow
	Tokens    Counts
}

func reconcileCommitted(c CommittedCall, base Basis, thread string, old OldRow, members []Member, sources []Source) (keep, proven bool) {
	if c.Selection.Basis.Ticket <= 0 || c.Selection.Basis.Ticket > base.BaseTicket || c.Selection.Basis.CoreEpoch != base.CoreEpoch ||
		c.Selection.Diff == ([32]byte{}) || c.Key.Kind != Response || c.Key.Thread != thread || c.Key.EventKey() != old.EventKey || c.Prior != old {
		return false, false
	}
	// A confirmed response is one-to-one regardless of representative movement.
	// Different stable IDs prove independence. An interval without a request
	// boundary cannot establish independence from an unreplayable old event.
	for _, m := range members {
		if m.Key == c.Key {
			return false, true
		}
	}
	// Multiple currently verified incompatible values revoke the previous
	// representative as well. Its stable identity is already authenticated.
	var prior Counts
	seen := false
	for _, s := range sources {
		for _, m := range s.meters {
			if m.record.ResponseID != c.Key.ID {
				continue
			}
			n, ok := counts(m.record.Usage)
			if !ok {
				continue
			}
			if seen {
				var agree bool
				prior, agree = commonCounts(prior, n)
				if !agree {
					return false, true
				}
			} else {
				prior = n
				seen = true
			}
		}
	}
	if len(members) == 0 {
		return false, false
	}
	for _, m := range members {
		if m.Key.Kind != Response {
			return false, false
		}
	}
	for _, s := range sources {
		for _, meter := range s.meters {
			if meter.record.ResponseID == c.Key.ID {
				return false, false
			}
		}
	}
	return true, true
}
