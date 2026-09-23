import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type {
  ComponentType,
  Key,
  PointerEvent as ReactPointerEvent,
  ThHTMLAttributes,
} from 'react'
import type { TableColumnType, TableColumnsType, TableProps } from 'antd'

/**
 * Draggable column widths for the grids.
 *
 * Every grid here is an antd `Table`, and antd leaves a column's width to the
 * browser: `column.width` is a hint that a cell with a long value is free to
 * ignore, so a column can be widened but never narrowed past its longest cell —
 * the browser will not shrink a column below the content it has to hold. That is
 * backwards for a data grid, where dragging a column edge is how you decide how
 * much of a value you want on screen.
 *
 * So the browser stops being asked: the first time any header is dragged, every
 * column's *rendered* width is measured and frozen, and from then on the table is
 * laid out `fixed` with those widths — `table-layout: fixed` honours a width
 * exactly, whatever the cells contain, which is also what makes clipping (and
 * therefore narrowing) possible at all.
 *
 * All of them are frozen rather than only the one being dragged. A table that is
 * still stretching its columns across the pane would move the column under the
 * cursor by some other amount than the mouse did, and the drag would feel like it
 * was slipping; freezing what is on screen first, then moving one column by the
 * delta, keeps the gesture exact.
 *
 * What was frozen is state of the pane, so the widths belong to the window: they
 * survive switching tabs (panes stay mounted once visited) and are forgotten when
 * the window is closed, which is also when the columns themselves are likely to
 * have changed.
 */

/** How narrow a column may be dragged, and how wide. */
const MIN_WIDTH = 48
const MAX_WIDTH = 1200

/**
 * The width given to a column that has none of its own and was not on screen when
 * the edges were frozen. Reached only by a table that grows new columns after the
 * first drag — none of the grids here do — but a fixed-layout table must not be
 * left with a widthless column, because the browser would give it nothing.
 */
const FALLBACK_WIDTH = 160

/** What a header cell needs in order to run the drag gesture for its column. */
interface ResizeHandle {
  /** The column this header belongs to, as the hook remembers widths by. */
  column: string
  /** Tells the hook which `<th>` is on screen for a column, so it can measure it. */
  register: (column: string, cell: HTMLTableCellElement | null) => void
  /** Starts a gesture: freezes the layout if needed, and answers where it starts. */
  begin: (column: string, cell: HTMLTableCellElement) => number
  apply: (column: string, width: number) => void
}

/**
 * A column that can opt out of the grip.
 *
 * A drag handle on the 30px grip column of the designer, or on a checkbox column,
 * is noise: there is nothing to read in them and nothing to widen.
 */
export type ResizableColumns<T> = (TableColumnsType<T>[number] & { resizable?: boolean })[]

/** The name a column goes by, which is what its width would be remembered by. */
function nameOf<T>(column: ResizableColumns<T>[number], index: number): string {
  const { key, dataIndex } = column as { key?: Key; dataIndex?: Key }
  if (key !== undefined && key !== null) return String(key)
  if (typeof dataIndex === 'string' || typeof dataIndex === 'number') return String(dataIndex)
  return `col-${index}`
}

/**
 * The columns' identities, one per column.
 *
 * Two columns of one result can share a name (`select a.id, b.id`), and a width has
 * to be remembered by something, so the later one is told apart by where it sits.
 */
function keysOf<T>(columns: ResizableColumns<T>): string[] {
  const seen = new Set<string>()
  return columns.map((column, index) => {
    const name = nameOf(column, index)
    const key = seen.has(name) ? `${name} (${index})` : name
    seen.add(key)
    return key
  })
}

type HeaderCellProps = ThHTMLAttributes<HTMLTableCellElement> & { dmResize?: ResizeHandle }

/**
 * A header cell that carries its column's drag handle.
 *
 * antd hands every `<th>` to this component instead of rendering one itself (see
 * the `components` the hook returns), which is the only place the handle can live:
 * it has to sit on the column's edge, inside the cell that draws that edge, and
 * the extra props the hook passes down (`dmResize`) must not reach the DOM.
 */
function ResizableHeaderCell({ dmResize, children, ...rest }: HeaderCellProps) {
  const cell = useRef<HTMLTableCellElement | null>(null)
  const drag = useRef<{ fromX: number; fromWidth: number } | null>(null)
  const register = dmResize?.register
  const column = dmResize?.column

  // A `<th>` is the only place a column's rendered width can be read. Columns that
  // are not on screen are not measured — the hook only ever freezes what it can see.
  useEffect(() => {
    if (!register || column === undefined) return undefined
    register(column, cell.current)
    return () => register(column, null)
  }, [column, register])

  const begin = dmResize?.begin
  const apply = dmResize?.apply

  const onPointerDown = (event: ReactPointerEvent<HTMLSpanElement>) => {
    const th = cell.current
    if (!th || !begin || !apply || !column || event.button !== 0) return
    // The gesture must not reach the sorter: antd sorts on a click, and a drag
    // that ends where it started is still a click.
    event.preventDefault()
    event.stopPropagation()
    // Pointer capture is what keeps the drag alive once the cursor leaves the
    // header — over the rows, over another column, out of the window — without a
    // listener on `window` to take down again afterwards.
    event.currentTarget.setPointerCapture(event.pointerId)
    drag.current = { fromX: event.clientX, fromWidth: begin(column, th) }
    document.body.classList.add('dm-col-resizing')
  }

  const onPointerMove = (event: ReactPointerEvent<HTMLSpanElement>) => {
    const started = drag.current
    if (!started || !apply || !column) return
    // Measured from where the gesture began, not from the last move: a drag that
    // is clamped at its limits must not lose the distance it was clamped by.
    apply(column, started.fromWidth + event.clientX - started.fromX)
  }

  const onLostPointerCapture = () => {
    drag.current = null
    document.body.classList.remove('dm-col-resizing')
  }

  return (
    <th {...rest} ref={cell}>
      {children}
      {begin && apply && column ? (
        <span
          className="dm-col-resizer"
          role="separator"
          aria-orientation="vertical"
          aria-label="Resize column"
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onLostPointerCapture}
          onLostPointerCapture={onLostPointerCapture}
          onClick={(event) => event.stopPropagation()}
        />
      ) : null}
    </th>
  )
}

export interface ColumnResize<T> {
  /** The columns, with the widths that were dragged applied. */
  columns: TableColumnsType<T>
  /** The antd props that go with them: the layout, and the header cell above. */
  tableProps: ColumnResizeTableProps
  /**
   * True once an edge has been dragged. The caller is expected to add a class so
   * the table can stop stretching — see `.dm-grid-resized` in `global.css`.
   */
  resized: boolean
}

/**
 * The antd props a resizable grid needs.
 *
 * Deliberately not generic in the row type, so that one window can hold two
 * column sets at once (the object list shows objects or indexes) and still hand
 * the table one set of props: `TableComponents<RecordType>` varies with it.
 */
export interface ColumnResizeTableProps {
  tableLayout?: TableProps['tableLayout']
  components: { header: { cell: ComponentType<HeaderCellProps> } }
}

/**
 * Makes the headers of a grid draggable, and remembers what came of it.
 *
 * The columns must be the same array on most renders (the panes memoize them): a
 * new array is fine, but a rebuilt one on every render rebuilds these columns too.
 */
export function useColumnResize<T>(columns: ResizableColumns<T>): ColumnResize<T> {
  const [widths, setWidths] = useState<Record<string, number>>({})
  const cells = useRef(new Map<string, HTMLTableCellElement>())
  const frozen = useRef(false)

  const register = useCallback((column: string, cell: HTMLTableCellElement | null) => {
    if (cell) cells.current.set(column, cell)
    else cells.current.delete(column)
  }, [])

  const apply = useCallback((column: string, width: number) => {
    const next = Math.round(Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, width)))
    setWidths((previous) => (previous[column] === next ? previous : { ...previous, [column]: next }))
  }, [])

  const begin = useCallback((column: string, cell: HTMLTableCellElement) => {
    const startWidth = cell.getBoundingClientRect().width
    if (frozen.current) return startWidth
    frozen.current = true
    // Freeze what is on screen: every column of this table, as the browser laid it
    // out a moment ago — which is also the only honest width for a column that was
    // never given one.
    const snapshot: Record<string, number> = {}
    for (const [key, element] of cells.current) {
      const width = Math.round(element.getBoundingClientRect().width)
      if (width > 0) snapshot[key] = width
    }
    setWidths(snapshot)
    return snapshot[column] ?? startWidth
  }, [])

  const resized = Object.keys(widths).length > 0
  const keys = useMemo(() => keysOf(columns), [columns])

  const merged = useMemo(
    () =>
      columns.map((column, index) => {
        const key = keys[index]
        const handle: ResizeHandle = { column: key, register, begin, apply }
        // A leaf column, so that `onHeaderCell` is the one the cell gets. The
        // result is a leaf column set either way: a group's own header has no
        // data column to resize.
        const next = { ...column } as TableColumnType<T>
        next.width = widths[key] ?? column.width ?? (resized ? FALLBACK_WIDTH : undefined)
        if (column.resizable !== false) {
          const own = next.onHeaderCell
          next.onHeaderCell = (col, at) => ({ ...(own?.(col, at) ?? {}), dmResize: handle })
        }
        return next
      }),
    [apply, begin, columns, keys, register, resized, widths],
  )

  const tableProps = useMemo<ColumnResizeTableProps>(
    () => ({
      // `fixed` is what turns a width into a decision instead of a suggestion.
      // Until an edge is dragged the table keeps antd's own automatic layout.
      tableLayout: resized ? 'fixed' : undefined,
      components: { header: { cell: ResizableHeaderCell } },
    }),
    [resized],
  )

  return { columns: merged, tableProps, resized }
}
