package mysqlcompat

import (
	"context"
	"database/sql"

	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/drivers/sqlutil"
)

// NativeDDL reads the engine's own CREATE statement for an object.
//
// SHOW CREATE TABLE is the authoritative definition — it carries the engine
// options, keys and storage clauses a reconstructed statement would lose — so
// it is preferred over rendering one from the catalog. Views need their own
// statement, which is why this tries both.
func NativeDDL(ctx context.Context, q sqlbase.Querier, database, schema, object string) (string, error) {
	target := sqlutil.Qualify(sqlutil.QuoteBacktick, database, object)

	rows, err := q.QueryContext(ctx, "SHOW CREATE TABLE "+target)
	if err != nil {
		rows, err = q.QueryContext(ctx, "SHOW CREATE VIEW "+target)
		if err != nil {
			return "", err
		}
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}
	if !rows.Next() {
		return "", sql.ErrNoRows
	}
	holders := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range holders {
		ptrs[i] = &holders[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return "", err
	}
	// The DDL is always the second column ("Create Table" / "Create View").
	if len(holders) >= 2 {
		if s, ok := holders[1].(string); ok {
			return s, nil
		}
		if b, ok := holders[1].([]byte); ok {
			return string(b), nil
		}
	}
	return "", nil
}
