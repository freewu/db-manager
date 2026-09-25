import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  App as AntApp,
  Button,
  Checkbox,
  Empty,
  Modal,
  Select,
  Space,
  Spin,
  Splitter,
  Tag,
  Tooltip,
  Typography,
} from 'antd'
import { CopyOutlined, ReloadOutlined, SwapOutlined } from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type {
  CompareSide,
  DesignPlan,
  DiffItem,
  DiffStatus,
  DriverInfo,
  DriverType,
  SchemaCompare,
  SessionInfo,
  SyncDatabaseRequest,
  TableDiff,
} from '../api/types'
import { capabilitiesOf, findDriver } from '../lib/capabilities'
import { useAppStore } from '../store/appStore'
import { SqlCode } from './SqlCode'
import { t, tn } from '../lib/i18n'

/** How a status reads as a word and a colour, in one place. */
const STATUS: Record<DiffStatus, { label: string; color: string }> = {
  added: { label: t('comparePane.only-left'), color: 'green' },
  removed: { label: t('comparePane.only-right'), color: 'red' },
  changed: { label: t('comparePane.differs'), color: 'gold' },
  same: { label: t('comparePane.same'), color: 'default' },
}

/**
 * Database comparison, and the script generated from it.
 *
 * Two databases of the same engine are read side by side and answered table by
 * table: which tables only one side has, which fields and indexes differ, and how
 * they differ. What the page produces is not a report but a script — the
 * difference in the designer's vocabulary is exactly the vocabulary a script can
 * be written in, so "generate the update script" is the same comparison read
 * once more.
 *
 * The two sides must be the same engine, because a script for one engine is not
 * a script for another; the right side is the one that changes, so it is the
 * side a read-only connection is refused for. Everything is read live at the
 * moment the comparison runs — a schema is the one thing that must not be a
 * snapshot taken when the window was opened — and nothing is executed until the
 * preview is approved.
 *
 * It is a page rather than a window in the working area, like the other pages
 * the rail names: it is about two connections at once, so it does not belong to
 * either of them.
 */
export function ComparePane() {
  const sessions = useAppStore((s) => s.sessions)
  const drivers = useAppStore((s) => s.drivers)

  const [left, setLeft] = useState<CompareSide>({ sessionId: '' })
  const [right, setRight] = useState<CompareSide>({ sessionId: '' })
  const [compare, setCompare] = useState<SchemaCompare | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [selected, setSelected] = useState(0)
  const [onlyDifferences, setOnlyDifferences] = useState(false)
  const [scripting, setScripting] = useState(false)

  const leftDriver = sessions.find((s) => s.id === left.sessionId)?.driver
  const rightDriver = sessions.find((s) => s.id === right.sessionId)?.driver
  // Once either side is picked, the other side's list narrows to the same
  // engine: the pair the page is built on is a pair of the same kind.

  // Standing on something to compare the first time the page is opened. Only the
  // empty state triggers this, so a choice the user made is never overwritten.
  useEffect(() => {
    if (left.sessionId || right.sessionId) return
    const usable = sessions.filter((s) => capabilitiesOf(findDriver(drivers, s.driver)).designable)
    if (usable.length === 0) return
    const first = usable[0]
    const second = usable.find((s) => s.id !== first.id && s.driver === first.driver)
    setLeft({ sessionId: first.id })
    if (second) setRight({ sessionId: second.id })
    // `sessions` and `drivers` are read once, to seed the page: re-seeding when
    // the connection list changes would throw away the two databases chosen.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const run = useCallback(
    async (pair: { left: CompareSide; right: CompareSide }) => {
      setBusy(true)
      try {
        const result = await api.compareDatabases(pair)
        setCompare(result)
        setSelected(0)
        setError(null)
      } catch (err) {
        setCompare(null)
        setError(toMessage(err))
      } finally {
        setBusy(false)
      }
    },
    [],
  )

  // Comparing again after a swap or a change of side is what the button is for;
  // an old answer does not belong under a new question, so it goes.
  const changeLeft = (next: CompareSide) => {
    setLeft(next)
    setCompare(null)
  }
  const changeRight = (next: CompareSide) => {
    setRight(next)
    setCompare(null)
  }

  const swap = () => {
    setLeft(right)
    setRight(left)
    setCompare(null)
  }

  const ready = Boolean(left.sessionId && right.sessionId)
  const shown = useMemo(() => {
    if (!compare) return []
    if (!onlyDifferences) return compare.tables
    return compare.tables.filter((table) => table.status !== 'same')
  }, [compare, onlyDifferences])

  const table = shown[Math.min(selected, shown.length - 1)]

  const counts = useMemo(() => {
    const out: Record<DiffStatus, number> = { added: 0, removed: 0, changed: 0, same: 0 }
    for (const item of compare?.tables ?? []) out[item.status]++
    return out
  }, [compare])

  // The request the script is generated from. Memoised so the modal's planner
  // sees a stable value and does not re-plan on every render of this page.
  const syncRequest: SyncDatabaseRequest = useMemo(
    () => ({ left, right, dropExtra: false }),
    [left, right],
  )

  return (
    <div className="dm-pane">
      <div className="dm-compare-bar">
        <SideField
          label={t('comparePane.left-the-reference')}
          side={left}
          sessions={sessions}
          drivers={drivers}
          engine={rightDriver}
          onChange={changeLeft}
        />
        <Tooltip title={t('comparePane.swap-the-two-sides')}>
          <Button
            className="dm-compare-swap"
            icon={<SwapOutlined />}
            onClick={swap}
            disabled={!left.sessionId && !right.sessionId}
          />
        </Tooltip>
        <SideField
          label={t('comparePane.right-the-one-the-script-changes')}
          side={right}
          sessions={sessions}
          drivers={drivers}
          engine={leftDriver}
          // Applying runs against this side, so a connection that refuses writes
          // is not one it can be pointed at.
          requireWritable
          onChange={changeRight}
        />
      </div>

      <div className="dm-editor-toolbar dm-compare-toolbar">
        <Typography.Text strong>{t('comparePane.differences')}</Typography.Text>
        {compare ? (
          <>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {compare.leftLabel} → {compare.rightLabel}
            </Typography.Text>
            <Tag color={STATUS.added.color} style={{ marginInlineEnd: 0 }}>
              {counts.added} {t('comparePane.only-left')}
            </Tag>
            <Tag color={STATUS.removed.color} style={{ marginInlineEnd: 0 }}>
              {counts.removed} {t('comparePane.only-right')}
            </Tag>
            <Tag color={STATUS.changed.color} style={{ marginInlineEnd: 0 }}>
              {counts.changed} {t('comparePane.differ')}
            </Tag>
            <Tag style={{ marginInlineEnd: 0 }}>{counts.same} {t('comparePane.same')}</Tag>
          </>
        ) : (
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {ready
              ? t('comparePane.compare-the-two-databases-to-see-what-differs')
              : t('comparePane.pick-a-connection-database-and-schema-on-each')}
          </Typography.Text>
        )}
        <div className="dm-toolbar-right">
          <Checkbox
            checked={onlyDifferences}
            onChange={(e) => setOnlyDifferences(e.target.checked)}
            disabled={!compare}
          >
            <Typography.Text style={{ fontSize: 12 }}>{t('comparePane.only-differences')}</Typography.Text>
          </Checkbox>
          <Button
            size="small"
            type="primary"
            icon={<ReloadOutlined />}
            loading={busy}
            disabled={!ready}
            onClick={() => void run({ left, right })}
          >
            {t('comparePane.compare')}
          </Button>
          <Button size="small" disabled={!compare} onClick={() => setScripting(true)}>
            {t('comparePane.generate-script')}
          </Button>
        </div>
      </div>

      {error ? (
        <Alert
          type="error"
          showIcon
          title={t('comparePane.the-comparison-could-not-be-run')}
          description={error}
          style={{ margin: 8 }}
        />
      ) : null}
      {compare && compare.warnings.length > 0 ? (
        <Alert
          type="warning"
          showIcon
          title={t('comparePane.what-this-comparison-did-not-look-at')}
          description={
            <ul style={{ margin: 0, paddingInlineStart: 18 }}>
              {compare.warnings.map((warning) => (
                <li key={warning}>{warning}</li>
              ))}
            </ul>
          }
          style={{ margin: 8 }}
        />
      ) : null}

      {compare ? (
        <Splitter style={{ flex: '1 1 auto', minHeight: 0 }}>
          <Splitter.Panel defaultSize={340} min={240} max={620}>
            <div className="dm-compare-list">
              {shown.length === 0 ? (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={
                    compare.tables.length === 0
                      ? t('comparePane.neither-database-has-a-table-to-compare')
                      : t('comparePane.every-table-is-the-same-on-both-sides')
                  }
                  style={{ marginTop: 40 }}
                />
              ) : (
                shown.map((entry, index) => (
                  <TableRow
                    key={entry.name}
                    table={entry}
                    active={entry === table}
                    onClick={() => setSelected(index)}
                  />
                ))
              )}
            </div>
          </Splitter.Panel>

          <Splitter.Panel min={320}>
            <div className="dm-pane-body dm-compare-detail">
              {table ? (
                <TableDetail
                  table={table}
                  leftLabel={compare.leftLabel}
                  rightLabel={compare.rightLabel}
                />
              ) : (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={t('comparePane.nothing-to-show')}
                  style={{ marginTop: 60 }}
                />
              )}
            </div>
          </Splitter.Panel>
        </Splitter>
      ) : (
        <div className="dm-pane-body">
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              sessions.length === 0
                ? t('comparePane.open-two-connections-to-compare-their-databases')
                : t('comparePane.compare-two-databases-of-the-same-engine')
            }
            style={{ marginTop: 60 }}
          />
        </div>
      )}

      {compare ? (
        <SyncScriptModal
          open={scripting}
          request={syncRequest}
          driver={compare.driver}
          rightLabel={compare.rightLabel}
          onClose={() => setScripting(false)}
          onApplied={() => void run({ left, right })}
        />
      ) : null}
    </div>
  )
}

/**
 * One side of the comparison: the connection, the database inside it and the
 * schema in turn.
 *
 * The lists are read from the session rather than from the explorer's cache: the
 * comparison is about what is on the server now, and a namespace the tree has
 * never visited would not be in any cache to pick from.
 */
function SideField({
  label,
  side,
  sessions,
  drivers,
  engine,
  requireWritable = false,
  onChange,
}: {
  label: string
  side: CompareSide
  sessions: SessionInfo[]
  drivers: DriverInfo[]
  /** The engine the other side is on, when one is chosen: the two must match. */
  engine?: DriverType
  requireWritable?: boolean
  onChange: (side: CompareSide) => void
}) {
  const [databases, setDatabases] = useState<string[]>([])
  const [schemas, setSchemas] = useState<string[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const session = sessions.find((s) => s.id === side.sessionId)
  const needsSchema = !capabilitiesOf(findDriver(drivers, session?.driver)).flatNamespace

  useEffect(() => {
    if (!side.sessionId) {
      setDatabases([])
      setSchemas([])
      return
    }
    let cancelled = false
    setBusy(true)
    api
      .listDatabases(side.sessionId)
      .then((names) => {
        if (cancelled) return
        setDatabases(names)
        setError(null)
        // Standing on the database the connection was opened into, when it is
        // one of them: an empty namespace is not a comparison anybody can make,
        // so a side that was just picked lands on something.
        if (!side.database && names.length > 0) {
          const preferred =
            session?.database && names.includes(session.database) ? session.database : names[0]
          onChange({ sessionId: side.sessionId, database: preferred })
        }
      })
      .catch((err) => {
        if (cancelled) return
        setDatabases([])
        setError(toMessage(err))
      })
      .finally(() => {
        if (!cancelled) setBusy(false)
      })
    return () => {
      cancelled = true
    }
    // Only the session is a dependency: a database that is already chosen is not
    // re-chosen, and re-reading the list is what picking the session again is for.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [side.sessionId])

  useEffect(() => {
    if (!side.sessionId || !needsSchema || !side.database) {
      setSchemas([])
      return
    }
    let cancelled = false
    api
      .listSchemas(side.sessionId, side.database)
      .then((names) => {
        if (cancelled) return
        setSchemas(names)
        if (!side.schema && names.length > 0) {
          onChange({ ...side, schema: names.includes('public') ? 'public' : names[0] })
        }
      })
      .catch(() => {
        if (!cancelled) setSchemas([])
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [side.sessionId, side.database, needsSchema])

  return (
    <div className="dm-compare-side">
      <Typography.Text strong style={{ fontSize: 12 }}>
        {label}
      </Typography.Text>
      <Space size={6} wrap>
        <Select
          size="small"
          style={{ width: 190 }}
          value={side.sessionId || undefined}
          placeholder={t('comparePane.connection')}
          onChange={(id: string) => onChange({ sessionId: id })}
          options={sessions.map((item) => {
            const designable = capabilitiesOf(findDriver(drivers, item.driver)).designable
            const otherEngine = engine !== undefined && item.driver !== engine
            const name = item.name || item.driver
            return {
              value: item.id,
              label: item.readOnly ? t('comparePane.name-read-only', { name }) : name,
              disabled: !designable || otherEngine || (requireWritable && item.readOnly),
            }
          })}
        />
        <Select
          size="small"
          style={{ width: 150 }}
          value={side.database || undefined}
          placeholder={t('comparePane.database')}
          loading={busy}
          disabled={!side.sessionId}
          onChange={(database: string) => onChange({ sessionId: side.sessionId, database })}
          options={databases.map((name) => ({ value: name, label: name }))}
        />
        {needsSchema ? (
          <Select
            size="small"
            style={{ width: 150 }}
            value={side.schema || undefined}
            placeholder={t('comparePane.schema')}
            disabled={!side.database}
            onChange={(schema: string) => onChange({ ...side, schema })}
            options={schemas.map((name) => ({ value: name, label: name }))}
          />
        ) : null}
      </Space>
      {/* The rules of the page are worth one line each while a side is still
          empty, and worth nothing once both are settled. */}
      {!side.sessionId && engine ? (
        <Typography.Text type="secondary" style={{ fontSize: 11 }}>
          {t('comparePane.only-engine-connections-are-offered', { engine })}
        </Typography.Text>
      ) : null}
      {!side.sessionId && requireWritable ? (
        <Typography.Text type="secondary" style={{ fontSize: 11 }}>
          {t('comparePane.read-only-connections-are-not-offered-here-this')}
        </Typography.Text>
      ) : null}
      {error ? (
        <Typography.Text type="danger" style={{ fontSize: 11 }}>
          {error}
        </Typography.Text>
      ) : null}
    </div>
  )
}

/** One table in the comparison list: its name, its standing and a line of why. */
function TableRow({
  table,
  active,
  onClick,
}: {
  table: TableDiff
  active: boolean
  onClick: () => void
}) {
  const status = STATUS[table.status]
  return (
    <button
      type="button"
      className={`dm-compare-row${active ? ' is-active' : ''}`}
      onClick={onClick}
    >
      <span className="dm-compare-row-top">
        <span className="dm-compare-name">{table.name}</span>
        <Tag color={status.color} style={{ marginInlineEnd: 0, fontSize: 10, lineHeight: '15px' }}>
          {status.label}
        </Tag>
      </span>
      <span className="dm-compare-row-summary">{table.summary}</span>
    </button>
  )
}

/** One table in full: how it stands, then its fields and its indexes. */
function TableDetail({
  table,
  leftLabel,
  rightLabel,
}: {
  table: TableDiff
  leftLabel: string
  rightLabel: string
}) {
  const [showSame, setShowSame] = useState(false)

  // One relevant row of the table, with a line above it that names it.
  const renderItem = (item: DiffItem) => (
    <div key={`${item.kind}:${item.name}`} className="dm-diff-item">
      <div className="dm-diff-item-head">
        <Tag
          color={STATUS[item.status].color}
          style={{ marginInlineEnd: 0, fontSize: 10, lineHeight: '15px' }}
        >
          {STATUS[item.status].label}
        </Tag>
        <Typography.Text code>{item.name}</Typography.Text>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {item.summary}
        </Typography.Text>
      </div>
      {item.fields && item.fields.length > 0 ? (
        <table className="dm-diff-fields">
          <thead>
            <tr>
              <th />
              <th>{leftLabel}</th>
              <th>{rightLabel}</th>
            </tr>
          </thead>
          <tbody>
            {item.fields.map((field) => (
              <tr key={field.field}>
                <td className="dm-diff-field-name">{field.field}</td>
                <td>{field.left}</td>
                <td>{field.right}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : null}
    </div>
  )

  const visible = (items: DiffItem[]) =>
    showSame ? items : items.filter((item) => item.status !== 'same')

  const columns = visible(table.columns)
  const indexes = visible(table.indexes)
  const unchanged = table.columns.length + table.indexes.length - columns.length - indexes.length

  return (
    <div className="dm-compare-body">
      <div className="dm-compare-detail-head">
        <Typography.Text strong style={{ fontSize: 13 }}>
          {table.name}
        </Typography.Text>
        <Tag color={STATUS[table.status].color} style={{ marginInlineEnd: 0 }}>
          {STATUS[table.status].label}
        </Tag>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {table.summary}
        </Typography.Text>
        <div className="dm-toolbar-right">
          <Checkbox
            checked={showSame}
            onChange={(e) => setShowSame(e.target.checked)}
            disabled={unchanged === 0 && !showSame}
          >
            <Typography.Text style={{ fontSize: 12 }}>{t('comparePane.show-unchanged')}</Typography.Text>
          </Checkbox>
        </div>
      </div>

      <Typography.Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 6 }}>
        {t('comparePane.fields')}
      </Typography.Text>
      {columns.length === 0 ? (
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {t('comparePane.every-field-is-the-same-on-both-sides')}
        </Typography.Text>
      ) : (
        columns.map(renderItem)
      )}

      <Typography.Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 10 }}>
        {t('comparePane.indexes')}
      </Typography.Text>
      {indexes.length === 0 ? (
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {table.indexes.length === 0
            ? t('comparePane.neither-side-has-a-secondary-index')
            : t('comparePane.every-index-is-the-same-on-both-sides')}
        </Typography.Text>
      ) : (
        indexes.map(renderItem)
      )}
    </div>
  )
}

/**
 * The script generated from a comparison, and the one thing there is to decide.
 *
 * `dropExtra` is that decision: a table that exists only on the right is left
 * alone unless it is asked for, because "make B look like A" and "delete
 * everything B has that A does not" are different promises. Toggling it re-plans
 * — the script really is different — rather than running the same statements with
 * one of them skipped.
 *
 * Running it plans the whole script again on the other side of the bridge, the
 * way every other write in this application does: the script that runs is
 * rendered from the catalog that is current when it runs, not from the preview
 * that was read a minute ago.
 */
function SyncScriptModal({
  open,
  request,
  driver,
  rightLabel,
  onClose,
  onApplied,
}: {
  open: boolean
  request: SyncDatabaseRequest
  driver: DriverType
  rightLabel: string
  onClose: () => void
  onApplied: () => void
}) {
  const { message } = AntApp.useApp()
  const [dropExtra, setDropExtra] = useState(false)
  const [plan, setPlan] = useState<DesignPlan | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [applying, setApplying] = useState(false)

  // Each opening starts from "do not delete anything", which is the decision that
  // cannot be undone. It is reset on the way out rather than on the way in, so a
  // second opening plans once instead of planning twice.
  useEffect(() => {
    if (!open) setDropExtra(false)
  }, [open])

  useEffect(() => {
    if (!open) return
    let cancelled = false
    setBusy(true)
    api
      .planSyncDatabase({ ...request, dropExtra })
      .then((next) => {
        if (cancelled) return
        setPlan(next)
        setError(null)
      })
      .catch((err) => {
        if (cancelled) return
        setPlan(null)
        setError(toMessage(err))
      })
      .finally(() => {
        if (!cancelled) setBusy(false)
      })
    return () => {
      cancelled = true
    }
  }, [open, dropExtra, request])

  const script = plan?.statements.map((statement) => `${statement};`).join('\n') ?? ''
  const empty = !plan || plan.statements.length === 0

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(script)
      message.success(t('comparePane.script-copied'))
    } catch (err) {
      message.error(toMessage(err))
    }
  }

  const apply = async () => {
    setApplying(true)
    try {
      const result = await api.applySyncDatabase({ ...request, dropExtra })
      if (result.error) {
        message.error(
          t('comparePane.the-script-stopped-at-statement', { failedIndex: result.failedIndex + 1, error: result.error }),
        )
        return
      }
      message.success(tn('comparePane.statements-applied-to', result.executed.length, { rightLabel }))
      onApplied()
      onClose()
    } catch (err) {
      message.error(toMessage(err))
    } finally {
      setApplying(false)
    }
  }

  return (
    <Modal
      open={open}
      onCancel={onClose}
      width={860}
      title={t('comparePane.update-script-for', { rightLabel })}
      footer={[
        <Button key="close" onClick={onClose}>
          {t('comparePane.cancel')}
        </Button>,
        <Button key="copy" icon={<CopyOutlined />} disabled={empty} onClick={() => void copy()}>
          {t('comparePane.copy')}
        </Button>,
        <Button
          key="run"
          type="primary"
          danger
          loading={applying}
          disabled={empty}
          onClick={() => void apply()}
        >
          {t('comparePane.run-script')}
        </Button>,
      ]}
    >
      {error ? (
        <Alert
          type="error"
          showIcon
          title={t('comparePane.the-script-could-not-be-generated')}
          description={error}
          style={{ marginBottom: 12 }}
        />
      ) : null}

      <Checkbox
        checked={dropExtra}
        onChange={(e) => setDropExtra(e.target.checked)}
        style={{ marginBottom: 12 }}
      >
        {t('comparePane.also-drop-tables-that-only-the-right-database')}
      </Checkbox>

      {plan && plan.warnings.length > 0 ? (
        <Alert
          type="warning"
          showIcon
          title={t('comparePane.before-this-runs')}
          style={{ marginBottom: 12 }}
          description={
            <ul style={{ margin: 0, paddingInlineStart: 18 }}>
              {plan.warnings.map((warning) => (
                <li key={warning}>{warning}</li>
              ))}
            </ul>
          }
        />
      ) : null}

      {busy && !plan ? (
        <Spin size="small" />
      ) : empty ? (
        <Alert
          type="success"
          showIcon
          title={t('comparePane.nothing-to-do')}
          description={t('comparePane.the-right-database-already-matches-the-left-one')}
        />
      ) : (
        <>
          <SqlCode className="dm-ddl" driver={driver} sql={script} />
          <Typography.Paragraph
            type="secondary"
            style={{ fontSize: 12, marginTop: 8, marginBottom: 0 }}
          >
            {t('comparePane.runs-against-one-at-a-time', { rightLabel })}
          </Typography.Paragraph>
        </>
      )}
    </Modal>
  )
}
