package usage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PriceHTTPClient is an injectable adapter for the explicit price update path.
// Production applies a bounded total timeout to commit discovery and download.
var PriceHTTPClient = newPriceHTTPClient

const (
	liteLLMLatestCommitURL = "https://api.github.com/repos/BerriAI/litellm/commits/main"
	liteLLMPriceURLPrefix  = "https://raw.githubusercontent.com/BerriAI/litellm/"
	liteLLMPriceURLSuffix  = "/model_prices_and_context_window.json"
	priceHTTPTimeout       = 60 * time.Second
)

func newPriceHTTPClient() *http.Client {
	return &http.Client{Timeout: priceHTTPTimeout}
}

// ValidateLiteLLMCommit validates an optional pinned commit override.
func ValidateLiteLLMCommit(commit string) error {
	if commit != "" && !validLiteLLMCommit(commit) {
		return errors.New("price update requires a pinned LiteLLM commit SHA")
	}
	return nil
}

// UpdateLiteLLM is the sole AgentDeck network adapter. It imports only direct
// OpenAI and Anthropic records from a pinned LiteLLM document. An empty commit
// resolves the current LiteLLM main commit before downloading the snapshot.
func (s *Service) UpdateLiteLLM(ctx context.Context, commit string, client *http.Client) (map[string]any, error) {
	if err := ValidateLiteLLMCommit(commit); err != nil {
		return nil, err
	}
	if client == nil {
		client = newPriceHTTPClient()
	}
	if commit == "" {
		var err error
		commit, err = latestLiteLLMCommit(ctx, client)
		if err != nil {
			return nil, err
		}
	}
	url := liteLLMPriceURLPrefix + commit + liteLLMPriceURLSuffix
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("price update: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	contentHash := hash(data)
	c, kept, err := liteLLMCatalog(data, commit, s.now())
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	if err = s.importCatalog(ctx, encoded, "litellm", url, commit, s.now(), contentHash); err != nil {
		return nil, err
	}
	return map[string]any{"version": c.Version, "models": kept, "commit_sha": commit, "content_sha256": contentHash}, nil
}

func latestLiteLLMCommit(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, liteLLMLatestCommitURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "agentdeck-price-update")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolve latest LiteLLM commit: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve latest LiteLLM commit: %s", resp.Status)
	}
	var result struct {
		SHA string `json:"sha"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return "", fmt.Errorf("resolve latest LiteLLM commit: %w", err)
	}
	if !validLiteLLMCommit(result.SHA) {
		return "", errors.New("resolve latest LiteLLM commit: response contained an invalid SHA")
	}
	return result.SHA, nil
}
