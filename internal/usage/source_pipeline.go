package usage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/perftrace"
	"github.com/kitdine/agent-deck/internal/store"
)

// writeTxRows and writeTxTime bound the work grouped into one write
// transaction: a group commits once it holds writeTxRows rows or has been open
// writeTxTime, since a source's fixed publication cost is not proportional to
// its rows. A source is never split beyond its blocks, so a larger source still
// commits alone.
const (
	writeTxRows = 512
	writeTxTime = 25 * time.Millisecond
)

// usageWriter publishes reduced sources in inventory order. Consecutive small
// sources share a transaction while their results are already reduced; each
// source runs under its own savepoint so a failing source rolls back alone and
// everything before it stays committed, as in the serial loop.
type usageWriter struct {
	s       *Service
	ctx     context.Context
	tx      *sql.Tx
	release func()
	rows    int
	started time.Time
	finish  func(...perftrace.Count)
}

func (w *usageWriter) begin() error {
	if w.tx != nil {
		return nil
	}
	finishWait := perftrace.Span(perftrace.UsagePublicationWait)
	tx, release, err := beginSourcePublication(w.ctx, w.s.Store.DB)
	finishWait()
	if err != nil {
		return err
	}
	w.tx, w.release, w.rows = tx, release, 0
	w.finish = perftrace.Span(perftrace.IngestWrite)
	w.started = time.Now()
	return nil
}

// commit ends the open transaction, if any.
func (w *usageWriter) commit() error {
	if w.tx == nil {
		return nil
	}
	finishCommit := perftrace.Span(perftrace.UsageCommit)
	err := w.tx.Commit()
	finishCommit()
	w.release()
	w.finish(perftrace.Count{Key: "rows", Value: int64(w.rows)})
	w.tx, w.release, w.finish = nil, nil, nil
	return err
}

// abort rolls back the open transaction, if any.
func (w *usageWriter) abort() {
	if w.tx == nil {
		return
	}
	_ = w.tx.Rollback()
	w.release()
	w.finish(perftrace.Count{Key: "rows", Value: 0})
	w.tx, w.release, w.finish = nil, nil, nil
}

func (w *usageWriter) savepoint(statement string) error {
	_, err := w.tx.ExecContext(w.ctx, statement)
	return err
}

// write publishes p and closes it. It reports whether everything written so
// far is committed.
func (w *usageWriter) write(p *preparedSource, nextReady bool) (bool, error) {
	defer p.close()
	if err := w.begin(); err != nil {
		return false, err
	}
	if err := w.savepoint("SAVEPOINT usage_source"); err != nil {
		return false, err
	}
	err := w.publish(p)
	if err != nil && w.tx != nil {
		if rollbackErr := w.savepoint("ROLLBACK TO usage_source"); rollbackErr != nil {
			return false, errors.Join(err, rollbackErr)
		}
	}
	if w.tx != nil {
		if releaseErr := w.savepoint("RELEASE usage_source"); err == nil {
			err = releaseErr
		}
	}
	if err != nil {
		return false, err
	}
	w.rows += p.rows()
	if w.rows < writeTxRows && nextReady && time.Since(w.started) < writeTxTime {
		return false, nil
	}
	return true, w.commit()
}

func (w *usageWriter) publish(p *preparedSource) error {
	current, err := w.s.checkpointCurrent(w.ctx, w.tx, p)
	if err != nil {
		return err
	}
	if !current {
		// An earlier source in this round published state this reduce read.
		// Recompute from the committed state; the shared stream is consumed,
		// so the source is read directly, once.
		if err = w.commit(); err != nil {
			return err
		}
		p.close()
		retry, prepareErr := w.s.prepareSource(w.ctx, p.entry, p.forceRebuild, nil)
		if prepareErr != nil {
			return prepareErr
		}
		defer retry.close()
		if err = w.begin(); err != nil {
			return err
		}
		if err = w.savepoint("SAVEPOINT usage_source"); err != nil {
			return err
		}
		*p = *retry
		retry.file = nil
		if current, err = w.s.checkpointCurrent(w.ctx, w.tx, p); err != nil {
			return err
		}
		if !current {
			return errUsageSourceChanged
		}
	}
	if p.unchanged {
		return w.s.commitUnchangedCoverageTx(w.ctx, w.tx, p.entry)
	}
	if err = p.validate(); err != nil {
		return err
	}
	if err = w.publishPrefix(p); err != nil {
		return err
	}
	return w.s.publishPrepared(w.ctx, w.tx, p)
}

// publishPrefix commits the facts of a large first import in transactions of
// at most writeTxRows rows, leaving the last block for publishPrepared. Those
// commits expose only a prefix of a source no query has seen before. The
// checkpoint row, signals, turn classification, sessions and coverage commit
// with the last block, so a crash leaves no checkpoint and the next scan
// imports the source again over the committed prefix. A rewritten or reset
// source is never split: its replacement must not become visible in parts.
func (w *usageWriter) publishPrefix(p *preparedSource) error {
	if p.found || p.reset || len(p.events)+len(p.tools) <= writeTxRows {
		return nil
	}
	if err := w.commit(); err != nil {
		return err
	}
	for len(p.events)+len(p.tools) > writeTxRows {
		events := p.events[:min(len(p.events), writeTxRows)]
		tools := p.tools[:min(len(p.tools), writeTxRows-len(events))]
		if err := w.begin(); err != nil {
			return err
		}
		if err := publishFacts(w.ctx, w.tx, p.entry.Path, true, events, tools, p.r); err != nil {
			w.abort()
			return err
		}
		w.rows = len(events) + len(tools)
		if err := w.commit(); err != nil {
			return err
		}
		p.events, p.tools = p.events[len(events):], p.tools[len(tools):]
	}
	if err := w.begin(); err != nil {
		return err
	}
	return w.savepoint("SAVEPOINT usage_source")
}

// scanSources reduces entries in parallel and publishes them in order. stats
// receives each source's counts after its publication.
func (s *Service) scanSources(ctx context.Context, entries []InventoryEntry, forceRebuild func(InventoryEntry) bool, stats func(map[string]int)) error {
	if len(entries) == 0 {
		return nil
	}
	options := ingest.OrderedOptions{Workers: ingest.ReduceWorkers()}
	if s.ReduceWorkers > 0 {
		options.Workers = s.ReduceWorkers
	}
	options.Window = 2 * options.Workers
	if s.Coordinator != nil {
		options.Budget = s.Coordinator
		// A failed source's stream is never drained, so the shared reader
		// would block on it while later reductions wait for that reader.
		options.Abort = func() { s.Coordinator.ConsumerDone(ingest.ConsumerUsage) }
	}
	writer := &usageWriter{s: s, ctx: ctx}
	return ingest.Ordered(ctx, len(entries), options,
		func(index int) ingest.Reduced[*preparedSource] {
			entry := entries[index]
			finish := perftrace.Span(perftrace.IngestReduce)
			p, err := s.prepareSource(ctx, entry, forceRebuild(entry), s.Coordinator)
			finish()
			if err != nil {
				return ingest.Reduced[*preparedSource]{Err: err}
			}
			p.forceRebuild = forceRebuild(entry)
			return ingest.Reduced[*preparedSource]{Value: p, Weight: p.weight(), Discard: p.close}
		},
		func(_ int, p *preparedSource, nextReady bool) (bool, error) {
			committed, err := writer.write(p, nextReady)
			if err == nil {
				stats(p.r)
			}
			return committed, err
		},
		writer.commit,
	)
}

// commitUnchangedCoverageTx is commitUnchangedSourcesCoverage for one entry
// inside the writer's transaction.
func (s *Service) commitUnchangedCoverageTx(ctx context.Context, tx *sql.Tx, entry InventoryEntry) error {
	if s.CoverageInventoryID == "" {
		return nil
	}
	var cursor int64
	if err := tx.QueryRowContext(ctx, `SELECT cursor FROM usage_source_files WHERE path=? AND identity=? AND size=? AND parser_version=? AND modified_at=? AND changed_at=?`, entry.Path, entry.Identity, entry.Size, ParserVersion, entry.ModifiedAt, entry.ChangedAt).Scan(&cursor); errors.Is(err, sql.ErrNoRows) {
		return errUsageSourceChanged
	} else if err != nil {
		return err
	}
	return store.CommitSourceCoverage(ctx, tx, s.CoverageInventoryID, entry.Path, entry.Identity, entry.Size, cursor)
}
