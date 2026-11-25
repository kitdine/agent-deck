package backup

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kitdine/agent-deck/internal/session"
	"github.com/kitdine/agent-deck/internal/store"
)

// These tables belong to the archive's session entry, not to a live index.
// They preserve the selected order without granting an epoch or a ticket.
const portableSessionOrderSchema = `
CREATE TABLE portable_session_state (
	singleton INTEGER PRIMARY KEY CHECK(singleton=1),
	doc_id_high_water INTEGER NOT NULL CHECK(doc_id_high_water>=0)
) STRICT;
CREATE TABLE portable_session_groups (
	source_path TEXT NOT NULL, client TEXT NOT NULL, session_id TEXT NOT NULL,
	order_verified INTEGER NOT NULL CHECK(order_verified IN (0,1)),
	PRIMARY KEY(source_path,client,session_id)
) STRICT, WITHOUT ROWID;
CREATE TABLE portable_session_order (
	doc_id INTEGER PRIMARY KEY CHECK(doc_id>0),
	source_path TEXT NOT NULL, client TEXT NOT NULL, session_id TEXT NOT NULL,
	detail_ordinal INTEGER NOT NULL CHECK(detail_ordinal>0),
	UNIQUE(source_path,client,session_id,detail_ordinal),
	FOREIGN KEY(source_path,client,session_id) REFERENCES portable_session_groups
) STRICT;`

// writeSelectedSessionOrder runs inside the session materializer's transaction,
// after it has inserted the eligible metadata and FTS rows into a clean
// database. The caller must discard the output if its published-view borrow
// fails revalidation. No physical order or allocator state is taken from an
// unselected attempt.
func writeSelectedSessionOrder(ctx context.Context, dst *sql.Tx, index store.CJSIIndexSnapshot) error {
	order, err := session.ReadCJSIOrderSnapshot(ctx, index)
	if err != nil {
		return err
	}
	groups, err := index.Tx.QueryContext(ctx, `SELECT source_path,client,session_id,order_verified FROM (`+index.CopiesSQL()+`)`)
	if err != nil {
		return err
	}
	defer groups.Close()
	if _, err := dst.ExecContext(ctx, portableSessionOrderSchema); err != nil {
		return err
	}
	if _, err := dst.ExecContext(ctx, "INSERT INTO portable_session_state VALUES(1,?)", order.HighWater); err != nil {
		return err
	}
	for groups.Next() {
		var path, client, thread string
		var verified bool
		if err := groups.Scan(&path, &client, &thread, &verified); err != nil {
			return err
		}
		if _, err := dst.ExecContext(ctx, "INSERT INTO portable_session_groups VALUES(?,?,?,?)", path, client, thread, verified); err != nil {
			return err
		}
	}
	if err := groups.Err(); err != nil {
		return err
	}
	for _, row := range order.Entries {
		if _, err := dst.ExecContext(ctx, "INSERT INTO portable_session_order VALUES(?,?,?,?,?)",
			row.DocumentID, row.SourcePath, row.Client, row.SessionID, row.Ordinal); err != nil {
			return err
		}
	}
	return validatePortableSessionOrder(ctx, dst)
}

type portableSessionQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// A separate cause lets validation distinguish a malformed SQLite key from
// a missing reference, while returning the same invalid-archive diagnostic.
var errPortableOrderKeyType = fmt.Errorf("%w: session document order", ErrInvalidArchive)

// This validation also runs on decoded databases. Constraints on an archive's
// catalog are not proof that its rows obey those constraints or use binary keys.
func validatePortableSessionOrder(ctx context.Context, db portableSessionQueryer) error {
	var orderKeysAreText, valid bool
	err := db.QueryRowContext(ctx, `SELECT
		NOT EXISTS(SELECT 1 FROM portable_session_order
			WHERE typeof(source_path)<>'text' OR typeof(client)<>'text' OR typeof(session_id)<>'text'),
		(SELECT count(*) FROM portable_session_state)=1
		AND EXISTS(SELECT 1 FROM portable_session_state WHERE singleton=1
			AND typeof(doc_id_high_water)='integer' AND doc_id_high_water>=coalesce((SELECT max(doc_id) FROM portable_session_order),0))
		AND NOT EXISTS(SELECT 1 FROM portable_session_groups
			WHERE typeof(source_path)<>'text' OR typeof(client)<>'text' OR typeof(session_id)<>'text'
			OR typeof(order_verified)<>'integer' OR order_verified NOT IN (0,1))
		AND NOT EXISTS(SELECT source_path COLLATE BINARY,client COLLATE BINARY,session_id COLLATE BINARY FROM portable_session_order
			EXCEPT SELECT source_path COLLATE BINARY,client COLLATE BINARY,session_id COLLATE BINARY FROM portable_session_groups)
		AND NOT EXISTS(SELECT source_path COLLATE BINARY,client COLLATE BINARY,session_id COLLATE BINARY FROM portable_session_groups GROUP BY 1,2,3 HAVING count(*)<>1)
		AND NOT EXISTS(SELECT source_path COLLATE BINARY,client COLLATE BINARY,session_id COLLATE BINARY FROM portable_session_groups
			EXCEPT SELECT source_path COLLATE BINARY,client COLLATE BINARY,session_id COLLATE BINARY FROM session_metadata)
		AND NOT EXISTS(SELECT source_path COLLATE BINARY,client COLLATE BINARY,session_id COLLATE BINARY FROM session_metadata
			EXCEPT SELECT source_path COLLATE BINARY,client COLLATE BINARY,session_id COLLATE BINARY FROM portable_session_groups)
		AND NOT EXISTS(SELECT rowid,source_path COLLATE BINARY,client COLLATE BINARY,session_id COLLATE BINARY FROM session_documents
			EXCEPT SELECT doc_id,source_path COLLATE BINARY,client COLLATE BINARY,session_id COLLATE BINARY FROM portable_session_order)
		AND NOT EXISTS(SELECT doc_id,source_path COLLATE BINARY,client COLLATE BINARY,session_id COLLATE BINARY FROM portable_session_order
			EXCEPT SELECT rowid,source_path COLLATE BINARY,client COLLATE BINARY,session_id COLLATE BINARY FROM session_documents)
		AND NOT EXISTS(SELECT doc_id FROM portable_session_order GROUP BY doc_id
			HAVING typeof(doc_id)<>'integer' OR doc_id<=0 OR count(*)<>1)
		AND NOT EXISTS(SELECT 1 FROM portable_session_order WHERE typeof(detail_ordinal)<>'integer' OR detail_ordinal<=0)
		AND NOT EXISTS(SELECT source_path COLLATE BINARY,client COLLATE BINARY,session_id COLLATE BINARY FROM portable_session_order
			GROUP BY 1,2,3 HAVING max(detail_ordinal)<>count(*) OR count(DISTINCT detail_ordinal)<>count(*))`).Scan(&orderKeysAreText, &valid)
	if err == nil && !orderKeysAreText {
		return errPortableOrderKeyType
	}
	if err != nil || !valid {
		return fmt.Errorf("%w: session document order", ErrInvalidArchive)
	}
	return nil
}
