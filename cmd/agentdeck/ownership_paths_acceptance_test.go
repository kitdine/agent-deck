//go:build agentdeck_acceptance && darwin

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/ownershiptest"
	"github.com/kitdine/agent-deck/internal/platform"
)

func TestOwnershipPhysicalAliasesBehindBarrier(t *testing.T) {
	for _, layout := range []string{"directory", "link-chain-unicode", "long-combining"} {
		t.Run(layout, func(t *testing.T) {
			f := ownershiptest.New(t)
			target := f.Real
			unicodeAlias := ""
			if layout != "directory" {
				name, alias := "Café", "Cafe\u0301"
				if layout == "long-combining" {
					name = "a" + strings.Repeat("\u0301", 30) + "\u0323"
					alias = "a\u0323" + strings.Repeat("\u0301", 30)
				}
				target = filepath.Join(f.Base, name)
				unicodeAlias = filepath.Join(f.Base, alias)
				if err := os.Rename(f.Real, target); err != nil {
					t.Fatal(err)
				}
				middle := filepath.Join(f.Base, "middle")
				if err := os.Symlink(target, middle); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(middle, f.Real); err != nil {
					t.Fatal(err)
				}
				f.Protected = append(f.Protected, middle, target)
			}
			inner := filepath.Join(f.Account, "inner")
			outside := filepath.Join(f.Base, "outside")
			for _, path := range []string{inner, outside} {
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			for link, destination := range map[string]string{filepath.Join(outside, "jump"): inner, filepath.Join(outside, "alias"): target, filepath.Join(f.Base, "account-alias"): f.Account} {
				if err := os.Symlink(destination, link); err != nil {
					t.Fatal(err)
				}
			}
			paths := map[string]string{
				"entrance": f.Real, "descendant": f.Real + "/sentinel", "physical-target": target + "/sentinel",
				"symlink": outside + "/alias/sentinel", "physical-dotdot": outside + "/jump/../.agentdeck/sentinel",
				"dotdot": inner + "/../.agentdeck/sentinel", "ancestor-link": filepath.Join(f.Base, "account-alias", ".agentdeck", "sentinel"),
				"case": filepath.Join(f.Account, ".AGENTDECK", "sentinel"),
			}
			if unicodeAlias != "" {
				paths["unicode"] = unicodeAlias + "/sentinel"
			}
			for name, path := range paths {
				t.Run(name, func(t *testing.T) {
					// Native identity is the independent oracle for volume-dependent aliases.
					info, statErr := os.Stat(path)
					sentinel, _ := os.Stat(filepath.Join(target, "sentinel"))
					rootInfo, _ := os.Stat(target)
					protected := statErr == nil && (os.SameFile(info, sentinel) || os.SameFile(info, rootInfo))
					if !protected && name != "case" && name != "unicode" {
						t.Fatalf("fixture is not protected: %s %v", name, statErr)
					}
					f.Controls(t, ownershipChild(f, []string{"--state-dir", f.Real, "version"}, false))
					command := ownershipChild(f, []string{"--state-dir", path, "version"}, false)
					f.Sandbox(command)
					out, err := command.CombinedOutput()
					want := 0
					if protected {
						want = 1
					}
					if command.ProcessState == nil || command.ProcessState.ExitCode() != want {
						t.Fatalf("alias %s: want exit %d, got %v %s", name, want, err, out)
					}
					if protected && !bytes.Contains(out, []byte("state_ownership_refused")) {
						t.Fatalf("alias not refused: %s", out)
					}
					t.Logf("native-equivalent=%t; guard and negative-access controls passed", protected)
				})
			}
		})
	}
}

type ownershipGolden struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	JSON   string `json:"json"`
	Exit   int    `json:"exit"`
}

func ownershipGoldens(t *testing.T) map[string]ownershipGolden {
	t.Helper()
	data, err := os.ReadFile("testdata/ownership-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]ownershipGolden
	if err = json.Unmarshal(data, &values); err != nil {
		t.Fatal(err)
	}
	return values
}

func TestOwnershipDoctorAndRefusalGoldenBehindBarrier(t *testing.T) {
	goldens := ownershipGoldens(t)
	for _, name := range []string{"refused_state", "doctor_not_installed", "doctor_formula_unclaimed", "doctor_refused"} {
		for _, format := range []string{"text", "json"} {
			t.Run(name+"/"+format, func(t *testing.T) {
				f := ownershiptest.New(t)
				executable := os.Args[0]
				if name == "doctor_formula_unclaimed" {
					executable = ownershipExecutable(t, f, "prefix/Cellar/agentdeck/1/bin/agentdeck")
				} else {
					owner := ownershipExecutable(t, f, "owner/agentdeck")
					registerOwnershipFixture(t, f, owner, "make")
				}
				args := []string{"--format", format, "doctor"}
				if name == "refused_state" {
					args = []string{"--format", format, "scan"}
				}
				if name == "refused_state" || name == "doctor_refused" {
					args = append([]string{"--state-dir", f.Real}, args...)
				}
				f.Controls(t, ownershipChild(f, []string{"--state-dir", f.Real, "version"}, false))
				command := ownershipChild(f, args, false)
				command.Path = executable
				command.Args[0] = executable
				f.Sandbox(command)
				var stdout, stderr bytes.Buffer
				command.Stdout = &stdout
				command.Stderr = &stderr
				err := command.Run()
				golden := goldens[name]
				if command.ProcessState == nil || command.ProcessState.ExitCode() != golden.Exit {
					t.Fatalf("wrong exit: %v %s %s", err, stdout.String(), stderr.String())
				}
				if format == "text" {
					if stdout.String() != golden.Stdout || stderr.String() != golden.Stderr {
						t.Fatalf("prototype mismatch\nstdout=%q\nwant=%q\nstderr=%q\nwant=%q", stdout.String(), golden.Stdout, stderr.String(), golden.Stderr)
					}
					return
				}
				actual := stdout.Bytes()
				if golden.Exit != 0 {
					actual = stderr.Bytes()
				}
				var got, want map[string]any
				if err = json.Unmarshal(actual, &got); err != nil {
					t.Fatal(err)
				}
				if err = json.Unmarshal([]byte(golden.JSON), &want); err != nil {
					t.Fatal(err)
				}
				delete(got, "generated_at")
				delete(want, "generated_at")
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("prototype JSON mismatch: got=%s want=%s", actual, golden.JSON)
				}
			})
		}
	}
	var revoked bytes.Buffer
	renderCommandErrorText(&revoked, &platform.ErrStateOwnershipRefused{Resource: "state_root", Reason: "receipt_other_executable", Revoked: true})
	if revoked.String() != goldens["revoked"].Stderr {
		t.Fatalf("revoked prototype mismatch: %q", revoked.String())
	}
}

func TestOwnershipNoticeOnlyOnceOnTextTTY(t *testing.T) {
	f := ownershiptest.New(t)
	golden := ownershipGoldens(t)["notice"]
	for _, tc := range []struct {
		name          string
		args          []string
		tty, wantNote bool
	}{
		{"text-tty", []string{"provider", "list"}, true, true},
		{"text-pipe", []string{"provider", "list"}, false, false},
		{"quiet", []string{"--quiet", "provider", "list"}, true, false},
		{"json", []string{"--format", "json", "provider", "list"}, true, false},
		{"ndjson", []string{"--format", "ndjson", "scan"}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f.Controls(t, ownershipChild(f, []string{"--state-dir", f.Real, "version"}, false))
			command := ownershipChild(f, tc.args, false)
			f.Sandbox(command)
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			var err error
			if tc.tty {
				master, slave := openSessionViewerPTY(t)
				command.Stderr = slave
				if err = command.Start(); err != nil {
					t.Fatal(err)
				}
				slave.Close()
				done := make(chan struct{})
				go func() { _, _ = io.Copy(&stderr, master); close(done) }()
				err = command.Wait()
				<-done
				master.Close()
			} else {
				command.Stderr = &stderr
				err = command.Run()
			}
			if err != nil {
				t.Fatalf("notice command failed: %v %s", err, stderr.String())
			}
			got := strings.ReplaceAll(stderr.String(), "\r\n", "\n")
			if tc.wantNote {
				if got != golden.Stderr || stdout.String() != golden.Stdout {
					t.Fatalf("notice differs: stderr=%q stdout=%q", got, stdout.String())
				}
			} else if strings.Contains(got, "note: not the installed") {
				t.Fatalf("unexpected notice: %q", got)
			}
		})
	}
}
