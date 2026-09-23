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

/**
 * The column that takes whatever the other columns leave over.
 *
 * A grid laid out `fixed` with exact widths no longer fills its pane by itself:
 * the table is as wide as the sum of its columns, so narrowing one leaves a strip
 * of background on the right — the table would look boxed into a corner instead of
 * lying in the window. So one extra, empty column is appended, with an `auto`
 * width: a fixed layout gives the space left over to the columns that have no
 * width of their own (and to nobody else), and the table's `min-width: 100%`
 * (antd's own inline style) then makes it fill the pane exactly.
 *
 * Nothing is measured for this: the leftover goes to the empty column, never to a
 * real one, which is also what keeps a drag exact — pulling one column's edge by
 * ten pixels widens that column by ten and shrinks this one by ten.
 */
const FILLER_KEY = '__dm-filler__'

/**
 * What a drag writes into while it is going on.
 *
 * The width of the column being dragged could be held in React state, and that is
 * what this started out as — but a state update rebuilds the column set, which
 * re-renders every cell of every row, once per pointer move. On a 200-row result
 * that measured ~80ms a move: the edge arrived well behind the cursor, which is
 * what "laggy" means. So while the pointer is down the width is written straight
 * to the `<col>` elements of the tables on screen, and the gesture is committed to
 * state once, when it ends. The browser lays the table out once per frame however
 * many moves arrive in it, so those writes cost a style property each.
 */
interface LiveDrag {
  /** The tables this header's columns are drawn in — see `tablesOf`. */
  roots: HTMLTableElement[]
  /** Where the dragged column sits in a row, so its `<col>` can be picked. */
  index: number
  /** The last width asked for; what gets committed when the gesture ends. */
  width: number
}

/**
 * The tables whose `<col>`s stand for this header's columns.
 *
 * One table answers for an ordinary grid. A grid whose header row is fixed draws
 * two — the header is a table of its own above the body — and both colgroups have
 * to move together, or the header would disagree with the rows below it.
 */
function tablesOf(cell: HTMLTableCellElement): HTMLTableElement[] {
  const root = cell.closest('.ant-table')
  if (!root) return []
  return Array.from(root.querySelectorAll('table'))
}

/** Keeps a dragged width inside the range an edge may be pulled to. */
function clampWidth(width: number): number {
  return Math.round(Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, width)))
}

/**
 * The `<col>` elements of one table, in the order the row's cells are in.
 *
 * The index of the dragged column comes from the header cell itself (`cellIndex`),
 * so a table that has an extra cell in front of it — the checkbox column antd
 * injects, which this hook never sees — needs no special case.
 */
function colsOf(table: HTMLTableElement): HTMLTableColElement[] | null {
  const cols = Array.from(table.querySelectorAll('colgroup > col')) as HTMLTableColElement[]
  // A colgroup that does not match the header row means the index names some other
  // column: rc-table leaves a `<col>` out for a column that has no width at all.
  if (cols.length !== table.querySelectorAll('thead th').length) return null
  return cols
}

/** What a header cell needs in order to run the drag gesture for its column. */
interface ResizeHandle {
  /** The column this header belongs to, as the hook remembers widths by. */
  column: string
  /** Tells the hook which `<th>` is on screen for a column, so it can measure it. */
  register: (column: string, cell: HTMLTableCellElement | null) => void
  /** Starts a gesture: freezes the layout if needed, and answers where it starts. */
  begin: (cell: HTMLTableCellElement) => number
  apply: (column: string, width: number) => void
  /** Ends the gesture: what is on screen becomes what React remembers. */
  end: (column: string) => void
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
  const end = dmResize?.end

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
    drag.current = { fromX: event.clientX, fromWidth: begin(th) }
    document.body.classList.add('dm-col-resizing')
  }

  const onPointerMove = (event: ReactPointerEvent<HTMLSpanElement>) => {
    const started = drag.current
    if (!started || !apply || !column) return
    // Measured from where the gesture began, not from the last move: a drag that
    // is clamped at its limits must not lose the distance it was clamped by.
    apply(column, started.fromWidth + event.clientX - started.fromX)
  }

  const finish = () => {
    if (!drag.current) return
    drag.current = null
    document.body.classList.remove('dm-col-resizing')
    if (column && end) end(column)
  }

  return (
    <th {...rest} ref={cell}>
      {children}
      {begin && apply && end && column ? (
        <span
          className="dm-col-resizer"
          role="separator"
          aria-orientation="vertical"
          aria-label="Resize column"
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={finish}
          onLostPointerCapture={finish}
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
   * the table can borrow the frozen layout — see `.dm-grid-resized` in
   * `global.css`.
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
  /** The gesture in progress, if any — see `LiveDrag`. */
  const drag = useRef<LiveDrag | null>(null)

  const register = useCallback((column: string, cell: HTMLTableCellElement | null) => {
    if (cell) cells.current.set(column, cell)
    else cells.current.delete(column)
  }, [])

  /**
   * Writes one width to the tables' `<col>` elements, and answers whether it
   * reached any. A table whose colgroup does not line up is left alone; the caller
   * then falls back to state, which is slower but cannot write to the wrong column.
   */
  const paint = useCallback((targets: LiveDrag, width: number) => {
    let painted = false
    for (const table of targets.roots) {
      const columns = colsOf(table)
      const col = columns?.[targets.index]
      if (!col) continue
      col.style.width = `${width}px`
      painted = true
    }
    return painted
  }, [])

  const apply = useCallback(
    (column: string, width: number) => {
      const next = clampWidth(width)
      const targets = drag.current
      if (!targets) {
        // No gesture to write into — a caller driving this by hand — so state is
        // the only place the width can go.
        setWidths((previous) => (previous[column] === next ? previous : { ...previous, [column]: next }))
        return
      }
      targets.width = next
      if (!paint(targets, next)) {
        // The colgroup does not line up after all, so state is the fallback.
        setWidths((previous) => (previous[column] === next ? previous : { ...previous, [column]: next }))
      }
    },
    [paint],
  )

  const begin = useCallback(
    (cell: HTMLTableCellElement) => {
      const startWidth = cell.getBoundingClientRect().width
      if (!frozen.current) {
        frozen.current = true
        // Freeze what is on screen: every column of this table, as the browser
        // laid it out a moment ago — which is also the only honest width for a
        // column that was never given one.
        const snapshot: Record<string, number> = {}
        for (const [key, element] of cells.current) {
          const width = Math.round(element.getBoundingClientRect().width)
          if (width > 0) snapshot[key] = width
        }
        setWidths(snapshot)
      }
      drag.current = {
        roots: tablesOf(cell),
        // The header cell's own position is the column's position in a row — the
        // checkbox column antd injects is counted by the browser, not by us.
        index: cell.cellIndex,
        width: clampWidth(startWidth),
      }
      return startWidth
    },
    [],
  )

  const end = useCallback(
    (column: string) => {
      const targets = drag.current
      if (!targets) return
      drag.current = null
      setWidths((previous) =>
        previous[column] === targets.width ? previous : { ...previous, [column]: targets.width },
      )
    },
    [],
  )

  const resized = Object.keys(widths).length > 0
  const keys = useMemo(() => keysOf(columns), [columns])

  const merged = useMemo(() => {
    const mapped = columns.map((column, index) => {
      const key = keys[index]
      const handle: ResizeHandle = { column: key, register, begin, apply, end }
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
    })
    if (!resized) return mapped
    const filler: TableColumnType<T> = {
      key: FILLER_KEY,
      // `auto`, rather than no width at all: an explicit auto column is what the
      // remaining space goes to, and it keeps the colgroup's column count equal to
      // the number of cells in a row.
      width: 'auto',
      title: '',
      render: () => null,
    }
    return [...mapped, filler]
  }, [apply, begin, columns, end, keys, register, resized, widths])

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
