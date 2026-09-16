package sqlbase

import (
	"fmt"
	"strings"

	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// RenderCreateTable produces a portable CREATE TABLE statement from the
// introspected structure.
//
// Engines that can do better supply Spec.NativeDDL (MySQL's SHOW CREATE TABLE,
// SQLite's sqlite_master.sql); this renderer is the fallback and is also what
// makes the "copy DDL" button work for engines without a native form.
func RenderCreateTable(
	d drivers.Dialect,
	obj models.ObjectInfo,
	cols []models.ColumnInfo,
	indexes []models.IndexInfo,
	fks []models.ForeignKeyInfo,
) string {
	if len(cols) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("CREATE ")
	if obj.Kind == models.KindView || obj.Kind == models.KindMatView {
		b.WriteString("VIEW ")
	} else {
		b.WriteString("TABLE ")
	}
	b.WriteString(d.Qualify(obj.Database, obj.Schema, obj.Name))
	b.WriteString(" (\n")

	lines := make([]string, 0, len(cols)+len(indexes)+len(fks))
	pk := make([]string, 0, 2)

	for _, c := range cols {
		line := "  " + d.Quote(c.Name) + " " + columnType(c)
		if c.AutoIncrement {
			// Portable-ish spelling; MySQL understands AUTO_INCREMENT, others
			// use identity/serial syntax which we cannot infer generically.
			line += " AUTO_INCREMENT"
		}
		if !c.Nullable {
			line += " NOT NULL"
		}
		if c.DefaultValue != nil && *c.DefaultValue != "" {
			def := *c.DefaultValue
			// Values coming from information_schema are sometimes already
			// expressions (nextval(...), CURRENT_TIMESTAMP). Wrap bare
			// literals in quotes instead of guessing per engine.
			if isBareLiteral(def) {
				def = "'" + strings.ReplaceAll(def, "'", "''") + "'"
			}
			line += " DEFAULT " + def
		}
		lines = append(lines, line)

		if c.PrimaryKey {
			pk = append(pk, d.Quote(c.Name))
		}
	}

	if len(pk) > 0 {
		lines = append(lines, "  PRIMARY KEY ("+strings.Join(pk, ", ")+")")
	}

	for _, ix := range indexes {
		if ix.Primary {
			continue
		}
		cols := quoteAll(d, ix.Columns)
		if len(cols) == 0 {
			continue
		}
		kind := "INDEX"
		if ix.Unique {
			kind = "UNIQUE INDEX"
		}
		lines = append(lines, fmt.Sprintf("  %s %s (%s)", kind, d.Quote(ix.Name), strings.Join(cols, ", ")))
	}

	for _, fk := range fks {
		cols := quoteAll(d, fk.Columns)
		refCols := quoteAll(d, fk.ReferencedColumns)
		if len(cols) == 0 || len(refCols) == 0 {
			continue
		}
		line := fmt.Sprintf("  CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
			d.Quote(fk.Name),
			strings.Join(cols, ", "),
			d.Qualify("", fk.ReferencedSchema, fk.ReferencedTable),
			strings.Join(refCols, ", "),
		)
		if fk.OnDelete != "" && !strings.EqualFold(fk.OnDelete, "NO ACTION") {
			line += " ON DELETE " + fk.OnDelete
		}
		if fk.OnUpdate != "" && !strings.EqualFold(fk.OnUpdate, "NO ACTION") {
			line += " ON UPDATE " + fk.OnUpdate
		}
		lines = append(lines, line)
	}

	b.WriteString(strings.Join(lines, ",\n"))
	b.WriteString("\n);")
	return b.String()
}

func columnType(c models.ColumnInfo) string {
	if c.ColumnType != "" {
		return c.ColumnType
	}
	if c.DataType != "" {
		return c.DataType
	}
	return "text"
}

func quoteAll(d drivers.Dialect, idents []string) []string {
	out := make([]string, 0, len(idents))
	for _, i := range idents {
		if strings.TrimSpace(i) == "" {
			continue
		}
		out = append(out, d.Quote(i))
	}
	return out
}

// isBareLiteral reports whether a default expression needs quoting. Anything
// containing parentheses or SQL keywords is treated as an expression.
func isBareLiteral(def string) bool {
	upper := strings.ToUpper(strings.TrimSpace(def))
	switch upper {
	case "NULL", "CURRENT_TIMESTAMP", "CURRENT_DATE", "CURRENT_TIME", "TRUE", "FALSE":
		return false
	}
	if strings.ContainsAny(def, "()") {
		return false
	}
	if strings.HasPrefix(def, "'") || strings.HasPrefix(def, "\"") {
		return false
	}
	// :: casts (PostgreSQL) and nextval sequences are expressions.
	if strings.Contains(def, "::") || strings.Contains(upper, "NEXTVAL") {
		return false
	}
	return true
}

// BuildPreview renders the SELECT that Fetch would run, so the UI can show the
// user exactly which query produced a grid page.
func BuildPreview(d drivers.Dialect, database, schema, object string, limit, offset int) string {
	return "SELECT * FROM " + d.Qualify(database, schema, object) + d.LimitOffset(limit, offset)
}
