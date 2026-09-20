import { Empty, Table, Tooltip, Typography } from 'antd'
import type { TableColumnsType } from 'antd'
import { InfoCircleOutlined } from '@ant-design/icons'

import type { OverviewGroup, OverviewTable } from '../../api/types'

/**
 * The pieces every engine view is built from.
 *
 * They are presentation only: no engine decides what to show here, so a view
 * stays a plain reading of its own payload.
 */

/** One titled block of labelled numbers. */
export function MetricGroup({ group }: { group: OverviewGroup }) {
  return (
    <section className="dm-metric-group">
      <header className="dm-metric-group-title">
        <span>{group.title}</span>
        {group.note ? (
          <Tooltip title={group.note}>
            <InfoCircleOutlined style={{ opacity: 0.45, fontSize: 12 }} />
          </Tooltip>
        ) : null}
      </header>
      {group.metrics.length === 0 ? (
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          Nothing to report.
        </Typography.Text>
      ) : (
        <div className="dm-metric-grid">
          {group.metrics.map((metric) => (
            <div
              key={metric.label}
              className={metric.state === 'warn' ? 'dm-metric is-warn' : 'dm-metric'}
            >
              <div className="dm-metric-label">{metric.label}</div>
              <Tooltip title={metric.hint}>
                <div className="dm-metric-value">{metric.value}</div>
              </Tooltip>
            </div>
          ))}
        </div>
      )}
      {group.note ? <div className="dm-metric-note">{group.note}</div> : null}
    </section>
  )
}

/** A capped list (process list, databases, schema objects). */
export function DataTable({ table }: { table: OverviewTable }) {
  const columns: TableColumnsType<string[]> = table.columns.map((title, index) => ({
    title,
    dataIndex: index,
    key: `${index}`,
    ellipsis: index >= 3,
    render: (value: string) =>
      value === '' ? <span className="dm-null">NULL</span> : <span>{value}</span>,
  }))

  return (
    <section className="dm-metric-group">
      <header className="dm-metric-group-title">
        <span>{table.title}</span>
      </header>
      {table.rows.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="Nothing running" />
      ) : (
        <>
          <Table<string[]>
            size="small"
            rowKey={(_, index) => String(index)}
            columns={columns}
            dataSource={table.rows}
            pagination={table.rows.length > 12 ? { pageSize: 12, size: 'small' } : false}
            scroll={{ x: 'max-content' }}
          />
          {table.note ? <div className="dm-metric-note">{table.note}</div> : null}
        </>
      )}
    </section>
  )
}
