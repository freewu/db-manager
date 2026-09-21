package all

import (
	"testing"

	"dbmanager/internal/drivers"
	"dbmanager/internal/drivers/planned"
	"dbmanager/internal/models"
)

// The registry is what the connection dialog draws, so this pins the two
// things the UI cannot check for itself: which engines are offered, and which
// of them are document stores rather than SQL engines.

func TestRegistryListsImplementedDriversInMenuOrder(t *testing.T) {
	infos := drivers.Infos()

	order := make([]models.DriverType, 0, len(infos))
	byType := map[models.DriverType]models.DriverInfo{}
	for _, info := range infos {
		order = append(order, info.Type)
		byType[info.Type] = info
		if !info.Implemented {
			t.Errorf("%s is listed without an implementation", info.Type)
		}
	}

	want := []models.DriverType{
		models.DriverMySQL,
		models.DriverPostgres,
		models.DriverSQLite,
		models.DriverMongoDB,
		models.DriverTiDB,
		models.DriverDoris,
	}
	if len(order) != len(want) {
		t.Fatalf("drivers = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("drivers = %v, want %v", order, want)
		}
	}

	// MongoDB is the first non-relational engine: the UI picks what to show
	// from this flag, so it must not claim to have tables or a SQL designer.
	// Databases it does have, and its "New database" window renders `use`.
	mongo := byType[models.DriverMongoDB]
	if mongo.Relational || mongo.SupportsSchema || mongo.RequiresFile {
		t.Errorf("mongodb advertised as a relational engine: %+v", mongo)
	}
	if !mongo.SupportsDatabase || mongo.DefaultDatabase == "" {
		t.Errorf("mongodb must offer its databases: %+v", mongo)
	}

	// The designer is offered per engine, not per protocol: Doris speaks MySQL
	// but cannot be planned by the MySQL designer, so the flag has to travel
	// with the driver info.
	designable := map[models.DriverType]bool{}
	for _, info := range infos {
		designable[info.Type] = info.SupportsDesign
		if info.SupportsDesign && !info.Relational {
			t.Errorf("%s offers a table designer without being relational", info.Type)
		}
	}
	for _, engine := range []models.DriverType{
		models.DriverMySQL, models.DriverPostgres, models.DriverSQLite, models.DriverTiDB,
	} {
		if !designable[engine] {
			t.Errorf("%s should offer the table designer", engine)
		}
	}
	for _, no := range []models.DriverType{models.DriverMongoDB, models.DriverDoris} {
		if designable[no] {
			t.Errorf("%s should not offer the table designer", no)
		}
	}
}

// Oracle and SQL Server were advertised before and are parked: they must not
// show up in the menu, but their research notes stay behind.
func TestParkedEnginesStayOutOfTheMenu(t *testing.T) {
	infos := drivers.Infos()
	for _, parked := range planned.Parked() {
		for _, info := range infos {
			if info.Type == parked.Type {
				t.Fatalf("%s is parked but still advertised", parked.Type)
			}
		}
		if parked.Notes == "" {
			t.Errorf("%s is parked without a note explaining why", parked.Type)
		}
	}
	if extra := planned.Infos(); len(extra) != 0 {
		t.Fatalf("planned.Infos() = %+v, want no roadmap entries while everything is either done or parked", extra)
	}
}
