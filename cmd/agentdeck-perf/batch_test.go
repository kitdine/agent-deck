package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// buildAgentdeck compiles the product CLI the batch runs as a separate process.
func buildAgentdeck(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "agentdeck")
	command := exec.Command("go", "build", "-mod=vendor", "-tags", "agentdeck_acceptance", "-o", binary, "github.com/kitdine/agent-deck/cmd/agentdeck")
	command.Env = append(os.Environ(), "GOFLAGS=")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build agentdeck: %v\n%s", err, output)
	}
	physical, err := filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatal(err)
	}
	runtimeCLIEnvironments.Store(physical, []string{"AGENTDECK_ACCEPTANCE_ACCOUNT_HOME=" + isolatedPerfAccount})
	t.Cleanup(func() { runtimeCLIEnvironments.Delete(physical) })
	return binary
}

// syntheticCorpus freezes the small synthetic fixture as if it were a real
// HOME, marking it representative for the test.
func syntheticCorpus(t *testing.T) string {
	t.Helper()
	opts, _ := smallFixture(t, 5)
	store := filepath.Join(t.TempDir(), "store")
	manifest, err := freezeCorpus(opts.out, store, "America/New_York", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	manifest.Representative = true
	raw, _ := json.Marshal(manifest)
	corpus := filepath.Join(store, manifest.Identity)
	if err := os.WriteFile(corpus+".manifest.json", raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return corpus
}

func TestBatchTemplateStartIsANoOpAndBindsTheSourceTree(t *testing.T) {
	if testing.Short() {
		t.Skip("builds agentdeck and scans a 64 MiB corpus")
	}
	batch := &batchState{Corpus: syntheticCorpus(t), Anchor: "2026-10-26", TZ: "America/New_York", Agentdeck: buildAgentdeck(t)}
	root, err := os.MkdirTemp("/private/tmp", "agentdeck-perf.")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	batch.Root, batch.Home = root, filepath.Join(root, "home")
	if err := batch.regenerate(); err != nil {
		t.Fatal(err)
	}
	if batch.WorkingCopy.Days != 4 || batch.WorkingCopy.Files != 400 || batch.Template.CoreSHA256 == "" {
		t.Fatalf("batch = %+v", batch)
	}

	noop := func() map[string]int64 {
		t.Helper()
		dir := privateTraceDir(t)
		counts, err := batch.noOpCheck(dir)
		if err != nil {
			t.Fatal(err)
		}
		return counts
	}
	for i := 0; i < 2; i++ {
		if counts := noop(); counts["read_bytes"] != 0 || counts["decoded_records"] != 0 {
			t.Fatalf("run unit %d from the template reread sources: %v", i, counts)
		}
	}

	// Any write to the sources (here an append) breaks the binding: the next
	// run unit is refused instead of silently rereading.
	if _, err := appendRecords(batch.Home, mustDay(t, batch.Anchor), 1, 1, false); err != nil {
		t.Fatal(err)
	}
	if err := batch.startRunUnit(stateFull); err == nil {
		t.Fatal("a changed source tree must not match the template binding")
	}
	batch.Dirty = true
	if err := batch.ensureClean(); err != nil {
		t.Fatal(err)
	}
	if counts := noop(); counts["read_bytes"] != 0 {
		t.Fatalf("regenerated copy is not a clean template start: %v", counts)
	}
	if batch.Generations != 2 {
		t.Fatalf("generations = %d", batch.Generations)
	}

	// A copy of the tree at another path has new inodes and ctimes.
	copyRoot := filepath.Join(root, "copy")
	if output, err := exec.Command("/bin/cp", "-cR", batch.Home, copyRoot).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, output)
	}
	copied, err := sourceBinding(copyRoot)
	if err != nil {
		t.Fatal(err)
	}
	if copied == batch.Binding {
		t.Fatal("a copied source tree must not share the template binding")
	}
}

func mustDay(t *testing.T, date string) time.Time {
	t.Helper()
	location, _ := time.LoadLocation("America/New_York")
	day, err := time.ParseInLocation("2006-01-02", date, location)
	if err != nil {
		t.Fatal(err)
	}
	return day
}

// The App standardizes its test HOME (dropping /private), and the product binds
// imported state to that string, so batches must use the App's form.
func TestAppPathFormMatchesTheAppStandardizedHome(t *testing.T) {
	for input, want := range map[string]string{
		"/private/tmp/agentdeck-perf.123":      "/tmp/agentdeck-perf.123",
		"/private/tmp/agentdeck-perf.123/home": "/tmp/agentdeck-perf.123/home",
		"/tmp/agentdeck-perf.123":              "/tmp/agentdeck-perf.123",
		"/private/var/folders/x":               "/private/var/folders/x",
	} {
		if got := appPathForm(input); got != want {
			t.Errorf("appPathForm(%q) = %q, want %q", input, got, want)
		}
	}
}
