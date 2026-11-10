package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kitdine/agent-deck/internal/installation"
	"github.com/kitdine/agent-deck/internal/platform"
)

// installationLifecycleSurfaceOnlyAnnotation marks commands that change which
// program owns AgentDeck state. They are run by people and installers, never
// by the GUI, so they are outside the fixed GUI JSON leaf inventory.
const installationLifecycleSurfaceOnlyAnnotation = "agentdeck.installation-lifecycle-surface-only"

type installOperator interface {
	Claim(context.Context, bool) (installation.ClaimResult, error)
	Register(context.Context, installation.RegisterRequest) (installation.RegisterResult, error)
	Unregister(context.Context, installation.UnregisterRequest) (installation.UnregisterResult, error)
	RepairIdentity(context.Context, installation.RepairIdentityRequest) (string, error)
	PrepareAttempt(installation.AttemptRequest) (installation.AttemptMaterials, error)
	CompleteAttempt(installation.AttemptRequest) error
	CleanupAttempt(context.Context, installation.CleanupRequest) (string, error)
}

// currentInstallOperator uses the process's injected installation resolver.
// Test processes inject fixtures without these operations, so an in-process
// test can never write the account's real receipt.
var currentInstallOperator = func() (installOperator, error) {
	operator, ok := platform.CurrentInstallationResolver().(installOperator)
	if !ok {
		return nil, errors.New("installation operations are unavailable in this process")
	}
	return operator, nil
}

type installClaimData struct {
	Channel  string `json:"channel"`
	Takeover bool   `json:"takeover"`
}

type installRegisterData struct {
	Channel        string `json:"channel"`
	InstallationID string `json:"installation_id"`
	Takeover       bool   `json:"takeover"`
}

type installUnregisterData struct {
	Outcome string `json:"outcome"`
}

type installRepairIdentityData struct {
	InstallationID string `json:"installation_id"`
}

type installAttemptData struct {
	AttemptID string `json:"attempt_id"`
	Directory string `json:"directory"`
	Helper    string `json:"helper"`
	Manifest  string `json:"manifest"`
}

var installRefusalText = map[string][2]string{
	installation.ReasonOwnedByOther:         {"Another AgentDeck installation owns AgentDeck state.", "Keep the current installation, or rerun with --takeover after quitting it."},
	installation.ReasonOwnerRunning:         {"The current owner is still using AgentDeck state.", "Quit AgentDeck.app, stop any agentdeck watch or telemetry listener, pause Codex and Claude sessions, then retry."},
	installation.ReasonNotCurrentFormulaKeg: {"Only the current Homebrew formula installation can claim AgentDeck state.", "Run $(brew --prefix)/opt/agentdeck/bin/agentdeck install claim instead."},
	installation.ReasonReceiptMissing:       {"The installation receipt is missing, so this installation cannot be verified for removal.", "Reinstall with the original installation method to repair the receipt, then uninstall again."},
	installation.ReasonReceiptInvalid:       {"The installation receipt or cleanup record is invalid, so this installation cannot be verified for removal.", "Reinstall with the original installation method to repair the receipt, then uninstall again."},
}

func installRefusalMessage(reason string) string {
	if text, ok := installRefusalText[reason]; ok {
		return text[0]
	}
	return "AgentDeck installation ownership was refused."
}

func renderInstallRefusal(w io.Writer, refusal *installation.InstallError) {
	fmt.Fprintf(w, "install_claim_refused: %s\n  reason: %s\n", installRefusalMessage(refusal.Reason), refusal.Reason)
	if refusal.Detail != "" {
		fmt.Fprintf(w, "  detail: %s\n", refusal.Detail)
	}
	if text, ok := installRefusalText[refusal.Reason]; ok {
		fmt.Fprintf(w, "  next: %s\n", text[1])
	}
}

func newInstallCommand(opts *commandOptions) *cobra.Command {
	command := &cobra.Command{
		Use:         "install",
		Short:       "Manage which AgentDeck installation owns AgentDeck state",
		Annotations: map[string]string{installationLifecycleSurfaceOnlyAnnotation: "true"},
	}
	command.AddCommand(newInstallClaimCommand(opts), newInstallRegisterCommand(opts), newInstallUnregisterCommand(opts), newInstallRepairIdentityCommand(opts), newInstallAttemptCommand(opts))
	return command
}

func newInstallClaimCommand(opts *commandOptions) *cobra.Command {
	var takeover bool
	command := &cobra.Command{
		Use:         "claim",
		Short:       "Make this Homebrew formula installation own AgentDeck state",
		Long:        "Record this Homebrew formula installation as the program that owns AgentDeck state. Only the keg that the formula's opt link points to can claim. --takeover replaces another installation, and only succeeds when that installation is not running.",
		Example:     "  $(brew --prefix)/opt/agentdeck/bin/agentdeck install claim",
		Annotations: map[string]string{installationLifecycleSurfaceOnlyAnnotation: "true"},
		Args:        exactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			operator, err := currentInstallOperator()
			if err != nil {
				return err
			}
			result, err := operator.Claim(command.Context(), takeover)
			if err != nil {
				return err
			}
			if opts.format == "text" {
				_, err = fmt.Fprintln(opts.stdout, "Claimed: this Homebrew formula installation now owns AgentDeck state.")
				return err
			}
			return writeEnvelope(opts.stdout, opts.format, "install.claim", installClaimData{Channel: result.Channel, Takeover: result.Takeover}, false, nil)
		},
	}
	command.Flags().BoolVar(&takeover, "takeover", false, "Replace another installation that is not running")
	return command
}

func newInstallRegisterCommand(opts *commandOptions) *cobra.Command {
	var request installation.RegisterRequest
	var expected string
	command := &cobra.Command{
		Use:         "register",
		Short:       "Publish and record an installation (installer use only)",
		Hidden:      true,
		Annotations: map[string]string{internalRuntimeSurfaceOnlyAnnotation: "true"},
		Args:        exactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			operator, err := currentInstallOperator()
			if err != nil {
				return err
			}
			if command.Flags().Changed("expected-installation") {
				request.BindExpectedInstallation(expected)
			}
			result, err := operator.Register(command.Context(), request)
			if err != nil {
				return err
			}
			if result.Residual != "" {
				fmt.Fprintf(opts.stderr, "warning: could not remove the replaced copy at %s\n", result.Residual)
			}
			if opts.format == "text" {
				_, err = fmt.Fprintf(opts.stdout, "registered %s (%s)\n", request.Executable, result.Channel)
				return err
			}
			return writeEnvelope(opts.stdout, opts.format, "install.register", installRegisterData{Channel: result.Channel, InstallationID: result.InstallationID, Takeover: result.Takeover}, false, nil)
		},
	}
	flags := command.Flags()
	flags.StringVar(&request.Channel, "channel", "", "Installation method: homebrew_cask, homebrew_formula, make, or local_app")
	flags.StringVar(&request.Executable, "executable", "", "Final physical path of the installed agentdeck program")
	flags.StringVar(&request.Source, "source", "", "Staged file or App bundle beside the target to publish")
	flags.StringVar(&request.CleanupManifest, "cleanup-manifest", "", "New private cleanup manifest for this installation")
	flags.BoolVar(&request.Takeover, "takeover", false, "Replace another installation that is not running")
	flags.BoolVar(&request.RequireUnowned, "require-unowned", false, "Publish only when no valid installation owns state and the target is absent")
	flags.BoolVar(&request.AuthorizedRepair, "authorized-repair", false, "With --require-unowned, also replace an existing target that has no valid owner")
	flags.StringVar(&expected, "expected-installation", "", "With --authorized-repair, replace only the installation that still has this ID")
	_ = command.MarkFlagRequired("channel")
	_ = command.MarkFlagRequired("executable")
	return command
}

func newInstallUnregisterCommand(opts *commandOptions) *cobra.Command {
	var request installation.UnregisterRequest
	command := &cobra.Command{
		Use:         "unregister",
		Short:       "Remove a recorded installation (installer use only)",
		Hidden:      true,
		Annotations: map[string]string{internalRuntimeSurfaceOnlyAnnotation: "true"},
		Args:        exactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			operator, err := currentInstallOperator()
			if err != nil {
				return err
			}
			result, err := operator.Unregister(command.Context(), request)
			if err != nil {
				return err
			}
			if opts.format == "text" {
				_, err = fmt.Fprintf(opts.stdout, "%s %s\n", strings.ReplaceAll(result.Outcome, "_", " "), request.Executable)
				return err
			}
			return writeEnvelope(opts.stdout, opts.format, "install.unregister", installUnregisterData{Outcome: result.Outcome}, false, nil)
		},
	}
	flags := command.Flags()
	flags.StringVar(&request.Channel, "channel", "", "Installation method recorded by this installation")
	flags.StringVar(&request.Executable, "executable", "", "Physical path of the installed agentdeck program")
	flags.StringVar(&request.ExpectedInstallation, "expected-installation", "", "Installation ID from this installation's cleanup manifest")
	flags.StringVar(&request.CleanupManifest, "cleanup-manifest", "", "This installation's cleanup manifest")
	flags.BoolVar(&request.Remove, "remove", false, "Also remove the installed program or App bundle")
	flags.BoolVar(&request.AuthorizedRepair, "authorized-repair", false, "Report an unverifiable installation as skipped instead of failing; never removes it")
	flags.BoolVar(&request.RequireIdentity, "require-identity", false, "Require the captured repair identity to match under the cleanup lock")
	for _, name := range []string{"channel", "executable", "expected-installation"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

func newInstallRepairIdentityCommand(opts *commandOptions) *cobra.Command {
	var request installation.RepairIdentityRequest
	command := &cobra.Command{
		Use:         "repair-identity",
		Short:       "Print this cask installation's ID for an identity-bound repair (installer use only)",
		Hidden:      true,
		Annotations: map[string]string{internalRuntimeSurfaceOnlyAnnotation: "true"},
		Args:        exactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			operator, err := currentInstallOperator()
			if err != nil {
				return err
			}
			id, err := operator.RepairIdentity(command.Context(), request)
			if err != nil {
				return err
			}
			if opts.format == "text" {
				_, err = fmt.Fprintln(opts.stdout, id)
				return err
			}
			return writeEnvelope(opts.stdout, opts.format, "install.repair-identity", installRepairIdentityData{InstallationID: id}, false, nil)
		},
	}
	flags := command.Flags()
	flags.StringVar(&request.Channel, "channel", "", "Installation method; only homebrew_cask is supported")
	flags.StringVar(&request.Executable, "executable", "", "Fixed path of this App's agentdeck helper")
	_ = command.MarkFlagRequired("channel")
	_ = command.MarkFlagRequired("executable")
	return command
}

// newInstallAttemptCommand groups the Homebrew cask's per-attempt materials.
// The cask passes an attempt ID and uses the paths these commands report; it
// derives no AgentDeck path itself.
func newInstallAttemptCommand(opts *commandOptions) *cobra.Command {
	command := &cobra.Command{
		Use:         "attempt",
		Short:       "Manage one Homebrew cask install attempt (installer use only)",
		Hidden:      true,
		Annotations: map[string]string{internalRuntimeSurfaceOnlyAnnotation: "true"},
	}
	attemptFlags := func(command *cobra.Command, request *installation.AttemptRequest, locator bool) {
		flags := command.Flags()
		flags.StringVar(&request.AttemptID, "attempt", "", "Attempt ID chosen by the cask for this install")
		flags.StringVar(&request.Token, "token", "", "Cask token")
		flags.StringVar(&request.Version, "version", "", "Cask version")
		flags.StringVar(&request.Executable, "executable", "", "Final physical path of the App's agentdeck helper")
		if locator {
			flags.StringVar(&request.Locator, "locator", "", "Attempt locator file inside the cask's staged directory")
		}
	}
	var prepare installation.AttemptRequest
	prepareCommand := &cobra.Command{
		Use:         "prepare",
		Short:       "Create an attempt's private cleanup materials",
		Hidden:      true,
		Annotations: map[string]string{internalRuntimeSurfaceOnlyAnnotation: "true"},
		Args:        exactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			operator, err := currentInstallOperator()
			if err != nil {
				return err
			}
			materials, err := operator.PrepareAttempt(prepare)
			if err != nil {
				return err
			}
			if opts.format == "text" {
				_, err = fmt.Fprintf(opts.stdout, "attempt_id=%s\ndirectory=%s\nhelper=%s\nmanifest=%s\n", materials.AttemptID, materials.Directory, materials.Helper, materials.Manifest)
				return err
			}
			return writeEnvelope(opts.stdout, opts.format, "install.attempt.prepare", installAttemptData{materials.AttemptID, materials.Directory, materials.Helper, materials.Manifest}, false, nil)
		},
	}
	attemptFlags(prepareCommand, &prepare, true)
	var complete installation.AttemptRequest
	completeCommand := &cobra.Command{
		Use:         "complete",
		Short:       "Mark an attempt as published",
		Hidden:      true,
		Annotations: map[string]string{internalRuntimeSurfaceOnlyAnnotation: "true"},
		Args:        exactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			operator, err := currentInstallOperator()
			if err != nil {
				return err
			}
			return operator.CompleteAttempt(complete)
		},
	}
	completeCommand.Flags().StringVar(&complete.AttemptID, "attempt", "", "Attempt ID chosen by the cask for this install")
	var cleanup installation.CleanupRequest
	var expected string
	cleanupCommand := &cobra.Command{
		Use:         "cleanup",
		Short:       "Conditionally remove the installation an attempt published",
		Hidden:      true,
		Annotations: map[string]string{internalRuntimeSurfaceOnlyAnnotation: "true"},
		Args:        exactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			operator, err := currentInstallOperator()
			if err != nil {
				return err
			}
			if command.Flags().Changed("expected-installation") {
				cleanup.BindExpectedInstallation(expected)
			}
			outcome, err := operator.CleanupAttempt(command.Context(), cleanup)
			if err != nil {
				return err
			}
			if outcome == installation.OutcomeSkippedUnverified {
				fmt.Fprintln(opts.stderr, "warning: this installation could not be verified, so nothing was removed and the App is still installed.")
			}
			if opts.format == "text" {
				_, err = fmt.Fprintf(opts.stdout, "%s %s\n", strings.ReplaceAll(outcome, "_", " "), cleanup.Executable)
				return err
			}
			return writeEnvelope(opts.stdout, opts.format, "install.attempt.cleanup", installUnregisterData{Outcome: outcome}, false, nil)
		},
	}
	attemptFlags(cleanupCommand, &cleanup.AttemptRequest, true)
	cleanupCommand.Flags().BoolVar(&cleanup.AuthorizedRepair, "authorized-repair", false, "Report an unverifiable installation as skipped instead of failing; never removes it")
	cleanupCommand.Flags().StringVar(&expected, "expected-installation", "", "With --authorized-repair, the installation ID captured for this repair")
	command.AddCommand(prepareCommand, completeCommand, cleanupCommand)
	return command
}
