// Package all blank-imports every concrete driver so that a single import in
// main pulls in the whole registry.
//
// Keeping the wiring here (instead of in main) means adding a driver is a
// one-line change and that tests can import "all" to get a fully populated
// registry.
package all

import (
	// SQL drivers (phase 1).
	_ "dbmanager/internal/drivers/mysql"
	_ "dbmanager/internal/drivers/postgres"
	_ "dbmanager/internal/drivers/sqlite"
	// Phase 2 drivers are added here:
	//   _ "dbmanager/internal/drivers/mongodb"
	//   _ "dbmanager/internal/drivers/oracle"
	//   _ "dbmanager/internal/drivers/sqlserver"
)
