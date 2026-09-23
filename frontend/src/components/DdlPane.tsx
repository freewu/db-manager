import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Alert,
  App as AntApp,
  Button,
  Space,
  Splitter,
  Tag,
  Tooltip,
  Typography,
} from 'antd'
import type { EditorView } from '@codemirror/view'
import {
  ClearOutlined,
  CopyOutlined,
  ExclamationCircleOutlined,
  FileTextOutlined,
  LockOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  SaveOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type { DriverType, QueryResult, ScriptAnalysis } from '../api/types'
import { formatDuration, qualifiedName, quoteIdent } from '../lib/format'
import { isMySQLFamily } from '../lib/sqlFlavor'
import { useAppStore, type WorkspaceTab } from '../store/appStore'
import { DataGrid } from './DataGrid'
import { SqlEditor } from './SqlEditor'

const KIND_COLOR: Record<string, string> = {
  query: 'blue',
  ddl: 'geekblue',
  dml: 'orange',
  unknown: 'default',
}

/** Seconds to wait after the last keystroke before re-analysing the script. */
const ANALYZE_DEBOUNCE_MS = 450

interface DdlPaneProps {
  tab: WorkspaceTab
}

/**
 * DDL editor window.
 *
 * It is a plain script runner with two differences from the query pad: it opens
 * with the object's live definition (or a template for a new object) and it
 * shows a dry run — statement kinds, destructive markers and what a read-only
 * connection would refuse — next to the editor. The dry run is a heuristic
 * computed by the backend without contacting the server, so it never blocks a
 * statement, it only asks for confirmation.
 */
export function DdlPane({ tab }: DdlPaneProps) {
  const session = useAppStore((s) => s.sessionOf(tab.sessionId))
  const theme = useAppStore((s) => s.resolvedTheme)
  const { message, modal } = AntApp.useApp()

  const driver: DriverType | undefined = session?.driver
  const database = tab.database ?? session?.database ?? ''
  const schema = tab.schema ?? ''
  const object = tab.object ?? ''
  const readOnly = Boolean(session?.readOnly)

  const [sql, setSql] = useState('')
  const [analysis, setAnalysis] = useState<ScriptAnalysis | null>(null)
  const [loadingDefinition, setLoadingDefinition] = useState(Boolean(object))
  const [definitionError, setDefinitionError] = useState<string | null>(null)
  const [running, setRunning] = useState(false)
  const [result, setResult] = useState<QueryResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  // Mirrors `sql` so the initial definition load does not overwrite typing.
  const loaded = useRef(false)
  const viewRef = useRef<EditorView | null>(null)

  /** Pulls the object's live definition into the editor. */
  const loadDefinition = useCallback(
    async (replace: boolean) => {
      if (!object) {
        if (replace) setSql(templateFor(driver, database, schema))
        return
      }
      setLoadingDefinition(true)
      setDefinitionError(null)
      try {
        const structure = await api.getStructure(tab.sessionId, database, schema, object)
        if (replace || !loaded.current) {
          setSql(structure.ddl)
          loaded.current = true
        }
      } catch (err) {
        setDefinitionError(toMessage(err))
      } finally {
        setLoadingDefinition(false)
      }
    },
    [database, driver, object, schema, tab.sessionId],
  )

  useEffect(() => {
    loaded.current = false
    void loadDefinition(true)
    // Only re-run when the window is pointed at another object.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab.id])

  // Debounced dry run: cheap, backend-side, never contacts the server.
  useEffect(() => {
    const text = sql.trim()
    if (!text) {
      setAnalysis(null)
      return
    }
    const timer = window.setTimeout(() => {
      api
        .analyzeSql(tab.sessionId, text)
        .then(setAnalysis)
        .catch(() => setAnalysis(null))
    }, ANALYZE_DEBOUNCE_MS)
    return () => window.clearTimeout(timer)
  }, [sql, tab.sessionId])

  const statements = analysis?.statements ?? []

  const run = useCallback(
    (script: string) => {
      const text = script.trim()
      if (!text) {
        message.info('Nothing to run')
        return
      }
      const start = async () => {
        setRunning(true)
        setError(null)
        try {
          const res = await api.executeSql({
            sessionId: tab.sessionId,
            database,
            // The window knows which object it opened, and the change log
            // records it: the statements below are written against this table
            // even when their own text does not say so.
            schema: schema || undefined,
            object: object || undefined,
            sql: text,
            maxRows: 2000,
            timeoutMs: 300000,
          })
          setResult(res)
          message.success(
            res.hasResultSet
              ? `${res.rowCount} row(s) returned`
              : `${res.affectedRows} row(s) affected`,
          )
          // The catalog may have moved: refresh the tree and the open windows.
          const state = useAppStore.getState()
          void state.loadObjects(tab.sessionId, database, schema)
          void state.loadIndexes(tab.sessionId, database, schema)
        } catch (err) {
          setError(toMessage(err))
          setResult(null)
        } finally {
          setRunning(false)
        }
      }

      // Only a destructive script interrupts; everything else runs on click.
      if (analysis?.destructive && text === sql.trim()) {
        const reasons = statements.filter((entry) => entry.destructive)
        modal.confirm({
          title: 'Run a script that destroys data or schema?',
          okText: 'Run anyway',
          okButtonProps: { danger: true },
          width: 620,
          content: (
            <div>
              <ul style={{ paddingInlineStart: 18, margin: '8px 0' }}>
                {reasons.map((entry) => (
                  <li key={entry.index}>
                    <Typography.Text code className="mono" style={{ fontSize: 12 }}>
                      {entry.preview.slice(0, 80)}
                    </Typography.Text>
                    <div>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        {entry.reason}
                      </Typography.Text>
                    </div>
                  </li>
                ))}
              </ul>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                DDL cannot be rolled back on every engine.
              </Typography.Text>
            </div>
          ),
          onOk: start,
        })
        return
      }
      void start()
    },
    [analysis, database, message, modal, schema, sql, statements, tab.sessionId],
  )

  const runAll = useCallback(() => run(sql), [run, sql])

  const runSelection = useCallback(() => {
    const view = viewRef.current
    if (!view) {
      run(sql)
      return
    }
    const { from, to } = view.state.selection.main
    const selected = view.state.sliceDoc(from, to)
    run(selected.trim() ? selected : sql)
  }, [run, sql])

  const copy = useCallback(
    async (text: string, what: string) => {
      try {
        await navigator.clipboard.writeText(text)
        message.success(`${what} copied`)
      } catch (err) {
        message.error(toMessage(err))
      }
    },
    [message],
  )

  const save = useCallback(async () => {
    if (!sql.trim()) {
      message.info('Nothing to save')
      return
    }
    const name = object ? `${object}.sql` : 'script.sql'
    try {
      await api.saveTextFile({
        defaultFilename: name,
        content: sql,
        filters: [{ displayName: 'SQL', pattern: '*.sql' }],
      })
      message.success('Script saved')
    } catch (err) {
      message.error(toMessage(err))
    }
  }, [message, object, sql])

  const warnings = useMemo(
    () => analysis?.warnings ?? [],
    [analysis],
  )

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
          Run script
        </Button>
        <Tooltip title="Run the selected statements only">
          <Button size="small" icon={<ThunderboltOutlined />} onClick={runSelection}>
            Run selection
          </Button>
        </Tooltip>
        {object ? (
          <Tooltip title="Replace the editor with the object's current definition">
            <Button
              size="small"
              icon={<ReloadOutlined />}
              loading={loadingDefinition}
              onClick={() => void loadDefinition(true)}
            >
              Reload
            </Button>
          </Tooltip>
        ) : (
          <Tooltip title="Start over from a template">
            <Button
              size="small"
              icon={<ReloadOutlined />}
              onClick={() => setSql(templateFor(driver, database, schema))}
            >
              Template
            </Button>
          </Tooltip>
        )}
        <Tooltip title="Copy the script">
          <Button size="small" icon={<CopyOutlined />} onClick={() => void copy(sql, 'Script')} />
        </Tooltip>
        <Tooltip title="Save the script to a file">
          <Button size="small" icon={<SaveOutlined />} onClick={() => void save()} />
        </Tooltip>
        <Tooltip title="Clear the editor">
          <Button size="small" icon={<ClearOutlined />} onClick={() => setSql('')} />
        </Tooltip>

        <span className="dm-toolbar-sep" />
        <FileTextOutlined style={{ opacity: 0.6 }} />
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {object
            ? qualifiedName(driver ?? '', database, schema, object)
            : `${database || schema || 'new object'}`}
        </Typography.Text>

        <div className="dm-toolbar-right">
          {analysis ? (
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {statements.length} statement{statements.length === 1 ? '' : 's'}
            </Typography.Text>
          ) : null}
          {analysis?.destructive ? (
            <Tag color="error" icon={<ExclamationCircleOutlined />}>
              destructive
            </Tag>
          ) : null}
          {readOnly ? (
            <Tooltip title="Write statements are rejected on this session">
              <Typography.Text type="warning" style={{ fontSize: 12 }}>
                <LockOutlined /> read-only
              </Typography.Text>
            </Tooltip>
          ) : null}
        </div>
      </div>

      <Splitter orientation="vertical" style={{ flex: '1 1 auto', minHeight: 0 }}>
        <Splitter.Panel defaultSize="45%" min={100}>
          <SqlEditor
            value={sql}
            driver={driver}
            theme={theme}
            height="100%"
            onChange={setSql}
            onRun={runAll}
            onRunSelection={runSelection}
            onReady={(view) => {
              viewRef.current = view
            }}
          />
        </Splitter.Panel>

        <Splitter.Panel min={120}>
          <div className="dm-pane">
            {definitionError ? (
              <div style={{ padding: 10, flex: '0 0 auto' }}>
                <Alert
                  type="error"
                  showIcon
                  title="Could not load the object definition"
                  description={
                    <span className="mono" style={{ fontSize: 12 }}>
                      {definitionError}
                    </span>
                  }
                  action={
                    <Button size="small" onClick={() => void loadDefinition(true)}>
                      Retry
                    </Button>
                  }
                />
              </div>
            ) : null}

            {statements.length > 0 ? (
              <div className="dm-ddl-analyze">
                {statements.map((entry) => (
                  <Tooltip
                    key={entry.index}
                    title={
                      <span className="mono" style={{ fontSize: 11 }}>
                        {entry.reason ? `${entry.preview}\n${entry.reason}` : entry.preview}
                      </span>
                    }
                  >
                    <Tag
                      color={entry.destructive ? 'error' : KIND_COLOR[entry.kind]}
                      className="dm-ddl-chip"
                      icon={entry.destructive ? <ExclamationCircleOutlined /> : undefined}
                    >
                      {entry.index + 1}. {entry.kind}
                    </Tag>
                  </Tooltip>
                ))}
                {analysis?.refused ? (
                  <Tag color="warning">{analysis.refused} will be refused (read-only)</Tag>
                ) : null}
              </div>
            ) : null}

            {warnings.length > 0 ? (
              <div style={{ padding: '0 10px 8px', flex: '0 0 auto' }}>
                <Alert
                  type={analysis?.destructive ? 'warning' : 'info'}
                  showIcon
                  title="Dry run"
                  description={
                    <ul style={{ paddingInlineStart: 18, margin: 0 }}>
                      {warnings.map((entry, index) => (
                        <li key={index}>
                          <Typography.Text style={{ fontSize: 12 }}>{entry}</Typography.Text>
                        </li>
                      ))}
                    </ul>
                  }
                />
              </div>
            ) : null}

            {error ? (
              <div style={{ padding: 10, flex: '0 0 auto' }}>
                <Alert
                  type="error"
                  showIcon
                  closable
                  onClose={() => setError(null)}
                  title="Statement failed"
                  description={
                    <span className="mono" style={{ fontSize: 12, whiteSpace: 'pre-wrap' }}>
                      {error}
                    </span>
                  }
                />
              </div>
            ) : null}

            {result ? (
              <>
                {!result.hasResultSet ? (
                  <div style={{ padding: 12, flex: '0 0 auto' }}>
                    <Alert
                      type="success"
                      showIcon
                      title={`${result.affectedRows} row(s) affected in ${formatDuration(result.durationMs)}`}
                    />
                  </div>
                ) : null}
                {result.hasResultSet ? (
                  <DataGrid result={result} loading={running} primaryKey={undefined} />
                ) : null}
                <div
                  className="dm-statusbar"
                  style={{ borderTop: '1px solid var(--dm-border)', background: 'transparent' }}
                >
                  <span className="dm-statusbar-item">{formatDuration(result.durationMs)}</span>
                  {result.statementCount > 1 ? (
                    <span className="dm-statusbar-item">
                      last statement {result.statementIndex + 1} of {result.statementCount}
                    </span>
                  ) : null}
                  <span className="dm-spacer" />
                </div>
              </>
            ) : !error && statements.length === 0 ? (
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
                <Typography.Text type="secondary">
                  {loadingDefinition
                    ? 'Loading the object definition…'
                    : 'Write a script — the dry run appears here before anything runs.'}
                </Typography.Text>
              </div>
            ) : null}

            {result?.messages && result.messages.length > 0 ? (
              <div className="dm-messages">
                {result.messages.map((entry, index) => (
                  <div key={index}>{entry}</div>
                ))}
              </div>
            ) : null}

            {result?.hasResultSet ? (
              <div className="dm-statusbar" style={{ background: 'transparent' }}>
                <span className="dm-statusbar-item">
                  {result.rowCount.toLocaleString()} row(s)
                </span>
                <span className="dm-spacer" />
                <Space size={4} />
              </div>
            ) : null}
          </div>
        </Splitter.Panel>
      </Splitter>
    </div>
  )
}

/** Starting point for a brand-new object on each engine. */
function templateFor(driver: DriverType | undefined, database: string, schema: string): string {
  if (driver === 'mongodb') {
    // A collection comes into existence with its first document, so the
    // template creates one and indexes a field of it.
    const collection = quoteIdent('new_collection', 'mongodb')
    return [
      '// New collection',
      `db.createCollection(${JSON.stringify(collection)})`,
      '',
      '// Insert a first document',
      `db.getCollection(${JSON.stringify(collection)}).insertOne({ name: "example" })`,
      '',
      '// Index a field',
      `db.getCollection(${JSON.stringify(collection)}).createIndex({ name: 1 }, { name: "by_name" })`,
      '',
    ].join('\n')
  }

  const name = qualifiedName(driver ?? '', database, schema, 'new_table')
  const id = quoteIdent('id', driver ?? '')
  if (isMySQLFamily(driver)) {
    // Doris and TiDB both take the MySQL form; a Doris table additionally
    // needs a data model and a distribution clause, which the note on its
    // pages points at.
    return [
      '-- New table',
      `CREATE TABLE ${name} (`,
      `  ${id} BIGINT NOT NULL AUTO_INCREMENT,`,
      `  PRIMARY KEY (${id})`,
      ') ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;',
      '',
    ].join('\n')
  }
  switch (driver) {
    case 'postgres':
      return ['-- New table', `CREATE TABLE ${name} (`, `  ${id} bigserial PRIMARY KEY`, ');', ''].join(
        '\n',
      )
    case 'sqlite':
      return [
        '-- New table',
        `CREATE TABLE ${name} (`,
        `  ${id} INTEGER PRIMARY KEY AUTOINCREMENT`,
        ');',
        '',
      ].join('\n')
    default:
      return ['-- New object', `CREATE TABLE ${name} (`, `  ${id} INTEGER PRIMARY KEY`, ');', ''].join(
        '\n',
      )
  }
}
