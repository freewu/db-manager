package sqlbase

import (
	"fmt"
	"strconv"

	"dbmanager/internal/drivers/sqlutil"
	"dbmanager/internal/models"
)

// --- MySQL / MariaDB -------------------------------------------------------

// MySQLDialect implements drivers.Dialect for MySQL and MariaDB.
//
// In MySQL a "schema" *is* a database, so schema segments are ignored and
// cross database references work by qualifying with the database name. The
// engines that speak the same wire protocol and the same identifier rules
// (TiDB, Doris) reuse this dialect and only change the name it reports, which
// is what tells the generated DDL where it is going.
type MySQLDialect struct {
	// Driver is the engine the statements are for; empty means MySQL.
	Driver models.DriverType
}

func (d MySQLDialect) Name() models.DriverType {
	if d.Driver == "" {
		return models.DriverMySQL
	}
	return d.Driver
}
func (MySQLDialect) Quote(ident string) string { return sqlutil.QuoteBacktick(ident) }
func (MySQLDialect) Qualify(database, schema, object string) string {
	return sqlutil.Qualify(sqlutil.QuoteBacktick, database, object)
}
func (MySQLDialect) Placeholder(int) string    { return "?" }
func (MySQLDialect) SupportsLimitOffset() bool { return true }
func (MySQLDialect) LimitOffset(limit, offset int) string {
	return " LIMIT " + strconv.Itoa(limit) + " OFFSET " + strconv.Itoa(offset)
}

// --- PostgreSQL ------------------------------------------------------------

// PostgresDialect implements drivers.Dialect for PostgreSQL.
//
// PostgreSQL cannot span databases in one connection, so the database segment
// is dropped: the pool is already bound to the right catalog (see Conn.DB).
type PostgresDialect struct{}

func (PostgresDialect) Name() models.DriverType   { return models.DriverPostgres }
func (PostgresDialect) Quote(ident string) string { return sqlutil.QuoteDouble(ident) }
func (PostgresDialect) Qualify(database, schema, object string) string {
	if schema == "" {
		schema = "public"
	}
	return sqlutil.Qualify(sqlutil.QuoteDouble, schema, object)
}
func (PostgresDialect) Placeholder(n int) string  { return "$" + strconv.Itoa(n) }
func (PostgresDialect) SupportsLimitOffset() bool { return true }
func (PostgresDialect) LimitOffset(limit, offset int) string {
	return " LIMIT " + strconv.Itoa(limit) + " OFFSET " + strconv.Itoa(offset)
}

// --- SQLite ----------------------------------------------------------------

// SQLiteDialect implements drivers.Dialect for SQLite.
type SQLiteDialect struct{}

func (SQLiteDialect) Name() models.DriverType   { return models.DriverSQLite }
func (SQLiteDialect) Quote(ident string) string { return sqlutil.QuoteDouble(ident) }
func (SQLiteDialect) Qualify(database, schema, object string) string {
	// Attached databases are referenced as main.table; the synthetic
	// "main" database is the default and can be omitted.
	if database == "" || database == "main" {
		return sqlutil.QuoteDouble(object)
	}
	return sqlutil.Qualify(sqlutil.QuoteDouble, database, object)
}
func (SQLiteDialect) Placeholder(int) string    { return "?" }
func (SQLiteDialect) SupportsLimitOffset() bool { return true }
func (SQLiteDialect) LimitOffset(limit, offset int) string {
	if offset <= 0 {
		return " LIMIT " + strconv.Itoa(limit)
	}
	return " LIMIT " + strconv.Itoa(limit) + " OFFSET " + strconv.Itoa(offset)
}

// --- SQL Server (phase 2) --------------------------------------------------

// SQLServerDialect is provided now so the future driver only needs catalog
// queries. SQL Server needs ORDER BY before OFFSET/FETCH.
type SQLServerDialect struct{}

func (SQLServerDialect) Name() models.DriverType   { return models.DriverSQLServer }
func (SQLServerDialect) Quote(ident string) string { return sqlutil.QuoteBracket(ident) }
func (SQLServerDialect) Qualify(database, schema, object string) string {
	return sqlutil.Qualify(sqlutil.QuoteBracket, database, schema, object)
}
func (SQLServerDialect) Placeholder(n int) string  { return "@p" + strconv.Itoa(n) }
func (SQLServerDialect) SupportsLimitOffset() bool { return true }
func (SQLServerDialect) LimitOffset(limit, offset int) string {
	return fmt.Sprintf(" OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, limit)
}

// --- Oracle (phase 2) ------------------------------------------------------

// OracleDialect targets Oracle 12c+ which supports OFFSET/FETCH.
type OracleDialect struct{}

func (OracleDialect) Name() models.DriverType   { return models.DriverOracle }
func (OracleDialect) Quote(ident string) string { return sqlutil.QuoteDouble(ident) }
func (OracleDialect) Qualify(database, schema, object string) string {
	// Oracle has no cross-database references; the schema (user) is the
	// namespace.
	return sqlutil.Qualify(sqlutil.QuoteDouble, schema, object)
}
func (OracleDialect) Placeholder(n int) string  { return ":" + strconv.Itoa(n) }
func (OracleDialect) SupportsLimitOffset() bool { return true }
func (OracleDialect) LimitOffset(limit, offset int) string {
	return fmt.Sprintf(" OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, limit)
}
