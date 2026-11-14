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
	command.AddCommand(newInstallClaimCommand(opts), newInstallRegisterCommand(opts), newInstallUnregisterCommand(opts))
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
	command := &cobra.Command{
		Use:         "register",
		Short:       "Record an installation, publishing it first when given a staged source (installer use only)",
		Hidden:      true,
		Annotations: map[string]string{internalRuntimeSurfaceOnlyAnnotation: "true"},
		Args:        exactArgs(0),
		RunE: func(command *cobra.Command, _ []string) error {
			operator, err := currentInstallOperator()
			if err != nil {
				return err
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
	for _, name := range []string{"channel", "executable", "expected-installation"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}
