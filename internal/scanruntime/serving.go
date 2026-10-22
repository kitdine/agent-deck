package scanruntime

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

func acceptDomainInventory(ctx context.Context, db *store.Store, id string, sources []ingest.Source, sessions bool, at time.Time) error {
	var epoch int64
	var err error
	if sessions {
		epoch, err = db.SessionIndexEpoch(ctx)
	} else {
		var generation store.DerivedSnapshotGeneration
		generation, err = db.DerivedSnapshotGeneration(ctx)
		epoch = generation.Epoch
	}
	if err != nil {
		return err
	}
	marks := make([]store.InventorySource, 0, len(sources))
	for _, source := range sources {
		marks = append(marks, store.InventorySource{Path: source.Path, Identity: source.Identity, End: source.Size})
	}
	return db.AcceptInventory(ctx, id, epoch, marks, at)
}

func finishDomainInventory(db *store.Store, id string, cause error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	state := "failed"
	if errors.Is(cause, context.Canceled) {
		state = "paused"
	}
	_ = db.FinishInventory(ctx, id, state)
}

func finishAcceptedInventory(ctx context.Context, db *store.Store, id string) error {
	err := db.FinishInventory(ctx, id, "complete")
	if errors.Is(err, store.ErrCoverageIncomplete) {
		// A captured unfinished physical record keeps coverage partial without
		// changing the finite CLI obligation for complete records into failure.
		return db.FinishInventory(ctx, id, "partial")
	}
	return err
}

// Date and mtime are priority hints, not filters or event-time coverage proof.
func servingSourceBatches(sources []ingest.Source, now time.Time) [][]ingest.Source {
	local := now.In(time.Local)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 0, 1)
	priority, history := []ingest.Source{}, []ingest.Source{}
	for _, source := range sources {
		recent := source.ModifiedAt >= start.UnixNano()
		for day := start.UTC(); day.Before(end.UTC()); day = day.Add(24 * time.Hour) {
			recent = recent || strings.Contains(source.Path, "/"+day.Format("2006/01/02")+"/")
		}
		if recent {
			priority = append(priority, source)
		} else {
			history = append(history, source)
		}
	}
	batches := [][]ingest.Source{}
	for _, group := range [][]ingest.Source{priority, history} {
		for len(group) > 0 {
			n := min(len(group), 64)
			batches = append(batches, group[:n:n])
			group = group[n:]
		}
	}
	return batches
}

// The caller owns the scan lock and has sealed the accepted inventory.
func (r *round) executeServingDiscovered(ctx context.Context, stateRoot string, started time.Time, sources []ingest.Source) {
	fail := func(err error) {
		r.setUsage(failedUsage(err, time.Since(started).Milliseconds()))
		r.setSession(failedSession(err, time.Since(started).Milliseconds()))
	}
	core, err := store.Open(ctx, stateRoot)
	if err != nil {
		fail(err)
		return
	}
	defer core.Close()
	sessions, err := store.OpenSessions(ctx, stateRoot)
	if err != nil {
		fail(err)
		return
	}
	defer sessions.Close()
	for _, domain := range []struct {
		db      *store.Store
		session bool
	}{{core, false}, {sessions, true}} {
		if err = acceptDomainInventory(ctx, domain.db, r.id, sources, domain.session, started); err != nil {
			fail(err)
			return
		}
	}
	service := usage.New(core, r.home)
	if os.Getenv("AGENTDECK_SCAN_PROFILE") == "1" {
		service.Profile = &usage.ScanProfile{}
		defer func() {
			r.mu.Lock()
			r.result.Stages.UsageDetails = service.Profile.Report()
			r.mu.Unlock()
		}()
	}
	service.PreparedSources = sources
	fullInventory, err := service.Inventory(ctx)
	if err != nil {
		fail(err)
		return
	}
	entries := map[string]usage.InventoryEntry{}
	for _, entry := range fullInventory.Entries {
		entries[entry.Path] = entry
	}
	now := time.Now()
	if r.cacheNow != nil {
		now = r.cacheNow()
	}
	batches := servingSourceBatches(sources, now)
	metadataOnly := false
	if len(fullInventory.Added)+len(fullInventory.Appended)+len(fullInventory.Mutated)+len(fullInventory.Removed) == 0 {
		unchanged, checkErr := session.PreparedSourcesUnchanged(ctx, sessions.DB, r.home, sources)
		if checkErr != nil {
			finishDomainInventory(core, r.id, checkErr)
			finishDomainInventory(sessions, r.id, checkErr)
			fail(checkErr)
			return
		}
		if unchanged && len(sources) > 0 {
			batches = [][]ingest.Source{sources}
			metadataOnly = true
		}
	}
	changes := map[string]int{"files": len(sources)}
	if len(batches) == 0 {
		// No batch will call ScanInventory, but removed sources still require
		// detach/reconciliation before coverage can become complete 0/0.
		changes, err = service.ScanInventory(ctx, fullInventory)
	}
	scan := session.ScanResult{}
	r.setProgressStage("importing")
	for index, batch := range batches {
		if err = ctx.Err(); err != nil {
			break
		}
		if r.beforeServingBatch != nil {
			r.beforeServingBatch(index)
		}
		if err = ctx.Err(); err != nil {
			break
		}
		coordinator := ingest.NewCoordinator(batch, ingest.Options{RequirePlans: true})
		batchService := usage.New(core, r.home)
		batchService.Profile = service.Profile
		batchService.PreparedSources = batch
		batchService.Coordinator = coordinator
		batchService.CoverageInventoryID = r.id
		batchService.CoverageBatch = true
		// Planning discovers differences using the full accepted set. Only this
		// batch's plans must be sealed before its shared reader can start.
		_, err = batchService.PlanForCoordinator(ctx, coordinator)
		if err == nil {
			err = session.PlanForCoordinator(ctx, sessions.DB, r.home, batch, coordinator)
		}
		if err != nil {
			coordinator.Close()
			break
		}
		inventory := fullInventory
		inventory.Entries = make([]usage.InventoryEntry, 0, len(batch))
		for _, source := range batch {
			inventory.Entries = append(inventory.Entries, entries[source.Path])
		}
		if index != 0 {
			inventory.Removed = nil
		}
		coordinator.Start(ctx)
		var usageErr, sessionErr error
		var batchChanges map[string]int
		var batchScan session.ScanResult
		var wait sync.WaitGroup
		wait.Add(2)
		go func() {
			defer wait.Done()
			defer coordinator.ConsumerDone(ingest.ConsumerUsage)
			batchChanges, usageErr = batchService.ScanInventory(ctx, inventory)
		}()
		go func() {
			defer wait.Done()
			defer coordinator.ConsumerDone(ingest.ConsumerSession)
			batchScan, sessionErr = session.ScanWithOptions(ctx, sessions.DB, r.home, session.ScanOptions{PreparedSources: batch, Coordinator: coordinator, CoverageInventoryID: r.id, Batch: true})
		}()
		wait.Wait()
		coordinator.Close()
		if usageErr != nil {
			err = usageErr
			break
		}
		if sessionErr != nil {
			err = sessionErr
			break
		}
		for key, value := range batchChanges {
			if key != "files" {
				changes[key] += value
			}
		}
		scan.Sources += batchScan.Sources
		scan.Documents += batchScan.Documents
		scan.Skipped += batchScan.Skipped
		if metadataOnly {
			if err = finishAcceptedInventory(ctx, core, r.id); err == nil {
				err = finishAcceptedInventory(ctx, sessions, r.id)
			}
			if err != nil {
				break
			}
		}
		// Publish after this safe commit checkpoint, before any history body is
		// admitted. Neither a global terminal nor worker exit is required.
		publication, publishErr := (desktop.Service{StateRoot: stateRoot, Home: r.home, Now: r.cacheNow}).PublishServingSnapshot(ctx, desktop.Request{WireVersion: desktop.WireVersion, RecentLimit: desktop.DefaultRecentLimit}, true)
		if publishErr == nil {
			r.setServingPublication(publication)
		}
	}
	if err == nil {
		err = service.FinalizeInventory(ctx, fullInventory)
	}
	if err == nil {
		_, err = session.ScanWithOptions(ctx, sessions.DB, r.home, session.ScanOptions{PreparedSources: sources, CoverageInventoryID: r.id})
	}
	if err == nil && !metadataOnly {
		err = finishAcceptedInventory(ctx, core, r.id)
	}
	if err == nil && !metadataOnly {
		err = finishAcceptedInventory(ctx, sessions, r.id)
	}
	if err != nil {
		finishDomainInventory(core, r.id, err)
		finishDomainInventory(sessions, r.id, err)
		fail(err)
		return
	}
	r.setUsage(UsageResult{State: "completed", Changes: changes, DurationMS: time.Since(started).Milliseconds()})
	r.setSession(SessionResult{State: "completed", Scan: scan, DurationMS: time.Since(started).Milliseconds()})
	_ = (desktop.Service{StateRoot: stateRoot, Home: r.home, Now: r.cacheNow}).PublishDerivedSnapshotCache(ctx, desktop.WireVersion)
	publication, publishErr := (desktop.Service{StateRoot: stateRoot, Home: r.home, Now: r.cacheNow}).PublishServingSnapshot(ctx, desktop.Request{WireVersion: desktop.WireVersion, RecentLimit: desktop.DefaultRecentLimit}, false)
	if publishErr == nil {
		r.setServingPublication(publication)
	}
}

func (r *round) setServingPublication(snapshot desktop.ServingSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.publication = &snapshot
	r.progressSeq++
	r.progress.Sequence = r.progressSeq
}

func (r *round) registerOwner(id string, app bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !app {
		r.cliObligation = true
		return
	}
	if r.appOwners == nil {
		r.appOwners = map[string]int{}
	}
	r.appOwners[id]++
}

func (r *round) releaseAppOwner(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if count := r.appOwners[id]; count > 1 {
		r.appOwners[id]--
	} else {
		delete(r.appOwners, id)
	}
	if len(r.appOwners) == 0 && !r.cliObligation && r.cancelWork != nil {
		r.pausing = true
		r.cancelWork()
	}
}

func localServingRequest(ctx context.Context, stateRoot, home string, scope Scope, now func() time.Time, progress func(Progress), app bool, publication func(desktop.ServingSnapshot)) (Result, error) {
	if !app && publication == nil {
		return localRequest(ctx, stateRoot, home, scope, now, progress)
	}
	localRounds.Lock()
	r := localRounds.byState[stateRoot]
	id := newID()
	pausing := false
	if r != nil {
		r.mu.RLock()
		pausing = r.pausing
		r.mu.RUnlock()
	}
	if r == nil || r.completed() || pausing {
		r = newRound(home)
		r.serving = publication != nil
		r.cacheNow = now
		workCtx, cancel := context.WithCancel(context.Background())
		r.cancelWork = cancel
		r.registerOwner(id, app)
		localRounds.byState[stateRoot] = r
		go func() { defer cancel(); r.execute(workCtx, stateRoot, false) }()
	} else {
		r.mu.Lock()
		if publication != nil && !r.serving && r.observedInventory() {
			r.mu.Unlock()
			localRounds.Unlock()
			return Result{}, errors.New("active local scan cannot accept a serving request after inventory admission")
		}
		r.serving = r.serving || publication != nil
		r.mu.Unlock()
		r.registerOwner(id, app)
	}
	localRounds.Unlock()
	defer func() { localRounds.Lock(); defer localRounds.Unlock(); r.releaseAppOwner(id) }()
	last := ""
	return r.waitWithProgress(ctx, scope, func(value Progress) error {
		r.mu.RLock()
		latest := r.publication
		r.mu.RUnlock()
		if latest != nil && publication != nil && latest.Header.PublicationID != last {
			publication(*latest)
			last = latest.Header.PublicationID
		}
		if progress != nil {
			progress(value)
		}
		return nil
	})
}
