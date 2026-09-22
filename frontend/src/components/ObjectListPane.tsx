import { useCallback, useEffect, useMemo, useState } from 'react'
import { App as AntApp, Button, Dropdown, Empty, Input, Space, Spin, Table, Tooltip, Typography } from 'antd'
import type { MenuProps, TableColumnsType } from 'antd'
import {
  AppstoreOutlined,
  CheckOutlined,
  EditOutlined,
  KeyOutlined,
  MoreOutlined,
  NumberOutlined,
  ReloadOutlined,
  SearchOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons'

import type { IndexEntry, ObjectInfo } from '../api/types'
import { formatBytes, formatCount } from '../lib/format'
import { indexesKey, KIND_SINGULAR, namespaceKey, objectsKey } from '../lib/tree'
import { useAppStore, type WorkspaceTab } from '../store/appStore'
import { objectIcon } from './objectIcon'

/** Engines can only estimate rows/bytes for relation-like objects. */
function measured(kind: ObjectInfo['kind']): boolean {
  return kind === 'table' || kind === 'materialized_view' || kind === 'collection'
}

/**
 * Navicat-style object list: the middle pane of the main window, listing every
 * object of one explorer folder (`Tables`, `Views`, `Indexes`, …).
 *
 * Data comes from the same cache the explorer tree fills, so opening a folder
 * costs nothing when the tree has already been expanded; the toolbar's reload
 * refreshes the cache — and therefore the tree's counts too.
 */
export function ObjectListPane({ tab }: { tab: WorkspaceTab }) {
  const { message } = AntApp.useApp()
  const tree = useAppStore((s) => s.tree)
  const session = useAppStore((s) => s.sessions.find((s) => s.id === tab.sessionId))
  const loadObjects = useAppStore((s) => s.loadObjects)
  const loadIndexes = useAppStore((s) => s.loadIndexes)
  const openTableTab = useAppStore((s) => s.openTableTab)
  const openQueryTab = useAppStore((s) => s.openQueryTab)

  const [filter, setFilter] = useState('')
  const [selectedKey, setSelectedKey] = useState<string>()

  const { database, schema, list = 'table' } = tab
  const isIndexes = list === 'index'

  const cacheKey = isIndexes
    ? indexesKey(tab.sessionId, database ?? '', schema ?? '')
    : namespaceKey(tab.sessionId, database ?? '', schema ?? '')

  const loading = Boolean(tree.loading[cacheKey])
  const error = tree.errors[cacheKey]

  const objects = tree.objects[objectsKey(tab.sessionId, database ?? '', schema ?? '')]
  const indexes = tree.indexes[cacheKey]

  const loaded = isIndexes ? Boolean(indexes) : Boolean(objects)

  const reload = useCallback(() => {
    if (!database || !schema) return
    if (isIndexes) void loadIndexes(tab.sessionId, database, schema)
    else void loadObjects(tab.sessionId, database, schema)
  }, [database, isIndexes, loadIndexes, loadObjects, schema, tab.sessionId])

  // A window can be opened before its folder was ever expanded in the tree.
  // A recorded error stops the effect from retrying on every render turn — the
  // failure message and the reload buttons own the retry instead; invalidating
  // the session cache clears the error and brings the automatic load back.
  useEffect(() => {
    if (!database || !schema || loaded || loading || error) return
    reload()
  }, [database, error, loaded, loading, reload, schema])

  const rows = useMemo<ObjectInfo[]>(() => {
    if (isIndexes) return []
    return (objects ?? []).filter((object) => object.kind === list)
  }, [isIndexes, list, objects])

  const indexRows = useMemo<IndexEntry[]>(() => (isIndexes ? (indexes ?? []) : []), [indexes, isIndexes])

  const needle = filter.trim().toLowerCase()
  const visibleObjects = useMemo(
    () => (needle ? rows.filter((o) => o.name.toLowerCase().includes(needle)) : rows),
    [needle, rows],
  )
  const visibleIndexes = useMemo(
    () =>
      needle
        ? indexRows.filter(
            (i) =>
              i.name.toLowerCase().includes(needle) || i.table.toLowerCase().includes(needle),
          )
        : indexRows,
    [indexRows, needle],
  )

  const openObject = useCallback(
    (object: ObjectInfo, view?: 'data' | 'structure' | 'indexes') => {
      if (!database || !schema) return
      openTableTab(tab.sessionId, database, schema, object, view ?? 'data')
    },
    [database, openTableTab, schema, tab.sessionId],
  )

  /** Indexes belong to a table; open the owning table's index sub-view. */
  const openIndex = useCallback(
    (index: IndexEntry) => {
      const owner = (objects ?? []).find((o) => o.name === index.table)
      const object: ObjectInfo = owner ?? {
        name: index.table,
        kind: 'table',
        rowEstimate: -1,
        sizeBytes: -1,
      }
      openObject(object, 'indexes')
    },
    [objects, openObject],
  )

  const copyName = useCallback(
    async (name: string) => {
      try {
        await navigator.clipboard.writeText(name)
        message.success(`Copied ${name}`)
      } catch {
        message.warning('Clipboard is not available')
      }
    },
    [message],
  )

  const rowMenu = useCallback(
    (object: ObjectInfo): MenuProps => ({
      items: [
        {
          key: 'data',
          icon: <UnorderedListOutlined />,
          label: 'Open data',
          onClick: () => openObject(object, 'data'),
        },
        {
          key: 'structure',
          icon: <AppstoreOutlined />,
          label: `Design ${KIND_SINGULAR[object.kind]}`,
          onClick: () => openObject(object, 'structure'),
        },
        {
          key: 'query',
          icon: <EditOutlined />,
          label: 'New query',
          onClick: () => openQueryTab(tab.sessionId, database),
        },
        { type: 'divider' as const },
        {
          key: 'copy',
          icon: <NumberOutlined />,
          label: 'Copy name',
          onClick: () => void copyName(object.name),
        },
      ],
    }),
    [copyName, database, openObject, openQueryTab, tab.sessionId],
  )

  const objectColumns = useMemo<TableColumnsType<ObjectInfo>>(
    () => [
      {
        title: 'Name',
        dataIndex: 'name',
        sorter: (a, b) => a.name.localeCompare(b.name),
        defaultSortOrder: 'ascend',
        render: (name: string, row) => (
          <Space size={6}>
            <span style={{ opacity: 0.65 }}>{objectIcon(row.kind)}</span>
            <span className="dm-list-name">{name}</span>
          </Space>
        ),
      },
      {
        title: 'Type',
        dataIndex: 'kind',
        width: 150,
        render: (kind: ObjectInfo['kind']) => (
          <Typography.Text type="secondary">{KIND_SINGULAR[kind]}</Typography.Text>
        ),
      },
      {
        title: 'Rows',
        dataIndex: 'rowEstimate',
        width: 110,
        align: 'right',
        sorter: (a, b) => a.rowEstimate - b.rowEstimate,
        render: (value: number, row) =>
          measured(row.kind) ? (
            <span className="mono">{formatCount(value)}</span>
          ) : (
            <Typography.Text type="secondary">—</Typography.Text>
          ),
      },
      {
        title: 'Size',
        dataIndex: 'sizeBytes',
        width: 110,
        align: 'right',
        sorter: (a, b) => a.sizeBytes - b.sizeBytes,
        render: (value: number, row) =>
          measured(row.kind) ? (
            <span className="mono">{formatBytes(value)}</span>
          ) : (
            <Typography.Text type="secondary">—</Typography.Text>
          ),
      },
      {
        title: 'Engine',
        dataIndex: 'engine',
        width: 130,
        render: (engine?: string) =>
          engine ? <span className="mono">{engine}</span> : <Typography.Text type="secondary">—</Typography.Text>,
      },
      {
        title: 'Comment',
        dataIndex: 'comment',
        render: (comment?: string) =>
          comment ? (
            <Tooltip title={comment}>
              <span className="dm-truncate">{comment}</span>
            </Tooltip>
          ) : (
            <Typography.Text type="secondary">—</Typography.Text>
          ),
      },
      {
        title: '',
        key: 'actions',
        width: 48,
        align: 'right',
        render: (_: unknown, row) => (
          <Dropdown menu={rowMenu(row)} trigger={['click']} placement="bottomRight">
            <Button
              size="small"
              type="text"
              icon={<MoreOutlined />}
              onClick={(event) => event.stopPropagation()}
            />
          </Dropdown>
        ),
      },
    ],
    [rowMenu],
  )

  const indexColumns = useMemo<TableColumnsType<IndexEntry>>(
    () => [
      {
        title: 'Name',
        dataIndex: 'name',
        sorter: (a, b) => a.name.localeCompare(b.name),
        defaultSortOrder: 'ascend',
        render: (name: string, row) => (
          <Space size={6}>
            <KeyOutlined style={{ opacity: 0.65, color: row.primary ? '#d4a017' : undefined }} />
            <span className="dm-list-name">{name}</span>
          </Space>
        ),
      },
      {
        title: 'Table',
        dataIndex: 'table',
        sorter: (a, b) => a.table.localeCompare(b.table),
        render: (table: string) => <span className="mono">{table}</span>,
      },
      {
        title: 'Columns',
        dataIndex: 'columns',
        render: (columns: string[]) => (
          <Tooltip title={columns.join(', ')}>
            <span className="mono dm-truncate">{columns.join(', ')}</span>
          </Tooltip>
        ),
      },
      {
        title: 'Unique',
        dataIndex: 'unique',
        width: 90,
        align: 'center',
        filters: [
          { text: 'Unique', value: true },
          { text: 'Not unique', value: false },
        ],
        onFilter: (value, row) => String(row.unique) === String(value),
        render: (unique: boolean) =>
          unique ? <CheckOutlined style={{ color: 'var(--dm-accent)' }} /> : null,
      },
      {
        title: 'Primary',
        dataIndex: 'primary',
        width: 90,
        align: 'center',
        render: (primary: boolean) => (primary ? <CheckOutlined style={{ color: '#d4a017' }} /> : null),
      },
      {
        title: 'Method',
        dataIndex: 'method',
        width: 130,
        render: (method?: string) =>
          method ? <span className="mono">{method}</span> : <Typography.Text type="secondary">—</Typography.Text>,
      },
    ],
    [],
  )

  if (!database || !schema) {
    return <Empty description="No namespace selected" style={{ marginTop: 80 }} />
  }

  const path = schema === database ? database : `${database}.${schema}`
  const total = isIndexes ? indexRows.length : rows.length
  const visible = isIndexes ? visibleIndexes.length : visibleObjects.length

  return (
    <div className="dm-pane">
      <div className="dm-editor-toolbar">
        <Typography.Text className="mono dm-object-path" style={{ fontSize: 12 }}>
          {path}.{tab.title}
        </Typography.Text>

        <div className="dm-toolbar-right">
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {session ? session.name : ''}
          </Typography.Text>
          <Tooltip title="Reload this list">
            <Button size="small" icon={<ReloadOutlined />} loading={loading} onClick={reload} />
          </Tooltip>
        </div>
      </div>

      <div className="dm-pane-body">
        {error ? (
          <div className="dm-empty">
            <Empty
              description={
                <Space direction="vertical" size={2}>
                  <Typography.Text>Could not load {tab.title.toLowerCase()}</Typography.Text>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    {error}
                  </Typography.Text>
                </Space>
              }
            >
              <Button size="small" icon={<ReloadOutlined />} onClick={reload}>
                Try again
              </Button>
            </Empty>
          </div>
        ) : !loaded && loading ? (
          <div className="dm-empty" style={{ textAlign: 'center' }}>
            <Spin />
          </div>
        ) : total === 0 ? (
          <div className="dm-empty">
            <Empty description={`No ${tab.title.toLowerCase()} in ${path}`} />
          </div>
        ) : (
          <Table
            className="dm-grid dm-object-list"
            size="small"
            rowKey={(row) =>
              isIndexes ? `${(row as IndexEntry).table}.${(row as IndexEntry).name}` : row.name
            }
            columns={
              (isIndexes ? indexColumns : objectColumns) as TableColumnsType<
                ObjectInfo | IndexEntry
              >
            }
            dataSource={isIndexes ? visibleIndexes : visibleObjects}
            pagination={false}
            scroll={{ x: 'max-content' }}
            rowClassName={(row) => {
              const key = isIndexes
                ? `${(row as IndexEntry).table}.${(row as IndexEntry).name}`
                : row.name
              return key === selectedKey ? 'dm-object-list-row is-selected' : 'dm-object-list-row'
            }}
            onRow={(row) => ({
              onClick: () => {
                const key = isIndexes
                  ? `${(row as IndexEntry).table}.${(row as IndexEntry).name}`
                  : row.name
                setSelectedKey(key)
                // Navicat opens on double-click; a single click opening the
                // window as well matches how the explorer tree behaves here.
                if (isIndexes) openIndex(row as IndexEntry)
                else openObject(row as ObjectInfo)
              },
              onDoubleClick: () => {
                if (isIndexes) openIndex(row as IndexEntry)
                else openObject(row as ObjectInfo, 'structure')
              },
            })}
          />
        )}
      </div>

      <div className="dm-list-footer">
        <Input
          size="small"
          allowClear
          value={filter}
          prefix={<SearchOutlined style={{ opacity: 0.5 }} />}
          placeholder="Filter"
          style={{ width: 200 }}
          onChange={(event) => setFilter(event.target.value)}
        />
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {needle ? `${visible} of ${total}` : `${total}`}{' '}
          {total === 1 ? 'item' : 'items'}
        </Typography.Text>
        <div className="dm-list-footer-hint">
          <Typography.Text type="secondary" style={{ fontSize: 11 }}>
            Click to open · double-click to design
          </Typography.Text>
        </div>
      </div>
    </div>
  )
}
