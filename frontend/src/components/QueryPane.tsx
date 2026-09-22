import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Alert,
  App as AntApp,
  Button,
  Dropdown,
  InputNumber,
  Segmented,
  Select,
  Space,
  Splitter,
  Tooltip,
  Typography,
} from 'antd'
import type { MenuProps } from 'antd'
import type { EditorView } from '@codemirror/view'
import {
  AlignLeftOutlined,
  ClearOutlined,
  CopyOutlined,
  DownloadOutlined,
  FileTextOutlined,
  HistoryOutlined,
  LockOutlined,
  NodeIndexOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  SaveOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type { DriverType, ExplainResult, QueryResult } from '../api/types'
import { capabilitiesOf, findDriver } from '../lib/capabilities'
import { downloadText, resultToCSV, resultToJSON, toInsertScript } from '../lib/export'
import { formatDuration } from '../lib/format'
import { formatSql, formatterDialect } from '../lib/sqlFormat'
import { catalogOf, objectsKey } from '../lib/tree'
import { useAppStore, type WorkspaceTab } from '../store/appStore'
import { DataGrid } from './DataGrid'
import { QueryFavorites } from './QueryFavorites'
import { SqlEditor } from './SqlEditor'

const MAX_HISTORY = 25

interface QueryPaneProps {
  tab: WorkspaceTab
}

/**
 * SQL scratchpad for one tab: editor on top, results below.
 *
 * Two shapes share the pane. A scratchpad tab (`tab.queryFile` unset) holds text
 * that nothing outside the window can see. A tab bound to a saved script edits
 * that file: its text is read from it when the window opens, Save writes it back,
 * and the tab is marked dirty until it has.
 */
export function QueryPane({ tab }: QueryPaneProps) {
  const session = useAppStore((s) => s.sessionOf(tab.sessionId))
  const drivers = useAppStore((s) => s.drivers)
  const theme = useAppStore((s) => s.resolvedTheme)
  const treeObjects = useAppStore((s) => s.tree.objects)
  const treeSchemas = useAppStore((s) => s.tree.schemas)
  const loadObjects = useAppStore((s) => s.loadObjects)
  const saveQueryFile = useAppStore((s) => s.saveQueryFile)
  const setTabDirty = useAppStore((s) => s.setTabDirty)
  const { message } = AntApp.useApp()

  const driver: DriverType | undefined = session?.driver
  const driverInfo = findDriver(drivers, driver)
  const capabilities = capabilitiesOf(driverInfo)
  // The formatter is the one place that needs a *grammar* rather than a
  // capability, so the button asks for the grammar (see lib/sqlFormat.ts).
  const canFormat = formatterDialect(driver) !== undefined

  const [sql, setSql] = useState('')
  const [running, setRunning] = useState(false)
  const [result, setResult] = useState<QueryResult | null>(null)
  /** The plan of the last statement that was explained, if any. */
  const [plan, setPlan] = useState<ExplainResult | null>(null)
  /** Which of the two answers the lower half shows. */
  const [answer, setAnswer] = useState<'results' | 'plan'>('results')
  const [explaining, setExplaining] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [maxRows, setMaxRows] = useState(1000)
  const [timeoutMs, setTimeoutMs] = useState(60000)
  const [history, setHistory] = useState<string[]>([])
  const [saving, setSaving] = useState(false)
  /** Why the saved script could not be opened, if it could not. */
  const [fileError, setFileError] = useState<string | null>(null)
  const viewRef = useRef<EditorView | null>(null)

  const database = tab.database ?? session?.database
  const file = tab.queryFile

  /**
   * The namespace whose objects the editor completes table names from.
   *
   * Where the caller knew the schema the window was opened from, that schema is
   * the namespace; on an engine whose namespaces *are* its databases, the
   * database is. A window that knows only its database (opened from the database
   * node, from a saved script, or from the tab strip on PostgreSQL) leaves this
   * out and completes from whatever the explorer has already loaded.
   */
  const scopeSchema = tab.schema ?? (driverInfo?.supportsSchema ? undefined : database)
  const catalog = useMemo(
    () =>
      catalogOf(
        { objects: treeObjects, schemas: treeSchemas },
        { sessionId: tab.sessionId, database, schema: scopeSchema },
        Boolean(driverInfo?.supportsSchema),
      ),
    [database, driverInfo?.supportsSchema, scopeSchema, tab.sessionId, treeObjects, treeSchemas],
  )

  // A window that knows its namespace asks for its object list the way the tree
  // does when a folder is expanded: what the editor completes should not depend
  // on somebody having opened the folder first. A window that knows only its
  // database asks for nothing — there is no single namespace to name there, and
  // `catalogOf` says why.
  useEffect(() => {
    if (!database || !scopeSchema) return
    if (treeObjects[objectsKey(tab.sessionId, database, scopeSchema)]) return
    void loadObjects(tab.sessionId, database, scopeSchema)
  }, [database, loadObjects, scopeSchema, tab.sessionId, treeObjects])

  /**
   * The window whose file has already been read into this pane.
   *
   * A rename moves the window onto a new name without touching the text in it,
   * and reading again then would throw away edits that are not saved. Keying on
   * the window (its id survives a rename) makes the read happen once per window,
   * which is what a window over a file means; a window that is closed and opened
   * again is a new pane and reads afresh.
   */
  const readFor = useRef<string | null>(null)

  // A bound window opens onto its file. Reading it here rather than in the tree
  // is what makes the file the truth: closing the tab and reopening it (or a
  // restart) shows what is on disk, including an edit made in another editor.
  useEffect(() => {
    if (!file) return
    if (readFor.current === tab.id) return
    readFor.current = tab.id
    let cancelled = false
    api
      .readQueryFile(file.connectionId, file.database, file.name)
      .then((read) => {
        if (cancelled) return
        setSql(read.sql ?? '')
        setFileError(null)
        setTabDirty(tab.id, false)
      })
      .catch((err) => {
        if (cancelled) return
        // The window stays open — whatever the user had typed is still in it —
        // but it says that there is no file behind it, because a Save would
        // otherwise look like it had worked.
        setFileError(toMessage(err))
      })
    return () => {
      cancelled = true
    }
  }, [file, setTabDirty, tab.id])

  const changeSql = useCallback(
    (next: string) => {
      setSql(next)
      if (file) setTabDirty(tab.id, true)
    },
    [file, setTabDirty, tab.id],
  )

  const save = useCallback(async () => {
    if (!file) return
    setSaving(true)
    try {
      await saveQueryFile({
        connectionId: file.connectionId,
        database: file.database,
        name: file.name,
        sql,
      })
      setFileError(null)
      setTabDirty(tab.id, false)
      message.success(`Saved ${file.name}`)
    } catch (err) {
      message.error(toMessage(err))
    } finally {
      setSaving(false)
    }
  }, [file, message, saveQueryFile, setTabDirty, sql, tab.id])

  // The only difference between the two script exports is the language, and
  // the label and the file extension are the only places that shows.
  const scriptExport = useMemo(
    () =>
      driver === 'mongodb'
        ? { menuLabel: 'Export insertMany script', label: 'JavaScript', extension: 'js' }
        : { menuLabel: 'Export INSERT statements', label: 'SQL', extension: 'sql' },
    [driver],
  )

  const execute = useCallback(
    async (statement: string) => {
      const text = statement.trim()
      if (!text) {
        message.info('Nothing to run')
        return
      }
      setRunning(true)
      setError(null)
      setAnswer('results')
      try {
        const res = await api.executeSql({
          sessionId: tab.sessionId,
          database,
          sql: text,
          maxRows,
          timeoutMs,
        })
        setResult(res)
        setHistory((previous) => {
          const next = [text, ...previous.filter((entry) => entry !== text)]
          return next.slice(0, MAX_HISTORY)
        })
      } catch (err) {
        setError(toMessage(err))
        setResult(null)
      } finally {
        setRunning(false)
      }
    },
    [database, maxRows, message, tab.sessionId, timeoutMs],
  )

  const runAll = useCallback(() => {
    void execute(sql)
  }, [execute, sql])

  const runSelection = useCallback(() => {
    const view = viewRef.current
    if (!view) {
      void execute(sql)
      return
    }
    const { from, to } = view.state.selection.main
    const selected = view.state.sliceDoc(from, to)
    void execute(selected.trim() ? selected : sql)
  }, [execute, sql])

  /**
   * What the editor would run: the selection when there is one, the whole
   * script otherwise. Explaining and formatting both act on the same text the
   * Run buttons would, so the three can never disagree about what is "current".
   */
  const targetText = useCallback(() => {
    const view = viewRef.current
    if (!view) return sql
    const { from, to } = view.state.selection.main
    const selected = view.state.sliceDoc(from, to)
    return selected.trim() ? selected : sql
  }, [sql])

  /**
   * Reads the plan of the current statement without running it.
   *
   * The engine is asked, not the user's text: a plan describes one statement,
   * and the service refuses a script rather than silently picking one out of it.
   */
  const explain = useCallback(async () => {
    const text = targetText().trim()
    if (!text) {
      message.info('Nothing to explain')
      return
    }
    setExplaining(true)
    setError(null)
    try {
      const res = await api.explainSql({
        sessionId: tab.sessionId,
        database,
        sql: text,
        timeoutMs,
      })
      setPlan(res)
      setAnswer('plan')
    } catch (err) {
      // A statement the engine cannot plan (a typo, a missing table) is the
      // same kind of failure as one it cannot run, and is reported in the same
      // place — the results half, which is where the eye already is.
      setError(toMessage(err))
      setPlan(null)
      setAnswer('results')
    } finally {
      setExplaining(false)
    }
  }, [database, message, tab.sessionId, targetText, timeoutMs])

  /**
   * Re-indents the current statement in place.
   *
   * A selection is replaced where it stands (a long script is formatted piece
   * by piece, which is why the selection is what gets formatted when there is
   * one); otherwise the whole text is replaced. A statement the grammar cannot
   * parse leaves the editor untouched and says why.
   */
  const reformat = useCallback(() => {
    const view = viewRef.current
    const whole = !view || !view.state.sliceDoc(view.state.selection.main.from, view.state.selection.main.to).trim()
    const text = whole ? sql : targetText()
    if (!text.trim()) {
      message.info('Nothing to format')
      return
    }

    const outcome = formatSql(text, driver)
    if (outcome.unsupported) {
      message.info('This engine\u2019s statements are not SQL, so there is no SQL formatting for them')
      return
    }
    if (outcome.error) {
      message.error(`Could not format: ${outcome.error}`)
      return
    }
    const formatted = outcome.formatted ?? text
    if (formatted === text) return

    if (!view || whole) {
      changeSql(formatted)
      return
    }
    const { from, to } = view.state.selection.main
    view.dispatch({ changes: { from, to, insert: formatted } })
  }, [changeSql, driver, message, sql, targetText])

  const exportResult = useCallback(
    async (format: 'csv' | 'json' | 'sql') => {
      if (!result || result.rows.length === 0) {
        message.info('There is nothing to export')
        return
      }
      const stamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
      const content =
        format === 'csv'
          ? resultToCSV(result)
          : format === 'json'
            ? resultToJSON(result)
            : toInsertScript(
                {
                  driver: driver ?? '',
                  database,
                  object: tab.object ?? 'exported',
                },
                result.columns,
                result.rows,
              )
      // A document store exports shell, not SQL, so the file says so.
      const extension = format === 'sql' ? scriptExport.extension : format
      const filename = `result-${stamp}.${extension}`
      try {
        await api.saveTextFile({
          defaultFilename: filename,
          content,
          filters: [
            {
              displayName: format === 'sql' ? scriptExport.label : format.toUpperCase(),
              pattern: `*.${extension}`,
            },
          ],
        })
        message.success('Export written')
      } catch (err) {
        const text = toMessage(err)
        if (/cancel/i.test(text)) return
        // Fall back to a browser download when the native dialog is unavailable.
        downloadText(filename, content, 'text/plain')
      }
    },
    [database, driver, message, result, scriptExport, tab.object],
  )

  const copyCSV = useCallback(async () => {
    if (!result) return
    try {
      await navigator.clipboard.writeText(resultToCSV(result))
      message.success('Copied result as CSV')
    } catch (err) {
      message.error(toMessage(err))
    }
  }, [message, result])

  const historyMenu: MenuProps = useMemo(
    () => ({
      items:
        history.length === 0
          ? [{ key: 'empty', label: 'No statements yet', disabled: true }]
          : history.map((entry, index) => ({
              key: String(index),
              label: (
                <span className="mono" style={{ maxWidth: 420, display: 'inline-block' }}>
                  {entry.replace(/\s+/g, ' ').slice(0, 120)}
                </span>
              ),
              onClick: () => changeSql(entry),
            })),
    }),
    [changeSql, history],
  )

  /**
   * The plan, dressed as a result set.
   *
   * Every engine describes its plan differently and the driver is the one that
   * knows how (see internal/drivers/sqlbase/explain.go), so by the time it gets
   * here it is already columns and rows: the grid needs no new code and a plan
   * can be copied out exactly like a result.
   */
  const planResult: QueryResult | null = useMemo(
    () =>
      plan && {
        columns: plan.columns,
        rows: plan.rows,
        rowCount: plan.rows.length,
        affectedRows: 0,
        lastInsertId: 0,
        durationMs: plan.durationMs,
        truncated: plan.truncated,
        hasResultSet: plan.rows.length > 0,
        sql: plan.statement,
        statementIndex: 0,
        statementCount: 1,
      },
    [plan],
  )

  const exportMenu: MenuProps = {
    items: [
      { key: 'csv', label: 'Export CSV', onClick: () => void exportResult('csv') },
      { key: 'json', label: 'Export JSON', onClick: () => void exportResult('json') },
      {
        key: 'sql',
        label: scriptExport.menuLabel,
        onClick: () => void exportResult('sql'),
      },
    ],
  }

  return (
    <div className="dm-pane">
      <div className="dm-editor-toolbar">
        <Button
          type="primary"
          size="small"
          icon={<PlayCircleOutlined />}
          loading={running}
          onClick={runAll}
        >
          Run
        </Button>
        <Tooltip title="Ctrl/Cmd+Shift+Enter">
          <Button size="small" icon={<ThunderboltOutlined />} onClick={runSelection}>
            Run selection
          </Button>
        </Tooltip>
        <Tooltip
          title={
            capabilities.explainable
              ? 'Plan the current statement without running it'
              : `${driverInfo?.displayName ?? 'This engine'} has no plan to read`
          }
        >
          {/* A disabled antd button swallows the hover event, so the tooltip
              that explains *why* it is disabled needs a wrapper to hang on. */}
          <span>
            <Button
              size="small"
              icon={<NodeIndexOutlined />}
              disabled={!capabilities.explainable}
              loading={explaining}
              onClick={() => void explain()}
            >
              Explain
            </Button>
          </span>
        </Tooltip>
        <Dropdown menu={historyMenu} trigger={['click']}>
          <Button size="small" icon={<HistoryOutlined />}>
            History
          </Button>
        </Dropdown>
        <QueryFavorites
          sql={sql}
          driver={driver}
          database={database}
          onLoad={changeSql}
        />
        <Tooltip
          title={
            !canFormat
              ? 'This engine\u2019s statements are not SQL'
              : 'Re-indent the selection, or the whole script (Ctrl/Cmd+Shift+F)'
          }
        >
          <span>
            <Button
              size="small"
              icon={<AlignLeftOutlined />}
              disabled={!canFormat}
              onClick={reformat}
            >
              Format
            </Button>
          </span>
        </Tooltip>
        <Tooltip title="Clear editor">
          <Button size="small" icon={<ClearOutlined />} onClick={() => setSql('')} />
        </Tooltip>
        {file ? (
          <Tooltip title="Ctrl/Cmd+S">
            <Button
              size="small"
              icon={tab.dirty ? <FileTextOutlined /> : <SaveOutlined />}
              loading={saving}
              onClick={() => void save()}
            >
              {tab.dirty ? 'Save *' : 'Save'}
            </Button>
          </Tooltip>
        ) : null}

        <span style={{ opacity: 0.35 }}>|</span>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          max rows
        </Typography.Text>
        <InputNumber
          size="small"
          min={1}
          max={50000}
          value={maxRows}
          style={{ width: 88 }}
          onChange={(value) => setMaxRows(value ?? 1000)}
        />
        <Select
          size="small"
          value={timeoutMs}
          style={{ width: 110 }}
          onChange={setTimeoutMs}
          options={[
            { value: 15000, label: '15s timeout' },
            { value: 60000, label: '60s timeout' },
            { value: 300000, label: '5m timeout' },
            { value: 900000, label: '15m timeout' },
          ]}
        />

        <div className="dm-toolbar-right">
          {session?.readOnly ? (
            <Tooltip title="Write statements are rejected on this session">
              <Typography.Text type="warning" style={{ fontSize: 12 }}>
                <LockOutlined /> read-only
              </Typography.Text>
            </Tooltip>
          ) : null}
          {database ? (
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {database}
            </Typography.Text>
          ) : null}
          <Tooltip title="Copy result as CSV">
            <Button size="small" icon={<CopyOutlined />} disabled={!result} onClick={() => void copyCSV()} />
          </Tooltip>
          <Dropdown menu={exportMenu} trigger={['click']} disabled={!result}>
            <Button size="small" icon={<DownloadOutlined />}>
              Export
            </Button>
          </Dropdown>
        </div>
      </div>

      {file && fileError ? (
        <Alert
          type="warning"
          showIcon
          banner
          title={`${file.name} could not be read`}
          description={
            <span className="mono" style={{ fontSize: 12 }}>
              {fileError}
            </span>
          }
        />
      ) : null}

      <Splitter orientation="vertical" style={{ flex: '1 1 auto', minHeight: 0 }}>
        <Splitter.Panel defaultSize="42%" min={100}>
          <SqlEditor
            value={sql}
            driver={driver}
            theme={theme}
            height="100%"
            catalog={catalog}
            onChange={changeSql}
            onRun={runAll}
            onRunSelection={runSelection}
            onSave={file ? () => void save() : undefined}
            onFormat={canFormat ? reformat : undefined}
            onReady={(view) => {
              viewRef.current = view
            }}
          />
        </Splitter.Panel>

        <Splitter.Panel min={120}>
          <div className="dm-pane">
            {error ? (
              <div style={{ padding: 10, flex: '0 0 auto' }}>
                <Alert
                  type="error"
                  showIcon
                  title="Statement failed"
                  description={
                    <span className="mono" style={{ fontSize: 12, whiteSpace: 'pre-wrap' }}>
                      {error}
                    </span>
                  }
                  closable
                  onClose={() => setError(null)}
                />
              </div>
            ) : null}

            {capabilities.explainable ? (
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                  padding: '6px 10px',
                  flex: '0 0 auto',
                }}
              >
                <Segmented
                  size="small"
                  value={answer}
                  options={[
                    { label: 'Results', value: 'results' },
                    { label: 'Plan', value: 'plan' },
                  ]}
                  onChange={(value) => setAnswer(value as 'results' | 'plan')}
                />
                {answer === 'plan' && plan ? (
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    estimated, not measured
                  </Typography.Text>
                ) : null}
              </div>
            ) : null}

            {answer === 'results' && result && !result.hasResultSet ? (
              <div style={{ padding: 12, flex: '0 0 auto' }}>
                <Alert
                  type="success"
                  showIcon
                  title={`${result.affectedRows} row(s) affected in ${formatDuration(result.durationMs)}`}
                />
              </div>
            ) : null}

            {answer === 'results' && result?.truncated ? (
              <div style={{ padding: '0 12px 8px', flex: '0 0 auto' }}>
                <Alert
                  type="warning"
                  showIcon
                  title={`Result truncated at ${result.rowCount.toLocaleString()} rows (max rows = ${maxRows})`}
                />
              </div>
            ) : null}

            {answer === 'results' && result?.hasResultSet ? (
              <DataGrid result={result} loading={running} primaryKey={undefined} />
            ) : null}

            {answer === 'results' && !result?.hasResultSet ? (
              <div
                className="dm-pane-body"
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  flexDirection: 'column',
                  gap: 8,
                  opacity: 0.55,
                }}
              >
                {driverInfo ? (
                  <>
                    <Typography.Text type="secondary">
                      {running ? 'Running…' : 'Run a statement to see results here.'}
                    </Typography.Text>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      Ctrl/Cmd+Enter runs everything, Ctrl/Cmd+Shift+Enter runs the selection.
                    </Typography.Text>
                  </>
                ) : (
                  <Typography.Text type="danger">
                    This session is no longer open.
                    <Button
                      type="link"
                      size="small"
                      icon={<ReloadOutlined />}
                      onClick={() => window.location.reload()}
                    >
                      Reload
                    </Button>
                  </Typography.Text>
                )}
              </div>
            ) : null}

            {answer === 'results' && result?.messages && result.messages.length > 0 ? (
              <div className="dm-messages">
                {result.messages.map((entry, index) => (
                  <div key={index}>{entry}</div>
                ))}
              </div>
            ) : null}

            {answer === 'results' && result?.hasResultSet ? (
              <div
                className="dm-statusbar"
                style={{ borderTop: '1px solid var(--dm-border)', background: 'transparent' }}
              >
                <span className="dm-statusbar-item">
                  {result.rowCount.toLocaleString()} row(s)
                </span>
                <span className="dm-statusbar-item">{formatDuration(result.durationMs)}</span>
                {result.statementCount > 1 ? (
                  <span className="dm-statusbar-item">
                    statement {result.statementIndex + 1} of {result.statementCount}
                  </span>
                ) : null}
                <span className="dm-spacer" />
                <Space size={4} />
              </div>
            ) : null}

            {answer === 'plan' ? (
              plan && planResult ? (
                <>
                  <div style={{ padding: '8px 12px 0', flex: '0 0 auto' }}>
                    {/* The text that was sent, verbatim: the wrapper is the
                        engine's own and hiding it would make the plan below
                        impossible to reproduce by hand. */}
                    <div
                      className="mono"
                      style={{ fontSize: 12, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}
                    >
                      {plan.statement}
                    </div>
                    {plan.notes?.length ? (
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        {plan.notes.join(' ')}
                      </Typography.Text>
                    ) : null}
                  </div>

                  {plan.rows.length > 0 ? (
                    <>
                      <DataGrid
                        result={planResult}
                        loading={explaining}
                        primaryKey={undefined}
                      />
                      <div
                        className="dm-statusbar"
                        style={{
                          borderTop: '1px solid var(--dm-border)',
                          background: 'transparent',
                        }}
                      >
                        <span className="dm-statusbar-item">
                          {plan.rows.length.toLocaleString()} plan step(s)
                        </span>
                        <span className="dm-statusbar-item">{formatDuration(plan.durationMs)}</span>
                        <span className="dm-spacer" />
                        <Space size={4} />
                      </div>
                    </>
                  ) : (
                    <div
                      className="dm-pane-body"
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        flex: '1 1 auto',
                      }}
                    >
                      <Typography.Text type="secondary">
                        The engine returned no plan steps for this statement.
                      </Typography.Text>
                    </div>
                  )}
                </>
              ) : (
                <div
                  className="dm-pane-body"
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    flexDirection: 'column',
                    gap: 8,
                    opacity: 0.55,
                    flex: '1 1 auto',
                  }}
                >
                  <Typography.Text type="secondary">
                    Press Explain to see how the engine would run the current statement.
                  </Typography.Text>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    Nothing is executed. Explaining a statement that writes is safe.
                  </Typography.Text>
                </div>
              )
            ) : null}
          </div>
        </Splitter.Panel>
      </Splitter>
    </div>
  )
}
