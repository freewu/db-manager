import { useCallback, useEffect, useState } from 'react'
import { Alert, Button, Space, Spin, Tag, Tooltip, Typography } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type { ServerOverview } from '../api/types'
import { useAppStore, type WorkspaceTab } from '../store/appStore'
import { MysqlOverview } from './overview/MysqlOverview'
import { PostgresOverview } from './overview/PostgresOverview'
import { SqliteOverview } from './overview/SqliteOverview'
import { UnsupportedOverview } from './overview/UnsupportedOverview'

interface RuntimePaneProps {
  tab: WorkspaceTab
}

/**
 * The runtime status of one connection — what the server is doing right now.
 *
 * This component owns the frame only: the header (who the session is, when the
 * snapshot was taken), the warnings, and the choice of view. Everything below
 * the header belongs to one engine, and each engine has its own component,
 * because "what is worth watching" is genuinely different per engine: MySQL has
 * a buffer pool and a process list, PostgreSQL has backends and autovacuum, and
 * SQLite is a file with pragmas. Sharing one table across all three would mean
 * showing mostly empty rows for two of them.
 */
export function RuntimePane({ tab }: RuntimePaneProps) {
  const session = useAppStore((s) => s.sessionOf(tab.sessionId))
  const connections = useAppStore((s) => s.connections)
  const openConnectionEditor = useAppStore((s) => s.openConnectionEditor)

  // Only a session that came from a saved profile can be edited; an ad-hoc
  // connection has nothing to edit.
  const profile = connections.find((c) => c.id === session?.connectionId)

  const [overview, setOverview] = useState<ServerOverview | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [attempt, setAttempt] = useState(0)

  const load = useCallback(() => {
    let cancelled = false
    setLoading(true)
    api
      .getServerOverview(tab.sessionId)
      .then((result) => {
        if (cancelled) return
        setOverview(result)
        setError(null)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setError(toMessage(err))
        setOverview(null)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [tab.sessionId])

  useEffect(() => load(), [load, attempt])

  // The numbers are a snapshot, never a subscription: say how stale they are
  // rather than leaving the reader to guess.
  const [age, setAge] = useState(0)
  useEffect(() => {
    if (!overview) return
    setAge(0)
    const timer = window.setInterval(() => setAge((n) => n + 1), 1000)
    return () => window.clearInterval(timer)
  }, [overview])

  const header = (
    <div className="dm-editor-toolbar">
      <Space size={6} wrap>
        <Typography.Text strong>{overview?.name ?? session?.name ?? 'Connection'}</Typography.Text>
        <Tag style={{ marginInlineEnd: 0 }}>{overview?.driver ?? session?.driver ?? '—'}</Tag>
        {overview?.serverVersion ? (
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {overview.serverVersion}
          </Typography.Text>
        ) : null}
        {overview?.readOnly ? (
          <Tooltip title="This session refuses statements that write">
            <Tag color="gold" style={{ marginInlineEnd: 0 }}>
              read-only
            </Tag>
          </Tooltip>
        ) : null}
        {profile ? (
          <Button size="small" type="link" onClick={() => openConnectionEditor(profile)}>
            Edit connection…
          </Button>
        ) : null}
      </Space>

      <div className="dm-toolbar-right">
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {overview ? `collected ${ageLabel(age)} ago in ${overview.elapsedMs} ms` : 'collecting…'}
        </Typography.Text>
        <Tooltip title="Collect a fresh snapshot">
          <Button
            size="small"
            icon={<ReloadOutlined />}
            loading={loading}
            onClick={() => setAttempt((n) => n + 1)}
          />
        </Tooltip>
      </div>
    </div>
  )

  if (loading && !overview) {
    return (
      <div className="dm-pane">
        {header}
        <div style={{ padding: 60, textAlign: 'center' }}>
          <Spin />
        </div>
      </div>
    )
  }

  if (error || !overview) {
    return (
      <div className="dm-pane">
        {header}
        <div style={{ padding: 14 }}>
          <Alert
            type="error"
            showIcon
            title="Could not read the server state"
            description={
              <span className="mono" style={{ fontSize: 12, whiteSpace: 'pre-wrap' }}>
                {error}
              </span>
            }
            action={
              <Button size="small" icon={<ReloadOutlined />} onClick={() => setAttempt((n) => n + 1)}>
                Retry
              </Button>
            }
          />
        </div>
      </div>
    )
  }

  return (
    <div className="dm-pane">
      {header}
      <div className="dm-pane-body dm-runtime">
        {overview.warnings.length > 0 ? (
          <Alert
            type="warning"
            showIcon
            title={
              overview.warnings.length === 1
                ? 'Part of this page could not be read'
                : `${overview.warnings.length} parts of this page could not be read`
            }
            description={
              <ul style={{ paddingInlineStart: 18, margin: 0 }}>
                {overview.warnings.map((warning, index) => (
                  <li key={index}>
                    <Typography.Text style={{ fontSize: 12 }}>{warning}</Typography.Text>
                  </li>
                ))}
              </ul>
            }
          />
        ) : null}

        {/* One view per engine: the payload decides, not a flag on this page. */}
        {overview.driver === 'mysql' && overview.mysql ? (
          <MysqlOverview overview={overview} />
        ) : overview.driver === 'postgres' && overview.postgres ? (
          <PostgresOverview overview={overview} />
        ) : overview.driver === 'sqlite' && overview.sqlite ? (
          <SqliteOverview overview={overview} />
        ) : (
          <UnsupportedOverview overview={overview} />
        )}
      </div>

      <div className="dm-statusbar">
        <span className="dm-statusbar-item">
          {overview.database ? `session · ${overview.database}` : 'session'}
        </span>
        <span className="dm-statusbar-item">
          connected {connectedLabel(overview.connectedAt)}
        </span>
        <span className="dm-spacer" />
        <span className="dm-statusbar-item">snapshot · press refresh for current numbers</span>
      </div>
    </div>
  )
}

/** "3s" / "2m 5s" — how long ago the snapshot was taken. */
function ageLabel(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
}

function connectedLabel(at: number): string {
  if (!at) return '—'
  return new Date(at).toLocaleTimeString()
}
