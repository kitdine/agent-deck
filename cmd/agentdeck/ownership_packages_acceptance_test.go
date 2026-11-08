//go:build agentdeck_acceptance && darwin

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/backup"
	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/desktop"
	"github.com/kitdine/agent-deck/internal/doctor"
	"github.com/kitdine/agent-deck/internal/extension"
	"github.com/kitdine/agent-deck/internal/hookrefusal"
	"github.com/kitdine/agent-deck/internal/installation"
	"github.com/kitdine/agent-deck/internal/ownershiptest"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/scanruntime"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/telemetry"
	"github.com/kitdine/agent-deck/internal/watch"
)

var ownershipPackageCases = []string{
	"platform.GuardStateRoot", "platform.RevalidateOwnership", "platform.EnsureStateRoot", "platform.EnsureStateRootWithFS",
	"installation.StateRoot", "installation.StateRootWithHome",
	"store.Open", "store.OpenExisting", "store.OpenReadOnly", "store.OpenWithLockHeld", "store.OpenSessions", "store.OpenSessionsReadOnly",
	"store.AcquireLock", "store.AcquireScanLock", "store.AcquireQuotaRefreshLock", "store.AcquireDerivedSnapshotCacheLock", "store.AcquireServingSnapshotLock", "store.InspectScanLockContention", "store.OpenServingGenerationReader", "store.ReadServingGeneration",
	"credentialvault.Vault.Seal", "credentialvault.Vault.SealExisting", "credentialvault.Vault.Open", "credentialvault.Vault.InspectKey", "credentialvault.Vault.InspectKeyIDs", "credentialvault.Vault.InitializeNew",
	"hookrefusal.Read", "hookrefusal.Live", "hookrefusal.Write", "hookrefusal.Clear",
	"telemetry.LoadConfig", "telemetry.SaveConfig", "telemetry.EnsureToken", "telemetry.ReadToken", "telemetry.ReadListenerState", "telemetry.ReadStatus", "telemetry.WriteManagedBlock", "telemetry.RemoveManagedBlock", "telemetry.Serve", "telemetry.Receiver.ServeHTTP",
	"scanruntime.Client.Request", "scanruntime.Client.RequestWithProgress", "scanruntime.Serve", "scanruntime.WithMaintenance", "scanruntime.AcquireMaintenance",
	"doctor.Service.Check", "session.CheckHealth", "extension.DoctorFromStateRoot", "extension.DoctorFromStateRootWithDiscoverer",
	"backup.Service.Create", "backup.Restore",
	"desktop.Service.Build", "desktop.Service.BuildToday", "desktop.Service.PublishDerivedSnapshotCache", "desktop.Service.PublishServingSnapshot", "desktop.Service.ReadServingSnapshot", "desktop.Service.ReadOrBootstrapServingSnapshot",
	"provider.Service.Use", "provider.Service.UseCredential", "provider.Service.ProjectAttributionGateStatus", "provider.Service.RefreshProjectAttributionGate", "watch.Service.Poll",
}

func ownershipPackageProbe(name, root string) int {
	if strings.HasPrefix(name, "hold-") || strings.HasPrefix(name, "rounds-") {
		return ownershipSessionProbe(name, root)
	}
	ctx := context.Background()
	var err error
	var database *store.Store
	var lock *store.Lock
	var release func() error
	home := os.Getenv("HOME")
	vault := credentialvault.New(root, machineIdentity)
	service := desktop.Service{StateRoot: root, Home: home, Workdir: home}
	request := desktop.Request{WireVersion: desktop.WireVersion, RecentLimit: 5}
	switch name {
	case "platform.GuardStateRoot":
		_, err = platform.GuardStateRoot(root)
	case "platform.RevalidateOwnership":
		err = platform.RevalidateOwnership(root)
	case "platform.EnsureStateRoot":
		err = platform.EnsureStateRoot(root)
	case "platform.EnsureStateRootWithFS":
		err = platform.EnsureStateRootWithFS(platform.OSFileSystem{}, root)
	case "installation.StateRoot":
		_, err = installation.StateRoot(root)
	case "installation.StateRootWithHome":
		_, err = installation.StateRootWithHome(root, home)
	case "store.Open":
		database, err = store.Open(ctx, root)
	case "store.OpenExisting":
		database, err = store.OpenExisting(ctx, root)
	case "store.OpenReadOnly":
		database, err = store.OpenReadOnly(ctx, root)
	case "store.OpenWithLockHeld":
		database, err = store.OpenWithLockHeld(ctx, root)
	case "store.OpenSessions":
		database, err = store.OpenSessions(ctx, root)
	case "store.OpenSessionsReadOnly":
		database, err = store.OpenSessionsReadOnly(ctx, root)
	case "store.AcquireLock":
		lock, err = store.AcquireLock(ctx, root, time.Second)
	case "store.AcquireScanLock":
		lock, err = store.AcquireScanLock(ctx, root, time.Second)
	case "store.AcquireQuotaRefreshLock":
		lock, err = store.AcquireQuotaRefreshLock(ctx, root, time.Second)
	case "store.AcquireDerivedSnapshotCacheLock":
		lock, err = store.AcquireDerivedSnapshotCacheLock(ctx, root, time.Second)
	case "store.AcquireServingSnapshotLock":
		lock, err = store.AcquireServingSnapshotLock(ctx, root, time.Second)
	case "store.InspectScanLockContention":
		if store.InspectScanLockContention(root) != nil {
			return 9
		}
		fmt.Println("absent")
		return 0
	case "store.OpenServingGenerationReader":
		var reader *store.ServingGenerationReader
		reader, err = store.OpenServingGenerationReader(ctx, root)
		if reader != nil {
			reader.Close()
		}
	case "store.ReadServingGeneration":
		_, err = store.ReadServingGeneration(ctx, root)
	case "credentialvault.Vault.Seal":
		_, err = vault.Seal(ctx, "fixture", "synthetic-secret")
	case "credentialvault.Vault.SealExisting":
		_, err = vault.SealExisting(ctx, "fixture", "synthetic-secret")
	case "credentialvault.Vault.Open":
		_, err = vault.Open(ctx, "fixture", credentialvault.Sealed{Algorithm: credentialvault.AlgorithmAES256GCM, KeyVersion: credentialvault.KeyVersion, KeyID: "synthetic", Nonce: make([]byte, 12), Ciphertext: make([]byte, 16)})
	case "credentialvault.Vault.InspectKey":
		_, err = vault.InspectKey(ctx)
	case "credentialvault.Vault.InspectKeyIDs":
		_, err = vault.InspectKeyIDs(ctx)
	case "credentialvault.Vault.InitializeNew":
		_, err = vault.InitializeNew(ctx)
	case "hookrefusal.Read":
		if _, ok := hookrefusal.Read(root); ok {
			return 9
		}
		fmt.Println("absent")
		return 0
	case "hookrefusal.Live":
		if _, ok := hookrefusal.Live(root, 1); ok {
			return 9
		}
		fmt.Println("absent")
		return 0
	case "hookrefusal.Write":
		err = hookrefusal.Write(root, 2, 1)
	case "hookrefusal.Clear":
		err = hookrefusal.Clear(root)
	case "telemetry.LoadConfig":
		_, err = telemetry.LoadConfig(root)
	case "telemetry.SaveConfig":
		err = telemetry.SaveConfig(root, telemetry.Config{Port: 4318, RetentionDays: 7})
	case "telemetry.EnsureToken":
		_, err = telemetry.EnsureToken(root)
	case "telemetry.ReadToken":
		_, err = telemetry.ReadToken(root)
	case "telemetry.ReadListenerState":
		_, err = telemetry.ReadListenerState(root)
	case "telemetry.ReadStatus":
		_, err = telemetry.ReadStatus(ctx, root, home, func() (*store.Store, error) { return store.OpenReadOnly(ctx, root) })
	case "telemetry.WriteManagedBlock":
		err = telemetry.WriteManagedBlock(home, root, 4318, strings.Repeat("a", 64), time.Now())
	case "telemetry.RemoveManagedBlock":
		_, err = telemetry.RemoveManagedBlock(home, root)
	case "telemetry.Serve":
		err = telemetry.Serve(ctx, telemetry.ServeOptions{StateRoot: root, Home: home})
	case "telemetry.Receiver.ServeHTTP":
		token := strings.Repeat("a", 64)
		receiver := &telemetry.Receiver{StateRoot: root, Token: token, Journal: &telemetry.Journal{Dir: telemetry.JournalDir(root)}, OwnershipLost: func(cause error) { err = cause }}
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, telemetry.LogsPath, strings.NewReader(`{"resourceLogs":[]}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set(telemetry.TokenHeader, token)
		receiver.ServeHTTP(response, request)
		if response.Code != http.StatusServiceUnavailable {
			fmt.Fprintln(os.Stderr, "receiver did not refuse before journal access")
			return 9
		}
	case "scanruntime.Client.Request":
		_, err = (scanruntime.Client{StateRoot: root, Home: home, ForceLocal: true}).Request(ctx, scanruntime.ScopeBoth)
	case "scanruntime.Client.RequestWithProgress":
		_, err = (scanruntime.Client{StateRoot: root, Home: home, ForceLocal: true}).RequestWithProgress(ctx, scanruntime.ScopeBoth, func(scanruntime.Progress) {})
	case "scanruntime.Serve":
		err = scanruntime.Serve(ctx, root)
	case "scanruntime.WithMaintenance":
		err = scanruntime.WithMaintenance(ctx, root, time.Second, func(context.Context) error { return nil })
	case "scanruntime.AcquireMaintenance":
		release, err = scanruntime.AcquireMaintenance(ctx, root, time.Second)
	case "doctor.Service.Check":
		var report doctor.Report
		report, err = (doctor.Service{StateRoot: root, Home: home, Workdir: home}).Check(ctx, false)
		if err == nil && report.Partial && len(report.Checks) == 2 && report.Checks[1].Code == "state_ownership_refused" {
			fmt.Println("partial refusal")
			return 0
		}
	case "session.CheckHealth":
		_, err = session.CheckHealth(ctx, root, false)
	case "extension.DoctorFromStateRoot":
		_, err = extension.DoctorFromStateRoot(ctx, root, home, home)
	case "extension.DoctorFromStateRootWithDiscoverer":
		_, err = extension.DoctorFromStateRootWithDiscoverer(ctx, root, home, home, nil)
	case "backup.Service.Create":
		var core *store.Store
		core, err = store.Open(ctx, filepath.Join(filepath.Dir(home), "backup-source"))
		if err == nil {
			defer core.Close()
			_, err = (backup.Service{StateRoot: root, Core: core, Vault: newCredentialVault(filepath.Join(filepath.Dir(home), "backup-source"))}).Create(ctx, filepath.Join(filepath.Dir(home), "new.adb"), "passphrase", false)
		}
	case "backup.Restore":
		_, err = backup.Restore(ctx, filepath.Join(filepath.Dir(home), "input.adb"), root, "passphrase", machineIdentity)
	case "desktop.Service.Build":
		_, err = service.Build(ctx, request)
	case "desktop.Service.BuildToday":
		_, err = service.BuildToday(ctx)
	case "desktop.Service.PublishDerivedSnapshotCache":
		err = service.PublishDerivedSnapshotCache(ctx, desktop.WireVersion)
	case "desktop.Service.PublishServingSnapshot":
		_, err = service.PublishServingSnapshot(ctx, request, true)
	case "desktop.Service.ReadServingSnapshot":
		_, err = service.ReadServingSnapshot(ctx)
	case "desktop.Service.ReadOrBootstrapServingSnapshot":
		_, err = service.ReadOrBootstrapServingSnapshot(ctx, request)
	case "provider.Service.Use":
		err = (provider.Service{StateRoot: root}).Use(ctx, "p", provider.ClientCodex, filepath.Join(home, ".codex", "config.toml"), "")
	case "provider.Service.UseCredential":
		err = (provider.Service{StateRoot: root}).UseCredential(ctx, "p", provider.ClientCodex, "default", filepath.Join(home, ".codex", "config.toml"), "", false)
	case "provider.Service.ProjectAttributionGateStatus":
		_, err = (provider.Service{StateRoot: root}).ProjectAttributionGateStatus(ctx)
	case "provider.Service.RefreshProjectAttributionGate":
		err = (provider.Service{StateRoot: root}).RefreshProjectAttributionGate(ctx)
	case "watch.Service.Poll":
		w := watch.Service{StateRoot: root}
		_, err = w.Poll(ctx)
	default:
		fmt.Fprintln(os.Stderr, "unknown package probe", name)
		return 8
	}
	if database != nil {
		database.Close()
	}
	if lock != nil {
		lock.Release()
	}
	if release != nil {
		release()
	}
	var refused *platform.ErrStateOwnershipRefused
	if !errors.As(err, &refused) {
		fmt.Fprintf(os.Stderr, "probe %s returned %v\n", name, err)
		return 9
	}
	renderStateOwnershipRefusal(os.Stderr, refused)
	return 1
}

func ownershipProbeChild(f ownershiptest.Fixture, name, root string) *exec.Cmd {
	command := exec.Command(os.Args[0], "-test.run=^$")
	command.Env = f.Environment("AGENTDECK_OWNERSHIP_PROBE="+name, "AGENTDECK_OWNERSHIP_ROOT="+root)
	command.Dir = f.Home
	return command
}

func TestOwnershipPackageEntrypointsBehindBarrier(t *testing.T) {
	for _, name := range ownershipPackageCases {
		t.Run(name, func(t *testing.T) {
			f := ownershiptest.New(t)
			if name == "backup.Restore" {
				seedOwnershipCommand(t, f, filepath.Join(f.Base, "unused"), ownershipCommandCase{name: "backup.restore"})
			}
			f.Controls(t, ownershipChild(f, []string{"--state-dir", f.Real, "version"}, false))
			command := ownershipProbeChild(f, name, f.Real)
			f.Sandbox(command)
			out, err := command.CombinedOutput()
			want := 1
			if name == "hookrefusal.Read" || name == "hookrefusal.Live" || name == "store.InspectScanLockContention" || name == "doctor.Service.Check" {
				want = 0
			}
			if command.ProcessState == nil || command.ProcessState.ExitCode() != want {
				t.Fatalf("%s: expected exit %d, got %v, %s", name, want, err, out)
			}
			if want == 1 && !bytes.Contains(out, []byte("state_ownership_refused")) {
				t.Fatalf("wrong refusal: %s", out)
			}
			t.Logf("direct %s: denied before protected access; both controls passed", name)
		})
	}
}

// Structural enumeration is separate from the CLI command inventory. Pure
// constructors/path functions and APIs that consume an already guarded Store
// are explained explicitly rather than silently omitted from the matrix.
func TestOwnershipPackageEntrypointInventory(t *testing.T) {
	pure := map[string]string{
		"platform.StateRoot":                  "pure legacy path constructor; no filesystem access",
		"credentialvault.New":                 "lazy constructor; all six I/O methods are exercised",
		"credentialvault.Vault.KeyPath":       "pure path constructor; key I/O is guarded",
		"provider.ProjectAttributionGatePath": "pure path constructor",
		"telemetry.Dir":                       "pure path constructor",
		"telemetry.JournalDir":                "pure path constructor",
		"desktop.Service.RefreshQuota":        "AcquireQuotaRefreshLock guards before quota work; supplied Store owns its handle",
		"desktop.Service.BuildSubscription":   "supplied guarded Store; quota APIs own their lock",
	}
	covered := map[string]bool{}
	for _, name := range ownershipPackageCases {
		if covered[name] {
			t.Fatalf("duplicate probe %s", name)
		}
		covered[name] = true
	}
	var missing []string
	for _, pkg := range []string{"platform", "installation", "store", "credentialvault", "scanruntime", "hookrefusal", "telemetry", "doctor", "desktop", "backup", "session", "extension", "provider", "watch"} {
		files, err := filepath.Glob(filepath.Join("..", "..", "internal", pkg, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			tree, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, decl := range tree.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || !fn.Name.IsExported() {
					continue
				}
				name := pkg + "." + fn.Name.Name
				receiver := ""
				if fn.Recv != nil {
					field := fn.Recv.List[0]
					typ := field.Type
					if ptr, ok := typ.(*ast.StarExpr); ok {
						typ = ptr.X
					}
					if id, ok := typ.(*ast.Ident); ok {
						name = pkg + "." + id.Name + "." + fn.Name.Name
					}
					if len(field.Names) > 0 {
						receiver = field.Names[0].Name
					}
				}
				usesRoot := false
				for _, field := range fn.Type.Params.List {
					for _, param := range field.Names {
						if param.Name == "stateRoot" || param.Name == "stateDir" || param.Name == "root" || param.Name == "targetRoot" {
							usesRoot = true
						}
					}
				}
				if receiver != "" {
					ast.Inspect(fn.Body, func(n ast.Node) bool {
						if s, ok := n.(*ast.SelectorExpr); ok && (s.Sel.Name == "StateRoot" || s.Sel.Name == "stateRoot") {
							if id, ok := s.X.(*ast.Ident); ok && id.Name == receiver {
								usesRoot = true
							}
						}
						return true
					})
				}
				if usesRoot && !covered[name] && pure[name] == "" {
					missing = append(missing, name)
				}
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("uncovered exported state-root consumers: %q", missing)
	}
}
