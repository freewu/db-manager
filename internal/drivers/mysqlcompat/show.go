package mysqlcompat

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

// ProcessList reads SHOW FULL PROCESSLIST into a table for the status page.
//
// The process list needs a privilege (PROCESS on MySQL, ADMIN on some builds):
// a user without it still gets the counters the caller collected, so callers
// report a failure here as a note, not as an error.
func ProcessList(ctx context.Context, q sqlbase.Querier, title string) (*models.OverviewTable, error) {
	rows, err := q.QueryContext(ctx, "SHOW FULL PROCESSLIST")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	table := &models.OverviewTable{
		Title:   title,
		Columns: columns,
		Rows:    [][]string{},
	}
	const limit = 100
	for rows.Next() {
		if len(table.Rows) >= limit {
			table.Note = fmt.Sprintf("Only the first %d sessions are shown.", limit)
			break
		}
		cells := make([]sql.NullString, len(columns))
		targets := make([]any, len(columns))
		for i := range cells {
			targets[i] = &cells[i]
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "read the process list")
		}
		row := make([]string, len(columns))
		for i, cell := range cells {
			row[i] = strings.TrimSpace(cell.String)
		}
		table.Rows = append(table.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "read the process list")
	}
	if len(table.Rows) == 0 {
		return nil, nil
	}
	return table, nil
}

// ShowMap runs a "SHOW ..." statement whose two columns are name and value, and
// returns them keyed by the lower-cased name. MySQL's SHOW GLOBAL STATUS and
// SHOW VARIABLES both have that shape, and so do SHOW FRONTENDS/BACKENDS after
// being pivoted through ShowRows.
func ShowMap(ctx context.Context, q sqlbase.Querier, statement string) (map[string]string, error) {
	rows, err := q.QueryContext(ctx, statement)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "%s", strings.ToLower(statement))
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var name, value sql.NullString
		if err := rows.Scan(&name, &value); err != nil {
			return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "read %s", strings.ToLower(statement))
		}
		out[strings.ToLower(name.String)] = value.String
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "read %s", strings.ToLower(statement))
	}
	return out, nil
}

// ShowRow is one row of a SHOW statement, addressed by column name.
type ShowRow map[string]string

// ShowRows reads any SHOW statement generically: the columns differ per engine
// and per version (SHOW FRONTENDS has a dozen), so a fixed scan target list
// would break on the next release.
func ShowRows(ctx context.Context, q sqlbase.Querier, statement string) ([]ShowRow, error) {
	rows, err := q.QueryContext(ctx, statement)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "%s", strings.ToLower(statement))
	}
	defer rows.Close()

	out, err := readShowRows(rows)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "read %s", strings.ToLower(statement))
	}
	return out, nil
}

// readShowRows drains a SHOW statement whose result is still open.
func readShowRows(rows *sql.Rows) ([]ShowRow, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []ShowRow{}
	for rows.Next() {
		cells := make([]sql.NullString, len(columns))
		targets := make([]any, len(columns))
		for i := range cells {
			targets[i] = &cells[i]
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, err
		}
		row := ShowRow{}
		for i, name := range columns {
			row[strings.ToLower(name)] = strings.TrimSpace(cells[i].String)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// IndexesFromShow reads the output of SHOW INDEX, whose column names are the
// same wherever MySQL's SHOW INDEX was copied from. Engines that keep their
// indexes out of information_schema.STATISTICS (Doris) use it instead of the
// catalog query.
func IndexesFromShow(rows *sql.Rows) ([]models.IndexInfo, error) {
	showRows, err := readShowRows(rows)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(showRows, func(i, j int) bool {
		if a, b := showRows[i].Get("key_name"), showRows[j].Get("key_name"); a != b {
			return a < b
		}
		return showRows[i].Get("seq_in_index") < showRows[j].Get("seq_in_index")
	})

	byName := map[string]*models.IndexInfo{}
	order := []string{}
	for _, row := range showRows {
		name := row.Get("key_name")
		if name == "" {
			continue
		}
		index, ok := byName[name]
		if !ok {
			index = &models.IndexInfo{
				Name:    name,
				Unique:  row.Get("non_unique") == "0",
				Primary: strings.EqualFold(name, "PRIMARY"),
				Method:  row.Get("index_type"),
				Comment: row.Get("comment"),
				Columns: []string{},
			}
			byName[name] = index
			order = append(order, name)
		}
		column := row.Get("column_name")
		if column == "" {
			// A functional index has no column name; dropping the part would
			// misreport the index.
			column = "(expression)"
		}
		index.Columns = append(index.Columns, column)
	}

	out := make([]models.IndexInfo, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	return out, nil
}

// Get reads a column of a row, whichever of the given names the engine used.
func (r ShowRow) Get(names ...string) string {
	for _, name := range names {
		if value, ok := r[strings.ToLower(name)]; ok && value != "" {
			return value
		}
	}
	return ""
}

// Number reads a numeric variable or counter. Missing keys and non-numeric
// values read as 0, so an older server simply shows fewer numbers instead of
// failing the whole page.
func Number(values map[string]string, key string) float64 {
	raw, ok := values[strings.ToLower(key)]
	if !ok {
		return 0
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0
	}
	return parsed
}

// StringOr reads a value, falling back when it is missing or blank.
func StringOr(values map[string]string, key, fallback string) string {
	if value := strings.TrimSpace(values[strings.ToLower(key)]); value != "" {
		return value
	}
	return fallback
}

// Count renders a counter with grouping, without dragging in a dependency for
// it.
func Count(value float64) string { return sqlbase.FormatCount(int64(value)) }

// PeakConnections renders Max_used_connections against the configured ceiling.
func PeakConnections(status map[string]string, maxConns float64) string {
	peak := Number(status, "Max_used_connections")
	if maxConns <= 0 {
		return Count(peak)
	}
	return fmt.Sprintf("%s / %s", Count(peak), Count(maxConns))
}

// StartedAt turns "seconds of uptime" into a wall-clock start time.
func StartedAt(uptime float64) string {
	if uptime <= 0 {
		return "—"
	}
	return time.Now().Add(-time.Duration(uptime) * time.Second).Format("2006-01-02 15:04:05")
}

// WithState colours a metric when the number deserves a second look.
func WithState(metric models.OverviewMetric, state string) models.OverviewMetric {
	metric.State = state
	return metric
}
