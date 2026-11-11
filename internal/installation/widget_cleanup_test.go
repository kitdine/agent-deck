package installation

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWidgetCleanupRegistrationBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, query, diagnostic, queryStatus, removeStatus string
		wantError, wantRemove                              bool
	}{
		{name: "never_registered", removeStatus: "1"},
		{name: "empty_registry_summary", query: "summary", removeStatus: "1"},
		{name: "other_path", query: "other", removeStatus: "1"},
		{name: "registered", query: "registered", wantRemove: true},
		{name: "query_failure", queryStatus: "1", wantError: true},
		{name: "daemon_error_with_zero_exit", query: "invalid", wantError: true},
		{name: "other_path_with_stderr_diagnostic", query: "other", diagnostic: "match: Connection invalid", wantError: true},
		{name: "empty_summary_with_stderr_diagnostic", query: "summary", diagnostic: "match: Connection invalid", wantError: true},
		{name: "target_with_stderr_diagnostic", query: "registered", diagnostic: "match: Connection invalid", wantError: true},
		{name: "other_path_with_stdout_diagnostic", query: "other-invalid", wantError: true},
		{name: "empty_summary_with_stdout_diagnostic", query: "summary-invalid", wantError: true},
		{name: "unregister_failure", query: "registered", removeStatus: "1", wantError: true, wantRemove: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, root := installFixture(t)
			request := bundleRequest(t, root, "homebrew_cask", "widget")
			registered, err := r.Register(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			extension := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(request.Executable))), "Contents/PlugIns/AgentDeckWidget.appex")
			log := filepath.Join(root, "pluginkit.log")
			tool := filepath.Join(root, "pluginkit")
			program := `#!/bin/sh
printf '%s\n' "$*" >> "$ISO_WIDGET_LOG"
case "$1" in
  -m)
    if [ -n "$ISO_WIDGET_DIAGNOSTIC" ]; then
      printf '%s\n' "$ISO_WIDGET_DIAGNOSTIC" >&2
    fi
    case "$ISO_WIDGET_QUERY" in
      registered) printf '    Path = %s\n (1 plug-in)\n' "$ISO_WIDGET_EXTENSION" ;;
      other) printf '    Path = %s.other\n (1 plug-in)\n' "$ISO_WIDGET_EXTENSION" ;;
      summary) printf ' (0 plug-ins)\n' ;;
      invalid) printf 'match: Connection invalid\n' ;;
      other-invalid) printf '    Path = %s.other\n (1 plug-in)\nmatch: Connection invalid\n' "$ISO_WIDGET_EXTENSION" ;;
      summary-invalid) printf ' (0 plug-ins)\nmatch: Connection invalid\n' ;;
    esac
    exit "${ISO_WIDGET_QUERY_STATUS:-0}" ;;
  -r)
    if [ "${ISO_WIDGET_REMOVE_STATUS:-0}" != 0 ]; then
      printf 'remove: no plugin at %s\n' "$ISO_WIDGET_EXTENSION" >&2
    fi
    exit "${ISO_WIDGET_REMOVE_STATUS:-0}" ;;
esac
exit 2
`
			if err := os.WriteFile(tool, []byte(program), 0o755); err != nil {
				t.Fatal(err)
			}
			oldTool := pluginkitTool
			pluginkitTool = tool
			t.Cleanup(func() { pluginkitTool = oldTool })
			for key, value := range map[string]string{
				"ISO_WIDGET_LOG": log, "ISO_WIDGET_EXTENSION": extension, "ISO_WIDGET_QUERY": tc.query,
				"ISO_WIDGET_DIAGNOSTIC":   tc.diagnostic,
				"ISO_WIDGET_QUERY_STATUS": tc.queryStatus, "ISO_WIDGET_REMOVE_STATUS": tc.removeStatus,
			} {
				t.Setenv(key, value)
			}
			before := receiptBytes(t, r)
			manifest := readFile(t, request.CleanupManifest)
			result, err := r.Unregister(context.Background(), UnregisterRequest{
				Channel: request.Channel, Executable: request.Executable, ExpectedInstallation: registered.InstallationID,
				CleanupManifest: request.CleanupManifest, Remove: true,
			})
			if (err != nil) != tc.wantError {
				t.Fatalf("cleanup = %+v, %v; wantError=%v", result, err, tc.wantError)
			}
			if tc.wantError {
				appBinary := filepath.Join(root, "Applications/AgentDeck.app/Contents/MacOS/AgentDeck")
				if !bytes.Equal(before, receiptBytes(t, r)) || readFile(t, request.Executable) != "widget" || readFile(t, appBinary) != "widget" {
					t.Fatal("failed registry operation changed the App or receipt")
				}
				if _, err := os.Stat(extension); err != nil {
					t.Fatalf("failed registry operation removed the extension: %v", err)
				}
			} else if result.Outcome != OutcomeRemoved || receiptBytes(t, r) != nil {
				t.Fatalf("cleanup did not remove matching installation: %+v", result)
			} else if _, err := os.Stat(request.Executable); !os.IsNotExist(err) {
				t.Fatalf("App helper still exists: %v", err)
			}
			if readFile(t, request.CleanupManifest) != manifest {
				t.Fatal("cleanup changed immutable manifest")
			}
			calls := readFile(t, log)
			want := "-m -A -D -vv\n"
			if tc.wantRemove {
				want += "-r " + extension + "\n"
			}
			if calls != want || strings.Count(calls, "-r ") > 1 {
				t.Fatalf("registry calls = %q, want %q", calls, want)
			}
		})
	}
}
