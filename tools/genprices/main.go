// Command genprices regenerates internal/usage/model-prices.json, the price
// catalog compiled into the agentdeck binary.
//
// It downloads a pinned LiteLLM snapshot, keeps only direct openai/anthropic
// priced records, merges internal/usage/price-gapfill.json over the result,
// and stamps a content-derived catalog version. All of that logic lives in
// package usage (GenerateBundledCatalog) so it is unit-tested without network
// and cannot drift from the `agentdeck usage price update` path; this command
// is only the network and file shim.
//
// The generated file is a reviewed build artifact, never hand-edited:
//
//	make prices-regen LITELLM_COMMIT=<sha>   # pinned, reproducible
//	make prices-regen                        # resolves current litellm main, then pins it
//
// Release cadence and ownership are documented in docs/specs/cli-design.md.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/kitdine/agent-deck/internal/usage"
)

const (
	liteLLMPriceURL  = "https://raw.githubusercontent.com/BerriAI/litellm/%s/model_prices_and_context_window.json"
	liteLLMCommitURL = "https://api.github.com/repos/BerriAI/litellm/commits/main"
	maxCatalogBytes  = 32 << 20
	httpTimeout      = 60 * time.Second
)

func main() {
	commit := flag.String("commit", "", "LiteLLM commit SHA to pin (default: resolve current main)")
	out := flag.String("out", filepath.Join("internal", "usage", "model-prices.json"), "output path for the generated bundled catalog")
	gapfillPath := flag.String("gapfill", filepath.Join("internal", "usage", "price-gapfill.json"), "curated gap-fill input merged over the generated catalog")
	check := flag.Bool("check", false, "verify the committed catalog is byte-identical to a fresh generation instead of writing it")
	flag.Parse()

	client := &http.Client{Timeout: httpTimeout}
	if err := run(*commit, *out, *gapfillPath, *check, httpFetcher(client)); err != nil {
		fmt.Fprintln(os.Stderr, "genprices:", err)
		os.Exit(1)
	}
}

// fetcher is the single network seam. Injecting it lets the check-mode
// orchestration — pin discovery, which URLs are requested, and the byte
// comparison — be tested without network, which is the only way to protect
// that wiring rather than just the helpers it calls.
type fetcher func(ctx context.Context, url string, headers map[string]string) ([]byte, error)

func httpFetcher(client *http.Client) fetcher {
	return func(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
		return fetch(ctx, client, url, headers)
	}
}

func run(commit, out, gapfillPath string, check bool, get fetcher) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	gapfillData, err := os.ReadFile(gapfillPath)
	if err != nil {
		return err
	}
	gapfill, err := usage.ParseGapfill(gapfillData)
	if err != nil {
		return fmt.Errorf("%s: %w", gapfillPath, err)
	}

	if commit == "" {
		if check {
			// Check mode must reproduce the committed artifact, so it pins to
			// the commit that artifact records rather than to current main,
			// which would compare against a different input entirely.
			existing, readErr := os.ReadFile(out)
			if readErr != nil {
				return readErr
			}
			if commit, err = usage.RecordedLiteLLMCommit(existing); err != nil {
				return fmt.Errorf("%s: %w", out, err)
			}
			fmt.Fprintf(os.Stderr, "genprices: checking against the commit %s records: %s\n", out, commit)
		} else {
			if commit, err = latestCommit(ctx, get); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "genprices: pinned current LiteLLM main to %s\n", commit)
		}
	}

	snapshot, err := get(ctx, fmt.Sprintf(liteLLMPriceURL, commit), nil)
	if err != nil {
		return err
	}

	generated, err := usage.GenerateBundledCatalog(snapshot, commit, gapfill)
	if err != nil {
		return err
	}

	if check {
		existing, readErr := os.ReadFile(out)
		if readErr != nil {
			return readErr
		}
		if string(existing) != string(generated) {
			return fmt.Errorf("%s is not what regeneration from LiteLLM %s produces; run 'make prices-regen LITELLM_COMMIT=%s'", out, commit, commit)
		}
		fmt.Fprintf(os.Stderr, "genprices: %s matches regeneration from LiteLLM %s\n", out, commit)
		return nil
	}

	if err = os.WriteFile(out, generated, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "genprices: wrote %s from LiteLLM %s with %d curated entr(ies) merged\n", out, commit, len(gapfill.Entries))
	return nil
}

func latestCommit(ctx context.Context, get fetcher) (string, error) {
	data, err := get(ctx, liteLLMCommitURL, map[string]string{
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
		"User-Agent":           "agentdeck-genprices",
	})
	if err != nil {
		return "", err
	}
	var result struct {
		SHA string `json:"sha"`
	}
	if err = json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	if result.SHA == "" {
		return "", errors.New("resolve latest LiteLLM commit: empty SHA in response")
	}
	return result.SHA, nil
}

func fetch(ctx context.Context, client *http.Client, url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxCatalogBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxCatalogBytes {
		return nil, fmt.Errorf("GET %s: response exceeds %d bytes", url, maxCatalogBytes)
	}
	return data, nil
}
