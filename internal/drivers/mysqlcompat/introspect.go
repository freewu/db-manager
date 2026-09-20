package mysqlcompat

import (
	"context"
	"database/sql"
	"strings"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

// Introspector reads the MySQL catalog: information_schema plus SHOW CREATE.
//
// The zero value reads a MySQL server. An engine that speaks the same protocol
// but keeps its catalog elsewhere embeds this struct and overrides the methods
// that differ — Doris, for instance, answers SHOW DATABASES / SHOW FULL COLUMNS
// where MySQL answers information_schema queries.
type Introspector struct {
	// SystemSchemas are the databases hidden from the explorer. Nil means the
	// schemas MySQL ships with.
	SystemSchemas map[string]bool
}

var _ sqlbase.Introspector = Introspector{}

const objectSelect = `
SELECT TABLE_NAME, TABLE_TYPE, IFNULL(TABLE_COMMENT, ''), IFNULL(TABLE_ROWS, 0),
       IFNULL(DATA_LENGTH, 0) + IFNULL(INDEX_LENGTH, 0), IFNULL(ENGINE, '')
FROM information_schema.TABLES`

func (Introspector) Version(ctx context.Context, q sqlbase.Querier) (string, error) {
	var v string
	if err := q.QueryRowContext(ctx, "SELECT VERSION()").Scan(&v); err != nil {
		return "", apperr.Wrap(apperr.CodeQueryFailed, err, "read server version")
	}
	return v, nil
}

func (Introspector) CurrentDatabase(ctx context.Context, q sqlbase.Querier) (string, error) {
	var name sql.NullString
	if err := q.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&name); err != nil {
		return "", apperr.Wrap(apperr.CodeQueryFailed, err, "read current database")
	}
	return name.String, nil
}

func (i Introspector) Databases(ctx context.Context, q sqlbase.Querier) ([]string, error) {
	rows, err := q.QueryContext(ctx,
		"SELECT SCHEMA_NAME FROM information_schema.SCHEMATA ORDER BY SCHEMA_NAME")
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list databases")
	}
	defer rows.Close()

	all := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		all = append(all, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return i.FilterDatabases(all), nil
}

// Schemas is empty: these engines have no schema layer below the database.
func (Introspector) Schemas(context.Context, sqlbase.Querier, string) ([]string, error) {
	return []string{}, nil
}

func (i Introspector) Object(ctx context.Context, q sqlbase.Querier, database, schema, object string) (*models.ObjectInfo, error) {
	rows, err := q.QueryContext(ctx, objectSelect+" WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?", database, object)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "describe object")
	}
	defer rows.Close()
	objs, err := ScanObjects(rows, database, schema)
	if err != nil {
		return nil, err
	}
	if len(objs) == 0 {
		return nil, apperr.New(apperr.CodeNotFound, "object %s.%s does not exist", database, object)
	}
	return &objs[0], nil
}

func (i Introspector) Objects(ctx context.Context, q sqlbase.Querier, database, schema string) ([]models.ObjectInfo, error) {
	rows, err := q.QueryContext(ctx, objectSelect+" WHERE TABLE_SCHEMA = ? ORDER BY TABLE_NAME", database)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list objects")
	}
	defer rows.Close()
	return ScanObjects(rows, database, schema)
}

// ScanObjects reads the rows of objectSelect. It is exported because an engine
// whose listing comes from somewhere else (SHOW FULL TABLES) still wants the
// same ObjectInfo shape out of it.
func ScanObjects(rows *sql.Rows, database, schema string) ([]models.ObjectInfo, error) {
	out := []models.ObjectInfo{}
	for rows.Next() {
		var (
			name, tableType, comment, engine string
			rowEstimate, size                sql.NullInt64
		)
		if err := rows.Scan(&name, &tableType, &comment, &rowEstimate, &size, &engine); err != nil {
			return nil, err
		}
		out = append(out, models.ObjectInfo{
			Name:        name,
			Schema:      schema,
			Database:    database,
			Kind:        KindFromTableType(tableType),
			Comment:     comment,
			RowEstimate: rowEstimate.Int64,
			SizeBytes:   size.Int64,
			Engine:      engine,
		})
	}
	return out, rows.Err()
}

// KindFromTableType maps a catalog's table type onto the UI's object kinds.
func KindFromTableType(t string) models.ObjectKind {
	switch strings.ToUpper(t) {
	case "VIEW":
		return models.KindView
	default:
		return models.KindTable
	}
}

func (Introspector) Columns(ctx context.Context, q sqlbase.Querier, database, schema, object string) ([]models.ColumnInfo, error) {
	const query = `
SELECT COLUMN_NAME, ORDINAL_POSITION, DATA_TYPE, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT,
       COLUMN_KEY, EXTRA, IFNULL(COLUMN_COMMENT, ''),
       CHARACTER_MAXIMUM_LENGTH, NUMERIC_PRECISION, NUMERIC_SCALE
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
ORDER BY ORDINAL_POSITION`

	rows, err := q.QueryContext(ctx, query, database, object)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list columns")
	}
	defer rows.Close()

	out := []models.ColumnInfo{}
	for rows.Next() {
		var (
			c                     models.ColumnInfo
			isNullable, columnKey string
			def                   sql.NullString
			extra, comment        string
			charLen, prec, scale  sql.NullInt64
		)
		if err := rows.Scan(&c.Name, &c.Ordinal, &c.DataType, &c.ColumnType, &isNullable, &def,
			&columnKey, &extra, &comment, &charLen, &prec, &scale); err != nil {
			return nil, err
		}
		c.Nullable = strings.EqualFold(isNullable, "YES")
		c.PrimaryKey = strings.EqualFold(columnKey, "PRI")
		c.AutoIncrement = strings.Contains(strings.ToLower(extra), "auto_increment")
		c.Comment = comment
		if def.Valid {
			v := def.String
			c.DefaultValue = &v
		}
		c.CharMaxLength = NullableInt(charLen)
		c.NumericPrecision = NullableInt(prec)
		c.NumericScale = NullableInt(scale)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (Introspector) Indexes(ctx context.Context, q sqlbase.Querier, database, schema, object string) ([]models.IndexInfo, error) {
	const query = `
SELECT INDEX_NAME, NON_UNIQUE, SEQ_IN_INDEX, COLUMN_NAME, IFNULL(INDEX_TYPE, ''), IFNULL(INDEX_COMMENT, '')
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
ORDER BY INDEX_NAME, SEQ_IN_INDEX`

	rows, err := q.QueryContext(ctx, query, database, object)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list indexes")
	}
	defer rows.Close()

	byName := map[string]*models.IndexInfo{}
	order := []string{}
	for rows.Next() {
		var (
			name, indexType, comment string
			nonUnique                int
			seq                      int
			column                   sql.NullString
		)
		if err := rows.Scan(&name, &nonUnique, &seq, &column, &indexType, &comment); err != nil {
			return nil, err
		}
		idx, ok := byName[name]
		if !ok {
			idx = &models.IndexInfo{
				Name:    name,
				Unique:  nonUnique == 0,
				Primary: strings.EqualFold(name, "PRIMARY"),
				Method:  indexType,
				Comment: comment,
				Columns: []string{},
			}
			byName[name] = idx
			order = append(order, name)
		}
		idx.Columns = append(idx.Columns, indexColumn(column))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]models.IndexInfo, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	return out, nil
}

func (Introspector) NamespaceIndexes(ctx context.Context, q sqlbase.Querier, database, schema string) ([]models.IndexEntry, error) {
	const query = `
SELECT TABLE_NAME, INDEX_NAME, NON_UNIQUE, SEQ_IN_INDEX, COLUMN_NAME, IFNULL(INDEX_TYPE, '')
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = ?
ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX`

	rows, err := q.QueryContext(ctx, query, database)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list indexes")
	}
	defer rows.Close()

	type key struct{ table, name string }
	byKey := map[key]*models.IndexEntry{}
	order := []key{}
	for rows.Next() {
		var (
			table, name, indexType string
			nonUnique, seq         int
			column                 sql.NullString
		)
		if err := rows.Scan(&table, &name, &nonUnique, &seq, &column, &indexType); err != nil {
			return nil, err
		}
		k := key{table: table, name: name}
		entry, ok := byKey[k]
		if !ok {
			entry = &models.IndexEntry{
				Name:     name,
				Table:    table,
				Database: database,
				Schema:   schema,
				Unique:   nonUnique == 0,
				Primary:  strings.EqualFold(name, "PRIMARY"),
				Method:   indexType,
				Columns:  []string{},
			}
			byKey[k] = entry
			order = append(order, k)
		}
		entry.Columns = append(entry.Columns, indexColumn(column))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]models.IndexEntry, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out, nil
}

func (Introspector) ForeignKeys(ctx context.Context, q sqlbase.Querier, database, schema, object string) ([]models.ForeignKeyInfo, error) {
	const query = `
SELECT k.CONSTRAINT_NAME, k.COLUMN_NAME, k.REFERENCED_TABLE_SCHEMA, k.REFERENCED_TABLE_NAME,
       k.REFERENCED_COLUMN_NAME, r.UPDATE_RULE, r.DELETE_RULE
FROM information_schema.KEY_COLUMN_USAGE k
JOIN information_schema.REFERENTIAL_CONSTRAINTS r
  ON r.CONSTRAINT_SCHEMA = k.CONSTRAINT_SCHEMA AND r.CONSTRAINT_NAME = k.CONSTRAINT_NAME
WHERE k.TABLE_SCHEMA = ? AND k.TABLE_NAME = ? AND k.REFERENCED_TABLE_NAME IS NOT NULL
ORDER BY k.CONSTRAINT_NAME, k.ORDINAL_POSITION`

	rows, err := q.QueryContext(ctx, query, database, object)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list foreign keys")
	}
	defer rows.Close()

	byName := map[string]*models.ForeignKeyInfo{}
	order := []string{}
	for rows.Next() {
		var (
			name, column, refSchema, refTable, refColumn, onUpdate, onDelete string
		)
		if err := rows.Scan(&name, &column, &refSchema, &refTable, &refColumn, &onUpdate, &onDelete); err != nil {
			return nil, err
		}
		fk, ok := byName[name]
		if !ok {
			fk = &models.ForeignKeyInfo{
				Name:              name,
				ReferencedSchema:  refSchema,
				ReferencedTable:   refTable,
				OnUpdate:          onUpdate,
				OnDelete:          onDelete,
				Columns:           []string{},
				ReferencedColumns: []string{},
			}
			byName[name] = fk
			order = append(order, name)
		}
		fk.Columns = append(fk.Columns, column)
		fk.ReferencedColumns = append(fk.ReferencedColumns, refColumn)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]models.ForeignKeyInfo, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	return out, nil
}

// indexColumn names one column of an index. A functional index has no column
// name in the catalog, and dropping the part would misreport the index.
func indexColumn(column sql.NullString) string {
	if column.Valid && column.String != "" {
		return column.String
	}
	return "(expression)"
}

// NullableInt turns a NULLable integer column into a pointer, so "unknown"
// stays distinguishable from zero.
func NullableInt(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}
