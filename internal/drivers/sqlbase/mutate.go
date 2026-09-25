package sqlbase

import (
	"context"
	"database/sql"
	"strings"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// The two ways a row identity can be unusable. They are values rather than
// literals so that the preview and the run refuse a request with the same error.
var (
	errEmptyKeyColumn = apperr.New(apperr.CodeInvalidConfig, "a primary key column name is empty")
	errNoKey          = apperr.New(apperr.CodeInvalidConfig, "the row has no primary key to identify it")
)

// UpdateCell implements drivers.Conn.
//
// Every value is bound as a parameter: the only text spliced into the statement
// is an identifier, and that goes through the dialect's quoting.
func (c *Conn) UpdateCell(ctx context.Context, req models.CellUpdate) (int64, error) {
	if strings.TrimSpace(req.Column) == "" {
		return 0, apperr.New(apperr.CodeInvalidConfig, "a column name is required")
	}
	return c.UpdateRow(ctx, models.RowUpdate{
		SessionID: req.SessionID,
		Database:  req.Database,
		Schema:    req.Schema,
		Object:    req.Object,
		Key:       req.Key,
		Values:    []models.KeyValue{{Column: req.Column, Value: req.Value}},
	})
}

// UpdateRow implements drivers.Conn: one UPDATE covering every changed column.
func (c *Conn) UpdateRow(ctx context.Context, req models.RowUpdate) (int64, error) {
	if strings.TrimSpace(req.Object) == "" {
		return 0, apperr.New(apperr.CodeInvalidConfig, "an object name is required")
	}
	if len(req.Values) == 0 {
		return 0, apperr.New(apperr.CodeInvalidConfig, "no columns were changed")
	}

	assignments := make([]string, 0, len(req.Values))
	args := make([]any, 0, len(req.Values)+len(req.Key))
	d := c.spec.Dialect
	for _, value := range req.Values {
		column := strings.TrimSpace(value.Column)
		if column == "" {
			return 0, apperr.New(apperr.CodeInvalidConfig, "a column name is required")
		}
		args = append(args, value.Value)
		assignments = append(assignments, d.Quote(column)+" = "+d.Placeholder(len(args)))
	}

	db, err := c.DB(ctx, req.Database)
	if err != nil {
		return 0, err
	}

	target := d.Qualify(c.resolveDatabase(req.Database), req.Schema, req.Object)
	where, err := keyPredicate(d, req.Key, &args)
	if err != nil {
		return 0, err
	}

	ctx, cancel := withTimeout(ctx, 0)
	defer cancel()

	query := "UPDATE " + target + " SET " + strings.Join(assignments, ", ") + where
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, apperr.Wrap(apperr.CodeQueryFailed, err, "update row")
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return affected, nil
}

// PlanRowUpdate implements drivers.Conn: the statement UpdateRow would run, with
// its values spelled out, for a person to read before it is applied.
func (c *Conn) PlanRowUpdate(req models.RowUpdate) (string, error) {
	if strings.TrimSpace(req.Object) == "" {
		return "", apperr.New(apperr.CodeInvalidConfig, "an object name is required")
	}
	if len(req.Values) == 0 {
		return "", apperr.New(apperr.CodeInvalidConfig, "no columns were changed")
	}
	d := c.spec.Dialect
	assignments := make([]string, 0, len(req.Values))
	for _, value := range req.Values {
		column := strings.TrimSpace(value.Column)
		if column == "" {
			return "", apperr.New(apperr.CodeInvalidConfig, "a column name is required")
		}
		assignments = append(assignments, d.Quote(column)+" = "+sqlLiteral(d, value.Value))
	}
	where, err := literalWhere(d, req.Key)
	if err != nil {
		return "", err
	}
	target := d.Qualify(c.resolveDatabase(req.Database), req.Schema, req.Object)
	return "UPDATE " + target + " SET " + strings.Join(assignments, ", ") + where, nil
}

// PlanRowDelete implements drivers.Conn.
func (c *Conn) PlanRowDelete(req models.RowDelete) (string, error) {
	if strings.TrimSpace(req.Object) == "" {
		return "", apperr.New(apperr.CodeInvalidConfig, "an object name is required")
	}
	d := c.spec.Dialect
	where, err := literalWhere(d, req.Key)
	if err != nil {
		return "", err
	}
	target := d.Qualify(c.resolveDatabase(req.Database), req.Schema, req.Object)
	return "DELETE FROM " + target + where, nil
}

// DeleteRow implements drivers.Conn.
func (c *Conn) DeleteRow(ctx context.Context, req models.RowDelete) (int64, error) {
	if strings.TrimSpace(req.Object) == "" {
		return 0, apperr.New(apperr.CodeInvalidConfig, "an object name is required")
	}
	if len(req.Key) == 0 {
		return 0, apperr.New(apperr.CodeInvalidConfig, "the row has no primary key to identify it")
	}

	db, err := c.DB(ctx, req.Database)
	if err != nil {
		return 0, err
	}

	d := c.spec.Dialect
	target := d.Qualify(c.resolveDatabase(req.Database), req.Schema, req.Object)

	var args []any
	where, err := keyPredicate(d, req.Key, &args)
	if err != nil {
		return 0, err
	}

	ctx, cancel := withTimeout(ctx, 0)
	defer cancel()

	res, err := db.ExecContext(ctx, "DELETE FROM "+target+where, args...)
	if err != nil {
		return 0, apperr.Wrap(apperr.CodeQueryFailed, err, "delete row")
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return affected, nil
}

// InsertRows implements drivers.Inserter.
//
// The batch goes out as one multi-row INSERT — one round trip instead of one
// per row — and only the identifiers (table and columns) are spliced into it,
// through the dialect's quoting; every generated value is a bind parameter.
//
// If the server refuses that statement the batch is replayed one row at a time,
// which is the only way to find out *which* row it was: a multi-row INSERT is
// all-or-nothing per statement. What the replay then does with a refused row is
// the whole meaning of `SkipErrors` — off (the default) the batch stops there
// and reports the row, on it is counted as skipped and the next row is tried.
// The retry at worst repeats the rows the failed statement had already written
// on an engine whose INSERT is not atomic per statement (MySQL's
// non-transactional tables). A duplicate then surfaces as a row that failed,
// with the engine's own message — the count the window shows stays true, which
// is what matters to whoever is looking at a half-filled table.
func (c *Conn) InsertRows(ctx context.Context, req models.RowInsert) (models.RowInsertResult, error) {
	object := strings.TrimSpace(req.Object)
	if object == "" {
		return models.RowInsertResult{}, apperr.New(apperr.CodeInvalidConfig, "an object name is required")
	}
	columns, err := insertColumns(req.Columns)
	if err != nil {
		return models.RowInsertResult{}, err
	}
	if len(req.Rows) == 0 {
		return models.RowInsertResult{}, apperr.New(apperr.CodeInvalidConfig, "there is no row to insert")
	}
	for i, row := range req.Rows {
		if len(row) != len(columns) {
			return models.RowInsertResult{}, apperr.New(apperr.CodeInvalidConfig,
				"row %d has %d value(s) for %d column(s)", i+1, len(row), len(columns))
		}
	}

	db, err := c.DB(ctx, req.Database)
	if err != nil {
		return models.RowInsertResult{}, err
	}

	d := c.spec.Dialect
	target := d.Qualify(c.resolveDatabase(req.Database), req.Schema, object)

	ctx, cancel := withTimeout(ctx, 0)
	defer cancel()

	if inserted, err := execInsert(ctx, db, d, target, columns, req.Rows); err == nil {
		return models.RowInsertResult{Inserted: inserted}, nil
	}

	var landed int64
	var skipped int64
	var firstError string
	for i, row := range req.Rows {
		inserted, err := execInsert(ctx, db, d, target, columns, [][]any{row})
		if err != nil {
			if !req.SkipErrors {
				return models.RowInsertResult{Inserted: landed, Failed: i + 1, Error: err.Error()}, nil
			}
			// The row is left out and the run keeps its shape: the caller asked
			// for the count of what did not fit, not for the run to stop.
			skipped++
			if firstError == "" {
				firstError = err.Error()
			}
			continue
		}
		landed += inserted
	}
	return models.RowInsertResult{Inserted: landed, Skipped: skipped, Error: firstError}, nil
}

// PlanRowInsert implements drivers.Inserter: the statement InsertRows runs,
// with its values left as bind placeholders.
//
// Placeholders rather than literals, unlike the row previews: a batch is up to
// five hundred generated rows, and a preview — or a log line — carrying all of
// them is neither readable nor small. What is being shown is the statement's
// shape: which table, which columns, and that the values arrive as parameters,
// which is also the thing worth being sure about.
func (c *Conn) PlanRowInsert(req models.RowInsert) (string, error) {
	object := strings.TrimSpace(req.Object)
	if object == "" {
		return "", apperr.New(apperr.CodeInvalidConfig, "an object name is required")
	}
	columns, err := insertColumns(req.Columns)
	if err != nil {
		return "", err
	}
	d := c.spec.Dialect
	target := d.Qualify(c.resolveDatabase(req.Database), req.Schema, object)

	marks := make([]string, len(columns))
	for i := range columns {
		marks[i] = d.Placeholder(i + 1)
	}
	return insertHead(d, target, columns) + "(" + strings.Join(marks, ", ") + ")", nil
}

// insertColumns trims and validates the column list once, up front, so a bad
// request is refused before anything is sent to the server.
func insertColumns(columns []string) ([]string, error) {
	if len(columns) == 0 {
		return nil, apperr.New(apperr.CodeInvalidConfig, "there is no column to fill")
	}
	out := make([]string, 0, len(columns))
	seen := make(map[string]bool, len(columns))
	for _, column := range columns {
		name := strings.TrimSpace(column)
		if name == "" {
			return nil, apperr.New(apperr.CodeInvalidConfig, "a column name is empty")
		}
		if seen[name] {
			return nil, apperr.New(apperr.CodeInvalidConfig, "column %s is listed twice", name)
		}
		seen[name] = true
		out = append(out, name)
	}
	return out, nil
}

// insertHead renders everything before the values of an INSERT: the table, and
// the columns in the order the rows were built in. Only identifiers go in here,
// and they go through the dialect's quoting.
func insertHead(d drivers.Dialect, target string, columns []string) string {
	quoted := make([]string, len(columns))
	for i, column := range columns {
		quoted[i] = d.Quote(column)
	}
	return "INSERT INTO " + target + " (" + strings.Join(quoted, ", ") + ") VALUES "
}

// execInsert writes rows in one statement, in the order they were given.
func execInsert(ctx context.Context, db *sql.DB, d drivers.Dialect, target string, columns []string, rows [][]any) (int64, error) {
	args := make([]any, 0, len(rows)*len(columns))
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		marks := make([]string, len(columns))
		for i := range columns {
			args = append(args, row[i])
			marks[i] = d.Placeholder(len(args))
		}
		values = append(values, "("+strings.Join(marks, ", ")+")")
	}

	query := insertHead(d, target, columns) + strings.Join(values, ", ")
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, apperr.Wrap(apperr.CodeQueryFailed, err, "insert row")
	}
	affected, err := res.RowsAffected()
	if err != nil {
		// The statement went through; only the count is unavailable. A multi-row
		// INSERT is one statement, so every row in it is in.
		return int64(len(rows)), nil
	}
	return affected, nil
}

// keyPredicate renders "WHERE a = $1 AND b IS NULL", appending the bound
// values to args.
func keyPredicate(d drivers.Dialect, key []models.KeyValue, args *[]any) (string, error) {
	parts := make([]string, 0, len(key))
	for _, kv := range key {
		column := strings.TrimSpace(kv.Column)
		if column == "" {
			return "", errEmptyKeyColumn
		}
		if kv.Value == nil {
			parts = append(parts, d.Quote(column)+" IS NULL")
			continue
		}
		*args = append(*args, kv.Value)
		parts = append(parts, d.Quote(column)+" = "+d.Placeholder(len(*args)))
	}
	if len(parts) == 0 {
		return "", errNoKey
	}
	return " WHERE " + strings.Join(parts, " AND "), nil
}
