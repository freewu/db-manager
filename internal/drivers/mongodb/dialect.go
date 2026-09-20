package mongodb

import "dbmanager/internal/models"

// Dialect exists because drivers.Conn requires one, not because MongoDB has a
// SQL flavour.
//
// Nothing in this driver builds SQL with it, and the service's SQL-only code
// paths (the table designer, the script dry run, the ER fallback) are not
// reachable for a document store: the UI hides them and the collection-level
// operations all go through Fetch/Execute. Qualify still renders the shell's
// db.collection reference so log lines and error messages stay meaningful.
type Dialect struct{}

// Name implements drivers.Dialect.
func (Dialect) Name() models.DriverType { return models.DriverMongoDB }

// Quote implements drivers.Dialect. Field names are used as written: MongoDB
// has no identifier quoting, and dotted paths ("address.city") are meaningful.
func (Dialect) Quote(ident string) string { return ident }

// Qualify implements drivers.Dialect.
func (Dialect) Qualify(database, schema, object string) string {
	if database == "" {
		return object
	}
	return database + "." + object
}

// Placeholder implements drivers.Dialect.
func (Dialect) Placeholder(int) string { return "?" }

// LimitOffset implements drivers.Dialect: paging is expressed with skip/limit
// in the shell language, not as a SQL suffix.
func (Dialect) LimitOffset(int, int) string { return "" }

// SupportsLimitOffset implements drivers.Dialect.
func (Dialect) SupportsLimitOffset() bool { return false }
