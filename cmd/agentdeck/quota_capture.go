package main

import (
	"context"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/usagehook"
)

// newQuotaCommand groups subscription-quota commands. It is hidden for now:
// task 3 adds only the runtime capture entry Claude Code's statusLine
// invokes; the public `agentdeck quota` text/json surface is task 6's.
func newQuotaCommand(opts *commandOptions) *cobra.Command {
	command := &cobra.Command{Use: "quota", Short: "Subscription quota", Hidden: true, Args: exactArgs(0)}
	command.AddCommand(newQuotaCaptureCommand(opts))
	return command
}

func newQuotaCaptureCommand(opts *commandOptions) *cobra.Command {
	return &cobra.Command{
		Use: "capture",
		Short: "Read one status-line payload from stdin, persist any rate-limit fields it carries, " +
			"then invoke the command AgentDeck's registration replaced (if any) with the same stdin " +
			"and pass its stdout through unchanged.",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return runQuotaCapture(command.Context(), opts)
		},
	}
}

// quotaCaptureStoreTimeout bounds how long runQuotaCapture will wait for the
// core database's lock (CLA-R1-F3). Capture is best-effort by design (C3);
// losing one cycle's persistence is an acceptable cost, delaying the user's
// status line while another process holds the lock — or while a first-run
// migration executes — is not.
const quotaCaptureStoreTimeout = 200 * time.Millisecond

// runQuotaCapture is fail-open throughout, matching runUsageHookEvent's
// posture: a status-line command runs on every refresh of a live session, so
// nothing here may block or corrupt that refresh.
//
// Ordering is deliberate and load-bearing (CLA-R1-F3): the prior command is
// chained before anything capture-related runs, including resolving it from
// disk and opening the core database — none of that may sit in front of the
// chain. The full original stdin is read once, unbounded, and that exact
// slice always reaches the prior command via ChainStatusLine regardless of
// size (CLA-R1-F5) — the chain must never see a truncated or substituted
// payload. Capture's own parsing is a separate, smaller concern: once stdin
// exceeds quota.MaxStatusLinePayloadBytes, this skips calling
// RecordStatusLinePayload — and therefore skips opening the store at all —
// for this cycle, rather than parsing an oversized blob (CLA-R2-F1). Losing
// one cycle's capture is the acceptable cost C3 already assumes; the chain
// having already fully run by that point is unaffected either way.
func runQuotaCapture(ctx context.Context, opts *commandOptions) error {
	payload, _ := io.ReadAll(opts.stdin)

	var priorCommand string
	if home, homeErr := userHomeDir(); homeErr == nil {
		stateDir, stateErr := opts.stateRoot()
		if stateErr != nil {
			stateDir = ""
		}
		manager := usagehook.New(usagehook.Environment{Home: home, StateDir: stateDir})
		priorCommand, _ = manager.PriorStatusLineCommand()
	}

	quota.ChainStatusLine(ctx, priorCommand, payload, opts.stdout)

	if len(payload) > quota.MaxStatusLinePayloadBytes {
		return nil
	}

	captureCtx, cancel := context.WithTimeout(ctx, quotaCaptureStoreTimeout)
	defer cancel()
	database, _, err := opts.openStore(captureCtx)
	if err != nil {
		return nil
	}
	defer database.Close()
	quota.RecordStatusLinePayload(ctx, quota.NewStore(database.DB), payload, time.Now())
	return nil
}
