package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/scanruntime"
)

const desktopSnapshotChunkBytes = 48 * 1024

var desktopNow = time.Now
var desktopIndexRefreshObserver func(desktopIndexRefreshResult)
var desktopSnapshotObserver func(desktop.Result)

type desktopSnapshotChunkEnvelope struct {
	SchemaVersion int                      `json:"schema_version"`
	Command       string                   `json:"command"`
	GeneratedAt   time.Time                `json:"generated_at"`
	Data          desktopSnapshotChunkData `json:"data"`
	Warnings      []string                 `json:"warnings"`
	Partial       bool                     `json:"partial"`
}

type desktopSnapshotChunkData struct {
	Index      int    `json:"index"`
	Count      int    `json:"count"`
	TotalBytes int    `json:"total_bytes"`
	SHA256     string `json:"sha256"`
	Payload    string `json:"payload"`
}

type desktopIndexRefreshResult struct {
	Usage              desktopIndexDomainResult `json:"usage"`
	Sessions           desktopIndexDomainResult `json:"sessions"`
	discoveryMS        int64
	workerTotalMS      int64
	derivedCacheMS     int64
	workerCPUTimeMS    int64
	workerPeakRSSBytes int64
}

type desktopIndexDomainResult struct {
	Success              bool   `json:"success"`
	DurationMilliseconds int64  `json:"duration_ms"`
	Changes              any    `json:"changes,omitempty"`
	ErrorCode            string `json:"error_code,omitempty"`
	failureStage         string
}

func newDesktopCommand(opts *commandOptions) *cobra.Command {
	command := &cobra.Command{Use: "desktop", Short: "Read desktop integration data"}
	wireVersion := desktop.WireVersion
	recentLimit := desktop.DefaultRecentLimit
	stream := false
	snapshot := &cobra.Command{
		Use:     "snapshot",
		Short:   "Read one privacy-bounded desktop snapshot",
		Long:    "Read one coherent, versioned JSON snapshot without scanning sources, creating state, or using the network.",
		Example: "  agentdeck --format json desktop snapshot --wire-version 1 --recent-limit 5",
		Args:    exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.format != "json" {
				return &inputError{err: errors.New("desktop snapshot requires --format json")}
			}
			stateRoot, err := opts.stateRoot()
			if err != nil {
				return err
			}
			home, err := userHomeDir()
			if err != nil {
				return err
			}
			workdir, err := os.Getwd()
			if err != nil {
				return err
			}
			service := desktop.Service{
				RegistrationProbe: doctorRegistrationProbe,
				StateRoot:         stateRoot,
				Home:              home,
				Workdir:           workdir,
				Vault:             newCredentialVault(stateRoot),
				Now:               desktopNow,
				Location:          displayLocation(),
			}
			if wireVersion == desktop.ServingWireVersion {
				if stream {
					return &inputError{err: errors.New("serving snapshot does not use the v1 chunk stream")}
				}
				prepared, err := service.ReadOrBootstrapServingSnapshot(cmd.Context(), desktop.Request{WireVersion: desktop.WireVersion, RecentLimit: recentLimit})
				if err != nil {
					return err
				}
				return writeEnvelope(opts.stdout, opts.format, "desktop.serving", prepared, prepared.Payload.Partial, prepared.Payload.Warnings)
			}
			result, err := service.Build(cmd.Context(), desktop.Request{WireVersion: wireVersion, RecentLimit: recentLimit})
			if err != nil {
				return err
			}
			if desktopSnapshotObserver != nil {
				desktopSnapshotObserver(result)
			}
			if stream {
				return writeDesktopSnapshotStream(opts.stdout, result)
			}
			return writeEnvelope(opts.stdout, opts.format, "desktop.snapshot", result.Snapshot, result.Partial, result.Warnings)
		},
	}
	snapshot.Flags().IntVar(&wireVersion, "wire-version", desktop.WireVersion, "Desktop wire-contract version")
	snapshot.Flags().IntVar(&recentLimit, "recent-limit", desktop.DefaultRecentLimit, "Recent sessions to include (1-20)")
	snapshot.Flags().BoolVar(&stream, "stream", false, "Stream the snapshot as bounded, integrity-checked JSON chunks")
	refreshIndexes := &cobra.Command{
		Use:   "refresh-indexes",
		Short: "Incrementally refresh usage and session indexes in parallel",
		Args:  exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.format != "json" {
				return &inputError{err: errors.New("desktop refresh-indexes requires --format json")}
			}
			stateRoot, err := opts.stateRoot()
			if err != nil {
				return err
			}
			home, err := userHomeDir()
			if err != nil {
				return err
			}
			result, partial, warnings, err := refreshDesktopIndexes(cmd.Context(), stateRoot, home)
			if err != nil {
				return err
			}
			if desktopIndexRefreshObserver != nil {
				desktopIndexRefreshObserver(result)
			}
			return writeEnvelope(opts.stdout, opts.format, "desktop.refresh-indexes", result, partial, warnings)
		},
	}
	command.AddCommand(snapshot, refreshIndexes, newDesktopBootstrapCommand(opts), newDesktopQuotaRefreshCommand(opts), newDesktopQuotaSettingsCommand(opts), newDesktopQuotaStatusLineCommand(opts), newDesktopQuotaAlertsCommand(opts))
	return command
}

type desktopBootstrapEvent struct {
	SchemaVersion int    `json:"schema_version"`
	Command       string `json:"command"`
	Type          string `json:"type"`
	Data          any    `json:"data"`
}

func newDesktopBootstrapCommand(opts *commandOptions) *cobra.Command {
	return &cobra.Command{Use: "bootstrap", Short: "Run an App-owned finite bootstrap and stream publications", Hidden: true, Annotations: map[string]string{internalRuntimeSurfaceOnlyAnnotation: "true"}, Args: exactArgs(0), RunE: func(cmd *cobra.Command, _ []string) error {
		if opts.format != "ndjson" {
			return &inputError{err: errors.New("desktop bootstrap requires --format ndjson")}
		}
		root, err := opts.stateRoot()
		if err != nil {
			return err
		}
		home, err := userHomeDir()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()
		encoder := json.NewEncoder(opts.stdout)
		var outputErr error
		emit := func(kind string, value any) {
			if outputErr != nil {
				return
			}
			outputErr = encoder.Encode(desktopBootstrapEvent{SchemaVersion: 2, Command: "desktop.bootstrap", Type: kind, Data: value})
			if outputErr != nil {
				cancel()
			}
		}
		client := scanruntime.Client{StateRoot: root, Home: home, Executable: scanRuntimeExecutable(), ForceLocal: scanRuntimeForceLocal(), Now: desktopNow, AppOwned: true,
			OnPublication: func(value desktop.ServingSnapshot) { emit("publication", value) }}
		result, err := client.RequestWithProgress(ctx, scanruntime.ScopeBoth, func(value scanruntime.Progress) { emit("progress", value) })
		if outputErr != nil {
			return outputErr
		}
		if err != nil {
			return err
		}
		emit("terminal", result)
		if result.Usage.State != "completed" || result.Session.State != "completed" {
			return errors.New("desktop bootstrap did not complete")
		}
		return outputErr
	}}
}

func refreshDesktopIndexes(ctx context.Context, stateRoot, home string) (desktopIndexRefreshResult, bool, []string, error) {
	round, err := (scanruntime.Client{StateRoot: stateRoot, Home: home, Executable: scanRuntimeExecutable(), ForceLocal: scanRuntimeForceLocal(), Now: desktopNow}).Request(ctx, scanruntime.ScopeBoth)
	if err != nil {
		return desktopIndexRefreshResult{}, false, nil, err
	}
	result := desktopIndexResultFromRound(round)
	warnings := []string{}
	if !result.Usage.Success {
		warnings = append(warnings, "usage_index_refresh_failed")
	}
	if !result.Sessions.Success {
		warnings = append(warnings, "session_index_refresh_failed")
	}
	return result, len(warnings) > 0, warnings, nil
}

func desktopIndexResultFromRound(round scanruntime.Result) desktopIndexRefreshResult {
	result := desktopIndexRefreshResult{
		discoveryMS:        round.Stages.DiscoveryMS,
		workerTotalMS:      round.Stages.TotalMS,
		derivedCacheMS:     round.Stages.DerivedCacheMS,
		workerCPUTimeMS:    round.Stages.WorkerCPUTimeMS,
		workerPeakRSSBytes: round.Stages.WorkerPeakRSSBytes,
		Usage: desktopIndexDomainResult{
			Success:              round.Usage.State == "completed",
			DurationMilliseconds: round.Usage.DurationMS,
			Changes:              round.Usage.Changes,
			ErrorCode:            round.Usage.ErrorCode,
		},
		Sessions: desktopIndexDomainResult{
			Success:              round.Session.State == "completed",
			DurationMilliseconds: round.Session.DurationMS,
			Changes:              round.Session.Scan,
			ErrorCode:            round.Session.ErrorCode,
		},
	}
	if !result.Usage.Success {
		result.Usage.failureStage = scanRuntimeFailureStage(round.Usage.ErrorCode)
		result.Usage.Changes = nil
	}
	if !result.Sessions.Success {
		result.Sessions.failureStage = scanRuntimeFailureStage(round.Session.ErrorCode)
		result.Sessions.Changes = nil
	}
	return result
}

func scanRuntimeFailureStage(code string) string {
	if code == "deadline_exceeded" || code == "cancelled" {
		return "deadline"
	}
	return "scan"
}

func writeDesktopSnapshotStream(w interface{ Write([]byte) (int, error) }, result desktop.Result) error {
	envelope := output.New("desktop.snapshot", result.Snapshot, desktopNow())
	warnings := result.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	envelope.Partial, envelope.Warnings = result.Partial, warnings
	payload, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(payload)
	digestText := hex.EncodeToString(digest[:])
	count := max(1, (len(payload)+desktopSnapshotChunkBytes-1)/desktopSnapshotChunkBytes)
	encoder := json.NewEncoder(w)
	for index := 0; index < count; index++ {
		start := index * desktopSnapshotChunkBytes
		end := min(len(payload), start+desktopSnapshotChunkBytes)
		frame := desktopSnapshotChunkEnvelope{
			SchemaVersion: output.SchemaVersion,
			Command:       "desktop.snapshot.chunk",
			GeneratedAt:   desktopNow().UTC(),
			Data: desktopSnapshotChunkData{
				Index: index, Count: count, TotalBytes: len(payload), SHA256: digestText,
				Payload: base64.StdEncoding.EncodeToString(payload[start:end]),
			},
			Warnings: []string{},
		}
		if err = encoder.Encode(frame); err != nil {
			return err
		}
	}
	return nil
}
