package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dbmanager/internal/apperr"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

// overview implements the Spec.Overview hook: the MySQL side of the runtime
// status page.
//
// Everything comes from SHOW GLOBAL STATUS / SHOW GLOBAL VARIABLES and
// SHOW FULL PROCESSLIST. Those have been stable since MySQL 4 and MariaDB 5, so
// one implementation covers every server we can reach — unlike
// performance_schema tables, which change shape between releases.
func overview(ctx context.Context, q sqlbase.Querier, cfg models.ConnectionConfig) (*models.ServerOverview, error) {
	status, err := showMap(ctx, q, "SHOW GLOBAL STATUS")
	if err != nil {
		return nil, err
	}
	variables, err := showMap(ctx, q, "SHOW GLOBAL VARIABLES")
	if err != nil {
		return nil, err
	}

	pageSize := number(status, "Innodb_page_size")
	if pageSize <= 0 {
		pageSize = 16384
	}
	poolPages := number(status, "Innodb_buffer_pool_pages_total")
	poolFree := number(status, "Innodb_buffer_pool_pages_free")
	poolDirty := number(status, "Innodb_buffer_pool_pages_dirty")
	poolSize := number(variables, "innodb_buffer_pool_size")

	uptime := number(status, "Uptime")
	running := number(status, "Threads_running")
	maxConns := number(variables, "max_connections")
	connected := number(status, "Threads_connected")

	queries := number(status, "Questions")
	qps := 0.0
	if uptime > 0 {
		qps = queries / uptime
	}

	server := []models.OverviewMetric{
		sqlbase.Metric("Build", stringOr(variables, "version_comment", stringOr(variables, "version", "—")),
			"version_comment — the server version itself is in the page header"),
		sqlbase.Metric("Host", stringOr(variables, "hostname", "—"), "hostname"),
		sqlbase.Metric("Port", stringOr(variables, "port", "—"), "port"),
		sqlbase.Metric("Data directory", stringOr(variables, "datadir", "—"), "datadir"),
		sqlbase.Metric("Uptime", sqlbase.FormatDuration(uptime), "Seconds since the server started"),
		sqlbase.Metric("Started", startedAt(uptime), "Derived from Uptime"),
		sqlbase.Metric("Charset", stringOr(variables, "character_set_server", "—"),
			"character_set_server / collation_server"),
	}

	connections := []models.OverviewMetric{
		sqlbase.Metric("Connected threads", fmt.Sprintf("%s / %s", count(connected), count(maxConns)),
			"Threads_connected / max_connections"),
		sqlbase.Metric("Running threads", count(running), "Threads_running"),
		sqlbase.Metric("Peak connections", connectionPeak(status, maxConns),
			"Max_used_connections / max_connections"),
		sqlbase.Metric("Threads created", count(number(status, "Threads_created")),
			"Threads the server had to create; a fast-growing number means the thread cache is too small"),
		sqlbase.Metric("Open tables", count(number(status, "Open_tables")), "Open_tables"),
	}
	if refused := number(status, "Aborted_connects"); refused > 0 {
		connections = append(connections, withState(
			sqlbase.Metric("Aborted connects", count(refused), "Aborted_connects — handshakes that never finished"),
			"warn"))
	}
	if clients := number(status, "Aborted_clients"); clients > 0 {
		connections = append(connections, sqlbase.Metric("Aborted clients", count(clients),
			"Clients that disconnected without closing the session"))
	}

	queriesGroup := []models.OverviewMetric{
		sqlbase.Metric("Total queries", count(queries), "Questions since the server started"),
		sqlbase.Metric("Queries per second", strconv.FormatFloat(qps, 'f', 2, 64), "Questions / Uptime"),
		sqlbase.Metric("SELECT", count(number(status, "Com_select")), "Com_select"),
		sqlbase.Metric("INSERT / UPDATE / DELETE", fmt.Sprintf("%s / %s / %s",
			count(number(status, "Com_insert")), count(number(status, "Com_update")),
			count(number(status, "Com_delete"))), "Com_insert, Com_update, Com_delete"),
		sqlbase.Metric("Rows read", count(number(status, "Innodb_rows_read")), "Innodb_rows_read"),
		sqlbase.Metric("Bytes received", sqlbase.FormatBytes(int64(number(status, "Bytes_received"))), "Bytes_received"),
		sqlbase.Metric("Bytes sent", sqlbase.FormatBytes(int64(number(status, "Bytes_sent"))), "Bytes_sent"),
	}
	if slow := number(status, "Slow_queries"); slow > 0 {
		queriesGroup = append(queriesGroup, withState(
			sqlbase.Metric("Slow queries", count(slow), "Slow_queries — they took longer than long_query_time"),
			"warn"))
	}

	innodb := []models.OverviewMetric{
		sqlbase.Metric("Buffer pool size", sqlbase.FormatBytes(int64(poolSize)), "innodb_buffer_pool_size"),
		sqlbase.Metric("Buffer pool used",
			fmt.Sprintf("%s / %s", sqlbase.FormatBytes(int64((poolPages-poolFree)*pageSize)),
				sqlbase.FormatBytes(int64(poolPages*pageSize))),
			"Innodb_buffer_pool_pages_total - Innodb_buffer_pool_pages_free"),
		sqlbase.Metric("Read hit rate", sqlbase.FormatPercent(
			number(status, "Innodb_buffer_pool_read_requests"),
			number(status, "Innodb_buffer_pool_read_requests")+number(status, "Innodb_buffer_pool_reads")),
			"Innodb_buffer_pool_read_requests vs Innodb_buffer_pool_reads"),
		sqlbase.Metric("Dirty pages", fmt.Sprintf("%s / %s", count(poolDirty), count(poolPages)),
			"Innodb_buffer_pool_pages_dirty"),
		sqlbase.Metric("Pages read from disk", count(number(status, "Innodb_buffer_pool_reads")),
			"Innodb_buffer_pool_reads"),
	}
	// A log wait or a free-page wait is the classic "buffer pool too small"
	// signal, so they are the two counters worth colouring.
	for _, probe := range []struct {
		key   string
		label string
		hint  string
	}{
		{"Innodb_buffer_pool_wait_free", "Buffer pool waits", "Innodb_buffer_pool_wait_free — the pool had to flush before it could allocate"},
		{"Innodb_log_waits", "Log waits", "Innodb_log_waits — the redo log was too small for the write burst"},
		{"Innodb_row_lock_waits", "Row lock waits", "Innodb_row_lock_waits"},
	} {
		value := number(status, probe.key)
		metric := sqlbase.Metric(probe.label, count(value), probe.hint)
		if value > 0 {
			metric = withState(metric, "warn")
		}
		innodb = append(innodb, metric)
	}

	page := &models.ServerOverview{
		MySQL: &models.MySQLOverview{
			Groups: []models.OverviewGroup{
				{Title: "Server", Metrics: server},
				{Title: "Connections", Metrics: connections, Note: connectionNote(connected, maxConns)},
				{Title: "Queries", Metrics: queriesGroup},
				{Title: "InnoDB and caches", Metrics: innodb},
			},
		},
	}

	// The process list needs the PROCESS privilege. A user without it still
	// gets the counters above, so a failure here is a note, not an error.
	processes, err := processList(ctx, q)
	if err != nil {
		page.Warnings = append(page.Warnings, "The process list needs the PROCESS privilege: "+err.Error())
	} else {
		page.MySQL.Processes = processes
	}
	return page, nil
}

// connectionNote adds a one-liner when the connection count is worth a look.
func connectionNote(connected, maxConns float64) string {
	if maxConns > 0 && connected/maxConns >= 0.8 {
		return "More than 80% of max_connections is in use."
	}
	return ""
}

// processList reads SHOW FULL PROCESSLIST into a table.
func processList(ctx context.Context, q sqlbase.Querier) (*models.OverviewTable, error) {
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
		Title:   "Process list",
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

// showMap runs a "SHOW ..." statement with two text columns and returns a
// lookup table keyed by the first one.
func showMap(ctx context.Context, q sqlbase.Querier, statement string) (map[string]string, error) {
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

// number reads a numeric variable or counter. Missing keys and non-numeric
// values read as 0, so an older server simply shows fewer numbers instead of
// failing the whole page.
func number(values map[string]string, key string) float64 {
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

func stringOr(values map[string]string, key, fallback string) string {
	if value := strings.TrimSpace(values[strings.ToLower(key)]); value != "" {
		return value
	}
	return fallback
}

// count renders a counter with grouping, without dragging in a dependency for
// it.
func count(value float64) string { return sqlbase.FormatCount(int64(value)) }

// startedAt turns "seconds of uptime" into a wall-clock start time.
func startedAt(uptime float64) string {
	if uptime <= 0 {
		return "—"
	}
	return time.Now().Add(-time.Duration(uptime) * time.Second).Format("2006-01-02 15:04:05")
}

func connectionPeak(status map[string]string, maxConns float64) string {
	peak := number(status, "Max_used_connections")
	if maxConns <= 0 {
		return count(peak)
	}
	return fmt.Sprintf("%s / %s", count(peak), count(maxConns))
}

func withState(metric models.OverviewMetric, state string) models.OverviewMetric {
	metric.State = state
	return metric
}
