package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/kitdine/agent-deck/internal/doctor"
	"github.com/kitdine/agent-deck/internal/platform"
)

const isolatedStateNext = "Expected for development builds. If this should be your installation, reinstall it with Homebrew, make install, or make install-macos-app."
const refusedStateNext = "Omit --state-dir to use the isolated state, or run the installed agentdeck."

func ownershipSilentCommand(command *cobra.Command) bool {
	path := command.CommandPath()
	return strings.HasPrefix(path, "agentdeck usage hook event") || path == "agentdeck quota capture"
}

func prepareOwnershipCommand(opts *commandOptions, command *cobra.Command) error {
	root, err := opts.stateRoot()
	if err != nil {
		if ownershipSilentCommand(command) {
			command.RunE = func(*cobra.Command, []string) error { return nil }
			return nil
		}
		var refused *platform.ErrStateOwnershipRefused
		if errors.As(err, &refused) {
			if command.CommandPath() == "agentdeck doctor" {
				return nil // Doctor renders its own path-free partial report.
			}
		}
		return err
	}
	identity := platform.CurrentInstallation()
	if identity.Installed || opts.quiet || opts.format != "text" || ownershipSilentCommand(command) {
		return nil
	}
	file, ok := opts.stderr.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return nil
	}
	description := "isolated state ~/.agentdeck-dev"
	if opts.stateDir != "" {
		description = "state " + root
	}
	_, err = fmt.Fprintf(opts.stderr, "note: not the installed AgentDeck (%s); using %s\n", identity.Reason, description)
	return err
}

func renderStateOwnershipRefusal(w io.Writer, refused *platform.ErrStateOwnershipRefused) {
	fmt.Fprintf(w, "state_ownership_refused: %s\n  resource: %s\n  reason: %s\n", refused.Error(), refused.Resource, refused.Reason)
	if refused.Revoked {
		fmt.Fprintln(w, "  next: Another installation now owns AgentDeck state. Run that installation's agentdeck.")
	} else {
		fmt.Fprintln(w, "  next: "+refusedStateNext)
		fmt.Fprintln(w, "  manual prerequisite: To work with real data, make an authorized SQLite backup copy and pass that copy's directory.")
	}
	fmt.Fprintln(w, "  diagnose: agentdeck doctor")
}

func renderOwnershipCheck(w io.Writer, check doctor.Check) error {
	if check.Name == "installation" {
		if check.Status == "ok" {
			_, err := fmt.Fprintf(w, "  channel: %s\n", check.Reason)
			return err
		}
		if _, err := fmt.Fprintf(w, "  reason: %s\n  state: isolated ~/.agentdeck-dev\n  next: %s\n", check.Reason, isolatedStateNext); err != nil {
			return err
		}
	}
	if check.Name == "state" && check.Code == "state_ownership_refused" {
		_, err := fmt.Fprintf(w, "  reason: %s\n  next: %s\n", check.Reason, refusedStateNext)
		return err
	}
	return nil
}
