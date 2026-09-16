// Package planned advertises the drivers that are on the roadmap but not yet
// implemented.
//
// They are surfaced to the UI so the connection dialog can show the full
// target matrix (greyed out with a roadmap note) instead of hiding options
// users are actively waiting for. Adding a real driver means registering it in
// its own package and deleting the entry here.
package planned

import "dbmanager/internal/models"

// Infos returns the roadmap entries.
func Infos() []models.DriverInfo {
	return []models.DriverInfo{
		{
			Type:             models.DriverMongoDB,
			DisplayName:      "MongoDB",
			DefaultPort:      27017,
			Implemented:      false,
			Relational:       false,
			SupportsDatabase: true,
			SupportsSchema:   false,
			DefaultDatabase:  "admin",
			SortOrder:        40,
			Notes:            "Document store: databases become catalogs and collections become objects. Needs a query translation layer.",
		},
		{
			Type:             models.DriverOracle,
			DisplayName:      "Oracle",
			DefaultPort:      1521,
			Implemented:      false,
			Relational:       true,
			SupportsDatabase: false,
			SupportsSchema:   true,
			SortOrder:        50,
			DefaultDatabase:  "ORCL",
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
