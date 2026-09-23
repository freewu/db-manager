/**
 * Data generation: fill a table with made-up rows.
 *
 * The window is a two-sided thing. On the left, the tables the data could go
 * into — the databases of the open connections, and under each one the tables
 * it holds, since a run has exactly one destination. On the right, that table's
 * columns and what each of them gets: a mock, written in mock.js syntax, with a
 * sample of what it produces.
 *
 * A column takes part in a run only while its box is ticked. Every column
 * starts ticked except the auto-increment ones, whose keys the engine has to
 * hand out — a run that invented them would collide with itself. The tick, not
 * the text, is what decides the INSERT's column list: a ticked column with no
 * mock stops the run rather than being sent as an empty string.
 *
 * Nothing is written until Generate is pressed, and what it then does is send
 * batches of rendered rows to the backend, which binds every value as a
 * parameter and reports what landed. The loop lives here rather than in Go so
 * that progress can be shown between batches and a run can be stopped between
 * them; the batching is here so that the count on screen always matches the rows
 * that were really written.
 *
 * The connection is the window's: each one gets its own window, so the mocks
 * written for a table are not thrown away by looking at another table. Picking a
 * table under a different connection brings up *that* connection's window, where
 * the same rule holds.
 */
import { useCallback, useEffect, useMemo, useRef, useState, type Key } from 'react'
import {
  Alert,
  App as AntApp,
  Button,
  Empty,
  Input,
  InputNumber,
  Progress,
  Space,
  Splitter,
  Table,
  Tooltip,
  Tree,
  Typography,
} from 'antd'
import type { DataNode } from 'antd/es/tree'
import type { TableColumnsType } from 'antd'
import {
  ExperimentOutlined,
  ReloadOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type { ColumnInfo, TableStructure } from '../api/types'
import { capabilitiesOf } from '../lib/capabilities'
import { kindOfColumn, type ColumnKind } from '../lib/codegen'
import {
  coerceMockValue,
  compileTemplate,
  createRng,
  defaultMock,
  mockDescription,
  type CompiledTemplate,
  type MockValue,
} from '../lib/mock'
import {
  databaseKey,
  decodeNode,
  encodeNode,
  namespaceKey,
  objectsKey,
} from '../lib/tree'
import { useAppStore, type WorkspaceTab } from '../store/appStore'
import { MockPickerModal } from './MockPickerModal'

/**
 * Rows per request.
 *
 * Well under the service's cap of 500 per call, and small enough that a
 * statement stays comfortably inside MySQL's `max_allowed_packet` however wide
 * the table is.
 */
const INSERT_BATCH = 200

/** Rows a run starts with, and the most one run may be asked for. */
const DEFAULT_ROWS = 100
const MAX_ROWS = 100000

/** The separator used inside this window's own keys; it cannot occur in a name. */
const SEP = '\u0000'

const tableKey = (database: string, schema: string, object: string) =>
  [database, schema, object].join(SEP)

/**
 * A seed that depends only on a field's name.
 *
 * The sample in the description column has to hold still while the user types in
 * another row, so it is drawn from a source seeded by the name rather than from
 * the run's own generator.
 */
function seedOf(text: string): number {
  let hash = 2166136261
  for (let i = 0; i < text.length; i += 1) hash = Math.imul(hash ^ text.charCodeAt(i), 16777619)
  return hash >>> 0
}

/** One column of the table being filled, and the mock it will get. */
interface FieldRow {
  name: string
  column: ColumnInfo
  kind: ColumnKind
  /** The mock as written; empty means the column is left out of the INSERT. */
  template: string
  compiled: CompiledTemplate
}

/** Builds the rows of the fields grid from a structure and any mocks written. */
function fieldRowsOf(
  structure: TableStructure | undefined,
  overrides: Record<string, string> | undefined,
): FieldRow[] {
  return (structure?.columns ?? []).map((column) => {
    const template = overrides?.[column.name] ?? defaultMock(column)
    return {
      name: column.name,
      column,
      kind: kindOfColumn(column),
      template,
      compiled: compileTemplate(template),
    }
  })
}

/** What one run did, as the alert under the toolbar reports it. */
type RunResult =
  | { kind: 'done'; inserted: number }
  | { kind: 'stopped'; inserted: number }
  | { kind: 'failed'; inserted: number; row: number; error: string }
  | { kind: 'error'; inserted: number; error: string }

interface DataGenPaneProps {
  tab: WorkspaceTab
}

export function DataGenPane({ tab }: DataGenPaneProps) {
  const sessionId = tab.sessionId
  const { modal } = AntApp.useApp()
  const session = useAppStore((s) => s.sessionOf(sessionId))
  const sessions = useAppStore((s) => s.sessions)
  const drivers = useAppStore((s) => s.drivers)
  const tree = useAppStore((s) => s.tree)
  const loadDatabases = useAppStore((s) => s.loadDatabases)
  const loadSchemas = useAppStore((s) => s.loadSchemas)
  const loadObjects = useAppStore((s) => s.loadObjects)
  const openDataGenTab = useAppStore((s) => s.openDataGenTab)

  const driver = drivers.find((d) => d.type === session?.driver)
  const { flatNamespace } = capabilitiesOf(driver)
  const insertable = capabilitiesOf(driver).insertable
  const readOnly = Boolean(session?.readOnly)

  const database = tab.database ?? ''
  const schema = tab.schema ?? ''
  const object = tab.object ?? ''
  const currentKey = tableKey(database, schema, object)

  const [structure, setStructure] = useState<TableStructure>()
  const [structureError, setStructureError] = useState<string>()
  const [loading, setLoading] = useState(false)
  const [reloadToken, setReloadToken] = useState(0)
  /** The mocks written so far, per table, so switching tables keeps them. */
  const [overrides, setOverrides] = useState<Record<string, Record<string, string>>>({})
  /**
   * Which columns are ticked, per table. An absent entry means "the default":
   * every column but the auto-increment ones. Only the tables that were touched
   * are stored, so a table nobody has ticked anything in still follows its
   * structure when that is reloaded.
   */
  const [chosen, setChosen] = useState<Record<string, Record<string, boolean>>>({})
  const [pickerField, setPickerField] = useState<string>()
  const [rowsToWrite, setRowsToWrite] = useState(DEFAULT_ROWS)
  const [running, setRunning] = useState(false)
  const [progress, setProgress] = useState<{ done: number; total: number }>()
  const [result, setResult] = useState<RunResult>()
  const [expanded, setExpanded] = useState<string[]>([])
  const stopRef = useRef(false)

  const rows = useMemo(() => fieldRowsOf(structure, overrides[currentKey]), [structure, overrides, currentKey])

  // The tick is the window's own state rather than a reading of the mock: an
  // empty mock is a value decision, the tick is the decision that the column
  // takes part at all.
  const selectedOf = useCallback(
    (column: ColumnInfo) => chosen[currentKey]?.[column.name] ?? !column.autoIncrement,
    [chosen, currentKey],
  )
  const ticked = useMemo(() => rows.filter((row) => selectedOf(row.column)), [rows, selectedOf])
  const tickedNames = useMemo(() => ticked.map((row) => row.name), [ticked])
  // Only the ticked columns can stop a run: an unticked field's mock is not
  // read, so a mistake in it is not a mistake yet.
  const broken = useMemo(() => ticked.filter((row) => row.compiled.error), [ticked])
  const missing = useMemo(() => ticked.filter((row) => row.template.trim() === ''), [ticked])

  /* --- the structure of the picked table -------------------------------- */

  useEffect(() => {
    if (!insertable || !database || !object) {
      setStructure(undefined)
      setStructureError(undefined)
      return
    }
    let cancelled = false
    setLoading(true)
    setStructureError(undefined)
    api
      .getStructure(sessionId, database, schema, object)
      .then((answer) => {
        if (!cancelled) setStructure(answer)
      })
      .catch((error: unknown) => {
        if (cancelled) return
        setStructure(undefined)
        setStructureError(toMessage(error))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [insertable, sessionId, database, schema, object, reloadToken])

  /* --- the picker tree --------------------------------------------------- */

  /**
   * Opens the path to the window's own table.
   *
   * The tree is expanded declaratively, so a jump — from the explorer, or from
   * another connection's window — has to add the ancestors it passes through.
   * The keys already there are kept, which is what makes the tree usable at all:
   * opening a second table must not collapse the first one's database.
   */
  useEffect(() => {
    if (!insertable || !database) return
    const keys = [encodeNode({ t: 'session', sessionId })]
    if (object) {
      keys.push(encodeNode({ t: 'db', sessionId, database }))
      if (!flatNamespace && schema) {
        keys.push(encodeNode({ t: 'schema', sessionId, database, schema }))
      }
    }
    setExpanded((previous) => {
      const missing = keys.filter((key) => !previous.includes(key))
      return missing.length === 0 ? previous : [...previous, ...missing]
    })
    // The nodes just expanded have to have something to show. What is already
    // read is not read again: the tree beside this window is a picker, and the
    // toolbar's *Reload* is what refreshes it on purpose.
    const cache = useAppStore.getState().tree
    if (!cache.loaded[sessionId]) void loadDatabases(sessionId)
    if (!object) return
    if (!flatNamespace && schema) {
      if (!cache.loaded[databaseKey(sessionId, database)]) void loadSchemas(sessionId, database)
      if (!cache.loaded[namespaceKey(sessionId, database, schema)]) {
        void loadObjects(sessionId, database, schema)
      }
    } else if (!cache.loaded[namespaceKey(sessionId, database, database)]) {
      void loadObjects(sessionId, database, database)
    }
  }, [insertable, sessionId, database, schema, object, flatNamespace, loadDatabases, loadSchemas, loadObjects])

  const loadNode = useCallback(
    async (node: DataNode) => {
      const ref = decodeNode(String(node.key))
      if (!ref) return
      if (ref.t === 'session') {
        await loadDatabases(ref.sessionId)
        return
      }
      if (ref.t === 'db') {
        if (flatNamespace) await loadObjects(ref.sessionId, ref.database, ref.database)
        else await loadSchemas(ref.sessionId, ref.database)
        return
      }
      if (ref.t === 'schema') await loadObjects(ref.sessionId, ref.database, ref.schema)
    },
    [flatNamespace, loadDatabases, loadObjects, loadSchemas],
  )

  const treeData = useMemo<DataNode[]>(() => {
    const placeholder = (key: string, title: string, danger = false): DataNode => ({
      key,
      isLeaf: true,
      selectable: false,
      title: <Typography.Text type={danger ? 'danger' : 'secondary'}>{title}</Typography.Text>,
    })

    const objectNodes = (
      sessionIdHere: string,
      db: string,
      ns: string,
    ): DataNode[] | undefined => {
      const scope = namespaceKey(sessionIdHere, db, ns)
      if (tree.errors[scope]) return [placeholder(`error:${scope}`, tree.errors[scope], true)]
      if (!tree.loaded[scope]) return undefined
      const objects = tree.objects[objectsKey(sessionIdHere, db, ns)] ?? []
      const tables = objects.filter((entry) => entry.kind === 'table')
      if (tables.length === 0) return [placeholder(`placeholder:${scope}`, 'No tables')]
      return tables.map((entry) => ({
        key: encodeNode({
          t: 'object',
          sessionId: sessionIdHere,
          database: db,
          schema: ns,
          object: entry.name,
          kind: 'table',
        }),
        isLeaf: true,
        title: entry.name,
      }))
    }

    const dbNode = (sessionIdHere: string, db: string): DataNode => {
      const node: DataNode = {
        key: encodeNode({ t: 'db', sessionId: sessionIdHere, database: db }),
        title: db,
        isLeaf: false,
      }
      if (flatNamespace) {
        node.children = objectNodes(sessionIdHere, db, db)
      } else {
        const scope = databaseKey(sessionIdHere, db)
        if (tree.errors[scope]) node.children = [placeholder(`error:${scope}`, tree.errors[scope], true)]
        else if (tree.loaded[scope]) {
          const schemas = tree.schemas[scope] ?? []
          node.children =
            schemas.length === 0
              ? [placeholder(`placeholder:${scope}`, 'No schemas')]
              : schemas.map((schemaName) => ({
                  key: encodeNode({
                    t: 'schema',
                    sessionId: sessionIdHere,
                    database: db,
                    schema: schemaName,
                  }),
                  title: schemaName,
                  isLeaf: false,
                  children: objectNodes(sessionIdHere, db, schemaName),
                }))
        }
      }
      return node
    }

    return sessions.map((entry) => {
      const canFill = capabilitiesOf(drivers.find((d) => d.type === entry.driver)).insertable
      const node: DataNode = {
        key: encodeNode({ t: 'session', sessionId: entry.id }),
        title: (
          <Space size={6}>
            <span>{entry.name}</span>
            {canFill ? null : <Typography.Text type="secondary">(cannot insert)</Typography.Text>}
          </Space>
        ),
        isLeaf: false,
        disabled: !canFill,
      }
      if (!canFill) return node
      if (tree.errors[entry.id]) node.children = [placeholder(`error:${entry.id}`, tree.errors[entry.id], true)]
      else if (tree.loaded[entry.id]) {
        const databases = tree.databases[entry.id] ?? []
        node.children =
          databases.length === 0
            ? [placeholder(`placeholder:${entry.id}`, 'No databases')]
            : databases.map((db) => dbNode(entry.id, db))
      }
      return node
    })
  }, [sessions, drivers, tree, flatNamespace])

  const selectedKeys = useMemo(() => {
    if (!object) return []
    return [
      encodeNode({
        t: 'object',
        sessionId,
        database,
        schema: flatNamespace ? database : schema,
        object,
        kind: 'table',
      }),
    ]
  }, [sessionId, database, schema, object, flatNamespace])

  const selectNode = useCallback(
    (keys: Key[]) => {
      const ref = decodeNode(String(keys[0] ?? ''))
      if (!ref || ref.t !== 'object') return
      // A table belongs to one connection, and that connection has the window
      // that knows where it writes: picking one from here brings that window up
      // rather than aiming this one at a target it does not own.
      openDataGenTab(ref.sessionId, ref.database, ref.schema, ref.object)
    },
    [openDataGenTab],
  )

  /* --- editing the mocks ------------------------------------------------- */

  const setMock = useCallback(
    (name: string, template: string) => {
      setOverrides((previous) => ({
        ...previous,
        [currentKey]: { ...previous[currentKey], [name]: template },
      }))
    },
    [currentKey],
  )

  /** Ticks a column on or off; the whole table is snapshotted on the first toggle. */
  const chooseFields = useCallback(
    (names: string[]) => {
      const picked = new Set(names)
      setChosen((previous) => ({
        ...previous,
        [currentKey]: Object.fromEntries(rows.map((row) => [row.name, picked.has(row.name)])),
      }))
    },
    [rows, currentKey],
  )

  const resetMocks = useCallback(() => {
    // The ticks go back to their defaults too: this button restores the table's
    // starting point, and half a reset would leave a column out that the
    // structure says should be in.
    setOverrides((previous) => ({ ...previous, [currentKey]: {} }))
    setChosen((previous) => ({ ...previous, [currentKey]: {} }))
  }, [currentKey])

  /* --- the run ----------------------------------------------------------- */

  const blocker = !insertable
    ? 'This engine cannot insert rows'
    : readOnly
      ? 'This session is read-only'
      : !object
        ? 'Pick a table on the left'
        : broken.length > 0
          ? `Fix the mock in ${broken.map((row) => row.name).join(', ')}`
          : missing.length > 0
            ? `Write a mock for ${missing.map((row) => row.name).join(', ')} or untick the field`
            : ticked.length === 0
              ? 'No field is ticked'
              : undefined

  const generate = useCallback(() => {
    if (blocker || !structure || !object) return
    modal.confirm({
      title: `Generate ${rowsToWrite} row(s) into “${object}”?`,
      content: (
        <div style={{ fontSize: 12 }}>
          <div>
            {ticked.length} of {structure.columns.length} column(s) are sent:{' '}
            <span className="mono">{ticked.map((row) => row.name).join(', ')}</span>
          </div>
          <div style={{ marginTop: 6 }}>
            The rows go straight into the table in batches of {INSERT_BATCH}. There is no undo —
            delete them the way you would delete any other row.
          </div>
        </div>
      ),
      okText: 'Generate',
      onOk: async () => {
        const rng = createRng()
        const total = rowsToWrite
        const columns = ticked.map((row) => row.name)
        stopRef.current = false
        setResult(undefined)
        setRunning(true)
        setProgress({ done: 0, total })
        let inserted = 0
        try {
          for (let start = 0; start < total; start += INSERT_BATCH) {
            if (stopRef.current) {
              setResult({ kind: 'stopped', inserted })
              return
            }
            const size = Math.min(INSERT_BATCH, total - start)
            const batch: unknown[][] = []
            for (let i = 0; i < size; i += 1) {
              batch.push(
                ticked.map((row) =>
                  coerceMockValue(row.compiled.render(rng) as MockValue, row.kind),
                ),
              )
            }
            const answer = await api.insertRows({
              sessionId,
              database,
              schema,
              object,
              columns,
              rows: batch,
            })
            inserted += answer.inserted
            if (answer.failed) {
              setResult({
                kind: 'failed',
                inserted,
                row: start + answer.failed,
                error: answer.error ?? 'the engine refused the row',
              })
              return
            }
            setProgress({ done: start + size, total })
          }
          setResult({ kind: 'done', inserted })
        } catch (error) {
          setResult({ kind: 'error', inserted, error: toMessage(error) })
        } finally {
          setRunning(false)
          if (inserted > 0) void loadObjects(sessionId, database, schema)
        }
      },
    })
  }, [
    blocker,
    structure,
    object,
    rowsToWrite,
    ticked,
    modal,
    sessionId,
    database,
    schema,
    loadObjects,
  ])

  /* --- the fields grid --------------------------------------------------- */

  const columns: TableColumnsType<FieldRow> = useMemo(
    () => [
      {
        title: 'Field',
        dataIndex: 'name',
        width: 200,
        render: (name: string, row) => (
          <Space size={6}>
            <span className="mono">{name}</span>
            {row.column.primaryKey ? (
              <Typography.Text type="secondary" style={{ fontSize: 11 }}>
                key
              </Typography.Text>
            ) : null}
          </Space>
        ),
      },
      {
        title: 'Type',
        dataIndex: 'kind',
        width: 160,
        render: (_kind: ColumnKind, row) => (
          <span className="mono" style={{ fontSize: 12 }}>
            {row.column.columnType || row.column.dataType}
          </span>
        ),
      },
      {
        title: 'Mock',
        dataIndex: 'template',
        width: 320,
        render: (_value: string, row) => (
          <Space.Compact style={{ width: '100%' }}>
            <Input
              size="small"
              className="mono"
              value={row.template}
              status={selectedOf(row.column) && row.compiled.error ? 'error' : undefined}
              placeholder="(no mock)"
              onChange={(event) => setMock(row.name, event.target.value)}
            />
            <Tooltip title="Pick a placeholder">
              <Button
                size="small"
                icon={<ExperimentOutlined />}
                onClick={() => setPickerField(row.name)}
              />
            </Tooltip>
          </Space.Compact>
        ),
      },
      {
        title: 'Description',
        dataIndex: 'name',
        render: (_name: string, row) => <DescriptionCell row={row} selected={selectedOf(row.column)} />,
      },
    ],
    [selectedOf, setMock],
  )

  if (!insertable) {
    return (
      <div className="dm-pane">
        <Alert
          banner
          type="info"
          showIcon
          title="This engine cannot insert generated rows"
          description={
            driver?.type === 'mongodb'
              ? 'A collection has no column list, so there is nothing for a batch of generated values to line up with. Generate documents with the query window instead.'
              : 'The driver for this connection does not implement row inserts.'
          }
        />
      </div>
    )
  }

  return (
    <div className="dm-pane">
      <Splitter style={{ flex: '1 1 auto', minHeight: 0 }}>
        <Splitter.Panel defaultSize={280} min={200} max={460} className="dm-datagen-picker">
          <div className="dm-pane-body">
            <div className="dm-datagen-hint">
              <Typography.Text type="secondary" style={{ fontSize: 11 }}>
                {flatNamespace ? 'Connection → database → tables' : 'Connection → database → schema → tables'}
              </Typography.Text>
            </div>
            <Tree
              treeData={treeData}
              loadData={loadNode}
              expandedKeys={expanded}
              onExpand={(keys) => setExpanded(keys.map(String))}
              selectedKeys={selectedKeys}
              onSelect={selectNode}
              blockNode
            />
          </div>
        </Splitter.Panel>

        <Splitter.Panel min={420}>
          <div className="dm-pane">
            <div className="dm-editor-toolbar">
              <Typography.Text strong>
                {object ? <span className="mono">{object}</span> : 'No table picked'}
              </Typography.Text>
              {readOnly ? (
                <Typography.Text type="warning" style={{ fontSize: 12 }}>
                  read-only
                </Typography.Text>
              ) : null}
              <div className="dm-toolbar-right">
                <Space size={6}>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    Rows
                  </Typography.Text>
                  <InputNumber
                    size="small"
                    min={1}
                    max={MAX_ROWS}
                    value={rowsToWrite}
                    disabled={running}
                    onChange={(value) => setRowsToWrite(Math.max(1, Math.min(MAX_ROWS, Number(value ?? DEFAULT_ROWS))))}
                    style={{ width: 96 }}
                  />
                  <Tooltip title={`At most ${MAX_ROWS.toLocaleString()} rows per run`}>
                    <Typography.Text type="secondary" style={{ fontSize: 11 }}>
                      max {MAX_ROWS.toLocaleString()}
                    </Typography.Text>
                  </Tooltip>
                  <Button
                    size="small"
                    icon={<ReloadOutlined />}
                    disabled={!object}
                    onClick={() => {
                      setReloadToken((token) => token + 1)
                      if (database) void loadObjects(sessionId, database, flatNamespace ? database : schema)
                    }}
                  >
                    Reload
                  </Button>
                  <Tooltip title="Put every column back to the mock this table's types suggest, and tick them the way they start">
                    <Button size="small" disabled={!object} onClick={resetMocks}>
                      Reset mocks
                    </Button>
                  </Tooltip>
                  {running ? (
                    <Button size="small" danger onClick={() => (stopRef.current = true)}>
                      Stop
                    </Button>
                  ) : (
                    <Tooltip title={blocker ?? 'Insert the generated rows'}>
                      <Button
                        size="small"
                        type="primary"
                        icon={<ThunderboltOutlined />}
                        disabled={Boolean(blocker)}
                        onClick={generate}
                      >
                        Generate
                      </Button>
                    </Tooltip>
                  )}
                </Space>
              </div>
            </div>

            {readOnly && object ? (
              <Alert
                banner
                type="warning"
                showIcon
                title="This session is read-only"
                description="Connect without the read-only flag to write rows into this table."
              />
            ) : null}

            {structureError ? (
              <Alert
                banner
                type="error"
                showIcon
                title={`${object} could not be read`}
                description={<span className="mono">{structureError}</span>}
              />
            ) : null}

            {progress && running ? (
              <div className="dm-datagen-progress">
                <div className="dm-datagen-progress-head">
                  <span>
                    Inserted {progress.done.toLocaleString()} / {progress.total.toLocaleString()} row(s)
                  </span>
                  <Button size="small" type="link" danger onClick={() => (stopRef.current = true)}>
                    Stop
                  </Button>
                </div>
                <Progress
                  percent={progress.total === 0 ? 0 : Math.round((progress.done / progress.total) * 100)}
                  size="small"
                  status="active"
                />
              </div>
            ) : null}

            {result ? (
              <Alert
                banner
                showIcon
                closable
                onClose={() => setResult(undefined)}
                type={
                  result.kind === 'done' ? 'success' : result.kind === 'stopped' ? 'warning' : 'error'
                }
                title={
                  result.kind === 'done'
                    ? `Inserted ${result.inserted.toLocaleString()} row(s)`
                    : result.kind === 'stopped'
                      ? `Stopped after ${result.inserted.toLocaleString()} row(s)`
                      : result.kind === 'failed'
                        ? `Row ${result.row.toLocaleString()} was refused — ${result.inserted.toLocaleString()} row(s) are in the table`
                        : `Insert failed after ${result.inserted.toLocaleString()} row(s)`
                }
                description={
                  result.kind === 'failed' || result.kind === 'error' ? (
                    <span className="mono" style={{ fontSize: 12, whiteSpace: 'pre-wrap' }}>
                      {result.error}
                    </span>
                  ) : undefined
                }
              />
            ) : null}

            {broken.length > 0 && object ? (
              <Alert
                banner
                type="error"
                showIcon
                title={`Unknown placeholder in ${broken.map((row) => row.name).join(', ')}`}
                description="A mock is mock.js syntax: an @name this app does not implement cannot be rendered, and is never written as text. Pick one from the catalogue, or write @@ for a literal @."
              />
            ) : null}

            <div className="dm-pane-body">
              {!object ? (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description="Pick a table to fill"
                  style={{ marginTop: 60 }}
                />
              ) : loading && !structure ? (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description="Reading the table…"
                  style={{ marginTop: 60 }}
                />
              ) : structure && structure.columns.length === 0 ? (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description="This table has no columns"
                  style={{ marginTop: 60 }}
                />
              ) : structure ? (
                <Table
                  className="dm-grid dm-datagen-grid"
                  size="small"
                  rowKey="name"
                  columns={columns}
                  dataSource={rows}
                  pagination={false}
                  scroll={{ x: 'max-content' }}
                  // The tick is what puts a column in the INSERT, so it leads
                  // the row; the header box is the table-wide all / none.
                  rowSelection={{
                    selectedRowKeys: tickedNames,
                    onChange: (keys) => chooseFields(keys.map(String)),
                    columnWidth: 40,
                  }}
                  rowClassName={(row) => (selectedOf(row.column) ? '' : 'dm-datagen-off')}
                />
              ) : null}
            </div>
          </div>
        </Splitter.Panel>
      </Splitter>

      <MockPickerModal
        field={pickerField}
        onPick={(value) => {
          if (pickerField) setMock(pickerField, value)
          setPickerField(undefined)
        }}
        onClose={() => setPickerField(undefined)}
      />
    </div>
  )
}

/* --- the description column ------------------------------------------------ */

/** The description column: what the column is, and what the mock makes of it. */
function DescriptionCell({ row, selected }: { row: FieldRow; selected: boolean }) {
  const sample = useMemo(() => {
    if (!selected || row.compiled.error || row.template.trim() === '') return undefined
    const value = row.compiled.render(createRng(seedOf(row.name)))
    if (value === null || value === undefined || value === '') return undefined
    return String(value)
  }, [row, selected])

  return (
    <div className="dm-datagen-note">
      {row.column.comment ? <div>{row.column.comment}</div> : null}
      <div className={selected && row.compiled.error ? 'dm-datagen-bad' : undefined}>
        {selected
          ? mockDescription(row.column, row.template, row.compiled)
          : row.column.autoIncrement
            ? 'Auto-increment — the engine assigns this column'
            : 'Not sent — this column is left out of the INSERT'}
      </div>
      {sample ? <div className="dm-datagen-sample">e.g. {sample}</div> : null}
    </div>
  )
}
