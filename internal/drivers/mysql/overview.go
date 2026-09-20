package mysql

import (
	"context"

	"dbmanager/internal/drivers/mysqlcompat"
	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/models"
)

// overview implements the Spec.Overview hook: the MySQL side of the runtime
// status page.
//
// MySQL is the engine the shared page was written for, so this is the shared
// implementation with InnoDB reporting switched on.
func overview(ctx context.Context, q sqlbase.Querier, _ models.ConnectionConfig) (*models.ServerOverview, error) {
	return mysqlcompat.Overview(ctx, q, mysqlcompat.OverviewOptions{InnoDB: true})
}
