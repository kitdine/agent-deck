package quota

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestClaudeIdentityKeyParallelCreationAndRecovery(t *testing.T) {
	const callers = 8
	root := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(root, claudeIdentityKeyPrefix+"abandoned")
	if err := os.WriteFile(stale, []byte("incomplete"), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-11 * time.Minute)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	type result struct {
		key [32]byte
		err error
	}
	start := make(chan struct{})
	results := make(chan result, callers)
	for i := 0; i < callers; i++ {
		go func() {
			<-start
			key, err := loadClaudeIdentityKey(root)
			results <- result{key, err}
		}()
	}
	close(start)
	first := <-results
	if first.err != nil || first.key == ([32]byte{}) {
		t.Fatalf("first creation failed: %v", first.err)
	}
	for i := 1; i < callers; i++ {
		next := <-results
		if next.err != nil || next.key != first.key {
			t.Fatalf("competing creation mismatch: %v", next.err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != claudeIdentityKeyName {
		t.Fatalf("unexpected key directory contents: %v, %v", entries, err)
	}
}

func TestClaudeIdentityKeyRejectsUnsafePathsAndUsesOpenDescriptor(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "private")
	key, err := loadClaudeIdentityKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, claudeIdentityKeyName)
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	moved := filepath.Join(dir, "original")
	if err := os.Rename(path, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(moved, path); err != nil {
		t.Fatal(err)
	}
	if _, err := loadClaudeIdentityKey(dir); err == nil {
		t.Fatal("accepted a symlink at the key path")
	}
	descriptorKey, err := readClaudeIdentityKeyFile(file)
	if err != nil || descriptorKey != key {
		t.Fatalf("validation moved off the original descriptor: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadClaudeIdentityKey(dir); err == nil {
		t.Fatal("accepted a FIFO")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(moved, path); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadClaudeIdentityKey(dir); err == nil {
		t.Fatal("accepted an overexposed key")
	}
	st, err := os.Stat(path)
	if err != nil || st.Mode().Perm() != 0644 {
		t.Fatalf("invalid existing key was rewritten: %v", err)
	}
}

func TestClaudeIdentityKeyComparisonVector(t *testing.T) {
	var key [32]byte
	for i := range key {
		key[i] = byte(i)
	}
	result := deriveClaudeComparison(key, "claude.ai", "Alice@example.test", "org:org-42")
	if hex.EncodeToString(result.correlation[:]) != "e9336ad95aa6882e427a9da5729dd740712f79d7a09bad4cd68a82c137a13e10" ||
		hex.EncodeToString(result.keyID[:]) != "ca7f88668fc00ad9" {
		t.Fatal("comparison does not match the A6.5 domain and key-ID vector")
	}
}

func TestClaudeIdentityKeyCompetingProcesses(t *testing.T) {
	root := t.TempDir()
	isolateClaudeAdapterTestEnv(t, root)
	state := filepath.Join(root, "state")
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	type child struct {
		cmd     *exec.Cmd
		barrier io.WriteCloser
		output  bytes.Buffer
	}
	var processes [2]child
	for i := range processes {
		process := &processes[i]
		process.cmd = exec.CommandContext(ctx, os.Args[0], "-test.run=^TestClaudeIdentityKeyProcessHelper$")
		process.cmd.Env = append(os.Environ(), "AGENTDECK_QUOTA_KEY_FIXTURE_ROOT="+state)
		process.cmd.Stdout = &process.output
		stdin, err := process.cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		process.barrier = stdin
		if err := process.cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = process.barrier.Close()
			if process.cmd.ProcessState == nil {
				_ = process.cmd.Process.Kill()
				_ = process.cmd.Wait()
			}
		})
	}
	var ids [2]string
	for i := range processes {
		if err := processes[i].barrier.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for i := range processes {
		process := &processes[i]
		if err := process.cmd.Wait(); err != nil {
			t.Fatalf("isolated key helper failed: %v; %s", err, process.output.String())
		}
		ids[i] = strings.SplitN(process.output.String(), "\n", 2)[0]
	}
	if ids[0] != ids[1] || len(ids[0]) != 16 {
		t.Fatal("competing processes did not see the same 8-byte key ID")
	}
}

func TestClaudeIdentityKeyProcessHelper(t *testing.T) {
	root, ok := os.LookupEnv("AGENTDECK_QUOTA_KEY_FIXTURE_ROOT")
	if !ok {
		t.Skip("isolated subprocess only")
	}
	// Refuse execution without the parent-controlled stdin barrier.
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		t.Fatal(err)
	}
	key, err := loadClaudeIdentityKey(root)
	if err != nil {
		t.Fatal(err)
	}
	comparison := deriveClaudeComparison(key, "claude.ai", "alice@example.test", "org:none")
	fmt.Printf("%x\n", comparison.keyID)
}
