import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  App as AntApp,
  Button,
  Descriptions,
  Empty,
  Input,
  Select,
  Splitter,
  Tag,
  Tooltip,
  Typography,
} from 'antd'
import {
  CopyOutlined,
  ExclamationCircleOutlined,
  ReloadOutlined,
  SearchOutlined,
} from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type { ChangeLogEntry, ChangeLogFile } from '../api/types'
import { formatBytes, formatTimestamp } from '../lib/format'
import { useAppStore } from '../store/appStore'
import { SqlCode } from './SqlCode'

/** How many entries a window asks for. More than this is not a list any more. */
const PAGE = 500

/** Statements are shown by what they do, so the tag reads as an answer. */
const KIND_COLOR: Record<string, string> = {
  create: 'green',
  alter: 'geekblue',
  drop: 'red',
  truncate: 'red',
  rename: 'purple',
  comment: 'default',
  insert: 'cyan',
  update: 'orange',
  delete: 'volcano',
  replace: 'purple',
  merge: 'purple',
  copy: 'cyan',
  load: 'cyan',
}

const SOURCE_LABEL: Record<string, string> = {
  script: 'A script that was run',
  design: 'The structure page',
  create: 'The table designer',
  grid: 'The data grid',
}

/**
 * Change log window.
 *
 * It answers one question — what has this program done to the databases it was
 * pointed at — and it answers it from the data directory rather than from a
 * server: every statement this application executed that was not a read is a
 * line here, with the connection it ran on, the object it was applied to and
 * what came of it. Nothing has to be connected for this to open, which is when
 * a change usually needs looking up.
 *
 * The log is not one file. When the file being written reaches the size the user
 * set, it is moved aside as a whole — `<yyyymmdd>-<n>.log` — and a new one is
 * started, so nothing is ever dropped and the older statements are still here.
 * The picker in the toolbar is that shelf: it lists every file with how much is
 * in it, and the lists below show the one that is open.
 *
 * It is a page rather than a window in the working area: the log covers every
 * connection this application has touched, so it is not a document about the
 * table a tab strip would be pointing at, and it is the one thing here that can
 * be read with nothing connected.
 */
export function ChangeLogPane() {
  const drivers = useAppStore((s) => s.drivers)
  // The pane is mounted once and then kept behind the other pages, so "showing"
  // is a fact about the store rather than about being built.
  const showing = useAppStore((s) => s.page === 'changelog')

  const [entries, setEntries] = useState<ChangeLogEntry[]>([])
  const [files, setFiles] = useState<ChangeLogFile[]>([])
  const [file, setFile] = useState('')
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [filter, setFilter] = useState('')
  const [selected, setSelected] = useState(0)

  const load = useCallback(
    async (name: string) => {
      setLoading(true)
      try {
        const log = await api.listChangeLog(name, PAGE)
        setEntries(log.entries)
        setFiles(log.files)
        setFile(log.file)
        setTotal(log.total)
        setSelected(0)
        setError(null)
      } catch (err) {
        setError(toMessage(err))
        setEntries([])
        setTotal(0)
      } finally {
        setLoading(false)
      }
    },
    [],
  )

  // Re-read when the page comes to the front: a statement is run from another
  // page, and the only honest answer about a log of what has been run is the one
  // that was read after the last statement was. Re-reading also refreshes the
  // picker, so an archive a rotation just created is offered.
  useEffect(() => {
    if (showing) void load(file)
    // `file` is deliberately not a dependency: the file is re-read when it is
    // switched below, and re-reading it here would fight the switch.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [showing, load])

  const shown = useMemo(() => {
    const needle = filter.trim().toLowerCase()
    if (!needle) return entries
    return entries.filter((entry) =>
      [entry.statement, entry.table, entry.database, entry.schema, entry.connection.name, entry.kind]
        .filter(Boolean)
        .some((field) => String(field).toLowerCase().includes(needle)),
    )
  }, [entries, filter])

  // A filter can remove the entry that was selected; the first of what is left
  // is a better answer than an empty pane.
  const entry = shown[Math.min(selected, shown.length - 1)]
  const current = files.find((item) => item.name === file)

  return (
    <div className="dm-pane">
      <div className="dm-editor-toolbar">
        <Typography.Text strong>Change Log</Typography.Text>
        <Select
          size="small"
          value={file}
          onChange={(name) => void load(name)}
          // A fixed width would push the filter out of a narrow window; the
          // toolbar clips rather than wraps, so this shrinks instead.
          style={{ flex: '1 1 300px', minWidth: 180, maxWidth: 420 }}
          popupMatchSelectWidth={false}
          options={files.map((item) => ({
            value: item.name,
            label: fileLabel(item),
          }))}
          // A log that has never been written has no file to pick, and one
          // empty select is a clearer answer than a hidden control.
          placeholder="No log yet"
        />
        {current?.archived ? (
          <Tooltip
            title={`Rotated out on ${formatTimestamp(current.at ?? 0)}. Archived logs are kept as they were and never written to again.`}
          >
            <Tag color="default" style={{ marginInlineEnd: 0 }}>
              archived
            </Tag>
          </Tooltip>
        ) : null}
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {total === 0
            ? 'Nothing has been run yet'
            : total > entries.length
              ? `the newest ${entries.length} of ${total.toLocaleString()} statements`
              : `${total.toLocaleString()} statement${total === 1 ? '' : 's'}`}
        </Typography.Text>
        <div className="dm-toolbar-right">
          <Input
            allowClear
            size="small"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            prefix={<SearchOutlined style={{ opacity: 0.5 }} />}
            placeholder="Filter by table, database or statement"
            style={{ width: 260 }}
          />
          <Tooltip title="Read the log again">
            <Button
              size="small"
              type="text"
              icon={<ReloadOutlined />}
              loading={loading}
              onClick={() => void load(file)}
            />
          </Tooltip>
        </div>
      </div>

      {error ? (
        <Alert
          type="error"
          showIcon
          message="The change log could not be read"
          description={error}
          style={{ margin: 8 }}
        />
      ) : null}

      <Splitter style={{ flex: '1 1 auto', minHeight: 0 }}>
        <Splitter.Panel defaultSize={340} min={240} max={640}>
          <div className="dm-changelog-list">
            {shown.length === 0 ? (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={
                  entries.length === 0
                    ? 'Nothing has been run that changed anything'
                    : 'No entry matches the filter'
                }
                style={{ marginTop: 40 }}
              />
            ) : (
              shown.map((item, index) => (
                <EntryRow
                  key={`${item.at}:${index}`}
                  entry={item}
                  active={item === entry}
                  onClick={() => setSelected(index)}
                />
              ))
            )}
          </div>
        </Splitter.Panel>

        <Splitter.Panel min={320}>
          <div className="dm-pane-body dm-changelog-detail">
            {entry ? (
              <EntryDetail entry={entry} driverName={driverLabel(drivers, entry.connection.driver)} />
            ) : (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description="Nothing to show"
                style={{ marginTop: 60 }}
              />
            )}
          </div>
        </Splitter.Panel>
      </Splitter>
    </div>
  )
}

/** One line of the list: when, what was done, and to what. */
function EntryRow({
  entry,
  active,
  onClick,
}: {
  entry: ChangeLogEntry
  active: boolean
  onClick: () => void
}) {
  const where = entry.table || entry.database || '—'
  return (
    <button
      type="button"
      className={`dm-changelog-row${active ? ' is-active' : ''}`}
      onClick={onClick}
    >
      <span className="dm-changelog-row-top">
        <span className="dm-changelog-when">{shortStamp(entry.at)}</span>
        <Tag
          color={entry.error ? 'red' : (KIND_COLOR[entry.kind] ?? 'default')}
          style={{ marginInlineEnd: 0, fontSize: 10, lineHeight: '15px' }}
        >
          {entry.kind.toUpperCase()}
        </Tag>
        {entry.error ? <ExclamationCircleOutlined style={{ color: '#d4380d' }} /> : null}
        <span className="dm-changelog-where">{where}</span>
      </span>
      <span className="dm-changelog-row-sql">{oneLine(entry.statement)}</span>
    </button>
  )
}

/** What the selected entry holds, in full. */
function EntryDetail({ entry, driverName }: { entry: ChangeLogEntry; driverName: string }) {
  const { message } = AntApp.useApp()
  const { connection } = entry
  const database = entry.database || ''
  const namespace = [database, entry.schema].filter(Boolean).join(' · ')

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(entry.statement)
      message.success('Statement copied')
    } catch (err) {
      message.error(toMessage(err))
    }
  }

  return (
    <div className="dm-changelog-body">
      <Descriptions column={1} size="small" colon={false} className="dm-changelog-meta">
        <Descriptions.Item label="Time">{formatTimestamp(entry.at)}</Descriptions.Item>
        <Descriptions.Item label="Connection">
          {connection.name ? (
            <>
              {connection.name}
              <Typography.Text type="secondary" style={{ fontSize: 12, marginInlineStart: 6 }}>
                {[driverName, [connection.user, connection.address].filter(Boolean).join(' at ')]
                  .filter(Boolean)
                  .join(' · ')}
              </Typography.Text>
            </>
          ) : (
            <Typography.Text type="secondary">
              {[driverName, connection.address].filter(Boolean).join(' · ') || 'unknown'}
            </Typography.Text>
          )}
        </Descriptions.Item>
        <Descriptions.Item label="Database">
          {namespace || <span className="dm-null">none</span>}
        </Descriptions.Item>
        <Descriptions.Item label="Table">
          {entry.table || <span className="dm-null">—</span>}
        </Descriptions.Item>
        <Descriptions.Item label="Run from">
          {SOURCE_LABEL[entry.source] ?? entry.source}
        </Descriptions.Item>
      </Descriptions>

      {entry.table ? null : (
        <Typography.Paragraph type="secondary" className="dm-changelog-note">
          No table is recorded for this statement: either it is the statement that creates the
          table — nothing points at it yet, and the name it introduces is in the statement — or the
          window it was run from was not open on one, as a query window is not.
        </Typography.Paragraph>
      )}

      {entry.error ? (
        <Alert
          type="error"
          showIcon
          message="The run ended with this"
          description={entry.error}
          style={{ marginBottom: 12 }}
        />
      ) : null}

      <div className="dm-changelog-statement-head">
        <Typography.Text strong>Executed statement</Typography.Text>
        <Tooltip title="Copy the statement">
          <Button size="small" type="text" icon={<CopyOutlined />} onClick={() => void copy()} />
        </Tooltip>
      </div>
      <SqlCode sql={entry.statement} driver={connection.driver} className="dm-changelog-code" />
    </div>
  )
}

/**
 * How one file reads in the picker: its name, then what is in it — the file
 * being written says so, an archive says the day it was taken out.
 */
function fileLabel(file: ChangeLogFile): string {
  const size = `${formatBytes(file.bytes)}, ${file.entries.toLocaleString()} statement${file.entries === 1 ? '' : 's'}`
  if (!file.archived) return `${file.name} — being written (${size})`
  return `${file.name} — archived ${shortDay(file.at ?? 0)} (${size})`
}

/** Just the date, for a line that is one line. */
function shortDay(ms: number): string {
  if (!ms) return ''
  const at = new Date(ms)
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())}`
}

/** The driver's display name, falling back to the stored id. */
function driverLabel(
  drivers: { type: string; displayName: string }[],
  driver?: string,
): string {
  if (!driver) return ''
  return drivers.find((d) => d.type === driver)?.displayName ?? driver
}

/** One line, so a row in the list stays a row. */
function oneLine(sql: string): string {
  return sql.replace(/\s+/g, ' ').trim()
}

/**
 * A compact stamp for the list — the detail pane shows the full one — with the
 * year only when the entry is not from this one.
 */
function shortStamp(ms: number): string {
  const at = new Date(ms)
  const pad = (value: number) => String(value).padStart(2, '0')
  const year = at.getFullYear() === new Date().getFullYear() ? '' : `${at.getFullYear()}-`
  return `${year}${pad(at.getMonth() + 1)}-${pad(at.getDate())} ${pad(at.getHours())}:${pad(at.getMinutes())}:${pad(at.getSeconds())}`
}
