package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	ledger "github.com/kitdine/agent-deck/internal/usage/cjsiledger"
)

// IsCJSICore reports whether s is on the CJSI managed schema (>= 37), where
// OTel ownership transitions require explicit receipt-bearing authority rather
// than legacy trigger/header shortcuts.
func (s *Store) IsCJSICore(ctx context.Context) (bool, error) {
	if s == nil || s.DB == nil {
		return false, errors.New("nil store")
	}
	return isCJSICoreQuery(ctx, s.DB)
}

func isCJSICoreQuery(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) (bool, error) {
	var version int
	if err := q.QueryRowContext(ctx, `SELECT version FROM schema_metadata WHERE singleton = 1`).Scan(&version); err != nil {
		return false, err
	}
	return version >= cjsiLayoutCoreSchemaVersion, nil
}

// Schema 37 stores one catalog reference. Resolve its legacy alias before
// hashing, persisting, or comparing the caller's evidence with a receipt.
func (a CJSIOTelAttribution) canonical() CJSIOTelAttribution {
	if a.CatalogVersion == "" {
		a.CatalogVersion = a.PriceReference
	}
	if a.PriceReference == "" {
		a.PriceReference = a.CatalogVersion
	}
	return a
}

// CJSIOTelAttributionEvidence computes the canonical 32-byte digest pinning
// durable route and pricing inputs for one OTel receipt event.
func CJSIOTelAttributionEvidence(a CJSIOTelAttribution) [32]byte {
	type payload struct {
		RouteIDValid       bool   `json:"route_id_valid"`
		RouteID            int64  `json:"route_id"`
		RouteAt            string `json:"route_at"`
		Provider           string `json:"provider"`
		Quality            string `json:"quality"`
		Multiplier         string `json:"multiplier"`
		CatalogVersion     string `json:"catalog_version"`
		PriceModel         string `json:"price_model"`
		PriceEffectiveFrom string `json:"price_effective_from"`
		PriceReference     string `json:"price_reference"`
	}
	a = a.canonical()
	body, _ := json.Marshal(payload{
		RouteIDValid:       a.RouteID.Valid,
		RouteID:            a.RouteID.Int64,
		RouteAt:            a.RouteAt,
		Provider:           a.Provider,
		Quality:            a.Quality,
		Multiplier:         a.Multiplier,
		CatalogVersion:     a.CatalogVersion,
		PriceModel:         a.PriceModel,
		PriceEffectiveFrom: a.PriceEffectiveFrom,
		PriceReference:     a.PriceReference,
	})
	return sha256.Sum256(body)
}

func cjsiObservationDigest(o TelemetryObservation) [32]byte {
	body, _ := json.Marshal(o)
	return sha256.Sum256(body)
}

func cjsiTakeoverProofDigest(opened, support ledger.ProposalStamp, requests []ledger.Key) [32]byte {
	reqs := slices.Clone(requests)
	slices.SortFunc(reqs, compareCJSIKey)
	body, _ := json.Marshal(struct {
		Opened   ledger.ProposalStamp `json:"opened"`
		Support  ledger.ProposalStamp `json:"support"`
		Requests []ledger.Key         `json:"requests"`
	}{Opened: opened, Support: support, Requests: reqs})
	return sha256.Sum256(body)
}

func cjsiDBDisposition(d CJSIOTelDisposition) (string, error) {
	switch d {
	case cjsiOTelApplied:
		return "applied", nil
	case cjsiOTelRestored, cjsiOTelReturnedPending:
		return "restored", nil
	case cjsiOTelInvalidated:
		return "invalid", nil
	default:
		return "", errCJSIOTelPrecondition
	}
}

func cjsiFromDBDisposition(raw string, settled bool) (CJSIOTelDisposition, error) {
	switch raw {
	case "applied":
		return cjsiOTelApplied, nil
	case "invalid":
		return cjsiOTelInvalidated, nil
	case "restored":
		if settled {
			return cjsiOTelRestored, nil
		}
		return cjsiOTelReturnedPending, nil
	default:
		return "", errCJSIOTelPrecondition
	}
}

func nullableText(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func nullInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}

// CJSIOTelSQLiteStore implements CJSIOTelReceiptStore against the real schema-37
// tables (cjsi_takeovers, cjsi_receipt_observations, cjsi_receipt_events,
// cjsi_receipt_sessions, cjsi_receipt_markers) joined with cjsi_usage_groups,
// cjsi_usage_selection, cjsi_publication_attempts, and cjsi_usage_members.
type CJSIOTelSQLiteStore struct {
	Guard        func(context.Context, *sql.Tx, string) (CJSIOTelGuard, error)
	Attributions map[string]CJSIOTelAttribution
	Release      *CJSIOTelRelease
}

func (st CJSIOTelSQLiteStore) Current(ctx context.Context, tx *sql.Tx, thread string) (CJSIOTelGuard, error) {
	if st.Guard == nil {
		return CJSIOTelGuard{}, errCJSIOTelPrecondition
	}
	return st.Guard(ctx, tx, thread)
}

func (st CJSIOTelSQLiteStore) Select(ctx context.Context, tx *sql.Tx, thread string) (CJSIOTelSelection, error) {
	guard, err := st.Current(ctx, tx, thread)
	if err != nil {
		return CJSIOTelSelection{}, err
	}
	before, err := readCJSIOTelSide(ctx, tx, thread)
	if err != nil {
		return CJSIOTelSelection{}, err
	}
	period, err := readLatestCJSITakeoverPeriod(ctx, tx, thread)
	if err != nil {
		return CJSIOTelSelection{}, err
	}
	return CJSIOTelSelection{
		Revision:     guard.Revision,
		Thread:       guard.Thread,
		Stamp:        guard.Stamp,
		Before:       before,
		Period:       period,
		Attributions: maps.Clone(st.Attributions),
		Release:      st.Release,
	}, nil
}

func (st CJSIOTelSQLiteStore) Save(ctx context.Context, tx *sql.Tx, old *CJSIOTelPeriod, next CJSIOTelPeriod) error {
	if next.Version != 1 || next.Number <= 0 || next.Thread == "" || len(next.Receipts) == 0 || len(next.Requests) == 0 {
		return errCJSIOTelPrecondition
	}
	nextDigest := cjsiTakeoverProofDigest(next.Opened, next.Support, next.Requests)
	isNewPeriod := old == nil || old.Number != next.Number
	if isNewPeriod {
		if old != nil {
			if old.Closed == (ledger.ProposalStamp{}) || next.Number != old.Number+1 {
				return errCJSIOTelPrecondition
			}
			oldDigest := cjsiTakeoverProofDigest(old.Opened, old.Support, old.Requests)
			var exists bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM cjsi_takeovers
 WHERE client='codex' AND thread=? AND period=? AND opened_ticket=? AND closed_ticket=? AND attempt=? AND proof_digest=?)`,
				old.Thread, old.Number, old.Opened.Basis.Ticket, old.Closed.Basis.Ticket, old.Opened.Basis.Attempt, oldDigest[:]).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return errCJSIOTelPrecondition
			}
		}
		var closedTicket any
		if next.Closed != (ledger.ProposalStamp{}) {
			closedTicket = next.Closed.Basis.Ticket
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO cjsi_takeovers(
 client,thread,period,opened_ticket,closed_ticket,attempt,receipt_version,safe_old_side,proof_digest)
 VALUES('codex',?,?,?,?,?,1,1,?)`,
			next.Thread, next.Number, next.Opened.Basis.Ticket, closedTicket, next.Opened.Basis.Attempt, nextDigest[:]); err != nil {
			return err
		}
		for _, key := range slices.Sorted(maps.Keys(next.Receipts)) {
			if err := insertCJSIReceiptRow(ctx, tx, next.Thread, next.Number, next.Receipts[key]); err != nil {
				return err
			}
		}
		if next.Session != nil {
			var digest any
			if len(next.SessionBindingDigest) > 0 {
				if len(next.SessionBindingDigest) != 32 {
					return errCJSIOTelPrecondition
				}
				digest = next.SessionBindingDigest
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO cjsi_receipt_sessions(
 client,thread,period,first_at,last_at,started_at,route_id,binding_digest)
 VALUES('codex',?,?,?,?,?,?,?)`,
				next.Thread, next.Number, next.Session.First, next.Session.Last, next.Session.Started, next.SessionRouteID, digest); err != nil {
				return err
			}
		}
		if next.Marker != nil {
			if _, err := tx.ExecContext(ctx, `INSERT INTO cjsi_receipt_markers(
 client,thread,period,source_kind,model,first_at,last_at,support_entry,settled_at)
 VALUES('codex',?,?,?,?,?,?,?,?)`,
				next.Thread, next.Number, next.Marker.Kind, next.Marker.Model, next.Marker.First, next.Marker.Last, next.Marker.Support, next.Marker.Settled); err != nil {
				return err
			}
		}
		return nil
	}
	// Updating or closing an existing active period.
	if old.Closed != (ledger.ProposalStamp{}) || old.Opened != next.Opened {
		return errCJSIOTelPrecondition
	}
	oldDigest := cjsiTakeoverProofDigest(old.Opened, old.Support, old.Requests)
	if next.Closed != (ledger.ProposalStamp{}) {
		if err := cjsiOTelChanged(tx.ExecContext(ctx, `UPDATE cjsi_takeovers SET closed_ticket=?, proof_digest=?
 WHERE client='codex' AND thread=? AND period=? AND opened_ticket=? AND attempt=? AND closed_ticket IS NULL AND proof_digest=?`,
			next.Closed.Basis.Ticket, nextDigest[:], next.Thread, next.Number, old.Opened.Basis.Ticket, old.Opened.Basis.Attempt, oldDigest[:])); err != nil {
			return err
		}
	} else {
		if err := cjsiOTelChanged(tx.ExecContext(ctx, `UPDATE cjsi_takeovers SET proof_digest=?
 WHERE client='codex' AND thread=? AND period=? AND opened_ticket=? AND attempt=? AND closed_ticket IS NULL AND proof_digest=?`,
			nextDigest[:], next.Thread, next.Number, old.Opened.Basis.Ticket, old.Opened.Basis.Attempt, oldDigest[:])); err != nil {
			return err
		}
	}
	for _, key := range slices.Sorted(maps.Keys(next.Receipts)) {
		r := next.Receipts[key]
		prev, existed := old.Receipts[key]
		if !existed {
			if err := insertCJSIReceiptRow(ctx, tx, next.Thread, next.Number, r); err != nil {
				return err
			}
			continue
		}
		if prev.Disposition != r.Disposition {
			oldDisp, err := cjsiDBDisposition(prev.Disposition)
			if err != nil {
				return err
			}
			newDisp, err := cjsiDBDisposition(r.Disposition)
			if err != nil {
				return err
			}
			obsDigest := cjsiObservationDigest(prev.Original)
			if err := cjsiOTelChanged(tx.ExecContext(ctx, `UPDATE cjsi_receipt_observations SET disposition=?
 WHERE client='codex' AND thread=? AND period=? AND observation_key=? AND disposition=? AND observation_digest=?`,
				newDisp, next.Thread, next.Number, key, oldDisp, obsDigest[:])); err != nil {
				return err
			}
		}
	}
	return nil
}

func insertCJSIReceiptRow(ctx context.Context, tx *sql.Tx, thread string, period int64, r CJSIOTelReceipt) error {
	o := r.Original
	disp, err := cjsiDBDisposition(r.Disposition)
	if err != nil {
		return err
	}
	digest := cjsiObservationDigest(o)
	if _, err := tx.ExecContext(ctx, `INSERT INTO cjsi_receipt_observations(
 client,thread,period,observation_key,state,reason,metered,support_entry,source,producer,producer_version,
 originator,source_class,transport,model,event_kind,event_name,request_id,response_id,attempt_id,
 record_at,observed_at,received_at,input_tokens,output_tokens,cached_tokens,cache_write_tokens,reasoning_tokens,total_tokens,
 observation_digest,disposition)
 VALUES('codex',?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		thread, period, o.Key, o.State, o.Reason, boolInt(o.Metered), o.SupportEntry, o.Source, o.Producer, o.ProducerVersion,
		o.Originator, o.SourceClass, o.Transport, o.Model, o.EventKind, o.EventName, o.RequestID, o.ResponseID, o.AttemptID,
		o.RecordAt, o.ObservedAt, o.ReceivedAt, nullableInt(o.InputTokens), nullableInt(o.OutputTokens), nullableInt(o.CachedTokens),
		nullableInt(o.CacheWriteTokens), nullableInt(o.ReasoningTokens), nullableInt(o.TotalTokens), digest[:], disp); err != nil {
		return err
	}
	if r.Settled == nil {
		return nil
	}
	e := r.Settled.Event
	a := r.Settled.Attribution
	var bindingRun sql.NullInt64
	if r.Settled.Binding != nil {
		bindingRun = sql.NullInt64{Int64: r.Settled.Binding.Run, Valid: true}
	}
	a = a.canonical()
	if a.PriceReference != a.CatalogVersion || a.Evidence != CJSIOTelAttributionEvidence(a) {
		return errCJSIOTelPrecondition
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO cjsi_receipt_events(
 client,thread,period,observation_key,event_key,session_id,event_id,event_at,model,source_path,source_offset,turn_index,
 input_tokens,cached_input_tokens,output_tokens,cache_read_tokens,cache_creation_tokens,cache_write_tokens,cache_write_5m_tokens,cache_write_1h_tokens,
 run_id,binding_run_id,route_id,route_at,provider,quality,multiplier,catalog_version,price_model,price_effective_from)
 VALUES('codex',?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		thread, period, o.Key, e.Key, e.Thread, e.ID, e.At, e.Model, e.Path, e.Offset, e.Turn,
		e.Input, e.Cached, e.Output, e.CacheRead, e.CacheCreation, e.CacheWrite, e.Write5m, e.Write1h,
		e.Run, bindingRun, a.RouteID, nullableText(a.RouteAt), nullableText(a.Provider), nullableText(a.Quality),
		nullableText(a.Multiplier), nullableText(a.CatalogVersion), nullableText(a.PriceModel), nullableText(a.PriceEffectiveFrom))
	return err
}

func readCJSIGroupStamp(ctx context.Context, tx *sql.Tx, ticket, attempt int64, thread string) (ledger.ProposalStamp, error) {
	var baseTicket, coreEpoch int64
	var inputDigest, diffDigest []byte
	err := tx.QueryRowContext(ctx, `SELECT a.base_ticket, a.core_epoch, g.input_digest, g.diff_digest
 FROM cjsi_usage_groups g JOIN cjsi_publication_attempts a USING(ticket, attempt)
 WHERE g.ticket=? AND g.attempt=? AND g.thread=?`, ticket, attempt, thread).Scan(&baseTicket, &coreEpoch, &inputDigest, &diffDigest)
	if err != nil {
		return ledger.ProposalStamp{}, err
	}
	if len(inputDigest) != 32 || len(diffDigest) != 32 {
		return ledger.ProposalStamp{}, errCJSIOTelPrecondition
	}
	var stamp ledger.ProposalStamp
	stamp.Basis = ledger.Basis{Ticket: ticket, Attempt: attempt, BaseTicket: baseTicket, CoreEpoch: coreEpoch}
	copy(stamp.Basis.InputDigest[:], inputDigest)
	copy(stamp.Diff[:], diffDigest)
	return stamp, nil
}

func readCJSIGroupRequests(ctx context.Context, tx *sql.Tx, ticket, attempt int64, thread string) ([]ledger.Key, error) {
	rows, err := tx.QueryContext(ctx, `SELECT kind, member_id FROM cjsi_usage_members
 WHERE ticket=? AND attempt=? AND thread=? AND kind IN (1,2) ORDER BY kind, member_id`, ticket, attempt, thread)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []ledger.Key
	for rows.Next() {
		var kind int
		var id string
		if err := rows.Scan(&kind, &id); err != nil {
			return nil, err
		}
		keys = append(keys, ledger.Key{Kind: ledger.MemberKind(kind), Thread: thread, ID: id})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	slices.SortFunc(keys, compareCJSIKey)
	return keys, nil
}

func readLatestCJSITakeoverPeriod(ctx context.Context, tx *sql.Tx, thread string) (*CJSIOTelPeriod, error) {
	var periodNum, openedTicket, attempt, version, safeOld int64
	var closedTicket sql.NullInt64
	var proofDigest []byte
	err := tx.QueryRowContext(ctx, `SELECT period, opened_ticket, closed_ticket, attempt, receipt_version, safe_old_side, proof_digest
 FROM cjsi_takeovers WHERE client='codex' AND thread=? ORDER BY period DESC LIMIT 1`, thread).Scan(
		&periodNum, &openedTicket, &closedTicket, &attempt, &version, &safeOld, &proofDigest)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(proofDigest) != 32 || version != 1 || safeOld != 1 {
		return nil, errCJSIOTelPrecondition
	}
	opened, err := readCJSIGroupStamp(ctx, tx, openedTicket, attempt, thread)
	if err != nil {
		return nil, errCJSIOTelPrecondition
	}
	var support ledger.ProposalStamp
	var requests []ledger.Key
	var closed ledger.ProposalStamp
	if closedTicket.Valid {
		// Resolve the closing attempt (preferring cjsi_usage_selection or opened attempt).
		var cAttempt int64
		err = tx.QueryRowContext(ctx, `SELECT attempt FROM cjsi_usage_selection WHERE thread=? AND ticket=?`, thread, closedTicket.Int64).Scan(&cAttempt)
		if errors.Is(err, sql.ErrNoRows) {
			err = tx.QueryRowContext(ctx, `SELECT g.attempt FROM cjsi_usage_groups g JOIN cjsi_publication_attempts a USING(ticket,attempt)
 WHERE g.ticket=? AND g.thread=? ORDER BY (a.state='opened') DESC, g.attempt DESC LIMIT 1`, closedTicket.Int64, thread).Scan(&cAttempt)
		}
		if err != nil {
			return nil, errCJSIOTelPrecondition
		}
		closed, err = readCJSIGroupStamp(ctx, tx, closedTicket.Int64, cAttempt, thread)
		if err != nil {
			return nil, errCJSIOTelPrecondition
		}
	}
	// Resolve the exact supporting group (matching proof_digest).
	matchedSupport := false
	type candidateGroup struct{ ticket, attempt int64 }
	var candidates []candidateGroup
	var selTicket, selAttempt int64
	if err := tx.QueryRowContext(ctx, `SELECT ticket, attempt FROM cjsi_usage_selection WHERE thread=?`, thread).Scan(&selTicket, &selAttempt); err == nil {
		candidates = append(candidates, candidateGroup{selTicket, selAttempt})
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	candidates = append(candidates, candidateGroup{openedTicket, attempt})
	upperTicket := int64(1<<62 - 1)
	if closedTicket.Valid {
		upperTicket = closedTicket.Int64
	}
	rows, err := tx.QueryContext(ctx, `SELECT ticket, attempt FROM cjsi_usage_groups WHERE thread=? AND ticket>=? AND ticket<=? ORDER BY ticket DESC, attempt DESC`,
		thread, openedTicket, upperTicket)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var c candidateGroup
		if err := rows.Scan(&c.ticket, &c.attempt); err != nil {
			rows.Close()
			return nil, err
		}
		if !slices.Contains(candidates, c) {
			candidates = append(candidates, c)
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for _, c := range candidates {
		st, err := readCJSIGroupStamp(ctx, tx, c.ticket, c.attempt, thread)
		if err != nil {
			continue
		}
		reqs, err := readCJSIGroupRequests(ctx, tx, c.ticket, c.attempt, thread)
		if err != nil || len(reqs) == 0 {
			continue
		}
		d := cjsiTakeoverProofDigest(opened, st, reqs)
		if bytes.Equal(d[:], proofDigest) {
			support, requests, matchedSupport = st, reqs, true
			break
		}
	}
	if !matchedSupport {
		return nil, errCJSIOTelPrecondition
	}
	// Load settled event rows first so we know which observations have settled children.
	settledMap := map[string]*CJSIOTelSettled{}
	evRows, err := tx.QueryContext(ctx, `SELECT observation_key, event_key, session_id, event_id, event_at, model, source_path, source_offset, turn_index,
 input_tokens, cached_input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, cache_write_tokens, cache_write_5m_tokens, cache_write_1h_tokens,
 run_id, binding_run_id, route_id, route_at, provider, quality, multiplier, catalog_version, price_model, price_effective_from
 FROM cjsi_receipt_events WHERE client='codex' AND thread=? AND period=?`, thread, periodNum)
	if err != nil {
		return nil, err
	}
	for evRows.Next() {
		var obsKey string
		var e cjsiOTelEvent
		e.Client = "codex"
		var bindingRun, routeID sql.NullInt64
		var routeAt, provider, quality, multiplier, catalogVersion, priceModel, priceEffectiveFrom sql.NullString
		if err := evRows.Scan(&obsKey, &e.Key, &e.Thread, &e.ID, &e.At, &e.Model, &e.Path, &e.Offset, &e.Turn,
			&e.Input, &e.Cached, &e.Output, &e.CacheRead, &e.CacheCreation, &e.CacheWrite, &e.Write5m, &e.Write1h,
			&e.Run, &bindingRun, &routeID, &routeAt, &provider, &quality, &multiplier, &catalogVersion, &priceModel, &priceEffectiveFrom); err != nil {
			evRows.Close()
			return nil, err
		}
		part := &CJSIOTelSettled{
			Contribution: cjsiOTelContribution{Observation: obsKey, Event: e.Key},
			Event:        e,
			Attribution: CJSIOTelAttribution{
				RouteID:            routeID,
				RouteAt:            routeAt.String,
				Provider:           provider.String,
				Quality:            quality.String,
				Multiplier:         multiplier.String,
				CatalogVersion:     catalogVersion.String,
				PriceModel:         priceModel.String,
				PriceEffectiveFrom: priceEffectiveFrom.String,
				PriceReference:     catalogVersion.String,
			},
		}
		part.Attribution.Evidence = CJSIOTelAttributionEvidence(part.Attribution)
		if bindingRun.Valid {
			part.Binding = &cjsiOTelBinding{Event: e.Key, Run: bindingRun.Int64}
		}
		settledMap[obsKey] = part
	}
	if err := evRows.Close(); err != nil {
		return nil, err
	}
	// Load observation receipts and verify observation_digest.
	obsRows, err := tx.QueryContext(ctx, `SELECT observation_key, state, reason, metered, support_entry, source, producer, producer_version,
 originator, source_class, transport, model, event_kind, event_name, request_id, response_id, attempt_id,
 record_at, observed_at, received_at, input_tokens, output_tokens, cached_tokens, cache_write_tokens, reasoning_tokens, total_tokens,
 observation_digest, disposition
 FROM cjsi_receipt_observations WHERE client='codex' AND thread=? AND period=? ORDER BY observation_key`, thread, periodNum)
	if err != nil {
		return nil, err
	}
	receipts := map[string]CJSIOTelReceipt{}
	var rawThread string
	for obsRows.Next() {
		var o TelemetryObservation
		var meteredInt int
		var inTok, outTok, cachedTok, writeTok, reasonTok, totTok sql.NullInt64
		var obsDigest []byte
		var rawDisp string
		if err := obsRows.Scan(&o.Key, &o.State, &o.Reason, &meteredInt, &o.SupportEntry, &o.Source, &o.Producer, &o.ProducerVersion,
			&o.Originator, &o.SourceClass, &o.Transport, &o.Model, &o.EventKind, &o.EventName, &o.RequestID, &o.ResponseID, &o.AttemptID,
			&o.RecordAt, &o.ObservedAt, &o.ReceivedAt, &inTok, &outTok, &cachedTok, &writeTok, &reasonTok, &totTok, &obsDigest, &rawDisp); err != nil {
			obsRows.Close()
			return nil, err
		}
		o.Metered = meteredInt != 0
		o.InputTokens = nullInt64Ptr(inTok)
		o.OutputTokens = nullInt64Ptr(outTok)
		o.CachedTokens = nullInt64Ptr(cachedTok)
		o.CacheWriteTokens = nullInt64Ptr(writeTok)
		o.ReasoningTokens = nullInt64Ptr(reasonTok)
		o.TotalTokens = nullInt64Ptr(totTok)
		settled := settledMap[o.Key]
		o.ThreadID = thread
		if settled != nil {
			o.ThreadID = settled.Event.Thread
			if rawThread == "" || settled.Event.Thread < rawThread {
				rawThread = settled.Event.Thread
			}
		}
		// A canonical public event need not use its original observation's
		// spelling. Accept a recovered spelling only under the stored digest.
		if d := cjsiObservationDigest(o); !bytes.Equal(d[:], obsDigest) {
			if err := tx.QueryRowContext(ctx, `SELECT thread_id FROM telemetry_observations WHERE observation_key=?`, o.Key).Scan(&o.ThreadID); err != nil {
				obsRows.Close()
				return nil, err
			}
		}
		d := cjsiObservationDigest(o)
		if !bytes.Equal(d[:], obsDigest) {
			obsRows.Close()
			return nil, errCJSIOTelPrecondition
		}
		disp, err := cjsiFromDBDisposition(rawDisp, settled != nil)
		if err != nil {
			obsRows.Close()
			return nil, err
		}
		receipts[o.Key] = CJSIOTelReceipt{Disposition: disp, Original: o, Settled: settled}
	}
	if err := obsRows.Close(); err != nil {
		return nil, err
	}
	if rawThread == "" {
		rawThread = thread
	}
	p := &CJSIOTelPeriod{
		Version:  version,
		Number:   periodNum,
		Thread:   thread,
		Opened:   opened,
		Support:  support,
		Closed:   closed,
		Requests: requests,
		Receipts: receipts,
	}
	var sess cjsiOTelSession
	sess.Client = "codex"
	sess.Thread = rawThread
	var sessRouteID sql.NullInt64
	var sessDigest []byte
	err = tx.QueryRowContext(ctx, `SELECT first_at, last_at, started_at, route_id, binding_digest
 FROM cjsi_receipt_sessions WHERE client='codex' AND thread=? AND period=?`, thread, periodNum).Scan(
		&sess.First, &sess.Last, &sess.Started, &sessRouteID, &sessDigest)
	if err == nil {
		p.Session = &sess
		p.SessionRouteID = sessRouteID
		p.SessionBindingDigest = sessDigest
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var marker cjsiOTelMarker
	marker.Client = "codex"
	marker.Thread = rawThread
	err = tx.QueryRowContext(ctx, `SELECT source_kind, model, first_at, last_at, support_entry, settled_at
 FROM cjsi_receipt_markers WHERE client='codex' AND thread=? AND period=?`, thread, periodNum).Scan(
		&marker.Kind, &marker.Model, &marker.First, &marker.Last, &marker.Support, &marker.Settled)
	if err == nil {
		p.Marker = &marker
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return p, nil
}

func canonicalCJSIThread(raw string) string {
	if c := ingest.CodexLegacySessionID(raw); c != "" {
		return c
	}
	return raw
}

type cjsiTelemetryClues struct {
	activeTakeovers      map[string]bool
	blockedThreads       map[string]bool
	blockedThreadDigests map[[32]byte]bool
	clearedHistorical    map[string]bool
	unboundedOrphan      bool
}

func (c cjsiTelemetryClues) isBlocked(canon string) bool {
	if c.unboundedOrphan || c.activeTakeovers[canon] || c.blockedThreads[canon] {
		return true
	}
	if len(c.blockedThreadDigests) > 0 {
		if c.blockedThreadDigests[sha256.Sum256([]byte(canon))] {
			return true
		}
	}
	return false
}

// usage_source_files has no client column. A checkpoint's session_id alone
// cannot distinguish Claude from Codex. Bind it through the published facts
// or, before publication, a Codex header or a canonical rollout name.
func cjsiCheckpointThreads(path string, thread, session string, codex, other bool) []string {
	id, rollout := CodexRolloutThreadID(path)
	if !codex && (other || (!rollout && thread == "")) {
		return nil
	}
	var ids []string
	for _, raw := range []string{id, thread, session} {
		if raw != "" {
			ids = append(ids, canonicalCJSIThread(raw))
		}
	}
	return ids
}

func readCJSITelemetryClues(ctx context.Context, q telemetryQueryer) (cjsiTelemetryClues, error) {
	c := cjsiTelemetryClues{
		activeTakeovers:      map[string]bool{},
		blockedThreads:       map[string]bool{},
		blockedThreadDigests: map[[32]byte]bool{},
		clearedHistorical:    map[string]bool{},
	}
	// 1. Active CJSI takeover periods.
	rows, err := q.QueryContext(ctx, `SELECT thread FROM cjsi_takeovers WHERE client='codex' AND closed_ticket IS NULL`)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var thread string
		if err := rows.Scan(&thread); err != nil {
			rows.Close()
			return c, err
		}
		c.activeTakeovers[canonicalCJSIThread(thread)] = true
	}
	if err := rows.Close(); err != nil {
		return c, err
	}
	// 2. Selected CJSI usage group assessments.
	rows, err = q.QueryContext(ctx, `SELECT s.thread, g.state, g.gaps, g.unresolved_overlap,
 EXISTS(SELECT 1 FROM cjsi_usage_sources u WHERE u.ticket=g.ticket AND u.attempt=g.attempt AND u.thread=g.thread)
 FROM cjsi_usage_selection s JOIN cjsi_usage_groups g USING(ticket, attempt, thread)`)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var thread string
		var state, gaps, unresolved int
		var hasSources bool
		if err := rows.Scan(&thread, &state, &gaps, &unresolved, &hasSources); err != nil {
			rows.Close()
			return c, err
		}
		canon := canonicalCJSIThread(thread)
		if state == int(ledger.NoLocalCall) && unresolved == 0 && gaps == 0 && !hasSources {
			c.clearedHistorical[canon] = true
		} else {
			c.blockedThreads[canon] = true
		}
	}
	if err := rows.Close(); err != nil {
		return c, err
	}
	// 3. Usage scope overlays (orphan blocks new promotion globally; thread blocks only that thread).
	rows, err = q.QueryContext(ctx, `SELECT scope_kind, scope_key FROM cjsi_scope_overlays
 WHERE domain='usage' AND (pending=1 OR masked=1)`)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var kind string
		var key []byte
		if err := rows.Scan(&kind, &key); err != nil {
			rows.Close()
			return c, err
		}
		switch kind {
		case "orphan":
			c.unboundedOrphan = true
		case "thread":
			if len(key) == 32 {
				var digest [32]byte
				copy(digest[:], key)
				c.blockedThreadDigests[digest] = true
			}
		}
	}
	if err := rows.Close(); err != nil {
		return c, err
	}
	// 4. Non-OTel Codex usage_events (potential overlap clues per thread; never silent retraction).
	rows, err = q.QueryContext(ctx, `SELECT DISTINCT session_id FROM usage_events WHERE client='codex' AND substr(source_path,1,5)<>'otel:'`)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var sessionID string
		if err := rows.Scan(&sessionID); err != nil {
			rows.Close()
			return c, err
		}
		if canon := canonicalCJSIThread(sessionID); canon != "" {
			c.blockedThreads[canon] = true
		}
	}
	if err := rows.Close(); err != nil {
		return c, err
	}
	// 5. Stored usage_source_files (per-thread clues; unbounded only when unassessed non-canonical with published Codex usage).
	rows, err = q.QueryContext(ctx, `SELECT f.path, f.thread_id, f.session_id,
 EXISTS(SELECT 1 FROM usage_events e WHERE e.source_path=f.path AND e.client='codex'),
 EXISTS(SELECT 1 FROM usage_events e WHERE e.source_path=f.path AND e.client<>'codex')
 FROM usage_source_files f WHERE substr(f.path,1,5)<>'otel:'`)
	if err != nil {
		return c, err
	}
	for rows.Next() {
		var path string
		var thread, sessionID sql.NullString
		var hasCodexFacts, otherClient bool
		if err := rows.Scan(&path, &thread, &sessionID, &hasCodexFacts, &otherClient); err != nil {
			rows.Close()
			return c, err
		}
		for _, thread := range cjsiCheckpointThreads(path, thread.String, sessionID.String, hasCodexFacts, otherClient) {
			c.blockedThreads[thread] = true
		}
		if _, named := CodexRolloutThreadID(path); !named && !thread.Valid && hasCodexFacts {
			c.unboundedOrphan = true
		}
	}
	if err := rows.Close(); err != nil {
		return c, err
	}
	// 6. Orphan Codex usage_events paths without a usage_source_files row.
	orphans, err := q.QueryContext(ctx, orphanCodexFactPathsSQL)
	if err != nil {
		return c, err
	}
	for orphans.Next() {
		var path string
		if err := orphans.Scan(&path); err != nil {
			orphans.Close()
			return c, err
		}
		if id, ok := CodexRolloutThreadID(path); ok {
			c.blockedThreads[id] = true
		} else {
			c.unboundedOrphan = true
		}
	}
	if err := orphans.Close(); err != nil {
		return c, err
	}
	return c, nil
}

// A previous complete inventory can clear unrelated threads while a new
// inventory is in progress. An unchecked prefix on the visible head vetoes
// that evidence only for threads to which the source can be linked.
type cjsiTelemetryInventory struct {
	completed time.Time
	unchecked map[string]bool
}

func readCJSITelemetryInventory(ctx context.Context, q telemetryQueryer) (cjsiTelemetryInventory, error) {
	inv := cjsiTelemetryInventory{unchecked: make(map[string]bool)}
	rows, err := q.QueryContext(ctx, `SELECT captured_at FROM inventory_runs WHERE state='complete'`)
	if err != nil {
		return inv, err
	}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			rows.Close()
			return inv, err
		}
		if at, err := time.Parse(time.RFC3339Nano, raw); err == nil && at.After(inv.completed) {
			inv.completed = at
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return inv, err
	}
	rows.Close()
	rows, err = q.QueryContext(ctx, `SELECT c.source_path, f.thread_id, f.session_id,
 EXISTS(SELECT 1 FROM usage_events e WHERE e.source_path=f.path AND e.client='codex'),
 EXISTS(SELECT 1 FROM usage_events e WHERE e.source_path=f.path AND e.client<>'codex')
 FROM domain_coverage_heads h JOIN source_domain_checkpoints c USING(inventory_id)
 LEFT JOIN usage_source_files f ON f.path=c.source_path AND f.identity=c.identity
 WHERE h.singleton=1 AND (c.state<>'complete' OR c.committed_end<>c.captured_end)`)
	if err != nil {
		return inv, err
	}
	defer rows.Close()
	for rows.Next() {
		var path string
		var thread, session sql.NullString
		var codex, other bool
		if err := rows.Scan(&path, &thread, &session, &codex, &other); err != nil {
			return inv, err
		}
		for _, id := range cjsiCheckpointThreads(path, thread.String, session.String, codex, other) {
			inv.unchecked[id] = true
		}
	}
	return inv, rows.Err()
}

func (inv cjsiTelemetryInventory) covers(thread string, received time.Time) bool {
	return !inv.unchecked[thread] && inv.completed.After(received)
}

func telemetryNeedsInventoryCJSI(ctx context.Context, tx *sql.Tx) (bool, error) {
	inv, err := readCJSITelemetryInventory(ctx, tx)
	if err != nil {
		return false, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT thread_id, received_at FROM telemetry_observations WHERE state='pending'`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var thread, raw string
		if err := rows.Scan(&thread, &raw); err != nil {
			return false, err
		}
		received, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil || !inv.covers(canonicalCJSIThread(thread), received) {
			return true, nil
		}
	}
	return false, rows.Err()
}

// settleTelemetryCJSI implements unique whole-thread OTel settlement on CJSI
// schema 37. Unlike legacy schema 36, bare headers, rollout filenames, cursors,
// non-OTel usage_events, or historical jsonl_owned rows never trigger silent
// retraction or globally freeze unrelated threads.
func settleTelemetryCJSI(ctx context.Context, tx *sql.Tx, now time.Time, settleAfter time.Duration) (TelemetrySettleResult, bool, error) {
	var result TelemetrySettleResult
	var err error
	result.Retracted, err = retractIncompleteTelemetry(ctx, tx, now, true)
	if err != nil {
		return result, false, err
	}
	result.Changed = result.Retracted > 0

	clues, err := readCJSITelemetryClues(ctx, tx)
	if err != nil {
		return result, false, err
	}
	if !clues.unboundedOrphan {
		if _, err := readmitTelemetry(ctx, tx, TelemetryReadmissionLimit); err != nil {
			return result, false, err
		}
	}

	inventory, err := readCJSITelemetryInventory(ctx, tx)
	if err != nil {
		return result, false, err
	}
	cutoff := now.Add(-settleAfter)

	// Only threads with settlement work enter the state machine. A metered
	// capture-only observation can still contradict a different UUID spelling
	// on one of those threads, even when it cannot itself be readmitted.
	active, err := queryStrings(ctx, tx, `SELECT DISTINCT thread_id FROM telemetry_observations WHERE thread_id<>'' AND state IN ('pending','otel_owned','jsonl_owned','conflict')`)
	if err != nil {
		return result, false, err
	}
	groups := make(map[string]int)
	for _, raw := range active {
		groups[canonicalCJSIThread(raw)] = 0
	}
	if len(groups) == 0 {
		return result, !clues.unboundedOrphan, nil
	}
	spellings, err := queryStrings(ctx, tx, `SELECT DISTINCT thread_id FROM telemetry_observations WHERE thread_id<>'' AND `+telemetryConflictEligible)
	if err != nil {
		return result, false, err
	}
	for _, raw := range spellings {
		thread := canonicalCJSIThread(raw)
		if _, active := groups[thread]; active {
			groups[thread]++
		}
	}
	anyUnsettled := clues.unboundedOrphan
	for _, thread := range slices.Sorted(maps.Keys(groups)) {
		match := cjsiOTelMatch(thread)
		if groups[thread] > 1 {
			// Re-evaluate conflicts persisted before ingestion matched UUID aliases.
			times, err := queryStrings(ctx, tx, `SELECT record_at FROM (
 SELECT DISTINCT record_at,input_tokens,output_tokens,cached_tokens,cache_write_tokens,reasoning_tokens,total_tokens
 FROM telemetry_observations WHERE thread_id`+match+` AND record_at<>'' AND `+telemetryConflictEligible+`)
 GROUP BY record_at HAVING count(*)>1
 INTERSECT SELECT record_at FROM telemetry_observations WHERE thread_id`+match+` AND `+telemetryConflictEligible+` AND state<>'conflict'`, thread, thread)
			if err != nil {
				return result, false, err
			}
			for _, at := range times {
				changed, err := retractConflicting(ctx, tx, thread, at, match)
				if err != nil {
					return result, false, err
				}
				if changed {
					result.Retracted++
					result.Changed = true
				}
			}
		}
		blocked := clues.isBlocked(thread) || inventory.unchecked[thread]
		// Inspect which states exist for this thread.
		var hasPending, hasOTel, hasJSONLOwned bool
		if err := tx.QueryRowContext(ctx, `SELECT
 EXISTS(SELECT 1 FROM telemetry_observations WHERE thread_id`+match+` AND state='pending'),
 EXISTS(SELECT 1 FROM telemetry_observations WHERE thread_id`+match+` AND state='otel_owned'),
 EXISTS(SELECT 1 FROM telemetry_observations WHERE thread_id`+match+` AND state='jsonl_owned')`,
			thread, thread, thread).Scan(&hasPending, &hasOTel, &hasJSONLOwned); err != nil {
			return result, false, err
		}

		// Handle historical jsonl_owned observations (no active takeover receipt).
		if hasJSONLOwned && !clues.activeTakeovers[thread] {
			if !blocked && clues.clearedHistorical[thread] {
				// Verified clearance proves historical suppression was false and
				// no JSONL source or overlap exists for this thread. Lift
				// suppression for admitted complete observations into pending
				// so normal settleAfter + post-receipt inventory rules apply.
				predicate, args := telemetryAdmissionSQL()
				qArgs := append([]any{thread}, args...)
				lifted, err := tx.ExecContext(ctx, `UPDATE telemetry_observations
 SET state='pending', reason='ownership_not_settled',
     support_entry = producer || '/' || producer_version || '/' || originator || '/' || transport
 WHERE thread_id`+match+` AND state='jsonl_owned' AND metered=1 AND input_tokens IS NOT NULL AND output_tokens IS NOT NULL AND `+predicate, qArgs...)
				if err != nil {
					return result, false, err
				}
				if n, _ := lifted.RowsAffected(); n > 0 {
					hasPending = true
					hasJSONLOwned = false
				}
			} else {
				// Without verified clearance (and without an active receipt),
				// historical jsonl_owned remains a thread-scoped unresolved clue.
				blocked = true
				anyUnsettled = true
			}
		}

		if blocked {
			if hasPending && !clues.activeTakeovers[thread] {
				anyUnsettled = true
			}
			// Preserve any already-settled OTel side untouched; never retract
			// without receipt-bearing PrepareCJSIOTel/Apply and never promote.
			continue
		}

		// Thread is unblocked: evaluate aged pending observations against post-receipt inventory.
		if hasPending && !inventory.completed.IsZero() {
			obsRows, err := tx.QueryContext(ctx, `SELECT observation_key, received_at FROM telemetry_observations
 WHERE thread_id`+match+` AND state='pending' AND input_tokens IS NOT NULL AND output_tokens IS NOT NULL ORDER BY observation_key`, thread)
			if err != nil {
				return result, false, err
			}
			var promoteKeys []string
			for obsRows.Next() {
				var key, rawReceived string
				if err := obsRows.Scan(&key, &rawReceived); err != nil {
					obsRows.Close()
					return result, false, err
				}
				received, err := time.Parse(time.RFC3339Nano, rawReceived)
				if err != nil {
					continue
				}
				if !received.After(cutoff) && inventory.covers(thread, received) {
					promoteKeys = append(promoteKeys, key)
				}
			}
			if err := obsRows.Close(); err != nil {
				return result, false, err
			}
			for _, key := range promoteKeys {
				res, err := tx.ExecContext(ctx, `UPDATE telemetry_observations SET state='otel_owned', reason='no_jsonl_evidence'
 WHERE observation_key=? AND state='pending'`, key)
				if err != nil {
					return result, false, err
				}
				if n, _ := res.RowsAffected(); n > 0 {
					result.Owned += int(n)
					result.Changed = true
					hasOTel = true
				}
			}
		}

		if hasOTel {
			restored, err := ensureContributions(ctx, tx, thread, match)
			if err != nil {
				return result, false, err
			}
			if restored > 0 {
				result.Restored += restored
				result.Changed = true
			}
			if err := syncTelemetrySession(ctx, tx, thread, match, now); err != nil {
				return result, false, err
			}
		}
	}
	return result, !anyUnsettled, nil
}

func telemetrySourceIdentityReadyCJSI(ctx context.Context, q telemetryQueryer) (bool, error) {
	clues, err := readCJSITelemetryClues(ctx, q)
	if err != nil {
		return false, err
	}
	if clues.unboundedOrphan {
		return false, nil
	}
	rows, err := q.QueryContext(ctx, `SELECT DISTINCT thread_id, state FROM telemetry_observations WHERE thread_id <> '' AND state IN ('pending','jsonl_owned')`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var thread, state string
		if err := rows.Scan(&thread, &state); err != nil {
			return false, err
		}
		canon := canonicalCJSIThread(thread)
		if clues.activeTakeovers[canon] {
			continue
		}
		if state == "jsonl_owned" {
			if !clues.clearedHistorical[canon] || clues.isBlocked(canon) {
				return false, nil
			}
			continue
		}
		if clues.isBlocked(canon) {
			return false, nil
		}
	}
	return true, rows.Err()
}
