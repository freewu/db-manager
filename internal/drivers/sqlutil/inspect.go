package sqlutil

import (
	"strconv"
	"strings"

	"dbmanager/internal/models"
)

// Statement kinds returned by Analyze.
const (
	KindQuery   = "query"
	KindDDL     = "ddl"
	KindDML     = "dml"
	KindUnknown = "unknown"
)

// ddlLeaders are the leading keywords that change the schema.
var ddlLeaders = map[string]bool{
	"create":   true,
	"alter":    true,
	"drop":     true,
	"truncate": true,
	"rename":   true,
	"comment":  true,
	"grant":    true,
	"revoke":   true,
	"reindex":  true,
	"vacuum":   true,
	"analyze":  true,
	"attach":   true,
	"detach":   true,
}

// dmlLeaders are the leading keywords that change data.
var dmlLeaders = map[string]bool{
	"insert":  true,
	"update":  true,
	"delete":  true,
	"merge":   true,
	"replace": true,
	"copy":    true,
	"load":    true,
	"upsert":  true,
}

// Analyze inspects a script without running it and reports what each statement
// would do.
//
// The DDL editor uses this to show a dry run before anything is executed: which
// statements are DDL, which mutate data, and which of them would be refused on
// a read-only connection. It is a heuristic (no engine is contacted), so it
// errs towards warning: a false positive costs a glance, a false negative costs
// a dropped table.
func Analyze(sql string, readOnly bool) models.ScriptAnalysis {
	analysis := models.ScriptAnalysis{
		Statements: []models.ScriptStatement{},
		Warnings:   []string{},
		ReadOnly:   readOnly,
	}

	for i, statement := range SplitStatements(sql) {
		cleaned := stripComments(statement)
		word := strings.ToLower(leadingWord(cleaned))

		entry := models.ScriptStatement{
			Index:   i,
			Preview: preview(statement),
			Kind:    KindUnknown,
		}

		switch {
		case word == "":
			// A fragment that is only comments: SplitStatements drops those,
			// so this cannot normally happen.
			entry.Kind = KindUnknown
		case queryLeaders[word]:
			entry.Kind = KindQuery
		case ddlLeaders[word]:
			entry.Kind = KindDDL
		case dmlLeaders[word]:
			entry.Kind = KindDML
		}

		entry.Destructive, entry.Reason = destructive(cleaned)
		if entry.Destructive {
			analysis.Destructive = true
		}

		if entry.Kind == KindUnknown && word != "" {
			analysis.Warnings = append(analysis.Warnings,
				"statement "+strconv.Itoa(i+1)+": unrecognised statement, it will be sent to the server as-is")
		}
		if entry.Destructive {
			analysis.Warnings = append(analysis.Warnings,
				"statement "+strconv.Itoa(i+1)+": "+entry.Reason)
		}

		if readOnly && entry.Kind != KindQuery {
			analysis.Refused++
		}
		analysis.Statements = append(analysis.Statements, entry)
	}

	if readOnly && analysis.Refused > 0 {
		analysis.Warnings = append([]string{
			"this connection is read-only: " + strconv.Itoa(analysis.Refused) + " write statement(s) will be refused",
		}, analysis.Warnings...)
	}
	return analysis
}

// destructive reports whether a statement can lose schema or data, plus the
// sentence explaining why.
func destructive(cleaned string) (bool, string) {
	if cleaned == "" {
		return false, ""
	}
	lower := strings.ToLower(cleaned)
	flat := " " + strings.Join(strings.Fields(lower), " ") + " "

	switch strings.ToLower(leadingWord(lower)) {
	case "drop":
		if strings.HasPrefix(flat, " drop database") || strings.HasPrefix(flat, " drop schema") {
			return true, "DROP DATABASE/SCHEMA removes the whole namespace"
		}
		return true, "DROP removes an object and everything in it"
	case "truncate":
		return true, "TRUNCATE empties the table"
	case "delete":
		if !strings.Contains(flat, " where ") {
			return true, "DELETE without WHERE removes every row"
		}
	case "update":
		if !strings.Contains(flat, " where ") {
			return true, "UPDATE without WHERE rewrites every row"
		}
	case "alter":
		if strings.Contains(flat, " drop ") {
			return true, "ALTER … DROP removes a column, constraint or index"
		}
	}
	return false, ""
}

// preview is the one-line summary the analysis panel shows.
func preview(statement string) string {
	const limit = 140
	line := stripComments(statement)
	line = strings.Join(strings.Fields(line), " ")
	if len(line) > limit {
		return line[:limit] + "…"
	}
	return line
}

// stripComments removes -- and /* */ comments, honouring string literals and
// quoted identifiers so a comment marker inside 'text' is untouched.
func stripComments(s string) string {
	var out strings.Builder
	i, n := 0, len(s)
	for i < n {
		c := s[i]
		switch {
		case c == '-' && i+1 < n && s[i+1] == '-':
			for i < n && s[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && s[i+1] == '*':
			i += 2
			for i < n && !(s[i] == '*' && i+1 < n && s[i+1] == '/') {
				i++
			}
			i += 2
		case c == '\'' || c == '"' || c == '`':
			quote := c
			out.WriteByte(c)
			i++
			for i < n {
				out.WriteByte(s[i])
				if s[i] == '\\' && quote != '`' && i+1 < n {
					i++
					out.WriteByte(s[i])
					i++
					continue
				}
				if s[i] == quote {
					if i+1 < n && s[i+1] == quote {
						out.WriteByte(s[i+1])
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		default:
			out.WriteByte(c)
			i++
		}
	}
	return strings.TrimSpace(out.String())
}
