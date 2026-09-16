import { AutoComplete, Checkbox, Input, Select, Table, Tag, Tooltip, Typography } from 'antd'
import type { TableColumnsType } from 'antd'
import type { Key } from 'react'
import { KeyOutlined } from '@ant-design/icons'

import type { DesignColumn, DesignIndex, DriverType, TableDesign } from '../api/types'
import { typeSuggestions } from '../lib/design'

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

/** Turns a draft patch into a new draft (the draft is treated as immutable). */
function withIndexes(design: TableDesign, indexes: DesignIndex[]): TableDesign {
  return { ...design, indexes }
}

/**
 * The field list of the table designer.
 *
 * The grid edits the draft in place, the way Navicat's structure tab does; the
 * SQL preview next to it is what the backend would run for the draft as it
 * stands right now.
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

  const columns: TableColumnsType<DesignColumn> = [
    {
      title: '#',
      width: 44,
      align: 'right',
      render: (_value, _row, index) => (
        <Typography.Text type="secondary" style={{ fontSize: 11 }}>
          {index + 1}
        </Typography.Text>
      ),
    },
    {
      title: 'Name',
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
      title: 'Type',
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
          placeholder="varchar(255)"
          onChange={(value: string) => patch(index, { dataType: value })}
        />
      ),
    },
    {
      title: () => (
        <Tooltip title="Allow NULL values">
          <span>Null</span>
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
      title: 'Default',
      width: 180,
      render: (_value, row, index) => (
        <Input
          size="small"
          className="mono"
          value={row.defaultValue ?? ''}
          disabled={readOnly}
          placeholder="none"
          onChange={(event) =>
            patch(index, { defaultValue: event.target.value === '' ? null : event.target.value })
          }
        />
      ),
    },
    {
      title: () => (
        <Tooltip title="Primary key">
          <KeyOutlined />
        </Tooltip>
      ),
      width: 44,
      align: 'center',
      render: (_value, row, index) => (
        <Tooltip title={row.primaryKey ? 'Part of the primary key' : 'Make this field part of the primary key'}>
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
        <Tooltip title="Auto increment">
          <span>Auto</span>
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
      title: 'Comment',
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

  return (
    <Table
      className="dm-design-grid"
      size="small"
      bordered
      rowKey={(_row, index) => String(index)}
      columns={columns}
      dataSource={design.columns}
      pagination={false}
      rowSelection={selection ? rowSelection(selection) : undefined}
      scroll={{ x: 'max-content', y: '100%' }}
    />
  )
}

interface IndexGridProps extends DesignGridProps {
  /** The primary key of the table, shown read-only (edit it in the field list). */
  primary: DesignIndex | null
}

/** The index list of the table designer. */
export function IndexGrid({ design, readOnly, onChange, primary, selection }: IndexGridProps) {
  const patch = (index: number, next: Partial<DesignIndex>) => {
    const indexes = design.indexes.map((entry, i) => (i === index ? { ...entry, ...next } : entry))
    onChange(withIndexes(design, indexes))
  }

  const fields = design.columns
    .map((column) => column.name.trim())
    .filter(Boolean)
    .map((name) => ({ value: name, label: name }))

  const columns: TableColumnsType<DesignIndex> = [
    {
      title: 'Name',
      width: 240,
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
      title: 'Fields',
      render: (_value, row, index) => (
        <Select
          size="small"
          mode="multiple"
          className="mono"
          style={{ width: '100%' }}
          value={row.columns}
          options={fields}
          disabled={readOnly}
          placeholder="pick one or more fields"
          onChange={(value: string[]) => patch(index, { columns: value })}
        />
      ),
    },
    {
      title: 'Unique',
      width: 74,
      align: 'center',
      render: (_value, row, index) => (
        <Checkbox
          checked={row.unique}
          disabled={readOnly}
          onChange={(event) => patch(index, { unique: event.target.checked })}
        />
      ),
    },
    {
      title: 'Kind',
      width: 110,
      render: (_value, row) => (
        <Typography.Text type="secondary" style={{ fontSize: 11 }}>
          {row.originalName ? 'existing' : 'new'}
        </Typography.Text>
      ),
    },
  ]

  return (
    <Table
      className="dm-design-grid"
      size="small"
      bordered
      rowKey={(_row, index) => String(index)}
      columns={columns}
      dataSource={design.indexes}
      pagination={false}
      rowSelection={selection ? rowSelection(selection) : undefined}
      scroll={{ x: 'max-content', y: '100%' }}
      locale={{ emptyText: 'No secondary indexes' }}
      summary={() =>
        primary ? (
          <Table.Summary.Row className="dm-design-pk">
            <Table.Summary.Cell index={0}>
              <Typography.Text className="mono">{primary.name}</Typography.Text>
            </Table.Summary.Cell>
            <Table.Summary.Cell index={1}>
              <Typography.Text className="mono">{primary.columns.join(', ')}</Typography.Text>
            </Table.Summary.Cell>
            <Table.Summary.Cell index={2} align="center">
              <Tag color="gold">PK</Tag>
            </Table.Summary.Cell>
            <Table.Summary.Cell index={3}>
              <Typography.Text type="secondary" style={{ fontSize: 11 }}>
                edited above
              </Typography.Text>
            </Table.Summary.Cell>
          </Table.Summary.Row>
        ) : null
      }
    />
  )
}
