package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

// overview implements the Spec.Overview hook: the SQLite side of the runtime
// status page.
//
// There is no server to report on, so this page describes the file: what it
// occupies on disk, which pragmas define its format, and what is stored in it.
// That is exactly the information a SQLite user needs and that no client/server
// engine has to show.
func overview(ctx context.Context, q sqlbase.Querier, cfg models.ConnectionConfig) (*models.ServerOverview, error) {
	pragmas, warnings, err := pragmaValues(ctx, q, "page_size", "page_count", "freelist_count",
		"journal_mode", "auto_vacuum", "encoding", "schema_version", "user_version", "cache_size",
		"synchronous", "foreign_keys", "locking_mode", "wal_autocheckpoint")
	if err != nil {
		return nil, err
	}

	pageSize, _ := pragmas["page_size"].(int64)
	pages, _ := pragmas["page_count"].(int64)
	free, _ := pragmas["freelist_count"].(int64)
	allocated := pageSize * pages

	// ":memory:" has no file, and a file the user cannot stat is worth reporting
	// as unknown rather than as zero.
	fileSize := int64(-1)
	modified := "—"
	if info, err := os.Stat(cfg.FilePath); err == nil && !info.IsDir() {
		fileSize = info.Size()
		modified = info.ModTime().Format("2006-01-02 15:04:05")
	}

	fileMetrics := []models.OverviewMetric{
		sqlbase.Metric("File", cfg.FilePath, "The database this session opened"),
		sqlbase.Metric("Size on disk", sqlbase.FormatBytes(fileSize), "os.Stat of the file"),
		sqlbase.Metric("Modified", modified, "Last write to the file"),
		sqlbase.Metric("Allocated", sqlbase.FormatBytes(allocated),
			"page_size × page_count — the size SQLite reports for itself"),
		sqlbase.Metric("Session", sessionMode(cfg), "How this session opened the file"),
	}
	fileNote := ""
	if allocated > 0 && fileSize > allocated {
		// WAL and unvacuumed free space are the two usual explanations, so the
		// note names both instead of guessing.
		fileNote = "The file is larger than the pages it holds: a write-ahead log or free space " +
			"outside the database is waiting to be checkpointed or vacuumed."
	}

	formatMetrics := []models.OverviewMetric{
		sqlbase.Metric("Page size", bytesOf(pageSize), "PRAGMA page_size"),
		sqlbase.Metric("Page count", sqlbase.FormatCount(pages), "PRAGMA page_count"),
		sqlbase.Metric("Free pages", freePages(free, pages),
			"PRAGMA freelist_count / page_count — space a VACUUM would return"),
		sqlbase.Metric("Page cache", textOf(pragmas["cache_size"]),
			"PRAGMA cache_size — negative is a size in KiB, positive a page count"),
		sqlbase.Metric("Journal mode", textOf(pragmas["journal_mode"]), "PRAGMA journal_mode"),
		sqlbase.Metric("Synchronous", textOf(pragmas["synchronous"]), "PRAGMA synchronous"),
		sqlbase.Metric("Auto vacuum", autoVacuumLabel(pragmas["auto_vacuum"]), "PRAGMA auto_vacuum"),
		sqlbase.Metric("Encoding", textOf(pragmas["encoding"]), "PRAGMA encoding"),
		sqlbase.Metric("Schema version", textOf(pragmas["schema_version"]), "PRAGMA schema_version"),
		sqlbase.Metric("User version", textOf(pragmas["user_version"]), "PRAGMA user_version"),
		sqlbase.Metric("Foreign keys", boolLabel(intOf(pragmas["foreign_keys"]) == 1),
			"PRAGMA foreign_keys — a property of this session, not of the file"),
		sqlbase.Metric("Locking mode", textOf(pragmas["locking_mode"]), "PRAGMA locking_mode"),
		sqlbase.Metric("WAL autocheckpoint", textOf(pragmas["wal_autocheckpoint"]),
			"PRAGMA wal_autocheckpoint, in pages"),
	}
	if intOf(pragmas["foreign_keys"]) != 1 {
		formatMetrics[len(formatMetrics)-3] = withState(formatMetrics[len(formatMetrics)-3], "warn")
	}

	// The object list is capped: a schema with tens of thousands of tables can
	// be stored but not rendered, and the counts above already tell the story.
	objects, err := schemaObjects(ctx, q)
	if err != nil {
		return nil, err
	}

	page := &models.ServerOverview{
		Warnings: warnings,
		SQLite: &models.SQLiteOverview{
			Path:     cfg.FilePath,
			FileSize: fileSize,
			Groups: []models.OverviewGroup{
				{Title: "File", Note: fileNote, Metrics: fileMetrics},
				{Title: "Format and pragmas", Metrics: formatMetrics},
			},
			Objects: objects,
		},
	}

	attached, err := attachedDatabases(ctx, q)
	if err != nil {
		page.Warnings = append(page.Warnings, "Could not list attached databases: "+err.Error())
	} else {
		page.SQLite.Attached = attached
	}
	return page, nil
}

// pragmaValues reads a list of pragmas. A PRAGMA name is part of the statement
// grammar and cannot be parameterised, so the names are inlined — the callers
// pass literals from this file, never user input.
//
// A pragma this build does not know is reported as a warning instead of failing
// the page: the rest of the numbers are still true.
func pragmaValues(ctx context.Context, q sqlbase.Querier, names ...string) (map[string]any, []string, error) {
	values := make(map[string]any, len(names))
	warnings := []string{}
	for _, name := range names {
		var cell sql.NullString
		if err := q.QueryRowContext(ctx, "PRAGMA "+name).Scan(&cell); err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			warnings = append(warnings, fmt.Sprintf("PRAGMA %s could not be read: %v", name, err))
			continue
		}
		values[name] = normalizePragma(cell)
	}
	return values, warnings, nil
}

// normalizePragma keeps numbers as numbers so callers can compute with them, and
// everything else as text.
func normalizePragma(cell sql.NullString) any {
	if !cell.Valid {
		return ""
	}
	raw := strings.TrimSpace(cell.String)
	if value, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return value
	}
	return raw
}

// schemaObjects lists the tables, views, indexes and triggers of the main
// schema together with the table each one belongs to.
func schemaObjects(ctx context.Context, q sqlbase.Querier) (*models.OverviewTable, error) {
	rows, err := q.QueryContext(ctx, `
SELECT type, name, coalesce(tbl_name, '')
FROM main.sqlite_master
WHERE name NOT LIKE 'sqlite_%'
ORDER BY type, name
LIMIT 201`)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "list schema objects")
	}
	defer rows.Close()

	table := &models.OverviewTable{
		Title:   "Schema objects",
		Columns: []string{"Type", "Name", "Table"},
		Rows:    [][]string{},
	}
	const limit = 200
	for rows.Next() {
		if len(table.Rows) >= limit {
			table.Note = fmt.Sprintf("Only the first %d objects are shown.", limit)
			break
		}
		var kind, name, owner string
		if err := rows.Scan(&kind, &name, &owner); err != nil {
			return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "read schema objects")
		}
		table.Rows = append(table.Rows, []string{kind, name, owner})
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "read schema objects")
	}
	if len(table.Rows) == 0 {
		return nil, nil
	}
	return table, nil
}

// attachedDatabases reports the ATTACHed files — the closest thing SQLite has to
// "the other databases on this server".
func attachedDatabases(ctx context.Context, q sqlbase.Querier) (*models.OverviewTable, error) {
	rows, err := q.QueryContext(ctx, "PRAGMA database_list")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	table := &models.OverviewTable{
		Title:   "Attached databases",
		Columns: []string{"Seq", "Name", "File"},
		Rows:    [][]string{},
	}
	for rows.Next() {
		var (
			seq  int64
			name string
			file string
		)
		if err := rows.Scan(&seq, &name, &file); err != nil {
			return nil, err
		}
		if strings.EqualFold(name, "temp") {
			continue
		}
		table.Rows = append(table.Rows, []string{strconv.FormatInt(seq, 10), name, file})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(table.Rows) == 0 {
		return nil, nil
	}
	return table, nil
}

// --- small label helpers ---------------------------------------------------

// sessionMode says how the session opened the file, which is what decides
// whether writes are even possible.
func sessionMode(cfg models.ConnectionConfig) string {
	if cfg.ReadOnly {
		return "read-only"
	}
	if mode := strings.TrimSpace(cfg.Params["mode"]); mode != "" {
		return "read-write (mode=" + mode + ")"
	}
	return "read-write"
}

func bytesOf(n int64) string {
	if n <= 0 {
		return "—"
	}
	return fmt.Sprintf("%s (%d bytes)", sqlbase.FormatBytes(n), n)
}

func freePages(free, pages int64) string {
	if pages <= 0 {
		return "—"
	}
	return fmt.Sprintf("%s (%s)", sqlbase.FormatCount(free),
		sqlbase.FormatPercent(float64(free), float64(pages)))
}

func autoVacuumLabel(value any) string {
	switch intOf(value) {
	case 0:
		return "none"
	case 1:
		return "full"
	case 2:
		return "incremental"
	default:
		return textOf(value)
	}
}

func boolLabel(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func intOf(value any) int64 {
	number, _ := value.(int64)
	return number
}

func textOf(value any) string {
	switch typed := value.(type) {
	case nil:
		return "—"
	case string:
		if typed == "" {
			return "—"
		}
		return typed
	case int64:
		return sqlbase.FormatCount(typed)
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func withState(metric models.OverviewMetric, state string) models.OverviewMetric {
	metric.State = state
	return metric
}
