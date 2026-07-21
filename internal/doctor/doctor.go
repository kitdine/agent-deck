// Package doctor coordinates read-only state diagnostics.
package doctor

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/extension"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

type Check struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Code     string `json:"code,omitempty"`
	Count    int    `json:"count,omitempty"`
	Recovery string `json:"recovery_command,omitempty"`
}

type Report struct {
	Mode     string  `json:"mode"`
	Status   string  `json:"status"`
	Healthy  bool    `json:"healthy"`
	Checks   []Check `json:"checks"`
	Problems int     `json:"problems"`
	Warnings int     `json:"warnings"`
	Errors   int     `json:"errors"`
}

type Service struct {
	StateRoot string
	Home      string
	Workdir   string
	Vault     provider.CredentialVault
	Now       func() time.Time
}

func (s Service) Check(ctx context.Context, full bool) (Report, error) {
	report := Report{Mode: map[bool]string{false: "quick", true: "full"}[full], Status: "healthy", Healthy: true, Checks: []Check{}}
	stateInfo, err := os.Stat(s.StateRoot)
	if errors.Is(err, fs.ErrNotExist) {
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

	database, err := store.OpenReadOnly(ctx, s.StateRoot)
	if err != nil {
		report.add(Check{Name: "database", Status: "error", Code: databaseCode(err)})
		return report, nil
	}
	defer database.Close()
	version, err := database.SchemaVersion(ctx)
	if err != nil {
		return Report{}, err
	}
	hasToolCalls, schemaErr := databaseHasTable(ctx, database, "usage_tool_calls")
	if schemaErr != nil {
		report.add(Check{Name: "schema", Status: "error", Code: "schema_incompatible", Count: version})
	} else if version == store.CurrentSchemaVersion && !hasToolCalls {
		report.add(Check{Name: "schema", Status: "error", Code: "schema_incompatible", Count: version})
	} else if version != store.CurrentSchemaVersion {
		report.add(Check{Name: "schema", Status: "warning", Code: "schema_outdated", Count: version, Recovery: "agentdeck state migrate"})
	} else {
		report.add(Check{Name: "schema", Status: "ok", Count: version})
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
	// Version labels describe a migration boundary, not the physical schema. A
	// partially restored database can claim schema 13 while lacking its new
	// table, so probe the read-only catalog before running table-specific SQL.
	if schemaErr != nil {
		report.add(Check{Name: "database", Status: "error", Code: "schema_incompatible"})
	} else if err = s.checkUsage(ctx, database, &report, hasToolCalls); err != nil {
		report.add(Check{Name: "usage", Status: "error", Code: "schema_incompatible"})
	}
	sessionHealth, err := session.CheckHealth(ctx, s.StateRoot, full)
	if err != nil {
		return Report{}, err
	}
	if !sessionHealth.Present {
		report.add(Check{Name: "sessions", Status: "warning", Code: "session_index_missing", Recovery: "agentdeck session rebuild"})
	} else if !sessionHealth.FTSAvailable || sessionHealth.Integrity != "ok" {
		report.add(Check{Name: "sessions", Status: "error", Code: "session_index_unhealthy", Recovery: "agentdeck session rebuild"})
	} else {
		report.add(Check{Name: "sessions", Status: "ok"})
	}
	if full {
		if sessionHealth.UnreadableSources > 0 {
			report.add(Check{Name: "session_sources", Status: "warning", Code: "session_source_unreadable", Count: sessionHealth.UnreadableSources, Recovery: "agentdeck session rebuild"})
		} else {
			report.add(Check{Name: "session_sources", Status: "ok"})
		}
	}
	extensionReport, err := extension.Doctor(ctx, database, s.Home, s.Workdir)
	if err != nil {
		return Report{}, err
	}
	extensionProblems := len(extensionReport.Diagnostics) + len(extensionReport.MissingPaths) + len(extensionReport.DuplicateIDs) + len(extensionReport.DriftedIDs) + len(extensionReport.ManagementAnomalies)
	if extensionProblems > 0 {
		report.add(Check{Name: "extensions", Status: "warning", Code: "extension_diagnostics", Count: extensionProblems, Recovery: "agentdeck extension doctor"})
	} else {
		report.add(Check{Name: "extensions", Status: "ok"})
	}
	return report, nil
}

func (s Service) checkLock(report *Report) {
	info, err := os.Stat(filepath.Join(s.StateRoot, "state.lock"))
	if errors.Is(err, fs.ErrNotExist) {
		report.add(Check{Name: "state_lock", Status: "ok"})
		return
	}
	if err != nil {
		report.add(Check{Name: "state_lock", Status: "warning", Code: "lock_unreadable"})
		return
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	if now().Sub(info.ModTime()) > 10*time.Minute {
		report.add(Check{Name: "state_lock", Status: "warning", Code: "stale_lock"})
		return
	}
	report.add(Check{Name: "state_lock", Status: "warning", Code: "state_busy"})
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
		case item.sealed.Algorithm != credentialvault.AlgorithmAES256GCM || item.sealed.KeyVersion != credentialvault.KeyVersion:
			item.problemCode = credentialvault.ErrKeyVersionUnsupported.Error()
		case len(item.sealed.Nonce) != 12 || len(item.sealed.Ciphertext) < 16:
			item.problemCode = credentialvault.ErrCiphertextInvalid.Error()
		}
		owned = append(owned, item)
	}
	authCandidates := make([]ownedSecret, 0, len(owned))
	keyReady, rotationReady := false, false
	var keyID string
	if secretRows > 0 {
		if s.Vault == nil {
			quickProblems.add(credentialvault.ErrKeyMissing.Error(), "", secretRows)
		} else {
			var inspectErr error
			keyID, inspectErr = s.Vault.InspectKey(ctx)
			if inspectErr != nil {
				quickProblems.add(credentialErrorCode(inspectErr), "", secretRows)
			} else {
				keyReady = true
				keyIDs, keyIDsErr := database.CredentialSecretKeyIDs(ctx)
				if keyIDsErr != nil {
					return keyIDsErr
				}
				rotationReady = len(keyIDs) == 1 && keyIDs[0] != "" && keyIDs[0] == keyID
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
		if item.sealed.KeyID == "" || item.sealed.KeyID != keyID {
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
