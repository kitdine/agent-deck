//go:build darwin

package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRegistrationNativeMetadataIsBoundedAndAliasAware(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "旧版 Fixture.appex")
	contents := filepath.Join(bundle, "Contents")
	if err := os.MkdirAll(contents, 0700); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.example.registration-fixture</string><key>CFBundleShortVersionString</key><string>1</string><key>CFBundleVersion</key><string>2</string></dict></plist>`
	path := filepath.Join(contents, "Info.plist")
	if err := os.WriteFile(path, []byte(plist), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "Alias.appex")
	if err := os.Symlink(bundle, alias); err != nil {
		t.Fatal(err)
	}
	// Substitute only target constants; neither fixture ID is registered with OS.
	script := strings.NewReplacer("/Applications/AgentDeck.app/Contents/PlugIns/AgentDeckWidget.appex", bundle, "com.kitdine.agentdeck.widget", "com.example.registration-fixture").Replace(registrationScript)
	paths, _ := json.Marshal([]string{alias, bundle})
	run := func() registrationNative {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		output, reason := registrationCommand(ctx, "/usr/bin/osascript", "-l", "JavaScript", "-e", script, "widget", string(paths))
		if reason != "" {
			t.Fatal(reason)
		}
		var result registrationNative
		if !decodeRegistration(output, &result) {
			t.Fatalf("invalid metadata output %s", output)
		}
		return result
	}
	for _, binary := range []bool{false, true} {
		if binary {
			if _, reason := registrationCommand(context.Background(), "/usr/bin/plutil", "-convert", "binary1", path); reason != "" {
				t.Fatal(reason)
			}
		}
		r := run()
		if !r.Complete || len(r.Entries) != 1 || !r.Entries[0].Canonical || r.Entries[0].Build != "2" {
			t.Fatalf("binary=%v %#v", binary, r)
		}
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 262144)), 0600); err != nil {
		t.Fatal(err)
	}
	if r := run(); r.Complete || r.Reason != "output_limit" {
		t.Fatalf("exact byte cap %#v", r)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, reason := registrationCommand(ctx, "/usr/bin/osascript", "-l", "JavaScript", "-e", script, "widget", string(paths))
	if reason != "timeout" || time.Since(start) > 500*time.Millisecond {
		t.Fatalf("blocked metadata not bounded: %s %s", reason, time.Since(start))
	}
}

func TestRegistrationHostLimitUsesUniqueNormalizedIdentities(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "Fixture.app")
	contents := filepath.Join(bundle, "Contents")
	if err := os.MkdirAll(contents, 0700); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.example.registration-fixture</string><key>CFBundleShortVersionString</key><string>1</string><key>CFBundleVersion</key><string>2</string></dict></plist>`
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(plist), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "Alias.app")
	if err := os.Symlink(bundle, alias); err != nil {
		t.Fatal(err)
	}
	for _, unique := range []bool{false, true} {
		t.Logf("unique=%v", unique)
		paths := []string{}
		for i := 0; i < 65; i++ {
			path := bundle
			if i%2 == 1 {
				path = alias
			}
			if unique && i > 0 {
				path = filepath.Join(root, fmt.Sprintf("Missing%d.app", i))
			}
			paths = append(paths, path)
		}
		raw, _ := json.Marshal(paths)
		// Only the source fixture and expected/canonical IDs are substituted. The
		// production normalization, metadata, loop and unique-limit logic execute.
		script := strings.ReplaceAll(registrationScript, "'/Applications/AgentDeck.app'", strconv.Quote(bundle))
		script = strings.ReplaceAll(script, "'com.kitdine.agentdeck'", "'com.example.registration-fixture'")
		mock := `var fixturePaths=` + string(raw) + `;var ws={URLsForApplicationsWithBundleIdentifier:function(id){var p=id==='com.apple.finder'?['/System/Library/CoreServices/Finder.app']:fixturePaths;var a=$.NSMutableArray.array;for(var i=0;i<p.length;i++)a.addObject($.NSURL.fileURLWithPath(p[i]));return a}};`
		script = strings.Replace(script, "var ws=$.NSWorkspace.sharedWorkspace;", mock, 1)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		output, reason := registrationCommand(ctx, "/usr/bin/osascript", "-l", "JavaScript", "-e", script, "host", "null")
		cancel()
		var result registrationNative
		if reason != "" || !decodeRegistration(output, &result) {
			t.Fatalf("%s %s", reason, output)
		}
		if !unique && (!result.Complete || len(result.Entries) != 1 || !result.Entries[0].Canonical) {
			t.Fatalf("aliases incorrectly capped: %#v", result)
		}
		if unique && (result.Complete || result.Reason != "entry_limit" || len(result.Entries) != 64) {
			t.Fatalf("unique cap not enforced: %#v", result)
		}
	}
}

func TestRegistrationCanonicalAbsenceDoesNotUseUnsafeNSErrorRef(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "Missing.app")
	script := strings.ReplaceAll(registrationScript, "'/Applications/AgentDeck.app'", strconv.Quote(bundle))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	output, reason := registrationCommand(ctx, "/usr/bin/osascript", "-l", "JavaScript", "-e", script, "host", "null")
	var result registrationNative
	if reason != "" || !decodeRegistration(output, &result) || !result.Absent {
		t.Fatalf("missing canonical host not classified safely: %s %s", reason, output)
	}
	// EACCES differs from genuine ENOENT. Do not call an unreadable parent absent.
	blocked := filepath.Join(root, "blocked")
	if err := os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0700) })
	if err := os.Chmod(blocked, 0000); err != nil {
		t.Fatal(err)
	}
	inaccessible := filepath.Join(blocked, "Missing.app")
	script = strings.ReplaceAll(registrationScript, "'/Applications/AgentDeck.app'", strconv.Quote(inaccessible))
	secondCtx, secondCancel := context.WithTimeout(context.Background(), time.Second)
	defer secondCancel()
	output, reason = registrationCommand(secondCtx, "/usr/bin/osascript", "-l", "JavaScript", "-e", script, "host", "null")
	if reason != "" || !decodeRegistration(output, &result) || result.Absent || result.Complete {
		t.Fatalf("unreadable canonical host confused with absence: %s %s", reason, output)
	}
}
