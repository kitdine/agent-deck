package usage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
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
			retryable = retryableTruncatedJSON(lastErr)
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

// ImportOfficialCatalog imports a local catalog envelope with its optional
// v2 rules. The catalog URL describes the document and each rule carries its
// own independently validated sources and validity bounds.
func (s *Service) ImportOfficialCatalog(ctx context.Context, data []byte, sourceURL string) error {
	source, err := url.Parse(sourceURL)
	if err != nil || source.Scheme != "https" || source.Hostname() == "" || source.User != nil {
		return errors.New("official catalog requires an absolute HTTPS provenance URL without user credentials")
	}
	c, err := parseCatalog(data)
	if err != nil {
		return err
	}
	for model, price := range c.Models {
		if _, err := time.Parse(time.RFC3339Nano, price.EffectiveFrom); err != nil {
			return fmt.Errorf("official catalog model %q effective_from: %w", model, err)
		}
		for component, amount := range price.Prices {
			switch component {
			case "input", "output", "cached_input", "cache_read", "cache_write_5m", "cache_write_1h":
			default:
				return fmt.Errorf("official catalog model %q has unknown price component %q", model, component)
			}
			if _, err := decimal(amount); err != nil {
				return fmt.Errorf("official catalog model %q component %q: %w", model, component, err)
			}
		}
	}
	return s.importParsedCatalog(ctx, c, "official", sourceURL, "", s.now(), hash(data))
}

func (s *Service) importCatalog(ctx context.Context, data []byte, kind, url, commit string, effective time.Time, contentHash string) error {
	c, err := parseCatalog(data)
	if err != nil {
		return err
	}
	return s.importParsedCatalog(ctx, c, kind, url, commit, effective, contentHash)
}

func (s *Service) importParsedCatalog(ctx context.Context, c catalog, kind, url, commit string, effective time.Time, contentHash string) error {
	if !validSHA256(contentHash) {
		return errors.New("invalid catalog content SHA-256")
	}
	tx, err := s.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `INSERT INTO price_catalogs(version,source_kind,source_url,commit_sha,content_sha256,imported_at,effective_from,currency,schema_version)
VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(version) DO NOTHING`, c.Version, kind, url, commit, contentHash, s.now().Format(time.RFC3339Nano), effective.Format(time.RFC3339Nano), c.Currency, c.SchemaVersion)
	if err != nil {
		return err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if inserted == 0 {
		var existingHash, existingKind string
		if err = tx.QueryRowContext(ctx, `SELECT content_sha256,source_kind FROM price_catalogs WHERE version=?`, c.Version).Scan(&existingHash, &existingKind); err != nil {
			return err
		}
		if existingHash != contentHash {
			if kind != "bundled" || existingKind != "bundled" {
				return fmt.Errorf("price catalog version %q already has different content", c.Version)
			}
			// Bundled versions identify semantic content. A different encoding
			// can keep the original provenance only if the stored rates match.
			matches, err := storedCatalogModelsMatch(ctx, tx, c)
			if err != nil {
				return err
			}
			if !matches {
				return fmt.Errorf("price catalog version %q already has different content", c.Version)
			}
		}
	} else {
		for name, p := range c.Models {
			prices, _ := json.Marshal(p.Prices)
			aliases, _ := json.Marshal(p.Aliases)
			if _, err = tx.ExecContext(ctx, `INSERT INTO model_prices(catalog_version,model,provider,effective_from,prices_json,aliases_json) VALUES(?,?,?,?,?,?)`, c.Version, name, p.Provider, p.EffectiveFrom, prices, aliases); err != nil {
				return err
			}
		}
	}
	if c.PricingRules != nil || inserted == 0 {
		if err := storeCatalogPriceRules(ctx, tx, c.Version, c.PricingRules); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func storedCatalogModelsMatch(ctx context.Context, tx *sql.Tx, c catalog) (bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT model,provider,effective_from,prices_json,aliases_json FROM model_prices WHERE catalog_version=?`, c.Version)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	matched := 0
	for rows.Next() {
		var model, prices, aliases string
		var stored modelPrice
		if err := rows.Scan(&model, &stored.Provider, &stored.EffectiveFrom, &prices, &aliases); err != nil {
			return false, err
		}
		if err := json.Unmarshal([]byte(prices), &stored.Prices); err != nil {
			return false, err
		}
		if err := json.Unmarshal([]byte(aliases), &stored.Aliases); err != nil {
			return false, err
		}
		want, ok := c.Models[model]
		if !ok || stored.Provider != want.Provider || stored.EffectiveFrom != want.EffectiveFrom ||
			!maps.Equal(stored.Prices, want.Prices) || !slices.Equal(stored.Aliases, want.Aliases) {
			return false, nil
		}
		matched++
	}
	return matched == len(c.Models), rows.Err()
}

func storeCatalogPriceRules(ctx context.Context, tx *sql.Tx, version string, rules *PriceRuleSet) error {
	var stored string
	err := tx.QueryRowContext(ctx, `SELECT rules_json FROM price_rule_sets WHERE catalog_version=?`, version).Scan(&stored)
	missing := errors.Is(err, sql.ErrNoRows)
	if err != nil && !missing {
		return err
	}
	if rules == nil {
		if missing {
			return nil
		}
		return fmt.Errorf("price catalog version %q already has different pricing rules", version)
	}
	encoded, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	if missing {
		// A pre-migration catalog can have legacy model rows without its rule
		// set. Keep the original import and fill the rules in this transaction.
		_, err := tx.ExecContext(ctx, `INSERT INTO price_rule_sets(catalog_version,rules_json) VALUES(?,?)`, version, string(encoded))
		return err
	}
	if stored != string(encoded) {
		existing, err := decodePriceRules([]byte(stored), true)
		if err != nil {
			return err
		}
		if existing.Digest != rules.Digest {
			return fmt.Errorf("price catalog version %q already has different pricing rules", version)
		}
	}
	return nil
}

// CatalogPriceRules returns the validated rules for a stored version. A nil
// result means this version has no stored rule set.
func (s *Service) CatalogPriceRules(ctx context.Context, version string) (*PriceRuleSet, error) {
	supported, err := s.hasPricingFactsStorage(ctx)
	if err != nil || !supported {
		return nil, err
	}
	var raw string
	err = s.Store.DB.QueryRowContext(ctx, `SELECT rules_json FROM price_rule_sets WHERE catalog_version=?`, version).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rules, err := decodePriceRules([]byte(raw), true)
	if err != nil {
		return nil, err
	}
	return &rules, nil
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
			retryable = retryableTruncatedJSON(err)
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

func retryableTruncatedJSON(err error) bool {
	var syntaxErr *json.SyntaxError
	return errors.As(err, &syntaxErr) && syntaxErr.Error() == "unexpected end of JSON input"
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
	if resp.StatusCode != http.StatusOK {
		return nil, retryablePriceStatus(resp.StatusCode), errors.New(resp.Status)
	}
	if len(data) > int(maxBytes) {
		return nil, false, fmt.Errorf("response exceeds %d bytes", maxBytes)
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
