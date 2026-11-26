//go:build cjsi_acceptance_upgrade

// Command upgrader runs the committed managed CJSI migration on one isolated
// acceptance root. It is built only by TestCJSISourceIssueAcceptance with an
// overlay that maps ../store_upgrade.go into internal/store.
//
//	upgrader <state-root> complete            (store.MigrateState)
//	upgrader <state-root> pause-before-ready  (migrateCJSIManagedRootWithReady)
//
// In pause mode the migration's beforeReady hook prints "CJSI-UPGRADE-PAUSED"
// and blocks on stdin: the fence and private index are installed and
// verified, the ready journal and initial head are not yet committed. The
// test kills the process there; closing stdin instead lets the commit proceed.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/kitdine/agent-deck/internal/store"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: upgrader <state-root> complete|pause-before-ready")
		os.Exit(2)
	}
	var pause func()
	switch os.Args[2] {
	case "complete":
	case "pause-before-ready":
		pause = func() {
			fmt.Println("CJSI-UPGRADE-PAUSED")
			_, _ = io.Copy(io.Discard, os.Stdin)
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown mode", os.Args[2])
		os.Exit(2)
	}
	// Lock and legacy-drain waits are the production ones (store.lockWait);
	// the acceptance root has no other process, so they never decide an
	// ordering.
	if err := store.CJSIAcceptanceUpgrade(context.Background(), os.Args[1], pause); err != nil {
		fmt.Fprintln(os.Stderr, "upgrade:", err)
		os.Exit(1)
	}
	fmt.Println("CJSI-UPGRADE-COMPLETE")
}
