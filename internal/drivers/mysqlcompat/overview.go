package mysqlcompat

import (
	"context"
	"fmt"
	"strconv"

	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

// OverviewOptions say what the calling engine actually has, because the shared
// page reads counters that not every MySQL-compatible server fills in.
type OverviewOptions struct {
	// InnoDB keeps the "InnoDB and caches" group. TiDB and Doris answer those
	// counters with zeros (neither has an InnoDB buffer pool), so for them the
	// group would be a wall of zeroes pretending to be a measurement.
	InnoDB bool
}

// Overview builds the runtime status page of a MySQL-compatible server.
//
// Everything comes from SHOW GLOBAL STATUS / SHOW GLOBAL VARIABLES and
// SHOW FULL PROCESSLIST. Those have been stable since MySQL 4 and MariaDB 5, so
// one implementation covers every server we can reach — unlike
// performance_schema tables, which change shape between releases. Engines on
// top of this package append their own groups to the returned page.
func Overview(ctx context.Context, q sqlbase.Querier, opts OverviewOptions) (*models.ServerOverview, error) {
	status, err := ShowMap(ctx, q, "SHOW GLOBAL STATUS")
	if err != nil {
		return nil, err
	}
	variables, err := ShowMap(ctx, q, "SHOW GLOBAL VARIABLES")
	if err != nil {
		return nil, err
	}

	pageSize := Number(status, "Innodb_page_size")
	if pageSize <= 0 {
		pageSize = 16384
	}
	poolPages := Number(status, "Innodb_buffer_pool_pages_total")
	poolFree := Number(status, "Innodb_buffer_pool_pages_free")
	poolDirty := Number(status, "Innodb_buffer_pool_pages_dirty")
	poolSize := Number(variables, "innodb_buffer_pool_size")

	uptime := Number(status, "Uptime")
	running := Number(status, "Threads_running")
	maxConns := Number(variables, "max_connections")
	connected := Number(status, "Threads_connected")

	queries := Number(status, "Questions")
	qps := 0.0
	if uptime > 0 {
		qps = queries / uptime
	}

	server := []models.OverviewMetric{
		sqlbase.Metric("Build", StringOr(variables, "version_comment", StringOr(variables, "version", "—")),
			"version_comment — the server version itself is in the page header"),
		sqlbase.Metric("Host", StringOr(variables, "hostname", "—"), "hostname"),
		sqlbase.Metric("Port", StringOr(variables, "port", "—"), "port"),
		sqlbase.Metric("Data directory", StringOr(variables, "datadir", "—"), "datadir"),
		sqlbase.Metric("Uptime", sqlbase.FormatDuration(uptime), "Seconds since the server started"),
		sqlbase.Metric("Started", StartedAt(uptime), "Derived from Uptime"),
		sqlbase.Metric("Charset", StringOr(variables, "character_set_server", "—"),
			"character_set_server / collation_server"),
	}

	connections := []models.OverviewMetric{
		sqlbase.Metric("Connected threads", fmt.Sprintf("%s / %s", Count(connected), Count(maxConns)),
			"Threads_connected / max_connections"),
		sqlbase.Metric("Running threads", Count(running), "Threads_running"),
		sqlbase.Metric("Peak connections", PeakConnections(status, maxConns),
			"Max_used_connections / max_connections"),
		sqlbase.Metric("Threads created", Count(Number(status, "Threads_created")),
			"Threads the server had to create; a fast-growing number means the thread cache is too small"),
		sqlbase.Metric("Open tables", Count(Number(status, "Open_tables")), "Open_tables"),
	}
	if refused := Number(status, "Aborted_connects"); refused > 0 {
		connections = append(connections, WithState(
			sqlbase.Metric("Aborted connects", Count(refused), "Aborted_connects — handshakes that never finished"),
			"warn"))
	}
	if clients := Number(status, "Aborted_clients"); clients > 0 {
		connections = append(connections, sqlbase.Metric("Aborted clients", Count(clients),
			"Clients that disconnected without closing the session"))
	}

	queriesGroup := []models.OverviewMetric{
		sqlbase.Metric("Total queries", Count(queries), "Questions since the server started"),
		sqlbase.Metric("Queries per second", strconv.FormatFloat(qps, 'f', 2, 64), "Questions / Uptime"),
		sqlbase.Metric("SELECT", Count(Number(status, "Com_select")), "Com_select"),
		sqlbase.Metric("INSERT / UPDATE / DELETE", fmt.Sprintf("%s / %s / %s",
			Count(Number(status, "Com_insert")), Count(Number(status, "Com_update")),
			Count(Number(status, "Com_delete"))), "Com_insert, Com_update, Com_delete"),
		sqlbase.Metric("Rows read", Count(Number(status, "Innodb_rows_read")), "Innodb_rows_read"),
		sqlbase.Metric("Bytes received", sqlbase.FormatBytes(int64(Number(status, "Bytes_received"))), "Bytes_received"),
		sqlbase.Metric("Bytes sent", sqlbase.FormatBytes(int64(Number(status, "Bytes_sent"))), "Bytes_sent"),
	}
	if slow := Number(status, "Slow_queries"); slow > 0 {
		queriesGroup = append(queriesGroup, WithState(
			sqlbase.Metric("Slow queries", Count(slow), "Slow_queries — they took longer than long_query_time"),
			"warn"))
	}

	innodb := []models.OverviewMetric{
		sqlbase.Metric("Buffer pool size", sqlbase.FormatBytes(int64(poolSize)), "innodb_buffer_pool_size"),
		sqlbase.Metric("Buffer pool used",
			fmt.Sprintf("%s / %s", sqlbase.FormatBytes(int64((poolPages-poolFree)*pageSize)),
				sqlbase.FormatBytes(int64(poolPages*pageSize))),
			"Innodb_buffer_pool_pages_total - Innodb_buffer_pool_pages_free"),
		sqlbase.Metric("Read hit rate", sqlbase.FormatPercent(
			Number(status, "Innodb_buffer_pool_read_requests"),
			Number(status, "Innodb_buffer_pool_read_requests")+Number(status, "Innodb_buffer_pool_reads")),
			"Innodb_buffer_pool_read_requests vs Innodb_buffer_pool_reads"),
		sqlbase.Metric("Dirty pages", fmt.Sprintf("%s / %s", Count(poolDirty), Count(poolPages)),
			"Innodb_buffer_pool_pages_dirty"),
		sqlbase.Metric("Pages read from disk", Count(Number(status, "Innodb_buffer_pool_reads")),
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
		value := Number(status, probe.key)
		metric := sqlbase.Metric(probe.label, Count(value), probe.hint)
		if value > 0 {
			metric = WithState(metric, "warn")
		}
		innodb = append(innodb, metric)
	}

	groups := []models.OverviewGroup{
		{Title: "Server", Metrics: server},
		{Title: "Connections", Metrics: connections, Note: connectionNote(connected, maxConns)},
		{Title: "Queries", Metrics: queriesGroup},
	}
	if opts.InnoDB {
		groups = append(groups, models.OverviewGroup{Title: "InnoDB and caches", Metrics: innodb})
	}

	page := &models.ServerOverview{MySQL: &models.MySQLOverview{Groups: groups}}

	// The process list needs the PROCESS privilege. A user without it still
	// gets the counters above, so a failure here is a note, not an error.
	processes, err := ProcessList(ctx, q, "Process list")
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
