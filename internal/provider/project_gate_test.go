package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/store"
)

func TestProviderUseMaintainsProjectAttributionGateAcrossClients(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	stateRoot := filepath.Join(root, "state")
	database, err := store.Open(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	service := Service{Store: database, Home: root, StateRoot: stateRoot}
	if _, _, err := service.SetWrapper(
		ctx,
		OfficialProviderName,
		"https://wrapper.example",
		WrapperKindHeadroom,
		false,
	); err != nil {
		t.Fatal(err)
	}
	codexConfig := filepath.Join(root, "config.toml")
	claudeConfig := filepath.Join(root, "settings.json")
	if err := os.WriteFile(codexConfig, []byte("model = \"synthetic\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudeConfig, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	use := func(client Client, config, backup string, via bool) {
		t.Helper()
		if err := service.UseCredential(
			ctx,
			OfficialProviderName,
			client,
			"",
			config,
			filepath.Join(root, backup),
			via,
		); err != nil {
			t.Fatal(err)
		}
	}
	gatePath := ProjectAttributionGatePath(stateRoot)

	use(ClientCodex, codexConfig, "codex-via.backup", true)
	assertProjectAttributionGateFile(t, gatePath)
	use(ClientClaude, claudeConfig, "claude-via.backup", true)
	assertProjectAttributionGateFile(t, gatePath)

	use(ClientCodex, codexConfig, "codex-direct.backup", false)
	assertProjectAttributionGateFile(t, gatePath)
	use(ClientClaude, claudeConfig, "claude-direct.backup", false)
	if _, err := os.Lstat(gatePath); !os.IsNotExist(err) {
		t.Fatalf("gate after last eligible route removed = %v, want absent", err)
	}
}

func TestProjectAttributionGateReportsMissingStaleAndInvalidStates(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	stateRoot := filepath.Join(root, "state")
	database, err := store.Open(ctx, stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	service := Service{Store: database, Home: root, StateRoot: stateRoot}
	if _, _, err := service.SetWrapper(
		ctx,
		OfficialProviderName,
		"https://wrapper.example",
		WrapperKindHeadroom,
		false,
	); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config.toml")
	if err := os.WriteFile(config, []byte("model = \"synthetic\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.UseCredential(
		ctx,
		OfficialProviderName,
		ClientCodex,
		"",
		config,
		filepath.Join(root, "via.backup"),
		true,
	); err != nil {
		t.Fatal(err)
	}

	gatePath := ProjectAttributionGatePath(stateRoot)
	if err := os.Remove(gatePath); err != nil {
		t.Fatal(err)
	}
	status, err := service.ProjectAttributionGateStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Required || status.Present || status.Consistent {
		t.Fatalf("missing gate status = %#v", status)
	}

	if err := os.Mkdir(gatePath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := service.UseCredential(
		ctx,
		OfficialProviderName,
		ClientCodex,
		"",
		config,
		filepath.Join(root, "second-via.backup"),
		true,
	); err != nil {
		t.Fatalf("completed switch failed on gate refresh: %v", err)
	}
	if _, err := service.ProjectAttributionGateStatus(ctx); err == nil {
		t.Fatal("invalid gate was not reported")
	}

	if err := os.WriteFile(filepath.Join(gatePath, "keep"), nil, platform.FileMode); err != nil {
		t.Fatal(err)
	}
	if err := service.UseCredential(
		ctx,
		OfficialProviderName,
		ClientCodex,
		"",
		config,
		filepath.Join(root, "direct.backup"),
		false,
	); err != nil {
		t.Fatalf("completed switch failed on gate removal: %v", err)
	}
	eligibility, err := service.ProjectRouteEligibility(ctx, ClientCodex)
	if err != nil || eligibility != ProjectRouteNoWrapper {
		t.Fatalf("route after ignored gate removal failure = %q, %v", eligibility, err)
	}
	if _, err := service.ProjectAttributionGateStatus(ctx); err == nil {
		t.Fatal("failed gate removal was not reported")
	}
	if err := os.Remove(filepath.Join(gatePath, "keep")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(gatePath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gatePath, nil, platform.FileMode); err != nil {
		t.Fatal(err)
	}
	status, err = service.ProjectAttributionGateStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Required || !status.Present || status.Consistent {
		t.Fatalf("stale gate status = %#v", status)
	}
}

func assertProjectAttributionGateFile(t *testing.T, path string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 0 {
		t.Fatalf("gate contents = %q, want empty", contents)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != platform.FileMode {
		t.Fatalf("gate mode = %v, want regular %04o", info.Mode(), platform.FileMode)
	}
}
