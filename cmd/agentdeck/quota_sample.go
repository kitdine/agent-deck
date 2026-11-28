package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kitdine/agent-deck/internal/errdefs"
	"github.com/kitdine/agent-deck/internal/quota"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/usage"
)

const quotaSampleLockTimeout = 20 * time.Second

func newLocalQuotaSampleCommand(opts *commandOptions) *cobra.Command {
	return newQuotaSampleCommand(opts, func(ctx context.Context, action quota.SampleAction, client quota.Client, sessionID string) (quota.SampleResult, error) {
		core, stateRoot, err := opts.openStore(ctx)
		if err != nil {
			return quota.SampleResult{}, err
		}
		defer core.Close()
		service := quotaRefreshService(stateRoot, "")
		result, err := service.SampleQuota(ctx, core, quotaSampleLockTimeout, action, client, sessionID)
		if err != nil || action != quota.SampleFinish {
			return result, err
		}
		report := usage.New(core, "")
		report.Now = service.Now
		result.Valuation, err = report.CompletedSampleValuation(ctx, client, sessionID, result.Completion())
		return result, err
	})
}

// newQuotaSampleCommand is constructed independently of the core schema.
// Register it with a sampler once the sample table is installed by migration.
func newQuotaSampleCommand(opts *commandOptions, run func(context.Context, quota.SampleAction, quota.Client, string) (quota.SampleResult, error)) *cobra.Command {
	group := &cobra.Command{Use: "sample", Short: "Measure quota changes for a session", Args: exactArgs(0)}
	for _, entry := range []struct {
		action quota.SampleAction
		short  string
	}{
		{quota.SampleStart, "Record the first quota reading for a session"},
		{quota.SampleFinish, "Record the final quota reading for an active measurement"},
		{quota.SampleCancel, "Clear the active quota measurement for a session"},
	} {
		var client, sessionID string
		command := &cobra.Command{
			Use:   string(entry.action),
			Short: entry.short,
			Args:  exactArgs(0),
			RunE: func(command *cobra.Command, _ []string) error {
				return runQuotaSample(command.Context(), opts, entry.action, quota.Client(client), sessionID, run)
			},
		}
		command.Flags().StringVar(&client, "client", "", "Client (codex or claude)")
		command.Flags().StringVar(&sessionID, "session", "", "Usage session ID")
		group.AddCommand(command)
	}
	return group
}

func quotaSampleErrorCode(err error) string {
	var sampleErr *quota.SampleError
	if !errors.As(err, &sampleErr) {
		return ""
	}
	switch sampleErr.Code {
	case quota.SampleConflict, quota.SampleMissing, quota.SampleUnsettled,
		quota.SampleProbeDisabled, quota.SampleNotOfficial, quota.SampleProbeFailed:
		return string(sampleErr.Code)
	case quota.SampleAccountChanged:
		// The confirmed change invalidated the active measurement itself.
		return string(quota.SampleMissing)
	default:
		return ""
	}
}

func runQuotaSample(ctx context.Context, opts *commandOptions, action quota.SampleAction, client quota.Client, sessionID string, run func(context.Context, quota.SampleAction, quota.Client, string) (quota.SampleResult, error)) error {
	if opts.format != "text" && opts.format != "json" {
		return &inputError{err: errors.New("quota sample supports only text or json format")}
	}
	if client != quota.ClientCodex && client != quota.ClientClaude {
		return &inputError{err: errors.New("--client must be codex or claude")}
	}
	if strings.TrimSpace(sessionID) == "" {
		return &inputError{err: errors.New("--session is required")}
	}
	result, err := run(ctx, action, client, sessionID)
	if err != nil {
		var sampleErr *quota.SampleError
		if errors.As(err, &sampleErr) && sampleErr.Code == quota.SampleSessionNotFound {
			return errdefs.NewNotFound(session.CodeSessionNotFound, fmt.Sprintf("no session %q is known", sessionID), err)
		}
		return err
	}
	if opts.format == "json" {
		return writeResult(opts.stdout, opts.format, "quota.sample."+string(action), result)
	}
	var text strings.Builder
	switch action {
	case quota.SampleStart:
		text.WriteString("Measurement started")
		if result.StartedAt != nil {
			fmt.Fprintf(&text, " (%s)", renderDisplayTimeWithZone(*result.StartedAt))
		}
	case quota.SampleFinish:
		text.WriteString("Measurement completed")
		if result.FinishedAt != nil {
			fmt.Fprintf(&text, " (%s)", renderDisplayTimeWithZone(*result.FinishedAt))
		}
	case quota.SampleCancel:
		text.WriteString("Measurement cleared")
	}
	if _, err := fmt.Fprintln(opts.stdout, text.String()); err != nil {
		return err
	}
	return sessionShowWriteLines(opts.stdout, quotaValuationLines(result.Valuation, sessionShowTextWidth(opts.stdout)))
}
