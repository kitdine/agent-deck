// Package doctor coordinates read-only state diagnostics.
package doctor

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

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
	Healthy  bool    `json:"healthy"`
	Checks   []Check `json:"checks"`
	Problems int     `json:"problems"`
}

type Service struct {
	StateRoot string
	Home      string
	Workdir   string
	Secrets   platform.SecretStore
	Now       func() time.Time
}

func (s Service) Check(ctx context.Context, full bool) (Report, error) {
	report := Report{Mode: map[bool]string{false: "quick", true: "full"}[full], Healthy: true, Checks: []Check{}}
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
	if version != store.CurrentSchemaVersion {
		report.add(Check{Name: "schema", Status: "warning", Code: "schema_outdated", Count: version})
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
	} else {
		report.add(Check{Name: "pending_operations", Status: "ok"})
	}
	if err = s.checkProviders(ctx, database, &report); err != nil {
		return Report{}, err
	}
	if err = s.checkUsage(ctx, database, &report); err != nil {
		return Report{}, err
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

func (s Service) checkProviders(ctx context.Context, database *store.Store, report *Report) error {
	service := provider.Service{Store: database, Secrets: s.Secrets}
	statuses, err := service.List(ctx)
	if err != nil {
		return err
	}
	missing := 0
	for _, status := range statuses {
		if !status.Credential.Present {
			missing++
		}
	}
	if missing > 0 {
		report.add(Check{Name: "provider_credentials", Status: "error", Code: "credential_missing", Count: missing, Recovery: "agentdeck provider credential add"})
	} else {
		report.add(Check{Name: "provider_credentials", Status: "ok"})
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

func (s Service) checkUsage(ctx context.Context, database *store.Store, report *Report) error {
	service := usage.New(database, s.Home)
	diagnostics, err := service.Diagnose(ctx)
	if err != nil {
		return err
	}
	incomplete, _ := diagnostics["exact_runs"].(int)
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
		report.add(Check{Name: "prices", Status: "warning", Code: "price_catalog_missing", Recovery: "agentdeck usage price status"})
	} else {
		report.add(Check{Name: "prices", Status: "ok"})
	}
	invalidProvenance, unpricedModels, err := service.PriceDiagnostics(ctx)
	if err != nil {
		return err
	}
	if invalidProvenance > 0 {
		report.add(Check{Name: "price_provenance", Status: "warning", Code: "price_provenance_invalid", Count: invalidProvenance, Recovery: "agentdeck usage price status"})
	} else {
		report.add(Check{Name: "price_provenance", Status: "ok"})
	}
	if unpricedModels > 0 {
		report.add(Check{Name: "unpriced_models", Status: "warning", Code: "unpriced_models", Count: unpricedModels, Recovery: "agentdeck usage price status"})
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
	}
}
