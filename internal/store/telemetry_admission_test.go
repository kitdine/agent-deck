package store

import (
	"strings"
	"testing"
)

func TestTelemetryAdmittedIsFamilyAndShapeNotVersion(t *testing.T) {
	admitted := func(producer, version, originator, transport, name, kind string) bool {
		return TelemetryAdmitted(producer, version, originator, transport, name, kind)
	}
	for _, version := range []string{"0.165.0", "0.169.0", "2.0.0-rc.1", "x", strings.Repeat("v", 256), strings.Repeat("é", 128)} {
		if !admitted("codex_exec", version, "codex_exec", "sse", "codex.sse_event", "response.completed") ||
			!admitted("codex-app-server", version, "codex-tui", "sse", "codex.sse_event", "response.completed") {
			t.Errorf("version %q not admitted", version)
		}
	}
	rejected := [][6]string{
		{"codex_exec", "", "codex_exec", "sse", "codex.sse_event", "response.completed"},
		{"codex_exec", "0.169.0/", "codex_exec", "sse", "codex.sse_event", "response.completed"},
		{"codex_exec", strings.Repeat("v", 257), "codex_exec", "sse", "codex.sse_event", "response.completed"},
		{"codex_exec", strings.Repeat("é", 128) + "1", "codex_exec", "sse", "codex.sse_event", "response.completed"},
		{"codex_exec", "0.169.0", "codex-tui", "sse", "codex.sse_event", "response.completed"},
		{"codex-app-server", "0.169.0", "codex_exec", "sse", "codex.sse_event", "response.completed"},
		{"codex-app-server", "0.169.0", "Codex Desktop", "sse", "codex.sse_event", "response.completed"},
		{"", "0.169.0", "", "sse", "codex.sse_event", "response.completed"},
		{"codex_exec", "0.169.0", "codex_exec", "websocket", "codex.sse_event", "response.completed"},
		{"codex_exec", "0.169.0", "codex_exec", "sse", "codex.websocket_event", "response.completed"},
		{"codex_exec", "0.169.0", "codex_exec", "sse", "codex.sse_event", "response.created"},
	}
	for _, c := range rejected {
		if admitted(c[0], c[1], c[2], c[3], c[4], c[5]) {
			t.Errorf("admitted %q", c)
		}
	}
}
