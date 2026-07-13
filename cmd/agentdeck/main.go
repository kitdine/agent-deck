package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jobshen/agentdeck/internal/output"
	"github.com/jobshen/agentdeck/internal/platform"
	"github.com/jobshen/agentdeck/internal/provider"
	"github.com/jobshen/agentdeck/internal/session"
	"github.com/jobshen/agentdeck/internal/store"
	"github.com/jobshen/agentdeck/internal/usage"
)

// userHomeDir is injectable so command tests never traverse a real user's
// Codex or Claude logs.
var userHomeDir = os.UserHomeDir

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	stateDir, format, args, err := globals(args)
	if err != nil {
		return err
	}
	if len(args) < 1 {
		return fmt.Errorf("usage: agentdeck <provider|usage|session|run> <command>")
	}
	if stateDir == "" {
		home, err := userHomeDir()
		if err != nil {
			return err
		}
		stateDir = platform.StateRoot("", home)
	}
	if args[0] == "session" {
		return runSession(context.Background(), stateDir, format, args[1:], stdout)
	}
	database, err := store.Open(context.Background(), stateDir)
	if err != nil {
		return err
	}
	defer database.Close()
	if args[0] == "usage" {
		return runUsage(context.Background(), database, stateDir, format, args[1:], stdout)
	}
	if args[0] == "run" {
		return runClient(context.Background(), database, format, args[1:], stdout)
	}
	if len(args) < 2 {
		return fmt.Errorf("usage: agentdeck <provider|usage|session|run> <command>")
	}
	if args[0] != "provider" {
		return fmt.Errorf("usage: agentdeck <provider|usage|session> <command>")
	}
	service := provider.Service{Store: database, Secrets: platform.NewKeychainSecretStore("com.agentdeck.provider")}
	command := args[1]
	values := args[2:]
	var data any
	switch command {
	case "list", "status":
		data, err = service.List(context.Background())
	case "show":
		if len(values) != 1 {
			return fmt.Errorf("usage: provider show <name>")
		}
		var statuses []provider.Status
		statuses, err = service.List(context.Background())
		for _, status := range statuses {
			if status.Definition.Name == values[0] {
				data = status
				break
			}
		}
		if data == nil && err == nil {
			return fmt.Errorf("provider not found")
		}
	case "add":
		if len(values) < 5 {
			return fmt.Errorf("usage: provider add <name> <endpoint> <credential-ref> <multiplier> <codex|claude>")
		}
		credential, readErr := readCredential(stdin)
		if readErr != nil {
			return readErr
		}
		data, err = service.Add(context.Background(), provider.Definition{Name: values[0], Endpoint: values[1], CredentialRef: values[2], Multiplier: values[3], Clients: []provider.Client{provider.Client(values[4])}}, credential)
	case "edit":
		if len(values) < 5 {
			return fmt.Errorf("usage: provider edit <name> <endpoint> <credential-ref> <multiplier> <codex|claude>")
		}
		data, err = service.Edit(context.Background(), provider.Definition{Name: values[0], Endpoint: values[1], CredentialRef: values[2], Multiplier: values[3], Clients: []provider.Client{provider.Client(values[4])}}, "")
	case "remove":
		if len(values) != 2 {
			return fmt.Errorf("usage: provider remove <name> <credential-ref>")
		}
		err = service.Remove(context.Background(), values[0], values[1])
	case "credential":
		if len(values) < 1 || (values[0] != "add" && values[0] != "update" && values[0] != "remove" && values[0] != "list") {
			return fmt.Errorf("usage: provider credential <add|update|remove|list> [reference]")
		}
		if values[0] == "list" {
			if len(values) != 1 {
				return fmt.Errorf("usage: provider credential list")
			}
			data, err = service.List(context.Background())
		} else if len(values) != 2 {
			return fmt.Errorf("credential reference is required")
		} else if values[0] == "remove" {
			err = service.RemoveCredential(context.Background(), values[1])
		} else {
			var credential string
			credential, err = readCredential(stdin)
			if err == nil {
				err = service.UpdateCredential(context.Background(), values[1], credential)
			}
		}
	case "use":
		if len(values) != 4 {
			return fmt.Errorf("usage: provider use <name> <codex|claude> <config-path> <redacted-backup-path>")
		}
		err = service.Use(context.Background(), values[0], provider.Client(values[1]), values[2], values[3])
	case "recover":
		data, err = service.Recover(context.Background())
	default:
		return fmt.Errorf("unknown provider command %q", command)
	}
	if err != nil {
		return err
	}
	if format == "json" {
		return json.NewEncoder(stdout).Encode(output.New("provider."+command, data, time.Now()))
	}
	if data != nil {
		return json.NewEncoder(stdout).Encode(data)
	}
	return nil
}

func runSession(ctx context.Context, stateDir, format string, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agentdeck session <scan|list|search|show|exclude|rebuild|purge-index>")
	}
	if err := platform.EnsureStateRoot(stateDir); err != nil {
		return err
	}
	lock, err := store.AcquireLock(ctx, stateDir, 5*time.Second)
	if err != nil {
		return err
	}
	defer lock.Release()
	command := args[0]
	if command == "purge-index" {
		for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
			if err := os.Remove(filepath.Join(stateDir, "sessions.sqlite3") + suffix); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return writeSession(stdout, format, command, map[string]any{"purged": true})
	}
	sessions, err := store.OpenSessions(ctx, stateDir)
	if err != nil {
		return err
	}
	defer sessions.Close()
	home, err := userHomeDir()
	if err != nil {
		return err
	}
	var data any
	switch command {
	case "scan":
		data, err = session.Scan(ctx, sessions.DB, home)
	case "list":
		data, err = session.List(ctx, sessions.DB)
	case "search":
		if len(args) != 2 {
			return fmt.Errorf("usage: agentdeck session search <query>")
		}
		data, err = session.Search(ctx, sessions.DB, args[1])
	case "show":
		if len(args) != 3 {
			return fmt.Errorf("usage: agentdeck session show <codex|claude> <session-id>")
		}
		data, err = session.Show(ctx, sessions.DB, args[1], args[2])
	case "exclude":
		if len(args) != 3 {
			return fmt.Errorf("usage: agentdeck session exclude <project|path|session|client> <value>")
		}
		err = session.Exclude(ctx, sessions.DB, args[1], args[2])
		data = map[string]any{"excluded": err == nil}
	case "rebuild":
		data, err = session.Rebuild(ctx, sessions.DB, home)
	default:
		return fmt.Errorf("usage: agentdeck session <scan|list|search|show|exclude|rebuild|purge-index>")
	}
	if err != nil {
		return err
	}
	return writeSession(stdout, format, command, data)
}

func writeSession(w io.Writer, format, command string, data any) error {
	if format == "json" {
		return json.NewEncoder(w).Encode(output.New("session."+command, data, time.Now()))
	}
	return json.NewEncoder(w).Encode(data)
}

func runUsage(ctx context.Context, database *store.Store, stateDir, format string, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agentdeck usage <scan|diagnose|rebuild|price>")
	}
	home, err := userHomeDir()
	if err != nil {
		return err
	}
	service := usage.New(database, home)
	command := args[0]
	var data any
	partial := false
	warnings := []string{}
	switch command {
	case "scan":
		data, err = service.Scan(ctx)
	case "summary":
		// Reports opportunistically refresh their local log view. An unreadable
		// source preserves the last committed report but is never hidden.
		if _, scanErr := service.Scan(ctx); scanErr != nil {
			partial, warnings = true, []string{"scan_incomplete"}
		}
		data, err = service.Summary(ctx)
	case "sessions":
		data, err = service.Sessions(ctx)
	case "diagnose":
		data, err = service.Diagnose(ctx)
	case "rebuild":
		if _, err = database.Exec(ctx, "DELETE FROM usage_events; DELETE FROM usage_sessions; DELETE FROM usage_source_files"); err == nil {
			data, err = service.Scan(ctx)
		}
	case "price":
		if len(args) < 2 {
			return fmt.Errorf("usage: agentdeck usage price <status|update|history|override>")
		}
		switch args[1] {
		case "history":
			if len(args) != 2 {
				return fmt.Errorf("usage: agentdeck usage price history")
			}
			data, err = service.PriceHistory(ctx)
		case "status":
			if len(args) != 2 {
				return fmt.Errorf("usage: agentdeck usage price status")
			}
			data, err = service.PriceStatus(ctx)
		case "update":
			if len(args) != 6 || args[2] != "--url" || args[4] != "--commit" {
				return fmt.Errorf("usage: agentdeck usage price update --url <pinned-url> --commit <sha>")
			}
			data, err = service.UpdateLiteLLM(ctx, args[3], args[5], nil)
		case "override":
			if len(args) != 4 || args[2] != "--file" {
				return fmt.Errorf("usage: agentdeck usage price override --file <official-components.json>")
			}
			contents, readErr := os.ReadFile(args[3])
			if readErr != nil {
				return readErr
			}
			var overrides []usage.OfficialOverride
			if err = json.Unmarshal(contents, &overrides); err == nil {
				err = service.ImportOfficialOverrides(ctx, overrides)
			}
			if err == nil {
				data = map[string]any{"overrides": len(overrides)}
			}
		default:
			return fmt.Errorf("usage: agentdeck usage price <status|update|history|override>")
		}
	default:
		return fmt.Errorf("usage commands currently available: scan, summary, sessions, diagnose, rebuild, price status|update|history|override")
	}
	if err != nil {
		return err
	}
	if format == "json" {
		envelope := output.New("usage."+command, data, time.Now())
		envelope.Partial, envelope.Warnings = partial, warnings
		return json.NewEncoder(stdout).Encode(envelope)
	}
	return renderUsageText(stdout, command, data)
}

func renderUsageText(w io.Writer, command string, data any) error {
	switch v := data.(type) {
	case usage.Summary:
		_, e := fmt.Fprintf(w, "events: %d\ntokens: %v\ncatalog base cost: %v\nprovider cost: %v\nwarnings: %v\nunpriced: %v\n", v.Counts["events"], v.Tokens, v.CatalogBaseCost, v.ProviderCost, v.Warnings, v.Unpriced)
		return e
	case []usage.SessionSummary:
		for _, x := range v {
			if _, e := fmt.Fprintf(w, "%s %s %s..%s tokens=%v base=%v provider=%v warnings=%v unpriced=%v\n", x.Client, x.SessionID, x.FirstAt, x.LastAt, x.Tokens, x.CatalogBaseCost, x.ProviderCost, x.Warnings, x.Unpriced); e != nil {
				return e
			}
		}
		return nil
	default:
		return json.NewEncoder(w).Encode(data)
	}
}
func runClient(ctx context.Context, database *store.Store, format string, args []string, stdout io.Writer) error {
	if len(args) < 3 || (args[0] != "codex" && args[0] != "claude") || args[1] != "--" {
		return fmt.Errorf("usage: agentdeck run <codex|claude> -- <client arguments>")
	}
	service := usage.New(database, "")
	runID, start, err := service.StartRun(ctx, args[0], 0)
	if err != nil {
		return err
	}
	child := exec.CommandContext(ctx, args[0], args[2:]...)
	child.Stdin = os.Stdin
	child.Stdout = stdout
	child.Stderr = os.Stderr
	if err = child.Start(); err != nil {
		_ = service.EndRun(ctx, runID, args[0], start)
		return err
	}
	if err = service.SetRunPID(ctx, runID, child.Process.Pid); err != nil {
		return err
	}
	waitErr := child.Wait()
	scanErr := error(nil)
	if home, homeErr := userHomeDir(); homeErr == nil {
		service.Home = home
		_, scanErr = service.Scan(ctx)
	}
	endErr := service.EndRun(ctx, runID, args[0], start)
	if scanErr != nil {
		return scanErr
	}
	if endErr != nil {
		return endErr
	}
	exact, reason, statusErr := service.RunStatus(ctx, runID)
	if statusErr != nil {
		return statusErr
	}
	if format == "json" {
		_ = json.NewEncoder(stdout).Encode(output.New("run."+args[0], map[string]any{"run_id": runID, "exact": exact, "attribution": map[bool]string{true: "exact", false: "estimated"}[exact], "reason": reason}, time.Now()))
	}
	return waitErr
}

func globals(args []string) (string, string, []string, error) {
	stateDir, format := "", "text"
	for len(args) > 0 && strings.HasPrefix(args[0], "--") {
		if len(args) < 2 {
			return "", "", nil, fmt.Errorf("missing value for %s", args[0])
		}
		switch args[0] {
		case "--state-dir":
			stateDir = args[1]
		case "--format":
			format = args[1]
		default:
			return "", "", nil, fmt.Errorf("unknown flag %s", args[0])
		}
		args = args[2:]
	}
	if format != "text" && format != "json" {
		return "", "", nil, fmt.Errorf("invalid format")
	}
	return stateDir, format, args, nil
}
func readCredential(reader io.Reader) (string, error) {
	if file, ok := reader.(*os.File); ok {
		info, err := file.Stat()
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeCharDevice != 0 {
			return "", fmt.Errorf("credential must be supplied through non-interactive stdin")
		}
	}
	contents, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(contents))
	if value == "" {
		return "", fmt.Errorf("credential is empty")
	}
	return value, nil
}
