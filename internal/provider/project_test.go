package provider

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestProjectIdentityMatchesStoredSessionProject(t *testing.T) {
	ctx := context.Background()
	stateRoot := t.TempDir()
	home := filepath.Join(t.TempDir(), "home")
	cwd := filepath.Join(t.TempDir(), "parent", "..", "project")
	sourcePath := filepath.Join(home, ".claude", "projects", "fixture", "session.jsonl")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0700); err != nil {
		t.Fatal(err)
	}
	record := fmt.Sprintf(
		"{\"type\":\"user\",\"sessionId\":\"project-identity\",\"cwd\":%q,\"message\":{\"content\":\"visible prompt\"}}\n",
		cwd,
	)
	if err := os.WriteFile(sourcePath, []byte(record), 0600); err != nil {
		t.Fatal(err)
	}

	s, err := store.OpenSessions(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := session.Scan(ctx, s.DB, home); err != nil {
		t.Fatal(err)
	}
	result, err := session.Show(ctx, s.DB, "claude", "project-identity")
	if err != nil {
		t.Fatal(err)
	}

	if got, want := ProjectIdentity(cwd), result.Metadata.Project; got != want {
		t.Fatalf("ProjectIdentity(%q) = %q, stored session project = %q", cwd, got, want)
	}
	if got, want := result.Metadata.Project, filepath.Clean(cwd); got != want {
		t.Fatalf("stored session project = %q, want cleaned cwd %q", got, want)
	}
}

func TestProjectWireValueUsesOnlyThePercentEncodedBaseName(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{name: "space", base: "project name", want: "project%20name"},
		{name: "newline", base: "project\nname", want: "project%0Aname"},
		{name: "quote", base: "project\"name", want: "project%22name"},
		{name: "non ASCII", base: "项目", want: "%E9%A1%B9%E7%9B%AE"},
		{name: "plus", base: "my+project", want: "my%2Bproject"},
		{name: "multiple pluses", base: "c++", want: "c%2B%2B"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cwd := filepath.Join(t.TempDir(), tt.base)
			got := ProjectWireValue(cwd)
			if got != tt.want {
				t.Fatalf("ProjectWireValue(%q) = %q, want %q", cwd, got, tt.want)
			}
			if strings.Contains(got, "+") {
				t.Fatalf("ProjectWireValue(%q) contains a bare plus: %q", cwd, got)
			}
			decoded, err := url.PathUnescape(got)
			if err != nil {
				t.Fatal(err)
			}
			if decoded != tt.base {
				t.Fatalf("decoded wire value = %q, want %q", decoded, tt.base)
			}
		})
	}
}

func TestProjectWireValueAttributesNamelessDirectoriesToNothing(t *testing.T) {
	for _, cwd := range []string{"", ".", "..", filepath.Clean(string(filepath.Separator))} {
		t.Run(fmt.Sprintf("%q", cwd), func(t *testing.T) {
			if got := ProjectWireValue(cwd); got != "" {
				t.Fatalf("ProjectWireValue(%q) = %q, want empty", cwd, got)
			}
		})
	}
}
