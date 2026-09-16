import { useCallback, useEffect, useState } from 'react'
import {
  Alert,
  App as AntApp,
  Button,
  Descriptions,
  Empty,
  Space,
  Spin,
  Table,
  Tag,
  Tooltip,
  Typography,
} from 'antd'
import type { TableColumnsType } from 'antd'
import { CopyOutlined, DownloadOutlined, ReloadOutlined } from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type {
  ColumnInfo,
  ForeignKeyInfo,
  IndexInfo,
  TableStructure,
} from '../api/types'
import { formatBytes, formatCount } from '../lib/format'
import { useAppStore } from '../store/appStore'

/** Which slice of the structure a table window is showing. */
export type StructureSection = 'structure' | 'indexes' | 'foreignKeys' | 'ddl'

interface StructureViewProps {
  sessionId: string
  database: string
  schema: string
  object: string
  /** Which slice to render. */
  section: StructureSection
  /** Bumping this value forces a reload. */
  reloadToken?: number
}

/**
 * One slice of an object's definition.
 *
 * The whole structure is fetched once and cached for the lifetime of the
 * window, so switching between Columns / Indexes / Foreign keys / DDL is
 * instant and costs no extra round trip.
 */
export function StructureView({
  sessionId,
  database,
  schema,
  object,
  section,
  reloadToken = 0,
}: StructureViewProps) {
  const { message } = AntApp.useApp()
  const appInfo = useAppStore((s) => s.appInfo)
  const [structure, setStructure] = useState<TableStructure | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    api
      .getStructure(sessionId, database, schema, object)
      .then((value) => {
        if (!cancelled) setStructure(value)
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(toMessage(err))
          setStructure(null)
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [attempt, database, object, reloadToken, schema, sessionId])

  const saveDDL = useCallback(async () => {
    if (!structure) return
    try {
      await api.saveTextFile({
        defaultFilename: `${object}.sql`,
        content: structure.ddl,
        filters: [{ displayName: 'SQL', pattern: '*.sql' }],
      })
    } catch (err) {
      const text = toMessage(err)
      if (!/cancel/i.test(text)) message.error(text)
    }
  }, [message, object, structure])

  if (loading && !structure) {
    return (
      <div style={{ padding: 40, textAlign: 'center' }}>
        <Spin />
      </div>
    )
  }

  if (error) {
    return (
      <div style={{ padding: 16 }}>
        <Alert
          type="error"
          showIcon
          message="Could not read the structure"
          description={<span className="mono">{error}</span>}
          action={
            <Button size="small" icon={<ReloadOutlined />} onClick={() => setAttempt((n) => n + 1)}>
              Retry
            </Button>
          }
        />
      </div>
    )
  }

  if (!structure) {
    return <Empty description="No structure available" style={{ marginTop: 60 }} />
  }

  const info = structure.object

  if (section === 'indexes') {
    return (
      <div className="dm-pane-body" style={{ padding: 12 }}>
        {structure.indexes.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No indexes" />
        ) : (
          <Table
            className="dm-grid"
            size="small"
            rowKey="name"
            columns={indexColumns}
            dataSource={structure.indexes}
            pagination={false}
            scroll={{ x: 'max-content' }}
          />
        )}
      </div>
    )
  }

  if (section === 'foreignKeys') {
    return (
      <div className="dm-pane-body" style={{ padding: 12 }}>
        {structure.foreignKeys.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No foreign keys" />
        ) : (
          <Table
            className="dm-grid"
            size="small"
            rowKey="name"
            columns={fkColumns}
            dataSource={structure.foreignKeys}
            pagination={false}
            scroll={{ x: 'max-content' }}
          />
        )}
      </div>
    )
  }

  if (section === 'ddl') {
    return (
      <div className="dm-pane-body" style={{ padding: 12 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
          <Typography.Title level={5} style={{ margin: 0 }}>
            DDL
          </Typography.Title>
          <span style={{ flex: 1 }} />
          <Tooltip title="Copy DDL">
            <Button
              size="small"
              icon={<CopyOutlined />}
              onClick={() => {
                void navigator.clipboard
                  .writeText(structure.ddl)
                  .then(() => message.success('DDL copied'))
                  .catch((err: unknown) => message.error(toMessage(err)))
              }}
            >
              Copy
            </Button>
          </Tooltip>
          <Button size="small" icon={<DownloadOutlined />} onClick={() => void saveDDL()}>
            Save
          </Button>
        </div>
        <pre className="dm-ddl">{structure.ddl}</pre>
        <Typography.Paragraph type="secondary" style={{ fontSize: 11, marginTop: 12 }}>
          {appInfo ? `${appInfo.name} ${appInfo.version} · ` : ''}
          {structure.ddl.startsWith('--') ? 'DDL reconstructed from catalog metadata.' : ''}
        </Typography.Paragraph>
      </div>
    )
  }

  // section === 'structure'
  return (
    <div className="dm-pane-body" style={{ padding: 12 }}>
      <Descriptions
        size="small"
        column={4}
        bordered
        style={{ marginBottom: 16 }}
        items={[
          { key: 'name', label: 'Object', children: <span className="mono">{info.name}</span> },
          { key: 'kind', label: 'Kind', children: info.kind },
          {
            key: 'schema',
            label: 'Namespace',
            children: (
              <span className="mono">
                {[info.database, info.schema].filter(Boolean).join('.') || '—'}
              </span>
            ),
          },
          { key: 'engine', label: 'Engine', children: info.engine || '—' },
          {
            key: 'rows',
            label: 'Row estimate',
            children: formatCount(info.rowEstimate),
          },
          { key: 'size', label: 'Size', children: formatBytes(info.sizeBytes) },
          {
            key: 'columns',
            label: 'Columns',
            children: structure.columns.length,
          },
          {
            key: 'comment',
            label: 'Comment',
            children: info.comment || '—',
          },
        ]}
      />

      <Typography.Title level={5} style={{ marginTop: 0 }}>
        Columns
      </Typography.Title>
      <Table
        className="dm-grid"
        size="small"
        rowKey="name"
        columns={columnColumns}
        dataSource={structure.columns}
        pagination={false}
        scroll={{ x: 'max-content' }}
      />
    </div>
  )
}

/* ------------------------------------------------------------- column sets */

const columnColumns: TableColumnsType<ColumnInfo> = [
  { title: '#', dataIndex: 'ordinal', width: 52, align: 'right' },
  {
    title: 'Name',
    dataIndex: 'name',
    render: (name: string, row) => (
      <Space size={6}>
        <span className="mono">{name}</span>
        {row.primaryKey ? <Tag color="gold">PK</Tag> : null}
        {row.autoIncrement ? <Tag color="blue">auto</Tag> : null}
      </Space>
    ),
  },
  {
    title: 'Type',
    dataIndex: 'dataType',
    render: (type: string, row) => (
      <Tooltip title={row.columnType}>
        <span className="mono">{type}</span>
      </Tooltip>
    ),
  },
  {
    title: 'Nullable',
    dataIndex: 'nullable',
    width: 92,
    render: (nullable: boolean) =>
      nullable ? (
        <Typography.Text type="secondary">yes</Typography.Text>
      ) : (
        <Typography.Text strong>no</Typography.Text>
      ),
  },
  {
    title: 'Default',
    dataIndex: 'defaultValue',
    render: (value: string | null | undefined) =>
      value === null || value === undefined ? (
        <span className="dm-null">—</span>
      ) : (
        <span className="mono">{value}</span>
      ),
  },
  {
    title: 'Comment',
    dataIndex: 'comment',
    render: (value: string) => value || <span className="dm-null">—</span>,
  },
]

const indexColumns: TableColumnsType<IndexInfo> = [
  {
    title: 'Name',
    dataIndex: 'name',
    render: (name: string) => <span className="mono">{name}</span>,
  },
  {
    title: 'Columns',
    dataIndex: 'columns',
    render: (columns: string[]) => (
      <Space size={4} wrap>
        {columns.map((column, i) => (
          <Tag key={`${column}-${i}`} className="mono">
            {column}
          </Tag>
        ))}
      </Space>
    ),
  },
  {
    title: 'Unique',
    dataIndex: 'unique',
    width: 90,
    render: (unique: boolean, row) =>
      row.primary ? <Tag color="gold">PRIMARY</Tag> : unique ? <Tag color="green">unique</Tag> : '—',
  },
  {
    title: 'Method',
    dataIndex: 'method',
    width: 110,
    render: (value: string) => value || <span className="dm-null">—</span>,
  },
]

const fkColumns: TableColumnsType<ForeignKeyInfo> = [
  {
    title: 'Name',
    dataIndex: 'name',
    render: (name: string) => <span className="mono">{name}</span>,
  },
  {
    title: 'Columns',
    dataIndex: 'columns',
    render: (columns: string[]) => <span className="mono">{columns.join(', ')}</span>,
  },
  {
    title: 'References',
    key: 'references',
    render: (_value, row) => (
      <span className="mono">
        {[row.referencedSchema, row.referencedTable].filter(Boolean).join('.')}(
        {row.referencedColumns.join(', ')})
      </span>
    ),
  },
  {
    title: 'On delete',
    dataIndex: 'onDelete',
    width: 110,
    render: (value: string) => value || <span className="dm-null">—</span>,
  },
  {
    title: 'On update',
    dataIndex: 'onUpdate',
    width: 110,
    render: (value: string) => value || <span className="dm-null">—</span>,
  },
]
