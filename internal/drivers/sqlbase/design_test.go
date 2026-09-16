package sqlbase

import (
	"strings"
	"testing"

	"dbmanager/internal/drivers"
	"dbmanager/internal/models"
)

// The designer is the only place in the app that writes DDL the user did not
// type by hand, so the rendering is pinned down by tests for every engine.

func strptr(value string) *string { return &value }

func sampleStructure() *models.TableStructure {
	return &models.TableStructure{
		Object: models.ObjectInfo{
			Name:     "orders",
			Database: "shop",
			Schema:   "public",
			Kind:     models.KindTable,
		},
		Columns: []models.ColumnInfo{
			{Name: "id", DataType: "int", ColumnType: "int", PrimaryKey: true},
			{Name: "user_id", DataType: "int", ColumnType: "int", Nullable: true},
			{Name: "placed_at", DataType: "timestamp", ColumnType: "timestamp", Nullable: true},
		},
		Indexes: []models.IndexInfo{
			{Name: "PRIMARY", Columns: []string{"id"}, Unique: true, Primary: true},
			{Name: "idx_orders_user", Columns: []string{"user_id"}, Unique: false},
			{Name: "idx_orders_placed", Columns: []string{"placed_at"}, Unique: false},
		},
		ForeignKeys: []models.ForeignKeyInfo{
			{Name: "fk_orders_user", Columns: []string{"user_id"}, ReferencedTable: "users", ReferencedColumns: []string{"id"}},
		},
	}
}

func numbers(t *testing.T, dialect drivers.Dialect, want models.TableDesign) []string {
	t.Helper()
	plan, err := PlanAlter(dialect, sampleStructure(), want)
	if err != nil {
		t.Fatalf("PlanAlter: %v", err)
	}
	return plan.Statements
}

func assertStatements(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d statement(s), got %d:\n%s", len(want), len(got), strings.Join(got, "\n"))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("statement %d:\n got: %s\nwant: %s", i+1, got[i], want[i])
		}
	}
}

func TestPlanAlterMySQL(t *testing.T) {
	want := models.TableDesign{
		Object: "orders",
		Columns: []models.DesignColumn{
			{Name: "id", OriginalName: "id", DataType: "int", PrimaryKey: true},
			{Name: "customer_id", OriginalName: "user_id", DataType: "bigint", Nullable: false},
			{Name: "placed_at", OriginalName: "placed_at", DataType: "timestamp", Nullable: true},
			{Name: "total", DataType: "decimal(12,2)", Nullable: false, DefaultValue: strptr("0"), Comment: "order total"},
		},
		Indexes: []models.DesignIndex{
			{Name: "idx_orders_customer", OriginalName: "idx_orders_user", Columns: []string{"customer_id"}},
		},
	}

	assertStatements(t, numbers(t, MySQLDialect{}, want), []string{
		"ALTER TABLE `shop`.`orders` DROP INDEX `idx_orders_placed`",
		"ALTER TABLE `shop`.`orders` ADD COLUMN `total` decimal(12,2) NOT NULL DEFAULT 0 COMMENT 'order total'",
		"ALTER TABLE `shop`.`orders` CHANGE COLUMN `user_id` `customer_id` bigint NOT NULL",
		// The index follows its renamed column, and only the name changed, so
		// MySQL can rename it in place.
		"ALTER TABLE `shop`.`orders` RENAME INDEX `idx_orders_user` TO `idx_orders_customer`",
	})
}

func TestPlanAlterMySQLPrimaryKeySwapAndRenameIndex(t *testing.T) {
	want := models.TableDesign{
		Object: "orders",
		Columns: []models.DesignColumn{
			{Name: "id", OriginalName: "id", DataType: "int"},
			{Name: "user_id", OriginalName: "user_id", DataType: "int", PrimaryKey: true},
			{Name: "placed_at", OriginalName: "placed_at", DataType: "timestamp", Nullable: true},
		},
		Indexes: []models.DesignIndex{
			{Name: "idx_orders_customer", OriginalName: "idx_orders_user", Columns: []string{"user_id"}},
			{Name: "idx_orders_placed", OriginalName: "idx_orders_placed", Columns: []string{"placed_at"}},
		},
	}

	assertStatements(t, numbers(t, MySQLDialect{}, want), []string{
		"ALTER TABLE `shop`.`orders` DROP PRIMARY KEY",
		"ALTER TABLE `shop`.`orders` CHANGE COLUMN `user_id` `user_id` int NOT NULL",
		"ALTER TABLE `shop`.`orders` ADD PRIMARY KEY (`user_id`)",
		"ALTER TABLE `shop`.`orders` RENAME INDEX `idx_orders_user` TO `idx_orders_customer`",
	})
}

func TestPlanAlterFollowsARenamedColumnInItsIndexes(t *testing.T) {
	// The designer names the field by its new name but leaves the index alone:
	// the index follows the rename, exactly like the engine does, so no index
	// statement is rendered at all.
	want := models.TableDesign{
		Object: "orders",
		Columns: []models.DesignColumn{
			{Name: "id", OriginalName: "id", DataType: "int", PrimaryKey: true},
			{Name: "customer_id", OriginalName: "user_id", DataType: "int"},
			{Name: "placed_at", OriginalName: "placed_at", DataType: "timestamp", Nullable: true},
		},
		Indexes: []models.DesignIndex{
			{Name: "idx_orders_user", OriginalName: "idx_orders_user", Columns: []string{"user_id"}},
			{Name: "idx_orders_placed", OriginalName: "idx_orders_placed", Columns: []string{"placed_at"}},
		},
	}

	assertStatements(t, numbers(t, MySQLDialect{}, want), []string{
		"ALTER TABLE `shop`.`orders` CHANGE COLUMN `user_id` `customer_id` int NOT NULL",
	})
	assertStatements(t, numbers(t, SQLiteDialect{}, want), []string{
		`ALTER TABLE "shop"."orders" RENAME COLUMN "user_id" TO "customer_id"`,
	})

	// A field that is not renamed keeps the strict check: an index column that
	// names nothing at all is still a mistake.
	broken := want
	broken.Indexes = append([]models.DesignIndex{}, want.Indexes...)
	broken.Indexes[0].Columns = []string{"nope"}
	if _, err := PlanAlter(MySQLDialect{}, sampleStructure(), broken); err == nil {
		t.Fatal("expected an error for an index over an unknown field")
	}
}

func TestPlanAlterMySQLDropsIndexWithItsColumn(t *testing.T) {
	want := models.TableDesign{
		Object: "orders",
		Columns: []models.DesignColumn{
			{Name: "id", OriginalName: "id", DataType: "int", PrimaryKey: true},
			{Name: "user_id", OriginalName: "user_id", DataType: "int", Nullable: true},
		},
		Indexes: []models.DesignIndex{
			{Name: "idx_orders_user", OriginalName: "idx_orders_user", Columns: []string{"user_id"}},
		},
	}

	plan, err := PlanAlter(MySQLDialect{}, sampleStructure(), want)
	if err != nil {
		t.Fatalf("PlanAlter: %v", err)
	}
	assertStatements(t, plan.Statements, []string{
		"ALTER TABLE `shop`.`orders` DROP INDEX `idx_orders_placed`",
		"ALTER TABLE `shop`.`orders` DROP COLUMN `placed_at`",
	})
	if !plan.Destructive {
		t.Error("dropping a column should mark the plan as destructive")
	}
	joined := strings.Join(plan.Warnings, " | ")
	if !strings.Contains(joined, "idx_orders_placed is dropped together with field placed_at") {
		t.Errorf("expected a warning about the index that follows its column, got %q", joined)
	}
}

func TestPlanAlterWarnsAboutForeignKeyColumns(t *testing.T) {
	want := models.TableDesign{
		Object: "orders",
		Columns: []models.DesignColumn{
			{Name: "id", OriginalName: "id", DataType: "int", PrimaryKey: true},
			{Name: "placed_at", OriginalName: "placed_at", DataType: "timestamp", Nullable: true},
		},
	}

	plan, err := PlanAlter(MySQLDialect{}, sampleStructure(), want)
	if err != nil {
		t.Fatalf("PlanAlter: %v", err)
	}
	joined := strings.Join(plan.Warnings, " | ")
	if !strings.Contains(joined, "field user_id is used by foreign key fk_orders_user") {
		t.Errorf("expected a warning about the foreign key on user_id, got %q", joined)
	}
}

func TestPlanAlterPostgres(t *testing.T) {
	want := models.TableDesign{
		Object: "orders",
		Columns: []models.DesignColumn{
			{Name: "id", OriginalName: "id", DataType: "int", PrimaryKey: true},
			{Name: "user_id", OriginalName: "user_id", DataType: "int", Nullable: false},
			{Name: "created_at", OriginalName: "placed_at", DataType: "timestamptz", Nullable: false, DefaultValue: strptr("now()"), Comment: "when the order landed"},
			{Name: "note", DataType: "text", Nullable: true},
		},
		Indexes: []models.DesignIndex{
			{Name: "idx_orders_user", OriginalName: "idx_orders_user", Columns: []string{"user_id"}},
			{Name: "idx_orders_created", OriginalName: "idx_orders_placed", Columns: []string{"created_at"}, Unique: true},
		},
	}

	plan, err := PlanAlter(PostgresDialect{}, sampleStructure(), want)
	if err != nil {
		t.Fatalf("PlanAlter: %v", err)
	}

	assertStatements(t, plan.Statements, []string{
		`DROP INDEX "public"."idx_orders_placed"`,
		`ALTER TABLE "public"."orders" ADD COLUMN "note" text`,
		`ALTER TABLE "public"."orders" ALTER COLUMN "user_id" SET NOT NULL`,
		`ALTER TABLE "public"."orders" RENAME COLUMN "placed_at" TO "created_at"`,
		`ALTER TABLE "public"."orders" ALTER COLUMN "created_at" TYPE timestamptz`,
		`ALTER TABLE "public"."orders" ALTER COLUMN "created_at" SET NOT NULL`,
		`ALTER TABLE "public"."orders" ALTER COLUMN "created_at" SET DEFAULT now()`,
		`COMMENT ON COLUMN "public"."orders"."created_at" IS 'when the order landed'`,
		`CREATE UNIQUE INDEX "public"."idx_orders_created" ON "public"."orders" ("created_at")`,
	})

	joined := strings.Join(plan.Warnings, " | ")
	if !strings.Contains(joined, "needs a USING clause") {
		t.Errorf("expected a warning about the type change, got %q", joined)
	}
}

func TestPlanAlterPostgresDropsPrimaryKeyConstraint(t *testing.T) {
	structure := sampleStructure()
	structure.Indexes[0] = models.IndexInfo{Name: "orders_pkey", Columns: []string{"id"}, Unique: true, Primary: true}

	want := models.TableDesign{
		Object: "orders",
		Columns: []models.DesignColumn{
			{Name: "id", OriginalName: "id", DataType: "int"},
			{Name: "user_id", OriginalName: "user_id", DataType: "int", Nullable: true},
			{Name: "placed_at", OriginalName: "placed_at", DataType: "timestamp", Nullable: true},
		},
		Indexes: []models.DesignIndex{
			{Name: "idx_orders_user", OriginalName: "idx_orders_user", Columns: []string{"user_id"}},
			{Name: "idx_orders_placed", OriginalName: "idx_orders_placed", Columns: []string{"placed_at"}},
		},
	}

	plan, err := PlanAlter(PostgresDialect{}, structure, want)
	if err != nil {
		t.Fatalf("PlanAlter: %v", err)
	}
	assertStatements(t, plan.Statements, []string{
		`ALTER TABLE "public"."orders" DROP CONSTRAINT "orders_pkey"`,
	})
	if !plan.Destructive {
		t.Error("dropping the primary key should mark the plan as destructive")
	}
}

func TestPlanAlterSQLiteIsHonestAboutItsLimits(t *testing.T) {
	// A real SQLite structure reports its object as living in "main".
	structure := sampleStructure()
	structure.Object.Database = "main"
	structure.Object.Schema = ""

	want := models.TableDesign{
		Object: "orders",
		Columns: []models.DesignColumn{
			// Dropping the primary key and changing a type are the two things
			// SQLite refuses to do to an existing table.
			{Name: "id", OriginalName: "id", DataType: "int"},
			{Name: "customer_id", OriginalName: "user_id", DataType: "TEXT", Nullable: true},
			{Name: "total", DataType: "REAL", Nullable: false, DefaultValue: strptr("0")},
		},
		Indexes: []models.DesignIndex{
			{Name: "idx_orders_user", OriginalName: "idx_orders_user", Columns: []string{"customer_id"}},
		},
	}

	plan, err := PlanAlter(SQLiteDialect{}, structure, want)
	if err != nil {
		t.Fatalf("PlanAlter: %v", err)
	}

	// The renamed column keeps its index: SQLite rewrites index definitions on
	// RENAME COLUMN, so there is nothing to drop and create there.
	assertStatements(t, plan.Statements, []string{
		`DROP INDEX "idx_orders_placed"`,
		`ALTER TABLE "orders" DROP COLUMN "placed_at"`,
		`ALTER TABLE "orders" ADD COLUMN "total" REAL NOT NULL DEFAULT 0`,
		`ALTER TABLE "orders" RENAME COLUMN "user_id" TO "customer_id"`,
	})

	joined := strings.Join(plan.Warnings, " | ")
	if !strings.Contains(joined, "SQLite cannot change the type of field customer_id") {
		t.Errorf("expected a warning about the untouched column type, got %q", joined)
	}
	if !strings.Contains(joined, "SQLite cannot change a primary key") {
		t.Errorf("expected a warning about the primary key, got %q", joined)
	}
}

func TestPlanAlterSQLiteRebuildsARenamedIndex(t *testing.T) {
	structure := sampleStructure()
	structure.Object.Database = "main"
	structure.Object.Schema = ""

	want := models.TableDesign{
		Object: "orders",
		Columns: []models.DesignColumn{
			{Name: "id", OriginalName: "id", DataType: "int", PrimaryKey: true},
			{Name: "user_id", OriginalName: "user_id", DataType: "int", Nullable: true},
			{Name: "placed_at", OriginalName: "placed_at", DataType: "timestamp", Nullable: true},
		},
		Indexes: []models.DesignIndex{
			{Name: "idx_orders_customer", OriginalName: "idx_orders_user", Columns: []string{"user_id"}},
			{Name: "idx_orders_placed", OriginalName: "idx_orders_placed", Columns: []string{"placed_at"}},
		},
	}

	plan, err := PlanAlter(SQLiteDialect{}, structure, want)
	if err != nil {
		t.Fatalf("PlanAlter: %v", err)
	}
	assertStatements(t, plan.Statements, []string{
		`DROP INDEX "idx_orders_user"`,
		`CREATE INDEX "idx_orders_customer" ON "orders" ("user_id")`,
	})
	if joined := strings.Join(plan.Warnings, " | "); !strings.Contains(joined, "SQLite cannot rename an index") {
		t.Errorf("expected a warning about the index rename, got %q", joined)
	}
}

func TestPlanAlterRejectsDraftsNoEngineWouldAccept(t *testing.T) {
	base := models.TableDesign{
		Object: "orders",
		Columns: []models.DesignColumn{
			{Name: "id", OriginalName: "id", DataType: "int", PrimaryKey: true},
		},
	}

	cases := []struct {
		name  string
		mutate func(*models.TableDesign)
		want   string
	}{
		{
			name:   "no fields left",
			mutate: func(d *models.TableDesign) { d.Columns = nil },
			want:   "at least one field",
		},
		{
			name:   "field without a name",
			mutate: func(d *models.TableDesign) { d.Columns = append(d.Columns, models.DesignColumn{DataType: "int"}) },
			want:   "needs a name",
		},
		{
			name: "field without a type",
			mutate: func(d *models.TableDesign) {
				d.Columns = append(d.Columns, models.DesignColumn{Name: "total"})
			},
			want: "needs a type",
		},
		{
			name: "duplicate field",
			mutate: func(d *models.TableDesign) {
				d.Columns = append(d.Columns, models.DesignColumn{Name: "ID", DataType: "int"})
			},
			want: "two fields are called",
		},
		{
			name: "index without fields",
			mutate: func(d *models.TableDesign) {
				d.Indexes = []models.DesignIndex{{Name: "idx"}}
			},
			want: "does not name a field",
		},
		{
			name: "index on a field that is gone",
			mutate: func(d *models.TableDesign) {
				d.Indexes = []models.DesignIndex{{Name: "idx", Columns: []string{"placed_at"}}}
			},
			want: "refers to the unknown field",
		},
		{
			name:   "renaming the table",
			mutate: func(d *models.TableDesign) { d.Object = "orders_v2" },
			want:   "cannot rename a table",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			draft := base
			draft.Columns = append([]models.DesignColumn{}, base.Columns...)
			tc.mutate(&draft)
			_, err := PlanAlter(MySQLDialect{}, sampleStructure(), draft)
			if err == nil {
				t.Fatalf("expected an error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected an error containing %q, got %q", tc.want, err.Error())
			}
		})
	}
}

func TestPlanAlterUnchangedDesignIsEmpty(t *testing.T) {
	structure := sampleStructure()
	draft := models.TableDesign{Object: "orders"}
	for _, c := range structure.Columns {
		draft.Columns = append(draft.Columns, models.DesignColumn{
			Name:         c.Name,
			OriginalName: c.Name,
			DataType:     columnTypeOf(c),
			Nullable:     c.Nullable,
			PrimaryKey:   c.PrimaryKey,
			DefaultValue: c.DefaultValue,
			Comment:      c.Comment,
		})
	}
	for _, ix := range structure.Indexes {
		if ix.Primary {
			continue
		}
		draft.Indexes = append(draft.Indexes, models.DesignIndex{
			Name:         ix.Name,
			OriginalName: ix.Name,
			Columns:      ix.Columns,
			Unique:       ix.Unique,
		})
	}

	for _, dialect := range []drivers.Dialect{MySQLDialect{}, PostgresDialect{}, SQLiteDialect{}} {
		plan, err := PlanAlter(dialect, structure, draft)
		if err != nil {
			t.Fatalf("%s: %v", dialect.Name(), err)
		}
		if len(plan.Statements) != 0 {
			t.Errorf("%s: expected no statements, got %v", dialect.Name(), plan.Statements)
		}
		if plan.Destructive {
			t.Errorf("%s: an untouched design is not destructive", dialect.Name())
		}
	}
}

func TestPlanAlterRefusesUnknownEngine(t *testing.T) {
	draft := models.TableDesign{
		Object:  "orders",
		Columns: []models.DesignColumn{{Name: "id", OriginalName: "id", DataType: "int"}},
	}
	if _, err := PlanAlter(OracleDialect{}, sampleStructure(), draft); err == nil {
		t.Fatal("expected the designer to refuse an engine it cannot alter")
	}
}
