package store

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type cjsiQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

var cjsiQuota36ForbiddenLegacyTables = []string{
	"quota_windows",
	"quota_envelopes",
	"quota_alert_notices",
}

var cjsiQuota36CoreTables = []string{
	"quota_windows_v2",
	"quota_envelopes_v2",
	"quota_alert_notices_v2",
	"quota_cycle",
	"quota_projection_revision",
	"quota_identities",
}

var cjsiQuota36CoreSingletons = []string{
	"schema_metadata",
	"derived_snapshot_generation",
	"serving_generation",
	"quota_cycle",
	"quota_projection_revision",
}

type cjsiSchemaSnapshot struct {
	tables             map[string]cjsiTableSchema
	objects            []cjsiSchemaObjectDef
	requiredSingletons []string
}

type cjsiSchemaObjectDef struct {
	kind   string
	name   string
	table  string
	rawSQL string
}

type cjsiTableListMeta struct {
	tableType string
	ncol      int
	wr        int
	strict    int
}

type cjsiColumnSchema struct {
	cid          int
	name         string
	colType      string
	notNull      int
	defaultValue sql.NullString
	primaryKey   int
}

type cjsiIndexColumnSchema struct {
	seqno int
	cid   int
	name  sql.NullString
	desc  int
	coll  string
	key   int
}

type cjsiIndexSchema struct {
	name       string
	auto       bool
	unique     int
	origin     string
	partial    int
	definition []string
	columns    []cjsiIndexColumnSchema
}

type cjsiForeignKeyItem struct {
	seq      int
	table    string
	from     string
	to       sql.NullString
	onUpdate string
	onDelete string
	match    string
}

type cjsiForeignKeySchema struct {
	items []cjsiForeignKeyItem
}

type cjsiTableSchema struct {
	object      cjsiSchemaObjectDef
	meta        cjsiTableListMeta
	columns     []cjsiColumnSchema
	indexes     []cjsiIndexSchema
	foreignKeys []cjsiForeignKeySchema
}

type cjsiSchemaReferences struct {
	quota36, layout37 *cjsiSchemaSnapshot
}

var cjsiCanonicalSchemaReferences = sync.OnceValues(func() (*cjsiSchemaReferences, error) {
	if len(migrations) < cjsiBaseCoreSchemaVersion {
		return nil, fmt.Errorf("%w: incomplete built-in migration registry", ErrUnknownSchema)
	}
	registry := make([]migration, cjsiBaseCoreSchemaVersion)
	copy(registry, migrations[:cjsiBaseCoreSchemaVersion-1])
	registry[cjsiBaseCoreSchemaVersion-1] = migration{version: cjsiBaseCoreSchemaVersion, apply: migrateQuotaIdentity}
	ctx := context.Background()
	quota36, err := cjsiSnapshotFromRegistry(ctx, registry)
	if err != nil {
		return nil, err
	}
	if err := verifyCJSICanonicalQuota36Baseline(quota36); err != nil {
		return nil, err
	}
	step37 := cjsiCoreMigration37(func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, cjsiLayoutJournalSchema)
		return err
	})
	layout37, err := cjsiSnapshotFromRegistry(ctx, append(registry, step37))
	if err != nil {
		return nil, err
	}
	return &cjsiSchemaReferences{quota36: quota36, layout37: layout37}, nil
})

// cjsiFoldSQLiteIdent applies SQLite's ASCII-only case folding (matching
// sqlite3StrICmp and COLLATE NOCASE) to identifiers, leaving non-ASCII bytes
// unchanged.
func cjsiFoldSQLiteIdent(s string) string {
	hasUpper := false
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			hasUpper = true
			break
		}
	}
	if !hasUpper {
		return s
	}
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func cjsiFoldNullIdent(ns sql.NullString) sql.NullString {
	if !ns.Valid {
		return ns
	}
	return sql.NullString{String: cjsiFoldSQLiteIdent(ns.String), Valid: true}
}

func cjsiClassifySchemaQueryErr(ctx context.Context, err error, detail string) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	var driverErr *sqlite.Error
	if errors.As(err, &driverErr) {
		switch driverErr.Code() & 0xff {
		case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
			return err
		}
	}
	return fmt.Errorf("%w: %s: %v", ErrUnknownSchema, detail, err)
}

func cjsiQuoteSQLIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// Validate the candidate registry against the canonical quota migration before
// using the in-memory reference for either side of the layout migration.
func validatedCJSISchemaReferences(ctx context.Context, currentVersion int, registry []migration) (*cjsiSchemaReferences, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if currentVersion != cjsiBaseCoreSchemaVersion || len(registry) != cjsiBaseCoreSchemaVersion {
		return nil, fmt.Errorf("%w: unsupported schema ceiling %d (registry length %d)", ErrUnknownSchema, currentVersion, len(registry))
	}
	for i, m := range registry {
		if m.version != i+1 {
			return nil, fmt.Errorf("%w: non-contiguous migration %d at index %d", ErrUnknownSchema, m.version, i)
		}
	}
	v36 := registry[cjsiBaseCoreSchemaVersion-1]
	if v36.apply == nil || len(v36.statements) != 0 {
		return nil, fmt.Errorf("%w: invalid schema %d migration entry", ErrUnknownSchema, cjsiBaseCoreSchemaVersion)
	}

	refs, err := cjsiCanonicalSchemaReferences()
	if err != nil {
		return nil, err
	}

	candidateRef, err := cjsiSnapshotFromRegistry(ctx, registry)
	if err != nil {
		return nil, err
	}
	if err := compareCJSISchemaStructure(candidateRef, refs.quota36); err != nil {
		return nil, err
	}
	if !slices.Equal(candidateRef.requiredSingletons, refs.quota36.requiredSingletons) {
		return nil, fmt.Errorf("%w: registry schema singletons %v, want %v", ErrUnknownSchema, candidateRef.requiredSingletons, refs.quota36.requiredSingletons)
	}
	return refs, nil
}

func cjsiSnapshotFromRegistry(ctx context.Context, registry []migration) (*cjsiSchemaSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return nil, cjsiClassifySchemaQueryErr(ctx, err, "configure reference sqlite")
	}
	if err := bootstrapAndMigrate(ctx, db, registry); err != nil {
		return nil, cjsiClassifySchemaQueryErr(ctx, err, "reference schema migration failed")
	}
	version, err := schemaVersion(ctx, db)
	if err != nil {
		return nil, cjsiClassifySchemaQueryErr(ctx, err, "read reference schema version")
	}
	if version != len(registry) {
		return nil, fmt.Errorf("%w: reference schema version %d, want %d", ErrUnknownSchema, version, len(registry))
	}
	return captureCJSISchemaSnapshot(ctx, db, true)
}

func captureCJSISchemaSnapshot(ctx context.Context, q cjsiQueryer, inspectSingletons bool) (*cjsiSchemaSnapshot, error) {
	// 1. Read all sqlite_master objects in one pass.
	type masterRow struct {
		kind   string
		name   string
		table  string
		rawSQL string
	}
	masterRows, err := func() ([]masterRow, error) {
		rows, err := q.QueryContext(ctx, `SELECT type, name, tbl_name, coalesce(sql, '') FROM main.sqlite_master
			ORDER BY type COLLATE NOCASE, name COLLATE NOCASE`)
		if err != nil {
			return nil, cjsiClassifySchemaQueryErr(ctx, err, "read sqlite_master")
		}
		defer rows.Close()
		var out []masterRow
		for rows.Next() {
			var r masterRow
			if err := rows.Scan(&r.kind, &r.name, &r.table, &r.rawSQL); err != nil {
				return nil, cjsiClassifySchemaQueryErr(ctx, err, "scan sqlite_master")
			}
			name := cjsiFoldSQLiteIdent(r.name)
			if strings.HasPrefix(name, "sqlite_") {
				switch {
				case r.kind == "table" && (name == "sqlite_sequence" || name == "sqlite_stat1" || name == "sqlite_stat2" || name == "sqlite_stat3" || name == "sqlite_stat4"):
					continue
				case r.kind == "index" && strings.HasPrefix(name, "sqlite_autoindex_") && r.rawSQL == "":
					continue
				default:
					return nil, fmt.Errorf("%w: unexpected %s %q", ErrUnknownSchema, r.kind, r.name)
				}
			}
			out = append(out, r)
		}
		if err := rows.Err(); err != nil {
			return nil, cjsiClassifySchemaQueryErr(ctx, err, "iterate sqlite_master")
		}
		return out, nil
	}()
	if err != nil {
		return nil, err
	}

	snap := &cjsiSchemaSnapshot{tables: make(map[string]cjsiTableSchema)}
	// Map folded table name -> (folded index name -> raw index SQL).
	indexSQLByTable := make(map[string]map[string]string)
	// Map folded table name -> original stored table name for singleton SELECTs.
	storedTableName := make(map[string]string)

	for _, r := range masterRows {
		kind := cjsiFoldSQLiteIdent(r.kind)
		foldedName := cjsiFoldSQLiteIdent(r.name)
		foldedTable := cjsiFoldSQLiteIdent(r.table)
		obj := cjsiSchemaObjectDef{kind: kind, name: foldedName, table: foldedTable, rawSQL: r.rawSQL}
		snap.objects = append(snap.objects, obj)
		switch kind {
		case "table":
			snap.tables[foldedName] = cjsiTableSchema{object: obj}
			storedTableName[foldedName] = r.name
		case "index":
			if indexSQLByTable[foldedTable] == nil {
				indexSQLByTable[foldedTable] = make(map[string]string)
			}
			indexSQLByTable[foldedTable][foldedName] = r.rawSQL
		}
	}

	// 2. Read pragma_table_list for all main tables in one query.
	if err := func() error {
		rows, err := q.QueryContext(ctx, `SELECT name, type, ncol, wr, strict FROM pragma_table_list WHERE schema = 'main'`)
		if err != nil {
			return cjsiClassifySchemaQueryErr(ctx, err, "read pragma_table_list")
		}
		defer rows.Close()
		for rows.Next() {
			var name, tableType string
			var ncol, wr, strict int
			if err := rows.Scan(&name, &tableType, &ncol, &wr, &strict); err != nil {
				return cjsiClassifySchemaQueryErr(ctx, err, "scan pragma_table_list")
			}
			key := cjsiFoldSQLiteIdent(name)
			tbl, ok := snap.tables[key]
			if !ok {
				continue
			}
			tbl.meta = cjsiTableListMeta{
				tableType: cjsiFoldSQLiteIdent(tableType),
				ncol:      ncol,
				wr:        wr,
				strict:    strict,
			}
			snap.tables[key] = tbl
		}
		if err := rows.Err(); err != nil {
			return cjsiClassifySchemaQueryErr(ctx, err, "iterate pragma_table_list")
		}
		return nil
	}(); err != nil {
		return nil, err
	}

	// 3. Read pragma_table_info for all user tables in one bulk lateral join.
	if err := func() error {
		rows, err := q.QueryContext(ctx, `SELECT m.name, ti.cid, ti.name, ti.type, ti."notnull", ti.dflt_value, ti.pk
			FROM main.sqlite_master m
			JOIN pragma_table_info(m.name, 'main') ti
			WHERE m.type = 'table' AND substr(m.name, 1, 7) COLLATE NOCASE <> 'sqlite_'
			ORDER BY m.name COLLATE NOCASE, ti.cid`)
		if err != nil {
			return cjsiClassifySchemaQueryErr(ctx, err, "read bulk pragma_table_info")
		}
		defer rows.Close()
		for rows.Next() {
			var tableName string
			var col cjsiColumnSchema
			if err := rows.Scan(&tableName, &col.cid, &col.name, &col.colType, &col.notNull, &col.defaultValue, &col.primaryKey); err != nil {
				return cjsiClassifySchemaQueryErr(ctx, err, "scan bulk pragma_table_info")
			}
			key := cjsiFoldSQLiteIdent(tableName)
			tbl, ok := snap.tables[key]
			if !ok {
				continue
			}
			col.name = cjsiFoldSQLiteIdent(col.name)
			col.colType = cjsiFoldSQLiteIdent(col.colType)
			tbl.columns = append(tbl.columns, col)
			snap.tables[key] = tbl
		}
		if err := rows.Err(); err != nil {
			return cjsiClassifySchemaQueryErr(ctx, err, "iterate bulk pragma_table_info")
		}
		return nil
	}(); err != nil {
		return nil, err
	}

	// 4 & 5. Read pragma_index_list and pragma_index_xinfo for all user tables in two bulk queries.
	type rawIndexKey struct {
		table string
		index string
	}
	type rawIndexMeta struct {
		table   string
		name    string
		unique  int
		origin  string
		partial int
	}
	rawIndexesByTable := make(map[string][]rawIndexMeta)
	if err := func() error {
		rows, err := q.QueryContext(ctx, `SELECT m.name, il.name, il."unique", il.origin, il.partial
			FROM main.sqlite_master m
			JOIN pragma_index_list(m.name, 'main') il
			WHERE m.type = 'table' AND substr(m.name, 1, 7) COLLATE NOCASE <> 'sqlite_'
			ORDER BY m.name COLLATE NOCASE, il.seq`)
		if err != nil {
			return cjsiClassifySchemaQueryErr(ctx, err, "read bulk pragma_index_list")
		}
		defer rows.Close()
		for rows.Next() {
			var rm rawIndexMeta
			if err := rows.Scan(&rm.table, &rm.name, &rm.unique, &rm.origin, &rm.partial); err != nil {
				return cjsiClassifySchemaQueryErr(ctx, err, "scan bulk pragma_index_list")
			}
			tableKey := cjsiFoldSQLiteIdent(rm.table)
			if _, ok := snap.tables[tableKey]; !ok {
				continue
			}
			rm.table = tableKey
			rm.name = cjsiFoldSQLiteIdent(rm.name)
			rm.origin = cjsiFoldSQLiteIdent(rm.origin)
			rawIndexesByTable[tableKey] = append(rawIndexesByTable[tableKey], rm)
		}
		if err := rows.Err(); err != nil {
			return cjsiClassifySchemaQueryErr(ctx, err, "iterate bulk pragma_index_list")
		}
		return nil
	}(); err != nil {
		return nil, err
	}

	indexColsByKey := make(map[rawIndexKey][]cjsiIndexColumnSchema)
	if err := func() error {
		rows, err := q.QueryContext(ctx, `SELECT m.name, il.name, ix.seqno, ix.cid, ix.name, ix."desc", ix.coll, ix."key"
			FROM main.sqlite_master m
			JOIN pragma_index_list(m.name, 'main') il
			JOIN pragma_index_xinfo(il.name, 'main') ix
			WHERE m.type = 'table' AND substr(m.name, 1, 7) COLLATE NOCASE <> 'sqlite_'
			ORDER BY m.name COLLATE NOCASE, il.seq, ix.seqno`)
		if err != nil {
			return cjsiClassifySchemaQueryErr(ctx, err, "read bulk pragma_index_xinfo")
		}
		defer rows.Close()
		for rows.Next() {
			var tableName, indexName string
			var col cjsiIndexColumnSchema
			if err := rows.Scan(&tableName, &indexName, &col.seqno, &col.cid, &col.name, &col.desc, &col.coll, &col.key); err != nil {
				return cjsiClassifySchemaQueryErr(ctx, err, "scan bulk pragma_index_xinfo")
			}
			k := rawIndexKey{
				table: cjsiFoldSQLiteIdent(tableName),
				index: cjsiFoldSQLiteIdent(indexName),
			}
			col.name = cjsiFoldNullIdent(col.name)
			col.coll = cjsiFoldSQLiteIdent(col.coll)
			indexColsByKey[k] = append(indexColsByKey[k], col)
		}
		if err := rows.Err(); err != nil {
			return cjsiClassifySchemaQueryErr(ctx, err, "iterate bulk pragma_index_xinfo")
		}
		return nil
	}(); err != nil {
		return nil, err
	}

	for tableKey, rawList := range rawIndexesByTable {
		tbl := snap.tables[tableKey]
		sqlMap := indexSQLByTable[tableKey]
		indexes := make([]cjsiIndexSchema, 0, len(rawList))
		for _, raw := range rawList {
			cols := indexColsByKey[rawIndexKey{table: tableKey, index: raw.name}]
			rawSQL := sqlMap[raw.name]
			auto := strings.HasPrefix(raw.name, "sqlite_autoindex_") || (raw.origin != "c" && rawSQL == "")
			idx := cjsiIndexSchema{
				auto:    auto,
				unique:  raw.unique,
				origin:  raw.origin,
				partial: raw.partial,
				columns: cols,
			}
			if !auto {
				idx.name = raw.name
				// SQLite exposes ordinary keys through index_xinfo. Only an
				// expression or WHERE clause needs additional DDL comparison.
				hasExpression := cjsiIndexHasExpression(cols)
				if raw.partial != 0 || hasExpression {
					idx.definition = cjsiIndexDefinition(rawSQL, raw.partial != 0 && !hasExpression, tbl.columns)
				}
			}
			indexes = append(indexes, idx)
		}
		slices.SortFunc(indexes, compareCJSIIndexSchema)
		tbl.indexes = indexes
		snap.tables[tableKey] = tbl
	}

	// 6. Read pragma_foreign_key_list for all user tables in one bulk query.
	type fkGroupKey struct {
		table string
		id    int
	}
	fkByGroup := make(map[fkGroupKey][]cjsiForeignKeyItem)
	fkIDsByTable := make(map[string][]int)
	if err := func() error {
		rows, err := q.QueryContext(ctx, `SELECT m.name, fk.id, fk.seq, fk."table", fk."from", fk."to", fk.on_update, fk.on_delete, fk."match"
			FROM main.sqlite_master m
			JOIN pragma_foreign_key_list(m.name, 'main') fk
			WHERE m.type = 'table' AND substr(m.name, 1, 7) COLLATE NOCASE <> 'sqlite_'
			ORDER BY m.name COLLATE NOCASE, fk.id, fk.seq`)
		if err != nil {
			return cjsiClassifySchemaQueryErr(ctx, err, "read bulk pragma_foreign_key_list")
		}
		defer rows.Close()
		for rows.Next() {
			var tableName string
			var id int
			var item cjsiForeignKeyItem
			if err := rows.Scan(&tableName, &id, &item.seq, &item.table, &item.from, &item.to, &item.onUpdate, &item.onDelete, &item.match); err != nil {
				return cjsiClassifySchemaQueryErr(ctx, err, "scan bulk pragma_foreign_key_list")
			}
			tableKey := cjsiFoldSQLiteIdent(tableName)
			if _, ok := snap.tables[tableKey]; !ok {
				continue
			}
			item.table = cjsiFoldSQLiteIdent(item.table)
			item.from = cjsiFoldSQLiteIdent(item.from)
			item.to = cjsiFoldNullIdent(item.to)
			item.onUpdate = cjsiFoldSQLiteIdent(item.onUpdate)
			item.onDelete = cjsiFoldSQLiteIdent(item.onDelete)
			item.match = cjsiFoldSQLiteIdent(item.match)
			gk := fkGroupKey{table: tableKey, id: id}
			if _, exists := fkByGroup[gk]; !exists {
				fkIDsByTable[tableKey] = append(fkIDsByTable[tableKey], id)
			}
			fkByGroup[gk] = append(fkByGroup[gk], item)
		}
		if err := rows.Err(); err != nil {
			return cjsiClassifySchemaQueryErr(ctx, err, "iterate bulk pragma_foreign_key_list")
		}
		return nil
	}(); err != nil {
		return nil, err
	}

	for tableKey, ids := range fkIDsByTable {
		tbl := snap.tables[tableKey]
		fks := make([]cjsiForeignKeySchema, 0, len(ids))
		for _, id := range ids {
			fks = append(fks, cjsiForeignKeySchema{items: fkByGroup[fkGroupKey{table: tableKey, id: id}]})
		}
		slices.SortFunc(fks, compareCJSIForeignKeySchema)
		tbl.foreignKeys = fks
		snap.tables[tableKey] = tbl
	}

	if inspectSingletons {
		singletonTables := make([]string, 0, 8)
		for _, key := range cjsiSortedKeys(snap.tables) {
			if cjsiTableHasColumn(snap.tables[key], "singleton") {
				singletonTables = append(singletonTables, key)
			}
		}
		if len(singletonTables) > 0 {
			exprs := make([]string, len(singletonTables))
			counts := make([]int, len(singletonTables))
			dests := make([]any, len(singletonTables))
			for i, key := range singletonTables {
				exprs[i] = fmt.Sprintf("(SELECT count(*) FROM main.%s WHERE singleton = 1)", cjsiQuoteSQLIdent(storedTableName[key]))
				dests[i] = &counts[i]
			}
			if err := q.QueryRowContext(ctx, "SELECT "+strings.Join(exprs, ",")).Scan(dests...); err != nil {
				return nil, cjsiClassifySchemaQueryErr(ctx, err, "inspect reference singletons")
			}
			for i, key := range singletonTables {
				if counts[i] == 1 {
					snap.requiredSingletons = append(snap.requiredSingletons, key)
				}
			}
		}
		slices.Sort(snap.requiredSingletons)
	}

	return snap, nil
}

func verifyCJSICanonicalQuota36Baseline(ref *cjsiSchemaSnapshot) error {
	for _, legacy := range cjsiQuota36ForbiddenLegacyTables {
		if _, exists := ref.tables[cjsiFoldSQLiteIdent(legacy)]; exists {
			return fmt.Errorf("%w: reference schema retains legacy table %s", ErrUnknownSchema, legacy)
		}
	}
	for _, table := range cjsiQuota36CoreTables {
		if _, exists := ref.tables[cjsiFoldSQLiteIdent(table)]; !exists {
			return fmt.Errorf("%w: reference schema missing quota table %s", ErrUnknownSchema, table)
		}
	}
	for _, table := range cjsiQuota36CoreSingletons {
		if !slices.Contains(ref.requiredSingletons, cjsiFoldSQLiteIdent(table)) {
			return fmt.Errorf("%w: reference schema missing required singleton %s", ErrUnknownSchema, table)
		}
	}
	return nil
}

// Both catalogs are ordered by (type, ASCII-folded name). Check every object,
// including views and standalone indexes, before comparing table internals.
func compareCJSISchemaStructure(actual, ref *cjsiSchemaSnapshot) error {
	for i, want := range ref.objects {
		if i >= len(actual.objects) {
			return fmt.Errorf("%w: missing %s %q", ErrUnknownSchema, want.kind, want.name)
		}
		got := actual.objects[i]
		if got.kind != want.kind || got.name != want.name || got.table != want.table {
			return fmt.Errorf("%w: found %s %q on %q, expected %s %q on %q", ErrUnknownSchema,
				got.kind, got.name, got.table, want.kind, want.name, want.table)
		}
		switch want.kind {
		case "table":
			if err := compareCJSITableSchema(want.name, actual.tables[got.name], ref.tables[want.name]); err != nil {
				return err
			}
		case "trigger", "view":
			if !equalCJSISchemaObjectDef(got, want) {
				return fmt.Errorf("%w: %s %s definition mismatch", ErrUnknownSchema, want.kind, want.name)
			}
		}
	}
	if len(actual.objects) > len(ref.objects) {
		extra := actual.objects[len(ref.objects)]
		return fmt.Errorf("%w: unexpected %s %q", ErrUnknownSchema, extra.kind, extra.name)
	}
	return nil
}

func verifyCJSICoreSchema(ctx context.Context, q cjsiQueryer, ref *cjsiSchemaSnapshot) error {
	actual, err := captureCJSISchemaSnapshot(ctx, q, false)
	if err != nil {
		return err
	}
	if err := compareCJSISchemaStructure(actual, ref); err != nil {
		return err
	}

	// Check the required singleton rows and serving version in one query.
	exprs := make([]string, len(ref.requiredSingletons)+1)
	counts := make([]int, len(ref.requiredSingletons))
	dests := make([]any, len(ref.requiredSingletons)+1)
	for i, table := range ref.requiredSingletons {
		exprs[i] = fmt.Sprintf("(SELECT count(*) FROM main.%s WHERE singleton = 1)", cjsiQuoteSQLIdent(table))
		dests[i] = &counts[i]
	}
	var servingVersion sql.NullInt64
	exprs[len(ref.requiredSingletons)] = "(SELECT schema_version FROM main.serving_generation WHERE singleton = 1)"
	dests[len(ref.requiredSingletons)] = &servingVersion
	if err := q.QueryRowContext(ctx, "SELECT "+strings.Join(exprs, ",")).Scan(dests...); err != nil {
		return cjsiClassifySchemaQueryErr(ctx, err, "check required singleton rows")
	}
	for i, table := range ref.requiredSingletons {
		if counts[i] != 1 {
			return fmt.Errorf("%w: singleton table %s has %d rows with singleton=1, want 1", ErrUnknownSchema, table, counts[i])
		}
	}
	if !servingVersion.Valid || int(servingVersion.Int64) != ServingStorageVersion {
		return fmt.Errorf("%w: serving_generation schema_version %v, want %d", ErrUnknownSchema, servingVersion, ServingStorageVersion)
	}
	return nil
}

func readCJSITableSchema(ctx context.Context, q cjsiQueryer, table string) (cjsiTableSchema, error) {
	snap, err := captureCJSISchemaSnapshot(ctx, q, false)
	if err != nil {
		return cjsiTableSchema{}, err
	}
	tbl, ok := snap.tables[cjsiFoldSQLiteIdent(table)]
	if !ok {
		return cjsiTableSchema{}, fmt.Errorf("%w: missing table %s", ErrUnknownSchema, table)
	}
	return tbl, nil
}

func cjsiIndexHasExpression(cols []cjsiIndexColumnSchema) bool {
	for _, col := range cols {
		if col.cid == -2 || (col.key != 0 && col.cid < 0) {
			return true
		}
	}
	return false
}

func compareCJSITableSchema(name string, actual, ref cjsiTableSchema) error {
	if actual.meta != ref.meta {
		return fmt.Errorf("%w: table %s metadata mismatch", ErrUnknownSchema, name)
	}
	if !slices.Equal(actual.columns, ref.columns) {
		return fmt.Errorf("%w: table %s column/primary-key definition mismatch", ErrUnknownSchema, name)
	}
	if !equalCJSIIndexSlices(actual.indexes, ref.indexes) {
		return fmt.Errorf("%w: table %s index/unique constraint mismatch", ErrUnknownSchema, name)
	}
	if !equalCJSIForeignKeySlices(actual.foreignKeys, ref.foreignKeys) {
		return fmt.Errorf("%w: table %s foreign key mismatch", ErrUnknownSchema, name)
	}
	if !equalCJSISchemaObjectDef(actual.object, ref.object) {
		return fmt.Errorf("%w: table %s DDL mismatch", ErrUnknownSchema, name)
	}
	return nil
}

func cjsiTableHasColumn(tbl cjsiTableSchema, column string) bool {
	want := cjsiFoldSQLiteIdent(column)
	for _, col := range tbl.columns {
		if col.name == want {
			return true
		}
	}
	return false
}

// cjsiIndexDefinition supplements the index name, owning table, uniqueness,
// partial flag and index_xinfo already supplied by SQLite. The first unquoted
// '(' starts the key list in a parsed CREATE INDEX. An unfamiliar spelling
// or lexical construct must match the complete stored SQL byte for byte.
func cjsiIndexDefinition(ddl string, partialWithColumnKeys bool, columns []cjsiColumnSchema) []string {
	tokens := cjsiTokenizeSQLiteSQL(ddl)
	open := slices.Index(tokens, "(")
	if open < 0 {
		return []string{"SQL", ddl}
	}
	body := tokens[open:]
	if !partialWithColumnKeys {
		return body
	}
	// Every key in this partial index resolves to a real column. Its
	// ordering and collation are already compared. Keep the WHERE expression.
	where := slices.IndexFunc(body, func(s string) bool { return cjsiFoldSQLiteIdent(s) == "where" })
	if where < 1 || body[where-1] != ")" || where+1 >= len(body) {
		return []string{"SQL", ddl}
	}
	predicate := body[where+1:]
	if key := cjsiColumnNullTest(predicate, columns); key != nil {
		return key
	}
	return body[where:]
}

// Recognize just the null test used by the v36 partial indexes. The operand
// must resolve to a column of this table. SQLite's bare literal keywords may
// mean something different even when a like-named column exists.
func cjsiColumnNullTest(predicate []string, columns []cjsiColumnSchema) []string {
	var test string
	switch {
	case len(predicate) == 3:
		test = "IS NULL"
	case len(predicate) == 4 && cjsiFoldSQLiteIdent(predicate[2]) == "not":
		test = "IS NOT NULL"
	default:
		return nil
	}
	if cjsiFoldSQLiteIdent(predicate[1]) != "is" || cjsiFoldSQLiteIdent(predicate[len(predicate)-1]) != "null" {
		return nil
	}
	operand := predicate[0]
	switch operand[0] {
	case '\'':
		return nil
	case '"', '`':
		quote := operand[:1]
		operand = strings.ReplaceAll(operand[1:len(operand)-1], quote+quote, quote)
	case '[':
		operand = operand[1 : len(operand)-1]
	}
	name := cjsiFoldSQLiteIdent(operand)
	switch name {
	case "null", "true", "false", "current_date", "current_time", "current_timestamp":
		return nil
	}
	for _, col := range columns {
		if col.name == name {
			return []string{test, name}
		}
	}
	return nil
}

// Split the subset of index DDL for which we can identify token boundaries.
// Tokens retain their original bytes, including quotes and escapes. Numeric
// literals, blobs and other unsupported syntax take the exact-SQL path.
func cjsiTokenizeSQLiteSQL(s string) []string {
	var tokens []string
	for i := 0; i < len(s); {
		start := i
		c := s[i]
		i++
		switch {
		case strings.IndexByte(" \t\n\r\f", c) >= 0:
			continue
		case c == '-' && i < len(s) && s[i] == '-':
			if end := strings.IndexByte(s[i:], '\n'); end >= 0 {
				i += end + 1
			} else {
				i = len(s)
			}
			continue
		case c == '/' && i < len(s) && s[i] == '*':
			end := strings.Index(s[i+1:], "*/")
			if end < 0 {
				return nil
			}
			i += end + 3
			continue
		case c == '\'' || c == '"' || c == '`' || c == '[':
			closeQuote := c
			if c == '[' {
				closeQuote = ']'
			}
			for {
				end := strings.IndexByte(s[i:], closeQuote)
				if end < 0 {
					return nil
				}
				i += end + 1
				if c == '[' || i == len(s) || s[i] != closeQuote {
					break
				}
				i++ // Doubled delimiter within the quoted token.
			}
		case cjsiSQLWordStart(c):
			for i < len(s) && (cjsiSQLWordStart(s[i]) || (s[i] >= '0' && s[i] <= '9')) {
				i++
			}
			if i == start+1 && (c == 'x' || c == 'X') && i < len(s) && s[i] == '\'' {
				return nil // X'…' is one blob token. X '…' is not.
			}
		case c == '=':
			if i < len(s) && s[i] == '=' {
				i++
			}
		case c == '(' || c == ')' || c == ',':
		default:
			return nil
		}
		tokens = append(tokens, s[start:i])
	}
	return tokens
}

func cjsiSQLWordStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
}

func equalCJSISchemaObjectDef(a, b cjsiSchemaObjectDef) bool {
	// CHECKs and trigger bodies may contain arbitrary SQLite expressions.
	// Only the exact DDL from the historical migrations establishes identity.
	return a == b
}

func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case !a:
		return -1
	default:
		return 1
	}
}

func compareNullString(a, b sql.NullString) int {
	if c := compareBool(a.Valid, b.Valid); c != 0 {
		return c
	}
	return cmp.Compare(a.String, b.String)
}

func compareCJSIIndexColumn(a, b cjsiIndexColumnSchema) int {
	if c := cmp.Compare(a.seqno, b.seqno); c != 0 {
		return c
	}
	if c := cmp.Compare(a.cid, b.cid); c != 0 {
		return c
	}
	if c := compareNullString(a.name, b.name); c != 0 {
		return c
	}
	if c := cmp.Compare(a.desc, b.desc); c != 0 {
		return c
	}
	if c := cmp.Compare(a.coll, b.coll); c != 0 {
		return c
	}
	return cmp.Compare(a.key, b.key)
}

func compareCJSIIndexSchema(a, b cjsiIndexSchema) int {
	if c := compareBool(a.auto, b.auto); c != 0 {
		return c
	}
	if c := cmp.Compare(a.name, b.name); c != 0 {
		return c
	}
	if c := cmp.Compare(a.origin, b.origin); c != 0 {
		return c
	}
	if c := cmp.Compare(a.unique, b.unique); c != 0 {
		return c
	}
	if c := cmp.Compare(a.partial, b.partial); c != 0 {
		return c
	}
	if c := slices.Compare(a.definition, b.definition); c != 0 {
		return c
	}
	return slices.CompareFunc(a.columns, b.columns, compareCJSIIndexColumn)
}

func equalCJSIIndexSlices(a, b []cjsiIndexSchema) bool {
	return slices.EqualFunc(a, b, func(x, y cjsiIndexSchema) bool {
		return compareCJSIIndexSchema(x, y) == 0
	})
}

func compareCJSIForeignKeyItem(a, b cjsiForeignKeyItem) int {
	if c := cmp.Compare(a.seq, b.seq); c != 0 {
		return c
	}
	if c := cmp.Compare(a.table, b.table); c != 0 {
		return c
	}
	if c := cmp.Compare(a.from, b.from); c != 0 {
		return c
	}
	if c := compareNullString(a.to, b.to); c != 0 {
		return c
	}
	if c := cmp.Compare(a.onUpdate, b.onUpdate); c != 0 {
		return c
	}
	if c := cmp.Compare(a.onDelete, b.onDelete); c != 0 {
		return c
	}
	return cmp.Compare(a.match, b.match)
}

func compareCJSIForeignKeySchema(a, b cjsiForeignKeySchema) int {
	return slices.CompareFunc(a.items, b.items, compareCJSIForeignKeyItem)
}

func equalCJSIForeignKeySlices(a, b []cjsiForeignKeySchema) bool {
	return slices.EqualFunc(a, b, func(x, y cjsiForeignKeySchema) bool {
		return compareCJSIForeignKeySchema(x, y) == 0
	})
}

func cjsiSortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
