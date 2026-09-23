import { useCallback, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import { Button, Input, Pagination, Space, Table, Tooltip, Typography } from 'antd'
import type { TableColumnsType, TableProps } from 'antd'
import { CloseOutlined, LeftOutlined, RightOutlined } from '@ant-design/icons'

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
  /**
   * The row whose detail layer is on screen, as an index into the page, and the
   * way to open or close it. The two belong together: a grid that is told which
   * row is open is a grid whose rows can be clicked.
   */
  detailRow?: number | null
  onDetailRowChange?: (rowIndex: number | null) => void
  /**
   * The detail of one row, rendered into the layer that slides in from the right.
   * Passing it is what makes rows clickable; the content is the caller's, because
   * only the caller knows what the columns of this result mean.
   */
  renderRowDetail?: (row: CellValue[], rowIndex: number) => ReactNode
}

const MIN_COLUMN_WIDTH = 90
const MAX_COLUMN_WIDTH = 320
/** Where a column of unbounded text stops growing — see `looksLikeParagraph`. */
const PARAGRAPH_COLUMN_WIDTH = 200
/** …and where a column of ordinary values does. */
const VALUE_COLUMN_WIDTH = 260
/** Roughly one character of the grid's monospace font, and a cell's padding. */
const CHAR_WIDTH = 7.2
const CELL_PADDING = 26

/**
 * Whether a database type holds text of an unbounded length.
 *
 * A length is not a paragraph: `varchar(120)` is a label, while a `text` column —
 * or a `varchar` with no length at all — is a document.
 */
const PARAGRAPH_TYPES = new Set([
  'text',
  'tinytext',
  'mediumtext',
  'longtext',
  'ntext',
  'blob',
  'tinyblob',
  'mediumblob',
  'longblob',
  'bytea',
  'image',
  'json',
  'jsonb',
  'clob',
  'nclob',
  'xml',
])

export function looksLikeParagraph(databaseType?: string): boolean {
  const type = (databaseType ?? '').trim().toLowerCase()
  const base = type.split('(')[0].trim()
  if (PARAGRAPH_TYPES.has(base)) return true
  return !type.includes('(') && (base === 'varchar' || base === 'character varying')
}

/**
 * Estimates a readable column width from the header and the first rows.
 *
 * The longest value is the wrong thing to measure: one essay in a column of
 * labels would set the width of every row, and the grid would scroll sideways for
 * good. The upper quartile is what most rows need, and a column of unbounded text
 * is capped lower still — its long values are there to be read, on hover or in the
 * row detail, not to be laid out in the grid.
 */
function estimateWidth(column: ColumnMeta, rows: CellValue[][], index: number): number {
  const lengths: number[] = []
  const sample = Math.min(rows.length, 50)
  for (let i = 0; i < sample; i += 1) {
    const value = rows[i]?.[index]
    // NULL is rendered as a word, so it measures as one.
    lengths.push((value === null || value === undefined ? 'NULL' : String(value)).length)
  }
  lengths.sort((a, b) => a - b)
  const quartile = lengths[Math.max(0, Math.ceil(lengths.length * 0.75) - 1)] ?? 0
  const header = column.name.length * CHAR_WIDTH + CELL_PADDING
  const value = quartile * CHAR_WIDTH + CELL_PADDING
  const cap = looksLikeParagraph(column.databaseType) ? PARAGRAPH_COLUMN_WIDTH : VALUE_COLUMN_WIDTH
  const wanted = Math.max(header, Math.min(cap, value))
  return Math.max(MIN_COLUMN_WIDTH, Math.min(MAX_COLUMN_WIDTH, Math.round(wanted)))
}

/**
 * Renders one cell value.
 *
 * Long values are clipped with the full text available on hover, and NULL gets
 * its own style so it can never be confused with the string "NULL". The clip is
 * the column's own width when the caller knows it: a long value must not be able
 * to widen the column it sits in beyond what the column asked for.
 */
function CellContent({ value, width = 360 }: { value: CellValue; width?: number }) {
  if (value === null || value === undefined) {
    return <span className="dm-null">NULL</span>
  }
  if (typeof value === 'boolean') {
    return <span className="mono">{value ? 'true' : 'false'}</span>
  }
  const text = String(value)
  // A number is never clipped: its digits are the value, and an ellipsis would
  // hide the one thing the cell has to say. Text is clipped to the column.
  if (typeof value === 'number') {
    return <span className="mono">{text}</span>
  }
  const cap = Math.max(60, width - 16)
  const clipped = text.length * CHAR_WIDTH > cap
  const span = (
    <span className="dm-truncate" style={{ maxWidth: cap }}>
      {text.length > 400 ? `${text.slice(0, 400)}…` : text}
    </span>
  )
  if (!clipped) return span
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
        {span}
      </Tooltip>
    )
  }
  return <Tooltip title={text}>{span}</Tooltip>
}

/** A cell that becomes an input on double click. */
function EditableCell({
  value,
  width,
  onCommit,
}: {
  value: CellValue
  width: number
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
        <CellContent value={value} width={width} />
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
  detailRow,
  onDetailRowChange,
  renderRowDetail,
}: DataGridProps) {
  const canEdit = Boolean(editable && onEditCell && primaryKey && primaryKey.length > 0)

  // Which row's detail is on screen, and which one the layer is showing right
  // now: they differ while the layer slides out, when the row that was open has
  // to stay in it until it is out of sight. The layer itself is always there — an
  // element that appears already in its open state does not slide, it blinks.
  const openAt = detailRow ?? null
  const [lastAt, setLastAt] = useState<number | null>(null)
  useEffect(() => {
    if (openAt !== null) setLastAt(openAt)
  }, [openAt])
  const shownAt = openAt ?? lastAt

  useEffect(() => {
    if (openAt === null || !onDetailRowChange) return undefined
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onDetailRowChange(null)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [onDetailRowChange, openAt])

  const columns = useMemo<TableColumnsType<CellValue[]>>(() => {
    const pk = new Set((primaryKey ?? []).map((column) => column.toLowerCase()))
    const widths = result.columns.map((column, index) =>
      estimateWidth(column, result.rows, index),
    )
    const budget = widths.reduce((sum, width) => sum + width, 0)
    const scale = budget > 1400 ? 1400 / budget : 1

    return result.columns.map((column, index) => {
      const sortSpec = sort?.find((entry) => entry.column === column.name)
      const width = Math.round(widths[index] * scale)
      return {
        key: column.name,
        title: (
          <span title={`${column.name}${column.databaseType ? ` · ${column.databaseType}` : ''}`}>
            {column.name}
            {pk.has(column.name.toLowerCase()) ? <span style={{ opacity: 0.45 }}> 🔑</span> : null}
          </span>
        ),
        dataIndex: index,
        width,
        sorter: onSortChange ? true : false,
        sortOrder: sortSpec ? (sortSpec.desc ? 'descend' : 'ascend') : null,
        render: (value: CellValue, _row: CellValue[], rowIndex: number) => {
          if (canEdit && onEditCell) {
            return (
              <EditableCell
                value={value}
                width={width}
                onCommit={(next) => onEditCell(rowIndex, column.name, next, value)}
              />
            )
          }
          return <CellContent value={value} width={width} />
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
            onClick: (event) => {
              if (!onDetailRowChange || !renderRowDetail || index === undefined) return
              // A click on a control is a click on the control: a link or button in
              // a cell, or a cell that has already opened its editor, must not be
              // read as "show me this row". A click on a cell that is merely
              // editable still is one — the editor opens on the second click.
              const target = event.target as HTMLElement
              if (target.closest('button, a, textarea, .ant-select, .dm-grid-cell input')) {
                return
              }
              onDetailRowChange(index)
            },
          })}
          rowClassName={(_record, index) =>
            index === openAt && openAt !== null ? 'dm-grid-row is-detail' : 'dm-grid-row'
          }
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
      {renderRowDetail ? (
        <aside
          className={openAt !== null ? 'dm-row-detail-layer is-open' : 'dm-row-detail-layer'}
        >
          <button
            type="button"
            className="dm-row-detail-close"
            aria-label="Close row detail"
            onClick={() => onDetailRowChange?.(null)}
          >
            <CloseOutlined />
          </button>
          {shownAt !== null && result.rows[shownAt]
            ? renderRowDetail(result.rows[shownAt], shownAt)
            : null}
        </aside>
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
