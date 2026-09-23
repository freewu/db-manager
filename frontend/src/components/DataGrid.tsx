import { useCallback, useMemo, useState } from 'react'
import { Button, Input, Pagination, Space, Table, Tooltip, Typography } from 'antd'
import type { TableColumnsType, TableProps } from 'antd'
import { LeftOutlined, RightOutlined } from '@ant-design/icons'

import type { CellValue, ColumnMeta, QueryResult, SortSpec } from '../api/types'
import { useColumnResize } from './ResizableHeader'

export interface DataGridProps {
  result: QueryResult
  loading?: boolean
  /** Zero-based offset of the first row, used for the row-number column. */
  rowOffset?: number
  /** Server-side sorting: pass a handler and the headers become sortable. */
  sort?: SortSpec[]
  onSortChange?: (next: SortSpec[]) => void
  /** Server-side paging. */
  page?: number
  pageSize?: number
  total?: number
  hasTotal?: boolean
  onPageChange?: (page: number, pageSize: number) => void
  /** Primary key columns; when present, cells become editable. */
  primaryKey?: string[]
  editable?: boolean
  onEditCell?: (
    rowIndex: number,
    column: string,
    next: string | null,
    previous: CellValue,
  ) => Promise<void>
  selectedKeys?: readonly number[]
  onSelectionChange?: (keys: number[]) => void
  /** Fired when a row is double clicked and the grid is not editable. */
  onOpenRow?: (rowIndex: number) => void
}

const MIN_COLUMN_WIDTH = 90
const MAX_COLUMN_WIDTH = 420

/** Estimates a readable column width from the header and the first rows. */
function estimateWidth(column: ColumnMeta, rows: CellValue[][], index: number): number {
  let longest = column.name.length
  const sample = Math.min(rows.length, 50)
  for (let i = 0; i < sample; i += 1) {
    const value = rows[i]?.[index]
    if (value === null || value === undefined) continue
    const text = typeof value === 'string' ? value : String(value)
    if (text.length > longest) longest = text.length
    if (longest > 40) break
  }
  const estimated = longest * 7.2 + 26
  return Math.max(MIN_COLUMN_WIDTH, Math.min(MAX_COLUMN_WIDTH, Math.round(estimated)))
}

/**
 * Renders one cell value.
 *
 * Long values are clipped with the full text available on hover, and NULL gets
 * its own style so it can never be confused with the string "NULL".
 */
function CellContent({ value }: { value: CellValue }) {
  if (value === null || value === undefined) {
    return <span className="dm-null">NULL</span>
  }
  if (typeof value === 'boolean') {
    return <span className="mono">{value ? 'true' : 'false'}</span>
  }
  const text = String(value)
  if (text.length > 400) {
    return (
      <Tooltip
        styles={{ root: { maxWidth: 560 } }}
        title={
          <div className="dm-value-view" style={{ maxHeight: 340, overflow: 'auto' }}>
            {text}
          </div>
        }
      >
        <span className="dm-truncate" style={{ maxWidth: 'min(360px, 100%)' }}>
          {text.slice(0, 400)}…
        </span>
      </Tooltip>
    )
  }
  if (text.length > 40) {
    return (
      <Tooltip title={text}>
        <span className="dm-truncate" style={{ maxWidth: 'min(360px, 100%)' }}>
          {text}
        </span>
      </Tooltip>
    )
  }
  return <span className="mono">{text}</span>
}

/** A cell that becomes an input on double click. */
function EditableCell({
  value,
  onCommit,
}: {
  value: CellValue
  onCommit: (next: string | null) => Promise<void>
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')
  const [busy, setBusy] = useState(false)

  const begin = useCallback(() => {
    setDraft(value === null || value === undefined ? '' : String(value))
    setEditing(true)
  }, [value])

  const commit = useCallback(async () => {
    if (busy) return
    const original = value === null || value === undefined ? '' : String(value)
    if (draft === original) {
      setEditing(false)
      return
    }
    setBusy(true)
    try {
      // An emptied field means NULL: there is no way to type "the string NULL".
      await onCommit(draft === '' ? null : draft)
      setEditing(false)
    } finally {
      setBusy(false)
    }
  }, [busy, draft, onCommit, value])

  if (!editing) {
    return (
      <div className="dm-grid-cell" onDoubleClick={begin} title="Double click to edit">
        <CellContent value={value} />
      </div>
    )
  }

  return (
    <Input
      size="small"
      autoFocus
      value={draft}
      disabled={busy}
      onChange={(event) => setDraft(event.target.value)}
      onPressEnter={() => void commit()}
      onBlur={() => void commit()}
      onKeyDown={(event) => {
        if (event.key === 'Escape') setEditing(false)
      }}
    />
  )
}

/** Result grid shared by the query and table panes. */
export function DataGrid({
  result,
  loading,
  rowOffset = 0,
  sort,
  onSortChange,
  page,
  pageSize,
  total,
  hasTotal,
  onPageChange,
  primaryKey,
  editable,
  onEditCell,
  selectedKeys,
  onSelectionChange,
  onOpenRow,
}: DataGridProps) {
  const canEdit = Boolean(editable && onEditCell && primaryKey && primaryKey.length > 0)

  const columns = useMemo<TableColumnsType<CellValue[]>>(() => {
    const pk = new Set((primaryKey ?? []).map((column) => column.toLowerCase()))
    const widths = result.columns.map((column, index) =>
      estimateWidth(column, result.rows, index),
    )
    const budget = widths.reduce((sum, width) => sum + width, 0)
    const scale = budget > 1400 ? 1400 / budget : 1

    return result.columns.map((column, index) => {
      const sortSpec = sort?.find((entry) => entry.column === column.name)
      return {
        key: column.name,
        title: (
          <span title={`${column.name}${column.databaseType ? ` · ${column.databaseType}` : ''}`}>
            {column.name}
            {pk.has(column.name.toLowerCase()) ? <span style={{ opacity: 0.45 }}> 🔑</span> : null}
          </span>
        ),
        dataIndex: index,
        width: Math.round(widths[index] * scale),
        sorter: onSortChange ? true : false,
        sortOrder: sortSpec ? (sortSpec.desc ? 'descend' : 'ascend') : null,
        render: (value: CellValue, _row: CellValue[], rowIndex: number) => {
          if (canEdit && onEditCell) {
            return (
              <EditableCell
                value={value}
                onCommit={(next) => onEditCell(rowIndex, column.name, next, value)}
              />
            )
          }
          return <CellContent value={value} />
        },
      } satisfies TableColumnsType<CellValue[]>[number]
    })
  }, [canEdit, onEditCell, onSortChange, primaryKey, result.columns, result.rows, sort])

  // The columns keep their estimated widths until one of the edges is dragged.
  const grid = useColumnResize(columns)

  const handleChange: TableProps<CellValue[]>['onChange'] = (pagination, _filters, sorter) => {
    if (onSortChange) {
      const list = Array.isArray(sorter) ? sorter : [sorter]
      const next: SortSpec[] = []
      for (const entry of list) {
        if (!entry?.order) continue
        const name = String(entry.columnKey ?? '')
        if (!name) continue
        next.push({ column: name, desc: entry.order === 'descend' })
      }
      onSortChange(next)
    }
    if (onPageChange) {
      onPageChange(pagination.current ?? 1, pagination.pageSize ?? pageSize ?? 50)
    }
  }

  return (
    <div className="dm-result-area">
      <div className="dm-pane-body" style={{ flex: '1 1 auto' }}>
        <Table<CellValue[]>
          className={grid.resized ? 'dm-grid dm-grid-resized' : 'dm-grid'}
          {...grid.tableProps}
          size="small"
          loading={loading}
          columns={grid.columns}
          dataSource={result.rows}
          rowKey={(_row, index) => String((index ?? 0) + rowOffset)}
          scroll={{ x: 'max-content' }}
          onChange={handleChange}
          rowSelection={
            onSelectionChange
              ? {
                  selectedRowKeys: selectedKeys ? [...selectedKeys] : [],
                  onChange: (keys) => onSelectionChange(keys.map(Number)),
                  columnWidth: 36,
                  fixed: true,
                }
              : undefined
          }
          onRow={(_record, index) => ({
            onDoubleClick: () => {
              if (!canEdit && onOpenRow && index !== undefined) onOpenRow(index)
            },
          })}
          pagination={false}
          locale={{ emptyText: loading ? ' ' : 'No rows' }}
        />
      </div>
      {onPageChange ? (
        <Pager
          page={page ?? 1}
          pageSize={pageSize ?? 50}
          total={total ?? 0}
          hasTotal={hasTotal ?? false}
          rowCount={result.rows.length}
          onChange={onPageChange}
        />
      ) : null}
    </div>
  )
}

function Pager({
  page,
  pageSize,
  total,
  hasTotal,
  rowCount,
  onChange,
}: {
  page: number
  pageSize: number
  total: number
  hasTotal: boolean
  rowCount: number
  onChange: (page: number, pageSize: number) => void
}) {
  if (hasTotal) {
    return (
      <div
        style={{
          display: 'flex',
          justifyContent: 'flex-end',
          padding: '4px 10px',
          borderTop: '1px solid var(--dm-border)',
        }}
      >
        <Pagination
          size="small"
          current={page}
          pageSize={pageSize}
          total={total}
          showSizeChanger
          pageSizeOptions={[25, 50, 100, 200, 500]}
          showTotal={(count, range) => `${range[0]}–${range[1]} of ${count.toLocaleString()}`}
          onChange={onChange}
        />
      </div>
    )
  }

  const first = rowCount === 0 ? 0 : (page - 1) * pageSize + 1
  const last = (page - 1) * pageSize + rowCount

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        justifyContent: 'flex-end',
        padding: '4px 10px',
        borderTop: '1px solid var(--dm-border)',
      }}
    >
      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
        rows {first}–{last}
      </Typography.Text>
      <Space size={4}>
        <Button
          size="small"
          icon={<LeftOutlined />}
          disabled={page <= 1}
          onClick={() => onChange(page - 1, pageSize)}
        />
        <span style={{ fontSize: 12 }}>page {page}</span>
        <Button
          size="small"
          icon={<RightOutlined />}
          disabled={rowCount < pageSize}
          onClick={() => onChange(page + 1, pageSize)}
        />
      </Space>
      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
        {pageSize}/page
      </Typography.Text>
    </div>
  )
}

export { CellContent }
