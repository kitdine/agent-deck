package session

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

// preparedUpdate is one session source's reduce result.
type preparedUpdate struct {
	src       source
	update    sourceUpdate
	changed   bool
	documents int
}

func (p preparedUpdate) rows() int {
	rows := 1
	for _, result := range p.update.results {
		rows += 1 + len(result.Documents)
	}
	return rows
}

func (p preparedUpdate) weight() int64 {
	var bytes int64
	for _, result := range p.update.results {
		for _, document := range result.Documents {
			bytes += int64(len(document.Text)) + 256
		}
		bytes += 512
	}
	return bytes
}

// sessionWriter publishes reduced sources in inventory order. Consecutive
// sources share a transaction while their results are already reduced; each
// runs under its own savepoint so a failing source rolls back alone and the
// sources before it stay committed, as in the serial loop.
type sessionWriter struct {
	ctx         context.Context
	db          *sql.DB
	inventoryID string
	tx          *sql.Tx
	rows        int
	started     time.Time
	finish      func(...perftrace.Count)
}

func (w *sessionWriter) begin() error {
	if w.tx != nil {
		return nil
	}
	tx, err := beginSessionTx(w.ctx, w.db)
	if err != nil {
		return err
	}
	w.tx, w.rows = tx, 0
	w.finish = perftrace.Span(perftrace.IngestWrite)
	w.started = time.Now()
	return nil
}

func (w *sessionWriter) commit() error {
	if w.tx == nil {
		return nil
	}
	err := w.tx.Commit()
	if err != nil {
		_ = w.tx.Rollback()
	}
	w.finish(perftrace.Count{Key: "rows", Value: int64(w.rows)})
	w.tx, w.finish = nil, nil
	return err
}

func (w *sessionWriter) exec(statement string) error {
	_, err := w.tx.ExecContext(w.ctx, statement)
	return err
}

func (w *sessionWriter) write(p *preparedUpdate, nextReady bool) (bool, error) {
	if err := w.begin(); err != nil {
		return false, err
	}
	if err := w.exec("SAVEPOINT session_source"); err != nil {
		return false, err
	}
	err := w.publish(p)
	if err != nil && w.tx != nil {
		if rollbackErr := w.exec("ROLLBACK TO session_source"); rollbackErr != nil {
			return false, errors.Join(err, rollbackErr)
		}
	}
	if w.tx != nil {
		if releaseErr := w.exec("RELEASE session_source"); err == nil {
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

func (w *sessionWriter) publish(p *preparedUpdate) error {
	current, err := checkpointCurrent(w.ctx, w.tx, p.update)
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
		update, changed, prepareErr := prepareSourceUpdate(w.ctx, w.db, p.src, nil)
		if prepareErr != nil {
			return prepareErr
		}
		p.update, p.changed = update, changed
		if err = w.begin(); err != nil {
			return err
		}
		if err = w.exec("SAVEPOINT session_source"); err != nil {
			return err
		}
		if current, err = checkpointCurrent(w.ctx, w.tx, p.update); err != nil {
			return err
		}
		if !current {
			return ingest.ErrSourceChanged
		}
	}
	if !p.changed && p.update.movedFrom == "" {
		return commitUnchangedSessionCoverageTx(w.ctx, w.tx, []source{p.src}, w.inventoryID)
	}
	if err = validateSourceUpdate(w.ctx, w.tx, p.update); err != nil {
		return err
	}
	prefix, err := w.publishPrefix(p)
	if err != nil {
		return err
	}
	docs, err := applySourceUpdate(w.ctx, w.tx, p.update)
	if err != nil {
		return err
	}
	p.documents = prefix + docs
	return store.CommitSourceCoverage(w.ctx, w.tx, w.inventoryID, p.src.path, p.src.identity, p.src.size, p.update.state.cursor-int64(len(p.update.state.partial)))
}

// publishPrefix commits the documents of a large new source in transactions
// of at most writeTxRows documents and leaves the rest to applySourceUpdate.
// Search reaches documents only through their source and metadata rows, which
// commit with the last block, so the prefix stays invisible; a crash leaves
// orphan documents that the next scan's orphan check deletes before importing
// the source again. Rewritten, moved and appended sources are never split.
func (w *sessionWriter) publishPrefix(p *preparedUpdate) (int, error) {
	update := p.update
	if !update.newSource || update.movedFrom != "" || p.rows() <= writeTxRows {
		return 0, nil
	}
	results := make([]Result, len(update.results))
	copy(results, update.results)
	pending := 0
	for i := range results {
		results[i].SourcePath = update.path
		excluded, err := excluded(w.ctx, w.tx, results[i].Metadata)
		if err != nil {
			return 0, err
		}
		if excluded {
			continue
		}
		pending += len(results[i].Documents)
	}
	if pending <= writeTxRows {
		return 0, nil
	}
	if err := w.commit(); err != nil {
		return 0, err
	}
	published := 0
	for i := range results {
		if pending <= writeTxRows {
			break
		}
		excluded, err := excluded(w.ctx, w.db, results[i].Metadata)
		if err != nil {
			return published, err
		}
		for !excluded && len(results[i].Documents) > 0 && pending > writeTxRows {
			block := results[i].Documents[:min(len(results[i].Documents), writeTxRows)]
			if err = w.begin(); err != nil {
				return published, err
			}
			if err = insertDocuments(w.ctx, w.tx, update.path, block); err != nil {
				w.abort()
				return published, err
			}
			w.rows = len(block)
			if err = w.commit(); err != nil {
				return published, err
			}
			results[i].Documents = results[i].Documents[len(block):]
			published += len(block)
			pending -= len(block)
		}
	}
	p.update.results = results
	if err := w.begin(); err != nil {
		return published, err
	}
	return published, w.exec("SAVEPOINT session_source")
}

// abort rolls back the open transaction, if any.
func (w *sessionWriter) abort() {
	if w.tx == nil {
		return
	}
	_ = w.tx.Rollback()
	w.finish(perftrace.Count{Key: "rows", Value: 0})
	w.tx, w.finish = nil, nil
}

// checkpointCurrent reports whether the committed source state update was
// reduced from is still what tx sees.
func checkpointCurrent(ctx context.Context, tx *sql.Tx, update sourceUpdate) (bool, error) {
	precondition := update.precondition
	state, found, err := loadSource(ctx, tx, update.path)
	if err != nil {
		return false, err
	}
	if update.movedFrom != "" {
		if found {
			return false, nil
		}
		state, found, err = loadSourceByIdentity(ctx, tx, precondition.identity)
	} else if !found && !precondition.found {
		state, found, err = loadSourceByIdentity(ctx, tx, precondition.identity)
	}
	if err != nil {
		return false, err
	}
	return found == precondition.found && (!found || sameSourceState(state, precondition.state)), nil
}

// scanSources reduces paths in parallel and publishes them in order; each
// receives every source's outcome after its publication.
func scanSources(ctx context.Context, db *sql.DB, paths []source, coordinator *ingest.Coordinator, inventoryID string, workers int, each func(bool, int)) error {
	if len(paths) == 0 {
		return nil
	}
	options := ingest.OrderedOptions{Workers: ingest.ReduceWorkers()}
	if workers > 0 {
		options.Workers = workers
	}
	options.Window = 2 * options.Workers
	if coordinator != nil {
		options.Budget = coordinator
		// A failed source's stream is never drained, so the shared reader
		// would block on it while later reductions wait for that reader.
		options.Abort = func() { coordinator.ConsumerDone(ingest.ConsumerSession) }
	}
	writer := &sessionWriter{ctx: ctx, db: db, inventoryID: inventoryID}
	return ingest.Ordered(ctx, len(paths), options,
		func(index int) ingest.Reduced[*preparedUpdate] {
			finish := perftrace.Span(perftrace.IngestReduce)
			update, changed, err := prepareSourceUpdate(ctx, db, paths[index], coordinator)
			finish()
			if err != nil {
				return ingest.Reduced[*preparedUpdate]{Err: err}
			}
			p := &preparedUpdate{src: paths[index], update: update, changed: changed}
			return ingest.Reduced[*preparedUpdate]{Value: p, Weight: p.weight()}
		},
		func(_ int, p *preparedUpdate, nextReady bool) (bool, error) {
			committed, err := writer.write(p, nextReady)
			if err == nil {
				each(p.changed, p.documents)
			}
			return committed, err
		},
		writer.commit,
	)
}
