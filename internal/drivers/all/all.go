// Package all blank-imports every concrete driver so that a single import in
// main pulls in the whole registry.
//
// Keeping the wiring here (instead of in main) means adding a driver is a
// one-line change and that tests can import "all" to get a fully populated
// registry.
package all

import (
	// SQL drivers.
	_ "dbmanager/internal/drivers/mysql"
	_ "dbmanager/internal/drivers/postgres"
	_ "dbmanager/internal/drivers/sqlite"
	// Document store: it shares the Conn contract but not sqlbase, so
	// everything below the interface is its own code (see the package docs).
	_ "dbmanager/internal/drivers/mongodb"
	// Still to come, listed in internal/drivers/planned: Oracle, SQL Server.
)
