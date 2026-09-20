package doris

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers/mysqlcompat"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

// introspector reads Doris' catalog. It embeds the MySQL one so that anything
// not overridden here keeps working, and overrides every method whose query
// Doris would not answer: information_schema is too thin there, while the SHOW
// forms are documented and stable.
type introspector struct {
	mysqlcompat.Introspector
}

var _ sqlbase.Introspector = introspector{}

// Databases lists the catalogs: Doris calls them databases and has no schema
// layer below them.
func (i introspector) Databases(ctx context.Context, q sqlbase.Querier) ([]string, error) {
	rows, err := q.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list databases")
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	all := []string{}
	for rows.Next() {
		// Doris appends its own columns to SHOW DATABASES on some versions;
		// the database name is always the first one.
		values, err := scanRowValues(rows, columns)
		if err != nil {
			return nil, err
		}
		if name := strings.TrimSpace(values[strings.ToLower(columns[0])].String); name != "" {
			all = append(all, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list databases")
	}
	return i.FilterDatabases(all), nil
}

// Objects lists tables and views with SHOW FULL TABLES, then enriches what it
// can from information_schema: row counts, sizes and comments live there, and
// the tree shows them, but losing them must not lose the listing.
func (i introspector) Objects(ctx context.Context, q sqlbase.Querier, database, schema string) ([]models.ObjectInfo, error) {
	rows, err := q.QueryContext(ctx, "SHOW FULL TABLES FROM "+sqlutil.QuoteBacktick(database))
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list objects")
	}
	defer rows.Close()

	out := []models.ObjectInfo{}
	for rows.Next() {
		name, kind, err := scanShowTables(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, models.ObjectInfo{
			Name:     name,
			Schema:   schema,
			Database: database,
			Kind:     mysqlcompat.KindFromTableType(kind),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list objects")
	}
	enrich(ctx, q, database, out)
	return out, nil
}

// Object reads one table, going through information_schema first so that a
// single object still carries its size and comment.
func (i introspector) Object(ctx context.Context, q sqlbase.Querier, database, schema, object string) (*models.ObjectInfo, error) {
	if info, err := i.objectFromCatalog(ctx, q, database, schema, object); err == nil && info != nil {
		return info, nil
	}

	rows, err := q.QueryContext(ctx,
		"SHOW FULL TABLES FROM "+sqlutil.QuoteBacktick(database)+" LIKE ?", object)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "describe object")
	}
	defer rows.Close()
	for rows.Next() {
		name, kind, err := scanShowTables(rows)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(name, object) {
			continue
		}
		return &models.ObjectInfo{
			Name:     name,
			Schema:   schema,
			Database: database,
			Kind:     mysqlcompat.KindFromTableType(kind),
		}, nil
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "describe object")
	}
	return nil, apperr.New(apperr.CodeNotFound, "object %s.%s does not exist", database, object)
}

// objectFromCatalog looks one table up in information_schema. A nil result with
// a nil error means the table is not there; an error means Doris could not
// answer, and the caller falls back to SHOW FULL TABLES.
func (introspector) objectFromCatalog(ctx context.Context, q sqlbase.Querier, database, schema, object string) (*models.ObjectInfo, error) {
	rows, err := q.QueryContext(ctx, objectSelect+" WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?", database, object)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	objects, err := mysqlcompat.ScanObjects(rows, database, schema)
	if err != nil {
		return nil, err
	}
	if len(objects) == 0 {
		return nil, nil
	}
	return &objects[0], nil
}

// Columns reads SHOW FULL COLUMNS, which is the only listing Doris documents
// for every table type (the information_schema copy misses keys and defaults on
// several versions).
func (introspector) Columns(ctx context.Context, q sqlbase.Querier, database, schema, object string) ([]models.ColumnInfo, error) {
	rows, err := q.QueryContext(ctx, "SHOW FULL COLUMNS FROM "+table(database, object))
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list columns")
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []models.ColumnInfo{}
	for rows.Next() {
		row, err := scanRowValues(rows, columns)
		if err != nil {
			return nil, err
		}
		out = append(out, columnFromShow(row, len(out)+1))
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list columns")
	}
	return out, nil
}

// Indexes reads SHOW INDEX FROM, which Doris spells exactly like MySQL.
func (introspector) Indexes(ctx context.Context, q sqlbase.Querier, database, schema, object string) ([]models.IndexInfo, error) {
	rows, err := q.QueryContext(ctx, "SHOW INDEX FROM "+table(database, object))
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list indexes")
	}
	defer rows.Close()
	indexes, err := mysqlcompat.IndexesFromShow(rows)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list indexes")
	}
	return indexes, nil
}

// NamespaceIndexes lists the indexes of a whole database for the index page.
// Doris has no information_schema.STATISTICS on every version, so this walks
// the tables instead — and reports nothing rather than failing the page when a
// table refuses to answer.
func (i introspector) NamespaceIndexes(ctx context.Context, q sqlbase.Querier, database, schema string) ([]models.IndexEntry, error) {
	objects, err := i.Objects(ctx, q, database, schema)
	if err != nil {
		return nil, err
	}
	out := []models.IndexEntry{}
	for _, object := range objects {
		if object.Kind != models.KindTable {
			continue
		}
		rows, err := q.QueryContext(ctx, "SHOW INDEX FROM "+table(database, object.Name))
		if err != nil {
			// Internal tables (statistics, materialized views) reject SHOW
			// INDEX; one of them must not empty the whole page.
			continue
		}
		indexes, err := mysqlcompat.IndexesFromShow(rows)
		rows.Close()
		if err != nil {
			continue
		}
		for _, index := range indexes {
			out = append(out, models.IndexEntry{
				Name:     index.Name,
				Table:    object.Name,
				Database: database,
				Schema:   schema,
				Columns:  index.Columns,
				Unique:   index.Unique,
				Primary:  index.Primary,
				Method:   index.Method,
			})
		}
	}
	return out, nil
}

// ForeignKeys is always empty: Doris has no foreign keys to report, and saying
// so is more useful than a query that cannot succeed.
func (introspector) ForeignKeys(context.Context, sqlbase.Querier, string, string, string) ([]models.ForeignKeyInfo, error) {
	return []models.ForeignKeyInfo{}, nil
}

// --- helpers ---------------------------------------------------------------

// objectSelect mirrors the MySQL listing, but only where Doris' copy carries
// the same columns: TABLE_COMMENT and ENGINE are optional there.
const objectSelect = `
SELECT TABLE_NAME, TABLE_TYPE, IFNULL(TABLE_COMMENT, ''), IFNULL(TABLE_ROWS, 0),
       IFNULL(DATA_LENGTH, 0) + IFNULL(INDEX_LENGTH, 0), IFNULL(ENGINE, '')
FROM information_schema.TABLES`

// table quotes a database-qualified table name for a SHOW statement, which
// cannot take placeholders.
func table(database, object string) string {
	return sqlutil.Qualify(sqlutil.QuoteBacktick, database, object)
}

// enrich fills in the columns the tree wants but SHOW FULL TABLES does not
// have. Doris versions disagree on which information_schema columns exist, so
// this is best-effort by design: whatever cannot be read stays zero.
func enrich(ctx context.Context, q sqlbase.Querier, database string, objects []models.ObjectInfo) {
	if len(objects) == 0 {
		return
	}
	rows, err := q.QueryContext(ctx, objectSelect+" WHERE TABLE_SCHEMA = ?", database)
	if err != nil {
		return
	}
	defer rows.Close()
	details, err := mysqlcompat.ScanObjects(rows, database, "")
	if err != nil {
		return
	}
	byName := make(map[string]models.ObjectInfo, len(details))
	for _, detail := range details {
		byName[strings.ToLower(detail.Name)] = detail
	}
	for i, object := range objects {
		detail, ok := byName[strings.ToLower(object.Name)]
		if !ok {
			continue
		}
		objects[i].Comment = detail.Comment
		objects[i].RowEstimate = detail.RowEstimate
		objects[i].SizeBytes = detail.SizeBytes
		objects[i].Engine = detail.Engine
	}
}

// scanShowTables reads one row of SHOW FULL TABLES: the table name, then its
// type ("BASE TABLE" or "VIEW"). The first column is named after the database,
// so it is read positionally.
func scanShowTables(rows *sql.Rows) (string, string, error) {
	var name, kind sql.NullString
	if err := rows.Scan(&name, &kind); err != nil {
		return "", "", apperr.Wrap(apperr.CodeQueryFailed, err, "read the table list")
	}
	return strings.TrimSpace(name.String), strings.TrimSpace(kind.String), nil
}

// scanRowValues reads a row into NullStrings, keyed by column name lower-cased.
func scanRowValues(rows *sql.Rows, columns []string) (map[string]sql.NullString, error) {
	cells := make([]sql.NullString, len(columns))
	targets := make([]any, len(columns))
	for i := range cells {
		targets[i] = &cells[i]
	}
	if err := rows.Scan(targets...); err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "read the row")
	}
	out := make(map[string]sql.NullString, len(columns))
	for i, name := range columns {
		out[strings.ToLower(name)] = cells[i]
	}
	return out, nil
}

// columnFromShow turns one SHOW FULL COLUMNS row into a ColumnInfo. Doris
// reports the primary key as Key="true" (MySQL says "PRI"), and packs the
// precision into the type text ("decimal(10,2)", "varchar(20)").
func columnFromShow(row map[string]sql.NullString, ordinal int) models.ColumnInfo {
	rawType := strings.TrimSpace(row["type"].String)
	column := models.ColumnInfo{
		Name:          strings.TrimSpace(row["field"].String),
		Ordinal:       ordinal,
		DataType:      baseType(rawType),
		ColumnType:    rawType,
		Nullable:      isYes(row["null"]),
		PrimaryKey:    isYes(row["key"]) || strings.EqualFold(strings.TrimSpace(row["key"].String), "PRI"),
		AutoIncrement: strings.Contains(strings.ToLower(row["extra"].String), "auto_increment"),
		Comment:       row["comment"].String,
	}
	if def, ok := row["default"]; ok && def.Valid {
		value := def.String
		column.DefaultValue = &value
	}
	column.CharMaxLength, column.NumericPrecision, column.NumericScale = typeSizes(rawType)
	return column
}

// baseType strips the size from a type name, the way information_schema's
// DATA_TYPE reports it ("varchar", "decimal").
func baseType(text string) string {
	if i := strings.IndexByte(text, '('); i >= 0 {
		text = text[:i]
	}
	return strings.ToLower(strings.TrimSpace(text))
}

// typeSizes reads "decimal(10,2)" / "varchar(20)" / "int" into the numeric
// fields the table pane shows.
func typeSizes(text string) (charLen, precision, scale *int64) {
	open := strings.IndexByte(text, '(')
	close := strings.IndexByte(text, ')')
	if open < 0 || close < open {
		return nil, nil, nil
	}
	parts := strings.Split(text[open+1:close], ",")
	if len(parts) == 0 {
		return nil, nil, nil
	}
	first, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil {
		return nil, nil, nil
	}
	if base := baseType(text); strings.Contains(base, "char") || strings.Contains(base, "text") || base == "string" {
		return &first, nil, nil
	}
	precision = &first
	if len(parts) > 1 {
		if second, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); err == nil {
			scale = &second
		}
	}
	return nil, precision, scale
}

// isYes reads the Yes/No and true/false spellings the SHOW statements use.
func isYes(value sql.NullString) bool {
	switch strings.ToLower(strings.TrimSpace(value.String)) {
	case "yes", "true", "1":
		return true
	}
	return false
}
