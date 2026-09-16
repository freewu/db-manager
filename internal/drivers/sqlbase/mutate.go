package sqlbase

import (
	"context"
	"strings"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// UpdateCell implements drivers.Conn.
//
// Every value is bound as a parameter: the only text spliced into the statement
// is an identifier, and that goes through the dialect's quoting.
func (c *Conn) UpdateCell(ctx context.Context, req models.CellUpdate) (int64, error) {
	column := strings.TrimSpace(req.Column)
	if column == "" {
		return 0, apperr.New(apperr.CodeInvalidConfig, "a column name is required")
	}
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

	args := []any{req.Value}
	where, err := keyPredicate(d, req.Key, &args)
	if err != nil {
		return 0, err
	}

	ctx, cancel := withTimeout(ctx, 0)
	defer cancel()

	query := "UPDATE " + target + " SET " + d.Quote(column) + " = " + d.Placeholder(1) + where
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

// keyPredicate renders "WHERE a = $1 AND b IS NULL", appending the bound
// values to args.
func keyPredicate(d drivers.Dialect, key []models.KeyValue, args *[]any) (string, error) {
	parts := make([]string, 0, len(key))
	for _, kv := range key {
		column := strings.TrimSpace(kv.Column)
		if column == "" {
			return "", apperr.New(apperr.CodeInvalidConfig, "a primary key column name is empty")
		}
		if kv.Value == nil {
			parts = append(parts, d.Quote(column)+" IS NULL")
			continue
		}
		*args = append(*args, kv.Value)
		parts = append(parts, d.Quote(column)+" = "+d.Placeholder(len(*args)))
	}
	if len(parts) == 0 {
		return "", apperr.New(apperr.CodeInvalidConfig, "the row has no primary key to identify it")
	}
	return " WHERE " + strings.Join(parts, " AND "), nil
}
