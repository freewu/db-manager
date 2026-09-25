package sqlbase

import (
	"context"
	"strings"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

// StreamRows implements drivers.RowStreamer for every engine built on sqlbase.
//
// This is Fetch's sibling that does not page: one statement, read to the end,
// every row handed to the caller and dropped. The two share their shape (same
// qualification, same primary-key ordering, same value decoding) but not their
// limits — Fetch is capped at MaxPageSize rows because it answers a window,
// while a stream answers a file.
func (c *Conn) StreamRows(ctx context.Context, req drivers.StreamRequest, each func(row []any) error) error {
	db, err := c.DB(ctx, req.Database)
	if err != nil {
		return err
	}
	resolved := c.resolveDatabase(req.Database)
	d := c.spec.Dialect

	selectList := "*"
	if len(req.Columns) > 0 {
		quoted := make([]string, 0, len(req.Columns))
		for _, name := range req.Columns {
			name = strings.TrimSpace(name)
			if name == "" {
				return apperr.New(apperr.CodeInvalidConfig, "column name is required")
			}
			quoted = append(quoted, d.Quote(name))
		}
		selectList = strings.Join(quoted, ", ")
	}

	// Order by the primary key when there is one, for the same reason Fetch
	// does — a table has no order of its own — and because a dump that is read
	// twice should come out the same both times. A table without a key is read
	// in whatever order the engine likes, which is all that can be promised.
	orderBy := ""
	if pk := c.primaryKey(ctx, db, resolved, req.Schema, req.Object); len(pk) > 0 {
		specs := make([]models.SortSpec, 0, len(pk))
		for _, col := range pk {
			specs = append(specs, models.SortSpec{Column: col})
		}
		orderBy = sqlutil.BuildOrderBy(d, specs)
	}

	query := "SELECT " + selectList + " FROM " + d.Qualify(resolved, req.Schema, req.Object) + orderBy

	// No withTimeout here: the caller's context is the deadline, and an export
	// is exactly the case where the default 60s query budget is the wrong
	// answer. The caller knows whether to keep waiting, and it has a Stop
	// button for when it does not.
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return apperr.Wrap(apperr.CodeQueryFailed, err, "read rows of %s", objectLabel(req.Schema, req.Object))
	}
	defer rows.Close()

	names, err := rows.Columns()
	if err != nil {
		return apperr.Wrap(apperr.CodeQueryFailed, err, "read rows of %s", objectLabel(req.Schema, req.Object))
	}

	holders := make([]any, len(names))
	ptrs := make([]any, len(names))
	for i := range holders {
		ptrs[i] = &holders[i]
	}

	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return apperr.Wrap(apperr.CodeQueryFailed, err, "read rows of %s", objectLabel(req.Schema, req.Object))
		}
		record := make([]any, len(names))
		for i := range holders {
			record[i] = coerceValue(holders[i])
			holders[i] = nil
		}
		if err := each(record); err != nil {
			// The caller's own decision travels back untouched: whether this
			// was "stop, the user asked" or a broken pipe to a file, it is not
			// the query that failed.
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return apperr.Wrap(apperr.CodeQueryFailed, err, "read rows of %s", objectLabel(req.Schema, req.Object))
	}
	return nil
}

// objectLabel spells an object for an error message: qualified when there is a
// schema to qualify it with, bare otherwise.
func objectLabel(schema, object string) string {
	if schema == "" {
		return object
	}
	return schema + "." + object
}
