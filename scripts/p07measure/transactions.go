// Package p07measure is private measurement support. Product packages import it
// only in a generated Go overlay; ordinary binaries have no instrumentation.
package p07measure

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Record contains no SQL, arguments, database paths, or error messages. Duration
// starts immediately before Begin is invoked (including pool/lock wait) and ends
// immediately after the first explicit Commit/Rollback returns. Invalid endings
// are retained but must never enter the completed-write distribution.
type Record struct {
	Event           string   `json:"event"`
	RunID           string   `json:"run_id"`
	ID              uint64   `json:"id"`
	PID             int      `json:"pid"`
	Domain          string   `json:"domain"`
	Site            string   `json:"site"`
	ReadOnly        bool     `json:"read_only"`
	Boundary        string   `json:"boundary"`
	Operation       string   `json:"operation,omitempty"`
	BeginNS         int64    `json:"begin_ns"`
	EndNS           int64    `json:"end_ns"`
	DurationNS      int64    `json:"duration_ns"`
	Outcome         string   `json:"outcome"`
	Valid           bool     `json:"valid"`
	Caller          []string `json:"caller,omitempty"`
	BeginReturnNS   int64    `json:"begin_return_ns,omitempty"`
	TerminalStartNS int64    `json:"terminal_start_ns,omitempty"`
	BeginCallNS     int64    `json:"begin_call_ns,omitempty"`
	BodyNS          int64    `json:"body_ns,omitempty"`
	TerminalNS      int64    `json:"terminal_ns,omitempty"`
	PhasesComplete  bool     `json:"phases_complete"`
}

type span struct {
	record          Record
	started         time.Time
	beginReturned   time.Time
	terminalStarted time.Time
	ctx             context.Context
	autoRollback    bool
	connection      *sql.Conn
	unsupported     string
}

var state = struct {
	sync.Mutex
	writer io.Writer
	epoch  time.Time
	next   uint64
	runID  string
	dbs    map[*sql.DB]string
	conns  map[*sql.Conn]string
	active map[any]span
	connTx map[*sql.Conn]*sql.Tx
}{dbs: map[*sql.DB]string{}, conns: map[*sql.Conn]string{}, active: map[any]span{}, connTx: map[*sql.Conn]*sql.Tx{}}

// Start must precede opening measured databases. It is intentionally explicit
// for synthetic tests. The overlay-only environment entry uses exclusive files.
func Start(w io.Writer) {
	state.Lock()
	defer state.Unlock()
	if len(state.active) != 0 {
		panic("p07: cannot restart with active transactions")
	}
	state.writer, state.epoch, state.next = w, time.Now(), 0
	var identity [16]byte
	if _, err := rand.Read(identity[:]); err != nil {
		panic("p07: run identity unavailable")
	}
	state.runID = hex.EncodeToString(identity[:])
	state.dbs, state.conns = map[*sql.DB]string{}, map[*sql.Conn]string{}
	state.connTx = map[*sql.Conn]*sql.Tx{}
	if w != nil {
		if err := json.NewEncoder(w).Encode(map[string]any{"event": "process", "schema": "p07-transactions/v2", "coverage": "explicit-and-autocommit", "run_id": state.runID, "pid": os.Getpid()}); err != nil {
			panic("p07: transaction log write failed")
		}
	}
}

func init() {
	dir := os.Getenv("AGENTDECK_P07_TX_DIR")
	if dir != "" {
		if os.Getenv("AGENTDECK_P07_TX_LOG") != "" {
			panic("p07: specify a log directory or a single-process log, not both")
		}
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
			panic("p07: existing private log directory required")
		}
		f, err := os.CreateTemp(dir, fmt.Sprintf("transactions-%d-*.jsonl", os.Getpid()))
		if err != nil {
			panic("p07: cannot create per-process transaction log")
		}
		Start(f)
		return
	}
	if name := os.Getenv("AGENTDECK_P07_TX_LOG"); name != "" {
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			panic("p07: cannot create exclusive transaction log")
		}
		Start(f)
	}
}

func Open(driver, dsn string) (*sql.DB, error) {
	db, err := sql.Open(driver, dsn)
	if err == nil {
		path := strings.SplitN(strings.TrimPrefix(dsn, "file:"), "?", 2)[0]
		if decoded, e := url.PathUnescape(path); e == nil {
			path = decoded
		}
		domain := "unknown"
		switch filepath.Base(path) {
		case "agentdeck.sqlite3":
			domain = "core"
		case "sessions.sqlite3":
			domain = "session"
		}
		Register(db, domain)
	}
	return db, err
}

func Register(db *sql.DB, domain string) {
	state.Lock()
	defer state.Unlock()
	if state.writer != nil {
		state.dbs[db] = domain
	}
}

func Conn(db *sql.DB, ctx context.Context) (*sql.Conn, error) {
	c, err := db.Conn(ctx)
	if err == nil {
		state.Lock()
		if state.writer != nil {
			state.conns[c] = state.dbs[db]
		}
		state.Unlock()
	}
	return c, err
}

func CloseDB(db *sql.DB) error {
	err := db.Close()
	state.Lock()
	delete(state.dbs, db)
	state.Unlock()
	return err
}

func CloseConn(c *sql.Conn) error {
	err := c.Close()
	finish(c, time.Now(), "connection_closed", false)
	state.Lock()
	delete(state.conns, c)
	state.Unlock()
	return err
}

func start(receiver any, site string, ctx context.Context, readOnly bool) (span, bool) {
	state.Lock()
	defer state.Unlock()
	if state.writer == nil {
		return span{}, false
	}
	domain := "unknown"
	switch v := receiver.(type) {
	case *sql.DB:
		domain = state.dbs[v]
	case *sql.Conn:
		domain = state.conns[v]
	}
	if domain == "" {
		domain = "unknown"
	}
	state.next++
	s := span{record: Record{Event: "end", RunID: state.runID, ID: state.next, PID: os.Getpid(), Domain: domain, Site: site, ReadOnly: readOnly, Boundary: "explicit"}, ctx: ctx}
	// Function names only: no private paths, SQL, arguments or source content.
	var pcs [16]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.HasPrefix(frame.Function, "github.com/kitdine/agent-deck/") && !strings.Contains(frame.Function, "/scripts/p07measure.") {
			s.record.Caller = append(s.record.Caller, strings.TrimPrefix(frame.Function, "github.com/kitdine/agent-deck/"))
		}
		if !more || len(s.record.Caller) == 6 {
			break
		}
	}
	// Emit the attempt before timing Begin so a killed process or an omitted
	// terminal wrapper leaves a detectable unmatched record, not silent loss.
	if err := json.NewEncoder(state.writer).Encode(map[string]any{"event": "begin", "run_id": state.runID, "pid": s.record.PID, "id": s.record.ID}); err != nil {
		panic("p07: transaction log write failed")
	}
	return s, true
}

func emit(s span, ended time.Time, outcome string, valid bool) {
	s.record.BeginNS = s.started.Sub(state.epoch).Nanoseconds()
	s.record.EndNS = ended.Sub(state.epoch).Nanoseconds()
	s.record.DurationNS = ended.Sub(s.started).Nanoseconds()
	s.record.Outcome, s.record.Valid = outcome, valid
	if s.record.Boundary == "explicit" && !s.beginReturned.IsZero() && !s.terminalStarted.IsZero() {
		s.record.BeginReturnNS = s.beginReturned.Sub(state.epoch).Nanoseconds()
		s.record.TerminalStartNS = s.terminalStarted.Sub(state.epoch).Nanoseconds()
		s.record.BeginCallNS = s.beginReturned.Sub(s.started).Nanoseconds()
		s.record.BodyNS = s.terminalStarted.Sub(s.beginReturned).Nanoseconds()
		s.record.TerminalNS = ended.Sub(s.terminalStarted).Nanoseconds()
		s.record.PhasesComplete = s.record.BeginCallNS >= 0 && s.record.BodyNS >= 0 && s.record.TerminalNS >= 0 && s.record.BeginCallNS+s.record.BodyNS+s.record.TerminalNS == s.record.DurationNS
	}
	if err := json.NewEncoder(state.writer).Encode(s.record); err != nil {
		panic("p07: transaction log write failed")
	}
}

func finish(key any, ended time.Time, outcome string, valid bool, terminal ...time.Time) {
	state.Lock()
	defer state.Unlock()
	if s, exists := state.active[key]; exists {
		if len(terminal) == 1 {
			s.terminalStarted = terminal[0]
		}
		delete(state.active, key)
		if s.connection != nil {
			delete(state.connTx, s.connection)
		}
		// Cancellation can trigger database/sql's hidden rollback before this
		// call. Its return was not observed: keep it, never invent an endpoint.
		if s.autoRollback && s.ctx.Err() != nil {
			outcome, valid = "canceled_endpoint_unobserved", false
		}
		if s.unsupported != "" {
			outcome, valid = s.unsupported, false
		}
		emit(s, ended, outcome, valid)
	}
}

type beginner interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

func BeginTx(site string, db beginner, ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	s, enabled := start(db, site, ctx, opts != nil && opts.ReadOnly)
	s.autoRollback = true
	s.started = time.Now()
	tx, err := db.BeginTx(ctx, opts)
	ended := time.Now()
	s.beginReturned = ended
	if enabled {
		state.Lock()
		if err != nil {
			emit(s, ended, "begin_error", false)
		} else {
			state.active[tx] = s
			if conn, ok := db.(*sql.Conn); ok {
				s.connection = conn
				state.active[tx] = s
				state.connTx[conn] = tx
			}
		}
		state.Unlock()
	}
	return tx, err
}

func Begin(site string, db *sql.DB) (*sql.Tx, error) {
	return BeginTx(site, db, context.Background(), nil)
}

func Commit(tx *sql.Tx) error {
	started := time.Now()
	err := tx.Commit()
	ended := time.Now()
	outcome, valid := "commit", err == nil
	if err != nil {
		outcome = "commit_error"
	}
	if errors.Is(err, sql.ErrTxDone) {
		outcome = "terminal_unobserved"
	}
	finish(tx, ended, outcome, valid, started)
	return err
}

func Rollback(tx *sql.Tx) error {
	started := time.Now()
	err := tx.Rollback()
	ended := time.Now()
	outcome, valid := "rollback", err == nil
	if err != nil {
		outcome = "rollback_error"
	}
	if errors.Is(err, sql.ErrTxDone) {
		outcome = "terminal_unobserved"
	}
	finish(tx, ended, outcome, valid, started)
	return err
}

// ExecConn preserves pinned-connection raw BEGIN IMMEDIATE/COMMIT/ROLLBACK.
// Statements inside an observed explicit transaction do not become additional
// implicit samples. Outside it, use the autocommit statement boundary.
func ExecConn(site string, c *sql.Conn, ctx context.Context, query string, args ...any) (sql.Result, error) {
	command := strings.ToUpper(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(query), ";")))
	isBegin := command == "BEGIN" || command == "BEGIN IMMEDIATE" || command == "BEGIN TRANSACTION" || command == "BEGIN DEFERRED" || command == "BEGIN EXCLUSIVE"
	if !isBegin && command != "COMMIT" && command != "END" && command != "ROLLBACK" {
		return execStatement(site, c, ctx, query, func() (sql.Result, error) { return c.ExecContext(ctx, query, args...) })
	}
	var s span
	var enabled bool
	if isBegin {
		s, enabled = start(c, site, ctx, false)
	}
	s.started = time.Now()
	called := s.started
	result, err := c.ExecContext(ctx, query, args...)
	ended := time.Now()
	s.beginReturned = ended
	if isBegin && enabled {
		state.Lock()
		if err != nil {
			emit(s, ended, "begin_error", false)
		} else {
			state.active[c] = s
		}
		state.Unlock()
	} else if command == "COMMIT" || command == "END" || command == "ROLLBACK" {
		outcome := strings.ToLower(command)
		if command == "END" {
			outcome = "commit"
		}
		if err == nil {
			finish(c, ended, outcome, true, called)
		}
		// Failed raw COMMIT may leave the transaction alive. Its eventual
		// explicit rollback still determines the end, preserving SQLite semantics.
	}
	return result, err
}

// Flush reports any unobserved terminal boundary as invalid and returns the
// number found. Call only after measured workers have joined. No transaction is
// committed, rolled back, retried, or canceled by the measurement package.
func Flush() int {
	state.Lock()
	defer state.Unlock()
	n := len(state.active)
	for key, s := range state.active {
		emit(s, time.Now(), "terminal_unobserved", false)
		delete(state.active, key)
	}
	return n
}
