package perftrace

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func privateDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "perf")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readTrace(t *testing.T, path string) []map[string]any {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var out []map[string]any
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var line map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			t.Fatalf("trace line %q: %v", scanner.Text(), err)
		}
		out = append(out, line)
	}
	return out
}

func TestDisabledRecordsNothing(t *testing.T) {
	dir := privateDir(t)
	t.Setenv(EnvOut, "")
	if Start(RoleCLI) || Enabled() {
		t.Fatal("recording must stay off without AGENTDECK_PERF_OUT")
	}
	Mark(ScanBegin)
	Span(IngestRead)()
	Tx("core", "accept_inventory")(nil)
	if err := Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("disabled trace wrote %d entries, err %v", len(entries), err)
	}
}

func TestRejectsNonPrivateOutputDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvOut, dir)
	if Start(RoleCLI) {
		Close()
		t.Fatal("a 0755 directory must not enable recording")
	}
	t.Setenv(EnvOut, "relative/perf")
	if Start(RoleCLI) {
		Close()
		t.Fatal("a relative directory must not enable recording")
	}
}

func TestRecordsMarkersWithoutPrivateText(t *testing.T) {
	dir := privateDir(t)
	t.Setenv(EnvOut, dir)
	if !Start(RoleHelper) {
		t.Fatal("expected recording to start")
	}
	t.Cleanup(func() { _ = Close() })
	Mark(ScanBegin, Count{"sources", 3})
	Span(IngestRead)(Count{"bytes", 4096})
	Tx("core", "accept_inventory")(nil)
	Tx("session", "rebuild")(errors.New("/Users/someone/.codex/sessions/rollout-secret.jsonl"))
	// Free-form names must be rejected rather than written.
	Mark(Stage("/Users/someone/private.jsonl"))
	Tx("core", "Session 019a-secret")(nil)
	Mark(ScanEnd, Count{"session-id-019a", 1})
	if err := Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "helper-000001.jsonl")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("trace mode = %v, want 0600", info.Mode().Perm())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"/Users", "someone", "secret", "019a", "Session"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("trace leaked %q:\n%s", leak, raw)
		}
	}

	lines := readTrace(t, path)
	if lines[0]["role"] != "helper" || lines[0]["seq"] != float64(1) || lines[0]["clock"] != ClockName {
		t.Fatalf("header = %v", lines[0])
	}
	var stages []string
	for _, line := range lines[1:] {
		stages = append(stages, line["stage"].(string))
	}
	want := []string{"scan.begin", "ingest.read", "tx.core.accept_inventory", "tx.session.rebuild", "resource.lifetime", "trace.invalid"}
	if strings.Join(stages, ",") != strings.Join(want, ",") {
		t.Fatalf("stages = %v, want %v", stages, want)
	}
	if lines[4]["err"] != "rollback" {
		t.Fatalf("rollback code = %v", lines[4]["err"])
	}
	if got := lines[len(lines)-1]["n"].(map[string]any)["rejected"]; got != float64(3) {
		t.Fatalf("rejected = %v, want 3", got)
	}
	resource := lines[5]["n"].(map[string]any)
	if runtime.GOOS == "darwin" && (resource["maxrss_bytes"].(float64) < 1<<20 || resource["user_ns"].(float64) <= 0) {
		t.Fatalf("resource.lifetime = %v", resource)
	}
}

func TestSequenceSeparatesProcessesOfOneRole(t *testing.T) {
	dir := privateDir(t)
	t.Setenv(EnvOut, dir)
	for i := 0; i < 2; i++ {
		if !Start(RoleCLI) {
			t.Fatal("expected recording to start")
		}
		if err := Close(); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"cli-000001.jsonl", "cli-000002.jsonl"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
}

// The helper child prints its own Now(); the parent brackets it with two
// readings, so a shared system clock is the only way the value can fall inside.
func TestNowIsSharedAcrossProcesses(t *testing.T) {
	if os.Getenv("PERFTRACE_CLOCK_CHILD") == "1" {
		os.Stdout.WriteString(strconv.FormatInt(Now(), 10))
		os.Exit(0)
	}
	if runtime.GOOS != "darwin" {
		t.Skip("cross-process uptime is defined on darwin only")
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestNowIsSharedAcrossProcesses$")
	cmd.Env = append(os.Environ(), "PERFTRACE_CLOCK_CHILD=1")
	before := Now()
	out, err := cmd.Output()
	after := Now()
	if err != nil {
		t.Fatal(err)
	}
	child, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if child < before || child > after {
		t.Fatalf("child uptime %d outside parent bracket [%d, %d]", child, before, after)
	}
}

// Producers keep marking while Close runs; no record may be torn or lost
// before Close, and none may be written after it.
func TestConcurrentMarkersAndClose(t *testing.T) {
	dir := privateDir(t)
	t.Setenv(EnvOut, dir)
	if !Start(RoleCollector) {
		t.Fatal("expected recording to start")
	}
	var workers sync.WaitGroup
	for range 16 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 500 {
				done := Span(UsageCommit)
				Mark(IngestRead, Count{"bytes", 1})
				done()
			}
		}()
	}
	workers.Wait()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for range 1000 {
			Mark(ScanEnd)
		}
	}()
	if err := Close(); err != nil {
		t.Fatal(err)
	}
	<-finished
	lines := readTrace(t, filepath.Join(dir, "collector-000001.jsonl"))
	counts := map[string]int{}
	for _, line := range lines[1:] {
		counts[line["stage"].(string)]++
	}
	if counts["usage.commit"] != 8000 || counts["ingest.read"] != 8000 || counts["resource.lifetime"] != 1 {
		t.Fatalf("counts = %v", counts)
	}
	if counts["trace.invalid"] != 0 {
		t.Fatal("valid markers were rejected")
	}
}

// The producer holds the trace lock from Start until Close has written and
// closed the file, so a reader never sees a released trace that is missing
// its tail, even after the 64 KiB buffer has already flushed a prefix.
func TestTraceLockIsHeldUntilCloseFinishes(t *testing.T) {
	dir := privateDir(t)
	t.Setenv(EnvOut, dir)
	if !Start(RoleCollector) {
		t.Fatal("tracing did not start")
	}
	path := filepath.Join(dir, "collector-000001.jsonl")
	released := func(stage string) bool {
		t.Helper()
		ok, err := Released(path)
		if err != nil {
			t.Fatalf("%s: %v", stage, err)
		}
		return ok
	}
	if released("after start") {
		t.Fatal("a fresh trace is released before any record")
	}
	for i := 0; i < 2000; i++ {
		Mark(ScanEnd, Count{"read_bytes", int64(i)})
	}
	Mark(ScanBegin, Count{"bad key", 1}) // rejected: Close appends trace.invalid after the lifetime record
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Fatalf("the buffer did not flush a prefix yet: %v %v", info, err)
	}
	if released("after partial flush") {
		t.Fatal("a partly flushed trace is released")
	}
	if err := Close(); err != nil {
		t.Fatal(err)
	}
	if !released("after close") {
		t.Fatal("a closed trace is still held")
	}
	lines := readTrace(t, path)
	if lines[len(lines)-2]["stage"] != "resource.lifetime" || lines[len(lines)-1]["stage"] != "trace.invalid" {
		t.Fatalf("tail = %v %v", lines[len(lines)-2], lines[len(lines)-1])
	}
}
