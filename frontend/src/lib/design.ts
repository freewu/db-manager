/**
 * Table designer helpers.
 *
 * A draft is the *complete desired definition* of a table. It starts as a copy
 * of the live structure and every edit is a plain state change on that copy, so
 * "nothing changed" is exactly "the draft still equals the structure it came
 * from". The backend turns the difference into SQL.
 */
import type {
  DesignColumn,
  DriverType,
  TableDesign,
  TableStructure,
} from '../api/types'

/** Prefills a draft from a live structure, ready to be edited. */
export function designFrom(structure: TableStructure, sessionId: string): TableDesign {
  const design: TableDesign = {
    sessionId,
    database: structure.object.database,
    schema: structure.object.schema,
    object: structure.object.name,
    columns: [],
    indexes: [],
  }

  for (const column of structure.columns) {
    design.columns.push({
      name: column.name,
      originalName: column.name,
      dataType: column.columnType || column.dataType,
      nullable: column.nullable && !column.primaryKey,
      defaultValue: column.defaultValue ?? null,
      primaryKey: Boolean(column.primaryKey),
      autoIncrement: Boolean(column.autoIncrement),
      comment: column.comment ?? '',
    })
  }

  // The primary key is edited on the fields themselves, so its index is left
  // out of the index list (rendering it read-only instead).
  for (const index of structure.indexes) {
    if (index.primary) continue
    design.indexes.push({
      name: index.name,
      originalName: index.name,
      columns: [...index.columns],
      unique: index.unique,
    })
  }

  return design
}

/**
 * The structure of a table that does not exist yet.
 *
 * The designer compares a draft against the structure it was prefilled from, so
 * creating a table needs a baseline too: an empty one, which every draft of a
 * new table differs from as soon as it has a field.
 */
export function emptyStructure(database: string, schema: string): TableStructure {
  return {
    object: { name: '', kind: 'table', database, schema, rowEstimate: 0, sizeBytes: 0 },
    columns: [],
    indexes: [],
    foreignKeys: [],
    ddl: '',
  }
}

/**
 * A draft for a new table: a name to fill in and one auto-incrementing key, the
 * way a designer window opens on a fresh table.
 */
export function newTableDesign(
  sessionId: string,
  database: string,
  schema: string,
  driver: DriverType | undefined,
): TableDesign {
  const first = typeSuggestions(driver)[0] ?? 'int'
  return {
    sessionId,
    database,
    schema,
    object: '',
    // SQLite auto-increments its INTEGER PRIMARY KEY by definition, so the flag
    // would only make the designer claim something the engine writes itself.
    columns: [
      {
        name: 'id',
        originalName: '',
        dataType: first,
        nullable: false,
        defaultValue: null,
        primaryKey: true,
        autoIncrement: driver !== 'sqlite',
        comment: '',
      },
    ],
    indexes: [],
  }
}

/**
 * Canonical serialisation of a draft, used to tell "the user changed something"
 * from "the draft came straight out of the catalog".
 *
 * Only the things an engine can actually act on are compared: key order and
 * renamed-then-renamed-back fields must not count as changes.
 */
function canonical(design: TableDesign): string {
  return JSON.stringify({
    columns: design.columns.map((c) => [
      c.name.trim(),
      c.dataType.trim(),
      c.nullable && !c.primaryKey,
      c.defaultValue ?? null,
      c.primaryKey,
      c.autoIncrement,
      c.comment ?? '',
    ]),
    indexes: design.indexes.map((ix) => [ix.name.trim(), ix.unique, ix.columns.join('\u0000')]),
  })
}

/** True when the draft differs from the structure it was prefilled from. */
export function isDirty(design: TableDesign, baseline: TableStructure): boolean {
  return canonical(design) !== canonical(designFrom(baseline, design.sessionId))
}

/** A brand new field, appended to the end of the list. */
export function emptyColumn(design: TableDesign, driver: DriverType | undefined): DesignColumn {
  const suggestions = typeSuggestions(driver)
  const used = new Set(design.columns.map((c) => c.name.toLowerCase()))
  let name = 'field'
  for (let n = 2; used.has(name); n += 1) name = `field${n}`
  return {
    name,
    originalName: '',
    dataType: suggestions[0] ?? 'varchar(255)',
    nullable: true,
    defaultValue: null,
    primaryKey: false,
    autoIncrement: false,
    comment: '',
  }
}

/** Field names of a draft, in order (the options of an index editor). */
export function fieldNames(design: TableDesign): string[] {
  return design.columns.map((c) => c.name.trim()).filter(Boolean)
}

/** The fields the draft marks as the primary key. */
export function primaryKeyFields(design: TableDesign): string[] {
  return design.columns.filter((c) => c.primaryKey).map((c) => c.name.trim())
}

/**
 * Types offered while typing a column type. The list is a starting point, not a
 * restriction: every engine accepts far more than these, and the cell stays
 * free text.
 */
const TYPE_SUGGESTIONS: Partial<Record<DriverType, string[]>> = {
  mysql: [
    'int',
    'bigint',
    'smallint',
    'tinyint(1)',
    'decimal(10,2)',
    'double',
    'float',
    'varchar(255)',
    'char(36)',
    'text',
    'longtext',
    'json',
    'date',
    'datetime',
    'timestamp',
    'time',
    'blob',
  ],
  postgres: [
    'integer',
    'bigint',
    'smallint',
    'numeric(10,2)',
    'real',
    'double precision',
    'varchar(255)',
    'char(36)',
    'text',
    'jsonb',
    'uuid',
    'boolean',
    'date',
    'timestamp',
    'timestamptz',
    'time',
    'bytea',
  ],
  sqlite: ['INTEGER', 'REAL', 'TEXT', 'BLOB', 'NUMERIC', 'BOOLEAN', 'DATETIME'],
  // TiDB is MySQL-compatible at the type level too, so it gets the same list.
  tidb: [
    'int',
    'bigint',
    'smallint',
    'tinyint(1)',
    'decimal(10,2)',
    'double',
    'float',
    'varchar(255)',
    'char(36)',
    'text',
    'json',
    'date',
    'datetime',
    'timestamp',
    'time',
    'blob',
  ],
  // Doris has its own type names; the aggregate types are listed because they
  // are what a table on this engine is usually read through.
  doris: [
    'boolean',
    'tinyint',
    'smallint',
    'int',
    'bigint',
    'largeint',
    'float',
    'double',
    'decimal(10,2)',
    'date',
    'datetime',
    'datev2',
    'datetimev2',
    'char(10)',
    'varchar(255)',
    'string',
    'json',
    'jsonb',
    'bitmap',
    'hll',
  ],
}

export function typeSuggestions(driver: DriverType | undefined): string[] {
  if (!driver) return []
  return TYPE_SUGGESTIONS[driver] ?? []
}
