import { useCallback, useMemo, useRef, useState, type ReactNode } from 'react'
import { App as AntApp, Button, Input, Space, Tag, Typography } from 'antd'
import { CheckOutlined, UndoOutlined } from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type { CellValue, ColumnMeta, KeyValue, RowUpdate } from '../api/types'
import { looksLikeParagraph } from './DataGrid'

export interface RowDetailProps {
  /** The row whose detail is on screen, as it came back from the engine. */
  row: CellValue[]
  columns: ColumnMeta[]
  /** The row's identity, resolved by the caller that knows the primary key. */
  keyValues: KeyValue[]
  /** Where the row lives, so a statement can be rendered and then run. */
  sessionId: string
  database: string
  schema: string
  object: string
  /** True for a connection that may not write; the fields are then shown, not offered. */
  readOnly: boolean
  /** What this row is called in the header — "Row 12". */
  title: string
  /** The caller's own buttons (copy as JSON, copy as INSERT). */
  actions?: ReactNode
  /** Called after a write lands, so the page can reload the row it is showing. */
  onChanged: () => void
}

/** The text an editor starts from: a value, or the empty string for SQL NULL. */
function textOf(value: CellValue): string {
  return value === null || value === undefined ? '' : String(value)
}

/**
 * Whether a field wants room to grow.
 *
 * A value that already runs long, or a column whose type holds a document rather
 * than a label, gets a box that grows with it. The decision is made from the
 * value the row arrived with, not from what is being typed, so the editor does
 * not swap itself out from under the caret.
 */
function wantsRoom(column: ColumnMeta, text: string): boolean {
  if (text.includes('\n') || text.length > 80) return true
  return looksLikeParagraph(column.databaseType)
}

/**
 * The detail of one row, and the place its columns can be edited.
 *
 * An edit is collected field by field and then sent as one statement: the row is
 * identified once, by the key the caller passes, and every changed column goes
 * into the same WHERE — so a table whose primary key spans several columns is
 * still safe to edit here. Nothing is bound in this file; the values go to the
 * engine as parameters, exactly as an inline cell edit does.
 *
 * An emptied field means NULL — the same rule the grid's cells follow, and the
 * only way to type "no value" when the string "NULL" is a value someone may mean.
 */
export function RowDetail({
  row,
  columns,
  keyValues,
  sessionId,
  database,
  schema,
  object,
  readOnly,
  title,
  actions,
  onChanged,
}: RowDetailProps) {
  const { message, modal } = AntApp.useApp()

  // What each field looked like before it was touched, and what it looks like
  // now. A new page of rows (or a reload) hands over a new `row`, which is what
  // resets the drafts.
  const initial = useMemo(() => {
    const drafts: Record<string, string> = {}
    columns.forEach((column, at) => {
      drafts[column.name] = textOf(row[at])
    })
    return drafts
  }, [columns, row])
  const [drafts, setDrafts] = useState(initial)
  // A new page of rows (or a reload) hands over a new `row` object, and the
  // drafts have to become its values. Adjusting them during the render that
  // noticed the change means no frame is drawn with the previous row's text in
  // the fields.
  const lastInitial = useRef(initial)
  if (lastInitial.current !== initial) {
    lastInitial.current = initial
    setDrafts(initial)
  }

  const changed = useMemo(
    () => columns.filter((column) => (drafts[column.name] ?? '') !== initial[column.name]),
    [columns, drafts, initial],
  )

  const [busy, setBusy] = useState(false)

  /** The request as it stands now — read once by the preview, once by the run. */
  const request = useCallback(
    (): RowUpdate => ({
      sessionId,
      database,
      schema,
      object,
      key: keyValues,
      values: changed.map((column) => ({
        column: column.name,
        value: (drafts[column.name] ?? '') === '' ? null : (drafts[column.name] ?? ''),
      })),
    }),
    [changed, database, drafts, keyValues, object, schema, sessionId],
  )

  /**
   * Asks the engine to spell the statement, shows it, and only then runs it.
   *
   * The rendering is the engine's, done by the same code the run uses, so the
   * text in the box is the text that will be sent — this is a preview, not a
   * second opinion. Showing it before the write is the point: several columns at
   * once is more than a person can hold in their head.
   */
  const apply = useCallback(async () => {
    if (changed.length === 0 || busy || readOnly) return
    setBusy(true)
    let statement: string
    try {
      statement = await api.planRowUpdate(request())
    } catch (err) {
      setBusy(false)
      message.error(toMessage(err))
      return
    }
    setBusy(false)
    modal.confirm({
      title:
        changed.length === 1 ? `Change ${changed[0].name}?` : `Change ${changed.length} columns?`,
      width: 660,
      icon: null,
      content: (
        <div className="dm-row-detail-plan">
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            This is the statement the engine will run.
          </Typography.Text>
          <pre className="mono">{statement}</pre>
        </div>
      ),
      okText: 'Apply',
      onOk: async () => {
        try {
          const affected = await api.updateRow(request())
          if (affected === 0) {
            message.warning('No row matched: it may have been changed or deleted by someone else')
          } else {
            message.success(`Updated ${changed.length} column(s)`)
          }
          onChanged()
        } catch (err) {
          message.error(toMessage(err))
          throw err
        }
      },
    })
  }, [busy, changed, message, modal, onChanged, readOnly, request])

  return (
    <>
      <div className="dm-row-detail-head">
        <span className="dm-row-detail-title">{title}</span>
        {actions ? <div className="dm-row-detail-actions">{actions}</div> : null}
      </div>
      <div className="dm-row-detail-scroll">
        <div className="dm-row-detail">
          {columns.map((column, at) => {
            const value = drafts[column.name] ?? ''
            const dirty = value !== initial[column.name]
            const editable = !readOnly && column.editable !== false
            return (
              <div
                key={column.name}
                className={dirty ? 'dm-row-detail-item is-dirty' : 'dm-row-detail-item'}
              >
                <div className="dm-row-detail-label">
                  <span>{column.name}</span>
                  {column.isPrimaryKey ? (
                    <Tag color="gold" style={{ marginInlineStart: 6 }}>
                      PK
                    </Tag>
                  ) : null}
                  {dirty ? <span className="dm-row-detail-dirty">changed</span> : null}
                </div>
                {editable ? (
                  wantsRoom(column, initial[column.name] ?? '') ? (
                    <Input.TextArea
                      autoSize={{ minRows: 2, maxRows: 12 }}
                      value={value}
                      onChange={(event) =>
                        setDrafts((current) => ({ ...current, [column.name]: event.target.value }))
                      }
                    />
                  ) : (
                    <Input
                      value={value}
                      onChange={(event) =>
                        setDrafts((current) => ({ ...current, [column.name]: event.target.value }))
                      }
                    />
                  )
                ) : (
                  <div className="dm-row-detail-value mono">
                    {row[at] === null || row[at] === undefined ? (
                      <span className="dm-null">NULL</span>
                    ) : (
                      String(row[at])
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      </div>
      {!readOnly ? (
        <div className="dm-row-detail-footer">
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {changed.length === 0 ? 'No changes' : `${changed.length} column(s) changed`}
          </Typography.Text>
          <Space size={4}>
            <Button
              size="small"
              icon={<UndoOutlined />}
              disabled={changed.length === 0 || busy}
              onClick={() => setDrafts(initial)}
            >
              Reset
            </Button>
            <Button
              size="small"
              type="primary"
              icon={<CheckOutlined />}
              loading={busy}
              disabled={changed.length === 0}
              onClick={() => void apply()}
            >
              Apply
            </Button>
          </Space>
        </div>
      ) : null}
    </>
  )
}
