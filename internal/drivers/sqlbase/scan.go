package sqlbase

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"dbmanager/internal/apperr"
	"dbmanager/internal/models"
)

// scanQuery runs a query and materialises up to maxRows rows.
func scanQuery(ctx context.Context, q Querier, query string, args []any, maxRows int) (*models.QueryResult, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	names, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	columns := make([]models.ColumnMeta, len(names))
	for i, name := range names {
		meta := models.ColumnMeta{Name: name, Nullable: true}
		if i < len(colTypes) {
			meta.DatabaseType = colTypes[i].DatabaseTypeName()
			if nullable, ok := colTypes[i].Nullable(); ok {
				meta.Nullable = nullable
			}
		}
		columns[i] = meta
	}

	out := &models.QueryResult{
		Columns:      columns,
		Rows:         make([][]any, 0, 64),
		HasResultSet: true,
	}

	// Prepare scan targets once and reuse them across rows.
	holders := make([]any, len(names))
	ptrs := make([]any, len(names))
	for i := range holders {
		ptrs[i] = &holders[i]
	}

	for rows.Next() {
		if maxRows > 0 && len(out.Rows) >= maxRows {
			out.Truncated = true
			break
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		record := make([]any, len(names))
		for i := range holders {
			record[i] = coerceValue(holders[i])
			holders[i] = nil
		}
		out.Rows = append(out.Rows, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out.RowCount = len(out.Rows)
	return out, nil
}

// coerceValue turns a database/sql value into something that round-trips
// cleanly through JSON.
//
//   - []byte is decoded as UTF-8 text; genuinely binary blobs become a short
//     hex preview so the grid stays readable.
//   - time.Time is rendered as RFC3339 with nanoseconds.
func coerceValue(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		if utf8.Valid(t) {
			return string(t)
		}
		return "0x" + hex.EncodeToString(t)
	case time.Time:
		return t.Format(time.RFC3339Nano)
	default:
		return t
	}
}

// QuoteLiteral renders a value as a SQL literal. Used only for building
// human-facing DDL and preview statements - never for execution.
func QuoteLiteral(v any) string {
	if v == nil {
		return "NULL"
	}
	switch t := v.(type) {
	case string:
		return "'" + strings.ReplaceAll(t, "'", "''") + "'"
	case []byte:
		return "'" + strings.ReplaceAll(string(t), "'", "''") + "'"
	case bool:
		if t {
			return "TRUE"
		}
		return "FALSE"
	case time.Time:
		return "'" + t.Format(time.RFC3339Nano) + "'"
	default:
		return fmt.Sprint(t)
	}
}

// execAffected runs a statement and reports the affected row count, tolerating
// drivers that do not implement it.
func execAffected(ctx context.Context, q Querier, query string, args ...any) (int64, error) {
	res, err := q.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return n, nil
}

// requireDatabase guards catalog queries that need a database name.
func requireDatabase(database string) error {
	if strings.TrimSpace(database) == "" {
		return apperr.New(apperr.CodeInvalidConfig, "a database name is required")
	}
	return nil
}
