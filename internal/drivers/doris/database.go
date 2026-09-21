package doris

import (
	"context"

	"dbmanager/internal/drivers/sqlbase"
	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

// Creating a database in Doris
//
// Doris takes `CREATE DATABASE [IF NOT EXISTS] <name> [PROPERTIES (...)]` and
// nothing else: it has no database level character set and no collation, and
// SHOW CREATE DATABASE reports a bare `CREATE DATABASE \`demo\``.
//
// So unlike MySQL and TiDB — which share mysqlcompat's SHOW CHARACTER SET /
// SHOW COLLATION window — this engine offers the name and nothing more, and
// says why in the hint instead of showing a charset list the server would
// reject. (The PROPERTIES Doris does accept, replication_allocation and
// storage_vault_name, are cluster defaults rather than something to pick per
// database from this window.)

// DatabaseOptions implements the sqlbase spec hook.
func DatabaseOptions(context.Context, sqlbase.Querier) (*models.DatabaseOptions, error) {
	return &models.DatabaseOptions{
		Charsets: []models.DatabaseCharset{},
		Hint: "Doris has no database level character set or collation — the statement takes " +
			"the name and its own PROPERTIES (replica allocation, storage vault), which are " +
			"cluster defaults set once rather than per database.",
	}, nil
}

// CreateDatabase implements the sqlbase spec hook.
func CreateDatabase(req models.CreateDatabaseRequest) (models.DatabasePlan, error) {
	return models.DatabasePlan{Statement: "CREATE DATABASE " + sqlutil.QuoteBacktick(req.Name)}, nil
}
