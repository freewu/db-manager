package sqlbase

import (
	"fmt"
	"strconv"
	"strings"

	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// This file renders the *preview* of a statement: the text the change log keeps
// and the row detail layer shows before anything is applied. Every statement the
// drivers actually run binds its values (see mutate.go); the statements here are
// only ever read by a person, which is why they may spell their values out.
//
// The renderer lives here rather than on drivers.Dialect because not every
// driver has SQL literals at all: MongoDB's dialect quotes identifiers for a
// different language. A driver that cannot spell a literal simply does not use
// this file.

// backslashEscaper is implemented by dialects where a backslash inside a string
// literal is an escape character (MySQL and its forks, but not PostgreSQL, where
// it is an ordinary character unless the string is prefixed with E'').
//
// It is an optional method rather than part of drivers.Dialect: it is the
// difference between two renderings of the same literal, not a capability every
// driver should have to answer.
type backslashEscaper interface {
	spellsBackslash() bool
}

// sqlLiteral renders one value the way the dialect would spell it in SQL.
//
// A value of a kind no engine sends back (a struct, a slice of anything else) is
// rendered as a quoted string of its Go form, which is still what a reader would
// need to reproduce the statement by hand.
func sqlLiteral(d drivers.Dialect, value any) string {
	switch typed := value.(type) {
	case nil:
		return "NULL"
	case bool:
		if typed {
			return "TRUE"
		}
		return "FALSE"
	case int:
		return strconv.Itoa(typed)
	case int8:
		return strconv.FormatInt(int64(typed), 10)
	case int16:
		return strconv.FormatInt(int64(typed), 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint8:
		return strconv.FormatUint(uint64(typed), 10)
	case uint16:
		return strconv.FormatUint(uint64(typed), 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case float32:
		return strconv.FormatFloat(float64(typed), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64)
	case string:
		return quoteString(d, typed)
	default:
		return quoteString(d, fmt.Sprintf("%v", typed))
	}
}

// quoteString wraps a text value in single quotes, with the quotes inside it
// doubled — the spelling every SQL engine reads the same way.
func quoteString(d drivers.Dialect, text string) string {
	if escaper, ok := d.(backslashEscaper); ok && escaper.spellsBackslash() {
		// In an escaped dialect a backslash leads the character after it, so an
		// ordinary backslash has to be spelled as two before the quotes are
		// dealt with.
		text = strings.ReplaceAll(text, `\`, `\\`)
	}
	return "'" + strings.ReplaceAll(text, "'", "''") + "'"
}

// literalWhere renders the row predicate as literals: " WHERE a = 1 AND b IS
// NULL". It is the reading version of keyPredicate.
func literalWhere(d drivers.Dialect, key []models.KeyValue) (string, error) {
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
		parts = append(parts, d.Quote(column)+" = "+sqlLiteral(d, kv.Value))
	}
	if len(parts) == 0 {
		return "", errNoKey
	}
	return " WHERE " + strings.Join(parts, " AND "), nil
}
