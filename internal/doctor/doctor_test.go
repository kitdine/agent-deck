package doctor

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kitdine/agent-deck/internal/credentialvault"
	"github.com/kitdine/agent-deck/internal/provider"
	"github.com/kitdine/agent-deck/internal/store"
)

func doctorVault(stateRoot string) *credentialvault.Vault {
	return credentialvault.New(stateRoot, func(context.Context) (string, error) { return "synthetic-machine", nil })
}

func TestCheckMissingStateIsReadOnly(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	report, err := (Service{StateRoot: root}).Check(context.Background(), false)
	if err != nil || report.Healthy || report.Problems != 1 || report.Checks[0].Code != "state_missing" {
		t.Fatalf("Check = %#v, %v", report, err)
	}
	if _, err = os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("doctor created state: %v", err)
	}
}

func TestCheckOlderAndFutureSchemasAreSafeAndReadable(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, "DROP TABLE usage_tool_calls; DROP INDEX usage_events_client_session; UPDATE schema_metadata SET version=12"); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	report, err := (Service{StateRoot: state, Home: t.TempDir(), Workdir: t.TempDir(), Vault: doctorVault(state)}).Check(ctx, true)
	if err != nil || !hasCode(report, "schema_outdated") {
		t.Fatalf("schema 12 report = %#v, %v", report, err)
	}
	future := filepath.Join(t.TempDir(), "future")
	database, err = store.Open(ctx, future)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, "UPDATE schema_metadata SET version=99"); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	report, err = (Service{StateRoot: future}).Check(ctx, false)
	if err != nil || !hasCode(report, store.ErrUnknownSchema.Code) {
		t.Fatalf("future schema report = %#v, %v", report, err)
	}
}

func TestCheckUsageSchemaMatrixNeverLeaksSQL(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name, checkName, status, code, recovery string
		version, count                          int
		drop                                    bool
	}{
		{"schema12", "schema", "warning", "schema_outdated", "agentdeck state migrate", 12, 12, true},
		{"schema_current", "schema", "ok", "", "", store.CurrentSchemaVersion, store.CurrentSchemaVersion, false},
		{"schema_current_missing_tool_calls", "schema", "error", "schema_incompatible", "", store.CurrentSchemaVersion, store.CurrentSchemaVersion, true},
		{"future", "database", "error", "unknown_schema", "", 99, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, full := range []bool{false, true} {
				state := filepath.Join(t.TempDir(), "state")
				database, err := store.Open(ctx, state)
				if err != nil {
					t.Fatal(err)
				}
				if test.drop {
					if _, err = database.Exec(ctx, "DROP TABLE usage_tool_calls"); err != nil {
						database.Close()
						t.Fatal(err)
					}
				}
				if test.version != store.CurrentSchemaVersion {
					if _, err = database.Exec(ctx, "UPDATE schema_metadata SET version=?", test.version); err != nil {
						database.Close()
						t.Fatal(err)
					}
				}
				database.Close()
				report, err := (Service{StateRoot: state, Home: t.TempDir(), Workdir: t.TempDir(), Vault: doctorVault(state)}).Check(ctx, full)
				if err != nil || len(report.Checks) == 0 {
					t.Fatalf("full=%t report=%#v err=%v", full, report, err)
				}
				var matched *Check
				for _, check := range report.Checks {
					if strings.Contains(strings.ToLower(check.Code), "sql") {
						t.Fatalf("leaked sql check %#v", check)
					}
					if check.Name == "usage_tool_calls" {
						t.Fatalf("full=%t emitted contradictory usage_tool_calls check: %#v", full, report.Checks)
					}
					if check.Name == test.checkName {
						copy := check
						matched = &copy
					}
				}
				if matched == nil || matched.Status != test.status || matched.Code != test.code || matched.Count != test.count || matched.Recovery != test.recovery {
					t.Fatalf("full=%t check=%#v, want name=%s status=%s code=%s count=%d recovery=%q", full, matched, test.checkName, test.status, test.code, test.count, test.recovery)
				}
				if test.name == "schema_current" && (hasCode(report, "schema_outdated") || hasCode(report, "schema_incompatible")) {
					t.Fatalf("full=%t normal schema report=%#v", full, report)
				}
			}
		})
	}
}

func TestFullCheckReportsProblemsWithoutChangingDatabases(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.CreateProvider(ctx, store.Provider{Name: "missing-secret", Endpoint: "https://example.invalid", CredentialRef: "missing", Multiplier: "1", Clients: []store.ClientMapping{{Client: "codex"}}}); err != nil {
		t.Fatal(err)
	}
	if err = database.CreateOperation(ctx, store.Operation{ID: "pending", Kind: "provider.use", State: "prepared"}); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO price_catalogs(version,source_kind,source_url,content_sha256,imported_at,effective_from,currency,schema_version) VALUES('invalid','unknown','', 'short', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', 'USD', 1)`); err != nil {
		t.Fatal(err)
	}
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	sessions, err := store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err = sessions.Close(); err != nil {
		t.Fatal(err)
	}
	sessions, err = store.OpenSessions(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = sessions.DB.ExecContext(ctx, `INSERT INTO session_sources(source_path,identity,cursor,size,modified_at,prefix_hash,priority,parser_version,scanned_at) VALUES(?,?,?,?,?,?,?,?,?)`, filepath.Join(t.TempDir(), "missing.jsonl"), "synthetic", 0, 0, 0, "", 0, 1, "2026-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if err = sessions.Close(); err != nil {
		t.Fatal(err)
	}
	before := fileDigest(t, filepath.Join(root, "agentdeck.sqlite3"))
	report, err := (Service{StateRoot: root, Home: t.TempDir(), Workdir: t.TempDir(), Vault: doctorVault(root), Now: func() time.Time { return time.Unix(1000, 0) }}).Check(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.Healthy || !hasCode(report, "pending_operations") || !hasCode(report, "credential_missing") || !hasCode(report, "session_source_unreadable") || !hasCode(report, "price_provenance_invalid") {
		t.Fatalf("report = %#v", report)
	}
	after := fileDigest(t, filepath.Join(root, "agentdeck.sqlite3"))
	if before != after {
		t.Fatal("doctor changed core database")
	}
}

func TestProviderCheckAcceptsOfficialAfterBearer(t *testing.T) {
	ctx := context.Background()
	state, home := filepath.Join(t.TempDir(), "state"), t.TempDir()
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	vault := doctorVault(state)
	providerService := provider.Service{Store: database, Vault: vault, Home: home, StateRoot: state}
	if _, err := providerService.Add(ctx, provider.Definition{Name: "bearer", Endpoint: "https://provider.example", Clients: []provider.Client{provider.ClientCodex}, CredentialRef: "ref"}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(config), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("model = 'keep'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := providerService.Use(ctx, "bearer", provider.ClientCodex, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := providerService.Use(ctx, provider.OfficialProviderName, provider.ClientCodex, "", ""); err != nil {
		t.Fatal(err)
	}
	report := Report{}
	if err := (Service{StateRoot: state, Home: home, Vault: vault}).checkProviders(ctx, database, &report, false); err != nil {
		t.Fatal(err)
	}
	if hasCode(report, "provider_config_drift") {
		t.Fatalf("official provider reported drift: %#v", report)
	}
}

func TestProviderCheckValidatesEveryNamedCredential(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	vault := doctorVault(state)
	service := provider.Service{Store: database, Vault: vault}
	if _, err = service.Add(ctx, provider.Definition{Name: "multi", Endpoint: "https://example.invalid", CredentialRef: "default-ref", Clients: []provider.Client{provider.ClientCodex}}, "default-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.AddCredential(ctx, "multi", "missing", "https://example.invalid", "1", []provider.Client{provider.ClientCodex}, "temporary"); err != nil {
		t.Fatal(err)
	}
	missing, err := database.ProviderCredential(ctx, "multi", "missing")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `DELETE FROM credential_secrets WHERE credential_id=?`, missing.ID); err != nil {
		t.Fatal(err)
	}
	report := Report{}
	if err = (Service{StateRoot: state, Home: t.TempDir(), Vault: vault}).checkProviders(ctx, database, &report, false); err != nil {
		t.Fatal(err)
	}
	for _, check := range report.Checks {
		if check.Name == "provider_credentials" {
			if check.Code != "credential_missing" || check.Count != 1 {
				t.Fatalf("credential check = %#v", check)
			}
			return
		}
	}
	t.Fatal("provider credential check missing")
}

func TestProviderCheckTreatsZeroNamedCredentialsAsMissing(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	vault := doctorVault(state)
	service := provider.Service{Store: database, Vault: vault}
	if _, err = service.Add(ctx, provider.Definition{Name: "empty", Endpoint: "https://example.invalid", CredentialRef: "empty-ref", Clients: []provider.Client{provider.ClientCodex}}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	if err = service.RemoveNamedCredential(ctx, "empty", "default"); err != nil {
		t.Fatal(err)
	}
	report := Report{}
	if err = (Service{StateRoot: state, Home: t.TempDir(), Vault: vault}).checkProviders(ctx, database, &report, false); err != nil {
		t.Fatal(err)
	}
	for _, check := range report.Checks {
		if check.Name == "provider_credentials" {
			if check.Code != "credential_missing" || check.Count != 1 {
				t.Fatalf("credential check = %#v", check)
			}
			return
		}
	}
	t.Fatal("provider credential check missing")
}

func TestFullProviderCheckAuthenticatesCredentialCiphertext(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	vault := doctorVault(state)
	service := provider.Service{Store: database, Vault: vault}
	if _, err = service.Add(ctx, provider.Definition{Name: "tampered", Endpoint: "https://example.invalid", Clients: []provider.Client{provider.ClientCodex}}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE credential_secrets SET ciphertext = ciphertext || X'00'`); err != nil {
		t.Fatal(err)
	}
	report := Report{}
	if err = (Service{StateRoot: state, Home: t.TempDir(), Vault: vault}).checkProviders(ctx, database, &report, true); err != nil {
		t.Fatal(err)
	}
	check := findCheck(report, "provider_credential_authentication", credentialvault.ErrCiphertextInvalid.Error())
	if check == nil {
		t.Fatalf("full credential authentication report = %#v", report)
	}
	if check.Recovery != "agentdeck credential update tampered --credential default --rotate" {
		t.Fatalf("authentication recovery = %q", check.Recovery)
	}
}

func TestFullProviderCheckAuthenticatesValidCandidatesAfterQuickFailure(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	vault := doctorVault(state)
	service := provider.Service{Store: database, Vault: vault}
	for _, name := range []string{"unsupported", "tampered"} {
		if _, err = service.Add(ctx, provider.Definition{Name: name, Endpoint: "https://example.invalid", Clients: []provider.Client{provider.ClientCodex}}, name+"-secret"); err != nil {
			t.Fatal(err)
		}
	}
	unsupported, err := database.ProviderCredential(ctx, "unsupported", "default")
	if err != nil {
		t.Fatal(err)
	}
	tampered, err := database.ProviderCredential(ctx, "tampered", "default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE credential_secrets SET key_version=99 WHERE credential_id=?`, unsupported.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE credential_secrets SET ciphertext=ciphertext || X'00' WHERE credential_id=?`, tampered.ID); err != nil {
		t.Fatal(err)
	}

	report := Report{}
	if err = (Service{StateRoot: state, Home: t.TempDir(), Vault: vault}).checkProviders(ctx, database, &report, true); err != nil {
		t.Fatal(err)
	}
	quick := findCheck(report, "provider_credential_key", credentialvault.ErrKeyVersionUnsupported.Error())
	if quick == nil || quick.Recovery != "" {
		t.Fatalf("unsupported-version check = %#v in %#v", quick, report)
	}
	authenticated := findCheck(report, "provider_credential_authentication", credentialvault.ErrCiphertextInvalid.Error())
	if authenticated == nil || authenticated.Recovery != "agentdeck credential update tampered --credential default --rotate" {
		t.Fatalf("remaining authentication check = %#v in %#v", authenticated, report)
	}
}

func TestQuickProviderCheckFailsClosedWithoutRegeneratingCredentialKey(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	vault := doctorVault(state)
	service := provider.Service{Store: database, Vault: vault}
	if _, err = service.Add(ctx, provider.Definition{Name: "missing-key", Endpoint: "https://example.invalid", Clients: []provider.Client{provider.ClientCodex}}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(vault.KeyPath()); err != nil {
		t.Fatal(err)
	}
	report := Report{}
	if err = (Service{StateRoot: state, Home: t.TempDir(), Vault: vault}).checkProviders(ctx, database, &report, false); err != nil {
		t.Fatal(err)
	}
	if !hasCode(report, credentialvault.ErrKeyMissing.Error()) {
		t.Fatalf("quick key report = %#v", report)
	}
	if check := findCheck(report, "provider_credential_key", credentialvault.ErrKeyMissing.Error()); check == nil || check.Recovery != "" {
		t.Fatalf("missing-key recovery = %#v", check)
	}
	if _, statErr := os.Stat(vault.KeyPath()); !os.IsNotExist(statErr) {
		t.Fatalf("doctor regenerated missing key: %v", statErr)
	}
}

func TestQuickProviderCheckReportsMissingKeyWithMalformedOnlyCiphertext(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	vault := doctorVault(state)
	service := provider.Service{Store: database, Vault: vault}
	if _, err = service.Add(ctx, provider.Definition{Name: "malformed", Endpoint: "https://example.invalid", Clients: []provider.Client{provider.ClientCodex}}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE credential_secrets SET nonce=X'00'`); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(vault.KeyPath()); err != nil {
		t.Fatal(err)
	}

	report := Report{}
	if err = (Service{StateRoot: state, Home: t.TempDir(), Vault: vault}).checkProviders(ctx, database, &report, false); err != nil {
		t.Fatal(err)
	}
	malformed := findCheck(report, "provider_credential_key", credentialvault.ErrCiphertextInvalid.Error())
	if malformed == nil || malformed.Recovery != "" {
		t.Fatalf("malformed ciphertext check = %#v in %#v", malformed, report)
	}
	missing := findCheck(report, "provider_credential_key", credentialvault.ErrKeyMissing.Error())
	if missing == nil || missing.Recovery != "" {
		t.Fatalf("missing key check = %#v in %#v", missing, report)
	}
}

func TestQuickProviderCheckOffersTargetedRecoveryForMalformedCiphertext(t *testing.T) {
	ctx := context.Background()
	state := filepath.Join(t.TempDir(), "state")
	database, err := store.Open(ctx, state)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	vault := doctorVault(state)
	service := provider.Service{Store: database, Vault: vault}
	if _, err = service.Add(ctx, provider.Definition{Name: "malformed", Endpoint: "https://example.invalid", Clients: []provider.Client{provider.ClientCodex}}, "synthetic-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE credential_secrets SET ciphertext=X'00'`); err != nil {
		t.Fatal(err)
	}

	report := Report{}
	if err = (Service{StateRoot: state, Home: t.TempDir(), Vault: vault}).checkProviders(ctx, database, &report, false); err != nil {
		t.Fatal(err)
	}
	check := findCheck(report, "provider_credential_key", credentialvault.ErrCiphertextInvalid.Error())
	want := "agentdeck credential update malformed --credential default --rotate"
	if check == nil || check.Recovery != want {
		t.Fatalf("malformed ciphertext recovery = %#v, want %q in %#v", check, want, report)
	}
}

func fileDigest(t *testing.T, path string) [32]byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(contents)
}

func hasCode(report Report, code string) bool {
	for _, check := range report.Checks {
		if check.Code == code {
			return true
		}
	}
	return false
}

func findCheck(report Report, name, code string) *Check {
	for index := range report.Checks {
		if report.Checks[index].Name == name && report.Checks[index].Code == code {
			return &report.Checks[index]
		}
	}
	return nil
}
