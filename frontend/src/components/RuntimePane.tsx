import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Alert, Button, Space, Spin, Tag, Tooltip, Typography } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type { ServerOverview } from '../api/types'
import { useAppStore, type WorkspaceTab } from '../store/appStore'
import { overviewView } from './overview'

interface RuntimePaneProps {
  tab: WorkspaceTab
}

/**
 * The runtime status of one connection — what the server is doing right now.
 *
 * This component owns the frame only: the header (who the session is, when the
 * snapshot was taken), the warnings, and the rail. Everything inside the rail
 * belongs to one engine, and each engine has its own view, because "what is
 * worth watching" is genuinely different per engine: MySQL has a buffer pool and
 * a process list, PostgreSQL has backends and autovacuum, and SQLite is a file
 * with pragmas. Sharing one table across all three would mean showing mostly
 * empty rows for two of them.
 *
 * The page reads top to bottom through a single scrollbar, and the rail on the
 * left names the blocks it passes: this is a table of contents, not a set of
 * panes, so scrolling is what moves the highlight and clicking a name is only a
 * way to scroll there. Nothing is hidden, which is the point — a status page is
 * read, not clicked through, and one scrollbar keeps the whole snapshot in one
 * piece.
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

  const view = useMemo(() => (overview ? overviewView(overview) : null), [overview])
  const sections = view?.sections ?? []
  // One block is not a list of blocks, so it gets no rail.
  const rail = sections.length > 1

  const scrollRef = useRef<HTMLDivElement | null>(null)
  const sectionRefs = useRef<Array<HTMLDivElement | null>>([])
  const frameRef = useRef(0)
  const [current, setCurrent] = useState(0)

  /**
   * Which block the reader is looking at: the last one whose top has reached the
   * top of the column. The exception is the end of the scroll — the last block
   * may be too short to ever reach the top, and then it is the one on screen.
   */
  const syncCurrent = useCallback(() => {
    const container = scrollRef.current
    const count = sections.length
    if (!container || count === 0) return
    const top = container.getBoundingClientRect().top
    let next = 0
    if (container.scrollTop + container.clientHeight >= container.scrollHeight - 4) {
      next = count - 1
    } else {
      sectionRefs.current.slice(0, count).forEach((element, index) => {
        if (element && element.getBoundingClientRect().top - top <= 12) next = index
      })
    }
    setCurrent((previous) => (previous === next ? previous : next))
  }, [sections.length])

  // A wheel fires far more often than a frame, and every read here forces
  // layout, so the scroll handler only schedules the work.
  const handleScroll = useCallback(() => {
    if (frameRef.current) return
    frameRef.current = window.requestAnimationFrame(() => {
      frameRef.current = 0
      syncCurrent()
    })
  }, [syncCurrent])

  useEffect(
    () => () => {
      if (frameRef.current) window.cancelAnimationFrame(frameRef.current)
    },
    [],
  )

  // A block's contents arrive after the frame does (a capped list is fetched
  // with everything else, but tables lay themselves out later), so re-read the
  // offsets once the new page has been painted.
  useEffect(() => {
    const id = window.requestAnimationFrame(syncCurrent)
    return () => window.cancelAnimationFrame(id)
  }, [view, syncCurrent])

  // A new page starts at its first block, and the refs of the old one go away
  // with it.
  useEffect(() => {
    sectionRefs.current.length = sections.length
    if (scrollRef.current) scrollRef.current.scrollTop = 0
    setCurrent(0)
    syncCurrent()
  }, [sections.length, tab.sessionId, syncCurrent])

  // The column is resized by the splitter, not by the window, so the offsets
  // have to be re-read when its own box changes.
  useEffect(() => {
    const container = scrollRef.current
    if (!container || typeof ResizeObserver === 'undefined') return
    const observer = new ResizeObserver(() => syncCurrent())
    observer.observe(container)
    return () => observer.disconnect()
  }, [syncCurrent])

  const goToSection = (index: number) => {
    const container = scrollRef.current
    const element = sectionRefs.current[index]
    if (!container || !element) return
    container.scrollTo({
      top:
        container.scrollTop +
        (element.getBoundingClientRect().top - container.getBoundingClientRect().top),
      behavior: 'smooth',
    })
  }

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
      <div className="dm-pane-body dm-runtime-body">
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

        {view?.notice}

        <div className="dm-runtime">
          {rail ? (
            <nav className="dm-runtime-rail" aria-label="Blocks of this page">
              {sections.map((section, index) => (
                <button
                  key={`${index}-${section.label}`}
                  type="button"
                  className={index === current ? 'dm-runtime-tab is-active' : 'dm-runtime-tab'}
                  aria-current={index === current}
                  onClick={() => goToSection(index)}
                >
                  {section.label}
                </button>
              ))}
            </nav>
          ) : null}

          <div className="dm-runtime-scroll" ref={scrollRef} onScroll={handleScroll}>
            {sections.map((section, index) => (
              <div
                key={`${index}-${section.label}`}
                className="dm-runtime-section"
                ref={(element) => {
                  sectionRefs.current[index] = element
                }}
              >
                {section.node}
              </div>
            ))}

            {view?.footnote ? <div className="dm-runtime-footnote">{view.footnote}</div> : null}
          </div>
        </div>
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
