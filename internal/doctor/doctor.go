// Package doctor coordinates read-only state diagnostics.
package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/extension"
	"github.com/kitdine/agent-deck/internal/hookrefusal"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/telemetry"
	"github.com/kitdine/agent-deck/internal/usage"
)

type Check struct {
	Name                string               `json:"name"`
	Status              string               `json:"status"`
	Code                string               `json:"code,omitempty"`
	Count               int                  `json:"count,omitempty"`
	SupportedCount      int                  `json:"supported_count,omitempty"`
	Recovery            string               `json:"recovery_command,omitempty"`
	Resource            string               `json:"resource,omitempty"`
	Reason              string               `json:"reason,omitempty"`
	ActionKind          ActionKind           `json:"action_kind,omitempty"`
	DiagnosticCommand   string               `json:"diagnostic_command,omitempty"`
	ManualPrerequisite  string               `json:"manual_prerequisite,omitempty"`
	RegistrationDetails *RegistrationDetails `json:"registration_details,omitempty"`
}

type ActionKind string

func (a ActionKind) MarshalJSON() ([]byte, error) {
	if a == "null" {
		return []byte("null"), nil
	}
	return json.Marshal(string(a))
}

type Report struct {
	Mode     string  `json:"mode"`
	Partial  bool    `json:"-"`
	Status   string  `json:"status"`
	Healthy  bool    `json:"healthy"`
	Checks   []Check `json:"checks"`
	Problems int     `json:"problems"`
	Warnings int     `json:"warnings"`
	Errors   int     `json:"errors"`
}

type Service struct {
	StateRoot         string
	Home              string
	Workdir           string
	Vault             provider.CredentialVault
	Now               func() time.Time
	LockProcessCheck  store.LockProcessCheck
	RegistrationProbe func(context.Context, bool) RegistrationDetails
	cjsiLayoutProbe   func(context.Context, string) (bool, error)
}

func (s Service) Check(ctx context.Context, full bool) (Report, error) {
	root, err := platform.GuardStateRoot(s.StateRoot)
	if err != nil {
		var refused *platform.ErrStateOwnershipRefused
		if errors.As(err, &refused) {
			return RefusedStateReport(full, refused.Reason), nil
		}
		return Report{}, err
	}
	s.StateRoot = root
	report := newReport(full)
	stateInfo, err := os.Stat(s.StateRoot)
	if errors.Is(err, fs.ErrNotExist) {
		report.Partial = true
		report.add(Check{Name: "state", Status: "warning", Code: "state_missing"})
		return report, nil
	}
	if err != nil {
		return Report{}, err
	}
	if stateInfo.Mode().Perm() != platform.DirectoryMode {
		report.add(Check{Name: "state_permissions", Status: "error", Code: "insecure_permissions"})
	} else {
		report.add(Check{Name: "state_permissions", Status: "ok"})
	}
	s.checkLock(&report)
	if refusal, live := hookrefusal.Live(s.StateRoot, store.CurrentSchemaVersion); live {
		report.add(Check{Name: "hook_deliveries", Status: "warning", Code: "hook_deliveries_dropped", Count: refusal.Count})
	}
	if check, ok := managedCommandOwnerCheck(s.Home); ok {
		report.add(check)
	}

	database, err := store.OpenReadOnly(ctx, s.StateRoot)
	if err != nil {
		if ctx.Err() != nil {
			return Report{}, ctx.Err()
		}
		report.Partial = true
		if errors.Is(err, store.ErrStateRestoreInProgress) {
			report.add(Check{Name: "database", Status: "error", Code: store.ErrStateRestoreInProgress.Code})
			return report, nil
		}
		if errors.Is(err, store.ErrStateCoreUninitialized) {
			report.add(Check{Name: "database", Status: "warning", Code: store.ErrStateCoreUninitialized.Code, Recovery: "agentdeck state migrate"})
			return report, nil
		}
		if errors.Is(err, store.ErrStateRootIdentityMismatch) {
			report.add(Check{Name: "database", Status: "error", Code: store.ErrStateRootIdentityMismatch.Code})
			return report, nil
		}
		var layout *store.CJSILayoutOpenError
		if errors.As(err, &layout) {
			if layout.Installing() {
				report.add(cjsiDatabaseFailure(nil))
			} else {
				report.add(cjsiDatabaseFailure(err))
			}
			return report, nil
		}
		var ahead *store.SchemaAhead
		check := Check{Name: "database", Status: "error", Code: databaseCode(err)}
		if errors.As(err, &ahead) {
			check.Count = ahead.Stored
			check.SupportedCount = ahead.Supported
		}
		report.add(check)
		return report, nil
	}
	defer database.Close()
	version, err := database.SchemaVersion(ctx)
	if err != nil {
		return Report{}, err
	}
	if version == cjsiCoreSchemaVersion {
		hasToolCalls, err := databaseHasTable(ctx, database, "usage_tool_calls")
		if err != nil || !hasToolCalls {
			report.Partial = true
			report.add(cjsiDatabaseFailure(err))
			return report, nil
		}
		if err := database.Close(); err != nil {
			return Report{}, err
		}
		if s.cjsiLayoutProbe != nil {
			_, err := s.cjsiLayoutProbe(ctx, s.StateRoot)
			if ctx.Err() != nil {
				return Report{}, ctx.Err()
			}
			if err != nil {
				report.Partial = true
				report.add(cjsiDatabaseFailure(err))
				return report, nil
			}
		}
		return s.checkCJSIDatabase(ctx, full, report)
	}
	hasToolCalls, schemaErr := databaseHasTable(ctx, database, "usage_tool_calls")
	if schemaErr != nil || (version == cjsiCoreSchemaVersion-1 && !hasToolCalls) {
		report.add(Check{Name: "schema", Status: "error", Code: "schema_incompatible", Count: version})
	} else {
		report.add(Check{Name: "schema", Status: "warning", Code: "schema_outdated", Count: version, SupportedCount: store.CurrentSchemaVersion, Recovery: "agentdeck state migrate"})
	}
	if full {
		integrity, err := database.IntegrityCheck(ctx)
		if err != nil {
			return Report{}, err
		}
		if integrity != "ok" {
			report.add(Check{Name: "database_integrity", Status: "error", Code: "integrity_failed"})
		} else {
			report.add(Check{Name: "database_integrity", Status: "ok"})
		}
	}
	operations, err := database.PendingOperations(ctx)
	if err != nil {
		return Report{}, err
	}
	if len(operations) > 0 {
		report.add(Check{Name: "pending_operations", Status: "warning", Code: "pending_operations", Count: len(operations), Recovery: "agentdeck provider recover"})
		providerUseDiagnostics := map[string]int{}
		for _, operation := range operations {
			if code := provider.DiagnoseProviderUseOperation(operation); code != "" {
				providerUseDiagnostics[code]++
			}
		}
		codes := make([]string, 0, len(providerUseDiagnostics))
		for code := range providerUseDiagnostics {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		for _, code := range codes {
			report.add(Check{Name: "provider_operation_state", Status: "warning", Code: code, Count: providerUseDiagnostics[code], Recovery: "agentdeck provider recover"})
		}
	} else {
		report.add(Check{Name: "pending_operations", Status: "ok"})
	}
	if err = s.checkProviders(ctx, database, &report, full); err != nil {
		return Report{}, err
	}
	s.checkProjectAttributionGate(ctx, database, &report)
	// Version labels describe a migration boundary, not the physical schema. A
	// partially restored database can claim schema 13 while lacking its new
	// table, so probe the read-only catalog before running table-specific SQL.
	if schemaErr != nil {
		report.add(Check{Name: "database", Status: "error", Code: "schema_incompatible"})
	} else if err = s.checkUsage(ctx, database, &report, hasToolCalls); err != nil {
		report.add(Check{Name: "usage", Status: "error", Code: "schema_incompatible"})
	}
	// Known capture gap: Codex Memories runs background requests that only
	// OTel records, and collection is off. Reported only when it applies.
	if config, err := telemetry.LoadConfig(s.StateRoot); err == nil && !config.Enabled && telemetry.ReadCodexFacts(s.Home, config.Port).NeedsCollection() {
		report.add(Check{Name: "telemetry", Status: "warning", Code: telemetry.WarningMemoriesNotCounted, Reason: telemetry.WarningMemoriesNotCounted, Recovery: "agentdeck telemetry enable --write-codex-config"})
	}
	sessionHealth, err := session.CheckHealth(ctx, s.StateRoot, full)
	var preflightErr error
	if err != nil || (sessionHealth.Present && (full || sessionHealth.Integrity != "ok")) {
		preflightErr = store.CheckLegacySessionIndexForMigration(ctx, s.StateRoot)
	}
	switch {
	case errors.Is(preflightErr, store.ErrLegacySessionIndexInvalid):
		report.add(Check{Name: "sessions", Status: "error", Code: "session_index_unhealthy", Recovery: "agentdeck session purge-index"})
	case preflightErr != nil:
		if canceled := ctx.Err(); canceled != nil {
			return Report{}, canceled
		}
		check := Check{Name: "sessions", Status: "error", Code: databaseCode(preflightErr)}
		if errors.Is(preflightErr, store.ErrStateBusy) {
			check.Status, check.Code = "warning", store.ErrStateBusy.Code
		}
		report.add(check)
		if full && err == nil {
			addSessionSourceHealth(&report, sessionHealth.UnreadableSources)
		}
	case err != nil:
		return Report{}, err
	default:
		addSessionHealth(&report, sessionHealth, full)
	}
	extensionReport, err := extension.Doctor(ctx, database, s.Home, s.Workdir)
	if err != nil {
		var inventoryUnreadable *extension.ErrExtensionInventoryUnreadable
		if errors.As(err, &inventoryUnreadable) {
			report.add(Check{
				Name:               "extensions",
				Status:             "error",
				Code:               "extension_inventory_unreadable",
				Resource:           "extension_inventory",
				Reason:             "extension_inventory_unreadable",
				ActionKind:         "manual_prerequisite",
				ManualPrerequisite: "prereq_extension_inventory_unreadable",
			})
			return report, nil
		}
		return Report{}, err
	}
	if extensionReport.Reason != "" {
		recovery := ""
		if extensionReport.RecoveryCommand != nil {
			recovery = *extensionReport.RecoveryCommand
		}
		diagCmd := ""
		if recovery == "" {
			diagCmd = "agentdeck extension doctor"
		}
		manualPrereq := ""
		if extensionReport.ManualPrerequisite != nil {
			manualPrereq = *extensionReport.ManualPrerequisite
		}
		report.add(Check{
			Name:               "extensions",
			Status:             "warning",
			Code:               extensionReport.Reason,
			Count:              extensionReport.CountForReason(),
			Recovery:           recovery,
			Resource:           "extension_inventory",
			Reason:             extensionReport.Reason,
			ActionKind:         ActionKind(extensionReport.ActionKind),
			DiagnosticCommand:  diagCmd,
			ManualPrerequisite: manualPrereq,
		})
	} else {
		report.add(Check{Name: "extensions", Status: "ok"})
	}
	probe := s.RegistrationProbe
	if probe == nil {
		probe = probeRegistration
	}
	report.add(boundedRegistrationCheck(ctx, full, probe))
	return report, nil
}

func addSessionHealth(report *Report, sessionHealth session.Health, full bool) {
	if !sessionHealth.Present {
		report.add(Check{Name: "sessions", Status: "warning", Code: "session_index_missing", Recovery: "agentdeck session rebuild"})
	} else if !sessionHealth.FTSAvailable || sessionHealth.Integrity != "ok" {
		report.add(Check{Name: "sessions", Status: "error", Code: "session_index_unhealthy", Recovery: "agentdeck session rebuild"})
	} else {
		report.add(Check{Name: "sessions", Status: "ok"})
	}
	if full {
		addSessionSourceHealth(report, sessionHealth.UnreadableSources)
	}
}

func addSessionSourceHealth(report *Report, unreadable int) {
	if unreadable > 0 {
		report.add(Check{Name: "session_sources", Status: "warning", Code: "session_source_unreadable", Count: unreadable, Recovery: "agentdeck session rebuild"})
	} else {
		report.add(Check{Name: "session_sources", Status: "ok"})
	}
}

func (s Service) checkProjectAttributionGate(
	ctx context.Context,
	database *store.Store,
	report *Report,
) {
	status, err := (provider.Service{
		Store:     database,
		StateRoot: s.StateRoot,
	}).ProjectAttributionGateStatus(ctx)
	if err != nil {
		report.add(Check{
			Name:     "project_attribution_gate",
			Status:   "warning",
			Code:     "project_attribution_gate_unreadable",
			Recovery: "rerun the intended agentdeck provider use command",
		})
		return
	}
	if status.Consistent {
		report.add(Check{Name: "project_attribution_gate", Status: "ok"})
		return
	}
	code := "project_attribution_gate_stale"
	if status.Required {
		code = "project_attribution_gate_missing"
	}
	report.add(Check{
		Name:     "project_attribution_gate",
		Status:   "warning",
		Code:     code,
		Recovery: "rerun the intended agentdeck provider use command",
	})
}

// lockRows are the doctor's lock checks in report order: check name, lock file
// and the resource it guards.
var lockRows = [][3]string{{"state_lock", "state.lock", "state"}, {"scan_lock", "scan.lock", "scan"}}

func (s Service) checkLock(report *Report) {
	for _, row := range lockRows {
		s.checkNamedLock(report, row[0], row[1], row[2])
	}
}

// LockCheck re-evaluates one lock row ("state_lock" or "scan_lock") as Check
// would now. Lock ownership changes without any data change, so consumers of
// persisted health refresh these rows instead of trusting a past observation.
func (s Service) LockCheck(name string) (Check, bool) {
	for _, row := range lockRows {
		if row[0] == name {
			root, err := platform.GuardStateRoot(s.StateRoot)
			if err != nil {
				return Check{}, false
			}
			s.StateRoot = root
			var report Report
			s.checkNamedLock(&report, row[0], row[1], row[2])
			return report.Checks[0], true
		}
	}
	return Check{}, false
}

func (s Service) checkNamedLock(report *Report, checkName, fileName, resource string) {
	path := filepath.Join(s.StateRoot, fileName)
	reason, owned, err := store.ClassifyLockOwnership(path, s.LockProcessCheck)
	if errors.Is(err, fs.ErrNotExist) {
		report.add(Check{Name: checkName, Status: "ok"})
		return
	}
	if reason == store.LockReasonLive && owned {
		// The diagnosing process holds this lock itself, as a scan worker
		// does while it publishes: there is no other owner to wait for.
		report.add(Check{Name: checkName, Status: "ok"})
		return
	}
	switch reason {
	case store.LockReasonLive:
		report.add(Check{
			Name:       checkName,
			Status:     "warning",
			Code:       string(reason),
			Resource:   resource,
			Reason:     string(reason),
			ActionKind: "retry",
		})
	case store.LockReasonLegacy:
		report.add(Check{
			Name:               checkName,
			Status:             "warning",
			Code:               string(reason),
			Resource:           resource,
			Reason:             string(reason),
			ActionKind:         "manual_prerequisite",
			ManualPrerequisite: "prereq_legacy_lock_removal",
		})
	case store.LockReasonOwnerUnknown:
		report.add(Check{
			Name:       checkName,
			Status:     "warning",
			Code:       string(reason),
			Resource:   resource,
			Reason:     string(reason),
			ActionKind: "null",
		})
	case store.LockReasonReclaimable:
		report.add(Check{
			Name:     checkName,
			Status:   "ok",
			Code:     string(reason),
			Resource: resource,
			Reason:   string(reason),
		})
	default:
		report.add(Check{
			Name:     checkName,
			Status:   "warning",
			Code:     string(store.LockReasonOwnerUnknown),
			Resource: resource,
			Reason:   string(store.LockReasonOwnerUnknown),
		})
	}
}

func (s Service) checkProviders(ctx context.Context, database *store.Store, report *Report, full bool) error {
	service := provider.Service{Store: database, Vault: s.Vault}
	statuses, err := service.Status(ctx)
	if err != nil {
		return err
	}
	missing := 0
	for _, status := range statuses {
		if status.Definition.BuiltIn {
			continue
		}
		if len(status.Credentials) == 0 {
			missing++
			continue
		}
		for _, credential := range status.Credentials {
			if !credential.Present {
				missing++
			}
		}
	}
	if missing > 0 {
		report.add(Check{Name: "provider_credentials", Status: "error", Code: "credential_missing", Count: missing, Recovery: "agentdeck credential add"})
	} else {
		report.add(Check{Name: "provider_credentials", Status: "ok"})
	}
	credentials, listErr := database.ListProviderCredentials(ctx, "")
	if listErr != nil {
		return listErr
	}
	type ownedSecret struct {
		providerName   string
		credentialName string
		reference      string
		sealed         credentialvault.Sealed
		loaded         bool
		problemCode    string
	}
	owned := make([]ownedSecret, 0, len(credentials))
	quickProblems := credentialProblems{}
	var orphanSecrets int
	if orphanErr := database.DB.QueryRowContext(ctx, `SELECT count(*) FROM credential_secrets cs LEFT JOIN provider_credentials pc ON pc.id=cs.credential_id WHERE pc.id IS NULL`).Scan(&orphanSecrets); orphanErr != nil {
		return orphanErr
	}
	if orphanSecrets > 0 {
		quickProblems.add(credentialvault.ErrCiphertextInvalid.Error(), "", orphanSecrets)
	}
	secretRows := orphanSecrets
	for _, credential := range credentials {
		if !credential.SecretPresent {
			continue
		}
		secretRows++
		item := ownedSecret{providerName: credential.ProviderName, credentialName: credential.Name, reference: credential.CredentialRef}
		secret, secretErr := database.CredentialSecret(ctx, credential.ID)
		if secretErr != nil {
			item.problemCode = credentialvault.ErrCiphertextInvalid.Error()
			owned = append(owned, item)
			continue
		}
		item.loaded = true
		item.sealed = credentialvault.Sealed{Algorithm: secret.Algorithm, KeyVersion: secret.KeyVersion, KeyID: secret.KeyID, Nonce: secret.Nonce, Ciphertext: secret.Ciphertext}
		switch {
		case item.sealed.Algorithm != credentialvault.AlgorithmAES256GCM || !credentialvault.IsSupportedKeyVersion(item.sealed.KeyVersion):
			item.problemCode = credentialvault.ErrKeyVersionUnsupported.Error()
		case len(item.sealed.Nonce) != 12 || len(item.sealed.Ciphertext) < 16:
			item.problemCode = credentialvault.ErrCiphertextInvalid.Error()
		}
		owned = append(owned, item)
	}
	authCandidates := make([]ownedSecret, 0, len(owned))
	keyReady, rotationReady := false, false
	var liveKeyIDs map[int]string
	if secretRows > 0 {
		if s.Vault == nil {
			quickProblems.add(credentialvault.ErrKeyMissing.Error(), "", secretRows)
		} else {
			var inspectErr error
			liveKeyIDs, inspectErr = s.Vault.InspectKeyIDs(ctx)
			if inspectErr != nil {
				quickProblems.add(credentialErrorCode(inspectErr), "", secretRows)
			} else {
				keyReady = true
				storedKeys, keyIDsErr := database.CredentialSecretKeys(ctx)
				if keyIDsErr != nil {
					return keyIDsErr
				}
				rotationReady = len(storedKeys) > 0
				for _, storedKey := range storedKeys {
					expectedKeyID, supported := liveKeyIDs[storedKey.KeyVersion]
					if !supported {
						continue
					}
					if storedKey.KeyID == "" || storedKey.KeyID != expectedKeyID {
						rotationReady = false
						break
					}
				}
			}
		}
	}
	for _, item := range owned {
		if item.problemCode != "" {
			recovery := ""
			if item.problemCode == credentialvault.ErrCiphertextInvalid.Error() && rotationReady {
				recovery = fmt.Sprintf("agentdeck credential update %s --credential %s --rotate", item.providerName, item.credentialName)
			}
			quickProblems.add(item.problemCode, recovery, 1)
		}
		if !keyReady || !item.loaded {
			continue
		}
		if !credentialvault.IsSupportedKeyVersion(item.sealed.KeyVersion) {
			continue
		}
		expectedKeyID := liveKeyIDs[item.sealed.KeyVersion]
		if item.sealed.KeyID == "" || item.sealed.KeyID != expectedKeyID {
			quickProblems.add(credentialvault.ErrKeyMachineMismatch.Error(), "", 1)
			continue
		}
		if item.problemCode == "" {
			authCandidates = append(authCandidates, item)
		}
	}
	addCredentialProblems(report, "provider_credential_key", quickProblems)
	if full && keyReady && len(authCandidates) > 0 {
		authProblems := credentialProblems{}
		for _, item := range authCandidates {
			if _, openErr := s.Vault.Open(ctx, item.reference, item.sealed); openErr != nil {
				code := credentialErrorCode(openErr)
				recovery := ""
				if code == credentialvault.ErrCiphertextInvalid.Error() && rotationReady {
					recovery = fmt.Sprintf("agentdeck credential update %s --credential %s --rotate", item.providerName, item.credentialName)
				}
				authProblems.add(code, recovery, 1)
			}
		}
		addCredentialProblems(report, "provider_credential_authentication", authProblems)
	}
	drift, err := service.ConfigDrift(ctx, s.Home)
	if err != nil {
		return err
	}
	if drift > 0 {
		report.add(Check{Name: "provider_configuration", Status: "warning", Code: "provider_config_drift", Count: drift, Recovery: "agentdeck provider use"})
	} else {
		report.add(Check{Name: "provider_configuration", Status: "ok"})
	}
	return nil
}

func credentialErrorCode(err error) string {
	for _, target := range []error{
		credentialvault.ErrKeyMissing,
		credentialvault.ErrKeyPermissions,
		credentialvault.ErrKeyMachineMismatch,
		credentialvault.ErrKeyVersionUnsupported,
		credentialvault.ErrCiphertextInvalid,
		credentialvault.ErrMachineIdentityMissing,
	} {
		if errors.Is(err, target) {
			return target.Error()
		}
	}
	return credentialvault.ErrCiphertextInvalid.Error()
}

type credentialProblem struct {
	code     string
	recovery string
}

type credentialProblems map[credentialProblem]int

func (problems credentialProblems) add(code, recovery string, count int) {
	problems[credentialProblem{code: code, recovery: recovery}] += count
}

func addCredentialProblems(report *Report, name string, problems credentialProblems) {
	if len(problems) == 0 {
		report.add(Check{Name: name, Status: "ok"})
		return
	}
	ordered := make([]credentialProblem, 0, len(problems))
	for problem := range problems {
		ordered = append(ordered, problem)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].code == ordered[j].code {
			return ordered[i].recovery < ordered[j].recovery
		}
		return ordered[i].code < ordered[j].code
	})
	for _, problem := range ordered {
		report.add(Check{Name: name, Status: "error", Code: problem.code, Count: problems[problem], Recovery: problem.recovery})
	}
}

func databaseHasTable(ctx context.Context, database *store.Store, name string) (bool, error) {
	var count int
	err := database.DB.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&count)
	return count == 1, err
}

func (s Service) checkUsage(ctx context.Context, database *store.Store, report *Report, hasToolCalls bool) error {
	service := usage.New(database, s.Home)
	var incomplete int
	if err := database.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_runs WHERE ended_at IS NULL").Scan(&incomplete); err != nil {
		return err
	}
	if incomplete > 0 {
		report.add(Check{Name: "usage", Status: "warning", Code: "incomplete_runs", Count: incomplete, Recovery: "agentdeck usage diagnose"})
	} else {
		report.add(Check{Name: "usage", Status: "ok"})
	}
	if report.Mode == "full" {
		unreadable, err := service.CheckSourceReadability(ctx)
		if err != nil {
			return err
		}
		if unreadable > 0 {
			report.add(Check{Name: "usage_sources", Status: "warning", Code: "usage_source_unreadable", Count: unreadable, Recovery: "agentdeck usage rebuild"})
		} else {
			report.add(Check{Name: "usage_sources", Status: "ok"})
		}
	}
	prices, err := service.PriceStatus(ctx)
	if err != nil {
		return err
	}
	available, _ := prices["available"].(bool)
	if !available {
		report.add(Check{Name: "prices", Status: "warning", Code: "price_catalog_missing", Recovery: "agentdeck price status"})
	} else {
		report.add(Check{Name: "prices", Status: "ok"})
	}
	if report.Mode != "full" {
		return nil
	}
	invalidProvenance, unpricedModels, err := service.PriceDiagnostics(ctx)
	if err != nil {
		return err
	}
	if invalidProvenance > 0 {
		report.add(Check{Name: "price_provenance", Status: "warning", Code: "price_provenance_invalid", Count: invalidProvenance, Recovery: "agentdeck price status"})
	} else {
		report.add(Check{Name: "price_provenance", Status: "ok"})
	}
	if unpricedModels > 0 {
		report.add(Check{Name: "unpriced_models", Status: "warning", Code: "unpriced_models", Count: unpricedModels, Recovery: "agentdeck price status"})
	} else {
		report.add(Check{Name: "unpriced_models", Status: "ok"})
	}
	return nil
}

func databaseCode(err error) string {
	if errors.Is(err, store.ErrStateRestoreInProgress) {
		return store.ErrStateRestoreInProgress.Code
	}
	if errors.Is(err, store.ErrSchemaAhead) {
		return store.ErrSchemaAhead.Code
	}
	if errors.Is(err, store.ErrUnknownSchema) {
		return store.ErrUnknownSchema.Code
	}
	return "database_unreadable"
}

func (r *Report) add(check Check) {
	r.Checks = append(r.Checks, check)
	if check.Status != "ok" {
		r.Healthy = false
		r.Problems++
		if check.Status == "error" {
			r.Errors++
			r.Status = "unhealthy"
		} else {
			r.Warnings++
			if r.Status != "unhealthy" {
				r.Status = "degraded"
			}
		}
	}
}
