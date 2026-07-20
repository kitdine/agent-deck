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
	priceHTTPAttempts      = 3
	priceCommitMaxBytes    = 1 << 20
	priceCatalogMaxBytes   = 32 << 20
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
	var data []byte
	var c catalog
	var kept int
	var lastErr error
	for attempt := 1; attempt <= priceHTTPAttempts; attempt++ {
		var retryable bool
		data, retryable, lastErr = fetchPriceBody(ctx, client, url, nil, priceCatalogMaxBytes)
		if lastErr == nil {
			c, kept, lastErr = liteLLMCatalog(data, commit, s.now())
			// A truncated 200 response can be readable at the HTTP layer but invalid
			// JSON. Retrying the immutable URL is safe and leaves state untouched.
			retryable = lastErr != nil
		}
		if lastErr == nil {
			break
		}
		if !retryable || attempt == priceHTTPAttempts {
			return nil, fmt.Errorf("price update: %w", lastErr)
		}
		if err := waitPriceRetry(ctx, attempt); err != nil {
			return nil, fmt.Errorf("price update: %w", err)
		}
	}
	contentHash := hash(data)
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
	headers := map[string]string{
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
		"User-Agent":           "agentdeck-price-update",
	}
	var lastErr error
	for attempt := 1; attempt <= priceHTTPAttempts; attempt++ {
		data, retryable, err := fetchPriceBody(ctx, client, liteLLMLatestCommitURL, headers, priceCommitMaxBytes)
		if err == nil {
			var result struct {
				SHA string `json:"sha"`
			}
			if err = json.Unmarshal(data, &result); err == nil {
				if !validLiteLLMCommit(result.SHA) {
					return "", errors.New("resolve latest LiteLLM commit: response contained an invalid SHA")
				}
				return result.SHA, nil
			}
			retryable = true
		}
		lastErr = err
		if !retryable || attempt == priceHTTPAttempts {
			return "", fmt.Errorf("resolve latest LiteLLM commit: %w", lastErr)
		}
		if err = waitPriceRetry(ctx, attempt); err != nil {
			return "", fmt.Errorf("resolve latest LiteLLM commit: %w", err)
		}
	}
	return "", fmt.Errorf("resolve latest LiteLLM commit: %w", lastErr)
}

func fetchPriceBody(ctx context.Context, client *http.Client, url string, headers map[string]string, maxBytes int64) ([]byte, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, ctx.Err() == nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	closeErr := resp.Body.Close()
	if readErr != nil {
		return nil, true, readErr
	}
	if closeErr != nil {
		return nil, true, closeErr
	}
	if len(data) > int(maxBytes) {
		return nil, false, fmt.Errorf("response exceeds %d bytes", maxBytes)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, retryablePriceStatus(resp.StatusCode), errors.New(resp.Status)
	}
	return data, false, nil
}

func retryablePriceStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func waitPriceRetry(ctx context.Context, attempt int) error {
	timer := time.NewTimer(time.Duration(attempt) * 100 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
