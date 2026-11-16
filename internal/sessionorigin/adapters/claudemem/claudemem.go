// Package claudemem is the read-only claude-mem SDK observer mapping adapter
// (auxiliary-session-classification architecture §3.2). It reads only the
// identifier columns of an explicitly configured claude-mem database whose
// schema matches the supported fingerprint, and never modifies that database
// or its sidecars.
package claudemem

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/kitdine/agent-deck/internal/extension"
)

// DatabaseName is the claude-mem database file inside its data root.
const DatabaseName = "claude-mem.db"

// SupportedSchemaVersion is the highest schema_versions.version written by the
// claude-mem 15.2.1 SessionStore whose source was reviewed (SessionStore.js
// SHA-256 4ed2bf28…e1907c7). A higher or lower version is not adapted.
const SupportedSchemaVersion = 68

// SupportedPluginID and SupportedPackageVersion name the one reviewed plugin
// package. The database schema alone cannot prove that an observer's purpose
// semantics still match it, so the installed package version is confirmed
// first; a higher, lower or unconfirmable version is not adapted.
const (
	SupportedPluginID       = "claude-mem@thedotmack"
	SupportedPackageName    = "claude-mem-plugin"
	pluginName              = "claude-mem"
	SupportedPackageVersion = "15.2.1"
)

// ValidatedVersion labels evidence and run summaries of this adapter version:
// the confirmed package version and the database schema version.
var ValidatedVersion = SupportedPackageVersion + "/sdk-schema-" + strconv.Itoa(SupportedSchemaVersion)

// Outcomes of one adapter run, as stored in origin_adapter_runs.outcome.
const (
	OutcomeOK          = "ok"
	OutcomePartial     = "partial"
	OutcomeUnavailable = "unavailable"
	OutcomeUnsupported = "unsupported"
)

// Safe reasons of an unsuccessful or partial run. They never carry a path.
const (
	ReasonRootInvalid      = "root_invalid"
	ReasonDatabaseMissing  = "database_missing"
	ReasonReadOnlyUnmet    = "read_only_unmet"
	ReasonPermissionDenied = "permission_denied"
	ReasonReadFailed       = "read_failed"
	ReasonTimeout          = "timeout"
	ReasonSchemaVersion    = "schema_version_unsupported"
	ReasonSchemaShape      = "schema_shape_unsupported"
	ReasonPluginUnreadable = "plugin_catalog_unreadable"
	ReasonPluginVersion    = "plugin_version_unsupported"
)

// Bounds of one coordination phase.
const (
	BatchSize    = 200
	ReadTimeout  = 250 * time.Millisecond
	RoundTimeout = 2 * time.Second
)

// The complete query surface. Only identifier and version columns are read;
// content_session_id, platform_source, project, user_prompt and every other
// table are outside it.
const (
	querySchemaVersion = `SELECT max(version) FROM schema_versions`
	queryColumns       = `SELECT name FROM pragma_table_info('sdk_sessions')`
	queryIndexes       = `SELECT name, "unique" FROM pragma_index_list('sdk_sessions')`
	queryIndexColumns  = `SELECT name FROM pragma_index_info(?)`
	queryMappingPrefix = `SELECT id, memory_session_id FROM sdk_sessions WHERE memory_session_id IN `
)

// queryTrace observes every statement in tests.
var queryTrace func(statement string)

// readDelay injects a slow read in tests.
var readDelay func(ctx context.Context) error

var canonicalUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Result is one run over a candidate set.
type Result struct {
	Outcome          string
	Reason           string
	ValidatedVersion string
	// Mapped holds candidates whose exact memory_session_id exists, with the
	// mapping row ID as source token.
	Mapped map[string]int64
	// Checked holds candidates that were looked up, mapped or not. A
	// candidate outside it was not read and must stay unknown.
	Checked map[string]bool
}

// Read looks up candidate Claude canonical session IDs as exact
// memory_session_id values. It first confirms, through Claude Code's
// installed plugin catalog under home, that the reviewed plugin package is
// the installed one. Non-UUID candidates are never queried.
func Read(ctx context.Context, home, dataDir string, candidates []string) Result {
	result := Result{Mapped: map[string]int64{}, Checked: map[string]bool{}}
	fail := func(outcome, reason string) Result {
		result.Outcome, result.Reason = outcome, reason
		return result
	}
	if outcome, reason := confirmPlugin(home); outcome != "" {
		return fail(outcome, reason)
	}
	if !filepath.IsAbs(dataDir) || filepath.Clean(dataDir) != dataDir {
		return fail(OutcomeUnavailable, ReasonRootInvalid)
	}
	info, err := os.Stat(dataDir)
	if err != nil || !info.IsDir() {
		return fail(OutcomeUnavailable, accessReason(err, ReasonRootInvalid))
	}
	path := filepath.Join(dataDir, DatabaseName)
	if reason := readOnlyAccess(path); reason == ReasonSchemaShape {
		return fail(OutcomeUnsupported, reason)
	} else if reason != "" {
		return fail(OutcomeUnavailable, reason)
	}
	round, cancel := context.WithTimeout(ctx, RoundTimeout)
	defer cancel()
	db, err := open(path)
	if err != nil {
		return fail(OutcomeUnavailable, ReasonReadFailed)
	}
	defer db.Close()
	conn, err := db.Conn(round)
	if err != nil {
		return fail(OutcomeUnavailable, timeoutReason(err))
	}
	defer conn.Close()
	if outcome, reason := validateSchema(round, conn); outcome != "" {
		return fail(outcome, reason)
	}
	result.ValidatedVersion = ValidatedVersion
	ids := make([]string, 0, len(candidates))
	for _, id := range candidates {
		if canonicalUUID.MatchString(id) {
			ids = append(ids, id)
		} else {
			// Synthetic or non-UUID IDs are outside this adapter: checked, unmapped.
			result.Checked[id] = true
		}
	}
	for start := 0; start < len(ids); start += BatchSize {
		batch := ids[start:min(start+BatchSize, len(ids))]
		if err := readBatch(round, conn, batch, result.Mapped); err != nil {
			// Candidates already looked up keep their result; the rest stay
			// unknown. Nothing looked up at all is an unavailable source.
			result.Outcome, result.Reason = OutcomePartial, timeoutReason(err)
			if errors.Is(round.Err(), context.DeadlineExceeded) {
				result.Reason = ReasonTimeout
			}
			if start == 0 {
				result.Outcome = OutcomeUnavailable
			}
			return result
		}
		for _, id := range batch {
			result.Checked[id] = true
		}
	}
	result.Outcome = OutcomeOK
	return result
}

// confirmPlugin accepts only when the catalog holds exactly one install of
// claude-mem in any scope or marketplace, and that install is the reviewed
// user-scope package whose catalog version and manifest (claude-mem-plugin)
// both name the reviewed version. Nothing ties an install to the configured
// data root, so a second install of any scope, version or marketplace makes
// the producer of that data unprovable and the adapter refuses; no install is
// filtered out before this check. It reads the catalog and package.json only.
func confirmPlugin(home string) (string, string) {
	if !filepath.IsAbs(home) {
		return OutcomeUnavailable, ReasonPluginUnreadable
	}
	installs, err := extension.ClaudePluginInstalls(home)
	if err != nil {
		return OutcomeUnavailable, ReasonPluginUnreadable
	}
	var matches []extension.ClaudePluginInstall
	for _, install := range installs {
		if name, _, _ := strings.Cut(install.ID, "@"); name == pluginName {
			matches = append(matches, install)
		}
	}
	if len(matches) != 1 {
		return OutcomeUnsupported, ReasonPluginVersion
	}
	if install := matches[0]; install.ID != SupportedPluginID || (install.Scope != "" && install.Scope != "user") {
		return OutcomeUnsupported, ReasonPluginVersion
	}
	install := matches[0]
	if install.Version != SupportedPackageVersion || !filepath.IsAbs(install.InstallPath) || filepath.Clean(install.InstallPath) != install.InstallPath {
		return OutcomeUnsupported, ReasonPluginVersion
	}
	data, err := os.ReadFile(filepath.Join(install.InstallPath, "package.json"))
	if err != nil {
		return OutcomeUnsupported, ReasonPluginVersion
	}
	var manifest struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &manifest) != nil || manifest.Name != SupportedPackageName || manifest.Version != SupportedPackageVersion {
		return OutcomeUnsupported, ReasonPluginVersion
	}
	return "", ""
}

// readOnlyAccess admits only an existing database that a read-only connection
// can read without creating or changing anything: a WAL database must already
// have both sidecars, because a read-only open could otherwise create them.
func readOnlyAccess(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return accessReason(err, ReasonDatabaseMissing)
	}
	defer file.Close()
	header := make([]byte, 20)
	if _, err := io.ReadFull(file, header); err != nil {
		return ReasonReadFailed
	}
	if string(header[:16]) != "SQLite format 3\x00" {
		return ReasonSchemaShape
	}
	if header[18] == 2 || header[19] == 2 {
		for _, suffix := range []string{"-wal", "-shm"} {
			if _, err := os.Stat(path + suffix); err != nil {
				return ReasonReadOnlyUnmet
			}
		}
	}
	return ""
}

func open(path string) (*sql.DB, error) {
	u := url.URL{Scheme: "file", Path: path}
	// mode=ro plus query_only: no write, migration, checkpoint or VACUUM can
	// run; immutable=1 is never used because it would ignore an active WAL.
	db, err := sql.Open("sqlite", u.String()+"?mode=ro&_pragma=query_only(1)&_pragma=busy_timeout("+strconv.Itoa(int(ReadTimeout/time.Millisecond))+")")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func query(ctx context.Context, conn queryer, statement string, args ...any) (*sql.Rows, context.CancelFunc, error) {
	if queryTrace != nil {
		queryTrace(statement)
	}
	read, cancel := context.WithTimeout(ctx, ReadTimeout)
	if readDelay != nil {
		if err := readDelay(read); err != nil {
			cancel()
			return nil, nil, err
		}
	}
	rows, err := conn.QueryContext(read, statement, args...)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	return rows, cancel, nil
}

func validateSchema(ctx context.Context, conn queryer) (string, string) {
	var version sql.NullInt64
	rows, cancel, err := query(ctx, conn, querySchemaVersion)
	if err != nil {
		if isTimeout(err) {
			return OutcomeUnavailable, ReasonTimeout
		}
		// No schema_versions table: not a supported claude-mem database.
		return OutcomeUnsupported, ReasonSchemaShape
	}
	if rows.Next() {
		err = rows.Scan(&version)
	}
	rows.Close()
	cancel()
	if err != nil {
		return OutcomeUnavailable, ReasonReadFailed
	}
	if !version.Valid || version.Int64 != SupportedSchemaVersion {
		return OutcomeUnsupported, ReasonSchemaVersion
	}
	columns, err := names(ctx, conn, queryColumns)
	if err != nil {
		return OutcomeUnavailable, timeoutReason(err)
	}
	if !columns["id"] || !columns["memory_session_id"] {
		return OutcomeUnsupported, ReasonSchemaShape
	}
	// The exact lookup relies on the version's unique memory_session_id index.
	rows, cancel, err = query(ctx, conn, queryIndexes)
	if err != nil {
		return OutcomeUnavailable, timeoutReason(err)
	}
	var unique []string
	for rows.Next() {
		var name string
		var isUnique int
		if err = rows.Scan(&name, &isUnique); err != nil {
			break
		}
		if isUnique == 1 {
			unique = append(unique, name)
		}
	}
	rows.Close()
	cancel()
	if err != nil {
		return OutcomeUnavailable, ReasonReadFailed
	}
	for _, index := range unique {
		indexed, err := names(ctx, conn, queryIndexColumns, index)
		if err != nil {
			return OutcomeUnavailable, timeoutReason(err)
		}
		if len(indexed) == 1 && indexed["memory_session_id"] {
			return "", ""
		}
	}
	return OutcomeUnsupported, ReasonSchemaShape
}

func names(ctx context.Context, conn queryer, statement string, args ...any) (map[string]bool, error) {
	rows, cancel, err := query(ctx, conn, statement, args...)
	if err != nil {
		return nil, err
	}
	defer cancel()
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}

func readBatch(ctx context.Context, conn queryer, batch []string, mapped map[string]int64) error {
	args := make([]any, 0, len(batch))
	for _, id := range batch {
		args = append(args, id)
	}
	rows, cancel, err := query(ctx, conn, queryMappingPrefix+"(?"+strings.Repeat(",?", len(batch)-1)+")", args...)
	if err != nil {
		return err
	}
	defer cancel()
	defer rows.Close()
	found := map[string]int64{}
	for rows.Next() {
		var id int64
		var memory string
		if err := rows.Scan(&id, &memory); err != nil {
			return err
		}
		found[memory] = id
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for memory, id := range found {
		mapped[memory] = id
	}
	return nil
}

func accessReason(err error, fallback string) string {
	if errors.Is(err, os.ErrPermission) {
		return ReasonPermissionDenied
	}
	return fallback
}

func isTimeout(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || (err != nil && strings.Contains(err.Error(), "interrupted"))
}

func timeoutReason(err error) string {
	if isTimeout(err) {
		return ReasonTimeout
	}
	return ReasonReadFailed
}

// SourceToken encodes a mapping row as evidence source token.
func SourceToken(rowID int64) string { return fmt.Sprintf("sdk_sessions:%d", rowID) }
