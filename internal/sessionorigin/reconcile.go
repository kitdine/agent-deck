package sessionorigin

import (
	"context"
	"sort"
	"time"

	"github.com/kitdine/agent-deck/internal/sessionorigin/adapters/claudemem"
	"github.com/kitdine/agent-deck/internal/store"
)

// ReconcileOptions configures one classification pass.
type ReconcileOptions struct {
	// StateRoot locates the local origin-source configuration.
	StateRoot string
	// Home locates Claude Code's installed plugin catalog, through which the
	// claude-mem package version is confirmed before its data is adapted.
	Home string
	Now  func() time.Time
	// ReadClaudeMem replaces the claude-mem adapter in tests.
	ReadClaudeMem func(ctx context.Context, home, dataDir string, candidates []string) claudemem.Result
}

// ReconcileSummary reports one pass.
type ReconcileSummary struct {
	Sessions  int
	Changed   int
	Removed   int
	ClaudeMem *store.OriginAdapterRun
}

type originKey struct{ client, sessionID string }

// adapterFor is the one evidence source each client can have. Adapters never
// cross clients, so each client's derivation is replaced in one transaction.
func adapterFor(client string) string {
	switch client {
	case "codex":
		return SourceCodexOTelBackground
	case "claude":
		return SourceClaudeMem
	}
	return ""
}

// Reconcile classifies every canonical session of the session index in the
// existing scan/rebuild coordination phase. It reads the index's identity
// columns, the core OTel markers and, when explicitly configured and
// enabled, the claude-mem mapping; it writes only rows whose classification
// or evidence changed, so an unchanged pass publishes nothing. claude-mem's
// run summary is written with its evidence in one transaction. Scope
// switching never calls it.
func Reconcile(ctx context.Context, core, sessions *store.Store, options ReconcileOptions) (ReconcileSummary, error) {
	var summary ReconcileSummary
	now := time.Now().UTC()
	if options.Now != nil {
		now = options.Now().UTC()
	}
	observedAt := now.Format(time.RFC3339Nano)
	readClaudeMem := options.ReadClaudeMem
	if readClaudeMem == nil {
		readClaudeMem = claudemem.Read
	}

	sources, err := sessions.SessionOriginSources(ctx)
	if err != nil {
		return summary, err
	}
	generations := map[originKey][]SourceGeneration{}
	byClient := map[string][]string{}
	for _, source := range sources {
		key := originKey{source.Client, source.SessionID}
		if _, seen := generations[key]; !seen {
			byClient[source.Client] = append(byClient[source.Client], source.SessionID)
		}
		generations[key] = append(generations[key], SourceGeneration{Identity: source.Identity, PrefixHash: source.PrefixHash, ParserVersion: source.ParserVersion})
	}
	summary.Sessions = len(generations)

	stored, err := core.OriginDerivedState(ctx)
	if err != nil {
		return summary, err
	}
	storedOrigins := map[originKey]store.SessionOrigin{}
	for _, item := range stored.Origins {
		storedOrigins[originKey{item.Client, item.SessionID}] = item
	}
	storedEvidence := map[originKey][]store.OriginEvidence{}
	for _, item := range stored.Evidence {
		key := originKey{item.Client, item.SessionID}
		storedEvidence[key] = append(storedEvidence[key], item)
	}

	assessments := map[originKey]Assessment{}
	markers, err := core.TelemetrySessionOriginMarkers(ctx, "codex", byClient["codex"])
	if err != nil {
		return summary, err
	}
	for id, marker := range markers {
		assessments[originKey{"codex", id}] = AssessCodexMarker(marker)
	}

	var run *store.OriginAdapterRun
	config, configErr := LoadSourceConfig(options.StateRoot)
	switch {
	case configErr != nil:
		// The adapter's state cannot be confirmed: its evidence cannot be
		// trusted or refreshed, so its candidates are unknown, not work.
		for _, id := range byClient["claude"] {
			assessments[originKey{"claude", id}] = Assessment{Unavailable: RuleClaudeMemSDKMapV1}
		}
	case config.ClaudeMem.Enabled():
		result := readClaudeMem(ctx, options.Home, config.ClaudeMem.DataDir, byClient["claude"])
		run = &store.OriginAdapterRun{Adapter: SourceClaudeMem, Outcome: result.Outcome, FinishedAt: observedAt}
		if result.Reason != "" {
			reason := result.Reason
			run.Reason = &reason
		}
		if result.ValidatedVersion != "" {
			version := result.ValidatedVersion
			run.ValidatedVersion = &version
		}
		usable := result.Outcome == claudemem.OutcomeOK || result.Outcome == claudemem.OutcomePartial
		for _, id := range byClient["claude"] {
			key := originKey{"claude", id}
			if row, mapped := result.Mapped[id]; usable && mapped {
				assessments[key] = Assessment{Evidence: []Evidence{{Adapter: SourceClaudeMem, RuleVersion: RuleClaudeMemSDKMapV1,
					Purpose: PurposeAuxiliary, SourceToken: claudemem.SourceToken(row), ValidatedVersion: result.ValidatedVersion}}}
			} else if !usable || !result.Checked[id] {
				assessments[key] = Assessment{Unavailable: RuleClaudeMemSDKMapV1}
			}
		}
	}
	summary.ClaudeMem = run

	type change struct {
		ids      []string
		evidence []store.OriginEvidence
		origins  []store.SessionOrigin
	}
	changes := map[string]*change{}
	changeFor := func(client string) *change {
		if changes[client] == nil {
			changes[client] = &change{}
		}
		return changes[client]
	}
	for key, gens := range generations {
		assessment := assessments[key]
		result := Reduce(assessment)
		evidence := make([]store.OriginEvidence, 0, len(assessment.Evidence))
		for _, item := range assessment.Evidence {
			evidence = append(evidence, store.OriginEvidence{Adapter: item.Adapter, RuleVersion: item.RuleVersion, Client: key.client, SessionID: key.sessionID,
				SourceToken: item.SourceToken, Purpose: item.Purpose, ValidatedVersion: item.ValidatedVersion, ObservedAt: observedAt})
		}
		previous, had := storedOrigins[key]
		evidenceSame := keepObservedAt(evidence, storedEvidence[key])
		origin := store.SessionOrigin{Client: key.client, SessionID: key.sessionID, Automatic: result.Automatic, ReasonCode: result.ReasonCode,
			RuleVersion: result.RuleVersion, InputToken: InputToken(key.client, key.sessionID, gens, nil), EvidenceRevision: previous.EvidenceRevision}
		if !evidenceSame {
			origin.EvidenceRevision++
		}
		if had && evidenceSame && previous == origin {
			continue
		}
		c := changeFor(key.client)
		c.ids = append(c.ids, key.sessionID)
		c.evidence = append(c.evidence, evidence...)
		c.origins = append(c.origins, origin)
		summary.Changed++
	}
	// Sessions no longer indexed lose their derived rows; user overrides stay.
	removed := map[originKey]bool{}
	for key := range storedOrigins {
		if _, indexed := generations[key]; !indexed {
			removed[key] = true
		}
	}
	for key := range storedEvidence {
		if _, indexed := generations[key]; !indexed {
			removed[key] = true
		}
	}
	for key := range removed {
		c := changeFor(key.client)
		c.ids = append(c.ids, key.sessionID)
		summary.Removed++
	}
	if run != nil {
		changeFor("claude")
	}

	clients := make([]string, 0, len(changes))
	for client := range changes {
		clients = append(clients, client)
	}
	sort.Strings(clients)
	for _, client := range clients {
		c := changes[client]
		sort.Strings(c.ids)
		derivation := store.OriginDerivation{Adapter: adapterFor(client), Client: client, SessionIDs: c.ids, Evidence: c.evidence, Origins: c.origins}
		if client == "claude" {
			derivation.Run = run
		}
		if err := core.ReplaceOriginDerivation(ctx, derivation); err != nil {
			return summary, err
		}
	}
	return summary, nil
}

// keepObservedAt reports whether the evidence set equals the stored one,
// ignoring observation time, and if so keeps the stored observation times so
// an unchanged pass writes nothing.
func keepObservedAt(evidence []store.OriginEvidence, stored []store.OriginEvidence) bool {
	if len(evidence) != len(stored) {
		return false
	}
	previous := map[store.OriginEvidence]string{}
	for _, item := range stored {
		observed := item.ObservedAt
		item.ObservedAt = ""
		previous[item] = observed
	}
	times := make([]string, len(evidence))
	for index, item := range evidence {
		item.ObservedAt = ""
		observed, ok := previous[item]
		if !ok {
			return false
		}
		times[index] = observed
	}
	for index := range evidence {
		evidence[index].ObservedAt = times[index]
	}
	return true
}
