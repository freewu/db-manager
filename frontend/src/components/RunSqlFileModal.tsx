import { useCallback, useEffect, useRef, useState } from 'react'
import { Alert, App as AntApp, Button, Modal, Progress, Space, Switch, Typography } from 'antd'

import { api, onSqlFileProgress, toMessage } from '../api/client'
import type { SQLFileAnalysis, SQLFileProgress, SQLFileResult } from '../api/types'
import { formatBytes, formatCount, formatDuration } from '../lib/format'
import { t, tn } from '../lib/i18n'

/** The database a file is to be run against. */
export interface RunSqlFileScope {
  sessionId: string
  database: string
}

interface RunSqlFileModalProps {
  /** The database to run against, or `null` when the window is closed. */
  scope: RunSqlFileScope | null
  onClose: () => void
}

/** A name for this run, so its progress events can be told from another's. */
function newRunId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

/**
 * Running a file of SQL against a database.
 *
 * The file is read and run by the backend, statement by statement: a dump is
 * routinely larger than the bridge wants to carry, and reading it there is what
 * lets this window show progress and stop a run that has gone wrong. So this
 * window picks the file, is shown what is in it before anything happens, and
 * then watches — it never holds the statements themselves.
 *
 * Two things are said plainly rather than left to be discovered:
 *
 *   - Nothing wraps the file in a transaction. DDL does not roll back on every
 *     engine and the drivers have no transaction handle to offer, so a run that
 *     is stopped halfway leaves what it already ran in place. The summary says
 *     how far it got.
 *   - A file that says `USE` switches the database for the statements after it,
 *     which this window follows instead of sending the statement — every
 *     statement is sent with the database it belongs to.
 */
export function RunSqlFileModal({ scope, onClose }: RunSqlFileModalProps) {
  const { message } = AntApp.useApp()
  const [path, setPath] = useState('')
  const [analysis, setAnalysis] = useState<SQLFileAnalysis | null>(null)
  const [reading, setReading] = useState(false)
  const [stopOnError, setStopOnError] = useState(true)
  const [busy, setBusy] = useState(false)
  const [progress, setProgress] = useState<SQLFileProgress | null>(null)
  const [result, setResult] = useState<SQLFileResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  /** The id of the run in flight, so its events can be told from another's. */
  const runId = useRef('')

  // Every opening starts from the same place: no file, nothing read, and the
  // answer that needs no second look before the button can be pressed.
  useEffect(() => {
    if (!scope) return
    setPath('')
    setAnalysis(null)
    setReading(false)
    setStopOnError(true)
    setBusy(false)
    setProgress(null)
    setResult(null)
    setError(null)
  }, [scope])

  // One subscription for the life of the window: the events carry the run's id,
  // so a file being run from another window is simply not this one's.
  useEffect(
    () =>
      onSqlFileProgress((event) => {
        if (event.id === runId.current) setProgress(event)
      }),
    [],
  )

  /** Asks for a file and reads what is in it, without running any of it. */
  const choose = useCallback(async () => {
    if (!scope) return
    try {
      const chosen = await api.pickFile(t('runSqlFile.choose-a-sql-file'), ['*.sql'])
      if (!chosen) return
      setPath(chosen)
      setAnalysis(null)
      setResult(null)
      setProgress(null)
      setError(null)
      setReading(true)
      try {
        // No id: reading a file registers no run, so there is nothing for the
        // window to answer for.
        setAnalysis(
          await api.analyzeSqlFile({
            id: '',
            sessionId: scope.sessionId,
            database: scope.database,
            path: chosen,
            stopOnError,
          }),
        )
      } finally {
        setReading(false)
      }
    } catch (err) {
      setReading(false)
      setError(toMessage(err))
    }
  }, [scope, stopOnError])

  const run = useCallback(async () => {
    if (!scope || !path || busy) return
    const id = newRunId()
    runId.current = id
    setBusy(true)
    setError(null)
    setResult(null)
    setProgress({ id, bytes: 0, size: analysis?.size ?? 0, done: 0, failed: 0, rows: 0 })
    try {
      const done = await api.runSqlFile({
        id,
        sessionId: scope.sessionId,
        database: scope.database,
        path,
        stopOnError,
      })
      setResult(done)
      if (done.cancelled) message.warning(t('runSqlFile.run-stopped'))
      else if (done.failed > 0) message.warning(t('runSqlFile.finished-with-failures'))
      else message.success(t('runSqlFile.run-finished'))
    } catch (err) {
      setError(toMessage(err))
    } finally {
      runId.current = ''
      setBusy(false)
    }
  }, [analysis, busy, message, path, scope, stopOnError])

  const stop = useCallback(() => {
    if (!runId.current) return
    void api.cancelSqlFile(runId.current).catch(() => undefined)
  }, [])

  const close = useCallback(() => {
    // A run is stopped before the window goes: a file still writing to a
    // database the user can no longer see the progress of is worse than cutting
    // it short, and the statements already run are not going to be undone
    // either way.
    if (runId.current) {
      void api.cancelSqlFile(runId.current).catch(() => undefined)
      runId.current = ''
    }
    setProgress(null)
    setResult(null)
    setError(null)
    setBusy(false)
    onClose()
  }, [onClose])

  const percent =
    progress && progress.size > 0
      ? Math.min(100, Math.round((progress.bytes / progress.size) * 100))
      : 0

  const summary = result ? (
    <>
      <div>
        {tn('runSqlFile.n-statements-run', result.statements, {
          n: formatCount(result.statements),
        })}
        {' · '}
        {tn('runSqlFile.n-rows-changed', result.rows, { n: formatCount(result.rows) })}
        {result.failed > 0
          ? ` · ${tn('runSqlFile.n-failed', result.failed, { n: formatCount(result.failed) })}`
          : ''}
        {` · ${formatDuration(result.durationMs)}`}
      </div>
      {result.errors?.length ? (
        <>
          <ul className="dm-export-warnings">
            {result.errors.map((failure) => (
              <li key={failure.index}>
                {t('runSqlFile.statement-number', { index: formatCount(failure.index) })}{' '}
                <code>{failure.statement}</code>
                <div>{failure.message}</div>
              </li>
            ))}
          </ul>
          {result.errorsTruncated ? (
            <div>{t('runSqlFile.only-the-first-failures-are-listed')}</div>
          ) : null}
        </>
      ) : null}
      {/* What already ran stays: no transaction wraps the file. */}
      {result.cancelled || result.stoppedOnError ? (
        <div>{t('runSqlFile.what-already-ran-was-not-undone')}</div>
      ) : null}
    </>
  ) : null

  const footer = result ? (
    <Button type="primary" onClick={close}>
      {t('runSqlFile.close')}
    </Button>
  ) : busy ? (
    <Space>
      <Typography.Text type="secondary">{t('runSqlFile.running-the-file')}</Typography.Text>
      <Button danger onClick={stop}>
        {t('runSqlFile.stop')}
      </Button>
    </Space>
  ) : (
    <Space>
      <Button onClick={close}>{t('runSqlFile.cancel')}</Button>
      <Button
        type="primary"
        danger={(analysis?.destructive ?? 0) > 0}
        disabled={!path || reading}
        onClick={() => void run()}
      >
        {t('runSqlFile.execute')}
      </Button>
    </Space>
  )

  return (
    <Modal
      open={scope !== null}
      title={
        scope
          ? t('runSqlFile.run-sql-file-database', { database: scope.database })
          : t('runSqlFile.run-sql-file')
      }
      width={680}
      footer={footer}
      onCancel={close}
      maskClosable={false}
      destroyOnHidden
    >
      <div className="dm-export-section">
        <div className="dm-export-head">
          <Typography.Text strong>{t('runSqlFile.file')}</Typography.Text>
          <Button size="small" disabled={busy} onClick={() => void choose()}>
            {t('runSqlFile.choose-file')}
          </Button>
        </div>
        <Typography.Text type="secondary" className="dm-export-path" ellipsis={{ tooltip: path }}>
          {path || t('runSqlFile.no-file-chosen-yet')}
        </Typography.Text>
        {reading ? (
          <Typography.Text type="secondary" className="dm-export-hint">
            {t('runSqlFile.reading-the-file')}
          </Typography.Text>
        ) : null}
      </div>

      {analysis ? (
        <div className="dm-export-section">
          <div className="dm-export-head">
            <Typography.Text strong>{t('runSqlFile.what-is-in-the-file')}</Typography.Text>
            <Typography.Text type="secondary" className="dm-export-hint">
              {tn('runSqlFile.n-statements', analysis.statements, {
                n: formatCount(analysis.statements),
              })}
              {` · ${formatBytes(analysis.size)}`}
            </Typography.Text>
          </div>

          {analysis.destructive > 0 ? (
            <Alert
              type="warning"
              showIcon
              title={tn('runSqlFile.n-destructive', analysis.destructive, {
                n: formatCount(analysis.destructive),
              })}
              description={
                analysis.destructiveIndexes?.length
                  ? t('runSqlFile.which-statements', {
                      indexes: analysis.destructiveIndexes.map(formatCount).join(', '),
                    })
                  : undefined
              }
            />
          ) : null}

          {analysis.refused > 0 ? (
            <Alert
              type="warning"
              showIcon
              title={tn('runSqlFile.n-refused', analysis.refused, {
                n: formatCount(analysis.refused),
              })}
              description={t('runSqlFile.a-read-only-connection-refuses-them')}
            />
          ) : null}

          {analysis.warnings?.length ? (
            <ul className="dm-export-warnings">
              {analysis.warnings.map((warning, index) => (
                <li key={index}>{warning}</li>
              ))}
            </ul>
          ) : null}

          <div className="dm-export-list">
            {analysis.shown.map((statement) => (
              <div key={statement.index} className="dm-sqlfile-line">
                <span className="dm-sqlfile-number">{statement.index + 1}</span>
                <code
                  className={
                    statement.destructive ? 'dm-sqlfile-sql is-destructive' : 'dm-sqlfile-sql'
                  }
                >
                  {statement.preview}
                </code>
              </div>
            ))}
          </div>
          {analysis.shown.length < analysis.statements ? (
            <Typography.Text type="secondary" className="dm-export-hint">
              {tn('runSqlFile.n-more-not-shown', analysis.statements - analysis.shown.length, {
                n: formatCount(analysis.statements - analysis.shown.length),
              })}
            </Typography.Text>
          ) : null}

          <Typography.Text type="secondary" className="dm-export-hint">
            {t('runSqlFile.the-file-is-not-run-in-a-transaction')}
          </Typography.Text>
        </div>
      ) : null}

      <div className="dm-export-section">
        <div className="dm-export-head">
          <Typography.Text strong>{t('runSqlFile.stop-on-error')}</Typography.Text>
          <Switch checked={stopOnError} disabled={busy} onChange={setStopOnError} />
        </div>
        <Typography.Text type="secondary" className="dm-export-hint">
          {t('runSqlFile.the-run-ends-at-the-statement-that-fails')}
        </Typography.Text>
      </div>

      {progress ? (
        <div className="dm-export-section">
          <Progress
            percent={percent}
            status={busy ? 'active' : result?.cancelled || result?.failed ? 'exception' : 'success'}
            showInfo={false}
          />
          <Typography.Text type="secondary" className="dm-export-hint">
            {tn('runSqlFile.n-run', progress.done, { n: formatCount(progress.done) })}
            {progress.failed > 0
              ? ` · ${tn('runSqlFile.n-failed', progress.failed, { n: formatCount(progress.failed) })}`
              : ''}
            {' · '}
            {tn('runSqlFile.n-rows-changed', progress.rows, { n: formatCount(progress.rows) })}
            {` · ${formatBytes(progress.bytes)}`}
            {progress.statement ? ` · ${progress.statement}` : ''}
          </Typography.Text>
        </div>
      ) : null}

      {error ? (
        <Alert type="error" showIcon title={t('runSqlFile.the-run-failed')} description={error} />
      ) : null}

      {result ? (
        <Alert
          type={result.cancelled || result.stoppedOnError || result.failed > 0 ? 'warning' : 'success'}
          showIcon
          title={
            result.cancelled
              ? t('runSqlFile.run-stopped')
              : result.stoppedOnError
                ? t('runSqlFile.stopped-at-a-failed-statement')
                : result.failed > 0
                  ? t('runSqlFile.finished-with-failures')
                  : t('runSqlFile.run-finished')
          }
          description={summary}
        />
      ) : null}
    </Modal>
  )
}
