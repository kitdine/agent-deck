package main

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/kitdine/agent-deck/internal/desktop"
)

func newDesktopCommand(opts *commandOptions) *cobra.Command {
	command := &cobra.Command{Use: "desktop", Short: "Read desktop integration data"}
	wireVersion := desktop.WireVersion
	recentLimit := desktop.DefaultRecentLimit
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
			return writeEnvelope(opts.stdout, opts.format, "desktop.snapshot", result.Snapshot, result.Partial, result.Warnings)
		},
	}
	snapshot.Flags().IntVar(&wireVersion, "wire-version", desktop.WireVersion, "Desktop wire-contract version")
	snapshot.Flags().IntVar(&recentLimit, "recent-limit", desktop.DefaultRecentLimit, "Recent sessions to include (1-20)")
	command.AddCommand(snapshot)
	return command
}
