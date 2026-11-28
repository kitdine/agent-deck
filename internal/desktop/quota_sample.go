package desktop

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

func (s Service) SampleQuota(ctx context.Context, core *store.Store, lockTimeout time.Duration, action quota.SampleAction, client quota.Client, sessionID string) (quota.SampleResult, error) {
	sampler := quota.Sampler{
		StateRoot: s.StateRoot, LockTimeout: lockTimeout,
		Scheduler: quota.Scheduler{
			Store: quota.NewWritableStore(core), Now: s.now,
			ProbeCodex: s.QuotaProbeCodex, ProbeClaudeProse: s.QuotaProbeClaudeProse,
		},
		Gate: quotaSampleGate,
		UsagePending: func(ctx context.Context, tx *sql.Tx, client quota.Client, sessionID string, lastCallAt time.Time) (bool, error) {
			return usage.QuotaSampleUsagePending(ctx, tx, string(client), sessionID, lastCallAt)
		},
	}
	return sampler.Run(ctx, action, client, sessionID)
}

func quotaSampleGate(ctx context.Context, tx *sql.Tx, client quota.Client, probeStartedAt time.Time) (quota.SampleGate, error) {
	settings, err := quota.LoadSettings(ctx, store.TransactionSettings{Tx: tx})
	if err != nil {
		return quota.SampleGate{}, err
	}
	gate := quota.SampleGate{ProbeEnabled: settings.ProbeEnabled && settings.ClientEnabled(client)}
	selection, err := store.CurrentProviderSnapshotTx(ctx, tx, string(client))
	if errors.Is(err, sql.ErrNoRows) {
		return gate, nil
	}
	if err != nil {
		return gate, err
	}
	gate.RecordedOfficial = !selection.SelectedAt.IsZero() && strings.EqualFold(selection.Name, provider.OfficialProviderName)
	gate.SelectedAt = selection.SelectedAt
	gate.RouteChanged = selection.Name != "" && !gate.RecordedOfficial && !gate.SelectedAt.IsZero()
	if gate.RouteChanged || !gate.RecordedOfficial || gate.SelectedAt.IsZero() {
		return gate, nil
	}

	observed, observedAt, ok, err := usage.LatestObservedProviderTx(ctx, tx, string(client))
	if err != nil {
		return gate, err
	}
	if ok && !observedAt.IsZero() && !observedAt.Before(gate.SelectedAt) {
		gate.ObservedKnown = true
		gate.ObservedOfficial = strings.EqualFold(observed, provider.OfficialProviderName)
	}
	if !probeStartedAt.IsZero() && (!gate.ObservedKnown || gate.ObservedOfficial) {
		// A later official observation does not erase affirmative evidence
		// that routing was non-official during this probe.
		gate.RouteChanged, err = usage.ObservedNonOfficialSinceTx(ctx, tx, string(client), probeStartedAt)
	}
	return gate, err
}
