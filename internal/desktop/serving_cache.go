package desktop

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/kitdine/agent-deck/internal/store"
	"github.com/kitdine/agent-deck/internal/usage"
	"golang.org/x/sys/unix"
)

const (
	ServingWireVersion       = 2
	servingFormatVersion     = 1
	servingProjectionVersion = 1
	servingFilename          = "desktop-serving-snapshot.json"
	servingMaxBytes          = 8 << 20
)

var (
	ErrServingInvalid  = errors.New("serving_snapshot_invalid")
	ErrServingChanged  = errors.New("serving_generation_changed")
	ErrServingTooLarge = errors.New("serving_payload_limit")
)

type ServingHeader struct {
	FormatVersion       int                     `json:"format_version"`
	WireVersion         int                     `json:"wire_version"`
	Generation          store.ServingGeneration `json:"generation"`
	PublicationID       string                  `json:"publication_id"`
	ProjectionVersion   int                     `json:"projection_version"`
	UsageParserVersion  int                     `json:"usage_parser_version"`
	ActiveGeneration    string                  `json:"active_generation"`
	PricingRevision     int64                   `json:"pricing_revision"`
	DataObservedThrough string                  `json:"data_observed_through"`
	Freshness           string                  `json:"freshness"`
	SessionPrepared     bool                    `json:"session_prepared"`
	Timezone            string                  `json:"timezone"`
	WindowStart         string                  `json:"window_start"`
	WindowEnd           string                  `json:"window_end"`
	GeneratedAt         string                  `json:"generated_at"`
	UsageCoverage       store.SourceCoverage    `json:"usage_coverage"`
	SessionCoverage     store.SourceCoverage    `json:"session_coverage"`
}

// The nested envelope remains v1 so existing Widget serialization and native
// presentation reducers do not interpret a new mandatory field as legacy data.
type ServingEnvelope struct {
	SchemaVersion int      `json:"schema_version"`
	Command       string   `json:"command"`
	GeneratedAt   string   `json:"generated_at"`
	Partial       bool     `json:"partial"`
	Warnings      []string `json:"warnings"`
	Data          Snapshot `json:"data"`
}

type ServingSnapshot struct {
	Header  ServingHeader   `json:"header"`
	Payload ServingEnvelope `json:"payload"`
}

// Body is the exact UTF-8 header+payload byte sequence. A native reader verifies
// its checksum before decoding, without reserializing floating point or keys.
type servingDocument struct {
	FormatVersion int    `json:"format_version"`
	Body          []byte `json:"body"`
	Checksum      string `json:"checksum"`
}

func (s Service) BuildToday(ctx context.Context) (Result, error) {
	now := s.now().UTC()
	result := Result{Snapshot: Snapshot{
		WireVersion: WireVersion, GeneratedAt: now.Format(time.RFC3339Nano), NextRefreshAt: now.Add(RefreshInterval).Format(time.RFC3339Nano),
		Provider: ProviderSnapshot{Routes: []ProviderRoute{}, Candidates: []ProviderCandidate{}},
		Usage:    emptyUsageSnapshot(now, s.location()), Sessions: emptySessionsSnapshot(),
		Health: HealthSnapshot{Checks: []HealthCheck{}}, Subscription: unavailableSubscription(),
	}}
	core, err := store.OpenReadOnly(ctx, s.StateRoot)
	if err != nil {
		return Result{}, err
	}
	defer core.Close()
	coverage, err := core.ReadSourceCoverage(ctx)
	if err != nil {
		return Result{}, err
	}
	service := usage.New(core, s.Home)
	service.Now = func() time.Time { return now }
	presentation, err := service.TodayPresentation(ctx, now, s.location())
	if err != nil {
		return Result{}, err
	}
	from, to := localDay(now, s.location())
	result.Snapshot.Usage = usageSnapshot(presentation.Summary, from, to)
	result.Snapshot.Usage.Presentation = presentation
	observed, observedErr := time.Parse(time.RFC3339Nano, coverage.ObservedAt)
	if coverage.State == "complete" && (observedErr != nil || observed.Before(from)) {
		coverage.State = "unknown"
	}
	if coverage.State != "complete" {
		result.warn(coverageWarning(coverage.State))
		// An empty partial source set proves no measured zero.
		if len(presentation.Scopes) == 0 || presentation.Scopes[0].Periods.Items[0].Totals.Events == 0 {
			result.Snapshot.Usage.Available = false
			result.Snapshot.Usage.Presentation = usage.EmptyPresentationReport()
		}
	}
	s.loadProvider(ctx, core, &result)
	s.loadSubscription(ctx, core, now, &result)
	result.warn("sessions_unavailable")
	result.warn("work_signals_unavailable")
	result.warn("health_unavailable")
	sort.Strings(result.Warnings)
	return result, nil
}

func coverageWarning(state string) string {
	switch state {
	case "update_pending":
		return "source_update_pending"
	case "partial":
		return "source_coverage_partial"
	default:
		return "source_coverage_unknown"
	}
}

func (s Service) PublishServingSnapshot(ctx context.Context, request Request, todayOnly bool) (ServingSnapshot, error) {
	return s.prepareServingSnapshot(ctx, request, todayOnly, true)
}

// ReadOrBootstrapServingSnapshot never starts ingestion or creates state. A
// damaged/missing serving file falls back only to committed today's facts.
func (s Service) ReadOrBootstrapServingSnapshot(ctx context.Context, request Request) (ServingSnapshot, error) {
	if err := request.Validate(); err != nil {
		return ServingSnapshot{}, err
	}
	if snapshot, err := s.ReadServingSnapshot(ctx); err == nil {
		return snapshot, nil
	} else if errors.Is(err, store.ErrSchemaAhead) {
		return ServingSnapshot{}, err
	}
	return s.prepareServingSnapshot(ctx, request, true, false)
}

func (s Service) prepareServingSnapshot(ctx context.Context, request Request, todayOnly, persist bool) (ServingSnapshot, error) {
	if err := request.Validate(); err != nil {
		return ServingSnapshot{}, err
	}
	if persist {
		lock, err := store.AcquireServingSnapshotLock(ctx, s.StateRoot, time.Second)
		if err != nil {
			return ServingSnapshot{}, err
		}
		defer lock.Release()
	}
	before, err := store.ReadServingGeneration(ctx, s.StateRoot)
	if err != nil {
		return ServingSnapshot{}, err
	}
	var result Result
	if todayOnly {
		result, err = s.BuildToday(ctx)
	} else {
		result, err = s.Build(ctx, request)
	}
	if err != nil {
		return ServingSnapshot{}, err
	}
	core, err := store.OpenReadOnly(ctx, s.StateRoot)
	if err != nil {
		return ServingSnapshot{}, err
	}
	usageCoverage, err := core.ReadSourceCoverage(ctx)
	core.Close()
	if err != nil {
		return ServingSnapshot{}, err
	}
	sessions, err := store.OpenSessionsReadOnly(ctx, s.StateRoot)
	if err != nil {
		return ServingSnapshot{}, err
	}
	sessionCoverage, err := sessions.ReadSourceCoverage(ctx)
	sessions.Close()
	if err != nil {
		return ServingSnapshot{}, err
	}
	for _, coverage := range []store.SourceCoverage{usageCoverage, sessionCoverage} {
		if coverage.State != "complete" {
			result.warn(coverageWarning(coverage.State))
		}
	}
	after, err := store.ReadServingGeneration(ctx, s.StateRoot)
	if err != nil {
		return ServingSnapshot{}, err
	}
	if before != after {
		return ServingSnapshot{}, ErrServingChanged
	}
	var id [16]byte
	if _, err = rand.Read(id[:]); err != nil {
		return ServingSnapshot{}, err
	}
	now := s.now()
	start, end := localDay(now, s.location())
	if result.Snapshot.GeneratedAt != now.UTC().Format(time.RFC3339Nano) {
		generated, parseErr := time.Parse(time.RFC3339Nano, result.Snapshot.GeneratedAt)
		if parseErr != nil {
			return ServingSnapshot{}, ErrServingInvalid
		}
		oldStart, oldEnd := localDay(generated, s.location())
		if !start.Equal(oldStart) || !end.Equal(oldEnd) {
			return ServingSnapshot{}, ErrServingChanged
		}
	}
	sort.Strings(result.Warnings)
	if result.Warnings == nil {
		result.Warnings = []string{}
	}
	prepared := ServingSnapshot{
		Header: ServingHeader{FormatVersion: servingFormatVersion, WireVersion: ServingWireVersion, Generation: after,
			PublicationID: hex.EncodeToString(id[:]), ProjectionVersion: servingProjectionVersion, UsageParserVersion: usage.ParserVersion,
			ActiveGeneration: fmt.Sprintf("%d:%d/%d:%d", after.CoreEpoch, after.UsageRevision, after.SessionEpoch, after.SessionRevision), PricingRevision: after.UsageRevision,
			DataObservedThrough: usageCoverage.ObservedAt, Freshness: "current",
			SessionPrepared: !todayOnly && result.Snapshot.Sessions.Available, Timezone: s.location().String(), WindowStart: start.UTC().Format(time.RFC3339Nano), WindowEnd: end.UTC().Format(time.RFC3339Nano),
			GeneratedAt: result.Snapshot.GeneratedAt, UsageCoverage: usageCoverage, SessionCoverage: sessionCoverage},
		Payload: ServingEnvelope{SchemaVersion: 1, Command: "desktop.snapshot", GeneratedAt: result.Snapshot.GeneratedAt, Partial: result.Partial, Warnings: result.Warnings, Data: result.Snapshot},
	}
	contents, err := encodeServingSnapshot(prepared)
	if err != nil {
		return ServingSnapshot{}, err
	}
	if persist {
		if todayOnly {
			// Do not replace the last full, identity-valid publication with a
			// narrower bootstrap while it remains readable. Empty/no-cache
			// startups still persist each first usable partial publication.
			if previous, err := s.ReadServingSnapshot(ctx); err == nil && previous.Header.SessionPrepared {
				return prepared, nil
			}
		}
		if err = writeServingSnapshot(s.StateRoot, contents); err != nil {
			return ServingSnapshot{}, err
		}
	}
	return prepared, nil
}

func encodeServingSnapshot(snapshot ServingSnapshot) ([]byte, error) {
	body, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	contents, err := json.Marshal(servingDocument{FormatVersion: servingFormatVersion, Body: body, Checksum: hex.EncodeToString(sum[:])})
	if err != nil {
		return nil, err
	}
	contents = append(contents, '\n')
	if len(contents) > servingMaxBytes {
		return nil, ErrServingTooLarge
	}
	return contents, nil
}

func (s Service) ReadServingSnapshot(ctx context.Context) (ServingSnapshot, error) {
	reader, err := store.OpenServingGenerationReader(ctx, s.StateRoot)
	if err != nil {
		return ServingSnapshot{}, err
	}
	defer reader.Close()
	identity, err := reader.Read()
	if err != nil {
		return ServingSnapshot{}, err
	}
	directory, err := openServingDirectory(s.StateRoot)
	if err != nil {
		return ServingSnapshot{}, err
	}
	defer unix.Close(directory)
	fd, err := unix.Openat(directory, servingFilename, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return ServingSnapshot{}, err
	}
	file := os.NewFile(uintptr(fd), servingFilename)
	defer file.Close()
	var stat unix.Stat_t
	if err = unix.Fstat(fd, &stat); err != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&07777 != 0600 || stat.Uid != uint32(os.Getuid()) || stat.Size > servingMaxBytes {
		return ServingSnapshot{}, ErrServingInvalid
	}
	contents, err := io.ReadAll(io.LimitReader(file, servingMaxBytes+1))
	if err != nil || len(contents) > servingMaxBytes {
		return ServingSnapshot{}, ErrServingInvalid
	}
	var document servingDocument
	if err = json.Unmarshal(contents, &document); err != nil || document.FormatVersion != servingFormatVersion {
		return ServingSnapshot{}, ErrServingInvalid
	}
	sum := sha256.Sum256(document.Body)
	if document.Checksum != hex.EncodeToString(sum[:]) {
		return ServingSnapshot{}, ErrServingInvalid
	}
	var snapshot ServingSnapshot
	if err = json.Unmarshal(document.Body, &snapshot); err != nil {
		return ServingSnapshot{}, ErrServingInvalid
	}
	header := snapshot.Header
	old := header.Generation
	if header.FormatVersion != servingFormatVersion || header.WireVersion != ServingWireVersion || header.ProjectionVersion != servingProjectionVersion || header.PublicationID == "" ||
		old.StateIdentity != identity.StateIdentity || old.CoreEpoch != identity.CoreEpoch || old.SessionEpoch != identity.SessionEpoch || !compatibleServingCoreSchema(old.CoreSchema,identity.CoreSchema) || old.SessionSchema != identity.SessionSchema ||
		snapshot.Payload.SchemaVersion != 1 || snapshot.Payload.Command != "desktop.snapshot" || snapshot.Payload.Data.WireVersion != WireVersion || snapshot.Payload.Data.GeneratedAt != header.GeneratedAt {
		return ServingSnapshot{}, ErrServingInvalid
	}
	start, end := localDay(s.now(), s.location())
	if header.Timezone != s.location().String() || header.WindowStart != start.UTC().Format(time.RFC3339Nano) || header.WindowEnd != end.UTC().Format(time.RFC3339Nano) {
		return ServingSnapshot{}, ErrServingInvalid
	}
	generatedAt, err := time.Parse(time.RFC3339Nano, header.GeneratedAt)
	if err != nil || generatedAt.Before(start) || !generatedAt.Before(end) || snapshot.Payload.GeneratedAt != header.GeneratedAt || (header.Freshness != "current" && header.Freshness != "stale") {
		return ServingSnapshot{}, ErrServingInvalid
	}
	final, err := reader.Read()
	if err != nil {
		return ServingSnapshot{}, err
	}
	if final.StateIdentity != identity.StateIdentity || final.CoreEpoch != identity.CoreEpoch || final.SessionEpoch != identity.SessionEpoch || final.CoreSchema != identity.CoreSchema || final.SessionSchema != identity.SessionSchema {
		return ServingSnapshot{}, ErrServingInvalid
	}
	if old.UsageRevision != final.UsageRevision || old.SessionRevision != final.SessionRevision || old.CoreSchema != final.CoreSchema {
		snapshot.Header.Freshness = "stale"
	}
	// Revision differences retain readable, honestly dated data. Native consumers
	// use the committed generation and coverage rather than treating it as fresh.
	return snapshot, nil
}

func openServingDirectory(root string) (int, error) {
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return -1, err
	}
	var stat unix.Stat_t
	if err = unix.Fstat(fd, &stat); err != nil || stat.Mode&07777 != 0700 || stat.Uid != uint32(os.Getuid()) {
		unix.Close(fd)
		return -1, ErrServingInvalid
	}
	return fd, nil
}

func writeServingSnapshot(root string, contents []byte) error {
	if len(contents) > servingMaxBytes {
		return ErrServingTooLarge
	}
	directory, err := openServingDirectory(root)
	if err != nil {
		return err
	}
	defer unix.Close(directory)
	var id [16]byte
	if _, err = rand.Read(id[:]); err != nil {
		return err
	}
	name := ".desktop-serving-" + hex.EncodeToString(id[:])
	fd, err := unix.Openat(directory, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	defer unix.Unlinkat(directory, name, 0)
	file := os.NewFile(uintptr(fd), filepath.Join(root, name))
	if _, err = file.Write(contents); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = unix.Renameat(directory, name, directory, servingFilename); err != nil {
		return err
	}
	return unix.Fsync(directory)
}
