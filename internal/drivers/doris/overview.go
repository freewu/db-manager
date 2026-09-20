package doris

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"dbmanager/internal/drivers/mysqlcompat"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

// overview implements the Spec.Overview hook.
//
// Doris answers the MySQL status variables, but thinner than MySQL does, and
// some builds do not answer SHOW GLOBAL STATUS at all. The shared page is
// therefore best-effort: when it cannot be read, the Doris sections below are
// still built and the page says what was missing.
func overview(ctx context.Context, q sqlbase.Querier, _ models.ConnectionConfig) (*models.ServerOverview, error) {
	page, err := mysqlcompat.Overview(ctx, q, mysqlcompat.OverviewOptions{InnoDB: false})
	if err != nil {
		page = &models.ServerOverview{MySQL: &models.MySQLOverview{Groups: []models.OverviewGroup{}}}
		page.Warnings = append(page.Warnings,
			"The MySQL-compatible status variables are not readable on this server: "+err.Error())
	}
	page.MySQL.Groups = append(page.MySQL.Groups, clusterGroup(ctx, q, page), storageGroup(ctx, q, page))
	return page, nil
}

// clusterGroup reports the front ends and back ends from SHOW FRONTENDS and
// SHOW BACKENDS.
//
// Both statements exist in every Doris release, but their columns differ
// between versions, so the rows are read by column name and anything missing is
// simply not reported. A failure is a note, not a failed page: a user who can
// query their tables should still see the status page.
func clusterGroup(ctx context.Context, q sqlbase.Querier, page *models.ServerOverview) models.OverviewGroup {
	group := models.OverviewGroup{
		Title: "Cluster",
		Note:  "SHOW FRONTENDS / SHOW BACKENDS",
	}

	frontends, err := mysqlcompat.ShowRows(ctx, q, "SHOW FRONTENDS")
	if err != nil {
		page.Warnings = append(page.Warnings, "SHOW FRONTENDS is not readable: "+err.Error())
	} else {
		// Join is false for a front end that has not joined the cluster yet.
		group.Metrics = append(group.Metrics,
			nodeMetrics("Front end", frontends, []string{"Alive", "Join"}, "Version")...)
	}
	backends, err := mysqlcompat.ShowRows(ctx, q, "SHOW BACKENDS")
	if err != nil {
		page.Warnings = append(page.Warnings, "SHOW BACKENDS is not readable: "+err.Error())
	} else {
		group.Metrics = append(group.Metrics, nodeMetrics("Back end", backends, []string{"Alive"}, "Version")...)
	}
	return group
}

// nodeMetrics counts the instances of one SHOW FRONTENDS/BACKENDS result and
// reports the versions it saw. statusKeys are the "true" flags that mean the
// node is up; a flag the server does not have is ignored rather than treated as
// a failure.
func nodeMetrics(label string, rows []mysqlcompat.ShowRow, statusKeys []string, versionKey string) []models.OverviewMetric {
	if len(rows) == 0 {
		return []models.OverviewMetric{
			sqlbase.Metric(label+" instances", "0", "none reported"),
		}
	}
	metrics := []models.OverviewMetric{
		sqlbase.Metric(label+" instances", fmt.Sprint(len(rows)), "rows returned by the SHOW statement"),
	}

	versions := map[string]int{}
	for _, row := range rows {
		if version := row.Get(versionKey); version != "" {
			versions[version]++
		}
	}
	if len(versions) > 0 {
		names := make([]string, 0, len(versions))
		for version := range versions {
			names = append(names, version)
		}
		sort.Strings(names)
		parts := make([]string, 0, len(names))
		for _, version := range names {
			parts = append(parts, fmt.Sprintf("%s ×%d", version, versions[version]))
		}
		metrics = append(metrics, sqlbase.Metric(label+" versions", strings.Join(parts, ", "), "reported by the SHOW statement"))
	}

	// The status flags read "true"/"false".
	down := 0
	for _, row := range rows {
		for _, key := range statusKeys {
			if value := strings.ToLower(row.Get(key)); value != "" && value != "true" {
				down++
				break
			}
		}
	}
	if down > 0 {
		metrics = append(metrics, mysqlcompat.WithState(
			sqlbase.Metric(label+"s not up", fmt.Sprint(down), "a node reports something other than true"), "warn"))
	}
	return metrics
}

// storageGroup adds up what the cluster stores, which is the number an analyst
// actually wants from a status page.
func storageGroup(ctx context.Context, q sqlbase.Querier, page *models.ServerOverview) models.OverviewGroup {
	const statement = `
SELECT COUNT(*), IFNULL(SUM(TABLE_ROWS), 0), IFNULL(SUM(DATA_LENGTH), 0), IFNULL(SUM(INDEX_LENGTH), 0)
FROM information_schema.TABLES`

	group := models.OverviewGroup{
		Title: "Storage",
		Note:  "information_schema.TABLES",
	}
	var tables, rows, data, index int64
	if err := q.QueryRowContext(ctx, statement).Scan(&tables, &rows, &data, &index); err != nil {
		page.Warnings = append(page.Warnings, "The table sizes are not readable: "+err.Error())
		return group
	}
	group.Metrics = []models.OverviewMetric{
		sqlbase.Metric("Tables", sqlbase.FormatCount(tables), "rows in information_schema.TABLES"),
		sqlbase.Metric("Rows", sqlbase.FormatCount(rows), "SUM(TABLE_ROWS) — Doris reports the stored row count"),
		sqlbase.Metric("Data size", sqlbase.FormatBytes(data), "SUM(DATA_LENGTH)"),
		sqlbase.Metric("Index size", sqlbase.FormatBytes(index), "SUM(INDEX_LENGTH)"),
	}
	return group
}
