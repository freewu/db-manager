// Package mysqlcompat holds what every engine that speaks the MySQL wire
// protocol has in common: the DSN builder (TLS registry included), the catalog
// queries behind information_schema, and the SHOW-based helpers the status
// pages are written with.
//
// MySQL, TiDB and Doris all answer those queries — TiDB byte for byte, Doris
// for most of them — so the package exists to keep one copy of the parts that
// are identical instead of three that drift apart. It is not a driver: the thin
// packages in internal/drivers/{mysql,tidb,doris} each declare their own
// DriverInfo, DSN options and overrides, and register themselves.
//
// To build an engine on top of it: embed Introspector, override the methods
// whose catalog queries differ, and pass the whole thing as
// sqlbase.Spec.Introspector.
package mysqlcompat

import "strings"

// mysqlSystemSchemas are the schemas a MySQL server ships with. They are hidden
// from the explorer as long as the server has something else to show.
var mysqlSystemSchemas = map[string]bool{
	"information_schema": true,
	"performance_schema": true,
	"mysql":              true,
	"sys":                true,
}

// FilterDatabases hides the schemas this engine ships with — they are noise
// once it has real databases. A brand new server has nothing else though, and
// an explorer with no children reads as a broken connection, so in that case
// the system schemas are kept (they are still browsable, and the alternative is
// an empty tree with nothing to click).
func (i Introspector) FilterDatabases(all []string) []string {
	system := i.SystemSchemas
	if system == nil {
		system = mysqlSystemSchemas
	}
	out := make([]string, 0, len(all))
	for _, name := range all {
		if system[strings.ToLower(name)] {
			continue
		}
		out = append(out, name)
	}
	if len(out) == 0 && len(all) > 0 {
		return all
	}
	return out
}

// ExplainSQL wraps a statement in the EXPLAIN this family understands.
//
// MySQL, TiDB and Doris all answer a plain `EXPLAIN <statement>` — TiDB with
// its own operator tree, Doris with its plan-format rows — and none of them
// runs the statement to produce it. MariaDB also accepts `ANALYZE`, which does
// run the statement; that form is deliberately not used here.
func ExplainSQL(sql string) string { return "EXPLAIN " + sql }
