package sqlbase

import (
	"context"
	"sort"
	"strings"
	"sync"

	"dbmanager/internal/models"
)

// maxGraphObjects caps how many objects one diagram call reads. The ER window
// is a reading aid, not a migration tool: a namespace with thousands of tables
// would be unreadable anyway, and this keeps a slip of the mouse from firing
// thousands of catalog queries.
const maxGraphObjects = 300

// graphWorkers bounds the concurrency of the per-object catalog reads. Four
// keeps MySQL/PostgreSQL busy without exhausting the pool (maxOpenConns = 8)
// while the UI may already be running queries on the same session.
const graphWorkers = 4

var _ interface {
	Graph(ctx context.Context, database, schema string) (*models.SchemaGraph, error)
} = (*Conn)(nil)

// Graph implements drivers.Grapher: it reads a namespace's objects plus the
// columns and foreign keys of each one, using the Introspector the driver
// already provides.
//
// Failures are per object: an object whose columns cannot be read still shows
// up in the diagram, flagged in Warnings, instead of failing the whole window.
func (c *Conn) Graph(ctx context.Context, database, schema string) (*models.SchemaGraph, error) {
	db, err := c.DB(ctx, database)
	if err != nil {
		return nil, err
	}
	resolved := c.resolveDatabase(database)
	objects, err := c.spec.Introspector.Objects(ctx, db, resolved, schema)
	if err != nil {
		return nil, err
	}

	graph := &models.SchemaGraph{
		Driver:   string(c.spec.Info.Type),
		Database: database,
		Schema:   schema,
		Nodes:    make([]models.GraphNode, 0, len(objects)),
		Edges:    []models.GraphEdge{},
		Warnings: []string{},
	}
	if len(objects) > maxGraphObjects {
		objects = objects[:maxGraphObjects]
		graph.Truncated = true
	}

	type detail struct {
		columns []models.ColumnInfo
		fks     []models.ForeignKeyInfo
		err     error
	}
	details := make([]detail, len(objects))

	// Bounded worker pool over the per-object reads.
	indexes := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < graphWorkers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range indexes {
				object := objects[i]
				var entry detail

				pool, err := c.DB(ctx, database)
				if err != nil {
					entry.err = err
					details[i] = entry
					continue
				}
				if entry.columns, entry.err = c.spec.Introspector.Columns(
					ctx, pool, resolved, schema, object.Name); entry.err != nil {
					details[i] = entry
					continue
				}
				// Views and sequences have no foreign keys of their own.
				if object.Kind == models.KindTable {
					entry.fks, entry.err = c.spec.Introspector.ForeignKeys(
						ctx, pool, resolved, schema, object.Name)
				}
				details[i] = entry
			}
		}()
	}
	for i := range objects {
		indexes <- i
	}
	close(indexes)
	wg.Wait()

	// The namespace's own object names, lower-cased, to tell an edge that
	// stays inside the diagram from one that leaves it.
	known := make(map[string]string, len(objects))
	for _, object := range objects {
		known[strings.ToLower(object.Name)] = object.Name
	}

	for i, object := range objects {
		node := models.GraphNode{
			Name:    object.Name,
			Kind:    object.Kind,
			Comment: object.Comment,
			Columns: make([]models.GraphColumn, 0, len(details[i].columns)),
		}
		for _, column := range details[i].columns {
			node.Columns = append(node.Columns, models.GraphColumn{
				Name:       column.Name,
				Type:       column.DataType,
				Nullable:   column.Nullable,
				PrimaryKey: column.PrimaryKey,
			})
		}
		graph.Nodes = append(graph.Nodes, node)

		if details[i].err != nil {
			graph.Warnings = append(graph.Warnings, object.Name+": "+details[i].err.Error())
			continue
		}
		for _, fk := range details[i].fks {
			target := fk.ReferencedTable
			if fk.ReferencedSchema != "" && fk.ReferencedSchema != schema {
				target = fk.ReferencedSchema + "." + target
			}
			graph.Edges = append(graph.Edges, models.GraphEdge{
				From:       object.Name,
				FromColumn: fk.Columns,
				// Keep the catalog's spelling for tables inside the diagram so
				// the UI can bind the edge to a node.
				To:       matchName(known, fk.ReferencedTable, target),
				ToColumn: fk.ReferencedColumns,
				Name:     fk.Name,
				OnDelete: fk.OnDelete,
				OnUpdate: fk.OnUpdate,
			})
		}
	}

	sort.Strings(graph.Warnings)
	return graph, nil
}

// matchName returns the diagram's spelling of a referenced table when it is
// part of the namespace, and the catalog's (possibly qualified) name otherwise.
func matchName(known map[string]string, table, fallback string) string {
	if name, ok := known[strings.ToLower(table)]; ok {
		return name
	}
	return fallback
}
