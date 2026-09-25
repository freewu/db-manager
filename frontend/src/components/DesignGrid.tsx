import { AutoComplete, Checkbox, Input, Table, Tooltip, Typography } from 'antd'
import { useRef, useState } from 'react'
import type { DragEvent, Key } from 'react'
import { HolderOutlined, KeyOutlined } from '@ant-design/icons'

import type { DesignColumn, DriverType, TableDesign } from '../api/types'
import { typeSuggestions } from '../lib/design'
import { useColumnResize, type ResizableColumns } from './ResizableHeader'
import { t } from '../lib/i18n'

/** Which row of a designer grid is selected, and how to change that. */
export interface GridSelection {
  selected: number | null
  onChange: (index: number | null) => void
}

interface DesignGridProps {
  design: TableDesign
  driver?: DriverType
  readOnly: boolean
  onChange: (next: TableDesign) => void
  selection?: GridSelection
}

/** Row selection used by both grids: one row at a time, keyed by position. */
function rowSelection(selection: GridSelection | undefined) {
  return {
    type: 'radio' as const,
    hideSelectAll: true,
    columnWidth: 34,
    selectedRowKeys:
      selection?.selected === null || selection?.selected === undefined
        ? []
        : [String(selection.selected)],
    onChange: (keys: Key[]) =>
      selection?.onChange(keys.length > 0 ? Number(keys[0]) : null),
  }
}

/**
 * The field list of the table designer.
 *
 * The grid edits the draft in place, the way Navicat's structure tab does. The
 * order of the rows is meaning — it is the column order the DDL is rendered
 * with — so a field is moved by dragging the grip in front of it. Only the grip
 * arms the row for a drag, so selecting text inside the inputs still works.
 */
export function FieldGrid({ design, driver, readOnly, onChange, selection }: DesignGridProps) {
  const patch = (index: number, next: Partial<DesignColumn>) => {
    const columns = design.columns.map((column, i) =>
      i === index ? { ...column, ...next } : column,
    )
    // A field that becomes part of the primary key cannot stay nullable.
    if (next.primaryKey) columns[index] = { ...columns[index], nullable: false }
    // Indexes follow their field through a rename, the way the engine does:
    // showing the old name here would be a lie.
    const from = design.columns[index]?.name.trim() ?? ''
    const to = columns[index].name.trim()
    let indexes = design.indexes
    if (from && from !== to) {
      indexes = design.indexes.map((entry) => ({
        ...entry,
        columns: entry.columns.map((column) =>
          column.trim().toLowerCase() === from.toLowerCase() ? to : column,
        ),
      }))
    }
    onChange({ ...design, columns, indexes })
  }

  const suggestions = typeSuggestions(driver).map((value) => ({ value }))

  // Native HTML5 drag. `armed` is only true between pressing the grip and the
  // end of the gesture, so the rest of the time the row is not draggable and
  // the inputs behave like inputs.
  const dragFrom = useRef<number | null>(null)
  const [armed, setArmed] = useState(false)
  const [dropAt, setDropAt] = useState<number | null>(null)

  const move = (from: number, to: number) => {
    if (from === to) return
    const columns = design.columns.slice()
    const [moved] = columns.splice(from, 1)
    if (!moved) return
    columns.splice(to, 0, moved)
    onChange({ ...design, columns })
    // Keep the selection on the field that moved.
    if (selection?.selected === from) selection.onChange(to)
  }

  const columns: ResizableColumns<DesignColumn> = [
    {
      title: '',
      key: 'drag',
      width: 30,
      align: 'center',
      resizable: false,
      render: () => (
        <Tooltip title={t('designGrid.drag-to-reorder')}>
          <span
            className="dm-drag-handle"
            onMouseDown={() => setArmed(true)}
            onMouseUp={() => setArmed(false)}
          >
            <HolderOutlined />
          </span>
        </Tooltip>
      ),
    },
    {
      title: '#',
      width: 44,
      align: 'right',
      resizable: false,
      render: (_value, _row, index) => (
        <Typography.Text type="secondary" style={{ fontSize: 11 }}>
          {index + 1}
        </Typography.Text>
      ),
    },
    {
      title: t('designGrid.name'),
      width: 220,
      render: (_value, row, index) => (
        <Input
          size="small"
          className="mono"
          value={row.name}
          disabled={readOnly}
          status={row.name.trim() ? undefined : 'error'}
          onChange={(event) => patch(index, { name: event.target.value })}
        />
      ),
    },
    {
      title: t('designGrid.type'),
      width: 220,
      render: (_value, row, index) => (
        <AutoComplete
          size="small"
          className="mono"
          style={{ width: '100%' }}
          value={row.dataType}
          options={suggestions}
          disabled={readOnly}
          status={row.dataType.trim() ? undefined : 'error'}
          placeholder={t('designGrid.varchar-255')}
          onChange={(value: string) => patch(index, { dataType: value })}
        />
      ),
    },
    {
      title: () => (
        <Tooltip title={t('designGrid.allow-null-values')}>
          <span>{t('designGrid.null')}</span>
        </Tooltip>
      ),
      width: 60,
      align: 'center',
      render: (_value, row, index) => (
        <Checkbox
          checked={row.nullable}
          disabled={readOnly || row.primaryKey}
          onChange={(event) => patch(index, { nullable: event.target.checked })}
        />
      ),
    },
    {
      title: t('designGrid.default'),
      width: 180,
      render: (_value, row, index) => (
        <Input
          size="small"
          className="mono"
          value={row.defaultValue ?? ''}
          disabled={readOnly}
          placeholder={t('designGrid.none')}
          onChange={(event) =>
            patch(index, { defaultValue: event.target.value === '' ? null : event.target.value })
          }
        />
      ),
    },
    {
      title: () => (
        <Tooltip title={t('designGrid.primary-key')}>
          <KeyOutlined />
        </Tooltip>
      ),
      width: 44,
      align: 'center',
      render: (_value, row, index) => (
        <Tooltip title={row.primaryKey ? t('designGrid.part-of-the-primary-key') : t('designGrid.make-this-field-part-of-the-primary-key')}>
          <Checkbox
            checked={row.primaryKey}
            disabled={readOnly}
            onChange={(event) => patch(index, { primaryKey: event.target.checked })}
          />
        </Tooltip>
      ),
    },
    {
      title: () => (
        <Tooltip title={t('designGrid.auto-increment')}>
          <span>{t('designGrid.auto')}</span>
        </Tooltip>
      ),
      width: 62,
      align: 'center',
      render: (_value, row, index) => (
        <Checkbox
          checked={row.autoIncrement}
          disabled={readOnly}
          onChange={(event) => patch(index, { autoIncrement: event.target.checked })}
        />
      ),
    },
    {
      title: t('designGrid.comment'),
      render: (_value, row, index) => (
        <Input
          size="small"
          value={row.comment ?? ''}
          disabled={readOnly}
          onChange={(event) => patch(index, { comment: event.target.value })}
        />
      ),
    },
  ]

  const grid = useColumnResize(columns)
  return (
    <Table
      className={grid.resized ? 'dm-design-grid dm-grid-resized' : 'dm-design-grid'}
      {...grid.tableProps}
      size="small"
      bordered
      rowKey={(_row, index) => String(index)}
      columns={grid.columns}
      dataSource={design.columns}
      pagination={false}
      rowSelection={selection ? rowSelection(selection) : undefined}
      onRow={(_row, index) => {
        const rowIndex = index ?? 0
        return {
          draggable: armed && !readOnly && design.columns.length > 1,
          onDragStart: (event: DragEvent<HTMLElement>) => {
            dragFrom.current = rowIndex
            event.dataTransfer.effectAllowed = 'move'
            // Firefox refuses to start a drag without a payload.
            event.dataTransfer.setData('text/plain', String(rowIndex))
          },
          onDragOver: (event: DragEvent<HTMLElement>) => {
            if (dragFrom.current === null) return
            event.preventDefault()
            event.dataTransfer.dropEffect = 'move'
            setDropAt(rowIndex)
          },
          onDrop: (event: DragEvent<HTMLElement>) => {
            event.preventDefault()
            const from = dragFrom.current
            dragFrom.current = null
            setArmed(false)
            setDropAt(null)
            if (from !== null) move(from, rowIndex)
          },
          onDragEnd: () => {
            dragFrom.current = null
            setArmed(false)
            setDropAt(null)
          },
          className: dropAt === rowIndex ? 'dm-drag-over' : undefined,
        }
      }}
      scroll={{ x: 'max-content', y: '100%' }}
    />
  )
}
