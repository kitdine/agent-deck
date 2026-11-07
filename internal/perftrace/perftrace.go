// Package perftrace records the measurement markers defined by the
// scan-performance measurement contract. Recording is off unless
// AGENTDECK_PERF_OUT names a private (0700) directory owned by the current
// user; when off, every entry point costs one nil check.
//
// A record carries only a fixed stage name, CLOCK_UPTIME_RAW nanoseconds,
// integer counts, the process role and fixed error codes. It never carries
// paths, session identifiers, transcript text or credentials; Mark and Tx
// reject names that are not declared identifiers.
package perftrace

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"sync/atomic"
)

// EnvOut names the private run directory shared by the App and its helpers.
const EnvOut = "AGENTDECK_PERF_OUT"

// FormatVersion is written in every file header.
const FormatVersion = 1

// Role identifies the kind of process writing a trace file.
type Role string

const (
	RoleApp       Role = "app"
	RoleCLI       Role = "cli"
	RoleHelper    Role = "helper"
	RoleCollector Role = "collector"
	RoleDriver    Role = "driver"
)

// Stage is a fixed marker name from the measurement contract.
type Stage string

const (
	JobStart          Stage = "job.start"
	IngestRead        Stage = "ingest.read"
	IngestReduce      Stage = "ingest.reduce"
	IngestWrite       Stage = "ingest.write"
	ProjectionDone    Stage = "projection.done"
	PublicationCommit Stage = "publication.commit"
	SourceWritten     Stage = "source.written"
	ScanBegin         Stage = "scan.begin"
	ScanEnd           Stage = "scan.end"
	ResourceLifetime  Stage = "resource.lifetime"
	OTelAck           Stage = "otel.ack"
	OTelNormalized    Stage = "otel.normalized"
	OTelSend          Stage = "otel.send"

	// Usage sub-stages locate the P09 critical path; nested spans overlap.
	UsageSource          Stage = "usage.source"
	UsagePrepare         Stage = "usage.prepare"
	UsagePublicationWait Stage = "usage.publication_wait"
	UsagePublication     Stage = "usage.publication"
	UsageSessionRebuild  Stage = "usage.session_rebuild"
	UsageCommit          Stage = "usage.commit"
	UsageOrphanScan      Stage = "usage.orphan_candidates"
	UsageOrphanCleanup   Stage = "usage.orphan_cleanup"
	UsagePriceResolver   Stage = "usage.price_resolver"
	UsageEventsRange     Stage = "usage.events_range"
	UsageProjectedInputs Stage = "usage.projected_inputs"
	UsagePresentation    Stage = "usage.presentation"
	ScanDiscovery        Stage = "scan.discovery"
	ScanUsage            Stage = "scan.usage"
	ScanSession          Stage = "scan.session"
	ScanDerive           Stage = "scan.derive"
)

var goStages = map[Stage]bool{
	JobStart: true, IngestRead: true, IngestReduce: true, IngestWrite: true,
	ProjectionDone: true, PublicationCommit: true, SourceWritten: true,
	ScanBegin: true, ScanEnd: true, ResourceLifetime: true, OTelAck: true, OTelNormalized: true, OTelSend: true,
	UsageSource: true, UsagePrepare: true, UsagePublicationWait: true, UsagePublication: true,
	UsageSessionRebuild: true, UsageCommit: true, UsageOrphanScan: true, UsageOrphanCleanup: true,
	UsagePriceResolver: true, UsageEventsRange: true, UsageProjectedInputs: true, UsagePresentation: true,
	ScanDiscovery: true, ScanUsage: true, ScanSession: true, ScanDerive: true,
}

// identifier is the only shape accepted for transaction domains and labels,
// count keys and error codes, so no free-form value can reach a record.
var identifier = regexp.MustCompile(`^[a-z][a-z0-9_]{0,47}$`)

// Count is one integer attribute of a record.
type Count struct {
	Key   string
	Value int64
}

type header struct {
	V     int    `json:"v"`
	Role  Role   `json:"role"`
	Seq   int    `json:"seq"`
	Clock string `json:"clock"`
	Start int64  `json:"t"`
}

type record struct {
	Stage string           `json:"stage"`
	T     int64            `json:"t"`
	Dur   *int64           `json:"dur,omitempty"`
	N     map[string]int64 `json:"n,omitempty"`
	Err   string           `json:"err,omitempty"`
}

type recorder struct {
	mu      sync.Mutex
	file    *os.File
	w       *bufio.Writer
	invalid int64
	closed  bool
}

var active atomic.Pointer[recorder]

// Start enables recording for this process when EnvOut names a valid private
// directory. It returns false, and records nothing, otherwise. Start is meant
// to be called once from main before any marker.
func Start(role Role) bool {
	dir := os.Getenv(EnvOut)
	if dir == "" || active.Load() != nil {
		return active.Load() != nil
	}
	if !identifier.MatchString(string(role)) || validateDir(dir) != nil {
		return false
	}
	file, seq, err := createTraceFile(dir, role)
	if err != nil {
		return false
	}
	// Without the completion lock a reader could accept a partly flushed
	// trace, so an unlockable trace is withdrawn rather than written.
	if err := lockTrace(file); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return false
	}
	rec := &recorder{file: file, w: bufio.NewWriterSize(file, 64<<10)}
	if err := json.NewEncoder(rec.w).Encode(header{V: FormatVersion, Role: role, Seq: seq, Clock: ClockName, Start: Now()}); err != nil {
		_ = file.Close()
		return false
	}
	active.Store(rec)
	return true
}

// Enabled reports whether this process is recording.
func Enabled() bool { return active.Load() != nil }

// Mark records a point marker at the current uptime.
func Mark(stage Stage, counts ...Count) {
	if rec := active.Load(); rec != nil {
		rec.write(string(stage), goStages[stage], Now(), nil, counts, "")
	}
}

// Span records stage with its start time and duration when the returned
// function runs.
func Span(stage Stage) func(counts ...Count) {
	rec := active.Load()
	if rec == nil {
		return func(...Count) {}
	}
	started := Now()
	return func(counts ...Count) {
		dur := Now() - started
		rec.write(string(stage), goStages[stage], started, &dur, counts, "")
	}
}

// Tx starts timing one write transaction. The returned function records
// tx.<domain>.<label> from begin to the commit or rollback return; a non-nil
// error is recorded with the fixed code "rollback".
func Tx(domain, label string) func(err error) {
	rec := active.Load()
	if rec == nil {
		return func(error) {}
	}
	started := Now()
	return func(err error) {
		dur := Now() - started
		code := ""
		if err != nil {
			code = "rollback"
		}
		valid := identifier.MatchString(domain) && identifier.MatchString(label)
		rec.write("tx."+domain+"."+label, valid, started, &dur, nil, code)
	}
}

// Close writes the process lifetime resources and flushes the trace file.
// Go processes report getrusage(RUSAGE_SELF) only; phys_footprint is App-only.
func Close() error {
	rec := active.Load()
	if rec == nil {
		return nil
	}
	res := lifetimeResources()
	rec.write(string(ResourceLifetime), true, Now(), nil, []Count{
		{"maxrss_bytes", res.maxRSSBytes},
		{"user_ns", res.userNS},
		{"sys_ns", res.sysNS},
	}, "")
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.closed {
		return nil
	}
	rec.closed = true
	if rec.invalid > 0 {
		_ = json.NewEncoder(rec.w).Encode(record{Stage: "trace.invalid", T: Now(), N: map[string]int64{"rejected": rec.invalid}})
	}
	err := errors.Join(rec.w.Flush(), rec.file.Sync(), rec.file.Close())
	active.CompareAndSwap(rec, nil)
	return err
}

func (r *recorder) write(stage string, valid bool, t int64, dur *int64, counts []Count, code string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	var n map[string]int64
	for _, c := range counts {
		if !identifier.MatchString(c.Key) {
			valid = false
			break
		}
		if n == nil {
			n = make(map[string]int64, len(counts))
		}
		n[c.Key] = c.Value
	}
	if !valid {
		// A rejected marker makes the trace tool-invalid instead of leaking text.
		r.invalid++
		return
	}
	_ = json.NewEncoder(r.w).Encode(record{Stage: stage, T: t, Dur: dur, N: n, Err: code})
}

func validateDir(dir string) error {
	if !filepath.IsAbs(dir) {
		return errors.New("perftrace: output directory must be absolute")
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		return errors.New("perftrace: output directory must be a 0700 directory")
	}
	if !ownedByCurrentUser(info) {
		return errors.New("perftrace: output directory must be owned by the current user")
	}
	return nil
}

// createTraceFile names files by role and start sequence, never by path or pid.
func createTraceFile(dir string, role Role) (*os.File, int, error) {
	for seq := 1; seq <= 100000; seq++ {
		name := filepath.Join(dir, fmt.Sprintf("%s-%06d.jsonl", role, seq))
		file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return file, seq, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, 0, err
		}
	}
	return nil, 0, errors.New("perftrace: trace sequence exhausted")
}
