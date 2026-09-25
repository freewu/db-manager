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
 * A row the engine will not take normally ends the run: it is named, and the
 * count on screen is the count that landed. Tick *Skip bad rows* and the
 * refusals are counted instead — the backend leaves those rows out and carries
 * on, so a hundred thousand row run does not stop at one value the table will
 * not accept. Either way the alert says what the run did: rows in, rows skipped,
 * and how long it took, because a run that writes and cannot be undone deserves
 * a record rather than a progress bar that disappeared.
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
  Checkbox,
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
import { formatDuration } from '../lib/format'
import {
  coerceMockValue,
  compileTemplate,
  createRng,
  defaultMock,
  mockDescription,
  sampleOf,
  seedOf,
  type CompiledTemplate,
  type CustomPlaceholder,
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
import { useColumnResize } from './ResizableHeader'
import { SqlCode } from './SqlCode'
import { t, tn, useLanguage } from '../lib/i18n'

/**
 * Rows per request.
 *
 * Well under the service's cap of 500 per call, and small enough that a
 * statement stays comfortably inside MySQL's `max_allowed_packet` however wide
 * the table is.
 */
const INSERT_BATCH = 200

/** Rows a run starts with. The ceiling is the backend's — see the settings page. */
const DEFAULT_ROWS = 100

/**
 * What the Rows box allows until the backend has been asked for its ceiling.
 *
 * The number is a setting (Settings → Data generation → Rows per run), so this is
 * only what the window assumes while it is being read, and in a window that has
 * no backend to read it from at all. It matches the backend's default so that the
 * box does not move under the user's fingers once the answer arrives.
 */
const FALLBACK_MAX_ROWS = 1000000

/** The separator used inside this window's own keys; it cannot occur in a name. */
const SEP = '\u0000'

const tableKey = (database: string, schema: string, object: string) =>
  [database, schema, object].join(SEP)

/** One column of the table being filled, and the mock it will get. */
interface FieldRow {
  name: string
  column: ColumnInfo
  kind: ColumnKind
  /** The mock as written; empty means the column is left out of the INSERT. */
  template: string
  compiled: CompiledTemplate
}

/**
 * Builds the rows of the fields grid from a structure and any mocks written.
 *
 * The user's own placeholders are handed to the compiler here, so a mock that
 * says `@orderNo` is read the same way in every part of the window — the one
 * place a template is compiled is the one place that decides what it means.
 */
function fieldRowsOf(
  structure: TableStructure | undefined,
  overrides: Record<string, string> | undefined,
  placeholders: readonly CustomPlaceholder[],
): FieldRow[] {
  return (structure?.columns ?? []).map((column) => {
    const template = overrides?.[column.name] ?? defaultMock(column)
    return {
      name: column.name,
      column,
      kind: kindOfColumn(column),
      template,
      compiled: compileTemplate(template, placeholders),
    }
  })
}

/**
 * What one run did, as the alert under the toolbar reports it.
 *
 * Every kind carries the same tally — rows written, rows passed over, and the
 * wall-clock time of the whole run, round trips included — so the one line the
 * user is left with cannot depend on how the run ended.
 */
type RunResult =
  | { kind: 'done'; inserted: number; skipped: number; ms: number; skipReason?: string }
  | { kind: 'stopped'; inserted: number; skipped: number; ms: number; skipReason?: string }
  | { kind: 'failed'; inserted: number; skipped: number; ms: number; row: number; error: string }
  | { kind: 'error'; inserted: number; skipped: number; ms: number; error: string }

/**
 * How long a run took, as the alert and the progress line say it.
 *
 * Precise to the millisecond while it is short, and coarse once it is not: a run
 * of a hundred thousand rows is measured in minutes, where `formatDuration`'s
 * "304.61 s" would read as a number rather than as a length of time.
 */
function elapsedText(ms: number): string {
  if (ms < 10_000) return formatDuration(ms)
  const seconds = Math.round(ms / 1000)
  if (seconds < 60) return `${seconds} s`
  const minutes = Math.floor(seconds / 60)
  const rest = seconds % 60
  return rest === 0 ? `${minutes} min` : `${minutes} min ${rest} s`
}

/**
 * The headline of a finished run: how many rows, how long, how many were left
 * out. The skipped count is only worth a clause when there is one.
 */
function resultHeadline(result: RunResult): string {
  const rows = (count: number) => tn('dataGenPane.n-rows', count)
  const skipped = result.skipped > 0 ? tn('dataGenPane.rows-skipped', result.skipped) : ''
  switch (result.kind) {
    case 'done':
      return t('dataGenPane.inserted-in', {
        rows: rows(result.inserted),
        elapsed: elapsedText(result.ms),
        skipped,
      })
    case 'stopped':
      return t('dataGenPane.stopped-after-in', {
        rows: rows(result.inserted),
        elapsed: elapsedText(result.ms),
        skipped,
      })
    case 'failed':
      return t('dataGenPane.row-was-refused', {
        row: result.row.toLocaleString(),
        elapsed: elapsedText(result.ms),
        rows: rows(result.inserted),
      })
    case 'error':
      return t('dataGenPane.insert-failed-after-in', {
        rows: rows(result.inserted),
        elapsed: elapsedText(result.ms),
      })
  }
}

interface DataGenPaneProps {
  tab: WorkspaceTab
}

export function DataGenPane({ tab }: DataGenPaneProps) {
  const language = useLanguage()
  const sessionId = tab.sessionId
  const { message, modal } = AntApp.useApp()
  const session = useAppStore((s) => s.sessionOf(sessionId))
  const sessions = useAppStore((s) => s.sessions)
  const drivers = useAppStore((s) => s.drivers)
  const tree = useAppStore((s) => s.tree)
  const loadDatabases = useAppStore((s) => s.loadDatabases)
  const loadSchemas = useAppStore((s) => s.loadSchemas)
  const loadObjects = useAppStore((s) => s.loadObjects)
  const openDataGenTab = useAppStore((s) => s.openDataGenTab)
  const storedPlaceholders = useAppStore((s) => s.mockPlaceholders)
  const dataGenSettings = useAppStore((s) => s.dataGenSettings)
  const refreshMockPlaceholders = useAppStore((s) => s.refreshMockPlaceholders)

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
  // The ceiling is a setting rather than a constant of this window: what one run
  // may write is an answer about the installation, and the settings page is where
  // it is given. Until it has been read, the window allows its own default.
  const maxRows = dataGenSettings?.maxRows ?? FALLBACK_MAX_ROWS
  /**
   * Whether a row the engine refuses is left out or ends the run. Off by
   * default: a mock that keeps producing rows the table will not take is
   * something to be seen and fixed, so stopping at the first one is the honest
   * choice until the user says otherwise.
   */
  const [skipErrors, setSkipErrors] = useState(false)
  const [running, setRunning] = useState(false)
  const [progress, setProgress] = useState<{ done: number; total: number; skipped: number }>()
  const [result, setResult] = useState<RunResult>()
  /** When the run in flight started, so its elapsed time can be shown while it runs. */
  const [startedAt, setStartedAt] = useState(0)
  // Redraws once a second while a run is in flight, so the elapsed time in the
  // progress line moves. The counter itself is of no interest to anything.
  const [, setElapsedTick] = useState(0)
  const [expanded, setExpanded] = useState<string[]>([])
  const stopRef = useRef(false)

  /**
   * The user's placeholders as the engine wants them.
   *
   * A file that cannot be read has no template to render, so it is left out: a
   * mock that writes its name reports '@orderNo is not a placeholder this app
   * knows', and the settings page is where it is repaired.
   */
  const placeholders = useMemo<CustomPlaceholder[]>(
    () =>
      storedPlaceholders
        .filter((entry) => !entry.broken)
        .map((entry) => ({
          name: entry.name,
          template: entry.template,
          description: entry.description,
        })),
    [storedPlaceholders],
  )

  // Re-read once per window: the settings page may have written a placeholder
  // since the app started, and a mock of `@orderNo` would otherwise have to wait
  // for a restart to mean anything.
  useEffect(() => {
    refreshMockPlaceholders().catch(() => undefined)
  }, [refreshMockPlaceholders])

  // The ticking clock of a run in flight. Nothing else in the window wants a
  // timer, and the interval only exists while there is something to time.
  useEffect(() => {
    if (!running) return
    const id = window.setInterval(() => setElapsedTick((n) => n + 1), 1000)
    return () => window.clearInterval(id)
  }, [running])

  const rows = useMemo(
    () => fieldRowsOf(structure, overrides[currentKey], placeholders),
    [structure, overrides, currentKey, placeholders],
  )

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
      if (tables.length === 0) return [placeholder(`placeholder:${scope}`, t('dataGenPane.no-tables'))]
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
              ? [placeholder(`placeholder:${scope}`, t('dataGenPane.no-schemas'))]
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
            {canFill ? null : <Typography.Text type="secondary">{t('dataGenPane.cannot-insert')}</Typography.Text>}
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
            ? [placeholder(`placeholder:${entry.id}`, t('dataGenPane.no-databases'))]
            : databases.map((db) => dbNode(entry.id, db))
      }
      return node
    })
  }, [language, sessions, drivers, tree, flatNamespace])

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
    ? t('dataGenPane.this-engine-cannot-insert-rows')
    : readOnly
      ? t('dataGenPane.this-session-is-read-only')
      : !object
        ? t('dataGenPane.pick-a-table-on-the-left')
        : broken.length > 0
          ? t('dataGenPane.fix-the-mock-in', {
              names: broken.map((row) => row.name).join(', '),
            })
          : missing.length > 0
            ? t('dataGenPane.write-a-mock-for-or-untick', {
                names: missing.map((row) => row.name).join(', '),
              })
            : ticked.length === 0
              ? t('dataGenPane.no-field-is-ticked')
              : undefined

  /**
   * Writes the rows the confirmation asked for.
   *
   * It runs on its own rather than as the answer to the dialog: the dialog is
   * only there to be agreed to, and a run of a million rows would hold it open on
   * top of the progress bar for as long as the run lasts. Agreeing closes it, and
   * the run reports itself in the pane it was started from — where Stop is, and
   * where the result stays afterwards.
   */
  const runGeneration = useCallback(async () => {
    const rng = createRng()
    const total = rowsToWrite
    const columns = ticked.map((row) => row.name)
    // Time is taken here rather than asked of the backend: what the user waited
    // for is this loop, round trips and rendering included, and that is the
    // number a run of a hundred thousand rows is judged by.
    const started = Date.now()
    const elapsed = () => Date.now() - started
    stopRef.current = false
    setResult(undefined)
    setRunning(true)
    setStartedAt(started)
    setProgress({ done: 0, total, skipped: 0 })
    let inserted = 0
    let skipped = 0
    // The engine's own words for the first row it would not take. A run that
    // leaves rows out has to be able to say why, even when it finished.
    let skipReason: string | undefined
    try {
      for (let start = 0; start < total; start += INSERT_BATCH) {
        if (stopRef.current) {
          setResult({ kind: 'stopped', inserted, skipped, ms: elapsed(), skipReason })
          return
        }
        const size = Math.min(INSERT_BATCH, total - start)
        const batch: unknown[][] = []
        for (let i = 0; i < size; i += 1) {
          batch.push(
            ticked.map((row) => coerceMockValue(row.compiled.render(rng) as MockValue, row.kind)),
          )
        }
        const answer = await api.insertRows({
          sessionId,
          database,
          schema,
          object,
          columns,
          rows: batch,
          skipErrors,
        })
        inserted += answer.inserted
        skipped += answer.skipped ?? 0
        // The first refusal of the whole run, not of the batch in hand.
        if (!skipReason && answer.error) skipReason = answer.error
        if (answer.failed) {
          setResult({
            kind: 'failed',
            inserted,
            skipped,
            ms: elapsed(),
            row: start + answer.failed,
            error: answer.error ?? 'the engine refused the row',
          })
          return
        }
        setProgress({ done: start + size, total, skipped })
      }
      setResult({ kind: 'done', inserted, skipped, ms: elapsed(), skipReason })
    } catch (error) {
      setResult({ kind: 'error', inserted, skipped, ms: elapsed(), error: toMessage(error) })
    } finally {
      setRunning(false)
      if (inserted > 0) void loadObjects(sessionId, database, schema)
    }
  }, [rowsToWrite, skipErrors, ticked, sessionId, database, schema, object, loadObjects])

  const generate = useCallback(() => {
    if (blocker || !structure || !object) return
    void (async () => {
      // The statement the batches go in as, rendered by the engine that will run
      // them, and the same renderer the run uses — so the box below is what is
      // about to be sent rather than a second guess at it. The values are made up
      // while the run goes on, so what is shown is the statement's shape: the
      // table, the columns, and bind placeholders.
      let statement: string
      try {
        statement = await api.planInsertRows({
          sessionId,
          database,
          schema,
          object,
          columns: ticked.map((row) => row.name),
          rows: [],
        })
      } catch (error) {
        message.error(toMessage(error))
        return
      }
      modal.confirm({
        title: tn('dataGenPane.generate-rows-into', rowsToWrite, { object }),
        width: 660,
        icon: null,
        content: (
          <div style={{ fontSize: 12 }}>
            <div>
              {t('dataGenPane.n-of-m-columns-are-sent', {
                sent: ticked.length,
                total: structure.columns.length,
              })}{' '}
              <span className="mono">{ticked.map((row) => row.name).join(', ')}</span>
            </div>
            <SqlCode className="dm-ddl" driver={session?.driver} sql={statement + ';'} />
            <div style={{ marginTop: 6 }}>
              {t('dataGenPane.batches-no-undo', { batch: INSERT_BATCH })}
            </div>
            <div style={{ marginTop: 6 }}>
              {skipErrors
                ? t('dataGenPane.a-row-the-engine-refuses-is-left-out-and-counted')
                : t('dataGenPane.the-run-stops-at-the-first-row-the-engine')}
            </div>
          </div>
        ),
        okText: t('dataGenPane.generate'),
        // Closing here and running detached: the dialog is not what reports the run.
        onOk: () => {
          void runGeneration()
        },
      })
    })()
  }, [
    blocker,
    database,
    message,
    modal,
    object,
    rowsToWrite,
    runGeneration,
    schema,
    session?.driver,
    sessionId,
    skipErrors,
    structure,
    ticked,
  ])

  /* --- the fields grid --------------------------------------------------- */

  const columns: TableColumnsType<FieldRow> = useMemo(
    () => [
      {
        title: t('dataGenPane.field'),
        key: 'field',
        dataIndex: 'name',
        width: 200,
        render: (name: string, row) => (
          <Space size={6}>
            <span className="mono">{name}</span>
            {row.column.primaryKey ? (
              <Typography.Text type="secondary" style={{ fontSize: 11 }}>
                {t('dataGenPane.key')}
              </Typography.Text>
            ) : null}
          </Space>
        ),
      },
      {
        title: t('dataGenPane.type'),
        dataIndex: 'kind',
        width: 160,
        render: (_kind: ColumnKind, row) => (
          <span className="mono" style={{ fontSize: 12 }}>
            {row.column.columnType || row.column.dataType}
          </span>
        ),
      },
      {
        title: t('dataGenPane.mock'),
        dataIndex: 'template',
        width: 320,
        render: (_value: string, row) => (
          <Space.Compact style={{ width: '100%' }}>
            <Input
              size="small"
              className="mono"
              value={row.template}
              status={selectedOf(row.column) && row.compiled.error ? 'error' : undefined}
              placeholder={t('dataGenPane.no-mock')}
              onChange={(event) => setMock(row.name, event.target.value)}
            />
            <Tooltip title={t('dataGenPane.pick-a-placeholder')}>
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
        title: t('dataGenPane.description'),
        key: 'description',
        dataIndex: 'name',
        render: (_name: string, row) => (
          <DescriptionCell
            row={row}
            selected={selectedOf(row.column)}
            placeholders={placeholders}
          />
        ),
      },
    ],
    // The column headers and the mock hints are words, so a language change has
    // to rebuild the table rather than leave the old ones in place.
    [language, placeholders, selectedOf, setMock],
  )

  const grid = useColumnResize(columns)

  if (!insertable) {
    return (
      <div className="dm-pane">
        <Alert
          banner
          type="info"
          showIcon
          title={t('dataGenPane.this-engine-cannot-insert-generated-rows')}
          description={
            driver?.type === 'mongodb'
              ? t('dataGenPane.a-collection-has-no-column-list-so-there-is')
              : t('dataGenPane.the-driver-for-this-connection-does-not')
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
                {flatNamespace ? t('dataGenPane.connection-database-tables') : t('dataGenPane.connection-database-schema-tables')}
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
                {object ? <span className="mono">{object}</span> : t('dataGenPane.no-table-picked')}
              </Typography.Text>
              {readOnly ? (
                <Typography.Text type="warning" style={{ fontSize: 12 }}>
                  {t('dataGenPane.read-only')}
                </Typography.Text>
              ) : null}
              <div className="dm-toolbar-right">
                {/* The run settings, the two resets and Generate are one group, but
                    a window at its narrowest gets them on two lines rather than
                    clipped: the toolbar hides what does not fit, and Generate is
                    the one button that must not be the thing that falls off. */}
                <Space size={6} wrap>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    {t('dataGenPane.rows')}
                  </Typography.Text>
                  <InputNumber
                    size="small"
                    min={1}
                    max={maxRows}
                    value={rowsToWrite}
                    disabled={running}
                    onChange={(value) => setRowsToWrite(Math.max(1, Math.min(maxRows, Number(value ?? DEFAULT_ROWS))))}
                    style={{ width: 96 }}
                  />
                  <Tooltip title={t('dataGenPane.at-most-rows-per-run', { toLocaleString: maxRows.toLocaleString() })}>
                    <Typography.Text type="secondary" style={{ fontSize: 11 }}>
                      {t('dataGenPane.max')} {maxRows.toLocaleString()}
                    </Typography.Text>
                  </Tooltip>
                  <Tooltip
                    title={
                      skipErrors
                        ? t('dataGenPane.a-row-the-engine-refuses-is-left-out-and-counted-2')
                        : t('dataGenPane.the-run-stops-at-the-first-row-the-engine-2')
                    }
                  >
                    <Checkbox
                      checked={skipErrors}
                      disabled={running}
                      onChange={(event) => setSkipErrors(event.target.checked)}
                      style={{ fontSize: 12 }}
                    >
                      {t('dataGenPane.skip-bad-rows')}
                    </Checkbox>
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
                    {t('dataGenPane.reload')}
                  </Button>
                  <Tooltip title={t('dataGenPane.put-every-column-back-to-the-mock-this-table-s')}>
                    <Button size="small" disabled={!object} onClick={resetMocks}>
                      {t('dataGenPane.reset-mocks')}
                    </Button>
                  </Tooltip>
                  {running ? (
                    <Tooltip title={t('dataGenPane.finish-the-batch-in-flight-then-stop')}>
                      <Button size="small" danger onClick={() => (stopRef.current = true)}>
                        {t('dataGenPane.stop')}
                      </Button>
                    </Tooltip>
                  ) : (
                    <Tooltip title={blocker ?? t('dataGenPane.insert-the-generated-rows')}>
                      <Button
                        size="small"
                        type="primary"
                        icon={<ThunderboltOutlined />}
                        disabled={Boolean(blocker)}
                        onClick={generate}
                      >
                        {t('dataGenPane.generate')}
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
                title={t('dataGenPane.this-session-is-read-only')}
                description={t('dataGenPane.connect-without-the-read-only-flag-to-write-rows')}
              />
            ) : null}

            {structureError ? (
              <Alert
                banner
                type="error"
                showIcon
                title={t('dataGenPane.could-not-be-read', { object })}
                description={<span className="mono">{structureError}</span>}
              />
            ) : null}

            {progress && running ? (
              <div className="dm-datagen-progress">
                <div className="dm-datagen-progress-head">
                  <span>
                    {t('dataGenPane.inserted-done-of-total-rows', {
                      done: progress.done.toLocaleString(),
                      total: progress.total.toLocaleString(),
                    })}
                    {progress.skipped > 0
                      ? t('dataGenPane.skipped', { skipped: progress.skipped.toLocaleString() })
                      : ''}
                    {startedAt ? ` · ${elapsedText(Date.now() - startedAt)}` : ''}
                  </span>
                  <Tooltip title={t('dataGenPane.finish-the-batch-in-flight-then-stop')}>
                    <Button size="small" type="link" danger onClick={() => (stopRef.current = true)}>
                      {t('dataGenPane.stop')}
                    </Button>
                  </Tooltip>
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
                title={resultHeadline(result)}
                description={
                  result.kind === 'failed' || result.kind === 'error' ? (
                    <span className="mono" style={{ fontSize: 12, whiteSpace: 'pre-wrap' }}>
                      {result.error}
                    </span>
                  ) : result.skipReason ? (
                    <span className="mono" style={{ fontSize: 12, whiteSpace: 'pre-wrap' }}>
                      {t('dataGenPane.first-refusal')} {result.skipReason}
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
                title={t('dataGenPane.unknown-placeholder-in', {
                  names: broken.map((row) => row.name).join(', '),
                })}
                description={t('dataGenPane.a-mock-is-mock-js-syntax-an-name-this-app-does')}
              />
            ) : null}

            <div className="dm-pane-body">
              {!object ? (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={t('dataGenPane.pick-a-table-to-fill')}
                  style={{ marginTop: 60 }}
                />
              ) : loading && !structure ? (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={t('dataGenPane.reading-the-table')}
                  style={{ marginTop: 60 }}
                />
              ) : structure && structure.columns.length === 0 ? (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={t('dataGenPane.this-table-has-no-columns')}
                  style={{ marginTop: 60 }}
                />
              ) : structure ? (
                <Table
                  className={
                    grid.resized
                      ? 'dm-grid dm-datagen-grid dm-grid-resized'
                      : 'dm-grid dm-datagen-grid'
                  }
                  {...grid.tableProps}
                  size="small"
                  rowKey="name"
                  columns={grid.columns}
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
function DescriptionCell({
  row,
  selected,
  placeholders,
}: {
  row: FieldRow
  selected: boolean
  placeholders: readonly CustomPlaceholder[]
}) {
  // The sample is drawn from a source seeded by the field's name, so it holds
  // still while the user types in another row.
  const sample = useMemo(() => {
    if (!selected || row.compiled.error || row.template.trim() === '') return undefined
    return sampleOf(row.template, placeholders, seedOf(row.name)).value
  }, [row, selected, placeholders])

  return (
    <div className="dm-datagen-note">
      {row.column.comment ? <div>{row.column.comment}</div> : null}
      <div className={selected && row.compiled.error ? 'dm-datagen-bad' : undefined}>
        {selected
          ? mockDescription(row.column, row.template, row.compiled)
          : row.column.autoIncrement
            ? t('dataGenPane.auto-increment-the-engine-assigns-this-column')
            : t('dataGenPane.not-sent-this-column-is-left-out-of-the-insert')}
      </div>
      {sample ? <div className="dm-datagen-sample">{t('dataGenPane.e-g')} {sample}</div> : null}
    </div>
  )
}
