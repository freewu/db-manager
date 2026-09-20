// Package planned advertises the drivers that are on the roadmap but not yet
// implemented.
//
// They are surfaced to the UI so the connection dialog can show the full
// target matrix (greyed out with a roadmap note) instead of hiding options
// users are actively waiting for. Adding a real driver means registering it in
// its own package and deleting the entry here.
package planned

import "dbmanager/internal/models"

// Infos returns the roadmap entries. Only drivers that are being worked on are
// listed: Oracle and SQL Server are parked (see Parked) and hidden from the UI
// until that changes, so no half-promise shows up in the connection dialog.
func Infos() []models.DriverInfo {
	return []models.DriverInfo{}
}

// Parked documents the engines that were advertised before and are now
// hidden again.
//
// Nothing reads this at runtime; it exists so the next person does not have to
// rediscover what has already been researched. Restoring one means moving its
// entry back into Infos and deleting it here.
func Parked() []models.DriverInfo {
	return []models.DriverInfo{
		{
			Type:             models.DriverOracle,
			DisplayName:      "Oracle",
			DefaultPort:      1521,
			Implemented:      false,
			Relational:       true,
			SupportsDatabase: false,
			SupportsSchema:   true,
			DefaultDatabase:  "ORCL",
			SortOrder:        50,
			Notes:            "Planned on github.com/sijms/go-ora (pure Go, no Instant Client). Dialect already supports OFFSET/FETCH.",
		},
		{
			Type:             models.DriverSQLServer,
			DisplayName:      "SQL Server",
			DefaultPort:      1433,
			Implemented:      false,
			Relational:       true,
			SupportsDatabase: true,
			SupportsSchema:   true,
			DefaultDatabase:  "master",
			SortOrder:        60,
			Notes:            "Planned on github.com/microsoft/go-mssqldb. Dialect already emits OFFSET/FETCH and [bracketed] identifiers.",
		},
	}
}
