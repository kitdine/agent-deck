// Package usage imports read-only client usage logs and calculates catalog costs.
package usage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kitdine/agent-deck/internal/activity"
	"github.com/kitdine/agent-deck/internal/store"
)

//go:embed model-prices.json
var bundledCatalog []byte

const (
	bundledCatalogSourceURL       = "bundled://agentdeck/model-prices.json"
	legacyBundledCatalogSourceURL = "bundled://config/model-prices.json"
	usageParserVersion            = 3
)

var tokenNames = []string{"input_tokens", "cached_input_tokens", "output_tokens", "cache_read_tokens", "cache_creation_tokens", "cache_write_5m_tokens", "cache_write_1h_tokens"}

var errUsageSourceChanged = errors.New("usage source changed during inventory scan")

type Event struct {
	Key, Client, SessionID, EventID, EventAt, Model, SourcePath string
	SourceOffset                                                int64
	Tokens                                                      map[string]int64
}
type Result struct {
	Tokens               map[string]int64 `json:"tokens"`
	CatalogBaseCost      *string          `json:"catalog_base_cost"`
	ProviderCost         *string          `json:"provider_cost"`
	KnownCatalogBaseCost string           `json:"known_catalog_base_cost"`
	KnownProviderCost    string           `json:"known_provider_cost"`
	Unpriced             []string         `json:"unpriced_components"`
}
type Summary struct {
	Tokens               map[string]int64 `json:"tokens"`
	Counts               map[string]int64 `json:"counts"`
	CatalogBaseCost      *string          `json:"catalog_base_cost"`
	ProviderCost         *string          `json:"provider_cost"`
	KnownCatalogBaseCost *string          `json:"known_catalog_base_cost"`
	KnownProviderCost    *string          `json:"known_provider_cost"`
	Models               []ModelCoverage  `json:"model_coverage"`
	Unpriced             []string         `json:"unpriced_components"`
	Warnings             []string         `json:"warnings"`
}

type ModelCoverage struct {
	Client         string `json:"client"`
	Model          string `json:"model"`
	Events         int64  `json:"events"`
	PricedEvents   int64  `json:"priced_events"`
	UnpricedEvents int64  `json:"unpriced_events"`
}

type SessionSummary struct {
	Client               string           `json:"client"`
	SessionID            string           `json:"session_id"`
	FirstAt              string           `json:"first_at"`
	LastAt               string           `json:"last_at"`
	Tokens               map[string]int64 `json:"tokens"`
	CatalogBaseCost      *string          `json:"catalog_base_cost"`
	ProviderCost         *string          `json:"provider_cost"`
	KnownCatalogBaseCost *string          `json:"known_catalog_base_cost"`
	KnownProviderCost    *string          `json:"known_provider_cost"`
	Unpriced             []string         `json:"unpriced_components"`
	Warnings             []string         `json:"warnings"`
}

type StatsOptions struct {
	From     time.Time
	To       time.Time
	GroupBy  string
	Metric   string
	Client   string
	Model    string
	Provider string
	Timezone string
	Location *time.Location
	Activity bool
}

type StatsRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type StatsTotals struct {
	Tokens               int64   `json:"tokens"`
	InputTokens          int64   `json:"input_tokens"`
	OutputTokens         int64   `json:"output_tokens"`
	CachedReadTokens     int64   `json:"cached_read_tokens"`
	CacheWriteTokens     int64   `json:"cache_write_tokens"`
	Sessions             int64   `json:"sessions"`
	Events               int64   `json:"events"`
	CatalogBaseCost      *string `json:"catalog_base_cost"`
	ProviderCost         *string `json:"provider_cost"`
	KnownCatalogBaseCost string  `json:"known_catalog_base_cost"`
	KnownProviderCost    string  `json:"known_provider_cost"`
	AverageTokens        string  `json:"average_tokens_per_session"`
	AverageCost          *string `json:"average_cost_per_session"`
	KnownAverageCost     string  `json:"known_average_cost_per_session"`
}

type StatsBucket struct {
	Start             string  `json:"start"`
	End               string  `json:"end"`
	Tokens            int64   `json:"tokens"`
	InputTokens       int64   `json:"input_tokens"`
	OutputTokens      int64   `json:"output_tokens"`
	CachedReadTokens  int64   `json:"cached_read_tokens"`
	CacheWriteTokens  int64   `json:"cache_write_tokens"`
	Sessions          int64   `json:"sessions"`
	Events            int64   `json:"events"`
	ProviderCost      *string `json:"provider_cost"`
	KnownProviderCost string  `json:"known_provider_cost"`
	MetricValue       *string `json:"metric_value"`
	KnownMetricValue  string  `json:"known_metric_value"`
	Coverage          string  `json:"coverage"`
}

type StatsDimension struct {
	Name               string              `json:"name"`
	Client             string              `json:"client,omitempty"`
	Tokens             int64               `json:"tokens"`
	InputTokens        int64               `json:"input_tokens"`
	OutputTokens       int64               `json:"output_tokens"`
	CachedReadTokens   int64               `json:"cached_read_tokens"`
	CacheWriteTokens   int64               `json:"cache_write_tokens"`
	LogicalInputTokens int64               `json:"logical_input_tokens"`
	CacheHitRate       *string             `json:"cache_hit_rate"`
	Sessions           int64               `json:"sessions"`
	Events             int64               `json:"events"`
	ProviderCost       *string             `json:"provider_cost"`
	KnownProviderCost  string              `json:"known_provider_cost"`
	MetricValue        *string             `json:"metric_value"`
	KnownMetricValue   string              `json:"known_metric_value"`
	Share              *string             `json:"share"`
	KnownShare         string              `json:"known_share"`
	Coverage           string              `json:"coverage"`
	Activity           *StatsModelActivity `json:"activity,omitempty"`
}

type StatsToolCount struct {
	Name  string `json:"name"`
	Calls int64  `json:"calls"`
}

type StatsModelActivity struct {
	ActiveSessions  int64            `json:"active_sessions"`
	ActiveDays      int64            `json:"active_days"`
	FirstAt         string           `json:"first_at,omitempty"`
	LastAt          string           `json:"last_at,omitempty"`
	ToolCalls       int64            `json:"tool_calls"`
	CompletedCalls  int64            `json:"completed_calls"`
	FailedCalls     int64            `json:"failed_calls"`
	TotalDurationMS int64            `json:"total_duration_ms"`
	AverageDuration *int64           `json:"average_duration_ms"`
	Tools           []StatsToolCount `json:"tools"`
}

type StatsCacheSession struct {
	Client             string   `json:"client"`
	SessionID          string   `json:"session_id"`
	Models             []string `json:"models"`
	InputTokens        int64    `json:"input_tokens"`
	OutputTokens       int64    `json:"output_tokens"`
	CachedReadTokens   int64    `json:"cached_read_tokens"`
	CacheWriteTokens   int64    `json:"cache_write_tokens"`
	LogicalInputTokens int64    `json:"logical_input_tokens"`
	CacheHitRate       *string  `json:"cache_hit_rate"`
	Events             int64    `json:"events"`
	FirstAt            string   `json:"first_at"`
	LastAt             string   `json:"last_at"`
	DetailCommand      string   `json:"detail_command"`
}

type StatsActivity struct {
	Weekday          int     `json:"weekday"`
	Hour             int     `json:"hour"`
	Tokens           int64   `json:"tokens"`
	Sessions         int64   `json:"sessions"`
	Events           int64   `json:"events"`
	KnownCost        string  `json:"known_provider_cost"`
	MetricValue      *string `json:"metric_value"`
	KnownMetricValue string  `json:"known_metric_value"`
}

type StatsPeak struct {
	Start      string  `json:"start"`
	End        string  `json:"end"`
	Value      *string `json:"value"`
	KnownValue string  `json:"known_value"`
	Coverage   string  `json:"coverage"`
}

type StatsCoverage struct {
	PricedEvents   int64  `json:"priced_events"`
	UnpricedEvents int64  `json:"unpriced_events"`
	TotalEvents    int64  `json:"total_events"`
	Percent        string `json:"percent"`
}

type StatsReport struct {
	Range             StatsRange           `json:"range"`
	Timezone          string               `json:"timezone"`
	GroupBy           string               `json:"group_by"`
	Metric            string               `json:"metric"`
	Totals            StatsTotals          `json:"totals"`
	Buckets           []StatsBucket        `json:"buckets"`
	Models            []StatsDimension     `json:"models"`
	Clients           []StatsDimension     `json:"clients"`
	Providers         []StatsDimension     `json:"providers"`
	CacheSessions     []StatsCacheSession  `json:"cache_sessions"`
	Activity          []StatsActivity      `json:"activity"`
	Peak              StatsPeak            `json:"peak"`
	Coverage          StatsCoverage        `json:"coverage"`
	UnpricedModels    []StatsUnpricedModel `json:"unpriced_models"`
	ShowModelActivity bool                 `json:"-"`
}

type StatsUnpricedModel struct {
	Client     string   `json:"client"`
	Model      string   `json:"model"`
	Components []string `json:"missing_components"`
}
type SourceFile interface {
	io.Reader
	io.ReaderAt
	io.Seeker
	io.Closer
}

type InventoryEntry struct {
	Path       string `json:"path"`
	Client     string `json:"client"`
	Identity   string `json:"identity"`
	Size       int64  `json:"size"`
	ModifiedAt int64  `json:"modified_at"`
}

type Inventory struct {
	Fingerprint string           `json:"fingerprint"`
	Entries     []InventoryEntry `json:"entries"`
	Added       []string         `json:"added"`
	Appended    []string         `json:"appended"`
	Mutated     []string         `json:"mutated"`
	Removed     []string         `json:"removed"`
}
type Service struct {
	Store *store.Store
	Home  string
	Now   func() time.Time
	// ClientProcesses is injectable so overlap handling has deterministic tests.
	ClientProcesses func(string) ([]int, error)
	Stat            func(string) (os.FileInfo, error)
	Open            func(string) (SourceFile, error)
}
type catalog struct {
	SchemaVersion int                   `json:"schema_version"`
	Version       string                `json:"catalog_version"`
	Currency      string                `json:"currency"`
	Models        map[string]modelPrice `json:"models"`
}
type modelPrice struct {
	Provider      string            `json:"provider"`
	Aliases       []string          `json:"aliases"`
	EffectiveFrom string            `json:"effective_from"`
	Prices        map[string]string `json:"prices_per_million"`
}

// OfficialOverride is a vendor-published component correction.  Its provenance
// and effective time are mandatory so it cannot masquerade as invoice data.
type OfficialOverride struct {
	Model         string            `json:"model"`
	Provider      string            `json:"provider"`
	SourceURL     string            `json:"source_url"`
	EffectiveFrom time.Time         `json:"effective_from"`
	Prices        map[string]string `json:"prices"`
}

type PriceCatalog struct {
	Version       string `json:"version"`
	SourceKind    string `json:"source_kind"`
	SourceURL     string `json:"source_url"`
	CommitSHA     string `json:"commit_sha"`
	ContentSHA256 string `json:"content_sha256"`
	ImportedAt    string `json:"imported_at"`
	EffectiveFrom string `json:"effective_from"`
	Currency      string `json:"currency"`
	SchemaVersion int    `json:"schema_version"`
	Models        int64  `json:"models"`
	Components    int64  `json:"components"`
}

type PriceProvenance struct {
	CatalogVersion string `json:"catalog_version"`
	SourceKind     string `json:"source_kind"`
	SourceURL      string `json:"source_url"`
	CommitSHA      string `json:"commit_sha"`
	ContentSHA256  string `json:"content_sha256"`
	EffectiveFrom  string `json:"effective_from"`
}

type EffectivePrice struct {
	Provider   string                     `json:"provider"`
	Model      string                     `json:"model"`
	Unit       string                     `json:"unit"`
	Prices     map[string]string          `json:"prices"`
	Provenance map[string]PriceProvenance `json:"provenance"`
}

type priceLayerOrder struct {
	sourceKind       string
	catalogEffective time.Time
	importedAt       time.Time
	version          string
}

func priceLayerBefore(left, right priceLayerOrder) bool {
	leftOfficial, rightOfficial := left.sourceKind == "official", right.sourceKind == "official"
	if leftOfficial != rightOfficial {
		return leftOfficial
	}
	if !left.catalogEffective.Equal(right.catalogEffective) {
		return left.catalogEffective.After(right.catalogEffective)
	}
	if !left.importedAt.Equal(right.importedAt) {
		return left.importedAt.After(right.importedAt)
	}
	return left.version > right.version
}

func New(s *store.Store, home string) *Service {
	return &Service{Store: s, Home: home, Now: time.Now, Stat: os.Stat, Open: func(path string) (SourceFile, error) { return os.Open(path) }}
}
func (s *Service) stat(path string) (os.FileInfo, error) {
	if s.Stat != nil {
		return s.Stat(path)
	}
	return os.Stat(path)
}
func (s *Service) open(path string) (SourceFile, error) {
	if s.Open != nil {
		return s.Open(path)
	}
	return os.Open(path)
}
func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func decimal(v string) (*big.Rat, error) {
	r, ok := new(big.Rat).SetString(v)
	if !ok || r.Sign() < 0 {
		return nil, fmt.Errorf("invalid decimal %q", v)
	}
	return r, nil
}
func money(r *big.Rat) string               { return r.FloatString(9) }
func multiplier(v string) (*big.Rat, error) { return decimal(v) }

func Calculate(client, model string, tokens map[string]int64, prices modelPrice, mult string) (Result, error) {
	for _, n := range tokenNames {
		if tokens[n] < 0 {
			return Result{}, fmt.Errorf("invalid token count: %s", n)
		}
	}
	expected := map[string]string{"codex": "openai", "claude": "anthropic"}[client]
	if expected == "" || prices.Provider != expected {
		return Result{Tokens: tokens, KnownCatalogBaseCost: "0.000000000", KnownProviderCost: "0.000000000", Unpriced: []string{"unknown_model"}}, nil
	}
	m, err := multiplier(mult)
	if err != nil {
		return Result{}, err
	}
	base := new(big.Rat)
	unpriced := []string{}
	add := func(count int64, component string) error {
		if count == 0 {
			return nil
		}
		raw, ok := prices.Prices[component]
		if !ok {
			unpriced = append(unpriced, component)
			return nil
		}
		p, e := decimal(raw)
		if e != nil {
			return e
		}
		base.Add(base, new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(count, 1), p), big.NewRat(1000000, 1)))
		return nil
	}
	if client == "codex" {
		if tokens["cached_input_tokens"] > tokens["input_tokens"] {
			return Result{}, errors.New("cached_input_tokens exceeds input_tokens")
		}
		if err = add(tokens["input_tokens"]-tokens["cached_input_tokens"], "input"); err != nil {
			return Result{}, err
		}
		if err = add(tokens["cached_input_tokens"], "cached_input"); err != nil {
			return Result{}, err
		}
		if err = add(tokens["output_tokens"], "output"); err != nil {
			return Result{}, err
		}
	} else {
		if err = add(tokens["input_tokens"], "input"); err != nil {
			return Result{}, err
		}
		if err = add(tokens["output_tokens"], "output"); err != nil {
			return Result{}, err
		}
		if tokens["cache_creation_tokens"] > 0 && tokens["cache_write_5m_tokens"] == 0 && tokens["cache_write_1h_tokens"] == 0 {
			unpriced = append(unpriced, "cache_creation_tokens")
		}
		for _, p := range []struct{ n, p string }{{"cache_write_5m_tokens", "cache_write_5m"}, {"cache_write_1h_tokens", "cache_write_1h"}, {"cache_read_tokens", "cache_read"}} {
			if err = add(tokens[p.n], p.p); err != nil {
				return Result{}, err
			}
		}
	}
	b := money(base)
	f := money(new(big.Rat).Mul(base, m))
	result := Result{Tokens: tokens, KnownCatalogBaseCost: b, KnownProviderCost: f, Unpriced: unpriced}
	if len(unpriced) == 0 {
		result.CatalogBaseCost = &b
		result.ProviderCost = &f
	}
	return result, nil
}

func parseCatalog(data []byte) (catalog, error) {
	var c catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	if c.SchemaVersion != 1 || c.Version == "" || c.Currency != "USD" || len(c.Models) == 0 {
		return c, errors.New("invalid price catalog")
	}
	for name, p := range c.Models {
		if p.Provider != "openai" && p.Provider != "anthropic" || p.EffectiveFrom == "" {
			return c, fmt.Errorf("invalid model price: %s", name)
		}
	}
	return c, nil
}
func (s *Service) ImportBundledCatalog(ctx context.Context) error {
	c, err := parseCatalog(bundledCatalog)
	if err != nil {
		return err
	}
	effective := s.now()
	for _, model := range c.Models {
		if at, parseErr := time.Parse(time.RFC3339Nano, model.EffectiveFrom); parseErr == nil && at.Before(effective) {
			effective = at
		}
	}
	return s.importCatalog(ctx, bundledCatalog, "bundled", bundledCatalogSourceURL, "", effective, hash(bundledCatalog))
}
func (s *Service) importCatalog(ctx context.Context, data []byte, kind, url, commit string, effective time.Time, contentHash string) error {
	c, err := parseCatalog(data)
	if err != nil {
		return err
	}
	if !validSHA256(contentHash) {
		return errors.New("invalid catalog content SHA-256")
	}
	tx, err := s.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO price_catalogs(version,source_kind,source_url,commit_sha,content_sha256,imported_at,effective_from,currency,schema_version) VALUES(?,?,?,?,?,?,?,?,?)`, c.Version, kind, url, commit, contentHash, s.now().Format(time.RFC3339Nano), effective.Format(time.RFC3339Nano), c.Currency, c.SchemaVersion)
	if err != nil {
		return err
	}
	for name, p := range c.Models {
		prices, _ := json.Marshal(p.Prices)
		aliases, _ := json.Marshal(p.Aliases)
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO model_prices(catalog_version,model,provider,effective_from,prices_json,aliases_json) VALUES(?,?,?,?,?,?)`, c.Version, name, p.Provider, p.EffectiveFrom, prices, aliases)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ImportOfficialOverrides creates an immutable, provenance-bearing catalog
// layer. Component maps are overlaid on the applicable local catalog at read
// time; omitted components therefore retain their prior catalog values.
func (s *Service) ImportOfficialOverrides(ctx context.Context, overrides []OfficialOverride) error {
	if len(overrides) == 0 {
		return errors.New("official price overrides are empty")
	}
	c := catalog{SchemaVersion: 1, Currency: "USD", Models: map[string]modelPrice{}}
	var source string
	for _, o := range overrides {
		if o.Model == "" || o.SourceURL == "" || o.EffectiveFrom.IsZero() || (o.Provider != "openai" && o.Provider != "anthropic") || len(o.Prices) == 0 {
			return errors.New("official override requires model, direct provider, source URL, effective time, and prices")
		}
		if source == "" {
			source = o.SourceURL
		}
		if source != o.SourceURL {
			return errors.New("official overrides must share one provenance URL")
		}
		for _, v := range o.Prices {
			if _, err := decimal(v); err != nil {
				return err
			}
		}
		c.Models[o.Model] = modelPrice{Provider: o.Provider, EffectiveFrom: o.EffectiveFrom.UTC().Format(time.RFC3339Nano), Prices: o.Prices}
	}
	encoded, err := json.Marshal(c)
	if err != nil {
		return err
	}
	c.Version = "official-" + hash(encoded)[:16]
	encoded, err = json.Marshal(c)
	if err != nil {
		return err
	}
	return s.importCatalog(ctx, encoded, "official", source, "", earliestOverride(overrides), hash(encoded))
}
func earliestOverride(items []OfficialOverride) time.Time {
	at := items[0].EffectiveFrom.UTC()
	for _, x := range items[1:] {
		if x.EffectiveFrom.Before(at) {
			at = x.EffectiveFrom.UTC()
		}
	}
	return at
}

func (s *Service) Scan(ctx context.Context) (map[string]int, error) {
	inventory, err := s.Inventory(ctx)
	if err != nil {
		return nil, err
	}
	return s.ScanInventory(ctx, inventory)
}

func (s *Service) ScanInventory(ctx context.Context, inventory Inventory) (map[string]int, error) {
	if err := s.ImportBundledCatalog(ctx); err != nil {
		return nil, err
	}
	out := newScanResult(len(inventory.Entries))
	for _, path := range inventory.Removed {
		if err := s.detachSource(ctx, path); err != nil {
			return nil, err
		}
	}
	recovering, recoveryCandidates, err := s.orphanRecoveryCandidates(ctx, inventory.Entries)
	if err != nil {
		return nil, err
	}
	changed := make(map[string]bool, len(inventory.Added)+len(inventory.Appended)+len(inventory.Mutated))
	for _, paths := range [][]string{inventory.Added, inventory.Appended, inventory.Mutated} {
		for _, path := range paths {
			changed[path] = true
		}
	}
	for _, entry := range inventory.Entries {
		candidate := recoveryCandidates[entry.Path]
		if !candidate && !changed[entry.Path] {
			continue
		}
		stats, err := s.scanFileMode(ctx, entry, candidate)
		if err != nil {
			return nil, err
		}
		for key, value := range stats {
			out[key] += value
		}
	}
	if recovering {
		removed, cleanupErr := s.cleanupOrphanedEvents(ctx)
		if cleanupErr != nil {
			return nil, cleanupErr
		}
		out["removed"] += removed
	}
	if err := s.Store.SetSetting(ctx, "watch.fingerprint.usage", inventory.Fingerprint); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) Rebuild(ctx context.Context) (map[string]int, []string, error) {
	if err := s.ImportBundledCatalog(ctx); err != nil {
		return nil, nil, err
	}
	inventory, err := s.Inventory(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := newScanResult(len(inventory.Entries))
	warningSet := map[string]bool{}
	for _, path := range inventory.Removed {
		if err = s.detachSource(ctx, path); err != nil {
			warningSet["usage_source_rebuild_failed"] = true
		}
	}
	// Rebuild higher-priority paths first. Event ownership uses the same path
	// ordering, so a failed owner remains authoritative and a lower-priority
	// duplicate cannot take its row in a separately committed transaction.
	for index := len(inventory.Entries) - 1; index >= 0; index-- {
		entry := inventory.Entries[index]
		stats, rebuildErr := s.scanFileMode(ctx, entry, true)
		if rebuildErr != nil {
			warning := "usage_source_rebuild_failed"
			if errors.Is(rebuildErr, errUsageSourceChanged) {
				warning = "usage_source_unstable"
			}
			warningSet[warning] = true
			continue
		}
		mergeScanResult(out, stats)
	}
	warnings := make([]string, 0, len(warningSet))
	for warning := range warningSet {
		warnings = append(warnings, warning)
	}
	sort.Strings(warnings)
	if len(warnings) == 0 {
		if _, err = s.cleanupOrphanedEvents(ctx); err != nil {
			return nil, nil, err
		}
		if err = s.Store.SetSetting(ctx, "watch.fingerprint.usage", inventory.Fingerprint); err != nil {
			return nil, nil, err
		}
	}
	return out, warnings, nil
}

func newScanResult(files int) map[string]int {
	return map[string]int{"files": files, "imported": 0, "updated": 0, "removed": 0, "ignored_non_usage": 0, "unsupported_usage": 0, "malformed": 0, "source_resets": 0, "replaced": 0, "unsupported": 0}
}

func mergeScanResult(total, stats map[string]int) {
	for key, value := range stats {
		total[key] += value
	}
}

func (s *Service) InventoryFingerprint() (string, error) {
	entries, err := s.inventoryEntries()
	if err != nil {
		return "", err
	}
	return inventoryFingerprint(entries), nil
}

func (s *Service) Inventory(ctx context.Context) (Inventory, error) {
	entries, err := s.inventoryEntries()
	if err != nil {
		return Inventory{}, err
	}
	inventory := Inventory{Entries: entries, Fingerprint: inventoryFingerprint(entries)}
	rows, err := s.Store.DB.QueryContext(ctx, "SELECT path,identity,size,cursor,modified_at,parser_version FROM usage_source_files")
	if err != nil {
		return Inventory{}, err
	}
	defer rows.Close()
	type storedEntry struct {
		identity               string
		size, cursor, modified int64
		parserVersion          int
	}
	stored := map[string]storedEntry{}
	for rows.Next() {
		var path string
		var item storedEntry
		if err = rows.Scan(&path, &item.identity, &item.size, &item.cursor, &item.modified, &item.parserVersion); err != nil {
			return Inventory{}, err
		}
		stored[path] = item
	}
	if err = rows.Err(); err != nil {
		return Inventory{}, err
	}
	for _, entry := range entries {
		previous, found := stored[entry.Path]
		delete(stored, entry.Path)
		switch {
		case !found:
			inventory.Added = append(inventory.Added, entry.Path)
		case previous.parserVersion != usageParserVersion:
			inventory.Mutated = append(inventory.Mutated, entry.Path)
		case previous.identity == entry.Identity && previous.size == entry.Size && previous.modified == entry.ModifiedAt:
		case previous.identity == entry.Identity && entry.Size > previous.size && entry.Size >= previous.cursor:
			inventory.Appended = append(inventory.Appended, entry.Path)
		default:
			inventory.Mutated = append(inventory.Mutated, entry.Path)
		}
	}
	for path := range stored {
		inventory.Removed = append(inventory.Removed, path)
	}
	sort.Strings(inventory.Removed)
	return inventory, nil
}

func (s *Service) inventoryEntries() ([]InventoryEntry, error) {
	var entries []InventoryEntry
	for _, client := range []string{"codex", "claude"} {
		paths, err := s.sourcePaths(client)
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			info, err := s.stat(path)
			if err != nil {
				return nil, err
			}
			entries = append(entries, InventoryEntry{Path: path, Client: client, Identity: usageFileIdentity(info), Size: info.Size(), ModifiedAt: info.ModTime().UnixNano()})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func inventoryFingerprint(entries []InventoryEntry) string {
	records := make([]string, 0, len(entries))
	for _, entry := range entries {
		records = append(records, strings.Join([]string{entry.Path, entry.Identity, fmt.Sprint(entry.Size), fmt.Sprint(entry.ModifiedAt)}, "\x00"))
	}
	return hash([]byte(strings.Join(records, "\n")))
}
func (s *Service) sourcePaths(client string) ([]string, error) {
	var patterns []string
	if client == "codex" {
		patterns = []string{filepath.Join(s.Home, ".codex/sessions/**/*.jsonl"), filepath.Join(s.Home, ".codex/archived_sessions/*.jsonl")}
	} else {
		patterns = []string{filepath.Join(s.Home, ".claude/projects/**/*.jsonl")}
	}
	var ret []string
	for _, pattern := range patterns {
		marker := strings.Index(pattern, "**")
		if marker < 0 {
			m, _ := filepath.Glob(pattern)
			ret = append(ret, m...)
			continue
		}
		root := pattern[:marker]
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if !d.IsDir() && strings.HasSuffix(p, ".jsonl") {
				ret = append(ret, p)
			}
			return nil
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	sort.Strings(ret)
	return ret, nil
}
func (s *Service) scanFile(ctx context.Context, entry InventoryEntry) (map[string]int, error) {
	return s.scanFileMode(ctx, entry, false)
}

func (s *Service) scanFileMode(ctx context.Context, entry InventoryEntry, forceRebuild bool) (map[string]int, error) {
	r := map[string]int{"imported": 0, "updated": 0, "ignored_non_usage": 0, "unsupported_usage": 0, "malformed": 0, "source_resets": 0, "replaced": 0, "unsupported": 0}
	path, client := entry.Path, entry.Client
	var cursor, oldSize, oldModified int64
	var oldIdentity, oldHash string
	var cumulativeJSON string
	var parserVersion int
	state := parseState{codexCumulative: map[string]map[string]int64{}}
	row := s.Store.DB.QueryRowContext(ctx, "SELECT cursor,identity,size,modified_at,prefix_hash,COALESCE(session_id,''),COALESCE(turn_id,''),COALESCE(model,''),parser_version,codex_cumulative_json FROM usage_source_files WHERE path=?", path)
	loadErr := row.Scan(&cursor, &oldIdentity, &oldSize, &oldModified, &oldHash, &state.session, &state.turn, &state.model, &parserVersion, &cumulativeJSON)
	found := loadErr == nil
	if loadErr != nil && !errors.Is(loadErr, sql.ErrNoRows) {
		return r, loadErr
	}
	if found && client == "codex" {
		if err := json.Unmarshal([]byte(cumulativeJSON), &state.codexCumulative); err != nil {
			return r, fmt.Errorf("invalid Codex cumulative usage cursor for %q: %w", path, err)
		}
	}
	activityParser := activity.NewParser(client, path)
	activityParser.SetContext(state.session, state.turn, state.model)
	parserOutdated := found && parserVersion != usageParserVersion
	stableMetadata := found && !parserOutdated && oldIdentity == entry.Identity && oldSize == entry.Size && oldModified == entry.ModifiedAt
	if !forceRebuild && stableMetadata {
		return r, nil
	}
	file, err := s.open(path)
	if err != nil {
		return r, err
	}
	defer file.Close()
	appendOnly := found && oldIdentity == entry.Identity && entry.Size >= cursor
	var previousAnchor []byte
	var previousAnchorStart int64
	if appendOnly && cursor > 0 {
		previousAnchorStart = max(int64(0), cursor-4096)
		previousAnchor = make([]byte, cursor-previousAnchorStart)
		if _, err = io.ReadFull(io.NewSectionReader(file, previousAnchorStart, int64(len(previousAnchor))), previousAnchor); err != nil {
			return r, err
		}
		appendOnly = hash(previousAnchor) == oldHash
	}
	sourceMutated := !appendOnly && found
	reset := sourceMutated || parserOutdated || (forceRebuild && found)
	if reset {
		cursor = 0
		state = parseState{codexCumulative: map[string]map[string]int64{}}
		activityParser = activity.NewParser(client, path)
		if sourceMutated {
			r["source_resets"]++
		}
		r["replaced"]++
	}
	data, err := io.ReadAll(io.NewSectionReader(file, cursor, entry.Size-cursor))
	if err != nil {
		return r, err
	}
	events := make([]Event, 0)
	toolActivities := make([]activity.Record, 0)
	offset, line := cursor, data
	for len(line) > 0 {
		idx := bytes.IndexByte(line, '\n')
		if idx < 0 {
			break
		}
		raw := line[:idx]
		next := int64(idx + 1)
		var value map[string]any
		if err := json.Unmarshal(raw, &value); err != nil {
			r["malformed"]++
			offset += next
			line = line[idx+1:]
			continue
		}
		ev, ok := parse(client, value, &state, path, offset)
		toolActivities = append(toolActivities, activityParser.Parse(value, offset)...)
		if !ok {
			if looksLikeUsage(client, value) {
				r["unsupported_usage"]++
				r["unsupported"]++
			} else {
				r["ignored_non_usage"]++
			}
		} else if ev.Key != "" {
			events = append(events, ev)
		}
		offset += next
		line = line[idx+1:]
	}
	if err = s.validateSnapshot(path, file, entry, cursor, data, previousAnchorStart, previousAnchor); err != nil {
		return r, err
	}
	// A cursor is always the end of a complete record.  The unfinished suffix is
	// deliberately re-read next time, so an interrupted write cannot be skipped.
	cursor = offset
	anchorStart := max(int64(0), cursor-4096)
	anchor := make([]byte, cursor-anchorStart)
	if len(anchor) > 0 {
		if _, err = io.ReadFull(io.NewSectionReader(file, anchorStart, int64(len(anchor))), anchor); err != nil {
			return r, err
		}
	}
	tx, err := s.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return r, err
	}
	defer tx.Rollback()
	affected, err := affectedSessions(ctx, tx, path)
	if err != nil {
		return r, err
	}
	var preservedBindings []eventRunBinding
	if reset && !sourceMutated {
		preservedBindings, err = eventRunBindingsForSource(ctx, tx, path)
		if err != nil {
			return r, err
		}
	}
	if reset {
		if _, err = tx.ExecContext(ctx, "DELETE FROM usage_events WHERE source_path=?", path); err != nil {
			return r, err
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM usage_tool_calls WHERE source_path=?", path); err != nil {
			return r, err
		}
		if sourceMutated {
			if _, err = tx.ExecContext(ctx, "DELETE FROM usage_run_sources WHERE path=?", path); err != nil {
				return r, err
			}
		}
	}
	for _, event := range events {
		affected[event.Client+"\x00"+event.SessionID] = [2]string{event.Client, event.SessionID}
		inserted, changed, upsertErr := upsertTx(ctx, tx, event)
		if upsertErr != nil {
			return r, upsertErr
		}
		if !changed {
			continue
		}
		if inserted {
			r["imported"]++
		} else {
			r["updated"]++
			r["replaced"]++
		}
	}
	for _, item := range toolActivities {
		if err = upsertToolActivityTx(ctx, tx, item); err != nil {
			return r, err
		}
	}
	if err = restoreEventRunBindings(ctx, tx, path, preservedBindings); err != nil {
		return r, err
	}
	if reset && !sourceMutated {
		if err = restoreEventRunBindingsForSourceRange(ctx, tx, path); err != nil {
			return r, err
		}
	}
	cumulativeBytes, err := json.Marshal(state.codexCumulative)
	if err != nil {
		return r, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash,session_id,turn_id,model,parser_version,codex_cumulative_json,imported,replaced,malformed,unsupported,modified_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(path) DO UPDATE SET identity=excluded.identity,size=excluded.size,cursor=excluded.cursor,prefix_hash=excluded.prefix_hash,session_id=excluded.session_id,turn_id=excluded.turn_id,model=excluded.model,parser_version=excluded.parser_version,codex_cumulative_json=excluded.codex_cumulative_json,imported=usage_source_files.imported+excluded.imported,replaced=usage_source_files.replaced+excluded.replaced,malformed=usage_source_files.malformed+excluded.malformed,unsupported=usage_source_files.unsupported+excluded.unsupported,modified_at=excluded.modified_at`, path, entry.Identity, entry.Size, cursor, hash(anchor), state.session, state.turn, state.model, usageParserVersion, string(cumulativeBytes), r["imported"], r["replaced"], r["malformed"], r["unsupported"], entry.ModifiedAt)
	if err != nil {
		return r, err
	}
	if err = rebuildSessions(ctx, tx, affected); err != nil {
		return r, err
	}
	return r, tx.Commit()
}

func (s *Service) validateSnapshot(path string, file SourceFile, entry InventoryEntry, cursor int64, data []byte, previousAnchorStart int64, previousAnchor []byte) error {
	latest, err := s.stat(path)
	if err != nil {
		return err
	}
	if usageFileIdentity(latest) != entry.Identity || latest.Size() < entry.Size {
		return errUsageSourceChanged
	}
	current := make([]byte, len(data))
	if len(current) > 0 {
		if _, err = io.ReadFull(io.NewSectionReader(file, cursor, int64(len(current))), current); err != nil {
			return err
		}
		if !bytes.Equal(current, data) {
			return errUsageSourceChanged
		}
	}
	if len(previousAnchor) > 0 {
		currentAnchor := make([]byte, len(previousAnchor))
		if _, err = io.ReadFull(io.NewSectionReader(file, previousAnchorStart, int64(len(currentAnchor))), currentAnchor); err != nil {
			return err
		}
		if !bytes.Equal(currentAnchor, previousAnchor) {
			return errUsageSourceChanged
		}
	}
	if latest.Size() == entry.Size && latest.ModTime().UnixNano() != entry.ModifiedAt {
		return errUsageSourceChanged
	}
	return nil
}
func usageFileIdentity(info os.FileInfo) string {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return fmt.Sprintf("%d:%d", stat.Dev, stat.Ino)
	}
	return info.Name()
}
func looksLikeUsage(client string, value map[string]any) bool {
	if client == "codex" {
		p, _ := value["payload"].(map[string]any)
		return value["type"] == "event_msg" && p["type"] == "token_count"
	}
	if value["type"] != "assistant" {
		return false
	}
	m, _ := value["message"].(map[string]any)
	_, ok := m["usage"]
	return ok
}
func hash(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }

type parseState struct {
	session, turn, model string
	codexCumulative      map[string]map[string]int64
}

func tokenUsage(value any) (map[string]int64, bool) {
	raw, _ := value.(map[string]any)
	if raw == nil || !validTokenFields(raw, "input_tokens", "cached_input_tokens", "output_tokens") {
		return nil, false
	}
	return map[string]int64{
		"input_tokens":        integer(raw["input_tokens"]),
		"cached_input_tokens": integer(raw["cached_input_tokens"]),
		"output_tokens":       integer(raw["output_tokens"]),
	}, true
}

func codexUsageDelta(state *parseState, lastValue, totalValue any) (map[string]int64, bool) {
	last, lastOK := tokenUsage(lastValue)
	total, totalOK := tokenUsage(totalValue)
	if state.codexCumulative == nil {
		state.codexCumulative = map[string]map[string]int64{}
	}
	previous, previousOK := state.codexCumulative[state.session]
	if !totalOK {
		delete(state.codexCumulative, state.session)
		return last, lastOK
	}
	state.codexCumulative[state.session] = total
	if previousOK {
		delta := map[string]int64{}
		for _, name := range []string{"input_tokens", "cached_input_tokens", "output_tokens"} {
			if total[name] < previous[name] {
				return last, lastOK
			}
			delta[name] = total[name] - previous[name]
		}
		return delta, true
	}
	return last, lastOK
}

func codexEventKey(state parseState, timestamp string, lastUsage map[string]any, totalUsage any) string {
	identity, _ := json.Marshal(struct {
		Timestamp  string         `json:"timestamp"`
		Model      string         `json:"model"`
		LastUsage  map[string]any `json:"last_token_usage"`
		TotalUsage any            `json:"total_token_usage,omitempty"`
	}{
		Timestamp:  timestamp,
		Model:      state.model,
		LastUsage:  lastUsage,
		TotalUsage: totalUsage,
	})
	return "codex:" + state.session + ":" + state.turn + ":" + hash(identity)
}

func integer(v any) int64 {
	f, ok := v.(float64)
	if !ok || f < 0 || f != float64(int64(f)) {
		return 0
	}
	return int64(f)
}
func validTokenFields(values map[string]any, names ...string) bool {
	for _, name := range names {
		value, present := values[name]
		if !present {
			continue
		}
		f, ok := value.(float64)
		if !ok || f < 0 || f != float64(int64(f)) {
			return false
		}
	}
	return true
}
func parse(client string, v map[string]any, state *parseState, path string, offset int64) (Event, bool) {
	if client == "codex" {
		p, _ := v["payload"].(map[string]any)
		typ, _ := v["type"].(string)
		if typ == "session_meta" {
			state.session, _ = p["session_id"].(string)
			if state.session == "" {
				state.session, _ = p["id"].(string)
			}
			return Event{}, false
		}
		if typ == "turn_context" {
			state.turn, _ = p["turn_id"].(string)
			state.model, _ = p["model"].(string)
			return Event{}, false
		}
		if typ != "event_msg" || state.session == "" || state.turn == "" || state.model == "" {
			return Event{}, false
		}
		if p["type"] != "token_count" {
			return Event{}, false
		}
		info, _ := p["info"].(map[string]any)
		u, ok := codexUsageDelta(state, info["last_token_usage"], info["total_token_usage"])
		if !ok {
			return Event{}, false
		}
		if u["input_tokens"] == 0 && u["cached_input_tokens"] == 0 && u["output_tokens"] == 0 {
			return Event{}, true
		}
		timestamp := stringValue(v, "timestamp")
		lastUsage, _ := info["last_token_usage"].(map[string]any)
		return Event{Key: codexEventKey(*state, timestamp, lastUsage, info["total_token_usage"]), Client: client, SessionID: state.session, EventID: state.turn, EventAt: timestamp, Model: state.model, SourcePath: path, SourceOffset: offset, Tokens: u}, true
	}
	if v["type"] != "assistant" {
		return Event{}, false
	}
	msg, _ := v["message"].(map[string]any)
	sid, _ := v["sessionId"].(string)
	id, _ := msg["id"].(string)
	model, _ := msg["model"].(string)
	usage, _ := msg["usage"].(map[string]any)
	if sid == "" || id == "" || model == "" || usage == nil || model == "<synthetic>" {
		return Event{}, false
	}
	creation, _ := usage["cache_creation"].(map[string]any)
	if !validTokenFields(usage, "input_tokens", "output_tokens", "cache_read_input_tokens", "cache_creation_input_tokens") || (creation != nil && !validTokenFields(creation, "ephemeral_5m_input_tokens", "ephemeral_1h_input_tokens")) {
		return Event{}, false
	}
	t := map[string]int64{"input_tokens": integer(usage["input_tokens"]), "output_tokens": integer(usage["output_tokens"]), "cache_read_tokens": integer(usage["cache_read_input_tokens"]), "cache_creation_tokens": integer(usage["cache_creation_input_tokens"])}
	if creation != nil {
		t["cache_write_5m_tokens"] = integer(creation["ephemeral_5m_input_tokens"])
		t["cache_write_1h_tokens"] = integer(creation["ephemeral_1h_input_tokens"])
	}
	return Event{Key: "claude:" + sid + ":" + id, Client: client, SessionID: sid, EventID: id, EventAt: stringValue(v, "timestamp"), Model: model, SourcePath: path, SourceOffset: offset, Tokens: t}, true
}
func stringValue(v map[string]any, k string) string { x, _ := v[k].(string); return x }
func upsertTx(ctx context.Context, tx *sql.Tx, e Event) (inserted, changed bool, err error) {
	at, err := time.Parse(time.RFC3339Nano, e.EventAt)
	if err != nil {
		return false, false, fmt.Errorf("invalid usage event timestamp %q: %w", e.EventAt, err)
	}
	e.EventAt = at.UTC().Format(time.RFC3339Nano)
	var existingPath, existingEventAt, existingModel string
	var existingSourceIndexed int
	var existingInput, existingCachedInput, existingOutput, existingCacheRead, existingCacheCreation, existingCacheWrite5m, existingCacheWrite1h int64
	lookupErr := tx.QueryRowContext(ctx, `SELECT e.source_path,CASE WHEN f.path IS NULL THEN 0 ELSE 1 END,e.event_at,e.model,e.input_tokens,e.cached_input_tokens,e.output_tokens,e.cache_read_tokens,e.cache_creation_tokens,e.cache_write_5m_tokens,e.cache_write_1h_tokens FROM usage_events e LEFT JOIN usage_source_files f ON f.path=e.source_path WHERE e.event_key=?`, e.Key).Scan(&existingPath, &existingSourceIndexed, &existingEventAt, &existingModel, &existingInput, &existingCachedInput, &existingOutput, &existingCacheRead, &existingCacheCreation, &existingCacheWrite5m, &existingCacheWrite1h)
	exists := lookupErr == nil
	if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return false, false, lookupErr
	}
	if exists && existingSourceIndexed == 1 && existingPath > e.SourcePath {
		return false, false, nil
	}
	vals := []any{e.Key, e.Client, e.SessionID, e.EventID, e.EventAt, e.Model, e.Tokens["input_tokens"], e.Tokens["cached_input_tokens"], e.Tokens["output_tokens"], e.Tokens["cache_read_tokens"], e.Tokens["cache_creation_tokens"], e.Tokens["cache_write_5m_tokens"], e.Tokens["cache_write_1h_tokens"], e.SourcePath, e.SourceOffset}
	_, err = tx.ExecContext(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,cache_read_tokens,cache_creation_tokens,cache_write_5m_tokens,cache_write_1h_tokens,source_path,source_offset)VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(event_key) DO UPDATE SET event_at=excluded.event_at,model=excluded.model,input_tokens=excluded.input_tokens,cached_input_tokens=excluded.cached_input_tokens,output_tokens=excluded.output_tokens,cache_read_tokens=excluded.cache_read_tokens,cache_creation_tokens=excluded.cache_creation_tokens,cache_write_5m_tokens=excluded.cache_write_5m_tokens,cache_write_1h_tokens=excluded.cache_write_1h_tokens,source_path=excluded.source_path,source_offset=excluded.source_offset`, vals...)
	logicalChanged := !exists || existingEventAt != e.EventAt || existingModel != e.Model || existingInput != e.Tokens["input_tokens"] || existingCachedInput != e.Tokens["cached_input_tokens"] || existingOutput != e.Tokens["output_tokens"] || existingCacheRead != e.Tokens["cache_read_tokens"] || existingCacheCreation != e.Tokens["cache_creation_tokens"] || existingCacheWrite5m != e.Tokens["cache_write_5m_tokens"] || existingCacheWrite1h != e.Tokens["cache_write_1h_tokens"]
	return !exists, err == nil && logicalChanged, err
}

func upsertToolActivityTx(ctx context.Context, tx *sql.Tx, record activity.Record) error {
	var existingPath, startedAt string
	var existingSourceIndexed int
	lookupErr := tx.QueryRowContext(ctx, `SELECT a.source_path,a.started_at,CASE WHEN f.path IS NULL THEN 0 ELSE 1 END FROM usage_tool_calls a LEFT JOIN usage_source_files f ON f.path=a.source_path WHERE a.activity_key=?`, record.Key).Scan(&existingPath, &startedAt, &existingSourceIndexed)
	exists := lookupErr == nil
	if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
		return lookupErr
	}
	if exists && existingSourceIndexed == 1 && existingPath > record.SourcePath {
		return nil
	}
	if record.StartedAt != "" {
		_, err := tx.ExecContext(ctx, `INSERT INTO usage_tool_calls(activity_key,client,session_id,model,tool_name,started_at,completed_at,status,duration_ms,source_path,source_offset) VALUES(?,?,?,?,?,?,NULL,'started',NULL,?,?) ON CONFLICT(activity_key) DO UPDATE SET client=excluded.client,session_id=excluded.session_id,model=excluded.model,tool_name=excluded.tool_name,started_at=excluded.started_at,completed_at=NULL,status='started',duration_ms=NULL,source_path=excluded.source_path,source_offset=excluded.source_offset`, record.Key, record.Client, record.SessionID, record.Model, record.Tool, record.StartedAt, record.SourcePath, record.SourceOffset)
		return err
	}
	if !exists || record.CompletedAt == "" {
		return nil
	}
	var duration any
	started, startErr := time.Parse(time.RFC3339Nano, startedAt)
	completed, completeErr := time.Parse(time.RFC3339Nano, record.CompletedAt)
	if startErr == nil && completeErr == nil && !completed.Before(started) {
		duration = completed.Sub(started).Milliseconds()
	}
	_, err := tx.ExecContext(ctx, `UPDATE usage_tool_calls SET completed_at=?,status=?,duration_ms=?,source_path=? WHERE activity_key=?`, record.CompletedAt, record.Status, duration, record.SourcePath, record.Key)
	return err
}

type eventRunBinding struct {
	eventKey string
	runID    int64
}

func eventRunBindingsForSource(ctx context.Context, tx *sql.Tx, path string) ([]eventRunBinding, error) {
	rows, err := tx.QueryContext(ctx, `SELECT e.event_key,COALESCE(b.run_id,e.run_id) FROM usage_events e LEFT JOIN usage_run_bindings b ON b.event_key=e.event_key WHERE e.source_path=? AND COALESCE(b.run_id,e.run_id) IS NOT NULL ORDER BY e.event_key`, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var bindings []eventRunBinding
	for rows.Next() {
		var binding eventRunBinding
		if err = rows.Scan(&binding.eventKey, &binding.runID); err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	return bindings, rows.Err()
}

func restoreEventRunBindings(ctx context.Context, tx *sql.Tx, path string, bindings []eventRunBinding) error {
	for _, binding := range bindings {
		if _, err := tx.ExecContext(ctx, `INSERT INTO usage_run_bindings(event_key,run_id) SELECT ?,? WHERE EXISTS (SELECT 1 FROM usage_events WHERE event_key=? AND source_path=?)`, binding.eventKey, binding.runID, binding.eventKey, path); err != nil {
			return err
		}
	}
	return nil
}

func restoreEventRunBindingsForSourceRange(ctx context.Context, tx *sql.Tx, path string) error {
	_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO usage_run_bindings(event_key,run_id)
		SELECT e.event_key,r.run_id
		FROM usage_events e
		JOIN usage_run_sources r ON r.path=e.source_path
		JOIN usage_runs u ON u.id=r.run_id AND u.client=e.client
		WHERE e.source_path=? AND r.end_offset IS NOT NULL AND e.source_offset>=r.start_offset AND e.source_offset<r.end_offset
		ORDER BY r.run_id,e.source_offset,e.event_key`, path)
	return err
}

func affectedSessions(ctx context.Context, tx *sql.Tx, path string) (map[string][2]string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT DISTINCT client,session_id FROM usage_events WHERE source_path=?", path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	affected := map[string][2]string{}
	for rows.Next() {
		var client, sessionID string
		if err = rows.Scan(&client, &sessionID); err != nil {
			return nil, err
		}
		affected[client+"\x00"+sessionID] = [2]string{client, sessionID}
	}
	return affected, rows.Err()
}

func rebuildSessions(ctx context.Context, tx *sql.Tx, affected map[string][2]string) error {
	for _, pair := range affected {
		if _, err := tx.ExecContext(ctx, "DELETE FROM usage_sessions WHERE client=? AND session_id=?", pair[0], pair[1]); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO usage_sessions(client,session_id,first_at,last_at) SELECT client,session_id,MIN(event_at),MAX(event_at) FROM usage_events WHERE client=? AND session_id=? GROUP BY client,session_id`, pair[0], pair[1]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) detachSource(ctx context.Context, path string) error {
	tx, err := s.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM usage_run_sources WHERE path=?", path); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM usage_source_files WHERE path=?", path); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) orphanRecoveryCandidates(ctx context.Context, entries []InventoryEntry) (bool, map[string]bool, error) {
	rows, err := s.Store.DB.QueryContext(ctx, `SELECT client,session_id FROM (SELECT DISTINCT e.client,e.session_id FROM usage_events e LEFT JOIN usage_source_files f ON f.path=e.source_path WHERE f.path IS NULL UNION SELECT DISTINCT a.client,a.session_id FROM usage_tool_calls a LEFT JOIN usage_source_files f ON f.path=a.source_path WHERE f.path IS NULL)`)
	if err != nil {
		return false, nil, err
	}
	type sessionKey struct{ client, session string }
	orphanSessions := map[sessionKey]bool{}
	for rows.Next() {
		var key sessionKey
		if err = rows.Scan(&key.client, &key.session); err != nil {
			rows.Close()
			return false, nil, err
		}
		orphanSessions[key] = true
	}
	if err = rows.Close(); err != nil {
		return false, nil, err
	}
	if len(orphanSessions) == 0 {
		return false, map[string]bool{}, nil
	}
	entryClients := make(map[string]string, len(entries))
	for _, entry := range entries {
		entryClients[entry.Path] = entry.Client
	}
	candidates := map[string]bool{}
	rows, err = s.Store.DB.QueryContext(ctx, `SELECT path,COALESCE(session_id,'') FROM usage_source_files WHERE COALESCE(session_id,'')<>''`)
	if err != nil {
		return false, nil, err
	}
	for rows.Next() {
		var path, sessionID string
		if err = rows.Scan(&path, &sessionID); err != nil {
			rows.Close()
			return false, nil, err
		}
		if orphanSessions[sessionKey{client: entryClients[path], session: sessionID}] {
			candidates[path] = true
		}
	}
	if err = rows.Close(); err != nil {
		return false, nil, err
	}
	return true, candidates, nil
}

func (s *Service) cleanupOrphanedEvents(ctx context.Context) (int, error) {
	tx, err := s.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT e.client,e.session_id FROM usage_events e LEFT JOIN usage_source_files f ON f.path=e.source_path WHERE f.path IS NULL`)
	if err != nil {
		return 0, err
	}
	affected := map[string][2]string{}
	for rows.Next() {
		var client, sessionID string
		if err = rows.Scan(&client, &sessionID); err != nil {
			rows.Close()
			return 0, err
		}
		affected[client+"\x00"+sessionID] = [2]string{client, sessionID}
	}
	if err = rows.Close(); err != nil {
		return 0, err
	}
	deleted, err := tx.ExecContext(ctx, `DELETE FROM usage_events WHERE NOT EXISTS (SELECT 1 FROM usage_source_files f WHERE f.path=usage_events.source_path)`)
	if err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM usage_tool_calls WHERE NOT EXISTS (SELECT 1 FROM usage_source_files f WHERE f.path=usage_tool_calls.source_path)`); err != nil {
		return 0, err
	}
	if err = rebuildSessions(ctx, tx, affected); err != nil {
		return 0, err
	}
	removed, err := deleted.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return int(removed), nil
}
func (s *Service) Diagnose(ctx context.Context) (map[string]any, error) {
	out := map[string]any{}
	for k, q := range map[string]string{"files": "SELECT COUNT(*) FROM usage_source_files", "events": "SELECT COUNT(*) FROM usage_events", "sessions": "SELECT COUNT(*) FROM usage_sessions", "tool_calls": "SELECT COUNT(*) FROM usage_tool_calls", "exact_runs": "SELECT COUNT(*) FROM usage_runs WHERE ended_at IS NULL"} {
		var n int
		if err := s.Store.DB.QueryRowContext(ctx, q).Scan(&n); err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, nil
}

// CheckSourceReadability verifies indexed source handles without reading or
// returning their contents or paths.
func (s *Service) CheckSourceReadability(ctx context.Context) (int, error) {
	rows, err := s.Store.DB.QueryContext(ctx, "SELECT path FROM usage_source_files ORDER BY path")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	unreadable := 0
	for rows.Next() {
		var path string
		if err = rows.Scan(&path); err != nil {
			return 0, err
		}
		file, openErr := os.Open(path)
		if openErr != nil {
			unreadable++
			continue
		}
		if closeErr := file.Close(); closeErr != nil {
			unreadable++
		}
	}
	return unreadable, rows.Err()
}
func (s *Service) PriceHistory(ctx context.Context) ([]PriceCatalog, error) {
	return s.priceHistoryPortable(ctx)
}

func (s *Service) priceHistoryPortable(ctx context.Context) ([]PriceCatalog, error) {
	rows, err := s.Store.DB.QueryContext(ctx, `SELECT c.version,c.source_kind,c.source_url,COALESCE(c.commit_sha,''),c.content_sha256,c.imported_at,c.effective_from,c.currency,c.schema_version,COUNT(mp.model) FROM price_catalogs c LEFT JOIN model_prices mp ON mp.catalog_version=c.version GROUP BY c.version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PriceCatalog
	for rows.Next() {
		var item PriceCatalog
		if err = rows.Scan(&item.Version, &item.SourceKind, &item.SourceURL, &item.CommitSHA, &item.ContentSHA256, &item.ImportedAt, &item.EffectiveFrom, &item.Currency, &item.SchemaVersion, &item.Models); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	var sortErr error
	sort.SliceStable(out, func(i, j int) bool {
		leftEffective, leftErr := time.Parse(time.RFC3339Nano, out[i].EffectiveFrom)
		rightEffective, rightErr := time.Parse(time.RFC3339Nano, out[j].EffectiveFrom)
		leftImported, leftImportedErr := time.Parse(time.RFC3339Nano, out[i].ImportedAt)
		rightImported, rightImportedErr := time.Parse(time.RFC3339Nano, out[j].ImportedAt)
		if leftErr != nil || rightErr != nil || leftImportedErr != nil || rightImportedErr != nil {
			sortErr = errors.Join(leftErr, rightErr, leftImportedErr, rightImportedErr)
			return false
		}
		if !leftEffective.Equal(rightEffective) {
			return leftEffective.Before(rightEffective)
		}
		if !leftImported.Equal(rightImported) {
			return leftImported.Before(rightImported)
		}
		return out[i].Version < out[j].Version
	})
	if sortErr != nil {
		return nil, fmt.Errorf("invalid price catalog time: %w", sortErr)
	}
	for index := range out {
		priceRows, queryErr := s.Store.DB.QueryContext(ctx, `SELECT prices_json FROM model_prices WHERE catalog_version=?`, out[index].Version)
		if queryErr != nil {
			return nil, queryErr
		}
		for priceRows.Next() {
			var raw string
			if queryErr = priceRows.Scan(&raw); queryErr != nil {
				priceRows.Close()
				return nil, queryErr
			}
			var values map[string]string
			if queryErr = json.Unmarshal([]byte(raw), &values); queryErr != nil {
				priceRows.Close()
				return nil, queryErr
			}
			out[index].Components += int64(len(values))
		}
		if queryErr = priceRows.Close(); queryErr != nil {
			return nil, queryErr
		}
	}
	return out, nil
}

// PriceStatus reports the locally available catalog; it never accesses the network.
func (s *Service) PriceStatus(ctx context.Context) (map[string]any, error) {
	history, err := s.PriceHistory(ctx)
	if err != nil {
		return nil, err
	}
	if len(history) == 0 {
		return map[string]any{"available": false}, nil
	}
	now := s.now()
	active := make([]PriceCatalog, 0, len(history))
	for _, item := range history {
		effective, parseErr := time.Parse(time.RFC3339Nano, item.EffectiveFrom)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid catalog effective time %q: %w", item.EffectiveFrom, parseErr)
		}
		if !effective.After(now) {
			active = append(active, item)
		}
	}
	if len(active) == 0 {
		return map[string]any{"available": false}, nil
	}
	latest := active[len(active)-1]
	prices, err := s.priceListAt(ctx, "", "", now)
	if err != nil {
		return nil, err
	}
	components := 0
	for _, price := range prices {
		components += len(price.Prices)
	}
	return map[string]any{
		"available": true, "version": latest.Version, "source_kind": latest.SourceKind,
		"source_url": latest.SourceURL, "commit_sha": latest.CommitSHA,
		"content_sha256": latest.ContentSHA256, "effective_from": latest.EffectiveFrom,
		"aggregated_reference": latest.SourceKind == "litellm", "catalogs": active,
		"models": len(prices), "components": components,
	}, nil
}

// PriceList returns the current component-wise merged effective prices. Every
// component retains the provenance of the catalog row that supplied it.
func (s *Service) PriceList(ctx context.Context, providerFilter, modelFilter string) ([]EffectivePrice, error) {
	return s.priceListAt(ctx, providerFilter, modelFilter, s.now())
}

func (s *Service) priceListAt(ctx context.Context, providerFilter, modelFilter string, at time.Time) ([]EffectivePrice, error) {
	rows, err := s.Store.DB.QueryContext(ctx, `SELECT mp.model,mp.provider,mp.prices_json,c.version,c.source_kind,c.source_url,COALESCE(c.commit_sha,''),c.content_sha256,mp.effective_from,c.effective_from,c.imported_at FROM model_prices mp JOIN price_catalogs c ON c.version=mp.catalog_version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct {
		model, providerName, raw string
		provenance               PriceProvenance
		modelEffective           time.Time
		order                    priceLayerOrder
	}
	var priceRows []row
	at = at.UTC()
	for rows.Next() {
		var item row
		var modelText, catalogText, importedText string
		if err = rows.Scan(&item.model, &item.providerName, &item.raw, &item.provenance.CatalogVersion, &item.provenance.SourceKind, &item.provenance.SourceURL, &item.provenance.CommitSHA, &item.provenance.ContentSHA256, &modelText, &catalogText, &importedText); err != nil {
			return nil, err
		}
		item.provenance.EffectiveFrom = modelText
		item.modelEffective, err = time.Parse(time.RFC3339Nano, modelText)
		if err != nil {
			return nil, fmt.Errorf("invalid model effective time %q: %w", modelText, err)
		}
		item.order = priceLayerOrder{sourceKind: item.provenance.SourceKind, version: item.provenance.CatalogVersion}
		item.order.catalogEffective, err = time.Parse(time.RFC3339Nano, catalogText)
		if err != nil {
			return nil, fmt.Errorf("invalid catalog effective time %q: %w", catalogText, err)
		}
		item.order.importedAt, err = time.Parse(time.RFC3339Nano, importedText)
		if err != nil {
			return nil, fmt.Errorf("invalid catalog import time %q: %w", importedText, err)
		}
		if item.order.catalogEffective.After(at) || item.modelEffective.After(at) {
			continue
		}
		priceRows = append(priceRows, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(priceRows, func(i, j int) bool { return priceLayerBefore(priceRows[i].order, priceRows[j].order) })
	byKey := map[string]*EffectivePrice{}
	for _, priceRow := range priceRows {
		if providerFilter != "" && priceRow.providerName != providerFilter || modelFilter != "" && priceRow.model != modelFilter {
			continue
		}
		var components map[string]string
		if err = json.Unmarshal([]byte(priceRow.raw), &components); err != nil {
			return nil, err
		}
		key := priceRow.providerName + "\x00" + priceRow.model
		item := byKey[key]
		if item == nil {
			item = &EffectivePrice{Provider: priceRow.providerName, Model: priceRow.model, Unit: "USD / 1M tokens", Prices: map[string]string{}, Provenance: map[string]PriceProvenance{}}
			byKey[key] = item
		}
		for component, value := range components {
			if _, exists := item.Prices[component]; exists {
				continue
			}
			item.Prices[component] = value
			item.Provenance[component] = priceRow.provenance
		}
	}
	out := make([]EffectivePrice, 0, len(byKey))
	for _, item := range byKey {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider == out[j].Provider {
			return out[i].Model < out[j].Model
		}
		return out[i].Provider < out[j].Provider
	})
	return out, nil
}

// PriceDiagnostics returns only aggregate catalog health counts. It never
// exposes catalog contents or contacts the network.
func (s *Service) PriceDiagnostics(ctx context.Context) (invalidProvenance, unpricedModels int, err error) {
	rows, err := s.Store.DB.QueryContext(ctx, `SELECT source_kind,source_url,COALESCE(commit_sha,''),content_sha256,schema_version FROM price_catalogs`)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, url, commit, content string
		var schema int
		if err = rows.Scan(&kind, &url, &commit, &content, &schema); err != nil {
			return 0, 0, err
		}
		if !validPriceProvenance(kind, url, commit, content, schema) {
			invalidProvenance++
		}
	}
	if err = rows.Err(); err != nil {
		return 0, 0, err
	}
	events, err := s.events(ctx, "", "")
	if err != nil {
		return 0, 0, err
	}
	unpriced := make(map[string]bool)
	for _, event := range events {
		price, mult, _, priceErr := s.priceForEvent(ctx, event)
		if priceErr != nil {
			return 0, 0, priceErr
		}
		result, calculateErr := Calculate(event.Client, event.Model, event.Tokens, price, mult)
		if calculateErr != nil {
			return 0, 0, calculateErr
		}
		if result.CatalogBaseCost == nil {
			unpriced[event.Model] = true
		}
	}
	return invalidProvenance, len(unpriced), nil
}

func validPriceProvenance(kind, url, commit, content string, schema int) bool {
	if schema != 1 || !validSHA256(content) {
		return false
	}
	switch kind {
	case "bundled":
		return (url == bundledCatalogSourceURL || url == legacyBundledCatalogSourceURL) && commit == ""
	case "official":
		return url != "" && commit == ""
	case "litellm":
		return validLiteLLMCommit(commit) && url == "https://raw.githubusercontent.com/BerriAI/litellm/"+commit+"/model_prices_and_context_window.json"
	default:
		return false
	}
}

func validLiteLLMCommit(commit string) bool {
	if len(commit) != 40 {
		return false
	}
	for _, character := range commit {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// StartRun snapshots every known source's completed byte boundary before the
// child is launched.  A binding may subsequently use only bytes in this closed
// start/end snapshot range, never merely a matching timestamp or session ID.
func (s *Service) StartRun(ctx context.Context, client string, pid int) (int64, time.Time, error) {
	if client != "codex" && client != "claude" {
		return 0, time.Time{}, errors.New("unknown client")
	}
	if _, err := s.RecoverStaleRuns(ctx); err != nil {
		return 0, time.Time{}, err
	}
	snapshot, err := s.Store.CurrentProviderSnapshot(ctx, client)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, time.Time{}, errors.New("no provider selection for client")
	}
	if err != nil {
		return 0, time.Time{}, err
	}
	name, mult := snapshot.Name, snapshot.Multiplier
	start := s.now()
	exact, reason := 1, ""
	if pids, observeErr := s.clientProcesses(client); observeErr == nil {
		for _, other := range pids {
			if other != pid && other != 0 {
				exact, reason = 0, "external_client_overlap"
				break
			}
		}
	}
	tx, err := s.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, time.Time{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO usage_runs(client,provider,multiplier,started_at,process_pid,exact,ambiguity_reason) VALUES(?,?,?,?,?,?,?)`, client, name, mult, start.Format(time.RFC3339Nano), pid, exact, reason)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("exact run overlap: %w", err)
	}
	id, _ := result.LastInsertId()
	rows, err := tx.QueryContext(ctx, `SELECT path,cursor,prefix_hash FROM usage_source_files`)
	if err != nil {
		return 0, time.Time{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var path, prefix string
		var cursor int64
		if err := rows.Scan(&path, &cursor, &prefix); err != nil {
			return 0, time.Time{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO usage_run_sources(run_id,path,start_offset,start_hash) VALUES(?,?,?,?)`, id, path, cursor, prefix); err != nil {
			return 0, time.Time{}, err
		}
	}
	if err := rows.Err(); err != nil {
		return 0, time.Time{}, err
	}
	if err = tx.Commit(); err != nil {
		return 0, time.Time{}, err
	}
	return id, start, nil
}
func (s *Service) EndRun(ctx context.Context, runID int64, client string, start time.Time) error {
	end := s.now()
	tx, err := s.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Existing sources retain their start cursor; sources first seen during the
	// run begin at zero.  This is intentionally independent of event timestamps.
	if _, err = tx.ExecContext(ctx, `INSERT INTO usage_run_sources(run_id,path,start_offset,start_hash) SELECT ?,path,0,'' FROM usage_source_files WHERE path NOT IN (SELECT path FROM usage_run_sources WHERE run_id=?)`, runID, runID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE usage_run_sources SET end_offset=(SELECT cursor FROM usage_source_files f WHERE f.path=usage_run_sources.path),end_hash=(SELECT prefix_hash FROM usage_source_files f WHERE f.path=usage_run_sources.path) WHERE run_id=?`, runID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO usage_run_bindings(event_key,run_id) SELECT e.event_key,? FROM usage_events e JOIN usage_run_sources r ON r.run_id=? AND r.path=e.source_path WHERE e.client=? AND e.source_offset>=r.start_offset AND e.source_offset<r.end_offset`, runID, runID, client); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE usage_runs SET ended_at=? WHERE id=? AND ended_at IS NULL", end.Format(time.RFC3339Nano), runID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// FailRun closes a wrapper whose exact source range could not be proven.
func (s *Service) FailRun(ctx context.Context, runID int64, reason string) error {
	if reason == "" {
		reason = "wrapper_cleanup_failed"
	}
	_, err := s.Store.Exec(ctx, "UPDATE usage_runs SET ended_at=?,exact=0,ambiguity_reason=? WHERE id=? AND ended_at IS NULL", s.now().Format(time.RFC3339Nano), reason, runID)
	return err
}

func (s *Service) clientProcesses(client string) ([]int, error) {
	if s.ClientProcesses != nil {
		return s.ClientProcesses(client)
	}
	out, err := exec.Command("ps", "-axo", "pid=,comm=").Output()
	if err != nil {
		return nil, err
	}
	var ret []int
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		pid, err := strconv.Atoi(f[0])
		if err != nil {
			continue
		}
		if filepath.Base(f[len(f)-1]) == client {
			ret = append(ret, pid)
		}
	}
	return ret, nil
}
func (s *Service) SetRunPID(ctx context.Context, runID int64, pid int) error {
	_, err := s.Store.Exec(ctx, "UPDATE usage_runs SET process_pid=? WHERE id=?", pid, runID)
	return err
}

// RunStatus is intentionally read after EndRun so machine output never claims
// exact attribution when the wrapper discovered an overlapping client.
func (s *Service) RunStatus(ctx context.Context, runID int64) (bool, string, error) {
	var exact int
	var reason string
	err := s.Store.DB.QueryRowContext(ctx, "SELECT exact,ambiguity_reason FROM usage_runs WHERE id=?", runID).Scan(&exact, &reason)
	return exact == 1, reason, err
}

// RecoverStaleRuns closes interrupted wrappers. It never claims their events
// as exact because no end range can be proven.
func (s *Service) RecoverStaleRuns(ctx context.Context) (int64, error) {
	rows, err := s.Store.DB.QueryContext(ctx, "SELECT id,process_pid FROM usage_runs WHERE ended_at IS NULL")
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var stale []int64
	for rows.Next() {
		var id int64
		var pid sql.NullInt64
		if err := rows.Scan(&id, &pid); err != nil {
			return 0, err
		}
		if !pid.Valid || pid.Int64 <= 0 || !processAlive(int(pid.Int64)) {
			stale = append(stale, id)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, id := range stale {
		if _, err := s.Store.Exec(ctx, "UPDATE usage_runs SET ended_at=?,exact=0,ambiguity_reason='stale_wrapper' WHERE id=? AND ended_at IS NULL", s.now().Format(time.RFC3339Nano), id); err != nil {
			return 0, err
		}
	}
	return int64(len(stale)), nil
}

func processAlive(pid int) bool {
	return exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "pid=").Run() == nil
}

func liteLLMCatalog(data []byte, commit string, now time.Time) (catalog, int, error) {
	var rows map[string]map[string]json.RawMessage
	if err := json.Unmarshal(data, &rows); err != nil {
		return catalog{}, 0, err
	}
	c := catalog{SchemaVersion: 1, Version: "litellm-" + commit, Currency: "USD", Models: map[string]modelPrice{}}
	for name, r := range rows {
		var provider string
		_ = json.Unmarshal(r["litellm_provider"], &provider)
		if provider != "openai" && provider != "anthropic" {
			continue
		}
		price := func(k string) (string, bool) {
			raw, ok := r[k]
			if !ok {
				return "", false
			}
			var n json.Number
			if err := json.Unmarshal(raw, &n); err != nil {
				return "", false
			}
			perToken, err := decimal(n.String())
			if err != nil {
				return "", false
			}
			return money(new(big.Rat).Mul(perToken, big.NewRat(1000000, 1))), true
		}
		input, ok := price("input_cost_per_token")
		if !ok {
			continue
		}
		output, ok := price("output_cost_per_token")
		if !ok {
			continue
		}
		p := modelPrice{Provider: provider, EffectiveFrom: now.UTC().Format(time.RFC3339Nano), Prices: map[string]string{"input": input, "output": output}}
		if provider == "openai" {
			if v, ok := price("cache_read_input_token_cost"); ok {
				p.Prices["cached_input"] = v
			} else {
				continue
			}
		} else {
			for _, part := range []struct{ source, target string }{{"cache_read_input_token_cost", "cache_read"}, {"cache_creation_input_token_cost", "cache_write_5m"}, {"cache_creation_input_token_cost_above_1hr", "cache_write_1h"}} {
				v, ok := price(part.source)
				if !ok {
					p = modelPrice{}
					break
				}
				p.Prices[part.target] = v
			}
			if p.Provider == "" {
				continue
			}
		}
		c.Models[name] = p
	}
	if len(c.Models) == 0 {
		return catalog{}, 0, errors.New("LiteLLM catalog contains no validated direct-provider records")
	}
	return c, len(c.Models), nil
}

type storedEvent struct {
	Event
	runID         sql.NullInt64
	runExact      sql.NullInt64
	runMultiplier sql.NullString
	runProvider   sql.NullString
	sessionStart  sql.NullString
}

func (s *Service) Summary(ctx context.Context) (Summary, error) {
	events, err := s.events(ctx, "", "")
	if err != nil {
		return Summary{}, err
	}
	return s.summarize(ctx, events)
}

func (s *Service) SummaryRange(ctx context.Context, from, to time.Time) (Summary, error) {
	events, err := s.eventsRange(ctx, from, to, "", "")
	if err != nil {
		return Summary{}, err
	}
	return s.summarize(ctx, events)
}

func (s *Service) EarliestEventAt(ctx context.Context) (*time.Time, error) {
	var raw sql.NullString
	if err := s.Store.DB.QueryRowContext(ctx, `SELECT MIN(event_at) FROM usage_events`).Scan(&raw); err != nil {
		return nil, err
	}
	if !raw.Valid {
		return nil, nil
	}
	at, err := time.Parse(time.RFC3339Nano, raw.String)
	if err != nil {
		return nil, err
	}
	return &at, nil
}

type statsAccumulator struct {
	tokens, input, output, cachedRead, cacheWrite int64
	events, priced, unpriced                      int64
	base, provider                                *big.Rat
	complete                                      bool
	sessions                                      map[string]struct{}
	missing                                       map[string]struct{}
}

type statsSessionAccumulator struct {
	*statsAccumulator
	models          map[string]struct{}
	firstAt, lastAt string
}

type statsModelActivityAccumulator struct {
	sessions                                  map[string]struct{}
	days                                      map[string]struct{}
	tools                                     map[string]int64
	firstAt, lastAt                           string
	calls, completed, failed, timed, duration int64
}

func newStatsModelActivityAccumulator() *statsModelActivityAccumulator {
	return &statsModelActivityAccumulator{sessions: map[string]struct{}{}, days: map[string]struct{}{}, tools: map[string]int64{}}
}

func (a *statsModelActivityAccumulator) observe(client, sessionID, at string, location *time.Location) error {
	parsed, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return err
	}
	canonical := parsed.UTC().Format(time.RFC3339Nano)
	a.sessions[client+"\x00"+sessionID] = struct{}{}
	a.days[parsed.In(location).Format("2006-01-02")] = struct{}{}
	if a.firstAt == "" || canonical < a.firstAt {
		a.firstAt = canonical
	}
	if canonical > a.lastAt {
		a.lastAt = canonical
	}
	return nil
}

func (a *statsModelActivityAccumulator) summary() StatsModelActivity {
	tools := make([]StatsToolCount, 0, len(a.tools))
	for name, calls := range a.tools {
		tools = append(tools, StatsToolCount{Name: name, Calls: calls})
	}
	sort.Slice(tools, func(i, j int) bool {
		if tools[i].Calls != tools[j].Calls {
			return tools[i].Calls > tools[j].Calls
		}
		return tools[i].Name < tools[j].Name
	})
	var average *int64
	if a.timed > 0 {
		value := a.duration / a.timed
		average = &value
	}
	return StatsModelActivity{ActiveSessions: int64(len(a.sessions)), ActiveDays: int64(len(a.days)), FirstAt: a.firstAt, LastAt: a.lastAt, ToolCalls: a.calls, CompletedCalls: a.completed, FailedCalls: a.failed, TotalDurationMS: a.duration, AverageDuration: average, Tools: tools}
}

func newStatsAccumulator() *statsAccumulator {
	return &statsAccumulator{base: new(big.Rat), provider: new(big.Rat), complete: true, sessions: map[string]struct{}{}, missing: map[string]struct{}{}}
}

func (a *statsAccumulator) add(event storedEvent, result Result) error {
	a.tokens += eventTokenTotal(event.Client, event.Tokens)
	a.input += event.Tokens["input_tokens"]
	a.output += event.Tokens["output_tokens"]
	if event.Client == "codex" {
		a.cachedRead += event.Tokens["cached_input_tokens"]
	} else {
		a.cachedRead += event.Tokens["cache_read_tokens"]
		cacheWrite := event.Tokens["cache_write_5m_tokens"] + event.Tokens["cache_write_1h_tokens"]
		if cacheWrite == 0 {
			cacheWrite = event.Tokens["cache_creation_tokens"]
		}
		a.cacheWrite += cacheWrite
	}
	a.events++
	a.sessions[event.Client+"\x00"+event.SessionID] = struct{}{}
	for _, component := range result.Unpriced {
		a.missing[component] = struct{}{}
	}
	base, err := decimal(result.KnownCatalogBaseCost)
	if err != nil {
		return err
	}
	providerCost, err := decimal(result.KnownProviderCost)
	if err != nil {
		return err
	}
	a.base.Add(a.base, base)
	a.provider.Add(a.provider, providerCost)
	if result.CatalogBaseCost == nil {
		a.complete = false
		a.unpriced++
		return nil
	}
	a.priced++
	return nil
}

func eventTokenTotal(client string, tokens map[string]int64) int64 {
	if client == "codex" {
		return tokens["input_tokens"] + tokens["output_tokens"]
	}
	cacheWrite := tokens["cache_write_5m_tokens"] + tokens["cache_write_1h_tokens"]
	if cacheWrite == 0 {
		cacheWrite = tokens["cache_creation_tokens"]
	}
	return tokens["input_tokens"] + tokens["output_tokens"] + tokens["cache_read_tokens"] + cacheWrite
}

func statsCoverage(priced, unpriced int64) string {
	total := priced + unpriced
	if total == 0 {
		return "0.00"
	}
	return new(big.Rat).Mul(big.NewRat(priced, total), big.NewRat(100, 1)).FloatString(2)
}

func statsCost(value *statsAccumulator) (*string, string) {
	known := money(value.provider)
	if !value.complete {
		return nil, known
	}
	complete := known
	return &complete, known
}

func statsMetricValue(metric string, value *statsAccumulator) string {
	switch metric {
	case "cost":
		return money(value.provider)
	case "sessions":
		return strconv.Itoa(len(value.sessions))
	default:
		return strconv.FormatInt(value.tokens, 10)
	}
}

func statsMetricValues(metric string, value *statsAccumulator) (*string, string) {
	known := statsMetricValue(metric, value)
	if metric == "cost" && !value.complete {
		return nil, known
	}
	complete := known
	return &complete, known
}

func bucketStart(at time.Time, group string, location *time.Location) time.Time {
	local := at.In(location)
	switch group {
	case "hour":
		elapsed := time.Duration(local.Minute())*time.Minute + time.Duration(local.Second())*time.Second + time.Duration(local.Nanosecond())
		return local.Add(-elapsed)
	case "week":
		day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
		offset := (int(day.Weekday()) + 6) % 7
		return day.AddDate(0, 0, -offset)
	case "month":
		return time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
	default:
		return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	}
}

func nextBucket(at time.Time, group string) time.Time {
	switch group {
	case "hour":
		return at.Add(time.Hour)
	case "week":
		return at.AddDate(0, 0, 7)
	case "month":
		return at.AddDate(0, 1, 0)
	default:
		return at.AddDate(0, 0, 1)
	}
}

func statsMetricRat(metric string, value *statsAccumulator) *big.Rat {
	switch metric {
	case "cost":
		return new(big.Rat).Set(value.provider)
	case "sessions":
		return big.NewRat(int64(len(value.sessions)), 1)
	default:
		return big.NewRat(value.tokens, 1)
	}
}

func statsKnownMetricRat(value string) *big.Rat {
	parsed, err := decimal(value)
	if err != nil {
		return new(big.Rat)
	}
	return parsed
}

func statsDimension(name, client, metric string, value *statsAccumulator, total *big.Rat, totalComplete bool) StatsDimension {
	cost, known := statsCost(value)
	knownShare := "0.00"
	metricValue := statsMetricRat(metric, value)
	if total.Sign() > 0 {
		knownShare = new(big.Rat).Mul(new(big.Rat).Quo(metricValue, total), big.NewRat(100, 1)).FloatString(2)
	}
	metricComplete, metricKnown := statsMetricValues(metric, value)
	var share *string
	if metric != "cost" || value.complete && totalComplete {
		value := knownShare
		share = &value
	}
	clientName := client
	if clientName == "" {
		clientName = name
	}
	logicalInput := value.input
	var cacheHitRate *string
	if clientName == "codex" {
		cacheHitRate = percentPointer(value.cachedRead, value.input)
	} else if clientName == "claude" {
		logicalInput += value.cachedRead + value.cacheWrite
		cacheHitRate = percentPointer(value.cachedRead, logicalInput)
	}
	return StatsDimension{Name: name, Client: client, Tokens: value.tokens, InputTokens: value.input, OutputTokens: value.output, CachedReadTokens: value.cachedRead, CacheWriteTokens: value.cacheWrite, LogicalInputTokens: logicalInput, CacheHitRate: cacheHitRate, Sessions: int64(len(value.sessions)), Events: value.events, ProviderCost: cost, KnownProviderCost: known, MetricValue: metricComplete, KnownMetricValue: metricKnown, Share: share, KnownShare: knownShare, Coverage: statsCoverage(value.priced, value.unpriced)}
}

func percentPointer(numerator, denominator int64) *string {
	value := "0.00"
	if denominator > 0 {
		value = new(big.Rat).Mul(big.NewRat(numerator, denominator), big.NewRat(100, 1)).FloatString(2)
	}
	return &value
}

type statsPriceRow struct {
	catalogEffective time.Time
	modelEffective   time.Time
	order            priceLayerOrder
	price            modelPrice
}

type statsPriceResolver struct {
	current time.Time
	byModel map[string][]statsPriceRow
}

func (s *Service) loadStatsPriceResolver(ctx context.Context, current time.Time) (statsPriceResolver, error) {
	rows, err := s.Store.DB.QueryContext(ctx, `SELECT mp.model,mp.provider,c.effective_from,mp.effective_from,mp.prices_json,mp.aliases_json,c.source_kind,c.imported_at,c.version FROM model_prices mp JOIN price_catalogs c ON c.version=mp.catalog_version`)
	if err != nil {
		return statsPriceResolver{}, err
	}
	defer rows.Close()
	resolver := statsPriceResolver{current: current, byModel: map[string][]statsPriceRow{}}
	for rows.Next() {
		var model, providerName, catalogText, modelText, pricesText, aliasesText, sourceKind, importedText, version string
		if err = rows.Scan(&model, &providerName, &catalogText, &modelText, &pricesText, &aliasesText, &sourceKind, &importedText, &version); err != nil {
			return statsPriceResolver{}, err
		}
		catalogEffective, parseErr := time.Parse(time.RFC3339Nano, catalogText)
		if parseErr != nil {
			return statsPriceResolver{}, parseErr
		}
		modelEffective, parseErr := time.Parse(time.RFC3339Nano, modelText)
		if parseErr != nil {
			return statsPriceResolver{}, parseErr
		}
		importedAt, parseErr := time.Parse(time.RFC3339Nano, importedText)
		if parseErr != nil {
			return statsPriceResolver{}, parseErr
		}
		price := modelPrice{Provider: providerName, Prices: map[string]string{}}
		if err = json.Unmarshal([]byte(pricesText), &price.Prices); err != nil {
			return statsPriceResolver{}, err
		}
		var aliases []string
		if aliasesText != "" && aliasesText != "null" {
			if err = json.Unmarshal([]byte(aliasesText), &aliases); err != nil {
				return statsPriceResolver{}, err
			}
		}
		client := map[string]string{"openai": "codex", "anthropic": "claude"}[providerName]
		if client == "" {
			continue
		}
		seen := map[string]bool{}
		for _, candidate := range append([]string{model}, aliases...) {
			key := providerName + "\x00" + statsPriceModelKey(client, candidate)
			if seen[key] {
				continue
			}
			seen[key] = true
			resolver.byModel[key] = append(resolver.byModel[key], statsPriceRow{
				catalogEffective: catalogEffective,
				modelEffective:   modelEffective,
				order:            priceLayerOrder{sourceKind: sourceKind, catalogEffective: catalogEffective, importedAt: importedAt, version: version},
				price:            price,
			})
		}
	}
	if err = rows.Err(); err != nil {
		return statsPriceResolver{}, err
	}
	for key := range resolver.byModel {
		priceRows := resolver.byModel[key]
		sort.SliceStable(priceRows, func(i, j int) bool { return priceLayerBefore(priceRows[i].order, priceRows[j].order) })
		resolver.byModel[key] = priceRows
	}
	return resolver, nil
}

func statsPriceModelKey(client, model string) string {
	if client == "claude" && strings.HasPrefix(model, "claude-") {
		return strings.ReplaceAll(model, ".", "-")
	}
	return model
}

func (r statsPriceResolver) priceAt(client, model string, at time.Time) (modelPrice, bool) {
	providerName := map[string]string{"codex": "openai", "claude": "anthropic"}[client]
	merged := modelPrice{Provider: providerName, Prices: map[string]string{}}
	rows := r.byModel[providerName+"\x00"+statsPriceModelKey(client, model)]
	for _, row := range rows {
		if row.catalogEffective.After(at) || row.modelEffective.After(at) {
			continue
		}
		for component, value := range row.price.Prices {
			if _, exists := merged.Prices[component]; !exists {
				merged.Prices[component] = value
			}
		}
	}
	return merged, len(merged.Prices) > 0
}

func runtimeProviderName(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func runtimeProviderAt(timeline store.ProviderTimeline, client string, atValue sql.NullString) (string, error) {
	if !atValue.Valid || atValue.String == "" {
		return "unknown", nil
	}
	at, err := time.Parse(time.RFC3339Nano, atValue.String)
	if err != nil {
		return "", err
	}
	snapshot, err := timeline.SnapshotAt(client, at)
	if errors.Is(err, sql.ErrNoRows) {
		return "unknown", nil
	}
	if err != nil {
		return "", err
	}
	return runtimeProviderName(snapshot.Name), nil
}

func (r statsPriceResolver) priceForEvent(event storedEvent, timeline store.ProviderTimeline) (modelPrice, string, string, string, error) {
	quality, multiplierValue, runtimeProvider := "historical", "1", "unknown"
	if event.runID.Valid && event.runExact.Valid && event.runExact.Int64 == 1 {
		if !event.runMultiplier.Valid {
			return modelPrice{}, "", "", "", errors.New("exact usage run has no multiplier")
		}
		quality, multiplierValue = "exact", event.runMultiplier.String
		if event.runProvider.Valid {
			runtimeProvider = runtimeProviderName(event.runProvider.String)
		}
	} else {
		if event.sessionStart.Valid && event.sessionStart.String != "" {
			at, parseErr := time.Parse(time.RFC3339Nano, event.sessionStart.String)
			if parseErr != nil {
				return modelPrice{}, "", "", "", parseErr
			}
			snapshot, snapshotErr := timeline.SnapshotAt(event.Client, at)
			if snapshotErr == nil {
				runtimeProvider = runtimeProviderName(snapshot.Name)
				quality, multiplierValue = "estimated", snapshot.Multiplier
			} else if !errors.Is(snapshotErr, sql.ErrNoRows) {
				return modelPrice{}, "", "", "", snapshotErr
			}
		}
	}
	eventAt, err := time.Parse(time.RFC3339Nano, event.EventAt)
	if err != nil {
		return modelPrice{}, "", "", "", err
	}
	historical, historicalFound := r.priceAt(event.Client, event.Model, eventAt)
	current, currentFound := r.priceAt(event.Client, event.Model, r.current)
	if !historicalFound && !currentFound {
		return modelPrice{Provider: "unknown", Prices: map[string]string{}}, multiplierValue, quality, runtimeProvider, nil
	}
	if !historicalFound {
		return current, multiplierValue, quality, runtimeProvider, nil
	}
	for component, value := range current.Prices {
		if _, exists := historical.Prices[component]; !exists {
			historical.Prices[component] = value
		}
	}
	return historical, multiplierValue, quality, runtimeProvider, nil
}

func (s *Service) Stats(ctx context.Context, options StatsOptions) (StatsReport, error) {
	location := options.Location
	if location == nil {
		location = time.Local
	}
	if !options.From.Before(options.To) {
		return StatsReport{}, errors.New("usage stats range must have from before to")
	}
	if options.GroupBy != "hour" && options.GroupBy != "day" && options.GroupBy != "week" && options.GroupBy != "month" {
		return StatsReport{}, fmt.Errorf("invalid usage stats group-by %q", options.GroupBy)
	}
	if options.Metric != "tokens" && options.Metric != "cost" && options.Metric != "sessions" {
		return StatsReport{}, fmt.Errorf("invalid usage stats metric %q", options.Metric)
	}
	events, err := s.eventsRange(ctx, options.From, options.To, options.Client, options.Model)
	if err != nil {
		return StatsReport{}, err
	}
	resolver, err := s.loadStatsPriceResolver(ctx, s.now())
	if err != nil {
		return StatsReport{}, err
	}
	timeline, err := s.Store.LoadProviderTimeline(ctx)
	if err != nil {
		return StatsReport{}, err
	}
	total := newStatsAccumulator()
	buckets := map[string]*statsAccumulator{}
	models := map[string]*statsAccumulator{}
	clients := map[string]*statsAccumulator{}
	providers := map[string]*statsAccumulator{}
	sessions := map[string]*statsSessionAccumulator{}
	modelActivity := map[string]*statsModelActivityAccumulator{}
	activity := map[[2]int]*statsAccumulator{}
	for _, event := range events {
		at, parseErr := time.Parse(time.RFC3339Nano, event.EventAt)
		if parseErr != nil {
			return StatsReport{}, parseErr
		}
		price, multiplierValue, _, runtimeProvider, priceErr := resolver.priceForEvent(event, timeline)
		if priceErr != nil {
			return StatsReport{}, priceErr
		}
		if options.Provider != "" && runtimeProvider != options.Provider {
			continue
		}
		result, calculateErr := Calculate(event.Client, event.Model, event.Tokens, price, multiplierValue)
		if calculateErr != nil {
			return StatsReport{}, calculateErr
		}
		start := bucketStart(at, options.GroupBy, location)
		key := start.Format(time.RFC3339Nano)
		if buckets[key] == nil {
			buckets[key] = newStatsAccumulator()
		}
		modelKey := event.Client + "\x00" + event.Model
		if models[modelKey] == nil {
			models[modelKey] = newStatsAccumulator()
		}
		if clients[event.Client] == nil {
			clients[event.Client] = newStatsAccumulator()
		}
		providerKey := event.Client + "\x00" + runtimeProvider
		if providers[providerKey] == nil {
			providers[providerKey] = newStatsAccumulator()
		}
		sessionKey := event.Client + "\x00" + event.SessionID
		if sessions[sessionKey] == nil {
			sessions[sessionKey] = &statsSessionAccumulator{statsAccumulator: newStatsAccumulator(), models: map[string]struct{}{}}
		}
		if modelActivity[modelKey] == nil {
			modelActivity[modelKey] = newStatsModelActivityAccumulator()
		}
		local := at.In(location)
		activityKey := [2]int{(int(local.Weekday()) + 6) % 7, local.Hour()}
		if activity[activityKey] == nil {
			activity[activityKey] = newStatsAccumulator()
		}
		for _, accumulator := range []*statsAccumulator{total, buckets[key], models[modelKey], clients[event.Client], providers[providerKey], activity[activityKey]} {
			if err = accumulator.add(event, result); err != nil {
				return StatsReport{}, err
			}
		}
		if err = sessions[sessionKey].add(event, result); err != nil {
			return StatsReport{}, err
		}
		sessions[sessionKey].models[event.Model] = struct{}{}
		if sessions[sessionKey].firstAt == "" || event.EventAt < sessions[sessionKey].firstAt {
			sessions[sessionKey].firstAt = event.EventAt
		}
		if event.EventAt > sessions[sessionKey].lastAt {
			sessions[sessionKey].lastAt = event.EventAt
		}
		if err = modelActivity[modelKey].observe(event.Client, event.SessionID, event.EventAt, location); err != nil {
			return StatsReport{}, err
		}
	}
	toolQuery := `SELECT t.client,t.session_id,t.model,t.tool_name,t.started_at,t.status,t.duration_ms,us.first_at FROM usage_tool_calls t LEFT JOIN usage_sessions us ON us.client=t.client AND us.session_id=t.session_id WHERE t.started_at>=? AND t.started_at<?`
	toolArgs := []any{options.From.UTC().Format(time.RFC3339Nano), options.To.UTC().Format(time.RFC3339Nano)}
	if options.Client != "" {
		toolQuery += ` AND t.client=?`
		toolArgs = append(toolArgs, options.Client)
	}
	if options.Model != "" {
		toolQuery += ` AND t.model=?`
		toolArgs = append(toolArgs, options.Model)
	}
	toolQuery += ` ORDER BY t.started_at,t.activity_key`
	toolRows, err := s.Store.DB.QueryContext(ctx, toolQuery, toolArgs...)
	if err != nil {
		return StatsReport{}, err
	}
	for toolRows.Next() {
		var client, sessionID, model, tool, startedAt, status string
		var duration sql.NullInt64
		var sessionStart sql.NullString
		if err = toolRows.Scan(&client, &sessionID, &model, &tool, &startedAt, &status, &duration, &sessionStart); err != nil {
			toolRows.Close()
			return StatsReport{}, err
		}
		if model == "" {
			continue
		}
		if options.Provider != "" {
			// Unlike token events, tool calls have no run binding; use the session-start snapshot as a session-level approximation.
			provider, providerErr := runtimeProviderAt(timeline, client, sessionStart)
			if providerErr != nil {
				toolRows.Close()
				return StatsReport{}, providerErr
			}
			if provider != options.Provider {
				continue
			}
		}
		modelKey := client + "\x00" + model
		if models[modelKey] == nil {
			models[modelKey] = newStatsAccumulator()
		}
		if modelActivity[modelKey] == nil {
			modelActivity[modelKey] = newStatsModelActivityAccumulator()
		}
		value := modelActivity[modelKey]
		if err = value.observe(client, sessionID, startedAt, location); err != nil {
			toolRows.Close()
			return StatsReport{}, err
		}
		value.calls++
		value.tools[tool]++
		switch status {
		case "completed":
			value.completed++
		case "failed":
			value.failed++
		}
		if duration.Valid {
			value.timed++
			value.duration += duration.Int64
		}
	}
	if err = toolRows.Err(); err != nil {
		toolRows.Close()
		return StatsReport{}, err
	}
	if err = toolRows.Close(); err != nil {
		return StatsReport{}, err
	}
	timezone := options.Timezone
	if timezone == "" {
		timezone = location.String()
	}
	report := StatsReport{
		Range:    StatsRange{From: options.From.In(location).Format(time.RFC3339Nano), To: options.To.In(location).Format(time.RFC3339Nano)},
		Timezone: timezone, GroupBy: options.GroupBy, Metric: options.Metric,
		Buckets: []StatsBucket{}, Models: []StatsDimension{}, Clients: []StatsDimension{}, Providers: []StatsDimension{}, CacheSessions: []StatsCacheSession{}, Activity: []StatsActivity{}, UnpricedModels: []StatsUnpricedModel{}, ShowModelActivity: options.Activity,
		Coverage: StatsCoverage{PricedEvents: total.priced, UnpricedEvents: total.unpriced, TotalEvents: total.events, Percent: statsCoverage(total.priced, total.unpriced)},
	}
	completeProvider, _ := statsCost(total)
	knownBase := money(total.base)
	var completeBase *string
	if total.complete {
		value := knownBase
		completeBase = &value
	}
	zeroCost := "0.000000000"
	report.Totals = StatsTotals{Tokens: total.tokens, InputTokens: total.input, OutputTokens: total.output, CachedReadTokens: total.cachedRead, CacheWriteTokens: total.cacheWrite, Sessions: int64(len(total.sessions)), Events: total.events, CatalogBaseCost: completeBase, ProviderCost: completeProvider, KnownCatalogBaseCost: knownBase, KnownProviderCost: money(total.provider), AverageTokens: "0.00", AverageCost: &zeroCost, KnownAverageCost: zeroCost}
	if len(total.sessions) > 0 {
		report.Totals.AverageTokens = new(big.Rat).Quo(big.NewRat(total.tokens, 1), big.NewRat(int64(len(total.sessions)), 1)).FloatString(2)
		report.Totals.KnownAverageCost = money(new(big.Rat).Quo(total.provider, big.NewRat(int64(len(total.sessions)), 1)))
		if total.complete {
			value := report.Totals.KnownAverageCost
			report.Totals.AverageCost = &value
		} else {
			report.Totals.AverageCost = nil
		}
	}
	start := bucketStart(options.From, options.GroupBy, location)
	if start.Before(options.From) && options.GroupBy == "hour" {
		// Keep the first partial hour; its start is useful to chart consumers.
	}
	peakValue := new(big.Rat)
	for count := 0; start.Before(options.To); count++ {
		if count >= 10000 {
			return StatsReport{}, errors.New("usage stats range produces too many buckets")
		}
		end := nextBucket(start, options.GroupBy)
		value := buckets[start.Format(time.RFC3339Nano)]
		if value == nil {
			value = newStatsAccumulator()
		}
		cost, known := statsCost(value)
		metricComplete, metricKnown := statsMetricValues(options.Metric, value)
		bucket := StatsBucket{Start: start.Format(time.RFC3339Nano), End: end.Format(time.RFC3339Nano), Tokens: value.tokens, InputTokens: value.input, OutputTokens: value.output, CachedReadTokens: value.cachedRead, CacheWriteTokens: value.cacheWrite, Sessions: int64(len(value.sessions)), Events: value.events, ProviderCost: cost, KnownProviderCost: known, MetricValue: metricComplete, KnownMetricValue: metricKnown, Coverage: statsCoverage(value.priced, value.unpriced)}
		report.Buckets = append(report.Buckets, bucket)
		candidate := statsMetricRat(options.Metric, value)
		if len(report.Buckets) == 1 || candidate.Cmp(peakValue) > 0 {
			peakValue.Set(candidate)
			report.Peak = StatsPeak{Start: bucket.Start, End: bucket.End, Value: bucket.MetricValue, KnownValue: bucket.KnownMetricValue, Coverage: bucket.Coverage}
		}
		start = end
	}
	totalMetric := statsMetricRat(options.Metric, total)
	for key, value := range models {
		parts := strings.SplitN(key, "\x00", 2)
		dimension := statsDimension(parts[1], parts[0], options.Metric, value, totalMetric, total.complete)
		if modelActivity[key] != nil {
			summary := modelActivity[key].summary()
			dimension.Activity = &summary
		}
		report.Models = append(report.Models, dimension)
		if len(value.missing) > 0 {
			missing := make([]string, 0, len(value.missing))
			for component := range value.missing {
				missing = append(missing, component)
			}
			sort.Strings(missing)
			report.UnpricedModels = append(report.UnpricedModels, StatsUnpricedModel{Client: parts[0], Model: parts[1], Components: missing})
		}
	}
	for name, value := range clients {
		report.Clients = append(report.Clients, statsDimension(name, "", options.Metric, value, totalMetric, total.complete))
	}
	for key, value := range providers {
		parts := strings.SplitN(key, "\x00", 2)
		report.Providers = append(report.Providers, statsDimension(parts[1], parts[0], options.Metric, value, totalMetric, total.complete))
	}
	for key, value := range sessions {
		if value.cachedRead == 0 && value.cacheWrite == 0 {
			continue
		}
		parts := strings.SplitN(key, "\x00", 2)
		models := make([]string, 0, len(value.models))
		for model := range value.models {
			models = append(models, model)
		}
		sort.Strings(models)
		logicalInput := value.input
		if parts[0] == "claude" {
			logicalInput += value.cachedRead + value.cacheWrite
		}
		report.CacheSessions = append(report.CacheSessions, StatsCacheSession{Client: parts[0], SessionID: parts[1], Models: models, InputTokens: value.input, OutputTokens: value.output, CachedReadTokens: value.cachedRead, CacheWriteTokens: value.cacheWrite, LogicalInputTokens: logicalInput, CacheHitRate: percentPointer(value.cachedRead, logicalInput), Events: value.events, FirstAt: value.firstAt, LastAt: value.lastAt, DetailCommand: fmt.Sprintf("agentdeck session show %s --client %s --activity", parts[1], parts[0])})
	}
	sort.Slice(report.Models, func(i, j int) bool {
		left := statsKnownMetricRat(report.Models[i].KnownMetricValue)
		right := statsKnownMetricRat(report.Models[j].KnownMetricValue)
		if comparison := left.Cmp(right); comparison != 0 {
			return comparison > 0
		}
		if report.Models[i].Client == report.Models[j].Client {
			return report.Models[i].Name < report.Models[j].Name
		}
		return report.Models[i].Client < report.Models[j].Client
	})
	sort.Slice(report.Providers, func(i, j int) bool {
		left := statsKnownMetricRat(report.Providers[i].KnownMetricValue)
		right := statsKnownMetricRat(report.Providers[j].KnownMetricValue)
		if comparison := left.Cmp(right); comparison != 0 {
			return comparison > 0
		}
		if report.Providers[i].Client == report.Providers[j].Client {
			return report.Providers[i].Name < report.Providers[j].Name
		}
		return report.Providers[i].Client < report.Providers[j].Client
	})
	sort.Slice(report.Clients, func(i, j int) bool { return report.Clients[i].Name < report.Clients[j].Name })
	sort.Slice(report.CacheSessions, func(i, j int) bool {
		if report.CacheSessions[i].CachedReadTokens != report.CacheSessions[j].CachedReadTokens {
			return report.CacheSessions[i].CachedReadTokens > report.CacheSessions[j].CachedReadTokens
		}
		if report.CacheSessions[i].LogicalInputTokens != report.CacheSessions[j].LogicalInputTokens {
			return report.CacheSessions[i].LogicalInputTokens > report.CacheSessions[j].LogicalInputTokens
		}
		if report.CacheSessions[i].Client != report.CacheSessions[j].Client {
			return report.CacheSessions[i].Client < report.CacheSessions[j].Client
		}
		return report.CacheSessions[i].SessionID < report.CacheSessions[j].SessionID
	})
	sort.Slice(report.UnpricedModels, func(i, j int) bool {
		if report.UnpricedModels[i].Client == report.UnpricedModels[j].Client {
			return report.UnpricedModels[i].Model < report.UnpricedModels[j].Model
		}
		return report.UnpricedModels[i].Client < report.UnpricedModels[j].Client
	})
	if options.GroupBy != "hour" && naturalDayCount(options.From, options.To, location) >= 7 {
		for weekday := 0; weekday < 7; weekday++ {
			for hour := 0; hour < 24; hour++ {
				value := activity[[2]int{weekday, hour}]
				if value == nil {
					value = newStatsAccumulator()
				}
				metricComplete, metricKnown := statsMetricValues(options.Metric, value)
				report.Activity = append(report.Activity, StatsActivity{Weekday: weekday, Hour: hour, Tokens: value.tokens, Sessions: int64(len(value.sessions)), Events: value.events, KnownCost: money(value.provider), MetricValue: metricComplete, KnownMetricValue: metricKnown})
			}
		}
	}
	return report, nil
}

func naturalDayCount(from, to time.Time, location *time.Location) int {
	start := from.In(location)
	end := to.Add(-time.Nanosecond).In(location)
	startDate := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, location)
	endDate := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, location)
	days := 1
	for startDate.Before(endDate) {
		startDate = startDate.AddDate(0, 0, 1)
		days++
	}
	return days
}
func (s *Service) Sessions(ctx context.Context) ([]SessionSummary, error) {
	rows, err := s.Store.DB.QueryContext(ctx, `SELECT client,session_id,first_at,last_at FROM usage_sessions ORDER BY first_at DESC, client, session_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SessionSummary, 0)
	for rows.Next() {
		var item SessionSummary
		if err := rows.Scan(&item.Client, &item.SessionID, &item.FirstAt, &item.LastAt); err != nil {
			return nil, err
		}
		events, e := s.events(ctx, item.Client, item.SessionID)
		if e != nil {
			return nil, e
		}
		total, e := s.summarize(ctx, events)
		if e != nil {
			return nil, e
		}
		item.Tokens = total.Tokens
		item.CatalogBaseCost = total.CatalogBaseCost
		item.ProviderCost = total.ProviderCost
		item.KnownCatalogBaseCost = total.KnownCatalogBaseCost
		item.KnownProviderCost = total.KnownProviderCost
		item.Unpriced = total.Unpriced
		item.Warnings = total.Warnings
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Service) events(ctx context.Context, client, session string) ([]storedEvent, error) {
	q := `SELECT e.event_key,e.client,e.session_id,e.event_id,e.event_at,e.model,e.input_tokens,e.cached_input_tokens,e.output_tokens,e.cache_read_tokens,e.cache_creation_tokens,e.cache_write_5m_tokens,e.cache_write_1h_tokens,e.source_path,e.source_offset,COALESCE(b.run_id,e.run_id),r.exact,r.multiplier,us.first_at FROM usage_events e LEFT JOIN usage_run_bindings b ON b.event_key=e.event_key LEFT JOIN usage_runs r ON r.id=COALESCE(b.run_id,e.run_id) LEFT JOIN usage_sessions us ON us.client=e.client AND us.session_id=e.session_id`
	args := []any{}
	where := []string{}
	if client != "" {
		where = append(where, "e.client=?")
		args = append(args, client)
	}
	if session != "" {
		where = append(where, "e.session_id=?")
		args = append(args, session)
	}
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY e.event_at,e.event_key"
	rows, err := s.Store.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []storedEvent{}
	for rows.Next() {
		var e storedEvent
		var in, cached, outTokens, read, creation, write5, write1 int64
		err = rows.Scan(&e.Key, &e.Client, &e.SessionID, &e.EventID, &e.EventAt, &e.Model, &in, &cached, &outTokens, &read, &creation, &write5, &write1, &e.SourcePath, &e.SourceOffset, &e.runID, &e.runExact, &e.runMultiplier, &e.sessionStart)
		if err != nil {
			return nil, err
		}
		e.Tokens = map[string]int64{"input_tokens": in, "cached_input_tokens": cached, "output_tokens": outTokens, "cache_read_tokens": read, "cache_creation_tokens": creation, "cache_write_5m_tokens": write5, "cache_write_1h_tokens": write1}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Service) eventsRange(ctx context.Context, from, to time.Time, client, model string) ([]storedEvent, error) {
	query := `SELECT e.event_key,e.client,e.session_id,e.event_id,e.event_at,e.model,e.input_tokens,e.cached_input_tokens,e.output_tokens,e.cache_read_tokens,e.cache_creation_tokens,e.cache_write_5m_tokens,e.cache_write_1h_tokens,e.source_path,e.source_offset,COALESCE(b.run_id,e.run_id),r.exact,r.multiplier,r.provider,us.first_at FROM usage_events e LEFT JOIN usage_run_bindings b ON b.event_key=e.event_key LEFT JOIN usage_runs r ON r.id=COALESCE(b.run_id,e.run_id) LEFT JOIN usage_sessions us ON us.client=e.client AND us.session_id=e.session_id WHERE e.event_at>=? AND e.event_at<?`
	args := []any{from.UTC().Format(time.RFC3339Nano), to.UTC().Format(time.RFC3339Nano)}
	if client != "" {
		query += ` AND e.client=?`
		args = append(args, client)
	}
	if model != "" {
		query += ` AND e.model=?`
		args = append(args, model)
	}
	query += ` ORDER BY e.event_at,e.event_key`
	rows, err := s.Store.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []storedEvent{}
	for rows.Next() {
		var event storedEvent
		var input, cached, output, read, creation, write5, write1 int64
		if err = rows.Scan(&event.Key, &event.Client, &event.SessionID, &event.EventID, &event.EventAt, &event.Model, &input, &cached, &output, &read, &creation, &write5, &write1, &event.SourcePath, &event.SourceOffset, &event.runID, &event.runExact, &event.runMultiplier, &event.runProvider, &event.sessionStart); err != nil {
			return nil, err
		}
		event.Tokens = map[string]int64{"input_tokens": input, "cached_input_tokens": cached, "output_tokens": output, "cache_read_tokens": read, "cache_creation_tokens": creation, "cache_write_5m_tokens": write5, "cache_write_1h_tokens": write1}
		out = append(out, event)
	}
	return out, rows.Err()
}

func (s *Service) summarize(ctx context.Context, events []storedEvent) (Summary, error) {
	out := Summary{Tokens: map[string]int64{}, Counts: map[string]int64{"events": int64(len(events)), "exact": 0, "estimated": 0, "historical": 0, "priced": 0, "unpriced": 0}, Models: []ModelCoverage{}, Unpriced: []string{}, Warnings: []string{}}
	base := new(big.Rat)
	provider := new(big.Rat)
	complete := true
	warned := map[string]bool{}
	unpriced := map[string]bool{}
	coverage := map[string]*ModelCoverage{}
	for _, e := range events {
		coverageKey := e.Client + "\x00" + e.Model
		model := coverage[coverageKey]
		if model == nil {
			model = &ModelCoverage{Client: e.Client, Model: e.Model}
			coverage[coverageKey] = model
		}
		model.Events++
		for k, v := range e.Tokens {
			out.Tokens[k] += v
		}
		price, mult, quality, err := s.priceForEvent(ctx, e)
		if err != nil {
			return out, err
		}
		out.Counts[quality]++
		if quality != "exact" && !warned[quality] {
			out.Warnings = append(out.Warnings, quality+" attribution")
			warned[quality] = true
		}
		r, err := Calculate(e.Client, e.Model, e.Tokens, price, mult)
		if err != nil {
			return out, err
		}
		knownBase, _ := decimal(r.KnownCatalogBaseCost)
		knownProvider, _ := decimal(r.KnownProviderCost)
		base.Add(base, knownBase)
		provider.Add(provider, knownProvider)
		if r.CatalogBaseCost == nil {
			complete = false
			out.Counts["unpriced"]++
			model.UnpricedEvents++
			for _, u := range r.Unpriced {
				unpriced[u] = true
			}
			continue
		}
		out.Counts["priced"]++
		model.PricedEvents++
	}
	knownBase, knownProvider := money(base), money(provider)
	out.KnownCatalogBaseCost = &knownBase
	out.KnownProviderCost = &knownProvider
	if complete {
		out.CatalogBaseCost = &knownBase
		out.ProviderCost = &knownProvider
	}
	for u := range unpriced {
		out.Unpriced = append(out.Unpriced, u)
	}
	sort.Strings(out.Unpriced)
	for _, model := range coverage {
		out.Models = append(out.Models, *model)
	}
	sort.Slice(out.Models, func(i, j int) bool {
		if out.Models[i].Client == out.Models[j].Client {
			return out.Models[i].Model < out.Models[j].Model
		}
		return out.Models[i].Client < out.Models[j].Client
	})
	return out, nil
}
func (s *Service) priceForEvent(ctx context.Context, e storedEvent) (modelPrice, string, string, error) {
	quality, mult := "historical", "1"
	if e.runID.Valid && e.runExact.Valid && e.runExact.Int64 == 1 {
		quality = "exact"
		if err := s.Store.DB.QueryRowContext(ctx, "SELECT multiplier FROM usage_runs WHERE id=?", e.runID.Int64).Scan(&mult); err != nil {
			return modelPrice{}, "", "", err
		}
	} else {
		// File-only attribution belongs to the provider selected when the logical
		// session began, not a later provider selected mid-session.
		var sessionStart string
		_ = s.Store.DB.QueryRowContext(ctx, `SELECT first_at FROM usage_sessions WHERE client=? AND session_id=?`, e.Client, e.SessionID).Scan(&sessionStart)
		if sessionStart == "" {
			sessionStart = e.EventAt
		}
		at, parseErr := time.Parse(time.RFC3339Nano, sessionStart)
		if parseErr != nil {
			return modelPrice{}, "", "", parseErr
		}
		snapshot, err := s.Store.ProviderSnapshotAt(ctx, e.Client, at)
		if err == nil {
			mult = snapshot.Multiplier
			quality = "estimated"
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return modelPrice{}, "", "", err
		}
	}
	historical, historicalFound, err := s.mergedPriceAt(ctx, e.Client, e.Model, e.EventAt)
	if err != nil {
		return modelPrice{}, "", "", err
	}
	current, currentFound, err := s.mergedPriceAt(ctx, e.Client, e.Model, s.now().Format(time.RFC3339Nano))
	if err != nil {
		return modelPrice{}, "", "", err
	}
	if !historicalFound && !currentFound {
		return modelPrice{Provider: "unknown", Prices: map[string]string{}}, mult, quality, nil
	}
	if !historicalFound {
		return current, mult, quality, nil
	}
	// Current prices fill only components missing from the historical result.
	// A component that was calculable at event time is never repriced.
	for component, value := range current.Prices {
		if _, exists := historical.Prices[component]; !exists {
			historical.Prices[component] = value
		}
	}
	return historical, mult, quality, nil
}

func (s *Service) mergedPriceAt(ctx context.Context, client, eventModel, at string) (modelPrice, bool, error) {
	target, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return modelPrice{}, false, err
	}
	rows, err := s.Store.DB.QueryContext(ctx, `SELECT mp.model,mp.provider,mp.effective_from,mp.prices_json,mp.aliases_json,c.source_kind,c.effective_from,c.imported_at,c.version FROM model_prices mp JOIN price_catalogs c ON c.version=mp.catalog_version`)
	if err != nil {
		return modelPrice{}, false, err
	}
	defer rows.Close()
	type row struct {
		model, aliases string
		price          modelPrice
		modelEffective time.Time
		order          priceLayerOrder
	}
	var priceRows []row
	for rows.Next() {
		var item row
		var raw, modelText, sourceKind, catalogText, importedText, version string
		if err = rows.Scan(&item.model, &item.price.Provider, &modelText, &raw, &item.aliases, &sourceKind, &catalogText, &importedText, &version); err != nil {
			return modelPrice{}, false, err
		}
		item.price.EffectiveFrom = modelText
		item.modelEffective, err = time.Parse(time.RFC3339Nano, modelText)
		if err != nil {
			return modelPrice{}, false, err
		}
		item.order = priceLayerOrder{sourceKind: sourceKind, version: version}
		item.order.catalogEffective, err = time.Parse(time.RFC3339Nano, catalogText)
		if err != nil {
			return modelPrice{}, false, err
		}
		item.order.importedAt, err = time.Parse(time.RFC3339Nano, importedText)
		if err != nil {
			return modelPrice{}, false, err
		}
		if item.order.catalogEffective.After(target) || item.modelEffective.After(target) {
			continue
		}
		if err = json.Unmarshal([]byte(raw), &item.price.Prices); err != nil {
			return modelPrice{}, false, err
		}
		priceRows = append(priceRows, item)
	}
	if err = rows.Err(); err != nil {
		return modelPrice{}, false, err
	}
	sort.SliceStable(priceRows, func(i, j int) bool { return priceLayerBefore(priceRows[i].order, priceRows[j].order) })
	expected := map[string]string{"codex": "openai", "claude": "anthropic"}[client]
	merged := modelPrice{Provider: expected, Prices: map[string]string{}}
	found := false
	for _, priceRow := range priceRows {
		if priceRow.price.Provider != expected {
			continue
		}
		matches := usageModelMatches(client, priceRow.model, eventModel)
		var a []string
		_ = json.Unmarshal([]byte(priceRow.aliases), &a)
		for _, x := range a {
			if usageModelMatches(client, x, eventModel) {
				matches = true
			}
		}
		if !matches {
			continue
		}
		// Rows are newest-first. Preserve the first value of each component so an
		// official layer can override only one verified component.
		for k, v := range priceRow.price.Prices {
			if _, ok := merged.Prices[k]; !ok {
				merged.Prices[k] = v
			}
		}
		found = true
	}
	return merged, found, nil
}

func usageModelMatches(client, catalogModel, eventModel string) bool {
	if catalogModel == eventModel {
		return true
	}
	if client != "claude" || !strings.HasPrefix(catalogModel, "claude-") || !strings.HasPrefix(eventModel, "claude-") {
		return false
	}
	return strings.ReplaceAll(catalogModel, ".", "-") == strings.ReplaceAll(eventModel, ".", "-")
}
func ParseMultiplier(v string) (string, error) {
	r, e := decimal(v)
	if e != nil {
		return "", e
	}
	return r.FloatString(max(0, 9)), nil
}
func ParseInt(v string) (int64, error) { return strconv.ParseInt(v, 10, 64) }
