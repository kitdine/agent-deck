package main

import (
	"bytes"
	"errors"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNetworkImportsAreLimitedToPriceUpdate(t *testing.T) {
	root := filepath.Join("..", "..")
	allowed := filepath.Clean(filepath.Join(root, "internal", "usage", "price_update.go"))
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || (!strings.HasPrefix(path, filepath.Join(root, "cmd")+string(os.PathSeparator)) && !strings.HasPrefix(path, filepath.Join(root, "internal")+string(os.PathSeparator))) || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			name := strings.Trim(imported.Path.Value, "\"")
			if name == "net" || name == "net/http" {
				if filepath.Clean(path) != allowed || name != "net/http" {
					return &networkImportError{path: path, name: name}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPrivacyGateScansRepositoryAndGeneratedArtifacts(t *testing.T) {
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "check-privacy.sh"))
	if err != nil {
		t.Fatal(err)
	}
	secret := "sk-" + "abcdefghijklmnopqrstuvwx"
	for _, test := range []struct {
		name    string
		path    string
		tracked bool
	}{
		{name: "tracked", path: "tracked.txt", tracked: true},
		{name: "untracked non-ignored", path: "untracked.txt"},
		{name: "database", path: "state/agentdeck.sqlite3"},
		{name: "backup", path: "backups/portable.adb"},
		{name: "fixture", path: "testdata/fixture.json"},
		{name: "captured output", path: "test-output/captured.log"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := initPrivacyTestRepository(t)
			path := filepath.Join(repository, test.path)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(secret), 0600); err != nil {
				t.Fatal(err)
			}
			if test.tracked {
				runTestGit(t, repository, "add", test.path)
			}
			command := exec.Command("bash", script)
			command.Dir = repository
			output, err := command.CombinedOutput()
			if err == nil || !bytes.Contains(output, []byte(test.path)) {
				t.Fatalf("privacy gate accepted %s secret: err=%v output=%q", test.name, err, output)
			}
			if bytes.Contains(output, []byte(secret)) {
				t.Fatalf("privacy gate exposed matched content for %s: %q", test.name, output)
			}
		})
	}

	t.Run("ignored file", func(t *testing.T) {
		repository := initPrivacyTestRepository(t)
		if err := os.WriteFile(filepath.Join(repository, ".gitignore"), []byte("ignored.txt\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repository, "ignored.txt"), []byte(secret), 0600); err != nil {
			t.Fatal(err)
		}
		command := exec.Command("bash", script)
		command.Dir = repository
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("privacy gate scanned ignored file: %v: %s", err, output)
		}
	})

	t.Run("deleted tracked file", func(t *testing.T) {
		repository := initPrivacyTestRepository(t)
		path := filepath.Join(repository, "removed.txt")
		if err := os.WriteFile(path, []byte("synthetic public content"), 0600); err != nil {
			t.Fatal(err)
		}
		runTestGit(t, repository, "add", "removed.txt")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		command := exec.Command("bash", script)
		command.Dir = repository
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("privacy gate rejected a deleted tracked file: %v: %s", err, output)
		}
	})
}

func TestPrivacyGateFailsClosed(t *testing.T) {
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "check-privacy.sh"))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("scanner failure", func(t *testing.T) {
		repository := initPrivacyTestRepository(t)
		if err := os.WriteFile(filepath.Join(repository, "tracked.txt"), []byte("synthetic private content"), 0600); err != nil {
			t.Fatal(err)
		}
		runTestGit(t, repository, "add", "tracked.txt")
		bin := filepath.Join(t.TempDir(), "bin")
		if err := os.MkdirAll(bin, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bin, "grep"), []byte("#!/bin/sh\nexit 2\n"), 0700); err != nil {
			t.Fatal(err)
		}
		command := exec.Command("bash", script)
		command.Dir = repository
		command.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		output, err := command.CombinedOutput()
		assertPrivacyGateFailure(t, err, output, "privacy scan failed: unable to inspect repository files")
	})

	t.Run("git enumeration failure", func(t *testing.T) {
		command := exec.Command("bash", script)
		command.Dir = t.TempDir()
		output, err := command.CombinedOutput()
		assertPrivacyGateFailure(t, err, output, "privacy scan failed: unable to enumerate repository files")
	})
}

func assertPrivacyGateFailure(t *testing.T, err error, output []byte, diagnostic string) {
	t.Helper()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("privacy gate error = %v, output=%q; want exit 2", err, output)
	}
	if string(bytes.TrimSpace(output)) != diagnostic {
		t.Fatalf("privacy gate diagnostic = %q, want %q", output, diagnostic)
	}
	if bytes.Contains(output, []byte("synthetic private content")) {
		t.Fatalf("privacy gate diagnostic exposed file content: %q", output)
	}
}

func initPrivacyTestRepository(t *testing.T) string {
	t.Helper()
	repository := t.TempDir()
	runTestGit(t, repository, "init", "--quiet")
	return repository
}

func runTestGit(t *testing.T, repository string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = repository
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}

type networkImportError struct{ path, name string }

func (e *networkImportError) Error() string {
	return "network import outside explicit price update: " + e.path + " imports " + e.name
}
