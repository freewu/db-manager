import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
  type WheelEvent as ReactWheelEvent,
} from 'react'
import {
  Alert,
  App as AntApp,
  Button,
  Empty,
  Input,
  Segmented,
  Space,
  Spin,
  Switch,
  Tooltip,
  Typography,
} from 'antd'
import {
  DownloadOutlined,
  ExpandOutlined,
  EyeOutlined,
  ReloadOutlined,
  SearchOutlined,
  ZoomInOutlined,
  ZoomOutOutlined,
} from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type { GraphEdge, GraphNode, SchemaGraph } from '../api/types'
import { useAppStore, type ThemeMode, type WorkspaceTab } from '../store/appStore'

/* Node geometry, in diagram units (1 unit = 1 px at 100% zoom). */
const BOX_W = 268
const HEADER_H = 30
const ROW_H = 18
const BOX_PAD = 6
const GAP_X = 150
const GAP_Y = 30
/** Long tables are cut off; the footer says how many columns are hidden. */
const MAX_ROWS = 14

/** Kinds worth drawing: a sequence or procedure has no relationships. */
const DIAGRAM_KINDS = new Set(['table', 'view', 'materialized_view', 'collection'])

interface Palette {
  bg: string
  grid: string
  box: string
  border: string
  accent: string
  pk: string
  text: string
  dim: string
  edge: string
}

/** Inline colours (not CSS classes) so an exported SVG stands on its own. */
function paletteOf(theme: ThemeMode): Palette {
  return theme === 'dark'
    ? {
        bg: '#1b1b1f',
        grid: '#2a2a31',
        box: '#25252b',
        border: '#3b3b44',
        accent: '#36ab60',
        pk: '#d8a72a',
        text: '#e8e8ea',
        dim: '#9a9aa3',
        edge: '#7d7d8a',
      }
    : {
        bg: '#ffffff',
        grid: '#f1f1f3',
        box: '#ffffff',
        border: '#d8d8de',
        accent: '#36ab60',
        pk: '#b8860b',
        text: '#1f1f22',
        dim: '#8c8c94',
        edge: '#c2c2c9',
      }
}

interface Placed {
  name: string
  node: GraphNode
  /** Synthetic box for a foreign key that points outside the namespace. */
  external: boolean
  /** Foreign-key depth: what orders the boxes inside one row. */
  layer: number
  x: number
  y: number
  w: number
  h: number
  rows: number
  hidden: number
}

interface Layout {
  placed: Placed[]
  width: number
  height: number
  byName: Map<string, Placed>
}

/** How many column rows a box shows (0 in compact mode). */
function rowCount(node: GraphNode, showColumns: boolean): { rows: number; hidden: number } {
  if (!showColumns) return { rows: 0, hidden: 0 }
  const rows = Math.min(node.columns.length, MAX_ROWS)
  return { rows, hidden: Math.max(0, node.columns.length - rows) }
}

function boxHeight(rows: number, hidden: number): number {
  const body = rows * ROW_H + (hidden > 0 ? ROW_H : 0)
  return HEADER_H + (body > 0 ? body + BOX_PAD : 0)
}

/**
 * A name cut into lowercase words: `t_user_favorite` → `['t', 'user', 'favorite']`.
 *
 * Names are compared word by word rather than letter by letter, so `t_orders`
 * is not treated as a relative of `t_order_payment` just because the letters
 * happen to line up. Words are alphanumeric, so `' '` joins them back without
 * ever colliding.
 */
function nameWords(name: string): string[] {
  return name
    .toLowerCase()
    .split(/[^a-z0-9]+/)
    .filter((word) => word !== '')
}

/**
 * The row a table is drawn in: the name family it belongs to.
 *
 * Tables are remembered by name — `t_user`, `t_user_favorite` and
 * `t_user_profile` are read together, not scattered through the diagram — so a
 * table's row is its shortest leading word sequence that is also the *whole*
 * name of some table. That is exactly "the table this one is a variation of":
 *
 *   - `t_user_favorite` → `t_user`, because a table is called that;
 *   - `t_user` → `t_user`, it is its own root;
 *   - `t_product` → `t_product`, a row of its own: in a schema of `t_…` tables
 *     the bare `t` is a prefix of all of them and the whole name of none, so it
 *     never becomes a row key.
 *
 * That last case is what keeps this from putting every table sharing a common
 * leading `t_` on one endless line; only names somebody is a variation of are
 * grouped, and an unrelated table keeps its row to itself.
 */
function rowKeyOf(name: string, wholeNames: Set<string>): string {
  const words = nameWords(name)
  for (let end = 1; end <= words.length; end++) {
    const key = words.slice(0, end).join(' ')
    if (wholeNames.has(key)) return key
  }
  return words.join(' ')
}

/**
 * Deterministic layout, in two steps.
 *
 * The row is the name family (`rowKeyOf`) and the column is the foreign-key
 * layer: variations of one name are read side by side on one line, and inside
 * a row arrows still point rightwards, from the table being referenced to the
 * one holding the key. Across rows there is no such promise — a foreign key
 * between two families is drawn as it falls.
 *
 * The relaxation of the layers runs at most once per node, which also keeps it
 * finite when the schema has a cycle.
 */
function computeLayout(graph: SchemaGraph, showColumns: boolean): Layout {
  // Sequences and procedures have no relationships: they only show up when
  // something points at them.
  const visible = graph.nodes.filter(
    (node) => DIAGRAM_KINDS.has(node.kind) || isEdgeEnd(graph.edges, node.name),
  )
  const edges = graph.edges

  const layer = new Map<string, number>()
  for (const node of visible) layer.set(node.name, 0)
  for (let pass = 0; pass < visible.length; pass++) {
    let changed = false
    for (const edge of edges) {
      const from = layer.get(edge.from)
      const to = layer.get(edge.to)
      if (from === undefined || to === undefined) continue
      if (to < from + 1) {
        layer.set(edge.to, from + 1)
        changed = true
      }
    }
    if (!changed) break
  }

  // Foreign keys that leave the namespace get a dashed stub box so the arrow
  // still shows where the relationship goes.
  const known = new Set(visible.map((node) => node.name))
  const externalNames: string[] = []
  for (const edge of edges) {
    if (!known.has(edge.to) && !externalNames.includes(edge.to)) externalNames.push(edge.to)
  }
  const externalLayer = Math.max(0, ...[...layer.values()]) + (visible.length > 0 ? 1 : 0)
  for (const name of externalNames) layer.set(name, externalLayer)

  const all = [...visible.map((node) => node.name), ...externalNames]
  const wholeNames = new Set(all.map((name) => nameWords(name).join(' ')))

  const rows = new Map<string, string[]>()
  for (const name of all) {
    const key = rowKeyOf(name, wholeNames)
    const list = rows.get(key) ?? []
    if (list.length === 0) rows.set(key, list)
    list.push(name)
  }

  const nodeByName = new Map(graph.nodes.map((node) => [node.name, node]))
  const placed: Placed[] = []
  let y = 0
  for (const key of [...rows.keys()].sort((a, b) => a.localeCompare(b))) {
    const members = [...(rows.get(key) ?? [])].sort(
      (a, b) => (layer.get(a) ?? 0) - (layer.get(b) ?? 0) || a.localeCompare(b),
    )
    let x = 0
    let rowHeight = 0
    for (const name of members) {
      const node = nodeByName.get(name)
      const external = !node
      const { rows: columnRows, hidden: hiddenColumns } = node
        ? rowCount(node, showColumns)
        : { rows: 0, hidden: 0 }
      const h = boxHeight(columnRows, hiddenColumns)
      placed.push({
        name,
        node:
          node ??
          ({ name, kind: 'table', columns: [] } as GraphNode),
        external,
        layer: layer.get(name) ?? 0,
        x,
        y,
        w: BOX_W,
        h,
        rows: columnRows,
        hidden: hiddenColumns,
      })
      x += BOX_W + GAP_X
      rowHeight = Math.max(rowHeight, h)
    }
    y += rowHeight + GAP_Y
  }

  const width = placed.reduce((max, item) => Math.max(max, item.x + item.w), 0)
  const height = placed.reduce((max, item) => Math.max(max, item.y + item.h), 0)
  return {
    placed,
    width: width + 40,
    height: height + 40,
    byName: new Map(placed.map((item) => [item.name, item])),
  }
}

function isEdgeEnd(edges: GraphEdge[], name: string): boolean {
  return edges.some((edge) => edge.from === name || edge.to === name)
}

/** Vertical offset of a column's row inside its box, for edge anchoring. */
function columnOffset(item: Placed, columns: string[]): number {
  if (item.rows === 0) return item.h / 2
  const wanted = (columns[0] ?? '').toLowerCase()
  const index = item.node.columns.findIndex((column) => column.name.toLowerCase() === wanted)
  if (index < 0 || index >= item.rows) return HEADER_H + item.rows * ROW_H * 0.5
  return HEADER_H + index * ROW_H + ROW_H / 2
}

interface ErDiagramPaneProps {
  tab: WorkspaceTab
}

/**
 * ER diagram of one namespace.
 *
 * The whole graph is fetched in one backend call (`GetSchemaGraph`, which is
 * also where the per-object catalog reads are bounded); this component only
 * draws it, so it stays dependency-free and can export the same picture it
 * shows.
 */
export function ErDiagramPane({ tab }: ErDiagramPaneProps) {
  const session = useAppStore((s) => s.sessionOf(tab.sessionId))
  const theme = useAppStore((s) => s.theme)
  const openTableTab = useAppStore((s) => s.openTableTab)
  const { message } = AntApp.useApp()

  const database = tab.database ?? session?.database ?? ''
  const schema = tab.schema ?? ''
  const palette = useMemo(() => paletteOf(theme), [theme])

  const [graph, setGraph] = useState<SchemaGraph | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [attempt, setAttempt] = useState(0)
  const [showColumns, setShowColumns] = useState(true)
  const [showGrid, setShowGrid] = useState(true)
  const [query, setQuery] = useState('')
  const [hovered, setHovered] = useState<string | null>(null)
  const [view, setView] = useState({ k: 1, x: 0, y: 0 })

  const svgRef = useRef<SVGSVGElement | null>(null)
  const wrapperRef = useRef<HTMLDivElement | null>(null)
  /** The tab whose diagram has already been fitted into the viewport. */
  const fitted = useRef<string | null>(null)
  const drag = useRef<{ x: number; y: number; vx: number; vy: number } | null>(null)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    api
      .getSchemaGraph(tab.sessionId, database, schema)
      .then((result) => {
        if (cancelled) return
        setGraph(result)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setError(toMessage(err))
        setGraph(null)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [attempt, database, schema, tab.sessionId])

  const layout = useMemo(
    () => (graph ? computeLayout(graph, showColumns) : null),
    [graph, showColumns],
  )

  const needle = query.trim().toLowerCase()
  const matched = useMemo(() => {
    if (!layout) return new Set<string>()
    if (!needle) return new Set(layout.placed.map((item) => item.name))
    return new Set(
      layout.placed
        .filter((item) => item.name.toLowerCase().includes(needle))
        .map((item) => item.name),
    )
  }, [layout, needle])

  /** Scales and centres the diagram inside the viewport. */
  const fit = useCallback(() => {
    const wrapper = wrapperRef.current
    if (!wrapper || !layout || layout.width === 0 || layout.height === 0) return
    const w = wrapper.clientWidth
    const h = wrapper.clientHeight
    const k = Math.min(1, Math.min((w - 24) / layout.width, (h - 24) / layout.height))
    setView({
      k,
      x: Math.max(12, (w - layout.width * k) / 2),
      y: Math.max(12, (h - layout.height * k) / 2),
    })
  }, [layout])

  // Fit the diagram the first time it is on screen (and again after a reload
  // of the same window, when the layout has changed underneath the user).
  useEffect(() => {
    if (!layout || layout.placed.length === 0) return
    if (fitted.current === tab.id) return
    fitted.current = tab.id
    fit()
  }, [fit, layout, tab.id])

  const zoomBy = useCallback((factor: number) => {
    setView((current) => ({ ...current, k: Math.min(4, Math.max(0.15, current.k * factor)) }))
  }, [])

  const onWheel = useCallback((event: ReactWheelEvent<SVGSVGElement>) => {
    event.preventDefault()
    const wrapper = wrapperRef.current
    if (!wrapper) return
    const rect = wrapper.getBoundingClientRect()
    const px = event.clientX - rect.left
    const py = event.clientY - rect.top
    setView((current) => {
      const k = Math.min(4, Math.max(0.15, current.k * (event.deltaY < 0 ? 1.1 : 1 / 1.1)))
      const scale = k / current.k
      return { k, x: px - (px - current.x) * scale, y: py - (py - current.y) * scale }
    })
  }, [])

  const onPointerDown = (event: ReactPointerEvent<SVGSVGElement>) => {
    if (event.button !== 0) return
    drag.current = { x: event.clientX, y: event.clientY, vx: view.x, vy: view.y }
    event.currentTarget.setPointerCapture(event.pointerId)
  }

  const onPointerMove = (event: ReactPointerEvent<SVGSVGElement>) => {
    const state = drag.current
    if (!state) return
    setView((current) => ({
      ...current,
      x: state.vx + (event.clientX - state.x),
      y: state.vy + (event.clientY - state.y),
    }))
  }

  const onPointerUp = (event: ReactPointerEvent<SVGSVGElement>) => {
    drag.current = null
    event.currentTarget.releasePointerCapture(event.pointerId)
  }

  const openNode = useCallback(
    (item: Placed) => {
      if (item.external) return
      openTableTab(
        tab.sessionId,
        database,
        schema,
        {
          name: item.name,
          schema,
          database,
          kind: item.node.kind,
          rowEstimate: 0,
          sizeBytes: 0,
        },
        'structure',
      )
    },
    [database, openTableTab, schema, tab.sessionId],
  )

  const exportSvg = useCallback(async () => {
    const svg = svgRef.current
    if (!svg) return
    const clone = svg.cloneNode(true) as SVGSVGElement
    clone.setAttribute('xmlns', 'http://www.w3.org/2000/svg')
    clone.setAttribute('width', String(layout?.width ?? 0))
    clone.setAttribute('height', String(layout?.height ?? 0))
    clone.setAttribute('viewBox', `0 0 ${layout?.width ?? 0} ${layout?.height ?? 0}`)
    // Drop the interactive transform: the file shows the whole diagram.
    clone.querySelector('g[data-dm-diagram]')?.setAttribute('transform', 'translate(0,0)')
    const content = new XMLSerializer().serializeToString(clone)
    const filename = `er-${schema || database || 'diagram'}.svg`
    try {
      await api.saveTextFile({
        defaultFilename: filename,
        content: `<?xml version="1.0" encoding="UTF-8"?>\n${content}`,
        filters: [{ displayName: 'SVG', pattern: '*.svg' }],
      })
      message.success('Diagram exported')
    } catch (err) {
      message.error(toMessage(err))
    }
  }, [database, layout, message, schema])

  if (loading && !graph) {
    return (
      <div style={{ padding: 60, textAlign: 'center' }}>
        <Spin />
      </div>
    )
  }

  if (error) {
    return (
      <div style={{ padding: 14 }}>
        <Alert
          type="error"
          showIcon
          title="Could not read the schema"
          description={
            <span className="mono" style={{ fontSize: 12, whiteSpace: 'pre-wrap' }}>
              {error}
            </span>
          }
          action={
            <Button size="small" icon={<ReloadOutlined />} onClick={() => setAttempt((n) => n + 1)}>
              Retry
            </Button>
          }
        />
      </div>
    )
  }

  const nodes = graph?.nodes ?? []
  const edges = graph?.edges ?? []

  return (
    <div className="dm-pane">
      <div className="dm-editor-toolbar">
        <Input
          size="small"
          allowClear
          prefix={<SearchOutlined style={{ opacity: 0.5 }} />}
          placeholder="Find a table"
          value={query}
          style={{ width: 200 }}
          onChange={(event) => setQuery(event.target.value)}
        />
        <Tooltip title="Zoom out">
          <Button size="small" icon={<ZoomOutOutlined />} onClick={() => zoomBy(1 / 1.2)} />
        </Tooltip>
        <Typography.Text type="secondary" style={{ fontSize: 12, width: 42, textAlign: 'center' }}>
          {Math.round(view.k * 100)}%
        </Typography.Text>
        <Tooltip title="Zoom in">
          <Button size="small" icon={<ZoomInOutlined />} onClick={() => zoomBy(1.2)} />
        </Tooltip>
        <Tooltip title="Fit to window">
          <Button size="small" icon={<ExpandOutlined />} onClick={fit} />
        </Tooltip>
        <Tooltip title="Show the object's columns in each box">
          <Space size={4}>
            <EyeOutlined style={{ opacity: 0.6 }} />
            <Switch size="small" checked={showColumns} onChange={setShowColumns} />
          </Space>
        </Tooltip>
        <Segmented
          size="small"
          value={showGrid ? 'grid' : 'plain'}
          options={[
            { label: 'Grid', value: 'grid' },
            { label: 'Plain', value: 'plain' },
          ]}
          onChange={(value) => setShowGrid(value === 'grid')}
        />

        <div className="dm-toolbar-right">
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {nodes.length} object{nodes.length === 1 ? '' : 's'} · {edges.length} relation
            {edges.length === 1 ? '' : 's'}
          </Typography.Text>
          <Tooltip title="Refresh">
            <Button
              size="small"
              icon={<ReloadOutlined />}
              loading={loading}
              onClick={() => setAttempt((n) => n + 1)}
            />
          </Tooltip>
          <Tooltip title="Export the diagram as SVG">
            <Button size="small" icon={<DownloadOutlined />} onClick={() => void exportSvg()} />
          </Tooltip>
        </div>
      </div>

      {graph?.truncated ? (
        <div style={{ padding: '8px 10px 0', flex: '0 0 auto' }}>
          <Alert
            type="warning"
            showIcon
            title="Only the first 300 objects are shown"
            description="Open the namespace in a narrower scope (a schema, or a database per schema) to see the rest."
          />
        </div>
      ) : null}

      {graph && graph.warnings.length > 0 ? (
        <div style={{ padding: '8px 10px 0', flex: '0 0 auto' }}>
          <Alert
            type="warning"
            showIcon
            title={`${graph.warnings.length} object(s) could not be read in full`}
            description={
              <ul style={{ paddingInlineStart: 18, margin: 0 }}>
                {graph.warnings.slice(0, 6).map((warning, index) => (
                  <li key={index}>
                    <Typography.Text style={{ fontSize: 12 }}>{warning}</Typography.Text>
                  </li>
                ))}
              </ul>
            }
          />
        </div>
      ) : null}

      <div className="dm-er-canvas" ref={wrapperRef}>
        {nodes.length === 0 ? (
          <Empty description="This namespace has no objects" style={{ marginTop: 80 }} />
        ) : (
          <svg
            ref={svgRef}
            className="dm-er-svg"
            width="100%"
            height="100%"
            onWheel={onWheel}
            onPointerDown={onPointerDown}
            onPointerMove={onPointerMove}
            onPointerUp={onPointerUp}
            style={{ background: palette.bg, touchAction: 'none' }}
          >
            <defs>
              <pattern
                id="dm-er-grid"
                width={20}
                height={20}
                patternUnits="userSpaceOnUse"
              >
                <path d="M 20 0 L 0 0 0 20" fill="none" stroke={palette.grid} strokeWidth={1} />
              </pattern>
              <marker
                id="dm-er-arrow"
                viewBox="0 0 10 10"
                refX={9}
                refY={5}
                markerWidth={7}
                markerHeight={7}
                orient="auto-start-reverse"
              >
                <path d="M 0 0 L 10 5 L 0 10 z" fill={palette.edge} />
              </marker>
              <marker
                id="dm-er-arrow-active"
                viewBox="0 0 10 10"
                refX={9}
                refY={5}
                markerWidth={7}
                markerHeight={7}
                orient="auto-start-reverse"
              >
                <path d="M 0 0 L 10 5 L 0 10 z" fill={palette.accent} />
              </marker>
            </defs>

            {showGrid ? (
              <rect width="100%" height="100%" fill="url(#dm-er-grid)" />
            ) : null}

            <g data-dm-diagram transform={`translate(${view.x} ${view.y}) scale(${view.k})`}>
              {layout
                ? layout.placed.map((item) => (
                    <Box
                      key={`box-${item.name}`}
                      item={item}
                      palette={palette}
                      dim={needle !== '' && !matched.has(item.name)}
                      hovered={hovered === item.name}
                      onEnter={() => setHovered(item.name)}
                      onLeave={() => setHovered(null)}
                      onOpen={() => openNode(item)}
                    />
                  ))
                : null}

              {layout
                ? layout.placed.flatMap((item) =>
                    (item.external ? [] : edgesFor(edges, item.name)).map((edge, index) => {
                      const source = layout.byName.get(edge.from)
                      const target = layout.byName.get(edge.to)
                      if (!source || !target) return null
                      const active = hovered === edge.from || hovered === edge.to
                      const faded =
                        (needle !== '' && !matched.has(edge.from) && !matched.has(edge.to)) ||
                        (hovered !== null && !active)
                      return (
                        <EdgePath
                          key={`edge-${item.name}-${index}`}
                          edge={edge}
                          source={source}
                          target={target}
                          palette={palette}
                          active={active}
                          faded={faded}
                        />
                      )
                    }),
                  )
                : null}
            </g>
          </svg>
        )}
      </div>

      <div className="dm-statusbar" style={{ borderTop: '1px solid var(--dm-border)' }}>
        <span className="dm-statusbar-item">
          {schema ? `${database}.${schema}` : database}
        </span>
        {hovered ? <span className="dm-statusbar-item">focused: {hovered}</span> : null}
        <span className="dm-spacer" />
        <span className="dm-statusbar-item">
          click a box to open it · drag to pan · wheel to zoom
        </span>
      </div>
    </div>
  )
}

function edgesFor(edges: GraphEdge[], name: string): GraphEdge[] {
  // Draw each relation once, from the child side, so parallel keys to the same
  // parent do not overdraw each other.
  return edges.filter((edge) => edge.from === name)
}

/** One entity box: header, primary-key aware column rows, footer. */
function Box({
  item,
  palette,
  dim,
  hovered,
  onEnter,
  onLeave,
  onOpen,
}: {
  item: Placed
  palette: Palette
  dim: boolean
  hovered: boolean
  onEnter: () => void
  onLeave: () => void
  onOpen: () => void
}) {
  const border = hovered ? palette.accent : palette.border
  const opacity = dim ? 0.28 : 1
  return (
    <g
      transform={`translate(${item.x} ${item.y})`}
      opacity={opacity}
      style={{ cursor: item.external ? 'default' : 'pointer' }}
      onMouseEnter={onEnter}
      onMouseLeave={onLeave}
      onClick={onOpen}
    >
      <rect
        width={item.w}
        height={item.h}
        rx={6}
        fill={palette.box}
        stroke={border}
        strokeWidth={hovered ? 2 : 1}
        strokeDasharray={item.external ? '5 4' : undefined}
      />
      <path
        d={`M 0 6 A 6 6 0 0 1 6 0 L ${item.w - 6} 0 A 6 6 0 0 1 ${item.w} 6 L ${item.w} ${HEADER_H} L 0 ${HEADER_H} Z`}
        fill={item.external ? palette.dim : palette.accent}
        opacity={item.external ? 0.35 : 0.16}
      />
      <text
        x={10}
        y={HEADER_H / 2 + 4}
        fill={palette.text}
        fontSize={12.5}
        fontWeight={600}
        style={{ fontFamily: 'inherit' }}
      >
        {item.name}
      </text>
      <text
        x={item.w - 10}
        y={HEADER_H / 2 + 4}
        fill={palette.dim}
        fontSize={10.5}
        textAnchor="end"
        style={{ fontFamily: 'inherit' }}
      >
        {item.external ? 'external' : item.node.kind === 'table' ? '' : item.node.kind}
      </text>

      {item.node.columns.slice(0, item.rows).map((column, index) => (
        <g key={column.name} transform={`translate(0 ${HEADER_H + index * ROW_H})`}>
          <text
            x={10}
            y={ROW_H / 2 + 4}
            fill={column.primaryKey ? palette.pk : palette.text}
            fontSize={11}
            fontWeight={column.primaryKey ? 600 : 400}
            style={{ fontFamily: 'inherit' }}
          >
            {column.primaryKey ? '🔑 ' : ''}
            {column.name}
          </text>
          <text
            x={item.w - 10}
            y={ROW_H / 2 + 4}
            fill={palette.dim}
            fontSize={10.5}
            textAnchor="end"
            style={{ fontFamily: 'inherit' }}
          >
            {column.type}
            {column.nullable ? '' : ' *'}
          </text>
        </g>
      ))}

      {item.hidden > 0 ? (
        <text
          x={10}
          y={HEADER_H + item.rows * ROW_H + ROW_H / 2 + 4}
          fill={palette.dim}
          fontSize={10.5}
          style={{ fontFamily: 'inherit' }}
        >
          + {item.hidden} more column(s)
        </text>
      ) : null}
    </g>
  )
}

/** A foreign key, drawn as a bezier arrow between two boxes. */
function EdgePath({
  edge,
  source,
  target,
  palette,
  active,
  faded,
}: {
  edge: GraphEdge
  source: Placed
  target: Placed
  palette: Palette
  active: boolean
  faded: boolean
}) {
  const y1 = source.y + columnOffset(source, edge.fromColumns)
  const y2 = target.y + columnOffset(target, edge.toColumns)
  const right = source.x + source.w
  const stroke = active ? palette.accent : palette.edge

  // A self-reference loops out of the right edge and back into the left one.
  const selfLoop = `M ${right} ${y1 - 6} C ${right + 46} ${y1 - 46}, ${right + 46} ${y1 + 46}, ${right} ${y1 + 6}`

  // Otherwise the arrow leaves the edge that faces the other box. Rows are name
  // families, so a foreign key can point at a table drawn further left than the
  // one holding it: then the arrow leaves the source's left edge and comes into
  // the target's right one, looping around instead of crossing the box it
  // starts at. Boxes that share a column keep the plain facing-edge curve.
  const leftward = target.x + target.w <= source.x
  const x1 = leftward ? source.x : right
  const x2 = leftward ? target.x + target.w : target.x
  const bow = Math.max(40, Math.abs(x2 - x1) / 2)
  const path =
    edge.from === edge.to
      ? selfLoop
      : leftward
        ? `M ${x1} ${y1} C ${x1 - bow} ${y1}, ${x2 + bow} ${y2}, ${x2} ${y2}`
        : `M ${x1} ${y1} C ${x1 + bow} ${y1}, ${x2 - bow} ${y2}, ${x2} ${y2}`

  return (
    <g opacity={faded ? 0.15 : 1}>
      <path
        d={path}
        fill="none"
        stroke={stroke}
        strokeWidth={active ? 2 : 1.2}
        strokeDasharray={target.external ? '5 4' : undefined}
        markerEnd={active ? 'url(#dm-er-arrow-active)' : 'url(#dm-er-arrow)'}
      />
      {active ? (
        <text
          x={(x1 + x2) / 2}
          y={(y1 + y2) / 2 - 5}
          fill={palette.accent}
          fontSize={10.5}
          textAnchor="middle"
          style={{ fontFamily: 'inherit' }}
        >
          {edge.fromColumns.join(', ')} → {edge.toColumns.join(', ')}
        </text>
      ) : null}
    </g>
  )
}
