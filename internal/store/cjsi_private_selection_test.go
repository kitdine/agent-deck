package store

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func testCJSIPrivateSelection(t *testing.T) {
	ctx := t.Context()
	c := newCJSIMigrationCapsule(t)
	c.seed35(t)
	old, err := OpenSessions(ctx, c.root)
	if err != nil {
		t.Fatal(err)
	}
	cjsiFixtureExec(t, old.DB, `INSERT INTO session_sources(source_path,identity,cursor,partial_line,size,modified_at,prefix_hash,priority,parser_version,scanned_at)
	VALUES('a','one',9,x'7461696c',9,1,'p',3,6,''),('b','two',9,x'',9,1,'q',5,6,'');
	INSERT INTO session_metadata VALUES('a','codex','s','p','m',6,'',''),('b','codex','s','x','m',6,'','');
	INSERT INTO session_exclusions VALUES('project','x');
	INSERT INTO session_documents(rowid,source_path,client,session_id,event_at,kind,text) VALUES
	 (11,'a','codex','s','','user','retained'),(90,'a','codex','s','','user','removed'),
	 (97,'b','codex','s','','user','excluded'),(99,'unlinked','codex','s','','user','unlinked');
	INSERT INTO inventory_runs VALUES('inventory',1,'t',0,'complete');
	INSERT INTO domain_coverage_heads VALUES(1,'inventory')`)
	var oldRev int
	if err = old.DB.QueryRow("SELECT revision FROM serving_generation").Scan(&oldRev); err != nil {
		t.Fatal(err)
	}
	if err = old.Close(); err != nil {
		t.Fatal(err)
	}
	if err = migrateCJSIManagedRoot(ctx, c.root, 0, c); err != nil {
		t.Fatal(err)
	}
	core := c.openCore(t, "rw")
	defer core.Close()
	path := filepath.Join(c.root, cjsiPrivateIndexName)
	db, err := sql.Open("sqlite", cjsiSQLiteURI(path, "rw")+"&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	t.Cleanup(func() {
		_ = closeCJSIInstallReadPool(path)
		_ = closeCJSIInstallReadPool(filepath.Join(c.root, "agentdeck.sqlite3"))
	})
	cjsiFixtureExec(t, db, "PRAGMA journal_mode=WAL")
	t.Run("candidate FTS column filters", func(t *testing.T) {
		tx := beginCJSIFixtureTx(t, db)
		defer tx.Rollback()
		id, _, err := ReserveCJSIDocumentIDs(ctx, tx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO cjsi_candidate_payloads(doc_id,event_at,kind,text)
			VALUES(?,'timestampword','kindword','searchword')`, id); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			query string
			want  int
		}{
			{"{client text}:searchword", 1},
			{"{source_path text}:searchword", 1},
			{"{session_id text}:searchword", 1},
			{"{client source_path session_id}:searchword", 0},
			{"event_at:timestampword OR kind:kindword", 0},
		} {
			t.Run(tc.query, func(t *testing.T) {
				var count int
				err := tx.QueryRowContext(ctx, `SELECT count(*) FROM cjsi_candidate_documents
					WHERE rowid=? AND cjsi_candidate_documents MATCH ?`, id, tc.query).Scan(&count)
				if err != nil || count != tc.want {
					t.Fatalf("MATCH %q at payload %d: count=%d, want %d, err=%v", tc.query, id, count, tc.want, err)
				}
			})
		}
		var columns string
		if err := tx.QueryRowContext(ctx, `SELECT group_concat(name, ',' ORDER BY cid)
			FROM pragma_table_info('cjsi_candidate_documents')`).Scan(&columns); err != nil || columns != "source_path,client,session_id,event_at,kind,text" {
			t.Fatalf("candidate FTS columns in declaration order: %q, err=%v", columns, err)
		}
		var gotID int64
		if err := tx.QueryRowContext(ctx, `SELECT rowid FROM cjsi_candidate_documents WHERE cjsi_candidate_documents MATCH 'searchword'
			AND source_path IS NULL AND client IS NULL AND session_id IS NULL AND event_at='timestampword' AND kind='kindword' AND text='searchword'`).Scan(&gotID); err != nil || gotID != id {
			t.Fatalf("external-content rowid=%d, want %d: %v", gotID, id, err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM cjsi_candidate_payloads WHERE doc_id=?", id); err != nil {
			t.Fatal(err)
		}
		var remaining int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM cjsi_candidate_documents WHERE cjsi_candidate_documents MATCH 'searchword'`).Scan(&remaining); err != nil || remaining != 0 {
			t.Fatalf("payload deletion left %d FTS hits: %v", remaining, err)
		}
	})
	assertCJSIMigrationNumber(t, db, `SELECT count(*) FROM session_sources WHERE source_path='a' AND cursor=5 AND length(partial_line)=0`, 1)
	assertCJSIMigrationNumber(t, db, `SELECT count(*) FROM cjsi_legacy_cursor_hints WHERE source_path='a' AND read_end=9 AND complete_end=5 AND parser_version=6`, 1)
	assertCJSIMigrationNumber(t, db, `SELECT count(*) FROM inventory_runs WHERE state='complete'`, 1)
	assertCJSIMigrationNumber(t, db, "SELECT base_revision FROM cjsi_index_state", oldRev)
	assertCJSIMigrationNumber(t, core, "SELECT count(*) FROM sqlite_master WHERE name LIKE 'cjsi_candidate_%'", 0)
	assertCJSIMigrationNumber(t, db, "SELECT count(*) FROM sqlite_master WHERE name LIKE 'cjsi_receipt_%'", 0)
	read := func(fn func(CJSIIndexSnapshot) error) error {
		tx, err := core.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		return WithCJSIIndexSnapshot(ctx, c.root, tx, fn)
	}
	var oldID, newID CJSIDetailIdentity
	details := func(s CJSIIndexSnapshot, want string, id *CJSIDetailIdentity) error {
		got, err := s.DetailIdentity(ctx, "codex", "s")
		if err != nil {
			return err
		}
		if got.SourcePath != "a" || got.Stamp.Inode == 0 {
			t.Fatalf("bad selected identity: %+v", got)
		}
		if got.Stamp != s.Selection() {
			t.Fatal("detail identity not from selected view")
		}
		*id = got
		var ids string
		err = s.Tx.QueryRow(`SELECT group_concat(doc_id||':'||detail_ordinal||':'||doc_side,',') FROM
		 (SELECT * FROM (` + s.MembersSQL() + `) ORDER BY detail_ordinal)`).Scan(&ids)
		if err == nil && ids != want {
			t.Fatalf("got members %s; want %s", ids, want)
		}
		return err
	}
	baseSequence := "11:1:base,90:2:base"
	if err = read(func(s CJSIIndexSnapshot) error { return details(s, baseSequence, &oldID) }); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, last, err := ReserveCJSIDocumentIDs(ctx, tx, 2)
	if err != nil || first != 100 || last != 101 {
		t.Fatalf("allocation %d..%d %v", first, last, err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("synthetic candidate seal"))
	h := oldID.Stamp
	for _, attempt := range []int{1, 2} {
		cjsiFixtureExec(t, db, `INSERT INTO cjsi_session_candidates(ticket,attempt,core_epoch,index_epoch,base_ticket,base_attempt,
		base_seal,base_revision,parent_ticket,protection_revision,coverage_revision,session_revision)
		VALUES(1,?,?,?,0,0,?,?,0,0,0,?)`, attempt, h.CoreEpoch, h.IndexEpoch, h.BaseSeal[:], h.BaseRevision, h.BaseRevision+1)
		cjsiFixtureExec(t, db, `INSERT INTO cjsi_candidate_groups(ticket,attempt,source_path,client,session_id,op,
		project,model,parser_version,first_at,last_at,order_verified,sequence_id)
		VALUES(1,?,'a','codex','s','replace','p','m',7,'','',1,randomblob(32))`, attempt)
	}
	cjsiFixtureExec(t, db, `INSERT INTO cjsi_candidate_payloads VALUES(100,'','user','newword'),(101,'','user','unopened');
	INSERT INTO cjsi_candidate_sources(ticket,attempt,source_path,op,source_kind,identity,priority,parser_version,complete_end,
	  size,modified_at,changed_at,prefix_hash,scanned_at) VALUES(1,1,'a','upsert','codex-jsonl','one',3,7,9,9,1,1,'p','t');
	INSERT INTO cjsi_candidate_ranges VALUES(1,1,'a','codex','s',7,2,'one',3,4,9,65536,9,9,0,9,randomblob(32));
	INSERT INTO cjsi_candidate_order(ticket,attempt,source_path,client,session_id,doc_id,doc_side,detail_ordinal) VALUES
	(1,1,'a','codex','s',11,'base',2),(1,1,'a','codex','s',100,'candidate',3),
	(1,2,'a','codex','s',101,'candidate',1);
	INSERT INTO cjsi_candidate_doc_masks VALUES(1,1,'a','codex','s',90)`)
	mustRefuse := func(query string, args ...any) {
		t.Helper()
		if _, e := db.Exec(query, args...); e == nil {
			t.Fatalf("accepted invalid change: %s", query)
		}
	}
	mustRefuse(`UPDATE cjsi_session_candidates SET seal_digest=? WHERE attempt=1`, digest[:])
	cjsiFixtureExec(t, db, `UPDATE cjsi_candidate_order SET detail_ordinal=1 WHERE attempt=1 AND doc_id=100;
	UPDATE cjsi_session_candidates SET seal_digest=? WHERE attempt=1`, digest[:])
	mustRefuse(`UPDATE cjsi_candidate_groups SET sequence_id=randomblob(32) WHERE attempt=1`)
	mustRefuse(`INSERT OR REPLACE INTO cjsi_candidate_payloads VALUES(100,'','user','other')`)
	mustRefuse(`DELETE FROM cjsi_candidate_payloads WHERE doc_id=100`)
	tentative := beginCJSIFixtureTx(t, core)
	if _, err = tentative.Exec(`UPDATE cjsi_publication_head SET ticket=1,session_ticket=1,session_attempt=1,
		session_revision=base_revision+1,session_seal=?`, digest[:]); err != nil {
		t.Fatal(err)
	}
	err = WithCJSIIndexSnapshot(ctx, c.root, tentative, func(CJSIIndexSnapshot) error { t.Fatal("uncommitted head admitted callback"); return nil })
	if !errors.Is(err, ErrCJSIIndexChanged) {
		t.Fatalf("uncommitted head: %v", err)
	}
	if err = tentative.Rollback(); err != nil {
		t.Fatal(err)
	}
	// A sealed but unselected attempt is still invisible. A later publication
	// during the callback must invalidate the entire buffered result.
	if err = read(func(s CJSIIndexSnapshot) error {
		var id CJSIDetailIdentity
		if e := details(s, baseSequence, &id); e != nil {
			return e
		}
		if id != oldID {
			t.Fatal("private preparation changed reopened identity")
		}
		var n int
		if e := s.Tx.QueryRow(`SELECT count(*) FROM (` + s.RangesSQL() + `)`).Scan(&n); e != nil || n != 0 {
			t.Fatalf("unselected proof %d: %v", n, e)
		}
		cjsiFixtureExec(t, core, `UPDATE cjsi_publication_head SET ticket=1, session_ticket=1,session_attempt=1,
		  session_revision=base_revision+1,session_seal=? WHERE singleton=1`, digest[:])
		return nil
	}); !errors.Is(err, ErrCJSIIndexChanged) {
		t.Fatalf("accepted obsolete global ticket: %v", err)
	}
	if err = read(func(s CJSIIndexSnapshot) error {
		if e := details(s, "100:1:candidate,11:2:base", &newID); e != nil {
			return e
		}
		if newID == oldID || newID.Sequence == oldID.Sequence || newID.Stamp.Ticket != 1 {
			t.Fatal("viewer could stitch two generations")
		}
		var round, attempt, generation int
		if e := s.Tx.QueryRow(`SELECT workset_round,workset_attempt,generation FROM (`+s.RangesSQL()+`)`).Scan(&round, &attempt, &generation); e != nil || round != 7 || attempt != 2 || generation != 4 {
			t.Fatalf("selected proof %d/%d generation %d: %v", round, attempt, generation, e)
		}
		for _, test := range []struct {
			side, table, word string
			want              int
		}{
			{"base", "session_documents", "retained", 1}, {"base", "session_documents", "removed", 0},
			{"candidate", "cjsi_candidate_documents", "newword", 1}, {"candidate", "cjsi_candidate_documents", "unopened", 0},
		} {
			var n int
			q := `SELECT count(*) FROM (` + s.MembersSQL() + `) m JOIN ` + test.table + ` d ON d.rowid=m.doc_id WHERE m.doc_side=? AND ` + test.table + ` MATCH ?`
			if e := s.Tx.QueryRow(q, test.side, test.word).Scan(&n); e != nil {
				return e
			}
			if n != test.want {
				t.Fatalf("%s MATCH %s: got %d", test.side, test.word, n)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = read(func(s CJSIIndexSnapshot) error {
		got, e := s.DetailIdentity(ctx, "codex", "s")
		if e != nil || got != newID {
			t.Fatalf("reopened identity %+v: %v", got, e)
		}
		cjsiFixtureExec(t, db, `INSERT INTO cjsi_session_protections VALUES('a','codex','s',100)`)
		return nil
	}); !errors.Is(err, ErrCJSIIndexChanged) {
		t.Fatalf("revocation during callback ignored: %v", err)
	}
	deny := func(CJSIIndexSnapshot) error { t.Fatal("mismatched selection invoked callback"); return nil }
	if err = read(deny); !errors.Is(err, ErrCJSIIndexChanged) {
		t.Fatalf("unpaired protection revision: %v", err)
	}
	cjsiFixtureExec(t, core, "UPDATE cjsi_publication_head SET protection_revision=1")
	if err = read(func(s CJSIIndexSnapshot) error {
		var id CJSIDetailIdentity
		if e := details(s, "11:1:base", &id); e != nil {
			return e
		}
		if id == newID {
			t.Fatal("protection change retained viewer identity")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	cjsiFixtureExec(t, core, "UPDATE cjsi_publication_head SET session_seal=randomblob(32)")
	if err = read(deny); !errors.Is(err, ErrCJSIIndexChanged) {
		t.Fatalf("unmatched session seal: %v", err)
	}

	// A cumulative delta moves a surviving base row. Its effective path is
	// c, while its physical FTS payload remains at a until compaction.
	moveSeal := sha256.Sum256([]byte("synthetic move"))
	cjsiFixtureExec(t, db, `INSERT INTO cjsi_session_candidates(ticket,attempt,core_epoch,index_epoch,base_ticket,base_attempt,
		  base_seal,base_revision,parent_ticket,protection_revision,coverage_revision,session_revision)
		SELECT 2,1,core_epoch,index_epoch,base_ticket,base_attempt,base_seal,base_revision,1,1,0,session_revision+1
		FROM cjsi_session_candidates WHERE ticket=1 AND attempt=1;
		INSERT INTO cjsi_candidate_sources(ticket,attempt,source_path,op) VALUES(2,1,'a','delete');
		INSERT INTO cjsi_candidate_sources SELECT 2,1,'c',op,source_kind,identity,priority,parser_version,complete_end,size,
		  modified_at,changed_at,prefix_hash,scanned_at FROM cjsi_candidate_sources WHERE ticket=1 AND attempt=1;
		INSERT INTO cjsi_candidate_groups SELECT 2,1,'c',client,session_id,op,project,model,parser_version,first_at,last_at,
		  order_verified,randomblob(32),head_at,birth FROM cjsi_candidate_groups WHERE ticket=1 AND attempt=1;
		INSERT INTO cjsi_candidate_order(ticket,attempt,source_path,client,session_id,doc_id,doc_side,detail_ordinal)
		  VALUES(2,1,'c','codex','s',11,'base',1);
		UPDATE cjsi_session_candidates SET seal_digest=? WHERE ticket=2`, moveSeal[:])
	cjsiFixtureExec(t, core, `UPDATE cjsi_publication_head SET ticket=2,session_ticket=2,session_revision=base_revision+2,session_seal=?`, moveSeal[:])
	var moved CJSIDetailIdentity
	checkMove := func(s CJSIIndexSnapshot, physical string) error {
		var id int64
		var path, origin string
		if e := s.Tx.QueryRow(`SELECT doc_id,source_path,payload_path FROM (`+s.MembersSQL()+`)`).Scan(&id, &path, &origin); e != nil {
			return e
		}
		if id != 11 || path != "c" || origin != physical {
			t.Fatalf("wrong location %d %s:%s", id, path, origin)
		}
		identity, e := s.DetailIdentity(ctx, "codex", "s")
		if e != nil {
			return e
		}
		if physical == "a" {
			moved = identity
		} else if identity != moved {
			t.Fatal("physical compaction changed detail identity")
		}
		return nil
	}
	if err = read(func(s CJSIIndexSnapshot) error { return checkMove(s, "a") }); err != nil {
		t.Fatal(err)
	}
	// Simulate the compactor's committed physical postimage. This tests
	// reader selection, not a compactor or final publication implementation.
	cjsiFixtureExec(t, db, `BEGIN;
		INSERT INTO session_sources SELECT 'c',identity,cursor,partial_line,size,modified_at,changed_at,prefix_hash,
		  priority,7,scanned_at,parser_context FROM session_sources WHERE source_path='a';
		INSERT INTO session_metadata SELECT 'c',client,session_id,project,model,7,first_at,last_at FROM session_metadata WHERE source_path='a';
		INSERT INTO cjsi_base_groups SELECT source_path,client,session_id,order_verified,sequence_id,head_at,birth,'codex-jsonl'
		  FROM cjsi_candidate_groups WHERE ticket=2;
		UPDATE cjsi_base_order SET source_path='c' WHERE doc_id=11;
		UPDATE session_documents SET source_path='c' WHERE rowid=11;
		DELETE FROM session_documents WHERE rowid=90;
		DELETE FROM session_sources WHERE source_path='a';
		UPDATE cjsi_index_state SET compacted_ticket=2,compacted_attempt=1,compacted_seal=?,base_revision=base_revision+2;
		UPDATE serving_generation SET revision=(SELECT base_revision FROM cjsi_index_state);
		COMMIT`, moveSeal[:])
	if err = read(func(s CJSIIndexSnapshot) error { return checkMove(s, "c") }); err != nil {
		t.Fatal(err)
	}
	cjsiFixtureExec(t, core, `UPDATE cjsi_publication_head SET ticket=1,session_ticket=1,session_revision=base_revision+1,session_seal=?`, digest[:])
	if err = read(deny); !errors.Is(err, ErrCJSIIndexChanged) {
		t.Fatalf("new physical base answered older ticket: %v", err)
	}
}

func testCJSIC2Migration(t *testing.T) {
	ctx := t.Context()
	t.Run("typed core evidence and receipt keys", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		if err := migrateCJSIManagedRoot(ctx, c.root, 0, c); err != nil {
			t.Fatal(err)
		}
		db, err := sql.Open("sqlite", cjsiSQLiteURI(filepath.Join(c.root, "agentdeck.sqlite3"), "rw")+"&_pragma=foreign_keys(1)")
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		cjsiFixtureExec(t, db, `INSERT INTO cjsi_publication_attempts(ticket,attempt,base_ticket,core_epoch,index_epoch,session_revision,usage_revision,
		  user_revision,route_revision,price_revision,protection_revision,coverage_revision,input_digest,accept_until,state)
		  SELECT 1,1,0,core_epoch,0,0,0,0,0,0,0,0,randomblob(32),'t','draft' FROM cjsi_publication_head;
		  INSERT INTO cjsi_usage_groups VALUES(1,1,'s',randomblob(32),randomblob(32),3,0,0,0,0);
		  INSERT INTO cjsi_usage_sources(ticket,attempt,thread,name_digest,selection_epoch,workset_round,workset_attempt,
		    identity,generation,replay_parser,max_line_bytes,read_end,complete_end,prefix_digest)
		  VALUES(1,1,'s',randomblob(32),2,7,2,'source',4,9,4096,17,13,randomblob(32));
		  INSERT INTO cjsi_usage_members VALUES(1,1,'s',3,'ints','t','m',NULL,NULL,9007199254740993,1,0,NULL,NULL);
		  INSERT INTO cjsi_usage_intervals VALUES(1,1,'s',3,'ints',randomblob(32),NULL,'t',0,9007199254740993,0,1,0,0,NULL,NULL,NULL,NULL);
		  INSERT INTO cjsi_old_preconditions VALUES(1,1,'s','old',randomblob(32),'hold');
		  INSERT INTO cjsi_takeovers VALUES('codex','s',1,1,NULL,1,1,1,randomblob(32))`)
		assertCJSIMigrationNumber(t, db, `SELECT count(*) FROM cjsi_usage_intervals WHERE upper_input=9007199254740993 AND upper_cached=0 AND upper_write IS NULL AND lower_at IS NULL`, 1)
		assertCJSIMigrationNumber(t, db, `SELECT count(*) FROM cjsi_usage_sources
		  WHERE ticket=1 AND attempt=1 AND workset_round=7 AND workset_attempt=2`, 1)
		assertCJSIMigrationNumber(t, db, `SELECT count(*) FROM cjsi_usage_selection`, 0)
		t.Run("interval stamp schema", func(t *testing.T) { testCJSIIntervalDirtyRangeSchema(t, db) })
		assertCJSIMigrationNumber(t, db, `SELECT ticket FROM cjsi_publication_head`, 0)
		if _, err = db.Exec(`INSERT INTO cjsi_takeovers VALUES('codex','s',2,1,NULL,1,1,1,randomblob(32))`); err == nil {
			t.Fatal("second open takeover period")
		}
		if _, err = db.Exec(`DELETE FROM cjsi_usage_groups WHERE thread='s'`); err == nil {
			t.Fatal("discarded supporting proof of a retained receipt")
		}
		cjsiFixtureExec(t, db, `INSERT INTO cjsi_receipt_observations VALUES('codex','s',1,'obs','otel_owned','settled',1,
		  'support','import','producer','1','origin','class','transport','model','kind','event','req','resp','attempt',
		  'original-record','original-observed','original-received',9,1,0,NULL,NULL,10,randomblob(32),'applied');
		  INSERT INTO cjsi_receipt_events(client,thread,period,observation_key,event_key,session_id,event_id,event_at,model,source_path,source_offset,
		  input_tokens,cached_input_tokens,output_tokens,cache_read_tokens,cache_creation_tokens,cache_write_tokens,cache_write_5m_tokens,cache_write_1h_tokens,
		  run_id,binding_run_id,provider,quality,multiplier,catalog_version) VALUES
		  ('codex','s',1,'obs','key','s','event','original-record','model','otel:fixture',1,9,0,1,0,0,0,0,0,5,5,'provider','exact','1.2','original-catalog');
		  INSERT INTO cjsi_receipt_markers VALUES('codex','s',1,'otel','model','first','last','support','settled');
		  UPDATE cjsi_takeovers SET closed_ticket=2 WHERE period=1`)
		assertCJSIMigrationNumber(t, db, `SELECT count(*) FROM cjsi_receipt_observations o JOIN cjsi_receipt_events e USING(client,thread,period,observation_key)
		 WHERE o.cached_tokens=0 AND o.cache_write_tokens IS NULL AND e.multiplier='1.2' AND e.event_at=o.record_at AND e.binding_run_id=5`, 1)
	})
	t.Run("upgrade rollback restores legacy trigger", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		c.seed35(t)
		db := c.openCore(t, "rw")
		defer db.Close()
		abortCJSIVersion37Write(t, c)
		assertCJSIMigrationNumber(t, db, `SELECT version FROM schema_metadata`, cjsiBaseCoreSchemaVersion)
		assertCJSIMigrationNumber(t, db, `SELECT count(*) FROM sqlite_master WHERE name='telemetry_jsonl_retracts_otel'`, 1)
		assertCJSIMigrationNumber(t, db, `SELECT count(*) FROM sqlite_master WHERE name IN ('cjsi_publication_head','cjsi_takeovers','cjsi_layout_journal','cjsi_checkpoint_attempts','cjsi_usage_interval_dirty_ranges')`, 0)
		if err := migrateCJSIManagedRoot(ctx, c.root, 0, c); err != nil {
			t.Fatal(err)
		}
		assertCJSIMigrationNumber(t, db, `SELECT count(*) FROM sqlite_master WHERE name='telemetry_jsonl_retracts_otel'`, 0)
		t.Run("interval stamp schema", func(t *testing.T) { testCJSIIntervalDirtyRangeSchema(t, db) })
	})
	t.Run("ready head failure resumes never-built without creating index", func(t *testing.T) {
		c := newCJSIMigrationCapsule(t)
		if err := withCJSIManagedCore(ctx, c.root, 0, c, func(root string, core *sql.DB) error {
			if err := migrateCJSICore(ctx, root, core); err != nil {
				return err
			}
			// The ready transition requires coverage_revision=0 on the initial head.
			_, err := core.Exec(`UPDATE cjsi_publication_head SET coverage_revision=1 WHERE singleton=1`)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if err := migrateCJSIManagedRoot(ctx, c.root, 0, c); !errors.Is(err, errCJSILayoutInvalid) {
			t.Fatalf("expected ready head precondition failure, got %v", err)
		}
		db := c.openCore(t, "rw")
		defer db.Close()
		assertCJSIMigrationNumber(t, db, `SELECT phase FROM cjsi_layout_journal`, int(cjsiFenceDurable))
		cjsiFixtureExec(t, db, `UPDATE cjsi_publication_head SET coverage_revision=0 WHERE singleton=1`)
		if err := migrateCJSIManagedRoot(ctx, c.root, 0, c); err != nil {
			t.Fatal(err)
		}
		tx := beginCJSIFixtureTx(t, db)
		defer tx.Rollback()
		err := WithCJSIIndexSnapshot(ctx, c.root, tx, func(CJSIIndexSnapshot) error { t.Fatal("absent index opened"); return nil })
		if !errors.Is(err, ErrCJSIIndexAbsent) {
			t.Fatalf("valid absent: %v", err)
		}
		if err = cjsiRequireCleanIndexPath(filepath.Join(c.root, cjsiPrivateIndexName)); err != nil {
			t.Fatal(err)
		}
		assertCJSIMigrationNumber(t, db, `SELECT count(*) FROM sqlite_master WHERE name='telemetry_jsonl_retracts_otel'`, 0)
	})
	t.Run("private DDL late failure rolls back allocator and seal", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), cjsiPreparedName)
		createCJSIPrivateFixture(t, path)
		db := openCJSIFixtureDB(t, path, "rw")
		cjsiFixtureExec(t, db, cjsiSessionBaselineSchema+`CREATE TABLE cjsi_candidate_coverage(collision INTEGER)`)
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		if _, _, err := prepareCJSIProductionIndex(ctx, path); err == nil {
			t.Fatal("private collision accepted")
		}
		db = openCJSIFixtureDB(t, path, "rw")
		defer db.Close()
		assertCJSIMigrationNumber(t, db, `SELECT count(*) FROM sqlite_master WHERE name IN ('cjsi_layout_initial_seal','cjsi_index_state','cjsi_candidate_documents')`, 0)
	})
}

func testCJSIIntervalDirtyRangeSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := t.Context()
	assertCJSIMigrationNumber(t, db, `SELECT count(*) FROM pragma_table_list
		WHERE schema='main' AND name='cjsi_usage_interval_dirty_ranges' AND type='table' AND ncol=7 AND wr=1 AND strict=1`, 1)
	assertCJSIMigrationNumber(t, db, `SELECT count(*) FROM pragma_table_info('cjsi_usage_interval_dirty_ranges')
		WHERE "notnull"=1 AND type=CASE WHEN name IN ('scope_kind','client','model') THEN 'TEXT' ELSE 'INTEGER' END`, 7)
	var key string
	if err := db.QueryRowContext(ctx, `SELECT group_concat(name, ',' ORDER BY pk)
		FROM pragma_table_info('cjsi_usage_interval_dirty_ranges') WHERE pk>0`).Scan(&key); err != nil {
		t.Fatal(err)
	}
	if key != "core_epoch,scope_kind,client,model,from_day,to_day" {
		t.Fatalf("interval stamp primary key = %q", key)
	}

	const insert = `INSERT INTO main.cjsi_usage_interval_dirty_ranges
		(core_epoch,scope_kind,client,model,from_day,to_day,ticket) VALUES `
	assertCJSIMigrationNumber(t, db, `SELECT count(*) FROM main.cjsi_usage_interval_dirty_ranges`, 0)
	cjsiFixtureExec(t, db, insert+`
		(1,'aggregate-only','codex','',-2,0,1),
		(1,'client-summary','codex','',-2,0,1),
		(1,'all-summary','','',-2,0,1),
		(1,'coverage','codex','',-2,0,1),
		(1,'model','codex','',-2,0,1),
		(1,'model','codex','m',-2,0,1)`)
	for _, tc := range []struct {
		name, row string
		code      int
	}{
		{"same key with new ticket", "(1,'model','codex','m',-2,0,2)", sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY},
		{"unknown scope", "(1,'other','codex','',0,1,1)", sqlite3.SQLITE_CONSTRAINT_CHECK},
		{"all-summary with client", "(1,'all-summary','codex','',0,1,1)", sqlite3.SQLITE_CONSTRAINT_CHECK},
		{"missing client", "(1,'coverage','','',0,1,1)", sqlite3.SQLITE_CONSTRAINT_CHECK},
		{"model on another scope", "(1,'client-summary','codex','m',0,1,1)", sqlite3.SQLITE_CONSTRAINT_CHECK},
		{"empty range", "(1,'model','codex','m',0,0,1)", sqlite3.SQLITE_CONSTRAINT_CHECK},
		{"reversed range", "(1,'model','codex','m',1,0,1)", sqlite3.SQLITE_CONSTRAINT_CHECK},
		{"zero epoch", "(0,'model','codex','m',0,1,1)", sqlite3.SQLITE_CONSTRAINT_CHECK},
		{"zero ticket", "(1,'model','codex','m',0,1,0)", sqlite3.SQLITE_CONSTRAINT_CHECK},
	} {
		_, err := db.ExecContext(ctx, insert+tc.row)
		var sqliteErr *sqlite.Error
		if !errors.As(err, &sqliteErr) || sqliteErr.Code() != tc.code {
			t.Errorf("%s: got %v, want SQLite constraint code %d", tc.name, err, tc.code)
		}
	}
}
