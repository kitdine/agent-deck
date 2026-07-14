package usage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// PriceHTTPClient is an injectable adapter for the explicit price update path.
// Production returns nil so UpdateLiteLLM selects http.DefaultClient.
var PriceHTTPClient = func() *http.Client { return nil }

// UpdateLiteLLM is the sole AgentDeck network adapter. It imports only direct
// OpenAI and Anthropic records from the pinned LiteLLM document.
func (s *Service) UpdateLiteLLM(ctx context.Context, url, commit string, client *http.Client) (map[string]any, error) {
	if !validLiteLLMCommit(commit) {
		return nil, errors.New("price update requires a pinned LiteLLM commit SHA")
	}
	expectedURL := "https://raw.githubusercontent.com/BerriAI/litellm/" + commit + "/model_prices_and_context_window.json"
	if url != expectedURL {
		return nil, fmt.Errorf("price update URL must be the canonical pinned LiteLLM URL: %s", expectedURL)
	}
	if client == nil {
		client = http.DefaultClient
	}
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
	c, kept, err := liteLLMCatalog(data, commit, s.now())
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	if err = s.importCatalog(ctx, encoded, "litellm", url, commit, s.now()); err != nil {
		return nil, err
	}
	return map[string]any{"version": c.Version, "models": kept, "commit_sha": commit, "content_sha256": hash(data)}, nil
}
