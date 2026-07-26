package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kitdine/agent-deck/internal/usage"
)

const (
	recordedCommit = "f2479cc704f6e63d5510929d30ce8e11ffe43467"
	otherCommit    = "0123456789abcdef0123456789abcdef01234567"
)

// syntheticSnapshot is a minimal upstream document with one row per direct
// vendor, enough for GenerateBundledCatalog to produce a real artifact.
const syntheticSnapshot = `{
  "gpt-test-openai": {
    "litellm_provider": "openai",
    "input_cost_per_token": 0.0000025,
    "output_cost_per_token": 0.000015,
    "cache_read_input_token_cost": 0.00000025
  },
  "claude-test-anthropic": {
    "litellm_provider": "anthropic",
    "input_cost_per_token": 0.000003,
    "output_cost_per_token": 0.000015,
    "cache_read_input_token_cost": 0.0000003,
    "cache_creation_input_token_cost": 0.00000375,
    "cache_creation_input_token_cost_above_1hr": 0.000006
  }
}`

const emptyGapfill = `{"schema_version":1,"entries":[],"pending":[]}`

const unpublishedArtifact = "existing artifact must remain byte-for-byte unchanged\n"

// recordingFetcher stands in for the network and records every URL requested,
// so a test can prove which endpoints check mode does and does not touch.
type recordingFetcher struct {
	requested []string
	snapshots map[string]string // commit -> body
	failAll   bool
}

func (r *recordingFetcher) fetch(_ context.Context, url string, _ map[string]string) ([]byte, error) {
	r.requested = append(r.requested, url)
	if r.failAll {
		return nil, fmt.Errorf("network must not be reached in this test, but %s was requested", url)
	}
	if url == liteLLMCommitURL {
		return []byte(`{"sha":"` + otherCommit + `"}`), nil
	}
	for commit, body := range r.snapshots {
		if url == fmt.Sprintf(liteLLMPriceURL, commit) {
			return []byte(body), nil
		}
	}
	return nil, fmt.Errorf("unexpected URL %s", url)
}

func (r *recordingFetcher) requestedAny(substr string) bool {
	for _, url := range r.requested {
		if strings.Contains(url, substr) {
			return true
		}
	}
	return false
}

// writeFixture lays out a temporary artifact + gap-fill pair. When artifact is
// empty the artifact is generated from the synthetic snapshot pinned to
// recordedCommit, i.e. exactly what a correct regeneration would produce.
func writeFixture(t *testing.T, artifact string) (outPath, gapfillPath string) {
	t.Helper()
	dir := t.TempDir()
	outPath = filepath.Join(dir, "model-prices.json")
	gapfillPath = filepath.Join(dir, "price-gapfill.json")

	if artifact == "" {
		gapfill, err := usage.ParseGapfill([]byte(emptyGapfill))
		if err != nil {
			t.Fatal(err)
		}
		generated, err := usage.GenerateBundledCatalog([]byte(syntheticSnapshot), recordedCommit, gapfill)
		if err != nil {
			t.Fatal(err)
		}
		artifact = string(generated)
	}
	if err := os.WriteFile(outPath, []byte(artifact), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gapfillPath, []byte(emptyGapfill), 0o644); err != nil {
		t.Fatal(err)
	}
	return outPath, gapfillPath
}

func writeFailureFixture(t *testing.T) (outPath, gapfillPath string) {
	t.Helper()
	return writeFixture(t, unpublishedArtifact)
}

func requireArtifactUnchanged(t *testing.T, outPath string) {
	t.Helper()
	requireArtifactBytes(t, outPath, []byte(unpublishedArtifact))
}

func readArtifact(t *testing.T, outPath string) []byte {
	t.Helper()
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func requireArtifactBytes(t *testing.T, outPath string, want []byte) {
	t.Helper()
	got := readArtifact(t, outPath)
	if string(got) != string(want) {
		t.Fatalf("failed generation changed output to %q, want original %q", got, want)
	}
}

type generatorRoundTrip func(*http.Request) (*http.Response, error)

func (f generatorRoundTrip) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type repeatedResponseReader struct {
	remaining int64
	value     byte
}

func (r *repeatedResponseReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	value := r.value
	if value == 0 {
		value = 'x'
	}
	for i := range p {
		p[i] = value
	}
	r.remaining -= int64(len(p))
	return len(p), nil
}

func TestRunAcceptsCatalogAtExactHTTPSizeLimit(t *testing.T) {
	outPath, gapfillPath := writeFailureFixture(t)
	padding := int64(maxCatalogBytes - len(syntheticSnapshot))
	if padding < 0 {
		t.Fatalf("synthetic catalog is %d bytes, exceeds limit %d", len(syntheticSnapshot), maxCatalogBytes)
	}
	body := io.MultiReader(
		strings.NewReader(syntheticSnapshot),
		&repeatedResponseReader{remaining: padding, value: ' '},
	)
	client := &http.Client{Transport: generatorRoundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(body), Header: make(http.Header)}, nil
	})}

	if err := run(recordedCommit, outPath, gapfillPath, false, httpFetcher(client)); err != nil {
		t.Fatalf("run rejected an exact-size catalog: %v", err)
	}
	written, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := usage.RecordedLiteLLMCommit(written)
	if err != nil || pinned != recordedCommit {
		t.Fatalf("generated artifact commit = %q, %v; want %q, nil", pinned, err, recordedCommit)
	}
	if !strings.Contains(string(written), `"gpt-test-openai"`) || string(written) == unpublishedArtifact {
		t.Fatalf("exact-size catalog did not publish the generated models")
	}
}

func TestRunDoesNotPublishOnHTTPFetchFailures(t *testing.T) {
	priceURL := fmt.Sprintf(liteLLMPriceURL, recordedCommit)
	networkErr := errors.New("synthetic network failure")
	for _, test := range []struct {
		name      string
		transport generatorRoundTrip
		wantErr   string
		isErr     error
	}{
		{
			name: "network",
			transport: func(*http.Request) (*http.Response, error) {
				return nil, networkErr
			},
			isErr: networkErr,
		},
		{
			name: "non-2xx",
			transport: func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusBadGateway, Status: "502 Bad Gateway", Body: io.NopCloser(strings.NewReader("unavailable")), Header: make(http.Header)}, nil
			},
			wantErr: "GET " + priceURL + ": 502 Bad Gateway",
		},
		{
			name: "oversized response",
			transport: func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(&repeatedResponseReader{remaining: maxCatalogBytes + 1}), Header: make(http.Header)}, nil
			},
			wantErr: fmt.Sprintf("GET %s: response exceeds %d bytes", priceURL, maxCatalogBytes),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			outPath, gapfillPath := writeFailureFixture(t)
			client := &http.Client{Transport: test.transport}
			err := run(recordedCommit, outPath, gapfillPath, false, httpFetcher(client))
			if test.isErr != nil {
				if !errors.Is(err, test.isErr) {
					t.Fatalf("run error = %v, want errors.Is(%v)", err, test.isErr)
				}
			} else if err == nil || err.Error() != test.wantErr {
				t.Fatalf("run error = %v, want %q", err, test.wantErr)
			}
			requireArtifactUnchanged(t, outPath)
		})
	}
}

func TestRunRejectsInvalidLatestCommitBeforeCatalogFetch(t *testing.T) {
	for _, test := range []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "malformed response", body: "{", wantErr: "resolve latest LiteLLM commit: unexpected end of JSON input"},
		{name: "empty sha", body: `{"sha":""}`, wantErr: "resolve latest LiteLLM commit: empty SHA in response"},
		{name: "non-sha revision", body: `{"sha":"main"}`, wantErr: "resolve latest LiteLLM commit: response contained an invalid SHA"},
	} {
		t.Run(test.name, func(t *testing.T) {
			outPath, gapfillPath := writeFailureFixture(t)
			var requested []string
			get := func(_ context.Context, url string, _ map[string]string) ([]byte, error) {
				requested = append(requested, url)
				if url == liteLLMCommitURL {
					return []byte(test.body), nil
				}
				return []byte(syntheticSnapshot), nil
			}

			err := run("", outPath, gapfillPath, false, get)
			if err == nil || err.Error() != test.wantErr {
				t.Fatalf("run error = %v, want %q", err, test.wantErr)
			}
			requireArtifactUnchanged(t, outPath)
			if len(requested) != 1 || requested[0] != liteLLMCommitURL {
				t.Fatalf("invalid commit response requested %v, want only %s", requested, liteLLMCommitURL)
			}
		})
	}
}

func TestRunWrapsLatestCommitTransportErrorBeforeCatalogFetch(t *testing.T) {
	outPath, gapfillPath := writeFailureFixture(t)
	transportErr := errors.New("synthetic latest commit transport failure")
	var requested []string
	get := func(_ context.Context, url string, _ map[string]string) ([]byte, error) {
		requested = append(requested, url)
		return nil, transportErr
	}

	err := run("", outPath, gapfillPath, false, get)
	const wantPrefix = "resolve latest LiteLLM commit: "
	wantErr := wantPrefix + transportErr.Error()
	if err == nil || err.Error() != wantErr || !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Fatalf("run error = %v, want exact wrapped error %q", err, wantErr)
	}
	if !errors.Is(err, transportErr) {
		t.Fatalf("run error = %v, want errors.Is(%v)", err, transportErr)
	}
	if len(requested) != 1 || requested[0] != liteLLMCommitURL {
		t.Fatalf("latest-commit transport error requested %v, want only %s", requested, liteLLMCommitURL)
	}
	requireArtifactUnchanged(t, outPath)
}

func TestRunRejectsExplicitInvalidCommitBeforeCatalogFetch(t *testing.T) {
	outPath, gapfillPath := writeFailureFixture(t)
	var requested []string
	get := func(_ context.Context, url string, _ map[string]string) ([]byte, error) {
		requested = append(requested, url)
		return []byte(syntheticSnapshot), nil
	}

	err := run("main", outPath, gapfillPath, false, get)
	const wantErr = "price update requires a pinned LiteLLM commit SHA"
	if err == nil || err.Error() != wantErr {
		t.Fatalf("run error = %v, want %q", err, wantErr)
	}
	if len(requested) != 0 {
		t.Fatalf("explicit invalid commit made %d request(s), want zero: %v", len(requested), requested)
	}
	requireArtifactUnchanged(t, outPath)
}

func TestRunDoesNotPublishInvalidCatalogInputs(t *testing.T) {
	for _, test := range []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "malformed JSON", body: "{", wantErr: "unexpected end of JSON input"},
		{name: "no direct-provider records", body: `{"bedrock":{"litellm_provider":"bedrock","input_cost_per_token":1,"output_cost_per_token":1}}`, wantErr: "LiteLLM catalog contains no validated direct-provider records"},
	} {
		t.Run(test.name, func(t *testing.T) {
			outPath, gapfillPath := writeFailureFixture(t)
			requested := 0
			get := func(_ context.Context, url string, _ map[string]string) ([]byte, error) {
				requested++
				wantURL := fmt.Sprintf(liteLLMPriceURL, recordedCommit)
				if url != wantURL {
					t.Fatalf("catalog request = %s, want %s", url, wantURL)
				}
				return []byte(test.body), nil
			}

			err := run(recordedCommit, outPath, gapfillPath, false, get)
			if err == nil || err.Error() != test.wantErr {
				t.Fatalf("run error = %v, want %q", err, test.wantErr)
			}
			if requested != 1 {
				t.Fatalf("catalog requests = %d, want 1", requested)
			}
			requireArtifactUnchanged(t, outPath)
		})
	}
}

// TestCheckModePinsToTheArtifactsRecordedCommit is the wiring test Round 1
// asked for and Round 2 found still missing: it drives the real run() in check
// mode, with no commit supplied, and proves the pin comes from the artifact
// rather than from current main.
func TestCheckModePinsToTheArtifactsRecordedCommit(t *testing.T) {
	outPath, gapfillPath := writeFixture(t, "")
	get := &recordingFetcher{snapshots: map[string]string{recordedCommit: syntheticSnapshot}}

	if err := run("", outPath, gapfillPath, true, get.fetch); err != nil {
		t.Fatalf("check mode failed on an artifact that matches its inputs: %v", err)
	}

	if get.requestedAny("api.github.com") {
		t.Errorf("check mode resolved latest main; it must pin to the artifact's own commit. requested: %v", get.requested)
	}
	wantURL := fmt.Sprintf(liteLLMPriceURL, recordedCommit)
	if len(get.requested) != 1 || get.requested[0] != wantURL {
		t.Fatalf("check mode requested %v, want exactly one fetch of %s", get.requested, wantURL)
	}
	if strings.Contains(get.requested[0], otherCommit) {
		t.Errorf("check mode fetched the latest-main commit instead of the recorded pin")
	}
}

// TestCheckModeFailsWhenArtifactDiffersFromRegeneration proves the byte
// comparison is real: a hand-edited artifact must not pass.
func TestCheckModeFailsWhenArtifactDiffersFromRegeneration(t *testing.T) {
	outPath, gapfillPath := writeFixture(t, "")

	// Hand-edit one price, leaving the recorded pin and version untouched --
	// the exact "silently hand-edited artifact" this check exists to catch.
	existing, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err = json.Unmarshal(existing, &document); err != nil {
		t.Fatal(err)
	}
	models := document["models"].(map[string]any)
	model := models["gpt-test-openai"].(map[string]any)
	model["prices_per_million"].(map[string]any)["input"] = "999.000000000"
	tampered, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(outPath, append(tampered, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	before := readArtifact(t, outPath)

	get := &recordingFetcher{snapshots: map[string]string{recordedCommit: syntheticSnapshot}}
	err = run("", outPath, gapfillPath, true, get.fetch)
	if err == nil {
		t.Fatal("check mode passed a hand-edited artifact")
	}
	if !strings.Contains(err.Error(), "is not what regeneration from LiteLLM") {
		t.Fatalf("wrong failure for a tampered artifact: %v", err)
	}
	if !strings.Contains(err.Error(), "LITELLM_COMMIT="+recordedCommit) {
		t.Errorf("failure should name the exact regeneration command with the recorded pin: %v", err)
	}
	requireArtifactBytes(t, outPath, before)
}

// TestCheckModeFailsBeforeAnyNetworkWhenPinIsUnusable pins the ordering: a
// missing or corrupt pin must fail before a single request is made, so a
// broken artifact cannot be silently checked against some other snapshot.
func TestCheckModeFailsBeforeAnyNetworkWhenPinIsUnusable(t *testing.T) {
	for _, test := range []struct{ name, artifact, wantErr string }{
		{"malformed json", "{not json", "read recorded LiteLLM commit"},
		{"no generated_from", `{"schema_version":1,"catalog_version":"x","currency":"USD","models":{}}`, "records no litellm entry"},
		{"litellm origin without a sha", `{"generated_from":[{"kind":"litellm","commit_sha":""}]}`, "invalid pinned LiteLLM commit"},
		{"unpinned to a branch name", `{"generated_from":[{"kind":"litellm","commit_sha":"main"}]}`, "invalid pinned LiteLLM commit"},
		{"ambiguous provenance", `{"generated_from":[{"kind":"litellm","commit_sha":"` + recordedCommit + `"},{"kind":"litellm","commit_sha":"` + otherCommit + `"}]}`, "ambiguous"},
	} {
		t.Run(test.name, func(t *testing.T) {
			outPath, gapfillPath := writeFixture(t, test.artifact)
			before := readArtifact(t, outPath)
			// failAll turns any request at all into a test failure.
			get := &recordingFetcher{failAll: true}

			err := run("", outPath, gapfillPath, true, get.fetch)
			if err == nil {
				t.Fatal("check mode accepted an artifact with an unusable pin")
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("wrong error: %v (want substring %q)", err, test.wantErr)
			}
			if len(get.requested) != 0 {
				t.Fatalf("check mode made %d request(s) before validating the pin: %v", len(get.requested), get.requested)
			}
			requireArtifactBytes(t, outPath, before)
		})
	}
}

// TestCheckModeHonorsAnExplicitCommit confirms an explicitly supplied pin
// still wins and is the one fetched.
func TestCheckModeHonorsAnExplicitCommit(t *testing.T) {
	outPath, gapfillPath := writeFixture(t, "")
	before := readArtifact(t, outPath)
	get := &recordingFetcher{snapshots: map[string]string{otherCommit: syntheticSnapshot}}

	// The artifact records recordedCommit, so checking it against a different
	// pin must fetch that other pin -- and here the regenerated bytes differ
	// (different commit in generated_from), so it must fail rather than pass.
	err := run(otherCommit, outPath, gapfillPath, true, get.fetch)
	if err == nil {
		t.Fatal("checking against a different commit passed; generated_from should differ")
	}
	if !get.requestedAny(otherCommit) {
		t.Fatalf("explicit commit was not fetched: %v", get.requested)
	}
	if get.requestedAny("api.github.com") {
		t.Errorf("an explicit commit must not trigger latest-main resolution: %v", get.requested)
	}
	requireArtifactBytes(t, outPath, before)
}

// TestWriteModeResolvesLatestMainWhenUnpinned is the counterpart: outside
// check mode an omitted commit does resolve current main, so the check-mode
// behavior above is a deliberate difference rather than a broken resolver.
func TestWriteModeResolvesLatestMainWhenUnpinned(t *testing.T) {
	outPath, gapfillPath := writeFixture(t, "")
	get := &recordingFetcher{snapshots: map[string]string{otherCommit: syntheticSnapshot}}

	if err := run("", outPath, gapfillPath, false, get.fetch); err != nil {
		t.Fatalf("write mode failed: %v", err)
	}
	if !get.requestedAny("api.github.com") {
		t.Fatalf("write mode did not resolve latest main: %v", get.requested)
	}
	written, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := usage.RecordedLiteLLMCommit(written)
	if err != nil {
		t.Fatal(err)
	}
	if pinned != otherCommit {
		t.Fatalf("written artifact records %q, want the resolved %q", pinned, otherCommit)
	}
}

// TestCheckModeRejectsAGapfillThatFailsTheCurationBar proves the tool refuses
// to regenerate from a curated input that would not meet the bar, before it
// reaches the network.
func TestCheckModeRejectsAGapfillThatFailsTheCurationBar(t *testing.T) {
	outPath, gapfillPath := writeFixture(t, "")
	before := readArtifact(t, outPath)
	bad := `{"schema_version":1,"entries":[{"model":"m","provider":"openai","effective_from":"2026-02-12T00:00:00Z","source_url":"https://models.dev/m","verified_by":"x","verified_on":"2026-07-23","prices_per_million":{"input":"1"}}]}`
	if err := os.WriteFile(gapfillPath, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	get := &recordingFetcher{failAll: true}

	err := run("", outPath, gapfillPath, true, get.fetch)
	if err == nil || !strings.Contains(err.Error(), "not a openai vendor rate card") {
		t.Fatalf("aggregator-sourced gap-fill accepted: %v", err)
	}
	if len(get.requested) != 0 {
		t.Fatalf("network reached before validating the gap-fill: %v", get.requested)
	}
	requireArtifactBytes(t, outPath, before)
}
