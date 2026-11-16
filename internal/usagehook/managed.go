package usagehook

import (
	"path/filepath"
	"sort"
	"strings"
)

// ManagedCommandKind distinguishes the two client registrations AgentDeck owns.
type ManagedCommandKind string

const (
	ManagedHook       ManagedCommandKind = "hook"
	ManagedStatusLine ManagedCommandKind = "statusline"
)

// ManagedCommand is one AgentDeck-generated command found in a client
// configuration. Executable is empty for the PATH-based `agentdeck` form and
// the absolute program path otherwise.
type ManagedCommand struct {
	Client     Client
	Kind       ManagedCommandKind
	Executable string
}

// ManagedCommands enumerates AgentDeck's Hook and statusline commands from
// the Codex and Claude configurations under Home. It only reads: it never
// creates, repairs or rewrites a file, and an absent, unreadable or invalid
// configuration contributes no commands.
func (m *Manager) ManagedCommands() []ManagedCommand {
	var commands []ManagedCommand
	for _, client := range []Client{ClientCodex, ClientClaude} {
		snapshot, err := m.readSnapshot(m.path(client))
		if err != nil || !snapshot.exists {
			continue
		}
		if hooks, _, err := decodeHooks(snapshot.document); err == nil {
			events := make([]string, 0, len(hooks))
			for event := range hooks {
				events = append(events, event)
			}
			sort.Strings(events)
			for _, event := range events {
				for _, group := range hooks[event] {
					entries, _, err := decodeGroup(group)
					if err != nil {
						continue
					}
					for _, raw := range entries {
						command, ok := decodeStatusLineCommandEntry(raw)
						if !ok {
							continue
						}
						if executable, ok := parseManagedCommand(command, []string{"usage", "hook", "event", string(client)}); ok {
							commands = append(commands, ManagedCommand{Client: client, Kind: ManagedHook, Executable: executable})
						}
					}
				}
			}
		}
		if client != ClientClaude {
			continue
		}
		if raw, ok := snapshot.document[statusLineKey]; ok {
			if command, ok := decodeStatusLineCommandEntry(raw); ok {
				if executable, ok := parseManagedCommand(command, []string{"quota", "capture"}); ok && managedStatusLineCommand(command) {
					commands = append(commands, ManagedCommand{Client: client, Kind: ManagedStatusLine, Executable: executable})
				}
			}
		}
	}
	return commands
}

// parseManagedCommand accepts `<program> [--state-dir <dir>] <tail...>` where
// the program is the bare word `agentdeck` (returned as "") or an absolute
// path whose base name is agentdeck, plain or in shellQuote's encoding.
func parseManagedCommand(command string, tail []string) (string, bool) {
	executable, rest, quoted, ok := statusLineShellWord(strings.TrimSpace(command))
	if !ok {
		return "", false
	}
	switch {
	case executable == "agentdeck" && !quoted:
		executable = ""
	case filepath.IsAbs(executable) && filepath.Base(executable) == "agentdeck":
	default:
		return "", false
	}
	var args []string
	for rest != "" {
		if rest[0] != ' ' && rest[0] != '\t' {
			return "", false
		}
		rest = strings.TrimLeft(rest, " \t")
		if rest == "" {
			break
		}
		word, next, _, valid := statusLineShellWord(rest)
		if !valid {
			return "", false
		}
		args = append(args, word)
		rest = next
	}
	if len(args) == len(tail)+2 && args[0] == "--state-dir" && args[1] != "" {
		args = args[2:]
	}
	if len(args) != len(tail) {
		return "", false
	}
	for index := range tail {
		if args[index] != tail[index] {
			return "", false
		}
	}
	return executable, true
}
