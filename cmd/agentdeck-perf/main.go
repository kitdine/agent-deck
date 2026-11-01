// Command agentdeck-perf prepares and evaluates scan-performance measurement
// batches. It is a development tool and is never part of a release package.
package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: agentdeck-perf <corpus|batch|fixture|append|otel|template|run|stats|compare> [flags]")
		return 2
	}
	switch args[0] {
	case "fixture":
		return runFixture(args[1:], stdout, stderr)
	case "corpus":
		return runCorpus(args[1:], stdout, stderr)
	case "append":
		return runAppend(args[1:], stdout, stderr)
	case "otel":
		return runOTel(args[1:], stdout, stderr)
	case "template":
		return runTemplate(args[1:], stdout, stderr)
	case "batch":
		if len(args) < 2 || args[1] != "prepare" {
			fmt.Fprintln(stderr, "usage: agentdeck-perf batch prepare [flags]")
			return 2
		}
		return runBatchPrepare(args[2:], stdout, stderr)
	case "run":
		return runBatchCommand(args[1:], stdout, stderr)
	case "compare":
		return runCompare(args[1:], stdout, stderr)
	case "stats":
		return runStatsCommand(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "agentdeck-perf: unknown command %q\n", args[0])
		return 2
	}
}
