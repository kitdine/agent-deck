package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/output"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
	"github.com/kitdine/agent-deck/internal/watch"
)

const desktopSnapshotChunkBytes = 48 * 1024

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
	Usage    desktopIndexDomainResult `json:"usage"`
	Sessions desktopIndexDomainResult `json:"sessions"`
}

type desktopIndexDomainResult struct {
	Success              bool   `json:"success"`
	DurationMilliseconds int64  `json:"duration_ms"`
	Changes              any    `json:"changes,omitempty"`
	ErrorCode            string `json:"error_code,omitempty"`
}

type desktopIndexScan func() (any, error)

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
			result, err := (desktop.Service{
				StateRoot: stateRoot,
				Home:      home,
				Workdir:   workdir,
				Vault:     newCredentialVault(stateRoot),
				Location:  displayLocation(),
			}).Build(cmd.Context(), desktop.Request{WireVersion: wireVersion, RecentLimit: recentLimit})
			if err != nil {
				return err
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
			return writeEnvelope(opts.stdout, opts.format, "desktop.refresh-indexes", result, partial, warnings)
		},
	}
	command.AddCommand(snapshot, refreshIndexes)
	return command
}

func refreshDesktopIndexes(ctx context.Context, stateRoot, home string) (desktopIndexRefreshResult, bool, []string, error) {
	lock, err := store.AcquireLock(ctx, stateRoot, 5*time.Second)
	if err != nil {
		return desktopIndexRefreshResult{}, false, nil, err
	}
	defer lock.Release()

	core, err := store.OpenWithLockHeld(ctx, stateRoot)
	if err != nil {
		return desktopIndexRefreshResult{}, false, nil, err
	}
	defer core.Close()
	sessions, err := store.OpenSessions(ctx, stateRoot)
	if err != nil {
		return desktopIndexRefreshResult{}, false, nil, err
	}
	defer sessions.Close()

	result := runDesktopIndexScans(
		func() (any, error) {
			return usage.New(core, home).Scan(ctx)
		},
		func() (any, error) {
			return session.Scan(ctx, sessions.DB, home)
		},
	)
	if result.Sessions.Success {
		fingerprint, fingerprintErr := watch.FingerprintRoots(sessionWatchRoots(home)...)
		if fingerprintErr == nil {
			fingerprintErr = core.SetSetting(ctx, "watch.fingerprint.session", fingerprint)
		}
		if fingerprintErr != nil {
			result.Sessions.Success = false
			result.Sessions.ErrorCode = errorCode(fingerprintErr)
		}
	}
	warnings := []string{}
	if !result.Usage.Success {
		warnings = append(warnings, "usage_index_refresh_failed")
	}
	if !result.Sessions.Success {
		warnings = append(warnings, "session_index_refresh_failed")
	}
	return result, len(warnings) > 0, warnings, nil
}

func runDesktopIndexScans(usageScan, sessionScan desktopIndexScan) desktopIndexRefreshResult {
	var wait sync.WaitGroup
	wait.Add(2)
	var usageResult, sessionResult desktopIndexDomainResult
	go func() {
		defer wait.Done()
		usageResult = runDesktopIndexScan(usageScan)
	}()
	go func() {
		defer wait.Done()
		sessionResult = runDesktopIndexScan(sessionScan)
	}()
	wait.Wait()
	return desktopIndexRefreshResult{Usage: usageResult, Sessions: sessionResult}
}

func runDesktopIndexScan(scan desktopIndexScan) desktopIndexDomainResult {
	startedAt := time.Now()
	changes, err := scan()
	result := desktopIndexDomainResult{
		Success:              err == nil,
		DurationMilliseconds: time.Since(startedAt).Milliseconds(),
		Changes:              changes,
	}
	if err != nil {
		result.Changes = nil
		result.ErrorCode = errorCode(err)
	}
	return result
}

func writeDesktopSnapshotStream(w interface{ Write([]byte) (int, error) }, result desktop.Result) error {
	envelope := output.New("desktop.snapshot", result.Snapshot, time.Now())
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
			GeneratedAt:   time.Now().UTC(),
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
