import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Alert,
  App as AntApp,
  Button,
  Checkbox,
  Empty,
  Input,
  Modal,
  Progress,
  Radio,
  Space,
  Typography,
} from 'antd'

import { api, onExportProgress, toMessage } from '../api/client'
import type {
  ExportFormat,
  ExportMode,
  ExportProgress,
  ExportResult,
  ExportTable,
} from '../api/types'
import { formatBytes, formatCount } from '../lib/format'
import { t, tn } from '../lib/i18n'

/** The database being exported, and what the menu item asked for. */
export interface ExportScope {
  sessionId: string
  database: string
  /** The schema to read, on an engine whose objects live under one. */
  schema?: string
  /** Objects live directly under the database (MySQL, SQLite). */
  flatNamespace: boolean
  /** The mode the menu item presets; the window can still change it. */
  mode: ExportMode
}

interface ExportModalProps {
  /** The database to export, or `null` when the window is closed. */
  scope: ExportScope | null
  onClose: () => void
}

/** Where the tables of a scope are read from, and how many are not tables. */
interface TableList {
  tables: ExportTable[]
  /** Views and other objects that are listed but cannot be exported. */
  skipped: number
}

/** Which key a table row is remembered under. */
function keyOf(table: ExportTable): string {
  return table.schema ? `${table.schema}.${table.name}` : table.name
}

/** The tables of one scope, which may mean walking every schema of it. */
async function collectTables(scope: ExportScope): Promise<TableList> {
  const read = async (schema: string): Promise<TableList> => {
    const objects = await api.listObjects(scope.sessionId, scope.database, schema)
    const tables: ExportTable[] = []
    let skipped = 0
    for (const object of objects) {
      // Views are left out on purpose: their definition is a query, and writing
      // one the way a table is written would produce a file that does not run.
      if (object.kind === 'table') tables.push({ schema, name: object.name })
      else skipped += 1
    }
    return { tables, skipped }
  }

  // An engine without schemas reads its objects straight from the database; an
  // engine with them is read from the one the tree was pointing at, or from all
  // of them when the menu was opened on the database itself.
  if (scope.flatNamespace || scope.schema !== undefined) {
    return read(scope.schema ?? scope.database)
  }

  const schemas = await api.listSchemas(scope.sessionId, scope.database)
  const lists = await Promise.all(schemas.map((schema) => read(schema)))
  return lists.reduce<TableList>(
    (all, list) => ({ tables: [...all.tables, ...list.tables], skipped: all.skipped + list.skipped }),
    { tables: [], skipped: 0 },
  )
}

/** What the save dialog offers, and what the file is called before it opens. */
function destination(format: ExportFormat, base: string): [string, { displayName: string; pattern: string }[]] {
  switch (format) {
    case 'csv':
      return [`${base}.csv`, [{ displayName: 'CSV', pattern: '*.csv' }]]
    case 'json':
      return [`${base}.json`, [{ displayName: 'JSON', pattern: '*.json' }]]
    case 'jsonl':
      return [`${base}.jsonl`, [{ displayName: 'JSON Lines', pattern: '*.jsonl' }]]
    default:
      return [`${base}.sql`, [{ displayName: 'SQL', pattern: '*.sql' }]]
  }
}

/**
 * Dumping a database, or part of one, to a file.
 *
 * The file is written by the backend, row by row, straight from the driver: a
 * whole database does not fit through the bridge, and a table read a page at a
 * time costs more the larger it gets. So this window never sees the data — it
 * decides what goes in, picks the destination, and then watches the run's
 * progress events and can stop it.
 *
 * The three modes are the three things a dump is ever wanted for: the shape of a
 * database, the shape plus its contents, or just one table's rows for a
 * spreadsheet. Only the last one can be written as anything but SQL, because
 * only it has a single table's worth of columns to line up — a CSV of twelve
 * tables has nowhere to say which table a line came from.
 */
export function ExportModal({ scope, onClose }: ExportModalProps) {
  const { message } = AntApp.useApp()
  const [mode, setMode] = useState<ExportMode>('structure-data')
  const [format, setFormat] = useState<ExportFormat>('sql')
  const [tables, setTables] = useState<ExportTable[]>([])
  const [skipped, setSkipped] = useState(0)
  const [selected, setSelected] = useState<string[]>([])
  const [search, setSearch] = useState('')
  const [fields, setFields] = useState<string[]>([])
  const [fieldList, setFieldList] = useState<string[]>([])
  const [path, setPath] = useState('')
  const [loading, setLoading] = useState(false)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [progress, setProgress] = useState<ExportProgress | null>(null)
  const [result, setResult] = useState<ExportResult | null>(null)
  /** The id of the run in flight, so its events can be told from another's. */
  const runId = useRef('')

  // Every opening starts from the same place: the mode the menu asked for, every
  // table unchecked, and nothing remembered from the last database.
  useEffect(() => {
    if (!scope) return
    setMode(scope.mode)
    setFormat('sql')
    setSelected([])
    setSearch('')
    setFields([])
    setFieldList([])
    setPath('')
    setBusy(false)
    setError(null)
    setProgress(null)
    setResult(null)
  }, [scope])

  useEffect(() => {
    if (!scope) return
    let cancelled = false
    setLoading(true)
    setLoadError(null)
    setTables([])
    setSkipped(0)
    collectTables(scope)
      .then((list) => {
        if (cancelled) return
        setTables(list.tables)
        setSkipped(list.skipped)
      })
      .catch((err) => {
        if (!cancelled) setLoadError(toMessage(err))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [scope])

  // One subscription for the life of the window: the events carry the run's id,
  // so an export started from another window is simply not this one's.
  useEffect(
    () =>
      onExportProgress((event) => {
        if (event.id === runId.current) setProgress(event)
      }),
    [],
  )

  const picked = useMemo(
    () => tables.filter((table) => selected.includes(keyOf(table))),
    [selected, tables],
  )

  // The fields come from the catalog rather than from a first row, because the
  // file has to name them before the rows arrive — a CSV header, a statement's
  // column list — and because the window has to be able to offer the ones the
  // user did not pick.
  const one = mode === 'data' && picked.length === 1 ? picked[0] : null
  useEffect(() => {
    if (!one || !scope) {
      setFields([])
      setFieldList([])
      return
    }
    let cancelled = false
    api
      .getStructure(scope.sessionId, scope.database, one.schema ?? '', one.name)
      .then((structure) => {
        if (cancelled) return
        const names = structure.columns.map((column) => column.name)
        setFieldList(names)
        // Every field is what "all of them" means, and it is the answer that
        // needs no second look before the button can be pressed.
        setFields(names)
      })
      .catch(() => {
        if (cancelled) return
        setFieldList([])
        setFields([])
      })
    return () => {
      cancelled = true
    }
  }, [one, scope])

  const visible = useMemo(() => {
    const needle = search.trim().toLowerCase()
    if (!needle) return tables
    return tables.filter((table) => keyOf(table).toLowerCase().includes(needle))
  }, [search, tables])

  const close = useCallback(() => {
    // A run is stopped before the window goes: leaving one writing to a file the
    // user can no longer see is worse than cutting it short.
    if (runId.current) {
      void api.cancelExport(runId.current).catch(() => undefined)
      runId.current = ''
    }
    setProgress(null)
    setResult(null)
    setError(null)
    setBusy(false)
    onClose()
  }, [onClose])

  const toggle = useCallback(
    (table: ExportTable) => {
      const key = keyOf(table)
      if (mode === 'data') {
        setSelected([key])
        return
      }
      setSelected((current) =>
        current.includes(key) ? current.filter((item) => item !== key) : [...current, key],
      )
    },
    [mode],
  )

  /** Picks a mode, keeping the number of tables a mode allows. */
  const chooseMode = useCallback((next: ExportMode) => {
    setMode(next)
    // One table's rows are one table's rows: a second one would have to be
    // written into the same file, which is what the format cannot say.
    if (next === 'data') setSelected((current) => current.slice(0, 1))
    if (next !== 'data') setFormat('sql')
  }, [])

  const stop = useCallback(() => {
    if (!runId.current) return
    void api.cancelExport(runId.current).catch(() => undefined)
  }, [])

  /**
   * Asks for the destination.
   *
   * The path is asked for before anything is written — the backend writes as it
   * reads, so there is no later moment to choose it — and an empty answer means
   * the user closed the dialog, which is not a failure.
   */
  const pickDestination = useCallback(async (): Promise<string> => {
    if (!scope) return ''
    const base = mode === 'data' && picked.length === 1 ? picked[0].name : scope.database
    const [suggested, filters] = destination(format, base)
    const chosen = await api.pickSavePath(suggested, filters)
    if (chosen) setPath(chosen)
    return chosen
  }, [format, mode, picked, scope])

  const run = useCallback(async () => {
    if (!scope || busy || picked.length === 0) return
    let target = path
    try {
      if (!target) target = await pickDestination()
    } catch (err) {
      setError(toMessage(err))
      return
    }
    if (!target) return

    const id = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    runId.current = id
    setBusy(true)
    setError(null)
    setResult(null)
    setProgress({ id, done: 0, total: picked.length, table: '', rows: 0, bytes: 0 })
    try {
      const done = await api.exportDatabase({
        id,
        sessionId: scope.sessionId,
        database: scope.database,
        mode,
        tables: picked,
        columns: mode === 'data' && fields.length < fieldList.length ? fields : undefined,
        format,
        path: target,
      })
      setResult(done)
      if (done.cancelled) message.warning(t('exportModal.export-stopped'))
      else if (done.warnings?.length) message.warning(t('exportModal.finished-with-warnings'))
      else message.success(t('exportModal.export-finished'))
    } catch (err) {
      setError(toMessage(err))
    } finally {
      runId.current = ''
      setBusy(false)
    }
  }, [busy, fieldList.length, fields, format, message, mode, path, pickDestination, picked, scope])

  const choose = useCallback(() => {
    void pickDestination().catch((err) => setError(toMessage(err)))
  }, [pickDestination])

  const percent = progress && progress.total > 0 ? Math.round((progress.done / progress.total) * 100) : 0
  const ready = picked.length > 0 && !busy && result === null

  const footer = result ? (
    <Space>
      <Button onClick={() => void api.revealInExplorer(result.path)}>{t('exportModal.show-in-folder')}</Button>
      <Button type="primary" onClick={close}>
        {t('exportModal.close')}
      </Button>
    </Space>
  ) : busy ? (
    <Space>
      <Typography.Text type="secondary">{t('exportModal.writing-the-file')}</Typography.Text>
      <Button danger onClick={stop}>
        {t('exportModal.stop')}
      </Button>
    </Space>
  ) : (
    <Space>
      <Button onClick={close}>{t('exportModal.cancel')}</Button>
      <Button type="primary" disabled={!ready} onClick={() => void run()}>
        {t('exportModal.export')}
      </Button>
    </Space>
  )

  return (
    <Modal
      open={scope !== null}
      title={scope ? t('exportModal.export-database-database', { database: scope.database }) : t('exportModal.export-database')}
      width={640}
      footer={footer}
      onCancel={close}
      maskClosable={false}
      destroyOnHidden
    >
      <Radio.Group
        className="dm-export-modes"
        value={mode}
        disabled={busy}
        onChange={(event) => chooseMode(event.target.value as ExportMode)}
      >
        <Radio value="structure">
          {t('exportModal.structure-only')}
          <Typography.Text type="secondary" className="dm-export-hint">
            {t('exportModal.the-fields-keys-and-indexes')}
          </Typography.Text>
        </Radio>
        <Radio value="structure-data">
          {t('exportModal.structure-and-data')}
          <Typography.Text type="secondary" className="dm-export-hint">
            {t('exportModal.structure-and-rows')}
          </Typography.Text>
        </Radio>
        <Radio value="data">
          {t('exportModal.data-only')}
          <Typography.Text type="secondary" className="dm-export-hint">
            {t('exportModal.one-tables-rows-only')}
          </Typography.Text>
        </Radio>
      </Radio.Group>

      <div className="dm-export-section">
        <div className="dm-export-head">
          <Typography.Text strong>
            {mode === 'data' ? t('exportModal.table') : t('exportModal.tables')}
          </Typography.Text>
          <Space size={4}>
            {mode === 'data' ? null : (
              <>
                <Button
                  type="link"
                  size="small"
                  disabled={busy || loading}
                  onClick={() => setSelected(visible.map(keyOf))}
                >
                  {t('exportModal.select-all')}
                </Button>
                <Button type="link" size="small" disabled={busy} onClick={() => setSelected([])}>
                  {t('exportModal.select-none')}
                </Button>
              </>
            )}
          </Space>
        </div>
        <Input
          allowClear
          size="small"
          placeholder={t('exportModal.search-tables')}
          value={search}
          disabled={busy || loading}
          onChange={(event) => setSearch(event.target.value)}
        />
        <div className="dm-export-list">
          {loading ? (
            <Typography.Text type="secondary">{t('exportModal.reading-the-object-list')}</Typography.Text>
          ) : loadError ? (
            <Typography.Text type="danger">{loadError}</Typography.Text>
          ) : visible.length === 0 ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('exportModal.no-tables')} />
          ) : mode === 'data' ? (
            <Radio.Group
              value={selected[0] ?? ''}
              disabled={busy}
              onChange={(event) => {
                const table = tables.find((item) => keyOf(item) === event.target.value)
                if (table) toggle(table)
              }}
            >
              {visible.map((table) => (
                <Radio key={keyOf(table)} value={keyOf(table)} className="dm-export-row">
                  {keyOf(table)}
                </Radio>
              ))}
            </Radio.Group>
          ) : (
            visible.map((table) => (
              <Checkbox
                key={keyOf(table)}
                className="dm-export-row"
                checked={selected.includes(keyOf(table))}
                disabled={busy}
                onChange={() => toggle(table)}
              >
                {keyOf(table)}
              </Checkbox>
            ))
          )}
        </div>
        {skipped > 0 && !loading ? (
          <Typography.Text type="secondary" className="dm-export-hint">
            {tn('exportModal.skipped-not-tables', skipped, { n: formatCount(skipped) })}
          </Typography.Text>
        ) : null}
      </div>

      {one && fieldList.length > 0 ? (
        <div className="dm-export-section">
          <div className="dm-export-head">
            <Typography.Text strong>{t('exportModal.fields')}</Typography.Text>
            <Space size={4}>
              <Button
                type="link"
                size="small"
                disabled={busy}
                onClick={() => setFields(fieldList)}
              >
                {t('exportModal.select-all')}
              </Button>
              <Button type="link" size="small" disabled={busy} onClick={() => setFields([])}>
                {t('exportModal.select-none')}
              </Button>
              <Typography.Text type="secondary" className="dm-export-hint">
                {tn('exportModal.fields-picked', fieldList.length, {
                  picked: formatCount(fields.length),
                  total: formatCount(fieldList.length),
                })}
              </Typography.Text>
            </Space>
          </div>
          <div className="dm-export-list dm-export-fields">
            {fieldList.map((name) => (
              <Checkbox
                key={name}
                className="dm-export-row"
                checked={fields.includes(name)}
                disabled={busy}
                onChange={() =>
                  // Ticking a field puts it back where the catalog has it rather
                  // than at the end: the statement's column list follows this
                  // order, and it should read like the table.
                  setFields((current) =>
                    current.includes(name)
                      ? current.filter((item) => item !== name)
                      : fieldList.filter((item) => item === name || current.includes(item)),
                  )
                }
              >
                {name}
              </Checkbox>
            ))}
          </div>
        </div>
      ) : null}

      <div className="dm-export-section">
        <div className="dm-export-head">
          <Typography.Text strong>{t('exportModal.format')}</Typography.Text>
        </div>
        <Radio.Group
          value={format}
          disabled={busy || mode !== 'data'}
          onChange={(event) => setFormat(event.target.value as ExportFormat)}
        >
          <Radio.Button value="sql">SQL</Radio.Button>
          <Radio.Button value="csv">CSV</Radio.Button>
          <Radio.Button value="json">JSON</Radio.Button>
          <Radio.Button value="jsonl">JSONL</Radio.Button>
        </Radio.Group>
        {mode === 'data' ? null : (
          <Typography.Text type="secondary" className="dm-export-hint">
            {t('exportModal.a-whole-database-is-written-as-sql')}
          </Typography.Text>
        )}
      </div>

      <div className="dm-export-section">
        <div className="dm-export-head">
          <Typography.Text strong>{t('exportModal.destination')}</Typography.Text>
          <Button size="small" disabled={busy} onClick={choose}>
            {t('exportModal.choose-file')}
          </Button>
        </div>
        <Typography.Text type="secondary" className="dm-export-path" ellipsis={{ tooltip: path }}>
          {path || t('exportModal.no-file-chosen-yet')}
        </Typography.Text>
      </div>

      {progress ? (
        <div className="dm-export-section">
          <Progress
            percent={percent}
            status={busy ? 'active' : result?.cancelled ? 'exception' : 'success'}
            showInfo={false}
          />
          <Typography.Text type="secondary" className="dm-export-hint">
            {tn('exportModal.tables-written', progress.total, {
              done: formatCount(progress.done),
              total: formatCount(progress.total),
            })}
            {' · '}
            {tn('exportModal.rows-written', progress.rows, { n: formatCount(progress.rows) })}
            {' · '}
            {formatBytes(progress.bytes)}
            {progress.table ? ` · ${progress.table}` : ''}
          </Typography.Text>
        </div>
      ) : null}

      {error ? (
        <Alert type="error" showIcon title={t('exportModal.the-export-failed')} description={error} />
      ) : null}

      {result ? (
        <Alert
          type={result.cancelled || result.warnings?.length ? 'warning' : 'success'}
          showIcon
          title={
            result.cancelled
              ? t('exportModal.export-stopped')
              : result.warnings?.length
                ? t('exportModal.finished-with-warnings')
                : t('exportModal.export-finished')
          }
          description={
            <>
              <div>
                {tn('exportModal.n-tables-written', result.tables, { n: formatCount(result.tables) })}
                {' · '}
                {tn('exportModal.n-rows-written', result.rows, { n: formatCount(result.rows) })}
                {' · '}
                {formatBytes(result.bytes)}
              </div>
              <div>{result.path}</div>
              {result.warnings?.length ? (
                <ul className="dm-export-warnings">
                  {result.warnings.map((warning, index) => (
                    <li key={index}>{warning}</li>
                  ))}
                </ul>
              ) : null}
            </>
          }
        />
      ) : null}
    </Modal>
  )
}
