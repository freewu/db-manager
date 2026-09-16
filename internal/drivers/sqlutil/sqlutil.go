// Package sqlutil holds helpers shared by every SQL backed driver.
package sqlutil

import (
	"fmt"
	"strings"
	"unicode"

	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// Filter operators understood by the UI and by BuildWhere.
const (
	OpEq          = "eq"
	OpNe          = "ne"
	OpGt          = "gt"
	OpGte         = "gte"
	OpLt          = "lt"
	OpLte         = "lte"
	OpContains    = "contains"
	OpNotContains = "notContains"
	OpStartsWith  = "startsWith"
	OpEndsWith    = "endsWith"
	OpIsNull      = "isNull"
	OpIsNotNull   = "isNotNull"
	OpIn          = "in"
	OpNotIn       = "notIn"
	OpBetween     = "between"
)

// Operators returns the operator catalog for the UI, grouped by the column
// family they make sense for.
func Operators() []string {
	return []string{
		OpEq, OpNe, OpContains, OpNotContains, OpStartsWith, OpEndsWith,
		OpGt, OpGte, OpLt, OpLte, OpBetween, OpIn, OpNotIn, OpIsNull, OpIsNotNull,
	}
}

// QuoteDouble quotes an identifier with `"` (PostgreSQL, SQLite, Oracle).
func QuoteDouble(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}

// QuoteBacktick quotes an identifier with backticks (MySQL, MariaDB).
func QuoteBacktick(ident string) string {
	return "`" + strings.ReplaceAll(ident, "`", "``") + "`"
}

// QuoteBracket quotes an identifier with `[]` (SQL Server).
func QuoteBracket(ident string) string {
	return "[" + strings.ReplaceAll(ident, "]", "]]") + "]"
}

// Qualify joins non-empty, pre-quoted parts with a dot.
func Qualify(quote func(string) string, parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		out = append(out, quote(p))
	}
	return strings.Join(out, ".")
}

// BuildWhere compiles filters into a WHERE clause (including the leading
// keyword) plus its arguments, using the dialect's placeholder syntax starting
// at argIndex.
//
// It deliberately returns an error instead of silently dropping unknown
// filters: a data grid that shows unfiltered rows because a filter failed to
// compile is worse than an error message.
func BuildWhere(d drivers.Dialect, filters []models.FilterSpec, argIndex int) (string, []any, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}

	clauses := make([]string, 0, len(filters))
	args := make([]any, 0, len(filters))

	next := func() string {
		ph := d.Placeholder(argIndex)
		argIndex++
		return ph
	}

	for i, f := range filters {
		if strings.TrimSpace(f.Column) == "" {
			return "", nil, fmt.Errorf("filter %d: missing column", i+1)
		}
		col := d.Quote(f.Column)
		op := f.Operator
		if op == "" {
			op = OpEq
		}

		switch op {
		case OpEq:
			clauses = append(clauses, col+" = "+next())
			args = append(args, f.Value)
		case OpNe:
			clauses = append(clauses, col+" <> "+next())
			args = append(args, f.Value)
		case OpGt:
			clauses = append(clauses, col+" > "+next())
			args = append(args, f.Value)
		case OpGte:
			clauses = append(clauses, col+" >= "+next())
			args = append(args, f.Value)
		case OpLt:
			clauses = append(clauses, col+" < "+next())
			args = append(args, f.Value)
		case OpLte:
			clauses = append(clauses, col+" <= "+next())
			args = append(args, f.Value)
		case OpContains:
			clauses = append(clauses, col+" LIKE "+next())
			args = append(args, "%"+escapeLike(f.Value)+"%")
		case OpNotContains:
			clauses = append(clauses, col+" NOT LIKE "+next())
			args = append(args, "%"+escapeLike(f.Value)+"%")
		case OpStartsWith:
			clauses = append(clauses, col+" LIKE "+next())
			args = append(args, escapeLike(f.Value)+"%")
		case OpEndsWith:
			clauses = append(clauses, col+" LIKE "+next())
			args = append(args, "%"+escapeLike(f.Value))
		case OpIsNull:
			clauses = append(clauses, col+" IS NULL")
		case OpIsNotNull:
			clauses = append(clauses, col+" IS NOT NULL")
		case OpIn, OpNotIn:
			values, err := splitList(f.Value)
			if err != nil {
				return "", nil, err
			}
			ph := make([]string, 0, len(values))
			for _, v := range values {
				ph = append(ph, next())
				args = append(args, v)
			}
			kw := " IN ("
			if op == OpNotIn {
				kw = " NOT IN ("
			}
			clauses = append(clauses, col+kw+strings.Join(ph, ", ")+")")
		case OpBetween:
			lo, hi := next(), next()
			clauses = append(clauses, col+" BETWEEN "+lo+" AND "+hi)
			args = append(args, f.Value, f.Value2)
		default:
			return "", nil, fmt.Errorf("filter %d: unsupported operator %q", i+1, op)
		}
	}

	return " WHERE " + strings.Join(clauses, " AND "), args, nil
}

// BuildOrderBy compiles sort specs into an ORDER BY clause.
func BuildOrderBy(d drivers.Dialect, specs []models.SortSpec) string {
	if len(specs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(specs))
	for _, s := range specs {
		if strings.TrimSpace(s.Column) == "" {
			continue
		}
		dir := " ASC"
		if s.Desc {
			dir = " DESC"
		}
		parts = append(parts, d.Quote(s.Column)+dir)
	}
	if len(parts) == 0 {
		return ""
	}
	return " ORDER BY " + strings.Join(parts, ", ")
}

// escapeLike neutralises LIKE wildcards in user supplied values. The backslash
// escape character is understood by MySQL, PostgreSQL and SQLite alike (MySQL
// by default, PostgreSQL since it treats \ as the default escape).
func escapeLike(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, "%", `\%`)
	v = strings.ReplaceAll(v, "_", `\_`)
	return v
}

func splitList(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("IN filter requires at least one value")
	}
	return out, nil
}

// --- statement handling ----------------------------------------------------

// queryLeaders are the leading keywords that produce a result set.
var queryLeaders = map[string]bool{
	"select":   true,
	"show":     true,
	"describe": true,
	"desc":     true,
	"explain":  true,
	"with":     true,
	"values":   true,
	"table":    true,
	"pragma":   true,
	"call":     true,
}

// IsQueryStatement reports whether the statement is expected to return rows.
//
// This is a heuristic on purpose: database/sql cannot tell us in advance, and
// running an INSERT through Query() loses the affected-row count.
func IsQueryStatement(sql string) bool {
	stripped := stripLeadingComments(sql)
	if stripped == "" {
		return false
	}
	word := leadingWord(stripped)
	return queryLeaders[strings.ToLower(word)]
}

func stripLeadingComments(sql string) string {
	s := strings.TrimSpace(sql)
	for {
		switch {
		case strings.HasPrefix(s, "--"):
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = strings.TrimSpace(s[i+1:])
				continue
			}
			return ""
		case strings.HasPrefix(s, "/*"):
			if i := strings.Index(s, "*/"); i >= 0 {
				s = strings.TrimSpace(s[i+2:])
				continue
			}
			return ""
		default:
			return s
		}
	}
}

func leadingWord(s string) string {
	for i, r := range s {
		if !unicode.IsLetter(r) && r != '_' {
			return s[:i]
		}
	}
	return s
}

// SplitStatements splits a script into individual statements on semicolons
// while respecting string literals, quoted identifiers and comments.
//
// Trailing semicolons and whitespace-only fragments are dropped. A script with
// no trailing semicolon still yields its final statement.
func SplitStatements(script string) []string {
	var (
		out     []string
		start   int
		i       int
		n       = len(script)
		inLine  bool
		inBlock bool
	)

	flush := func(end int) {
		stmt := strings.TrimSpace(script[start:end])
		if stmt != "" && !isOnlyComments(stmt) {
			out = append(out, stmt)
		}
		start = end + 1
	}

	for i < n {
		c := script[i]
		switch {
		case inLine:
			if c == '\n' {
				inLine = false
			}
			i++
		case inBlock:
			if c == '*' && i+1 < n && script[i+1] == '/' {
				inBlock = false
				i += 2
				continue
			}
			i++
		case c == '-' && i+1 < n && script[i+1] == '-':
			inLine = true
			i += 2
		case c == '/' && i+1 < n && script[i+1] == '*':
			inBlock = true
			i += 2
		case c == '\'' || c == '"' || c == '`':
			quote := c
			i++
			for i < n {
				if script[i] == '\\' && quote != '`' && i+1 < n {
					i += 2
					continue
				}
				// Doubled quote is an escaped quote.
				if script[i] == quote {
					if i+1 < n && script[i+1] == quote {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		case c == ';':
			flush(i)
			i++
		default:
			i++
		}
	}
	// Tail without trailing semicolon.
	if tail := strings.TrimSpace(script[start:]); tail != "" && !isOnlyComments(tail) {
		out = append(out, tail)
	}
	return out
}

func isOnlyComments(s string) bool {
	return stripLeadingComments(s) == ""
}
