package p07measure

import (
	"context"
	"strings"
	"testing"
)

// Shape fixture; session_cte_regression_test.go.tmpl separately replays the
// actual product session.List under the generated overlay, preventing drift.
const sessionListCTEShape = `WITH visible AS (SELECT m.*,row_number() OVER (PARTITION BY m.client,m.session_id ORDER BY s.priority DESC,m.source_path) AS n FROM session_metadata m JOIN session_sources s ON s.source_path=m.source_path) SELECT client,session_id,project,source_path,model,first_at,last_at FROM visible WHERE n=1 ORDER BY last_at DESC,client,session_id`

func TestReadOnlyCTEClassification(t *testing.T) {
	queries := []string{
		sessionListCTEShape,
		"WITH visible AS (SELECT 1) SELECT * FROM visible",
		"/* start */ with visible AS (SELECT '; UPDATE value SET n=9') SELECT * FROM visible; -- end",
		"WITH RECURSIVE cnt(n) AS (VALUES(1) UNION ALL SELECT n+1 FROM cnt WHERE n<3) SELECT n FROM cnt",
		"WITH x(a) AS NOT MATERIALIZED (SELECT 1), y AS MATERIALIZED (SELECT a FROM x) SELECT * FROM y",
		"WITH x AS (WITH y AS (SELECT 1) SELECT * FROM y) SELECT * FROM x",
		"WITH \"AS;()\"(\"SELECT\") AS (SELECT 'it''s ; UPDATE value') SELECT * FROM \"AS;()\"",
		"WITH `visible`([n]) AS (SELECT 1) VALUES(2)",
	}
	for _, query := range queries {
		if got := statementKind(query); got != "read" {
			t.Fatalf("read-only CTE classified as %s: %s", got, query)
		}
	}
}

func TestCTEWritesAndAmbiguityRemainUnknown(t *testing.T) {
	queries := []string{
		"WITH x AS (SELECT 1) UPDATE value SET n=9 RETURNING n",
		"WITH x AS (SELECT 1) INSERT INTO value SELECT * FROM x",
		"WITH x AS (SELECT 1) DELETE FROM value RETURNING n",
		"WITH x AS (SELECT 1) REPLACE INTO value VALUES(2)",
		"WITH x AS (UPDATE value SET n=9 RETURNING n) SELECT * FROM x",
		"WITH x AS (SELECT 1), y AS (DELETE FROM value RETURNING n) SELECT * FROM x",
		"WITH x AS (WITH y AS (INSERT INTO value VALUES(1)) SELECT 1) SELECT * FROM x",
		"WITH x AS (SELECT 1) SELECT * FROM x; UPDATE value SET n=9",
		"WITH x AS (SELECT 1); SELECT * FROM x",
		"WITH x AS (SELECT 1) SELECT * FROM x;;",
		"WITH x (SELECT 1) SELECT * FROM x",
		"WITH x AS NOT (SELECT 1) SELECT * FROM x",
		"WITH x AS () SELECT * FROM x",
		"WITH x AS (SELECT 1 SELECT * FROM x",
		"WITH x AS (SELECT 1)) SELECT * FROM x",
		"WITH x AS (SELECT 1), SELECT * FROM x",
		"WITH 'ambiguous-name' AS (SELECT 1) SELECT 1",
		"WITH [x]] AS (SELECT 1) SELECT 1",
		"WITH x AS (SELECT 'unterminated) SELECT * FROM x",
		"WITH x AS (SELECT 1) /* unterminated",
		"WITH x AS (SELECT 1) SELECT * FROM x; /* ; */ DELETE FROM value",
	}
	for _, query := range queries {
		if got := statementKind(query); got != "unknown" {
			t.Fatalf("unsafe/ambiguous CTE admitted as %s: %s", got, query)
		}
	}
	query := "SELECT 1"
	for i := 0; i < 65; i++ {
		query = "WITH x AS (" + query + ") SELECT * FROM x"
	}
	if statementKind(query) != "unknown" {
		t.Fatal("CTE recursion bound missing")
	}
}

func TestReadOnlyCTEDoesNotCreateWriteRecords(t *testing.T) {
	db, log := implicitFixture(t)
	ctx := context.Background()
	before := log.Len()
	rows, err := QuerySQL("synthetic/read-cte", db, ctx, "WITH visible AS (SELECT n FROM value) SELECT n FROM visible")
	if err != nil {
		t.Fatal(err)
	}
	var values []int
	for rows.Next() {
		var n int
		if err = rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		values = append(values, n)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if len(values) != 1 || values[0] != 0 {
		t.Fatal("read result changed")
	}
	var n int
	if err = ScanRow(QueryRowSQL("synthetic/read-cte-row", db, ctx, "WITH visible AS (SELECT n FROM value) SELECT n FROM visible"), &n); err != nil {
		t.Fatal(err)
	}
	if log.Len() != before || Flush() != 0 {
		t.Fatalf("read-only CTE polluted observer: %s", log.String())
	}
	// A valid write-root CTE executes unchanged but must retain an invalid
	// observation; admitting read-only WITH cannot mask this write.
	if err = ScanRow(QueryRowSQL("synthetic/write-cte", db, ctx, "WITH input AS (SELECT 1) UPDATE value SET n=n+1 RETURNING n"), &n); err != nil || n != 1 {
		t.Fatalf("write semantics changed: %d %v", n, err)
	}
	got := records(t, log)
	if len(got) != 1 || got[0].Valid || !strings.Contains(got[0].Outcome, "unknown") {
		t.Fatalf("write-root CTE falsely passed: %+v", got)
	}
}
