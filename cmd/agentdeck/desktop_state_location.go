package main

import (
	"errors"
	"fmt"

	"github.com/kitdine/agent-deck/internal/installation"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/spf13/cobra"
)

type desktopStateLocation struct {
	Installed bool   `json:"installed"`
	Channel   string `json:"channel,omitempty"`
	Reason    string `json:"reason,omitempty"`
	StateRoot string `json:"state_root"`
}

// newDesktopStateLocationCommand is the App's per-round query for the state
// root its helper serves. The root passes the state guard: when this helper
// does not own the real state and the requested root is the real one, the
// isolated default is returned instead, so the real path is never disclosed.
func newDesktopStateLocationCommand(opts *commandOptions) *cobra.Command {
	return &cobra.Command{Use: "state-location", Short: "Report the state directory this helper serves", Hidden: true, Annotations: map[string]string{internalRuntimeSurfaceOnlyAnnotation: "true"}, Args: exactArgs(0), RunE: func(cmd *cobra.Command, _ []string) error {
		if opts.format != "json" {
			return &inputError{err: errors.New("desktop state-location requires --format json")}
		}
		root, err := opts.stateRoot()
		var refused *platform.ErrStateOwnershipRefused
		if errors.As(err, &refused) {
			home, homeErr := userHomeDir()
			if homeErr != nil {
				return homeErr
			}
			root, err = installation.StateRootWithHome("", home)
		}
		if err != nil {
			return err
		}
		identity := platform.CurrentInstallation()
		data := desktopStateLocation{Installed: identity.Installed, StateRoot: root}
		if identity.Installed {
			data.Channel = identity.Channel
		} else {
			data.Reason = identity.Reason
		}
		if data.StateRoot == "" {
			return fmt.Errorf("state root unresolved")
		}
		return writeEnvelope(opts.stdout, opts.format, "desktop.state-location", data, false, nil)
	}}
}
