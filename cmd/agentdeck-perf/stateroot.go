package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kitdine/agent-deck/internal/platform"
)

// The measured build decides where its default state lives: a build that is
// not the installed program keeps it in an isolated root, an older build in
// the legacy ~/.agentdeck. The batch restores templates into, resets and
// waits on exactly the root the measured binary reports for the batch HOME,
// never on an assumed name.
const (
	stateRootReported = "state_location" // desktop state-location
	stateRootLegacy   = "legacy_default" // a build without that command
	legacyStateName   = ".agentdeck"
)

// fixtureNames are the batch HOME's own entries: sources and the App's
// isolated group container and preferences. None of them is a state root.
var fixtureNames = map[string]bool{".codex": true, ".claude": true, "app-group": true, "Library": true, ".metadata_never_index": true}

type stateLocation struct {
	Root   string `json:"state_root"`
	Source string `json:"source"`
	Reason string `json:"reason,omitempty"`
}

// queryStateLocation runs binary's desktop state-location with the
// environment the App gives its helpers on the batch HOME. A build older
// than installed-state ownership has no such command: cobra then prints the
// desktop help and exits 0, and that build uses the legacy root. Any other
// failure, refusal or malformed answer is an error, never a default.
func queryStateLocation(binary, home, tz string) (stateLocation, error) {
	command := exec.Command(binary, "desktop", "state-location", "--format", "json")
	command.Env = []string{"HOME=" + home, "TZ=" + tz, "PATH=/usr/bin:/bin", "LANG=en_US_POSIX", "LC_ALL=en_US_POSIX"}
	addTestCLIEnvironment(command, binary)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return stateLocation{}, fmt.Errorf("%s desktop state-location: %v: %s", binary, err, firstLine(stderr.String()))
	}
	var envelope struct {
		SchemaVersion int    `json:"schema_version"`
		Command       string `json:"command"`
		Data          *struct {
			Installed bool   `json:"installed"`
			Reason    string `json:"reason"`
			StateRoot string `json:"state_root"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		if strings.Contains(stdout.String(), "Usage:\n  agentdeck desktop [command]") {
			physical, err := filepath.EvalSymlinks(home)
			if err != nil {
				return stateLocation{}, err
			}
			return confineStateRoot(home, stateLocation{Root: filepath.Join(physical, legacyStateName), Source: stateRootLegacy})
		}
		return stateLocation{}, fmt.Errorf("%s desktop state-location: malformed output: %s", binary, firstLine(stdout.String()))
	}
	if envelope.SchemaVersion != 1 || envelope.Command != "desktop.state-location" || envelope.Data == nil || envelope.Data.StateRoot == "" {
		return stateLocation{}, fmt.Errorf("%s desktop state-location: unexpected envelope", binary)
	}
	if envelope.Data.Installed {
		// The installed program owns the account's real state; a batch never
		// measures it.
		return stateLocation{}, fmt.Errorf("%s is the installed AgentDeck; measure an uninstalled build", binary)
	}
	return confineStateRoot(home, stateLocation{Root: envelope.Data.StateRoot, Source: stateRootReported, Reason: envelope.Data.Reason})
}

// confineStateRoot accepts a physical root that is a direct child of the
// batch HOME, is not one of the fixture's own entries, is not a link, and is
// not a protected real state path. Every destructive reset relies on it.
func confineStateRoot(home string, location stateLocation) (stateLocation, error) {
	root := location.Root
	physicalHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		return stateLocation{}, err
	}
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) != physicalHome || fixtureNames[filepath.Base(root)] {
		return stateLocation{}, fmt.Errorf("state root %q is not confined to the batch HOME %q", root, physicalHome)
	}
	if info, err := os.Lstat(root); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
		return stateLocation{}, fmt.Errorf("state root %q is not a directory", root)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return stateLocation{}, err
	}
	resolved, err := platform.ResolveStatePath(root)
	if err != nil {
		return stateLocation{}, fmt.Errorf("state root %q refused: %w", root, err)
	}
	if resolved != root {
		return stateLocation{}, fmt.Errorf("state root %q resolves to %q", root, resolved)
	}
	return location, nil
}

// bindStateRoot fixes the arm's state root from the measured binaries. An App
// condition asks the App's embedded helper, which chooses the App's state,
// and requires the CLI under test to agree: CLI runs, App and scan workers
// then share one root. Every run unit re-asks before its reset, so a reset
// HOME is resolved again and a changed answer stops the batch.
func bindStateRoot(cfg *runConfig, app bool) error {
	b := cfg.Batch
	binaries := []string{cfg.Agentdeck}
	if app {
		binaries = append([]string{filepath.Join(cfg.App, "Contents", "Helpers", "agentdeck")}, binaries...)
	}
	tz := cfg.TZ
	b.resolveState = func() (stateLocation, error) {
		var first stateLocation
		for i, binary := range binaries {
			location, err := queryStateLocation(binary, b.Home, tz)
			if err != nil {
				return stateLocation{}, err
			}
			if i == 0 {
				first = location
			} else if location.Root != first.Root {
				return stateLocation{}, fmt.Errorf("measured binaries disagree on the state root: %q and %q", first.Root, location.Root)
			}
		}
		return first, nil
	}
	b.State, b.StateSource = "", ""
	_, err := b.stateRoot()
	return err
}

// stateRoot re-resolves the bound state root before it is used.
func (b *batchState) stateRoot() (string, error) {
	if b.resolveState == nil {
		return "", errors.New("the measured state root is not bound")
	}
	location, err := b.resolveState()
	if err != nil {
		return "", err
	}
	if b.State != "" && location.Root != b.State {
		return "", fmt.Errorf("measured state root changed from %q to %q", b.State, location.Root)
	}
	b.State, b.StateSource = location.Root, location.Source
	return location.Root, nil
}

// pathAliases are the spellings one path can have in a process listing:
// itself, its resolution, and both forms of the /tmp link.
func pathAliases(path string) []string {
	aliases := []string{path}
	add := func(p string) {
		for _, known := range aliases {
			if known == p {
				return
			}
		}
		aliases = append(aliases, p)
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		add(resolved)
	}
	add(appPathForm(path))
	if rest, ok := strings.CutPrefix(path, "/tmp/"); ok {
		add("/private/tmp/" + rest)
	}
	return aliases
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
