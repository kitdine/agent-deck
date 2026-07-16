// Package usage imports read-only client usage logs and calculates catalog costs.
package usage

import (
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

	"github.com/kitdine/agent-deck/internal/store"
)

//go:embed model-prices.json
var bundledCatalog []byte

const (
	bundledCatalogSourceURL       = "bundled://agentdeck/model-prices.json"
	legacyBundledCatalogSourceURL = "bundled://config/model-prices.json"
)

var tokenNames = []string{"input_tokens", "cached_input_tokens", "output_tokens", "cache_read_tokens", "cache_creation_tokens", "cache_write_5m_tokens", "cache_write_1h_tokens"}

type Event struct {
	Key, Client, SessionID, EventID, EventAt, Model, SourcePath string
	SourceOffset                                                int64
	Tokens                                                      map[string]int64
}
type Result struct {
	Tokens          map[string]int64 `json:"tokens"`
	CatalogBaseCost *string          `json:"catalog_base_cost"`
	ProviderCost    *string          `json:"provider_cost"`
	Unpriced        []string         `json:"unpriced_components"`
}
type Summary struct {
	Tokens          map[string]int64 `json:"tokens"`
	Counts          map[string]int64 `json:"counts"`
	CatalogBaseCost *string          `json:"catalog_base_cost"`
	ProviderCost    *string          `json:"provider_cost"`
	Unpriced        []string         `json:"unpriced_components"`
	Warnings        []string         `json:"warnings"`
}
type SessionSummary struct {
	Client          string           `json:"client"`
	SessionID       string           `json:"session_id"`
	FirstAt         string           `json:"first_at"`
	LastAt          string           `json:"last_at"`
	Tokens          map[string]int64 `json:"tokens"`
	CatalogBaseCost *string          `json:"catalog_base_cost"`
	ProviderCost    *string          `json:"provider_cost"`
	Unpriced        []string         `json:"unpriced_components"`
	Warnings        []string         `json:"warnings"`
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
		return Result{Tokens: tokens, Unpriced: []string{"unknown_model"}}, nil
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
	if len(unpriced) > 0 {
		return Result{Tokens: tokens, Unpriced: unpriced}, nil
	}
	b := money(base)
	f := money(new(big.Rat).Mul(base, m))
	return Result{Tokens: tokens, CatalogBaseCost: &b, ProviderCost: &f}, nil
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
	return s.importCatalog(ctx, bundledCatalog, "bundled", bundledCatalogSourceURL, "", effective)
}
func (s *Service) importCatalog(ctx context.Context, data []byte, kind, url, commit string, effective time.Time) error {
	c, err := parseCatalog(data)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	tx, err := s.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO price_catalogs(version,source_kind,source_url,commit_sha,content_sha256,imported_at,effective_from,currency,schema_version) VALUES(?,?,?,?,?,?,?,?,?)`, c.Version, kind, url, commit, hex.EncodeToString(hash[:]), s.now().Format(time.RFC3339Nano), effective.Format(time.RFC3339Nano), c.Currency, c.SchemaVersion)
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
	return s.importCatalog(ctx, encoded, "official", source, "", earliestOverride(overrides))
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
	out := map[string]int{"files": 0, "imported": 0, "updated": 0, "ignored_non_usage": 0, "unsupported_usage": 0, "malformed": 0, "source_resets": 0, "replaced": 0, "unsupported": 0}
	out["files"] = len(inventory.Entries)
	for _, path := range inventory.Removed {
		if err := s.removeSource(ctx, path); err != nil {
			return nil, err
		}
	}
	changed := make(map[string]bool, len(inventory.Added)+len(inventory.Appended)+len(inventory.Mutated))
	for _, paths := range [][]string{inventory.Added, inventory.Appended, inventory.Mutated} {
		for _, path := range paths {
			changed[path] = true
		}
	}
	for _, entry := range inventory.Entries {
		if !changed[entry.Path] {
			continue
		}
		stats, err := s.scanFile(ctx, entry)
		if err != nil {
			return nil, err
		}
		for key, value := range stats {
			out[key] += value
		}
	}
	if err := s.Store.SetSetting(ctx, "watch.fingerprint.usage", inventory.Fingerprint); err != nil {
		return nil, err
	}
	return out, nil
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
	rows, err := s.Store.DB.QueryContext(ctx, "SELECT path,identity,size,cursor,modified_at FROM usage_source_files")
	if err != nil {
		return Inventory{}, err
	}
	defer rows.Close()
	type storedEntry struct {
		identity               string
		size, cursor, modified int64
	}
	stored := map[string]storedEntry{}
	for rows.Next() {
		var path string
		var item storedEntry
		if err = rows.Scan(&path, &item.identity, &item.size, &item.cursor, &item.modified); err != nil {
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
	r := map[string]int{"imported": 0, "updated": 0, "ignored_non_usage": 0, "unsupported_usage": 0, "malformed": 0, "source_resets": 0, "replaced": 0, "unsupported": 0}
	path, client := entry.Path, entry.Client
	var cursor, oldSize, oldModified int64
	var oldIdentity, oldHash string
	state := parseState{}
	row := s.Store.DB.QueryRowContext(ctx, "SELECT cursor,identity,size,modified_at,prefix_hash,COALESCE(session_id,''),COALESCE(turn_id,''),COALESCE(model,'') FROM usage_source_files WHERE path=?", path)
	loadErr := row.Scan(&cursor, &oldIdentity, &oldSize, &oldModified, &oldHash, &state.session, &state.turn, &state.model)
	found := loadErr == nil
	if loadErr != nil && !errors.Is(loadErr, sql.ErrNoRows) {
		return r, loadErr
	}
	if found && oldIdentity == entry.Identity && oldSize == entry.Size && oldModified == entry.ModifiedAt {
		return r, nil
	}
	file, err := s.open(path)
	if err != nil {
		return r, err
	}
	defer file.Close()
	appendOnly := found && oldIdentity == entry.Identity && entry.Size >= cursor
	if appendOnly && cursor > 0 {
		start := max(int64(0), cursor-4096)
		anchor := make([]byte, cursor-start)
		if _, err = io.ReadFull(io.NewSectionReader(file, start, int64(len(anchor))), anchor); err != nil {
			return r, err
		}
		appendOnly = hash(anchor) == oldHash
	}
	reset := !appendOnly && found
	if reset {
		cursor = 0
		state = parseState{}
		r["source_resets"]++
		r["replaced"]++
	}
	data, err := io.ReadAll(io.NewSectionReader(file, cursor, entry.Size-cursor))
	if err != nil {
		return r, err
	}
	events := make([]Event, 0)
	offset, line := cursor, data
	for len(line) > 0 {
		idx := strings.IndexByte(string(line), '\n')
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
		if !ok {
			if looksLikeUsage(client, value) {
				r["unsupported_usage"]++
				r["unsupported"]++
			} else {
				r["ignored_non_usage"]++
			}
		} else {
			events = append(events, ev)
		}
		offset += next
		line = line[idx+1:]
	}
	latest, err := s.stat(path)
	if err != nil {
		return r, err
	}
	if usageFileIdentity(latest) != entry.Identity || latest.Size() != entry.Size || latest.ModTime().UnixNano() != entry.ModifiedAt {
		return r, errors.New("usage source changed during inventory scan")
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
	if reset {
		if _, err = tx.ExecContext(ctx, "DELETE FROM usage_events WHERE source_path=?", path); err != nil {
			return r, err
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM usage_run_sources WHERE path=?", path); err != nil {
			return r, err
		}
	}
	for _, event := range events {
		affected[event.Client+"\x00"+event.SessionID] = [2]string{event.Client, event.SessionID}
		inserted, upsertErr := upsertTx(ctx, tx, event)
		if upsertErr != nil {
			return r, upsertErr
		}
		if inserted {
			r["imported"]++
		} else {
			r["updated"]++
			r["replaced"]++
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO usage_source_files(path,identity,size,cursor,prefix_hash,session_id,turn_id,model,imported,replaced,malformed,unsupported,modified_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(path) DO UPDATE SET identity=excluded.identity,size=excluded.size,cursor=excluded.cursor,prefix_hash=excluded.prefix_hash,session_id=excluded.session_id,turn_id=excluded.turn_id,model=excluded.model,imported=usage_source_files.imported+excluded.imported,replaced=usage_source_files.replaced+excluded.replaced,malformed=usage_source_files.malformed+excluded.malformed,unsupported=usage_source_files.unsupported+excluded.unsupported,modified_at=excluded.modified_at`, path, entry.Identity, entry.Size, cursor, hash(anchor), state.session, state.turn, state.model, r["imported"], r["replaced"], r["malformed"], r["unsupported"], entry.ModifiedAt)
	if err != nil {
		return r, err
	}
	if err = rebuildSessions(ctx, tx, affected); err != nil {
		return r, err
	}
	return r, tx.Commit()
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

type parseState struct{ session, turn, model string }

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
		u, _ := info["last_token_usage"].(map[string]any)
		if u == nil || !validTokenFields(u, "input_tokens", "cached_input_tokens", "output_tokens") {
			return Event{}, false
		}
		return Event{Key: "codex:" + state.session + ":" + state.turn, Client: client, SessionID: state.session, EventID: state.turn, EventAt: stringValue(v, "timestamp"), Model: state.model, SourcePath: path, SourceOffset: offset, Tokens: map[string]int64{"input_tokens": integer(u["input_tokens"]), "cached_input_tokens": integer(u["cached_input_tokens"]), "output_tokens": integer(u["output_tokens"])}}, true
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
func upsertTx(ctx context.Context, tx *sql.Tx, e Event) (bool, error) {
	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT 1 FROM usage_events WHERE event_key=?", e.Key).Scan(&exists); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	vals := []any{e.Key, e.Client, e.SessionID, e.EventID, e.EventAt, e.Model, e.Tokens["input_tokens"], e.Tokens["cached_input_tokens"], e.Tokens["output_tokens"], e.Tokens["cache_read_tokens"], e.Tokens["cache_creation_tokens"], e.Tokens["cache_write_5m_tokens"], e.Tokens["cache_write_1h_tokens"], e.SourcePath, e.SourceOffset}
	_, err := tx.ExecContext(ctx, `INSERT INTO usage_events(event_key,client,session_id,event_id,event_at,model,input_tokens,cached_input_tokens,output_tokens,cache_read_tokens,cache_creation_tokens,cache_write_5m_tokens,cache_write_1h_tokens,source_path,source_offset)VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(event_key) DO UPDATE SET event_at=excluded.event_at,model=excluded.model,input_tokens=excluded.input_tokens,cached_input_tokens=excluded.cached_input_tokens,output_tokens=excluded.output_tokens,cache_read_tokens=excluded.cache_read_tokens,cache_creation_tokens=excluded.cache_creation_tokens,cache_write_5m_tokens=excluded.cache_write_5m_tokens,cache_write_1h_tokens=excluded.cache_write_1h_tokens,source_path=excluded.source_path,source_offset=excluded.source_offset`, vals...)
	return exists == 0, err
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

func (s *Service) removeSource(ctx context.Context, path string) error {
	tx, err := s.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	affected, err := affectedSessions(ctx, tx, path)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM usage_run_sources WHERE path=?", path); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM usage_events WHERE source_path=?", path); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM usage_source_files WHERE path=?", path); err != nil {
		return err
	}
	if err = rebuildSessions(ctx, tx, affected); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) Diagnose(ctx context.Context) (map[string]any, error) {
	out := map[string]any{}
	for k, q := range map[string]string{"files": "SELECT COUNT(*) FROM usage_source_files", "events": "SELECT COUNT(*) FROM usage_events", "sessions": "SELECT COUNT(*) FROM usage_sessions", "exact_runs": "SELECT COUNT(*) FROM usage_runs WHERE ended_at IS NULL"} {
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
func (s *Service) PriceHistory(ctx context.Context) ([]map[string]string, error) {
	rows, err := s.Store.DB.QueryContext(ctx, "SELECT version,source_kind,source_url,content_sha256,effective_from FROM price_catalogs ORDER BY effective_from")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]string
	for rows.Next() {
		var version, kind, url, content, effective string
		if err := rows.Scan(&version, &kind, &url, &content, &effective); err != nil {
			return nil, err
		}
		out = append(out, map[string]string{"version": version, "source_kind": kind, "source_url": url, "content_sha256": content, "effective_from": effective})
	}
	return out, rows.Err()
}

// PriceStatus reports the locally available catalog; it never accesses the network.
func (s *Service) PriceStatus(ctx context.Context) (map[string]any, error) {
	var version, kind, source, commit, hash, effective string
	err := s.Store.DB.QueryRowContext(ctx, `SELECT version,source_kind,source_url,COALESCE(commit_sha,''),content_sha256,effective_from FROM price_catalogs ORDER BY effective_from DESC, imported_at DESC LIMIT 1`).Scan(&version, &kind, &source, &commit, &hash, &effective)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]any{"available": false}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"available": true, "version": version, "source_kind": kind, "source_url": source, "commit_sha": commit, "content_sha256": hash, "effective_from": effective, "aggregated_reference": kind == "litellm"}, nil
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
	runID    sql.NullInt64
	runExact sql.NullInt64
}

func (s *Service) Summary(ctx context.Context) (Summary, error) {
	events, err := s.events(ctx, "", "")
	if err != nil {
		return Summary{}, err
	}
	return s.summarize(ctx, events)
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
		item.Tokens, item.CatalogBaseCost, item.ProviderCost, item.Unpriced, item.Warnings = total.Tokens, total.CatalogBaseCost, total.ProviderCost, total.Unpriced, total.Warnings
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Service) events(ctx context.Context, client, session string) ([]storedEvent, error) {
	q := `SELECT e.event_key,e.client,e.session_id,e.event_id,e.event_at,e.model,e.input_tokens,e.cached_input_tokens,e.output_tokens,e.cache_read_tokens,e.cache_creation_tokens,e.cache_write_5m_tokens,e.cache_write_1h_tokens,e.source_path,e.source_offset,COALESCE(b.run_id, e.run_id),r.exact FROM usage_events e LEFT JOIN usage_run_bindings b ON b.event_key=e.event_key LEFT JOIN usage_runs r ON r.id=COALESCE(b.run_id,e.run_id)`
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
		err = rows.Scan(&e.Key, &e.Client, &e.SessionID, &e.EventID, &e.EventAt, &e.Model, &in, &cached, &outTokens, &read, &creation, &write5, &write1, &e.SourcePath, &e.SourceOffset, &e.runID, &e.runExact)
		if err != nil {
			return nil, err
		}
		e.Tokens = map[string]int64{"input_tokens": in, "cached_input_tokens": cached, "output_tokens": outTokens, "cache_read_tokens": read, "cache_creation_tokens": creation, "cache_write_5m_tokens": write5, "cache_write_1h_tokens": write1}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (s *Service) summarize(ctx context.Context, events []storedEvent) (Summary, error) {
	out := Summary{Tokens: map[string]int64{}, Counts: map[string]int64{"events": int64(len(events)), "exact": 0, "estimated": 0, "historical": 0}, Unpriced: []string{}, Warnings: []string{}}
	base := new(big.Rat)
	provider := new(big.Rat)
	complete := true
	warned := map[string]bool{}
	unpriced := map[string]bool{}
	for _, e := range events {
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
		if r.CatalogBaseCost == nil {
			complete = false
			for _, u := range r.Unpriced {
				unpriced[u] = true
			}
			continue
		}
		b, _ := decimal(*r.CatalogBaseCost)
		p, _ := decimal(*r.ProviderCost)
		base.Add(base, b)
		provider.Add(provider, p)
	}
	if complete {
		b, p := money(base), money(provider)
		out.CatalogBaseCost = &b
		out.ProviderCost = &p
	}
	for u := range unpriced {
		out.Unpriced = append(out.Unpriced, u)
	}
	sort.Strings(out.Unpriced)
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
	rows, err := s.Store.DB.QueryContext(ctx, `SELECT mp.model,mp.provider,mp.effective_from,mp.prices_json,mp.aliases_json FROM model_prices mp JOIN price_catalogs c ON c.version=mp.catalog_version WHERE c.effective_from<=? AND mp.effective_from<=? ORDER BY CASE c.source_kind WHEN 'official' THEN 0 ELSE 1 END, c.effective_from DESC, c.imported_at DESC`, e.EventAt, e.EventAt)
	if err != nil {
		return modelPrice{}, "", "", err
	}
	defer rows.Close()
	expected := map[string]string{"codex": "openai", "claude": "anthropic"}[e.Client]
	merged := modelPrice{Provider: expected, Prices: map[string]string{}}
	found := false
	for rows.Next() {
		var model string
		var p modelPrice
		var raw, aliases string
		if err = rows.Scan(&model, &p.Provider, &p.EffectiveFrom, &raw, &aliases); err != nil {
			return p, "", "", err
		}
		if p.Provider != expected {
			continue
		}
		if err = json.Unmarshal([]byte(raw), &p.Prices); err != nil {
			return p, "", "", err
		}
		matches := model == e.Model
		var a []string
		_ = json.Unmarshal([]byte(aliases), &a)
		for _, x := range a {
			if x == e.Model {
				matches = true
			}
		}
		if !matches {
			continue
		}
		// Rows are newest-first. Preserve the first value of each component so an
		// official layer can override only one verified component.
		for k, v := range p.Prices {
			if _, ok := merged.Prices[k]; !ok {
				merged.Prices[k] = v
			}
		}
		found = true
	}
	if err := rows.Err(); err != nil {
		return modelPrice{}, "", "", err
	}
	if found {
		return merged, mult, quality, nil
	}
	return modelPrice{Provider: "unknown", Prices: map[string]string{}}, mult, quality, nil
}
func ParseMultiplier(v string) (string, error) {
	r, e := decimal(v)
	if e != nil {
		return "", e
	}
	return r.FloatString(max(0, 9)), nil
}
func ParseInt(v string) (int64, error) { return strconv.ParseInt(v, 10, 64) }
