package tidb

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"dbmanager/internal/drivers/mysqlcompat"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

// overview implements the Spec.Overview hook: the MySQL-compatible status page
// plus the cluster section TiDB can answer and no MySQL server can.
func overview(ctx context.Context, q sqlbase.Querier, _ models.ConnectionConfig) (*models.ServerOverview, error) {
	// Without InnoDB: TiDB stores rows in TiKV, so the buffer pool counters are
	// zeros, not measurements.
	page, err := mysqlcompat.Overview(ctx, q, mysqlcompat.OverviewOptions{InnoDB: false})
	if err != nil {
		return nil, err
	}
	page.MySQL.Groups = append(page.MySQL.Groups, clusterGroup(ctx, q, page))
	return page, nil
}

// kindLabel spells an instance type the way the project does. CLUSTER_INFO
// reports the types in lower case ("tikv"), which would end up as "TIKV" in a
// label; the components have their own capitalisation.
func kindLabel(kind string) string {
	switch kind {
	case "tidb":
		return "TiDB"
	case "tikv":
		return "TiKV"
	case "tiflash":
		return "TiFlash"
	case "ticdc":
		return "TiCDC"
	case "pd":
		return "PD"
	default:
		return strings.ToUpper(kind)
	}
}

// clusterGroup reports the topology from information_schema.CLUSTER_INFO.
//
// The table is the one place TiDB names every TiDB, TiKV and PD instance with
// its version and status, but reading it needs the PROCESS privilege and older
// releases do not have it at all — so a failure is a note on the page, never a
// failed status page.
func clusterGroup(ctx context.Context, q sqlbase.Querier, page *models.ServerOverview) models.OverviewGroup {
	const statement = `SELECT TYPE, INSTANCE, STATUS, VERSION FROM information_schema.CLUSTER_INFO`
	group := models.OverviewGroup{
		Title: "Cluster",
		Note:  "information_schema.CLUSTER_INFO",
	}

	rows, err := mysqlcompat.ShowRows(ctx, q, statement)
	if err != nil {
		page.Warnings = append(page.Warnings,
			"Reading information_schema.CLUSTER_INFO needs the PROCESS privilege: "+err.Error())
		return group
	}

	instances := map[string]int{}
	versions := map[string]string{}
	down := []string{}
	for _, row := range rows {
		kind := strings.ToLower(row.Get("type"))
		if kind == "" {
			continue
		}
		instances[kind]++
		if versions[kind] == "" {
			versions[kind] = row.Get("version")
		}
		if status := row.Get("status"); status != "" && !strings.EqualFold(status, "up") {
			down = append(down, fmt.Sprintf("%s (%s)", row.Get("instance"), status))
		}
	}
	if len(instances) == 0 {
		group.Note = "information_schema.CLUSTER_INFO returned no instances"
		return group
	}

	kinds := make([]string, 0, len(instances))
	for kind := range instances {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		hint := kind + " instances reporting into CLUSTER_INFO"
		if version := versions[kind]; version != "" {
			hint = kind + " " + version
		}
		group.Metrics = append(group.Metrics, sqlbase.Metric(
			kindLabel(kind)+" instances", fmt.Sprint(instances[kind]), hint))
	}
	if len(down) > 0 {
		sort.Strings(down)
		group.Metrics = append(group.Metrics, mysqlcompat.WithState(sqlbase.Metric(
			"Not Up", fmt.Sprint(len(down)), strings.Join(down, ", ")), "warn"))
		group.Note = "At least one instance reports a status other than Up"
	}
	return group
}
