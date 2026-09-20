package postgres

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

// overview implements the Spec.Overview hook: the PostgreSQL side of the
// runtime status page.
//
// PostgreSQL answers through pg_stat_* views. Those carry the columns used here
// (counts, block reads, wait events, per-database sizes) from 9.6 through 17, so
// a single implementation works without branching: anything newer is additive.
// The one thing that does vary is privilege — a plain user cannot see other
// backends' queries or the size of databases it may not connect to — so each
// optional section degrades into a warning instead of failing the page, and a
// query that returns nothing leaves a dash rather than a zero that would be a
// lie about a busy server.
func overview(ctx context.Context, q sqlbase.Querier, cfg models.ConnectionConfig) (*models.ServerOverview, error) {
	page := &models.ServerOverview{Postgres: &models.PostgresOverview{}, Warnings: []string{}}

	settings, err := settingsMap(ctx, q, "server_version", "data_directory", "port", "TimeZone",
		"server_encoding", "max_connections", "shared_buffers", "wal_level")
	if err != nil {
		page.Warnings = append(page.Warnings, "The server settings could not be read: "+err.Error())
		settings = map[string]string{}
	}

	// A start time in the future (a clock moved backwards) would make the uptime
	// negative, which is worse than no uptime at all.
	uptime := "—"
	startedLabel := "—"
	if started, err := queryTime(ctx, q, "SELECT pg_postmaster_start_time()"); err != nil {
		page.Warnings = append(page.Warnings, "The server start time could not be read: "+err.Error())
	} else if !started.IsZero() {
		startedLabel = started.Format("2006-01-02 15:04:05")
		if since := time.Since(started).Seconds(); since >= 0 {
			uptime = sqlbase.FormatDuration(since)
		}
	}

	groups := []models.OverviewGroup{
		{
			Title: "Server",
			Metrics: []models.OverviewMetric{
				sqlbase.Metric("Version", sqlbase.Dash(settings["server_version"]), "server_version"),
				sqlbase.Metric("Started", startedLabel, "pg_postmaster_start_time()"),
				sqlbase.Metric("Uptime", uptime, "Since pg_postmaster_start_time()"),
				sqlbase.Metric("Data directory", sqlbase.Dash(settings["data_directory"]), "data_directory"),
				sqlbase.Metric("Port", sqlbase.Dash(settings["port"]), "port"),
				sqlbase.Metric("Encoding", sqlbase.Dash(settings["server_encoding"]), "server_encoding"),
				sqlbase.Metric("Time zone", sqlbase.Dash(settings["TimeZone"]), "TimeZone"),
				sqlbase.Metric("WAL level", sqlbase.Dash(settings["wal_level"]), "wal_level"),
				sqlbase.Metric("Shared buffers", sqlbase.Dash(settings["shared_buffers"]), "shared_buffers"),
			},
		},
	}

	// --- who is connected, and what are they doing --------------------------
	// -1 means "not answered": pg_stat_activity is readable by every user, but a
	// failure here should cost one group, not the page.
	var (
		total, active, idleInTx, waiting, longRunning int64 = -1, -1, -1, -1, -1
		maxBackends                                   int64
	)
	err = q.QueryRowContext(ctx, `
SELECT count(*),
       count(*) FILTER (WHERE state = 'active'),
       count(*) FILTER (WHERE state = 'idle in transaction'),
       count(*) FILTER (WHERE wait_event_type IN ('Lock', 'LWLock')),
       count(*) FILTER (WHERE state = 'active' AND now() - query_start > interval '5 seconds')
FROM pg_stat_activity`).Scan(&total, &active, &idleInTx, &waiting, &longRunning)
	if err != nil {
		page.Warnings = append(page.Warnings, "pg_stat_activity could not be read: "+err.Error())
		total, active, idleInTx, waiting, longRunning = -1, -1, -1, -1, -1
	}
	maxBackends, _ = strconv.ParseInt(settings["max_connections"], 10, 64)

	activity := []models.OverviewMetric{
		sqlbase.Metric("Backends", fmt.Sprintf("%s / %s", count(total), count(maxBackends)),
			"pg_stat_activity vs max_connections"),
		sqlbase.Metric("Active queries", count(active), "state = 'active'"),
		sqlbase.Metric("Idle", count(idle(active, total, idleInTx)), "state = 'idle'"),
	}
	if idleInTx > 0 {
		activity = append(activity, withState(sqlbase.Metric("Idle in transaction", count(idleInTx),
			"state = 'idle in transaction' — these hold locks and block vacuum"), "warn"))
	}
	if waiting > 0 {
		activity = append(activity, withState(sqlbase.Metric("Waiting on a lock", count(waiting),
			"wait_event_type is Lock or LWLock"), "warn"))
	}
	if longRunning > 0 {
		activity = append(activity, withState(sqlbase.Metric("Running over 5s", count(longRunning),
			"Active queries started more than 5 seconds ago"), "warn"))
	}
	groups = append(groups, models.OverviewGroup{Title: "Connections", Metrics: activity})

	// --- throughput and cache health ----------------------------------------
	var (
		commits, rollbacks, blocksRead, blocksHit float64 = -1, -1, -1, -1
		deadlocks, conflicts, tempFiles           float64 = -1, -1, -1
		tempBytes                                 float64 = -1
	)
	err = q.QueryRowContext(ctx, `
SELECT coalesce(sum(xact_commit), 0),
       coalesce(sum(xact_rollback), 0),
       coalesce(sum(blks_read), 0),
       coalesce(sum(blks_hit), 0),
       coalesce(sum(deadlocks), 0),
       coalesce(sum(conflicts), 0),
       coalesce(sum(temp_files), 0),
       coalesce(sum(temp_bytes), 0)
FROM pg_stat_database`).Scan(&commits, &rollbacks, &blocksRead, &blocksHit,
		&deadlocks, &conflicts, &tempFiles, &tempBytes)
	if err != nil {
		page.Warnings = append(page.Warnings, "pg_stat_database could not be read: "+err.Error())
		commits, rollbacks, blocksRead, blocksHit = -1, -1, -1, -1
		deadlocks, conflicts, tempFiles, tempBytes = -1, -1, -1, -1
	}

	throughput := []models.OverviewMetric{
		sqlbase.Metric("Transactions committed", count(int64(commits)), "xact_commit, all databases"),
		sqlbase.Metric("Transactions rolled back", count(int64(rollbacks)), "xact_rollback, all databases"),
		sqlbase.Metric("Commit ratio", sqlbase.FormatPercent(commits, commits+rollbacks),
			"xact_commit vs xact_rollback"),
		sqlbase.Metric("Cache hit rate", sqlbase.FormatPercent(blocksHit, blocksHit+blocksRead),
			"blks_hit vs blks_read, all databases"),
		sqlbase.Metric("Blocks read from disk", count(int64(blocksRead)), "blks_read"),
		sqlbase.Metric("Temp files", tempFilesLabel(tempFiles, tempBytes),
			"temp_files and temp_bytes — sorting and hashing that did not fit in work_mem"),
	}
	if deadlocks > 0 {
		throughput = append(throughput, withState(sqlbase.Metric("Deadlocks", count(int64(deadlocks)),
			"deadlocks, all databases"), "warn"))
	}
	if conflicts > 0 {
		throughput = append(throughput, withState(sqlbase.Metric("Recovery conflicts", count(int64(conflicts)),
			"conflicts — queries cancelled by recovery on a standby"), "warn"))
	}
	groups = append(groups, models.OverviewGroup{Title: "Throughput", Metrics: throughput})
	page.Postgres.Groups = groups

	databases, err := databaseSizes(ctx, q)
	if err != nil {
		page.Warnings = append(page.Warnings,
			"Database sizes need the CONNECT privilege on each database: "+err.Error())
	} else {
		page.Postgres.Databases = databases
	}

	queries, err := runningQueries(ctx, q)
	if err != nil {
		page.Warnings = append(page.Warnings,
			"Other sessions' queries are only visible to superusers and pg_read_all_stats members: "+err.Error())
	} else {
		page.Postgres.Activity = queries
	}
	return page, nil
}

// count renders a counter that was never read as a dash instead of a zero.
func count(value int64) string {
	if value < 0 {
		return "—"
	}
	return sqlbase.FormatCount(value)
}

// idle derives the number of idle backends, and keeps -1 ("unknown") from
// turning into a confident zero when the arithmetic is not available.
func idle(active, total, idleInTx int64) int64 {
	if active < 0 || total < 0 || idleInTx < 0 {
		return -1
	}
	return total - active - idleInTx
}

// tempFilesLabel pairs the spill count with its size, and degrades to a dash
// when neither could be read.
func tempFilesLabel(files, bytes float64) string {
	if files < 0 {
		return "—"
	}
	return sqlbase.FormatCount(int64(files)) + " · " + sqlbase.FormatBytes(int64(bytes))
}

// settingsMap reads a list of settings in one round trip. The names are ours
// (never user input), so they are inlined as literals rather than parameters:
// current_setting() takes a name, not a value. The 2-argument form asks for a
// NULL instead of an error on a name this release does not know, and the ::text
// cast keeps the VALUES list from being inferred as an anonymous type.
func settingsMap(ctx context.Context, q sqlbase.Querier, names ...string) (map[string]string, error) {
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("(%s::text)", sqlLiteral(name)))
	}
	query := "SELECT t.name, coalesce(current_setting(t.name, true), '') FROM (VALUES " +
		strings.Join(parts, ", ") + ") AS t(name)"

	rows, err := q.QueryContext(ctx, query)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "read server settings")
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "read server settings")
		}
		out[name] = value
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Wrap(apperr.CodeQueryFailed, err, "read server settings")
	}
	return out, nil
}

// sqlLiteral renders a trusted, internal string as a SQL literal.
func sqlLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func queryTime(ctx context.Context, q sqlbase.Querier, query string) (time.Time, error) {
	var at sql.NullTime
	if err := q.QueryRowContext(ctx, query).Scan(&at); err != nil {
		return time.Time{}, apperr.Wrap(apperr.CodeQueryFailed, err, "read server start time")
	}
	return at.Time, nil
}

// databaseSizes lists every database with its size, connection count and commit
// count. pg_database_size() needs CONNECT on the database (or
// pg_read_all_stats); when that is missing the caller shows a warning instead.
func databaseSizes(ctx context.Context, q sqlbase.Querier) (*models.OverviewTable, error) {
	rows, err := q.QueryContext(ctx, `
SELECT d.datname,
       pg_size_pretty(pg_database_size(d.datname)),
       coalesce(s.numbackends, 0),
       coalesce(s.xact_commit, 0)
FROM pg_database d
LEFT JOIN pg_stat_database s ON s.datname = d.datname
WHERE d.datallowconn
ORDER BY pg_database_size(d.datname) DESC
LIMIT 25`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	table := &models.OverviewTable{
		Title:   "Databases",
		Columns: []string{"Database", "Size", "Backends", "Commits"},
		Rows:    [][]string{},
	}
	for rows.Next() {
		var (
			name     string
			size     string
			backends int64
			commits  int64
		)
		if err := rows.Scan(&name, &size, &backends, &commits); err != nil {
			return nil, err
		}
		table.Rows = append(table.Rows, []string{
			name, size, sqlbase.FormatCount(backends), sqlbase.FormatCount(commits),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(table.Rows) == 0 {
		return nil, nil
	}
	return table, nil
}

// runningQueries lists the backends that are not idle. It is deliberately
// capped: a hung server can have thousands of them and the page has to render.
func runningQueries(ctx context.Context, q sqlbase.Querier) (*models.OverviewTable, error) {
	rows, err := q.QueryContext(ctx, `
SELECT pid,
       coalesce(usename, ''),
       coalesce(datname, ''),
       coalesce(host(client_addr), 'local'),
       coalesce(state, ''),
       coalesce(wait_event_type, ''),
       coalesce(extract(epoch FROM (now() - query_start))::bigint, 0),
       left(replace(coalesce(query, ''), E'\n', ' '), 200)
FROM pg_stat_activity
WHERE state IS NOT NULL AND state <> 'idle'
ORDER BY query_start ASC NULLS LAST
LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	table := &models.OverviewTable{
		Title:   "Running sessions",
		Columns: []string{"PID", "User", "Database", "Client", "State", "Waiting on", "Elapsed", "Query"},
		Rows:    [][]string{},
	}
	for rows.Next() {
		var (
			pid     int64
			user    string
			db      string
			client  string
			state   string
			waiting string
			elapsed int64
			query   string
		)
		if err := rows.Scan(&pid, &user, &db, &client, &state, &waiting, &elapsed, &query); err != nil {
			return nil, err
		}
		table.Rows = append(table.Rows, []string{
			strconv.FormatInt(pid, 10), user, db, client, state, waiting,
			sqlbase.FormatDuration(float64(elapsed)), strings.TrimSpace(query),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(table.Rows) == 0 {
		return nil, nil
	}
	return table, nil
}

func withState(metric models.OverviewMetric, state string) models.OverviewMetric {
	metric.State = state
	return metric
}
