package sqlite

import (
	"context"
	"testing"

	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// The ER diagram is fed by drivers.Grapher. SQLite must satisfy it through the
// shared sqlbase implementation, which is the same path MySQL and PostgreSQL
// take — if this drifts, the diagram goes blank for every engine at once.
func TestGraphDescribesForeignKeys(t *testing.T) {
	ctx := context.Background()
	conn := newConn(t)

	exec(t, conn, `
CREATE TABLE authors (
	id   INTEGER PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE books (
	id        INTEGER PRIMARY KEY,
	author_id INTEGER NOT NULL REFERENCES authors (id) ON DELETE CASCADE,
	title     TEXT
);
CREATE VIEW recent_books AS SELECT id, title FROM books;`)

	grapher, ok := conn.(drivers.Grapher)
	if !ok {
		t.Fatal("the sqlite connection must implement drivers.Grapher")
	}

	graph, err := grapher.Graph(ctx, "main", "main")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if graph.Driver != string(models.DriverSQLite) || graph.Database != "main" {
		t.Fatalf("unexpected graph header: %+v", graph)
	}
	if graph.Truncated {
		t.Fatal("a three-object namespace cannot be truncated")
	}
	if len(graph.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", graph.Warnings)
	}

	byName := map[string]models.GraphNode{}
	for _, node := range graph.Nodes {
		byName[node.Name] = node
	}
	if len(byName) != 3 {
		t.Fatalf("expected authors, books and the view, got %+v", graph.Nodes)
	}
	if byName["books"].Kind != models.KindTable {
		t.Fatalf("books should be a table, got %q", byName["books"].Kind)
	}
	if byName["recent_books"].Kind != models.KindView {
		t.Fatalf("recent_books should be a view, got %q", byName["recent_books"].Kind)
	}

	// Columns travel with the node so the diagram can render them offline.
	var sawPrimaryKey, sawNullable bool
	for _, column := range byName["books"].Columns {
		switch column.Name {
		case "id":
			sawPrimaryKey = column.PrimaryKey
		case "title":
			sawNullable = column.Nullable
		}
	}
	if !sawPrimaryKey {
		t.Fatal("the primary key flag must survive into the graph")
	}
	if !sawNullable {
		t.Fatal("the nullability flag must survive into the graph")
	}

	if len(graph.Edges) != 1 {
		t.Fatalf("expected exactly the books -> authors edge, got %+v", graph.Edges)
	}
	edge := graph.Edges[0]
	if edge.From != "books" || edge.To != "authors" {
		t.Fatalf("unexpected edge direction: %+v", edge)
	}
	if len(edge.FromColumn) != 1 || edge.FromColumn[0] != "author_id" {
		t.Fatalf("unexpected referencing columns: %+v", edge.FromColumn)
	}
	if len(edge.ToColumn) != 1 || edge.ToColumn[0] != "id" {
		t.Fatalf("unexpected referenced columns: %+v", edge.ToColumn)
	}
	if edge.OnDelete == "" {
		t.Fatal("ON DELETE should be reported when the engine exposes it")
	}
}

func TestGraphOnAnEmptyDatabase(t *testing.T) {
	grapher, ok := newConn(t).(drivers.Grapher)
	if !ok {
		t.Fatal("the sqlite connection must implement drivers.Grapher")
	}

	result, err := grapher.Graph(context.Background(), "main", "main")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if len(result.Nodes) != 0 || len(result.Edges) != 0 {
		t.Fatalf("an empty database has no diagram, got %+v", result)
	}
	// The slices must serialise as arrays so the UI can map over them.
	if result.Nodes == nil || result.Edges == nil || result.Warnings == nil {
		t.Fatal("nodes, edges and warnings must never be null")
	}
}
