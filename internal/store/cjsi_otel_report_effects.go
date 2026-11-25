package store

import (
	"context"
	"database/sql"
	"reflect"
	"slices"

	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// CJSIOTelReportRows contains detached SQL row images; nullable run/turn
// fields and the original public spelling of a session are preserved.
type CJSIOTelReportRows struct {
	Events   []cjsiOTelEvent
	Bindings []cjsiOTelBinding
	Sessions []cjsiOTelSession
}

type CJSIOTelReportEffects struct {
	Remove, Insert CJSIOTelReportRows
	Restoring      bool
}

// VerifyReportVacated checks the restoration preconditions on the report's
// read-only overlay after the proposed JSONL withdrawal and before restoration.
func (change *CJSIOTelChange) VerifyReportVacated(ctx context.Context, q CJSIOTelReadView) error {
	if change == nil || q == nil || !change.restoring {
		return errCJSIOTelPrecondition
	}
	return change.checkVacated(ctx, q, nil)
}

// Preparation owns disposition selection. Both consumers traverse the same
// selected operations, in order: the observation, then its settled rows.
func (change *CJSIOTelChange) visitEdits(observation func(cjsiOTelEdit) error, settled func(*cjsiOTelSettled) error) error {
	for _, edit := range change.edits {
		if observation != nil {
			if err := observation(edit); err != nil {
				return err
			}
		}
		if edit.settled != nil {
			if err := settled(edit.settled); err != nil {
				return err
			}
		}
	}
	return nil
}

// ReportEffects projects the prepared Apply plan, not the historical period:
// releaseCJSIOTel has already removed invalidated receipts and narrowed the
// session bounds. Check the same guard and the independently read preimage
// before lending these rows to a different (rendering) transaction.
func (change *CJSIOTelChange) ReportEffects(ctx context.Context, tx *sql.Tx, receipts CJSIOTelReceiptStore, thread string, stamp ledger.ProposalStamp) (CJSIOTelReportEffects, error) {
	var out CJSIOTelReportEffects
	if change == nil || tx == nil || receipts == nil || change.guard.Thread != thread || change.guard.Stamp != stamp {
		return out, errCJSIOTelPrecondition
	}
	guard, err := receipts.Current(ctx, tx, thread)
	if err != nil {
		return out, err
	}
	if guard != change.guard {
		return out, errCJSIOTelPrecondition
	}
	before, err := readCJSIOTelSide(ctx, tx, thread)
	if err != nil {
		return out, err
	}
	if !reflect.DeepEqual(before, change.before) {
		return out, errCJSIOTelPrecondition
	}
	out.Restoring = change.restoring
	side := &out.Remove
	if change.restoring {
		side = &out.Insert
		if change.retire {
			out.Remove.Sessions = slices.Clone(change.summaries)
		}
	}
	err = change.visitEdits(nil, func(s *cjsiOTelSettled) error {
		side.Events = append(side.Events, s.Event)
		if s.Binding != nil {
			side.Bindings = append(side.Bindings, *s.Binding)
		}
		return nil
	})
	if err != nil {
		return CJSIOTelReportEffects{}, err
	}
	if change.session != nil {
		side.Sessions = append(side.Sessions, *change.session)
	}
	return out, nil
}
