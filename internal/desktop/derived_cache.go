package desktop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kitdine/agent-deck/internal/ingest"
	"github.com/kitdine/agent-deck/internal/platform"
	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
)

const (
	derivedSnapshotCacheFilename         = "desktop-derived-cache.json"
	derivedSnapshotCacheVersion          = 1
	derivedSnapshotCacheAlgorithmVersion = 1
	derivedSnapshotCacheMaxBytes         = 8 << 20
	derivedSnapshotCacheLockWait         = time.Second
)

var ErrDerivedSnapshotCacheChanged = errors.New("derived snapshot cache inputs changed during publication")
var ErrDerivedSnapshotCacheTooLarge = errors.New("derived snapshot cache exceeds size limit")
var beforeDerivedSnapshotCacheGenerationRecheck = func(*store.Store) {}

type derivedSnapshotCacheHeader struct {
	Version              int    `json:"version"`
	AlgorithmVersion     int    `json:"algorithm_version"`
	WireVersion          int    `json:"wire_version"`
	DatabaseIdentity     string `json:"database_identity"`
	SchemaVersion        int    `json:"schema_version"`
	UsageParserVersion   int    `json:"usage_parser_version"`
	SessionParserVersion int    `json:"session_parser_version"`
	Epoch                int64  `json:"epoch"`
	Revision             int64  `json:"revision"`
	Timezone             string `json:"timezone"`
	WindowStart          string `json:"window_start"`
	WindowEnd            string `json:"window_end"`
	CreatedAt            string `json:"created_at"`
	ValidUntil           string `json:"valid_until"`
}

type derivedSnapshotPriceAvailability struct {
	Validated          bool `json:"validated"`
	PricingComplete    bool `json:"pricing_complete"`
	UnpricedComponents int  `json:"unpriced_components"`
}

// derivedSnapshotCachePayload deliberately contains only the four approved
// derived values. Provider, session, health, credentials, and source material
// remain live inputs to each desktop snapshot.
type derivedSnapshotCachePayload struct {
	Presentation      usage.PresentationReport         `json:"presentation"`
	Summary           usage.Summary                    `json:"summary"`
	WorkSignals       WorkSignalsSnapshot              `json:"work_signals"`
	PriceAvailability derivedSnapshotPriceAvailability `json:"price_availability"`
}

type derivedSnapshotCacheDocument struct {
	Header   derivedSnapshotCacheHeader  `json:"header"`
	Payload  derivedSnapshotCachePayload `json:"payload"`
	Checksum string                      `json:"checksum"`
}

type derivedSnapshotCacheUnsigned struct {
	Header  derivedSnapshotCacheHeader  `json:"header"`
	Payload derivedSnapshotCachePayload `json:"payload"`
}

func (s Service) derivedSnapshotCachePath() string {
	return filepath.Join(s.StateRoot, derivedSnapshotCacheFilename)
}

func (s Service) derivedSnapshotPayload(ctx context.Context, core *store.Store, now time.Time) (derivedSnapshotCachePayload, error) {
	service := usage.New(core, s.Home)
	service.Now = func() time.Time { return now }
	presentation, err := service.Presentation(ctx, now, s.location())
	if err != nil {
		return derivedSnapshotCachePayload{}, err
	}
	workSignals, err := s.workSignalsSnapshot(ctx, core, now)
	if err != nil {
		return derivedSnapshotCachePayload{}, err
	}
	return derivedSnapshotCachePayload{
		Presentation: presentation,
		Summary:      presentation.Summary,
		WorkSignals:  workSignals,
		PriceAvailability: derivedSnapshotPriceAvailability{
			Validated:          true,
			PricingComplete:    len(presentation.Summary.Unpriced) == 0,
			UnpricedComponents: len(presentation.Summary.Unpriced),
		},
	}, nil
}

func (s Service) applyDerivedSnapshotPayload(now time.Time, result *Result, payload derivedSnapshotCachePayload) {
	from, to := localDay(now, s.location())
	presentation := payload.Presentation
	presentation.Summary = payload.Summary
	usageSnapshot := usageSnapshot(payload.Summary, from, to)
	usageSnapshot.Presentation = presentation
	result.Snapshot.Usage = usageSnapshot
	result.Snapshot.Sessions.WorkSignals = payload.WorkSignals
}

func (s Service) derivedSnapshotCacheHeader(ctx context.Context, core *store.Store, generation store.DerivedSnapshotGeneration, now time.Time, wireVersion int) (derivedSnapshotCacheHeader, error) {
	identity, err := derivedSnapshotDatabaseIdentity(filepath.Join(s.StateRoot, "agentdeck.sqlite3"))
	if err != nil {
		return derivedSnapshotCacheHeader{}, err
	}
	location := s.location()
	from, to := localDay(now, location)
	validUntil, err := nextDerivedSnapshotValidityBoundary(ctx, core, now, location, to)
	if err != nil {
		return derivedSnapshotCacheHeader{}, err
	}
	return derivedSnapshotCacheHeader{
		Version:              derivedSnapshotCacheVersion,
		AlgorithmVersion:     derivedSnapshotCacheAlgorithmVersion,
		WireVersion:          wireVersion,
		DatabaseIdentity:     identity,
		SchemaVersion:        generation.SchemaVersion,
		UsageParserVersion:   usage.ParserVersion,
		SessionParserVersion: session.ParserVersion,
		Epoch:                generation.Epoch,
		Revision:             generation.Revision,
		Timezone:             location.String(),
		WindowStart:          from.UTC().Format(time.RFC3339Nano),
		WindowEnd:            to.UTC().Format(time.RFC3339Nano),
		CreatedAt:            now.UTC().Format(time.RFC3339Nano),
		ValidUntil:           validUntil.UTC().Format(time.RFC3339Nano),
	}, nil
}

func derivedSnapshotDatabaseIdentity(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	identity, _, stable := ingest.FileGeneration(info)
	if !stable {
		return "", errors.New("unsupported derived cache database identity")
	}
	return identity, nil
}

func nextDerivedSnapshotValidityBoundary(ctx context.Context, core *store.Store, now time.Time, location *time.Location, nextDay time.Time) (time.Time, error) {
	local := now.In(location)
	nextHour := time.Date(local.Year(), local.Month(), local.Day(), local.Hour()+1, 0, 0, 0, location)
	validUntil := nextHour
	if nextDay.Before(validUntil) {
		validUntil = nextDay
	}
	rows, err := core.DB.QueryContext(ctx, `SELECT effective_from FROM price_catalogs UNION SELECT effective_from FROM model_prices`)
	if err != nil {
		return time.Time{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return time.Time{}, err
		}
		effective, parseErr := parseDerivedSnapshotPriceBoundary(raw)
		if parseErr != nil {
			return time.Time{}, parseErr
		}
		if effective.After(now) && effective.Before(validUntil) {
			validUntil = effective
		}
	}
	return validUntil, rows.Err()
}

func parseDerivedSnapshotPriceBoundary(raw string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid price effective boundary %q", raw)
}

func (s Service) loadDerivedSnapshotCache(ctx context.Context, core *store.Store, now time.Time, wireVersion int) (derivedSnapshotCachePayload, bool) {
	document, err := readDerivedSnapshotCache(s.derivedSnapshotCachePath())
	if err != nil {
		return derivedSnapshotCachePayload{}, false
	}
	generation, err := core.DerivedSnapshotGeneration(ctx)
	if err != nil || generation.Dirty {
		return derivedSnapshotCachePayload{}, false
	}
	header, err := s.derivedSnapshotCacheHeader(ctx, core, generation, now, wireVersion)
	if err != nil || !derivedSnapshotCacheHeaderMatches(document.Header, header, now) || !validDerivedSnapshotCachePayload(document.Payload) {
		return derivedSnapshotCachePayload{}, false
	}
	// Recheck after the bounded cache read so a writer cannot make a stale hit
	// appear current merely because it committed during validation.
	after, err := core.DerivedSnapshotGeneration(ctx)
	if err != nil || after.Dirty || after.Epoch != generation.Epoch || after.Revision != generation.Revision {
		return derivedSnapshotCachePayload{}, false
	}
	return document.Payload, true
}

func derivedSnapshotCacheHeaderMatches(actual, expected derivedSnapshotCacheHeader, now time.Time) bool {
	if actual.Version != expected.Version || actual.AlgorithmVersion != expected.AlgorithmVersion || actual.WireVersion != expected.WireVersion || actual.DatabaseIdentity != expected.DatabaseIdentity || actual.SchemaVersion != expected.SchemaVersion || actual.UsageParserVersion != expected.UsageParserVersion || actual.SessionParserVersion != expected.SessionParserVersion || actual.Epoch != expected.Epoch || actual.Revision != expected.Revision || actual.Timezone != expected.Timezone || actual.WindowStart != expected.WindowStart || actual.WindowEnd != expected.WindowEnd {
		return false
	}
	created, err := time.Parse(time.RFC3339Nano, actual.CreatedAt)
	if err != nil || now.Before(created) {
		return false
	}
	validUntil, err := time.Parse(time.RFC3339Nano, actual.ValidUntil)
	return err == nil && now.Before(validUntil)
}

func validDerivedSnapshotCachePayload(payload derivedSnapshotCachePayload) bool {
	return payload.PriceAvailability.Validated && payload.Presentation.Available && payload.Summary.Tokens != nil && payload.Summary.Counts != nil && payload.Summary.AttributionReasons != nil && payload.WorkSignals.Activity.Items != nil && payload.WorkSignals.Workflow.Items != nil && payload.WorkSignals.Tooling.Items != nil
}

func readDerivedSnapshotCache(path string) (derivedSnapshotCacheDocument, error) {
	directory, err := os.Stat(filepath.Dir(path))
	if err != nil {
		return derivedSnapshotCacheDocument{}, err
	}
	if !directory.IsDir() || directory.Mode().Perm() != platform.DirectoryMode {
		return derivedSnapshotCacheDocument{}, errors.New("invalid derived snapshot cache directory")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return derivedSnapshotCacheDocument{}, err
	}
	if info.Size() > derivedSnapshotCacheMaxBytes {
		return derivedSnapshotCacheDocument{}, ErrDerivedSnapshotCacheTooLarge
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != platform.FileMode || info.Size() <= 0 {
		return derivedSnapshotCacheDocument{}, errors.New("invalid derived snapshot cache file")
	}
	file, err := os.Open(path)
	if err != nil {
		return derivedSnapshotCacheDocument{}, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return derivedSnapshotCacheDocument{}, err
	}
	if !opened.Mode().IsRegular() || opened.Mode().Perm() != platform.FileMode || !os.SameFile(info, opened) {
		return derivedSnapshotCacheDocument{}, errors.New("derived snapshot cache changed while opening")
	}
	contents, err := io.ReadAll(io.LimitReader(file, derivedSnapshotCacheMaxBytes+1))
	if err != nil || len(contents) > derivedSnapshotCacheMaxBytes {
		if err != nil {
			return derivedSnapshotCacheDocument{}, err
		}
		return derivedSnapshotCacheDocument{}, ErrDerivedSnapshotCacheTooLarge
	}
	var document derivedSnapshotCacheDocument
	if err = json.Unmarshal(contents, &document); err != nil {
		return derivedSnapshotCacheDocument{}, err
	}
	checksum, err := derivedSnapshotCacheChecksum(document.Header, document.Payload)
	if err != nil || document.Checksum == "" || document.Checksum != checksum {
		return derivedSnapshotCacheDocument{}, errors.New("derived snapshot cache checksum mismatch")
	}
	return document, nil
}

func derivedSnapshotCacheChecksum(header derivedSnapshotCacheHeader, payload derivedSnapshotCachePayload) (string, error) {
	contents, err := json.Marshal(derivedSnapshotCacheUnsigned{Header: header, Payload: payload})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:]), nil
}

func encodeDerivedSnapshotCache(header derivedSnapshotCacheHeader, payload derivedSnapshotCachePayload) ([]byte, error) {
	checksum, err := derivedSnapshotCacheChecksum(header, payload)
	if err != nil {
		return nil, err
	}
	contents, err := json.Marshal(derivedSnapshotCacheDocument{Header: header, Payload: payload, Checksum: checksum})
	if err != nil {
		return nil, err
	}
	contents = append(contents, '\n')
	if len(contents) > derivedSnapshotCacheMaxBytes {
		return nil, ErrDerivedSnapshotCacheTooLarge
	}
	return contents, nil
}

func writeDerivedSnapshotCache(path string, contents []byte) error {
	if len(contents) > derivedSnapshotCacheMaxBytes {
		return ErrDerivedSnapshotCacheTooLarge
	}
	directory := filepath.Dir(path)
	info, err := os.Stat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm() != platform.DirectoryMode {
		return errors.New("invalid derived snapshot cache directory")
	}
	temporary, err := os.CreateTemp(directory, ".desktop-derived-cache-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err = temporary.Chmod(platform.FileMode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err = temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return err
	}
	if err = temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return os.Chmod(path, platform.FileMode)
}

func cleanupDerivedSnapshotCacheTemps(directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".desktop-derived-cache-") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != platform.FileMode {
			continue
		}
		if err = os.Remove(path); err != nil {
			return err
		}
	}
	return nil
}

// PublishDerivedSnapshotCache is the worker-only publisher. It never runs from
// the read-only snapshot path and deliberately does not include session data.
func (s Service) PublishDerivedSnapshotCache(ctx context.Context, wireVersion int) error {
	lock, err := store.AcquireDerivedSnapshotCacheLock(ctx, s.StateRoot, derivedSnapshotCacheLockWait)
	if err != nil {
		return err
	}
	defer lock.Release()
	if err = cleanupDerivedSnapshotCacheTemps(s.StateRoot); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	core, err := store.OpenExisting(ctx, s.StateRoot)
	if err != nil {
		return err
	}
	defer core.Close()
	now := s.now().UTC()
	current, err := core.DerivedSnapshotGeneration(ctx)
	if err != nil {
		return err
	}
	if !current.Dirty {
		if _, found := s.loadDerivedSnapshotCache(ctx, core, now, wireVersion); found {
			return nil
		}
	}
	before, err := core.BeginDerivedSnapshotGenerationBuild(ctx)
	if err != nil {
		return err
	}
	beforeHeader, err := s.derivedSnapshotCacheHeader(ctx, core, before, now, wireVersion)
	if err != nil {
		return err
	}
	payload, err := s.derivedSnapshotPayload(ctx, core, now)
	if err != nil {
		return err
	}
	beforeDerivedSnapshotCacheGenerationRecheck(core)
	after, err := core.DerivedSnapshotGeneration(ctx)
	if err != nil {
		return err
	}
	if after.Dirty || before.Epoch != after.Epoch || before.Revision != after.Revision {
		return ErrDerivedSnapshotCacheChanged
	}
	now = s.now().UTC()
	header, err := s.derivedSnapshotCacheHeader(ctx, core, after, now, wireVersion)
	if err != nil {
		return err
	}
	if beforeHeader.Timezone != header.Timezone || beforeHeader.WindowStart != header.WindowStart || beforeHeader.WindowEnd != header.WindowEnd || !now.Before(parseDerivedSnapshotCacheValidUntil(beforeHeader)) {
		return ErrDerivedSnapshotCacheChanged
	}
	contents, err := encodeDerivedSnapshotCache(header, payload)
	if err != nil {
		return err
	}
	if err = writeDerivedSnapshotCache(s.derivedSnapshotCachePath(), contents); err != nil {
		return err
	}
	final, err := core.DerivedSnapshotGeneration(ctx)
	if err != nil || final.Dirty || final.Epoch != after.Epoch || final.Revision != after.Revision {
		_ = os.Remove(s.derivedSnapshotCachePath())
		if err != nil {
			return err
		}
		return ErrDerivedSnapshotCacheChanged
	}
	return nil
}

func parseDerivedSnapshotCacheValidUntil(header derivedSnapshotCacheHeader) time.Time {
	validUntil, err := time.Parse(time.RFC3339Nano, header.ValidUntil)
	if err != nil {
		return time.Time{}
	}
	return validUntil
}
