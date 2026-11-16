package doctor

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/usagehook"
)

// currentExecutable is the owner program the managed commands are compared
// with; tests replace it.
var currentExecutable = func() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

// managedCommandOwnerCheck reads AgentDeck's Hook and statusline commands
// from the client configurations and reports absolute commands that point to
// another program. It only runs for the installed program and never reports
// a path.
func managedCommandOwnerCheck(home string) (Check, bool) {
	if !platform.CurrentInstallation().Installed {
		return Check{}, false
	}
	check := Check{Name: "managed_command_owner", Status: "ok"}
	commands := usagehook.New(usagehook.Environment{Home: home}).ManagedCommands()
	self, selfErr := currentExecutable()
	absolute, pathOnly := false, false
	var hookClients []string
	statusLine := false
	for _, command := range commands {
		if command.Executable == "" {
			pathOnly = true
			continue
		}
		absolute = true
		if selfErr == nil {
			if resolved, err := filepath.EvalSymlinks(command.Executable); err == nil && resolved == self {
				continue
			}
		}
		if command.Kind == usagehook.ManagedStatusLine {
			statusLine = true
		} else if !containsString(hookClients, string(command.Client)) {
			hookClients = append(hookClients, string(command.Client))
		}
	}
	affected := append([]string{}, hookClients...)
	if statusLine && !containsString(affected, string(usagehook.ClientClaude)) {
		affected = append(affected, string(usagehook.ClientClaude))
	}
	sortClients(affected)
	if len(affected) == 0 {
		if pathOnly && !absolute {
			check.Reason = "path_resolved"
		}
		return check, true
	}
	check.Status = "warning"
	check.Code = "managed_command_owner_mismatch"
	check.Reason = strings.Join(affected, ",")
	switch {
	case len(hookClients) == 2:
		check.Recovery = "agentdeck usage hook setup --client all"
	case len(hookClients) == 1:
		check.Recovery = "agentdeck usage hook setup --client " + hookClients[0]
	default:
		check.Recovery = "agentdeck desktop quota-statusline enable"
	}
	return check, true
}

// ManagedCommandOwnerNext is the text-only explanation for a mismatch.
func ManagedCommandOwnerNext(reason string) string {
	names := []string{}
	for _, client := range strings.Split(reason, ",") {
		switch client {
		case "claude":
			names = append(names, "Claude")
		case "codex":
			names = append(names, "Codex")
		}
	}
	return strings.Join(names, " and ") + " hooks point to another AgentDeck program and are ignored."
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

// sortClients keeps the contract order claude, codex.
func sortClients(values []string) {
	if len(values) == 2 && values[0] == "codex" {
		values[0], values[1] = values[1], values[0]
	}
}
