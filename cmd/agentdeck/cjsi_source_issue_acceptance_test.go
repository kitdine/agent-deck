package main

// TestCJSISourceIssueAcceptance is Task 10 source-issue-acceptance: the
// combined acceptance matrix of docs/topics/codex-jsonl-session-identity
// tasks.md ("需求、回归与发布验收映射" A1–A7 and "综合测试命令与隔离").
//
// Unit 10a provides the isolated harness, the real old-binary fence matrix
// over the committed managed layout install (store.MigrateState and its
// beforeReady hook), and the A1–A5 observations that depend only on committed
// work. Every subtest that needs Task 8's production selector, final CAS and
// CLI gates (2b-1, 2b-2a; not committed at base 0d91b76) is an explicit
// PENDING skip that names the dependency; a skip never stands in for a pass.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
)

func TestCJSISourceIssueAcceptance(t *testing.T) {
	upgrader := &cjsiAccUpgrader{dir: t.TempDir()}

	t.Run("OldBinaryFence", func(t *testing.T) { cjsiAccOldBinaryFence(t, upgrader) })
	t.Run("IdentityAndAccounting", cjsiAccIdentityAndAccounting)
	t.Run("OTelOwnership", func(t *testing.T) { cjsiAccOTelOwnership(t, upgrader) })
	t.Run("DetailOrder", cjsiAccDetailOrder)
	t.Run("SlotDurability", cjsiAccSlotDurability)
	t.Run("ReplayAndHistoricalAbsence", cjsiAccReplayAndHistoricalAbsence)
	t.Run("PortableRecoveryAndPrivacy", cjsiAccPortableRecoveryAndPrivacy)
	t.Run("BuildScriptRefusesTmp", cjsiAccBuildScriptSelfTest)
	t.Run("HostProbeGuard", cjsiAccHostProbeGuard)
	t.Run("StateOracle", cjsiAccStateOracle)
}

// cjsiAccStateOracle exercises the state comparison on synthetic directories
// only: every change the fence matrix must catch is reported, and only the
// protocol changes the rules allow pass.
func cjsiAccStateOracle(t *testing.T) {
	write := func(t *testing.T, path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fixture := func(t *testing.T) (string, string) {
		t.Helper()
		dir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		state := filepath.Join(dir, "state")
		if err := os.Mkdir(state, 0o700); err != nil {
			t.Fatal(err)
		}
		return state, filepath.Join(dir, "home")
	}
	expectDiff := func(t *testing.T, state string, before cjsiAccSnapshot, want bool) {
		t.Helper()
		diffs := cjsiAccSideEffects(before, cjsiAccSnapshotState(t, state))
		if (len(diffs) > 0) != want {
			t.Fatalf("reported=%v want %v: %v", len(diffs) > 0, want, diffs)
		}
	}
	deadPID := func(t *testing.T) int {
		t.Helper()
		cmd := exec.Command("/usr/bin/true")
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		return cmd.Process.Pid
	}
	t.Run("PrivateIndexSameSizeAndMtime", func(t *testing.T) {
		state, _ := fixture(t)
		path := filepath.Join(state, "session-index-v1.sqlite3")
		write(t, path, "synthetic-bytes-1")
		fixed := time.Unix(1234567890, 0)
		if err := os.Chtimes(path, fixed, fixed); err != nil {
			t.Fatal(err)
		}
		before := cjsiAccSnapshotState(t, state)
		write(t, path, "synthetic-bytes-2")
		if err := os.Chtimes(path, fixed, fixed); err != nil {
			t.Fatal(err)
		}
		expectDiff(t, state, before, true)
	})
	for _, name := range []string{".scan-receipts-unexpected", ".scan-runtime-parent", "rogue.lock", "session-index-v1.sqlite3-shm", "rogue-dir"} {
		t.Run("NewPath/"+name, func(t *testing.T) {
			state, _ := fixture(t)
			before := cjsiAccSnapshotState(t, state)
			if name == "rogue-dir" {
				if err := os.Mkdir(filepath.Join(state, name), 0o700); err != nil {
					t.Fatal(err)
				}
			} else {
				write(t, filepath.Join(state, name), "synthetic")
			}
			expectDiff(t, state, before, true)
		})
	}
	t.Run("Locks", func(t *testing.T) {
		for _, tc := range []struct {
			name          string
			before, after string // "" = absent
			recorded      bool   // the harness recorded it as its own stale lock
			want          bool
		}{
			{"new arbitrary content", "", "unexpected synthetic bytes", false, true},
			{"new valid token with a dead owner (R10)", "", fmt.Sprintf("v1:%d:%032x", deadPID(t), 1), false, true},
			{"new valid token held at quiescence", "", fmt.Sprintf("v1:%d:%032x", os.Getpid(), 5), false, true},
			{"recorded upgrader lock reclaimed", fmt.Sprintf("v1:%d:%032x", deadPID(t), 2), "", true, false},
			{"unrecorded dead-owner lock reclaimed", fmt.Sprintf("v1:%d:%032x", deadPID(t), 6), "", false, true},
			{"live owner removed", fmt.Sprintf("v1:%d:%032x", os.Getpid(), 3), "", false, true},
			{"garbage removed", "not a token", "", false, true},
			{"rewritten in place", fmt.Sprintf("v1:%d:%032x", deadPID(t), 4), "other bytes", true, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				state, _ := fixture(t)
				path := filepath.Join(state, "scan.lock")
				if tc.before != "" {
					write(t, path, tc.before)
				}
				before := cjsiAccSnapshotState(t, state)
				if tc.recorded {
					cjsiAccKnownStaleLocks.Store(cjsiAccLockKey(path, before["scan.lock"].ino, []byte(tc.before)), true)
				}
				switch {
				case tc.after == "":
					_ = os.Remove(path)
				default:
					write(t, path, tc.after)
				}
				expectDiff(t, state, before, tc.want)
			})
		}
	})
	t.Run("NonRegularLocks", func(t *testing.T) {
		// R12: a lock path that is not a regular file is reported by the
		// observer at once and by the comparison; nothing ever opens it.
		for _, kind := range []string{"fifo", "directory", "symlink"} {
			t.Run(kind, func(t *testing.T) {
				state, _ := fixture(t)
				before := cjsiAccSnapshotState(t, state)
				lock := filepath.Join(state, "scan.lock")
				switch kind {
				case "fifo":
					if err := syscall.Mkfifo(lock, 0o600); err != nil {
						t.Fatal(err)
					}
				case "directory":
					if err := os.Mkdir(lock, 0o700); err != nil {
						t.Fatal(err)
					}
				case "symlink":
					target := filepath.Join(filepath.Dir(state), "elsewhere")
					write(t, target, fmt.Sprintf("v1:%d:%032x", os.Getpid(), 7))
					if err := os.Symlink(target, lock); err != nil {
						t.Fatal(err)
					}
				}
				done := make(chan error, 1)
				go func() { done <- (&cjsiAccRoot{state: state}).quiesceErr() }()
				select {
				case err := <-done:
					if !errors.Is(err, errCJSIAccNotRegular) {
						t.Fatalf("observer did not report the %s lock: %v", kind, err)
					}
				case <-time.After(cjsiAccCommandGuard):
					t.Fatalf("observer blocked on the %s lock", kind)
				}
				expectDiff(t, state, before, true)
			})
		}
		t.Run("fifo receipt journal", func(t *testing.T) {
			state, _ := fixture(t)
			before := cjsiAccSnapshotState(t, state)
			if err := syscall.Mkfifo(filepath.Join(state, "scan-receipts.json"), 0o600); err != nil {
				t.Fatal(err)
			}
			expectDiff(t, state, before, true)
		})
	})
	t.Run("StrictJSON", func(t *testing.T) {
		// The validator must accept exactly what the production reader
		// (json.Unmarshal) accepts at the document's end.
		valid := `{"version":1,"entries":[]}`
		for _, tc := range []struct {
			name string
			doc  string
			ok   bool
		}{
			{"trailing whitespace", valid + " \n\t\r\n", true},
			{"no trailing data", valid, true},
			{"stray closing bracket", valid + "]", false},
			{"stray closing brace", valid + "}", false},
			{"stray comma", valid + ",", false},
			{"second value", valid + valid, false},
			{"whitespace then garbage", valid + "\n x", false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				var journal struct {
					Version int               `json:"version"`
					Entries []json.RawMessage `json:"entries"`
				}
				err := cjsiAccStrictJSON([]byte(tc.doc), &journal)
				production := json.Unmarshal([]byte(tc.doc), &journal)
				if (err == nil) != tc.ok || (production == nil) != tc.ok {
					t.Fatalf("strict=%v production=%v, want accepted=%v", err, production, tc.ok)
				}
			})
		}
	})
	t.Run("ReceiptIdentity", func(t *testing.T) {
		// R13: the journal is only ever replaced by rename to change its
		// content. Same inode with new bytes, or new inode with the same
		// bytes, is someone else's write; a renamed-in update that adds a
		// valid no-op receipt is the protocol.
		entry := func(home, id string) string {
			return fmt.Sprintf(`{"id":%q,"state_id":"s","home":%q,"scope":"both","round_id":%q,"observation":1,"state":"terminal","round_terminal":true,"accepted_at":"2026-01-02T00:00:00Z","terminal_at":"2026-01-02T00:00:01Z","result":{"round_id":%q,"started_at":"2026-01-02T00:00:00Z","completed_at":"2026-01-02T00:00:01Z","usage":{"state":"failed","error_code":"schema_ahead","duration_ms":0},"session":{"state":"failed","scan":{"sources":0,"documents":0,"skipped":0,"removed":0},"error_code":"scan_failed","duration_ms":0},"stages":{"total_ms":1}}}`, id, home, id, id)
		}
		for _, tc := range []struct {
			name   string
			change func(t *testing.T, journal, home string)
			want   bool
		}{
			{"deleted and rebuilt with the same bytes", func(t *testing.T, journal, home string) {
				content, _, err := cjsiAccReadRegular(journal, cjsiAccStateReadLimit)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(journal); err != nil {
					t.Fatal(err)
				}
				write(t, journal, string(content))
			}, true},
			{"rewritten in place with a valid extra receipt", func(t *testing.T, journal, home string) {
				file, err := os.OpenFile(journal, os.O_WRONLY|os.O_TRUNC, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				if _, err := file.WriteString(`{"version":1,"entries":[` + entry(home, "a") + "," + entry(home, "b") + `]}`); err != nil {
					t.Fatal(err)
				}
			}, true},
			{"renamed-in update adding a valid receipt with a stray ]", func(t *testing.T, journal, home string) {
				temporary := journal + ".synthetic-temp"
				write(t, temporary, `{"version":1,"entries":[`+entry(home, "a")+","+entry(home, "b")+`]}]`)
				if err := os.Rename(temporary, journal); err != nil {
					t.Fatal(err)
				}
			}, true},
			{"renamed-in update adding a valid receipt", func(t *testing.T, journal, home string) {
				temporary := journal + ".synthetic-temp"
				write(t, temporary, `{"version":1,"entries":[`+entry(home, "a")+","+entry(home, "b")+`]}`)
				if err := os.Rename(temporary, journal); err != nil {
					t.Fatal(err)
				}
			}, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				state, home := fixture(t)
				journal := filepath.Join(state, "scan-receipts.json")
				write(t, journal, `{"version":1,"entries":[`+entry(home, "a")+`]}`)
				before := cjsiAccSnapshotState(t, state)
				tc.change(t, journal, home)
				expectDiff(t, state, before, tc.want)
			})
		}
	})
	t.Run("Receipts", func(t *testing.T) {
		receipt := func(home, usage, session string, extra string) string {
			return fmt.Sprintf(`{"id":"r1","state_id":"s","home":%q,"scope":"both","round_id":"x","observation":1,"state":"terminal","round_terminal":true,"accepted_at":"2026-01-02T00:00:00Z","terminal_at":"2026-01-02T00:00:01Z","result":{"round_id":"x","started_at":"2026-01-02T00:00:00Z","completed_at":"2026-01-02T00:00:01Z","usage":%s,"session":%s,"stages":{"total_ms":1}}%s}`, home, usage, session, extra)
		}
		failedUsage := `{"state":"failed","error_code":"schema_ahead","duration_ms":0}`
		failedSession := `{"state":"failed","scan":{"sources":0,"documents":0,"skipped":0,"removed":0},"error_code":"scan_failed","duration_ms":0}`
		for _, tc := range []struct {
			name string
			doc  func(home string) string
			want bool
		}{
			{"no-op failed round", func(h string) string {
				return `{"version":1,"entries":[` + receipt(h, failedUsage, failedSession, "") + `]}`
			}, false},
			{"completed session", func(h string) string {
				return `{"version":1,"entries":[` + receipt(h, failedUsage, `{"state":"completed","scan":{"sources":1,"documents":1,"skipped":0,"removed":0},"duration_ms":1}`, "") + `]}`
			}, true},
			{"other home", func(string) string {
				return `{"version":1,"entries":[` + receipt("/elsewhere", failedUsage, failedSession, "") + `]}`
			}, true},
			{"unknown field", func(h string) string {
				return `{"version":1,"entries":[` + receipt(h, failedUsage, failedSession, `,"payload":"x"`) + `]}`
			}, true},
			{"not json", func(string) string { return "synthetic garbage" }, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				state, home := fixture(t)
				before := cjsiAccSnapshotState(t, state)
				write(t, filepath.Join(state, "scan-receipts.json"), tc.doc(home))
				expectDiff(t, state, before, tc.want)
			})
		}
	})
}

// cjsiAccHostProbeGuard proves the guard every fixture subprocess runs under
// is active: the registration tools cannot be executed (not even by a child
// shell) and the host app and real App Group cannot be read. Mutation M6
// (old doctor on a healthy root) shows the output detector firing.
func cjsiAccHostProbeGuard(t *testing.T) {
	for _, tool := range cjsiAccHostTools[:3] {
		ctx, cancel := context.WithTimeout(context.Background(), cjsiAccCommandGuard)
		cmd := cjsiAccGuarded(ctx, t, "/bin/sh", "-c", tool+" -h >/dev/null 2>&1; echo status=$?")
		cmd.Env = []string{"PATH=/usr/bin:/bin"}
		output, err := cmd.CombinedOutput()
		cancel()
		if err != nil || strings.TrimSpace(string(output)) != "status=126" {
			t.Fatalf("guard let %s execute: %v %q", tool, err, output)
		}
	}
	// The host bundle is refused as a whole, metadata included: stat of the
	// bundle itself must be EPERM (or ENOENT only when it is not installed).
	{
		ctx, cancel := context.WithTimeout(context.Background(), cjsiAccCommandGuard)
		cmd := cjsiAccGuarded(ctx, t, "/usr/bin/stat", "/Applications/AgentDeck.app")
		cmd.Env = []string{"PATH=/usr/bin:/bin"}
		output, err := cmd.CombinedOutput()
		cancel()
		switch {
		case err != nil && strings.Contains(string(output), "Operation not permitted"):
		case err != nil && strings.Contains(string(output), "No such file"):
			t.Log("host bundle is not installed; its refusal is covered by the profile only")
		default:
			t.Fatalf("guard let the host bundle be stat'ed: %v %q", err, output)
		}
	}
	// The private-directory rules are proven on a synthetic home built by the
	// same generator, so no real directory is read even if a rule is wrong:
	// content and anything below are refused, the entrance's own metadata
	// (needed by the state ownership guard) stays readable.
	fake, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	profile := cjsiAccGuardProfileFor(fake)
	for _, dir := range cjsiAccGuardPrivateDirs(fake) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "canary"), []byte("synthetic"), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, probe := range []struct {
			args    []string
			refused bool
		}{
			{[]string{"/usr/bin/stat", dir}, false},
			{[]string{"/usr/bin/stat", filepath.Join(dir, "canary")}, true},
			{[]string{"/bin/cat", filepath.Join(dir, "canary")}, true},
			{[]string{"/bin/ls", dir}, true},
			{[]string{"/usr/bin/touch", filepath.Join(dir, "written")}, true},
		} {
			ctx, cancel := context.WithTimeout(context.Background(), cjsiAccCommandGuard)
			cmd := cjsiAccGuardedWith(ctx, t, profile, probe.args[0], probe.args[1:]...)
			cmd.Env = []string{"PATH=/usr/bin:/bin"}
			output, err := cmd.CombinedOutput()
			cancel()
			refused := err != nil && strings.Contains(string(output), "Operation not permitted")
			if refused != probe.refused {
				t.Fatalf("guard on %q: refused=%v want %v: %v %q", probe.args, refused, probe.refused, err, output)
			}
		}
	}
	account, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	// The installation directory is closed completely: a synthetic receipt
	// can be neither read nor stat'ed, nor can the directory itself.
	install := cjsiAccGuardClosedDirs(fake)[1]
	if err := os.MkdirAll(install, 0o700); err != nil {
		t.Fatal(err)
	}
	receipt := filepath.Join(install, "installation.json")
	if err := os.WriteFile(receipt, []byte(`{"receipt_version":1,"executable":"/synthetic/agentdeck","channel":"synthetic","installed_at":"2026-01-02T00:00:00Z","installation_id":"synthetic"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"/usr/bin/stat", install}, {"/usr/bin/stat", receipt}, {"/bin/cat", receipt}, {"/usr/bin/touch", filepath.Join(install, "ownership.lock")}} {
		ctx, cancel := context.WithTimeout(context.Background(), cjsiAccCommandGuard)
		cmd := cjsiAccGuardedWith(ctx, t, profile, args[0], args[1:]...)
		cmd.Env = []string{"PATH=/usr/bin:/bin"}
		output, err := cmd.CombinedOutput()
		cancel()
		if err == nil || !strings.Contains(string(output), "Operation not permitted") {
			t.Fatalf("guard let %q through: %v %q", args, err, output)
		}
	}
	// Closed single files (a synthetic ~/.claude.json and shell start-up
	// files): neither content nor metadata, nor writes.
	for _, file := range cjsiAccGuardClosedFiles(fake) {
		write := `{"mcpServers":{"synthetic":{"command":"/synthetic/server"}}}`
		if err := os.WriteFile(file, []byte(write), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"/usr/bin/stat", file}, {"/bin/cat", file}, {"/usr/bin/touch", file}} {
			ctx, cancel := context.WithTimeout(context.Background(), cjsiAccCommandGuard)
			cmd := cjsiAccGuardedWith(ctx, t, profile, args[0], args[1:]...)
			cmd.Env = []string{"PATH=/usr/bin:/bin"}
			output, err := cmd.CombinedOutput()
			cancel()
			if err == nil || !strings.Contains(string(output), "Operation not permitted") {
				t.Fatalf("guard let %q through: %v %q", args, err, output)
			}
		}
	}
	fish := filepath.Join(cjsiAccGuardClosedDirs(fake)[2], "config.fish")
	if err := os.MkdirAll(filepath.Dir(fish), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fish, []byte("# synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"/usr/bin/stat", filepath.Dir(fish)}, {"/bin/cat", fish}} {
		ctx, cancel := context.WithTimeout(context.Background(), cjsiAccCommandGuard)
		cmd := cjsiAccGuardedWith(ctx, t, profile, args[0], args[1:]...)
		cmd.Env = []string{"PATH=/usr/bin:/bin"}
		output, err := cmd.CombinedOutput()
		cancel()
		if err == nil || !strings.Contains(string(output), "Operation not permitted") {
			t.Fatalf("guard let %q through: %v %q", args, err, output)
		}
	}
	// A neighbour of a closed file stays readable: the rules are exact.
	neighbour := filepath.Join(fake, ".claude.json.synthetic-neighbour")
	if err := os.WriteFile(neighbour, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	{
		ctx, cancel := context.WithTimeout(context.Background(), cjsiAccCommandGuard)
		cmd := cjsiAccGuardedWith(ctx, t, profile, "/bin/cat", neighbour)
		cmd.Env = []string{"PATH=/usr/bin:/bin"}
		output, err := cmd.CombinedOutput()
		cancel()
		if err != nil || string(output) != "ok" {
			t.Fatalf("guard over-reached to %s: %v %q", neighbour, err, output)
		}
	}
	real := cjsiAccGuardProfile(t)
	for _, path := range cjsiAccGuardClosedFiles(account.HomeDir) {
		if !strings.Contains(real, "(literal "+cjsiAccSandboxQuote(path)+")") {
			t.Fatalf("the subprocess profile does not close %s", path)
		}
	}
	for _, path := range append(cjsiAccGuardClosedDirs(account.HomeDir), cjsiAccGuardPrivateDirs(account.HomeDir)...) {
		if !strings.Contains(real, cjsiAccSandboxQuote(path)) {
			t.Fatalf("the subprocess profile does not cover %s", path)
		}
	}
	if !strings.Contains(real, cjsiAccSandboxQuote(cjsiAccGuardPrivateDirs(account.HomeDir)[1])) {
		t.Fatal("the subprocess profile does not cover the real ~/.agentdeck")
	}
}

// cjsiAccBuildScriptSelfTest runs build-binaries.sh --self-test: every /tmp
// spelling (/tmp, /private/tmp, //tmp, /var/../tmp, relative paths from /tmp,
// links to /tmp) is refused before anything is created, and a normal path is
// accepted without being created by the check.
func cjsiAccBuildScriptSelfTest(t *testing.T) {
	scratch, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if scratch == "/private/tmp" || strings.HasPrefix(scratch, "/private/tmp/") {
		t.Skipf("TMPDIR resolves into /tmp (%s); the self-test needs a scratch directory outside it", scratch)
	}
	cmd := exec.Command("/bin/bash", filepath.Join("testdata", "cjsi_acceptance", "build-binaries.sh"), "--self-test", filepath.Join(scratch, "self-test"))
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + scratch, "LANG=C"}
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "self-test failures=0") {
		t.Fatalf("build-binaries.sh --self-test: %v\n%s", err, output)
	}
}

// ---------------------------------------------------------------------------
// OldBinaryFence

func cjsiAccSeedFamily(t *testing.T, r *cjsiAccRoot) {
	t.Helper()
	for rel, content := range cjsiAccFamily() {
		r.write(t, rel, content)
	}
}

func cjsiAccMustSucceed(t *testing.T, res cjsiAccResult) {
	t.Helper()
	if res.exit != 0 || res.errorCode() != "" {
		t.Fatalf("expected success: %s", res)
	}
}

func cjsiAccSchemaVersion(t *testing.T, state string) int {
	t.Helper()
	values := cjsiAccCoreQuery(t, state, `SELECT version FROM schema_metadata WHERE singleton = 1`)
	if len(values) != 1 {
		t.Fatalf("schema_metadata rows: %v", values)
	}
	version, err := strconv.Atoi(values[0])
	if err != nil {
		t.Fatal(err)
	}
	return version
}

// cjsiAccOldBaseline proves that the same real old build works on its own
// schema-35 root: rebuild, list, search and show all succeed and see the
// synthetic family.
func cjsiAccOldBaseline(t *testing.T, r *cjsiAccRoot, old string) {
	t.Helper()
	cjsiAccSeedFamily(t, r)
	rebuild := r.run(t, old, "", "session", "rebuild")
	cjsiAccMustSucceed(t, rebuild)
	list := r.run(t, old, "", "session", "list", "--client", "codex")
	cjsiAccMustSucceed(t, list)
	var rows []struct {
		SessionID string `json:"session_id"`
		Model     string `json:"model"`
	}
	if err := json.Unmarshal(list.data(t), &rows); err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, row := range rows {
		seen[row.SessionID] = row.Model
	}
	for _, id := range []string{cjsiAccP, cjsiAccC, cjsiAccS, cjsiAccG} {
		if _, ok := seen[id]; !ok {
			t.Fatalf("old baseline list lacks %s: %s", id, list)
		}
	}
	search := r.run(t, old, "", "session", "search", cjsiAccParentToken)
	cjsiAccMustSucceed(t, search)
	if !strings.Contains(search.stdout, cjsiAccP) {
		t.Fatalf("old baseline search missed the parent document: %s", search)
	}
	show := r.run(t, old, "", "session", "show", cjsiAccP, "--client", "codex")
	cjsiAccMustSucceed(t, show)
	var detail struct {
		Documents []json.RawMessage `json:"documents"`
	}
	if err := json.Unmarshal(show.data(t), &detail); err != nil || len(detail.Documents) == 0 {
		t.Fatalf("old baseline show has no documents: %v %s", err, show)
	}
	if got := cjsiAccSchemaVersion(t, r.state); got != 35 {
		t.Fatalf("old baseline core schema = %d, want 35", got)
	}
	// The source defect is present in the real old build: the child head's
	// model is not attributed to C, while P keeps its own. This confirms the
	// fixture exercises the defect.
	t.Logf("old baseline models: P=%q C=%q S=%q G=%q", seen[cjsiAccP], seen[cjsiAccC], seen[cjsiAccS], seen[cjsiAccG])
	if seen[cjsiAccC] != "" || seen[cjsiAccP] != "parent-model" {
		t.Fatalf("old baseline no longer reproduces the empty child model: P=%q C=%q", seen[cjsiAccP], seen[cjsiAccC])
	}
}

// cjsiAccAssertFenced checks the installed layout: core schema-ahead, the
// fixed legacy name holding the private non-SQLite fence, and the private
// index at its new locator.
func cjsiAccAssertFenced(t *testing.T, state string) {
	t.Helper()
	if got := cjsiAccSchemaVersion(t, state); got != 37 {
		t.Fatalf("upgraded core schema = %d, want 37", got)
	}
	fence, _, err := cjsiAccReadRegular(filepath.Join(state, "sessions.sqlite3"), cjsiAccStateReadLimit)
	if err != nil {
		t.Fatal(err)
	}
	if len(fence) != 4097 || !strings.HasPrefix(string(fence), "AgentDeck private session index fence\n") || strings.HasPrefix(string(fence), "SQLite format 3") {
		t.Fatalf("legacy name is not the fence: %d bytes %q", len(fence), cjsiAccClip(string(fence[:min(len(fence), 48)])))
	}
	for _, b := range fence[len("AgentDeck private session index fence\n"):] {
		if b != 0 {
			t.Fatal("fence carries data beyond its fixed marker")
		}
	}
	if info, err := os.Lstat(filepath.Join(state, "sessions.sqlite3")); err != nil || info.Mode() != 0o600 {
		t.Fatalf("fence mode: %v %v, want regular 0600", info.Mode(), err)
	}
	index, _, err := cjsiAccReadRegular(filepath.Join(state, "session-index-v1.sqlite3"), cjsiAccStateReadLimit)
	if err != nil || !strings.HasPrefix(string(index), "SQLite format 3\x00") {
		t.Fatalf("private index missing or not SQLite: %v", err)
	}
	// The installed private index is closed and self-contained: no
	// rollback journal, WAL or wal-index may exist beside it.
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Lstat(filepath.Join(state, "session-index-v1.sqlite3"+suffix)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("private index has a %s sidecar: %v", suffix, err)
		}
	}
}

// cjsiAccLayoutPhase reads the durable install stage from a core copy:
// "journal phase / head index epoch set / both head seals set / head epoch
// equals journal epoch / base seal is the session seal / head ticket /
// head session ticket".
func cjsiAccLayoutPhase(t *testing.T, state string) string {
	t.Helper()
	got := cjsiAccCoreQuery(t, state, `SELECT printf('%d/%d/%d/%d/%d/%d/%d', j.phase, h.index_epoch > 0,
		h.base_seal IS NOT NULL AND h.session_seal IS NOT NULL, h.index_epoch = coalesce(j.index_epoch, -1),
		h.base_seal IS h.session_seal, h.ticket, h.session_ticket)
		FROM cjsi_layout_journal j CROSS JOIN cjsi_publication_head h WHERE j.singleton = 1 AND h.singleton = 1`)
	if len(got) != 1 {
		t.Fatalf("layout journal/head rows: %v", got)
	}
	return got[0]
}

// Durable install stages (cjsiAccLayoutPhase). Ready: phase 5, the head's
// index epoch is set and equals the journal's, base and session seals are
// set and equal, no ticket opened yet. Pre-ready (killed before the initial
// head commit): phase 4, no head epoch or seals. Never built: phase 5 with no
// private index, so no epoch or seals.
const (
	cjsiAccPhasePreReady   = "4/0/0/0/1/0/0"
	cjsiAccPhaseReady      = "5/1/1/1/1/0/0"
	cjsiAccPhaseNeverBuilt = "5/0/0/0/1/0/0"
)

func cjsiAccAssertLayoutPhase(t *testing.T, state, want string) {
	t.Helper()
	if got := cjsiAccLayoutPhase(t, state); got != want {
		t.Fatalf("layout phase/epoch/seal/head = %s, want %s", got, want)
	}
}

type cjsiAccEntry struct {
	name   string
	format string // default json
	args   []string
	stdin  string
	expect func(*testing.T, cjsiAccResult)
}

// cjsiAccNotADatabase is the old session reader/writer stopping on the fence:
// the SQLite driver rejects the non-database header (SQLITE_NOTADB) at the
// first pragma, before migrateSessionSchema could run (architecture §7.1).
func cjsiAccNotADatabase(t *testing.T, res cjsiAccResult) {
	t.Helper()
	if res.exit == 0 || res.errorCode() != "runtime_error" || !strings.Contains(res.errorMessage(), "file is not a database (26)") {
		t.Fatalf("want SQLITE_NOTADB on the fence: %s", res)
	}
}

// cjsiAccErrorCode expects a directed error envelope with this code.
func cjsiAccErrorCode(code, message string) func(*testing.T, cjsiAccResult) {
	return func(t *testing.T, res cjsiAccResult) {
		t.Helper()
		if res.exit == 0 || res.errorCode() != code || !strings.Contains(res.errorMessage(), message) {
			t.Fatalf("want %s (%q): %s", code, message, res)
		}
	}
}

// cjsiAccUnavailable expects an existing read-only surface that answers with
// its established unavailable/unhealthy form instead of data.
func cjsiAccUnavailable(check func(map[string]any) bool) func(*testing.T, cjsiAccResult) {
	return func(t *testing.T, res cjsiAccResult) {
		t.Helper()
		var data map[string]any
		if res.exit != 0 || res.errorCode() != "" || json.Unmarshal(res.data(t), &data) != nil || !check(data) {
			t.Fatalf("want an established unavailable answer: %s", res)
		}
	}
}

func cjsiAccField(value any, path ...string) any {
	for _, key := range path {
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = object[key]
	}
	return value
}

func cjsiAccSchemaAheadCheck(checks any) bool {
	list, _ := checks.([]any)
	for _, check := range list {
		if cjsiAccField(check, "name") == "database" && cjsiAccField(check, "code") == "schema_ahead" && cjsiAccField(check, "count") == float64(37) {
			return true
		}
	}
	return false
}

// cjsiAccFailedRound is the shared scan command's answer: a partial round
// whose usage domain stopped on schema_ahead and whose session domain failed,
// followed by a directed error envelope.
func cjsiAccFailedRound(t *testing.T, res cjsiAccResult) {
	t.Helper()
	var round struct {
		Data struct {
			Usage, Session struct {
				State     string `json:"state"`
				ErrorCode string `json:"error_code"`
			}
		} `json:"data"`
	}
	first, _, _ := strings.Cut(res.stdout, "\n")
	if res.exit == 0 || res.errorCode() == "" || json.Unmarshal([]byte(first), &round) != nil ||
		round.Data.Usage.State != "failed" || round.Data.Usage.ErrorCode != "schema_ahead" || round.Data.Session.State != "failed" {
		t.Fatalf("want a failed round with usage schema_ahead: %s", res)
	}
}

// cjsiAccOldEntries are the old-build entry points tasks.md names for the
// fence: session list/search/show (plus --activity and --tokens)/classify/
// classify list/scope get/exclude/rebuild/scan/purge-index, scan, watch, the
// local telemetry projection entry, doctor, desktop serving/helper, backup
// create/restore and state migrate. Each row pins the directed refusal or
// established unavailable answer of the real 4ac56db build.
func cjsiAccOldEntries(r *cjsiAccRoot, otelFile, archive string) []cjsiAccEntry {
	ahead := cjsiAccErrorCode("schema_ahead", "database version 37 exceeds supported version 35")
	return []cjsiAccEntry{
		{name: "session-list", args: []string{"session", "list"}, expect: cjsiAccNotADatabase},
		{name: "session-search", args: []string{"session", "search", cjsiAccParentToken}, expect: cjsiAccNotADatabase},
		{name: "session-show", args: []string{"session", "show", cjsiAccP, "--client", "codex"}, expect: cjsiAccNotADatabase},
		{name: "session-show-activity", args: []string{"session", "show", cjsiAccC, "--client", "codex", "--activity"}, expect: cjsiAccNotADatabase},
		{name: "session-classify", args: []string{"session", "classify", cjsiAccP, "--client", "codex"}, expect: cjsiAccErrorCode("runtime_error", "session scope classification is unavailable: sessions_unavailable")},
		{name: "session-classify-list", args: []string{"session", "classify", "list"}, expect: cjsiAccErrorCode("runtime_error", "Session origin state could not be read.")},
		{name: "session-scope-get", args: []string{"session", "scope", "get"}, expect: cjsiAccErrorCode("runtime_error", "The default session scope could not be read.")},
		{name: "session-show-tokens", args: []string{"session", "show", cjsiAccP, "--client", "codex", "--tokens"}, expect: cjsiAccNotADatabase},
		{name: "session-exclude", args: []string{"session", "exclude", "--kind", "client", "--value", "codex"}, expect: cjsiAccNotADatabase},
		{name: "session-rebuild", args: []string{"session", "rebuild"}, expect: cjsiAccNotADatabase},
		// The scan worker records its failed round (schema_ahead/scan_failed);
		// cjsiAccAssertReceipts checks the receipt journal.
		{name: "session-scan", args: []string{"session", "scan"}, expect: cjsiAccErrorCode("runtime_error", "session scan failed")},
		{name: "session-purge-index", args: []string{"session", "purge-index"}, expect: ahead},
		{name: "scan", args: []string{"scan"}, expect: cjsiAccFailedRound},
		{name: "watch", format: "ndjson", args: []string{"watch", "--domains", "usage,session"}, expect: ahead},
		{name: "telemetry-import", args: []string{"telemetry", "import", otelFile}, expect: ahead},
		{name: "doctor", args: []string{"doctor"}, expect: cjsiAccUnavailable(func(d map[string]any) bool {
			return d["healthy"] == false && cjsiAccSchemaAheadCheck(d["checks"])
		})},
		{name: "doctor-full", args: []string{"doctor", "--full"}, expect: cjsiAccUnavailable(func(d map[string]any) bool {
			return d["healthy"] == false && d["mode"] == "full" && cjsiAccSchemaAheadCheck(d["checks"])
		})},
		{name: "desktop-snapshot", args: []string{"desktop", "snapshot", "--wire-version", "1"}, expect: cjsiAccUnavailable(func(d map[string]any) bool {
			return cjsiAccField(d, "sessions", "available") == false && cjsiAccField(d, "usage", "available") == false &&
				cjsiAccField(d, "session_scope_view", "available") == false && cjsiAccSchemaAheadCheck(cjsiAccField(d, "health", "checks"))
		})},
		{name: "desktop-session-scope", args: []string{"desktop", "session-scope", "--scope", "all"}, expect: cjsiAccUnavailable(func(d map[string]any) bool {
			return d["available"] == false && d["reason"] == "sessions_unavailable"
		})},
		{name: "desktop-refresh-indexes", args: []string{"desktop", "refresh-indexes"}, expect: cjsiAccUnavailable(func(d map[string]any) bool {
			return cjsiAccField(d, "usage", "success") == false && cjsiAccField(d, "usage", "error_code") == "schema_ahead" &&
				cjsiAccField(d, "sessions", "success") == false
		})},
		{name: "usage-summary", args: []string{"usage", "summary"}, expect: ahead},
		{name: "backup-create", args: []string{"backup", "create", filepath.Join(r.dir, "after-upgrade.adb"), "--include-sessions"}, stdin: cjsiAccPassphrase + "\n", expect: ahead},
		{name: "backup-restore-nonempty", args: []string{"backup", "restore", archive}, stdin: cjsiAccPassphrase + "\n", expect: cjsiAccErrorCode("restore_target_not_empty", "restore_target_not_empty")},
		{name: "state-migrate", args: []string{"state", "migrate"}, expect: ahead},
	}
}

// cjsiAccCheckStep compares the state after one command twice: against the
// snapshot taken right before that command (so a receipt, lock or file that
// an earlier command legitimately produced cannot be modified or removed by a
// later one) and against the snapshot at the start of the round (so the round
// as a whole only produced allowed changes). previous always advances.
func cjsiAccCheckStep(t *testing.T, label, state string, start cjsiAccSnapshot, previous *cjsiAccSnapshot) {
	t.Helper()
	current := cjsiAccSnapshotState(t, state)
	adjacent := cjsiAccSideEffects(*previous, current)
	cumulative := cjsiAccSideEffects(start, current)
	*previous = current
	if len(adjacent) > 0 || len(cumulative) > 0 {
		t.Fatalf("%s: since the previous command %v; since the round started %v", label, adjacent, cumulative)
	}
}

// cjsiAccRunEntry runs one old entry, checks its answer, and requires that
// no synthetic session identity leaked into the output.
func cjsiAccRunEntry(t *testing.T, r *cjsiAccRoot, old string, entry cjsiAccEntry) {
	t.Helper()
	format := entry.format
	if format == "" {
		format = "json"
	}
	res := r.runFormat(t, old, format, entry.stdin, entry.args...)
	t.Logf("observed: exit=%d code=%s message=%q", res.exit, res.errorCode(), res.errorMessage())
	entry.expect(t, res)
	for _, id := range []string{cjsiAccP, cjsiAccC, cjsiAccS, cjsiAccG} {
		if strings.Contains(res.stdout+res.stderr, id) {
			t.Fatalf("old build emitted session identity from fenced state: %s", res)
		}
	}
}

func cjsiAccOldBinaryFence(t *testing.T, upgrader *cjsiAccUpgrader) {
	old := cjsiAccExternalBinary(t, cjsiAccOldBinaryEnv)
	r := newCJSIAccRoot(t, "fence")
	cjsiAccOldBaseline(t, r, old)

	// An old archive of the schema-35 root, for the Restore rows below.
	archive := filepath.Join(r.dir, "old-baseline.adb")
	cjsiAccMustSucceed(t, r.run(t, old, cjsiAccPassphrase+"\n", "backup", "create", archive, "--include-sessions"))
	otelFile := filepath.Join(r.dir, "otel.json")
	if err := os.WriteFile(otelFile, []byte(cjsiAccOTelExport(cjsiAccC, 7, 0)), 0o600); err != nil {
		t.Fatal(err)
	}

	r.upgrade(t, upgrader.binary(t))
	cjsiAccAssertLayoutPhase(t, r.state, cjsiAccPhaseReady)
	cjsiAccAssertFenced(t, r.state)
	baseline := cjsiAccSnapshotState(t, r.state)

	t.Run("EntryMatrix", func(t *testing.T) {
		previous := baseline
		for _, entry := range cjsiAccOldEntries(r, otelFile, archive) {
			t.Run(entry.name, func(t *testing.T) {
				cjsiAccRunEntry(t, r, old, entry)
				cjsiAccCheckStep(t, "old "+entry.name+" wrote fenced state", r.state, baseline, &previous)
			})
		}
		// Purge refused before any remover: fence and private index kept
		// inode and content (the snapshot above). Backup create refused
		// before creating an archive, and no scan round completed.
		if _, err := os.Lstat(filepath.Join(r.dir, "after-upgrade.adb")); !os.IsNotExist(err) {
			t.Fatalf("old backup create left an archive for fenced state: %v", err)
		}
		cjsiAccAssertReceipts(t, r.state)
		cjsiAccAssertFenced(t, r.state)
	})

	t.Run("OldArchiveRestoresToEmptyRoot", func(t *testing.T) {
		// Control: the old archive is valid, so the fenced root's refusal
		// above is the non-empty-target check, not a broken archive.
		other := newCJSIAccRoot(t, "restore-target")
		cjsiAccMustSucceed(t, other.run(t, old, cjsiAccPassphrase+"\n", "backup", "restore", archive))
		if got := cjsiAccSchemaVersion(t, other.state); got != 35 {
			t.Fatalf("old restore produced schema %d", got)
		}
	})

	t.Run("InterruptedMigrationAndResume", func(t *testing.T) {
		ir := newCJSIAccRoot(t, "interrupted")
		cjsiAccSeedFamily(t, ir)
		cjsiAccMustSucceed(t, ir.run(t, old, "", "session", "rebuild"))
		ir.upgradeInterrupted(t, upgrader.binary(t))
		// Killed at the initial-head commit: the core step already made the
		// schema ahead, so every old entry refuses and writes nothing.
		if got := cjsiAccSchemaVersion(t, ir.state); got != 37 {
			t.Fatalf("interrupted core schema = %d, want 37 (schema step commits before the head)", got)
		}
		// Pre-ready: journal phase 4 with the private index prepared, but the
		// head has no index epoch and no seals yet.
		cjsiAccAssertLayoutPhase(t, ir.state, cjsiAccPhasePreReady)
		paused := cjsiAccSnapshotState(t, ir.state)
		t.Logf("interrupted layout files: %v", paused.names())
		// Every entry of the matrix runs against the interrupted layout too.
		previous := paused
		for _, entry := range cjsiAccOldEntries(ir, otelFile, archive) {
			cjsiAccRunEntry(t, ir, old, entry)
			cjsiAccCheckStep(t, "old "+entry.name+" wrote an interrupted layout", ir.state, paused, &previous)
		}
		// Resume with the same committed migration; the data files match an
		// uninterrupted install and the old build still stops on the fence.
		ir.upgrade(t, upgrader.binary(t))
		cjsiAccAssertLayoutPhase(t, ir.state, cjsiAccPhaseReady)
		cjsiAccAssertFenced(t, ir.state)
		want := cjsiAccDataNames(baseline)
		if got := cjsiAccDataNames(cjsiAccSnapshotState(t, ir.state)); !slices.Equal(got, want) {
			t.Fatalf("resumed layout %v differs from uninterrupted %v", got, want)
		}
		cjsiAccNotADatabase(t, ir.run(t, old, "", "session", "list"))
		cjsiAccAssertReceipts(t, ir.state)
	})

	t.Run("CandidateBaseline", func(t *testing.T) {
		candidate := cjsiAccExternalBinary(t, cjsiAccCandidateBinaryEnv)
		// The candidate reads a root the old build created (schema 35).
		cr := newCJSIAccRoot(t, "candidate-baseline")
		cjsiAccSeedFamily(t, cr)
		cjsiAccMustSucceed(t, cr.run(t, old, "", "session", "rebuild"))
		cjsiAccMustSucceed(t, cr.run(t, candidate, "", "session", "list"))
		search := cr.run(t, candidate, "", "session", "search", cjsiAccParentToken)
		cjsiAccMustSucceed(t, search)
		if !strings.Contains(search.stdout, cjsiAccP) {
			t.Fatalf("candidate lost the old baseline document: %s", search)
		}
	})

	t.Run("CandidateOnUpgradedRoot", func(t *testing.T) {
		cjsiAccPending(t, "Task 8 2b-1/2b-2a", "the candidate CLI must read the fenced layout through the production selector and CLI read gates; 0d91b76 installs the layout (CurrentSchemaVersion 37, store.MigrateState) but the CLI gates are not committed")
	})
	t.Run("OldRestoreRejectsAheadArchive", func(t *testing.T) {
		cjsiAccPending(t, "Task 8 2b-2a", "needs a candidate `backup create` of a fenced root producing an archive whose core schema is ahead of the old build; the managed backup itself is committed (Task 9b-a, 0d91b76) but its CLI wiring is Task 8's shared entry")
	})
}

// cjsiAccDataNames lists the paths compared as data (exact or core rule).
func cjsiAccDataNames(s cjsiAccSnapshot) []string {
	var names []string
	for _, name := range s.names() {
		if rule := cjsiAccRuleFor(name); rule == cjsiAccExact || rule == cjsiAccCoreRule {
			names = append(names, name)
		}
	}
	return names
}

// ---------------------------------------------------------------------------
// IdentityAndAccounting (A1–A4 through committed Task 1 proofs)

type cjsiAccProof struct {
	source  ingest.CodexSourceProof
	records []ingest.CodexResolvedRecord
}

func cjsiAccCapture(t *testing.T, path string) cjsiAccProof {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	reduction, err := ingest.CaptureCodexReduction(t.Context(), file, path, info.Size(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	proof := cjsiAccProof{source: reduction.Source()}
	for record := range reduction.Records() {
		proof.records = append(proof.records, record)
	}
	return proof
}

// localTotals sums the primary input+output of locally proved metered
// records per execution thread, and counts the requests.
func (p cjsiAccProof) localTotals() (map[string]int64, map[string]int) {
	totals, requests := map[string]int64{}, map[string]int{}
	for _, record := range p.records {
		if !record.Local() {
			continue
		}
		var counts ingest.CodexCounts
		switch record.Record.Kind {
		case ingest.CodexRequestUsage:
			counts = record.Record.Usage
		case ingest.CodexLegacyUsage:
			counts = record.Record.LegacyLast
		default:
			continue
		}
		if total, ok := counts.Total(); ok {
			totals[record.ThreadID] += total
			requests[record.ThreadID]++
		}
	}
	return totals, requests
}

func cjsiAccIdentityAndAccounting(t *testing.T) {
	r := newCJSIAccRoot(t, "identity")
	proofs := map[string]cjsiAccProof{}
	for rel, content := range cjsiAccFamily() {
		path := r.write(t, rel, content)
		proof := cjsiAccCapture(t, path)
		proofs[proof.source.ThreadID] = proof
	}

	t.Run("A1SourceAndRelation", func(t *testing.T) {
		want := map[string]struct{ parent, root string }{
			cjsiAccP: {"", ""},
			cjsiAccC: {cjsiAccP, cjsiAccP},
			cjsiAccS: {cjsiAccP, cjsiAccP}, // thread_spawn parent form, archived
			cjsiAccG: {cjsiAccC, cjsiAccP}, // direct parent is C, root stays P
		}
		var sources []ingest.CodexSourceProof
		for id, w := range want {
			proof, ok := proofs[id]
			if !ok || proof.source.State != ingest.CodexSourceVerified || proof.source.ThreadID != id {
				t.Fatalf("source %s not verified: %+v", id, proof.source)
			}
			if proof.source.ParentCandidate != w.parent {
				t.Fatalf("%s direct parent = %q, want %q", id, proof.source.ParentCandidate, w.parent)
			}
			if id != cjsiAccP && proof.source.RootSessionID != w.root {
				t.Fatalf("%s root = %q, want %q", id, proof.source.RootSessionID, w.root)
			}
			sources = append(sources, proof.source)
		}
		edges := ingest.ResolveCodexParents(sources)
		wantEdges := map[string]string{cjsiAccC: cjsiAccP, cjsiAccS: cjsiAccP, cjsiAccG: cjsiAccC}
		if len(edges) != len(wantEdges) {
			t.Fatalf("parent edges %v, want %v", edges, wantEdges)
		}
		for child, parent := range wantEdges {
			if edges[child] != parent {
				t.Fatalf("parent edges %v, want %v", edges, wantEdges)
			}
		}
	})

	t.Run("A1ConflictHasNoOwner", func(t *testing.T) {
		rel, content := cjsiAccConflict()
		proof := cjsiAccCapture(t, r.write(t, rel, content))
		if proof.source.State != ingest.CodexSourceConflict {
			t.Fatalf("head %s under canonical name %s is not a conflict: %+v", cjsiAccY, cjsiAccX, proof.source)
		}
		if slices.ContainsFunc(proof.records, ingest.CodexResolvedRecord.Local) {
			t.Fatal("a conflicting source produced locally owned records")
		}
	})

	t.Run("A2InheritedHistoryAndRightAnchor", func(t *testing.T) {
		child := proofs[cjsiAccC]
		var models []string
		foreign := 0
		for _, record := range child.records {
			switch kind := record.Record.Kind; {
			case kind == ingest.CodexLegacyUsage && record.Local():
				t.Fatalf("inherited parent token_count is owned by C: exec=%d thread=%q", record.Execution, record.ThreadID)
			case kind == ingest.CodexRequestUsage && record.ThreadID == cjsiAccP:
				// The copied native P request is explicit foreign evidence,
				// never C's and never local to this source.
				if record.Execution != ingest.CodexForeign || record.Local() {
					t.Fatalf("inherited native P request: exec=%d local=%v", record.Execution, record.Local())
				}
				foreign++
			case kind == ingest.CodexRequestUsage && record.Local():
				models = append(models, record.Model)
				if record.ThreadID != cjsiAccC {
					t.Fatalf("child request executed by %s", record.ThreadID)
				}
			}
		}
		if foreign != 1 {
			t.Fatalf("inherited native P requests seen = %d, want 1", foreign)
		}
		// No second C head precedes the child turn; the request is still
		// recognised as C's and carries the child turn's model.
		if !slices.Equal(models, []string{"child-model"}) {
			t.Fatalf("child request models = %v", models)
		}

		// Right anchor: C's final answer is shown only once the turn's end
		// anchors it. Cut before task_complete it is not local; with the
		// anchor it is CodexRightAnchored for C.
		full := cjsiAccFamily()[cjsiAccActive("09-01-00", cjsiAccC)]
		cut := full[:strings.LastIndex(strings.TrimSuffix(full, "\n"), "\n")+1]
		if !strings.Contains(full[len(cut):], "task_complete") {
			t.Fatal("fixture no longer ends with the child's task_complete")
		}
		final := func(source string) ingest.CodexResolvedRecord {
			t.Helper()
			proof := cjsiAccCapture(t, r.write(t, filepath.Join("anchor", cjsiAccC+".jsonl"), source))
			var last *ingest.CodexResolvedRecord
			for i := range proof.records {
				if proof.records[i].Record.VisibleKind == ingest.CodexAssistantFinal {
					last = &proof.records[i]
				}
			}
			if last == nil {
				t.Fatal("no assistant final record")
			}
			return *last
		}
		if got := final(cut); got.Local() {
			t.Fatalf("final answer without its right anchor is local: exec=%d", got.Execution)
		}
		if got := final(full); got.Execution != ingest.CodexRightAnchored || got.ThreadID != cjsiAccC {
			t.Fatalf("anchored final answer: exec=%d thread=%q, want right-anchored C", got.Execution, got.ThreadID)
		}
	})

	t.Run("A3LocalTotals", func(t *testing.T) {
		all, requests := map[string]int64{}, 0
		for _, id := range []string{cjsiAccP, cjsiAccC} {
			totals, counts := proofs[id].localTotals()
			for thread, total := range totals {
				if thread != id {
					t.Fatalf("%s file owns %d tokens for %s", id, total, thread)
				}
				all[thread] += total
				requests += counts[thread]
			}
		}
		if all[cjsiAccP] != 120 || all[cjsiAccC] != 35 || all[cjsiAccP]+all[cjsiAccC] != 155 || requests != 2 {
			t.Fatalf("P/C/All = %d/%d/%d over %d requests, want 120/35/155 over 2", all[cjsiAccP], all[cjsiAccC], all[cjsiAccP]+all[cjsiAccC], requests)
		}
		sibling, _ := proofs[cjsiAccS].localTotals()
		grandchild, _ := proofs[cjsiAccG].localTotals()
		if sibling[cjsiAccS] != 13 || grandchild[cjsiAccG] != 11 {
			t.Fatalf("S/G independent totals = %d/%d, want 13/11", sibling[cjsiAccS], grandchild[cjsiAccG])
		}
	})

	t.Run("A1CLIReadSurfaces", func(t *testing.T) {
		cjsiAccPending(t, "Task 8 2b-1/2b-2a", "real index/search/show and usage/model/marker for P/C/S/G (active, archived, canonical, reverted, moved, conflict) need the production selector and CLI read gates, which are not committed at base 0d91b76")
	})
	t.Run("A3PricedReconciliation", func(t *testing.T) {
		cjsiAccPending(t, "Task 8 2b-1/2b-2a", "amounts 140/90/230 with routes, periods, models and cache/reasoning meters need a published generation read by `usage` CLI with a fixed catalog; fixture: cjsiAccFamily plus an archived replica of C")
	})
	t.Run("A4LegacyMirror", func(t *testing.T) {
		cjsiAccPending(t, "Task 8 2b-1/2b-2a", "old 20/R=10 possible/contained/independent = 10/1, 20/1, 30/2 and same-ID contradiction need published usage through the CLI; generator: cjsiAccLegacyMirror")
	})
	t.Run("A4bIntervalDimensions", func(t *testing.T) {
		cjsiAccPending(t, "Task 8 2b-1/2b-2a", "session summary 20/10 calls, Codex/All 20, unknown model rows and 50%% share need CLI and desktop projections over a published generation")
	})
}

// cjsiAccLegacyMirror is the A4 generator for the pending usage rows: a C
// turn with an old token_count of 20 and, depending on relation, a native
// request R=10 that is possibly contained, contained, or independent.
func cjsiAccLegacyMirror(relation string) string {
	head := cjsiAccLine(`{"timestamp":"%sT10:00:00Z","type":"session_meta","payload":{"id":"%s","session_id":"%s","parent_thread_id":"%s"}}`, cjsiAccDay, cjsiAccC, cjsiAccP, cjsiAccP)
	legacy := cjsiAccLine(`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":15,"output_tokens":5},"total_token_usage":{"input_tokens":15,"output_tokens":5}}}}`)
	request := func(turn string) string {
		return cjsiAccLine(`{"type":"token_usage_record","payload":{"thread_id":"%s","turn_id":"%s","response_id":"r-%s","usage":{"input_tokens":8,"output_tokens":2}}}`, cjsiAccC, turn, turn)
	}
	turn := cjsiAccLine(`{"type":"turn_context","payload":{"turn_id":"mirror","model":"child-model"}}`)
	switch relation {
	case "contained":
		return head + turn + request("mirror") + legacy
	case "independent":
		return head + turn + legacy + cjsiAccLine(`{"type":"turn_context","payload":{"turn_id":"other","model":"child-model"}}`) + request("other")
	default:
		return head + turn + legacy + request("")
	}
}

// ---------------------------------------------------------------------------
// OTelOwnership (A5)

// cjsiAccRetractionTriggers selects every trigger that could retract OTel
// evidence: the legacy name family or any trigger body touching the OTel
// observation/contribution tables. Baseline, upgraded and cold checks share it.
const cjsiAccRetractionTriggers = `SELECT name FROM sqlite_master WHERE type = 'trigger' AND (name LIKE 'telemetry%' OR sql LIKE '%telemetry_observations%' OR sql LIKE '%telemetry_contributions%') ORDER BY name`

func cjsiAccOTelOwnership(t *testing.T, upgrader *cjsiAccUpgrader) {
	t.Run("LegacyTriggerBaselineAndRemoval", func(t *testing.T) {
		old := cjsiAccExternalBinary(t, cjsiAccOldBinaryEnv)
		r := newCJSIAccRoot(t, "otel-trigger")
		cjsiAccSeedFamily(t, r)
		cjsiAccMustSucceed(t, r.run(t, old, "", "session", "rebuild"))
		before := cjsiAccCoreQuery(t, r.state, cjsiAccRetractionTriggers)
		if !slices.Equal(before, []string{"telemetry_jsonl_retracts_otel"}) {
			t.Fatalf("real schema-35 baseline telemetry triggers = %v", before)
		}
		body := cjsiAccCoreQuery(t, r.state, `SELECT sql FROM sqlite_master WHERE name = 'telemetry_jsonl_retracts_otel'`)
		if len(body) != 1 || !strings.Contains(body[0], "AFTER INSERT ON usage_events") || !strings.Contains(body[0], "DELETE FROM telemetry_contributions") {
			t.Fatalf("baseline trigger is not the legacy JSONL retraction: %v", body)
		}
		// Behaviour, not only DDL: a bare JSONL usage insert on a copy of the
		// real schema-35 core retracts the OTel contribution.
		if left, state := cjsiAccRetractionProbe(t, r.state); left != 0 || state != "jsonl_owned" {
			t.Fatalf("schema-35 baseline kept OTel evidence: contributions=%d state=%q", left, state)
		}
		r.upgrade(t, upgrader.binary(t))
		if after := cjsiAccCoreQuery(t, r.state, cjsiAccRetractionTriggers); len(after) != 0 {
			t.Fatalf("upgraded core keeps legacy retraction triggers: %v", after)
		}
		if left, state := cjsiAccRetractionProbe(t, r.state); left != 1 || state != "otel_owned" {
			t.Fatalf("upgraded core retracted OTel evidence on a bare insert: contributions=%d state=%q", left, state)
		}
	})
	t.Run("ColdBuildHasNoRetractionTrigger", func(t *testing.T) {
		// A never-built root migrated by the managed path has no legacy
		// retraction trigger and does not retract on a bare insert either.
		r := newCJSIAccRoot(t, "otel-cold")
		r.upgrade(t, upgrader.binary(t))
		cjsiAccAssertLayoutPhase(t, r.state, cjsiAccPhaseNeverBuilt)
		if got := cjsiAccCoreQuery(t, r.state, cjsiAccRetractionTriggers); len(got) != 0 {
			t.Fatalf("cold managed core has retraction triggers: %v", got)
		}
		if left, state := cjsiAccRetractionProbe(t, r.state); left != 1 || state != "otel_owned" {
			t.Fatalf("cold core retracted OTel evidence on a bare insert: contributions=%d state=%q", left, state)
		}
		if _, err := os.Lstat(filepath.Join(r.state, "session-index-v1.sqlite3")); !os.IsNotExist(err) {
			t.Fatalf("a never-built root gained a private session index: %v", err)
		}
	})
	t.Run("WholeThreadTakeover", func(t *testing.T) {
		cjsiAccPending(t, "Task 8 2b-1", "7/11/13 -> 7/17/13 and C=5/11/unknown with original keys, times, prices and markers need receipts opened by the final CAS and read through the CLI; both arrival orders, restart, S settled/blocked and ambiguous history follow")
	})
}

// cjsiAccRetractionProbe works on a private copy of the core: it seeds one
// OTel-owned observation with one contribution for thread X, then performs a
// bare JSONL-style INSERT INTO usage_events for X, and reports how many
// contributions remain and the observation state. Only NOT NULL columns
// without defaults are filled; the probe never touches the fixture root.
func cjsiAccRetractionProbe(t *testing.T, state string) (int, string) {
	t.Helper()
	db := cjsiAccCoreCopy(t, state)
	defer db.Close()
	const thread = "00000000-0000-4000-8000-00000000a0f7"
	const key = "synthetic-observation"
	cjsiAccInsertRequired(t, db, "telemetry_sessions", map[string]any{"client": "codex", "session_id": thread})
	cjsiAccInsertRequired(t, db, "telemetry_observations", map[string]any{"observation_key": key, "thread_id": thread, "state": "otel_owned", "source": "import", "metered": 1, "reason": "otel_evidence"})
	cjsiAccInsertRequired(t, db, "telemetry_contributions", map[string]any{"observation_key": key, "event_key": "otel:synthetic-event"})
	cjsiAccInsertRequired(t, db, "usage_events", map[string]any{"client": "codex", "session_id": thread, "source_path": "/synthetic/rollout.jsonl", "event_key": "synthetic-jsonl-event"})
	var left int
	var observation string
	if err := db.QueryRow(`SELECT count(*) FROM telemetry_contributions WHERE observation_key = ?`, key).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT state FROM telemetry_observations WHERE observation_key = ?`, key).Scan(&observation); err != nil {
		t.Fatal(err)
	}
	return left, observation
}

// cjsiAccInsertRequired inserts one row with the given values plus a
// synthetic value for every other NOT NULL column that has no default.
func cjsiAccInsertRequired(t *testing.T, db *sql.DB, table string, values map[string]any) {
	t.Helper()
	rows, err := db.Query(`SELECT name, upper(type), "notnull", dflt_value IS NOT NULL, pk FROM pragma_table_info(?)`, table)
	if err != nil {
		t.Fatal(err)
	}
	var columns []string
	var args []any
	for rows.Next() {
		var name, kind string
		var notNull, hasDefault bool
		var pk int
		if err := rows.Scan(&name, &kind, &notNull, &hasDefault, &pk); err != nil {
			t.Fatal(err)
		}
		value, given := values[name]
		switch {
		case given:
		case !notNull || hasDefault || (pk == 1 && kind == "INTEGER"):
			continue
		case strings.Contains(kind, "INT"):
			value = 1
		case strings.Contains(kind, "REAL") || strings.Contains(kind, "NUM"):
			value = 1.0
		default:
			value = "2026-01-02T09:00:00Z"
		}
		columns = append(columns, `"`+name+`"`)
		args = append(args, value)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if len(columns) == 0 {
		t.Fatalf("table %s not found", table)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",")
	if _, err := db.Exec(`INSERT INTO `+table+`(`+strings.Join(columns, ",")+`) VALUES(`+placeholders+`)`, args...); err != nil {
		t.Fatalf("seed %s: %v", table, err)
	}
}

// ---------------------------------------------------------------------------
// Pending composite subtests (skeletons with their fixture generators)

func cjsiAccDetailOrder(t *testing.T) {
	cjsiAccPending(t, "Task 8 2b-1/2b-2a", "Round 2 F1: ABC IDs 1/2/3, insert X (ID 4) at source position 2; cold, incremental, compact and backup/Restore all AXBC total=4, limit=2 pages AX/BC/empty with has_more, append D -> AXBCD, over CLI show/--all/--activity, viewer and ShowWithActivity (managed backup/Restore committed in Task 9b-a; publication and CLI wiring pending); generator: cjsiAccDetailOrderSource")
}

// cjsiAccDetailOrderSource is the F1 single-thread source. Each visible
// document is a user prompt whose text names its letter.
func cjsiAccDetailOrderSource(letters ...string) string {
	b := strings.Builder{}
	b.WriteString(cjsiAccLine(`{"timestamp":"%sT11:00:00Z","type":"session_meta","payload":{"id":"%s"}}`, cjsiAccDay, cjsiAccP))
	for i, letter := range letters {
		b.WriteString(cjsiAccLine(`{"timestamp":"%sT11:00:%02dZ","type":"turn_context","payload":{"turn_id":"t%d","model":"parent-model"}}`, cjsiAccDay, i+1, i))
		b.WriteString(cjsiAccLine(`{"timestamp":"%sT11:00:%02dZ","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"order-%s synthetic"}]}}`, cjsiAccDay, i+1, letter))
	}
	return b.String()
}

func cjsiAccSlotDurability(t *testing.T) {
	cjsiAccPending(t, "Task 8 2b-1", "Round 2 F2: independently controllable file-fsync and directory-fsync barriers in the production publication path, reachable from a fixture subprocess; a second connection commits BEGIN IMMEDIATE meanwhile; both failures leave no durable seal or side switch; plus late result, revision-changed CAS rejection and three crash points")
}

func cjsiAccReplayAndHistoricalAbsence(t *testing.T) {
	cjsiAccPending(t, "Task 8 2b-1/2b-2a", "A6a–A6d: true P=40, located+replayable 50/60, unreplayable 60/70, budget/cancel/rewrite/concurrent CAS, registered plaintext missing/unreadable/compressed-only, never-registered .jsonl.zst with open-call probes; needs the production backfill round and CLI")
}

func cjsiAccPortableRecoveryAndPrivacy(t *testing.T) {
	cjsiAccPending(t, "Task 8 2b-1/2b-2a", "A6c backup/Restore of one generation without candidates, new epochs and same-ticket detail (managed backup committed in Task 9b-a; generation publication and CLI wiring pending); A7a bindings/exclusion; A7b forbidden sentinels across both databases, FTS shadow, candidates, activity, classification, cursors, derived/Widget contracts, backups and logs")
}
