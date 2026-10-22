package p07measure

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func records(t *testing.T, b *bytes.Buffer) []Record {
	t.Helper()
	out := []Record{}
	d := json.NewDecoder(bytes.NewReader(b.Bytes()))
	for d.More() {
		var r Record
		if err := d.Decode(&r); err != nil {
			t.Fatal(err)
		}
		if r.Event != "end" {
			continue
		}
		out = append(out, r)
	}
	return out
}

func TestFullTransactionBoundaries(t *testing.T) {
	var b bytes.Buffer
	Start(&b)
	defer Start(nil)
	for _, domain := range []string{"core", "session"} {
		name := "agentdeck.sqlite3"
		if domain == "session" {
			name = "sessions.sqlite3"
		}
		db, err := Open("sqlite", filepath.Join(t.TempDir(), name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec("CREATE TABLE value(n INTEGER)"); err != nil {
			t.Fatal(err)
		}
		for _, commit := range []bool{true, false} {
			outer := time.Now()
			tx, e := BeginTx("synthetic", db, context.Background(), nil)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = tx.Exec("INSERT INTO value VALUES (1)"); e != nil {
				t.Fatal(e)
			}
			time.Sleep(12 * time.Millisecond)
			if commit {
				e = Commit(tx)
			} else {
				e = Rollback(tx)
			}
			ended := time.Since(outer)
			if e != nil {
				t.Fatal(e)
			}
			if !errors.Is(Rollback(tx), sql.ErrTxDone) {
				t.Fatal("deferred rollback semantics changed")
			}
			r := records(t, &b)
			last := r[len(r)-1]
			if last.Domain != domain || !last.Valid || last.DurationNS < int64(12*time.Millisecond) || last.DurationNS > int64(ended) || last.EndNS-last.BeginNS != last.DurationNS {
				t.Fatalf("bad full boundary: %+v vs %s", last, ended)
			}
		}
		var count int
		if err = db.QueryRow("SELECT count(*) FROM value").Scan(&count); err != nil || count != 1 {
			t.Fatalf("commit/rollback changed: %d %v", count, err)
		}
		if err = CloseDB(db); err != nil {
			t.Fatal(err)
		}
	}
	if r := records(t, &b); len(r) != 4 {
		t.Fatalf("duplicate terminal records: %d", len(r))
	}
	if n := Flush(); n != 0 {
		t.Fatalf("active spans: %d", n)
	}
}

func TestBeginWaitAndFailures(t *testing.T) {
	var b bytes.Buffer
	Start(&b)
	defer Start(nil)
	db, e := Open("sqlite", filepath.Join(t.TempDir(), "agentdeck.sqlite3"))
	if e != nil {
		t.Fatal(e)
	}
	defer CloseDB(db)
	db.SetMaxOpenConns(1)
	conn, e := Conn(db, context.Background())
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, e = BeginTx("pool-wait", db, ctx, nil); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
	r := records(t, &b)
	if len(r) != 1 || r[0].Outcome != "begin_error" || r[0].Valid || r[0].DurationNS < int64(20*time.Millisecond) {
		t.Fatalf("pool wait missing: %+v", r)
	}
	CloseConn(conn)
	ctx2, cancel2 := context.WithCancel(context.Background())
	tx, e := BeginTx("cancel", db, ctx2, nil)
	if e != nil {
		t.Fatal(e)
	}
	cancel2()
	Rollback(tx)
	r = records(t, &b)
	if len(r) != 2 || r[1].Valid || r[1].Outcome != "canceled_endpoint_unobserved" {
		t.Fatalf("hidden rollback counted: %+v", r)
	}
	tx, e = BeginTx("missing-end", db, context.Background(), nil)
	if e != nil {
		t.Fatal(e)
	}
	// Deliberately bypass one terminal wrapper: Flush must expose the gap.
	tx.Rollback()
	if Flush() != 1 {
		t.Fatal("lost transaction was not reported")
	}
	r = records(t, &b)
	if r[2].Valid || r[2].Outcome != "terminal_unobserved" {
		t.Fatal(r[2])
	}
}

func TestPinnedRawTransaction(t *testing.T) {
	var b bytes.Buffer
	Start(&b)
	defer Start(nil)
	db, e := Open("sqlite", filepath.Join(t.TempDir(), "agentdeck.sqlite3"))
	if e != nil {
		t.Fatal(e)
	}
	defer CloseDB(db)
	c, e := Conn(db, context.Background())
	if e != nil {
		t.Fatal(e)
	}
	defer CloseConn(c)
	ctx := context.Background()
	for _, end := range []string{"COMMIT", "ROLLBACK"} {
		if _, e = ExecConn("raw", c, ctx, "BEGIN IMMEDIATE"); e != nil {
			t.Fatal(e)
		}
		if _, e = ExecConn("raw", c, ctx, "CREATE TABLE IF NOT EXISTS value(n)"); e != nil {
			t.Fatal(e)
		}
		time.Sleep(6 * time.Millisecond)
		if _, e = ExecConn("raw", c, ctx, end); e != nil {
			t.Fatal(e)
		}
	}
	r := records(t, &b)
	if len(r) != 2 || r[0].Outcome != "commit" || r[1].Outcome != "rollback" {
		t.Fatalf("raw end missing: %+v", r)
	}
	for _, v := range r {
		if !v.Valid || v.Domain != "core" || v.DurationNS < int64(6*time.Millisecond) {
			t.Fatal(v)
		}
	}
}

func TestConcurrentReadOnlyAndUnknown(t *testing.T) {
	var b bytes.Buffer
	Start(&b)
	defer Start(nil)
	db, e := Open("sqlite", filepath.Join(t.TempDir(), "other.sqlite3"))
	if e != nil {
		t.Fatal(e)
	}
	defer CloseDB(db)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx, err := BeginTx("concurrent", db, context.Background(), &sql.TxOptions{ReadOnly: true})
			if err != nil {
				t.Error(err)
				return
			}
			if err = Commit(tx); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	r := records(t, &b)
	ids := map[uint64]bool{}
	for _, v := range r {
		if ids[v.ID] || !v.ReadOnly || v.Domain != "unknown" {
			t.Fatal(v)
		}
		ids[v.ID] = true
	}
	if len(r) != 16 || Flush() != 0 {
		t.Fatal("records lost")
	}
}

func TestSyntheticOverhead(t *testing.T) {
	// Alternating pairs avoid grouping all warm runs in one mode. These are
	// calibration observations, never P07 acceptance or a guessed overhead pass.
	db, e := sql.Open("sqlite", ":memory:")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = db.Exec("CREATE TABLE value(n INTEGER)"); e != nil {
		t.Fatal(e)
	}
	for pair := 0; pair < 3; pair++ {
		order := []bool{false, true}
		if pair%2 == 1 {
			order = []bool{true, false}
		}
		for _, sink := range []string{"memory", "file"} {
			for _, enabled := range order {
				Start(nil)
				var writer io.Writer = &bytes.Buffer{}
				var output *os.File
				if sink == "file" {
					output, e = os.CreateTemp(t.TempDir(), "transactions-")
					if e != nil {
						t.Fatal(e)
					}
					writer = output
				}
				if enabled {
					Start(writer)
					Register(db, "core")
				}
				started := time.Now()
				for n := 0; n < 200; n++ {
					var tx *sql.Tx
					if enabled {
						tx, e = BeginTx("calibration", db, context.Background(), nil)
					} else {
						tx, e = db.Begin()
					}
					if e != nil {
						t.Fatal(e)
					}
					if _, e = tx.Exec("INSERT INTO value VALUES (1)"); e != nil {
						t.Fatal(e)
					}
					if enabled {
						e = Commit(tx)
					} else {
						e = tx.Commit()
					}
					if e != nil {
						t.Fatal(e)
					}
				}
				t.Logf("P07_SYNTHETIC_OVERHEAD %s", fmt.Sprintf(`{"pair":%d,"enabled":%t,"transactions":200,"wall_ns":%d,"sink":%q}`, pair, enabled, time.Since(started).Nanoseconds(), sink))
				if Flush() != 0 {
					t.Fatal("incomplete calibration")
				}
				if output != nil {
					output.Close()
				}
			}
		}
	}
	Start(nil)
}

func TestPerProcessLogDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var environment []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "AGENTDECK_P07_TX_") {
			environment = append(environment, entry)
		}
	}
	for n := 0; n < 2; n++ {
		cmd := exec.Command(program, "-test.run=^TestDirectoryLogChild$")
		cmd.Env = append(append([]string(nil), environment...), "AGENTDECK_P07_TX_DIR="+dir, "P07_SYNTHETIC_CHILD=1")
		if output, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("synthetic logger child: %v %s", e, output)
		}
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 2 {
		t.Fatalf("per-process files: %d %v", len(files), err)
	}
	identities := map[string]bool{}
	for _, f := range files {
		path := filepath.Join(dir, f.Name())
		info, e := os.Stat(path)
		if e != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("private file mode: %v", e)
		}
		data, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		values := records(t, bytes.NewBuffer(data))
		if len(values) != 2 {
			t.Fatalf("missing child records: %+v", values)
		}
		if identities[values[0].RunID] {
			t.Fatal("reused process log identity")
		}
		identities[values[0].RunID] = true
	}
}

func TestDirectoryLogChild(t *testing.T) {
	if os.Getenv("P07_SYNTHETIC_CHILD") != "1" {
		t.Skip("controlled subprocess fixture")
	}
	for _, domain := range []string{"core", "session"} {
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		Register(db, domain)
		tx, err := BeginTx("synthetic-child", db, context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err = Commit(tx); err != nil {
			t.Fatal(err)
		}
		CloseDB(db)
	}
	if Flush() != 0 {
		t.Fatal("incomplete child span")
	}
}
