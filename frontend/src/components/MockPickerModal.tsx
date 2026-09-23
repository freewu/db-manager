/**
 * The placeholder picker of the data generation window.
 *
 * It is a catalogue, not an editor: picking a tile sets that field's mock and
 * closes the modal. The field stays editable, so a template that starts from a
 * picked placeholder (`order-@natural(1, 9999)`) is written in the cell, where
 * the description column can explain it.
 *
 * The groups are tabs down the left side and each placeholder is a tile, because
 * the interesting part of a placeholder is not its name but what it produces:
 * every tile renders an example of itself, from the same engine the window uses,
 * so two placeholders that sound alike can be told apart before one is picked.
 *
 * The last tab is the user's own placeholders — the ones the settings page keeps
 * in the data directory's `.mock` folder. They are shown like the built-ins,
 * with an example rendered from their template, which is what makes a custom
 * placeholder checkable at the moment of use rather than only where it is
 * written. A placeholder file that cannot be read is not offered (picking it
 * could only fail); it is reported instead, with a pointer to the settings page
 * where it can be removed.
 */
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Empty, Input, Modal, Tabs, Tag, Typography } from 'antd'

import {
  PLACEHOLDER_GROUPS,
  sampleOf,
  type CustomPlaceholder,
  type Placeholder,
  type PlaceholderGroup,
} from '../lib/mock'
import { useAppStore } from '../store/appStore'

/** The key of the tab holding the user's own placeholders. */
const CUSTOM_GROUP = 'custom'

/**
 * How long the next scroll notch waits before it walks another group. A wheel
 * fires a burst of events per notch, so without a cooldown one flick would run
 * from the first group to the last.
 */
const WHEEL_COOLDOWN_MS = 150

/**
 * Brings the group that just became active into the strip's own view.
 *
 * The strip is read from the top, so the tab being read has to be the one on
 * screen. Measured from the rectangles rather than `offsetTop`, because the nav
 * is not the tab's offset parent.
 */
function showActiveTab(host: HTMLElement) {
  const nav = host.querySelector<HTMLElement>('.ant-tabs-nav')
  const tab = nav?.querySelector<HTMLElement>('.ant-tabs-tab-active')
  if (!nav || !tab || nav.scrollHeight <= nav.clientHeight) return
  const navBox = nav.getBoundingClientRect()
  const tabBox = tab.getBoundingClientRect()
  nav.scrollTop += tabBox.top + tabBox.height / 2 - (navBox.top + navBox.height / 2)
}

interface MockPickerModalProps {
  /** The field being filled; the modal is closed while this is undefined. */
  field?: string
  /** Called with the placeholder text, which becomes the field's mock. */
  onPick: (value: string) => void
  onClose: () => void
}

/**
 * One tile: the placeholder, an example of it, and what it is.
 *
 * The example is rendered once per item and seed, so it is stable across
 * redraws — an example that changed every time the modal was opened would say
 * nothing about the placeholder, only about the random source.
 */
function PlaceholderTile({
  item,
  placeholders,
  tag,
  onPick,
}: {
  item: Placeholder
  placeholders: CustomPlaceholder[]
  /** The group's label, shown only in search results, where tabs are gone. */
  tag?: string
  onPick: (value: string) => void
}) {
  const sample = useMemo(
    () => sampleOf(item.value, placeholders),
    [item.value, placeholders],
  )
  return (
    <button type="button" className="dm-mock-tile" onClick={() => onPick(item.value)}>
      <span className="dm-mock-tile-head">
        <span className="mono dm-mock-value">{item.value}</span>
        {tag ? (
          <Tag style={{ marginInlineStart: 'auto', fontSize: 10, lineHeight: '14px' }}>{tag}</Tag>
        ) : null}
      </span>
      {sample.error ? (
        <span className="dm-mock-sample dm-mock-sample-bad">{sample.error}</span>
      ) : (
        <span className="mono dm-mock-sample">{sample.value ?? '—'}</span>
      )}
      <span className="dm-mock-desc">{item.desc}</span>
    </button>
  )
}

export function MockPickerModal({ field, onPick, onClose }: MockPickerModalProps) {
  const [query, setQuery] = useState('')
  const [group, setGroup] = useState(PLACEHOLDER_GROUPS[0].key)
  const unbindStrip = useRef<(() => void) | null>(null)
  const steppedAt = useRef(0)

  const stored = useAppStore((state) => state.mockPlaceholders)
  const refreshMockPlaceholders = useAppStore((state) => state.refreshMockPlaceholders)
  const open = field !== undefined

  // The tab strip is only as long as the last read, so the modal re-reads the
  // folder as it opens: another window (or a hand-edited file) may have changed
  // it since. A failure leaves the list as it was rather than emptying it.
  useEffect(() => {
    if (!open) return
    setQuery('')
    setGroup(PLACEHOLDER_GROUPS[0].key)
    refreshMockPlaceholders().catch(() => undefined)
  }, [open, field, refreshMockPlaceholders])

  // What the engine compiles with. Broken files are left out of it on purpose:
  // they have no template to render, and a name that cannot be rendered is a
  // name this picker must not offer.
  const broken = useMemo(() => stored.filter((entry) => entry.broken), [stored])
  const placeholders = useMemo<CustomPlaceholder[]>(
    () =>
      stored
        .filter((entry) => !entry.broken)
        .map((entry) => ({
          name: entry.name,
          template: entry.template,
          description: entry.description,
        })),
    [stored],
  )

  const groups = useMemo<PlaceholderGroup[]>(
    () => [
      ...PLACEHOLDER_GROUPS,
      {
        key: CUSTOM_GROUP,
        label: 'Custom',
        items: placeholders.map((entry) => ({
          value: `@${entry.name}`,
          desc: entry.description?.trim() || entry.template,
        })),
      },
    ],
    [placeholders],
  )

  /**
   * The wheel over the tab strip walks the groups.
   *
   * The groups are a column down the left, and a column that answers the wheel
   * by moving two pixels reads as broken: the wheel over it is how the next group
   * is reached.
   *
   * The listener is bound to the strip's element rather than to a ref read in an
   * effect, because the modal's body is only mounted a render or two after it
   * opens — an effect would run with nothing to listen on. It is registered in
   * the capture phase so that the event stops here: antd slides the tab list
   * under the wheel as well, and a strip that both walks the groups and slides
   * under them moves two groups at a time.
   *
   * Only the strip: the tiles below are a list of their own, and a wheel over
   * them scrolls them.
   */
  const bindStrip = useCallback(
    (node: HTMLDivElement | null) => {
      unbindStrip.current?.()
      unbindStrip.current = null
      if (!node) return
      const onWheel = (event: WheelEvent) => {
        const target = event.target as HTMLElement | null
        if (!target?.closest('.ant-tabs-nav') || Math.abs(event.deltaY) < 4) return
        const at = groups.findIndex((entry) => entry.key === group)
        const next = at < 0 ? undefined : groups[at + (event.deltaY > 0 ? 1 : -1)]
        // At either end there is no next group, and the wheel is left to the strip
        // itself — it may have a notch of its own to give.
        if (!next) return
        event.preventDefault()
        event.stopPropagation()
        // One group per notch: a wheel fires a burst of events per notch, and
        // without the wait a single flick would run from the first group to the
        // last.
        const now = Date.now()
        if (now - steppedAt.current < WHEEL_COOLDOWN_MS) return
        steppedAt.current = now
        setGroup(next.key)
        window.requestAnimationFrame(() => showActiveTab(node))
      }
      node.addEventListener('wheel', onWheel, { capture: true })
      unbindStrip.current = () => node.removeEventListener('wheel', onWheel, { capture: true })
    },
    [groups, group],
  )

  const needle = query.trim().toLowerCase()
  const matches = useMemo(() => {
    if (!needle) return []
    const found: { group: string; item: Placeholder }[] = []
    for (const entry of groups) {
      for (const item of entry.items) {
        if (item.value.toLowerCase().includes(needle) || item.desc.toLowerCase().includes(needle)) {
          found.push({ group: entry.label, item })
        }
      }
    }
    return found
  }, [needle, groups])

  const brokenNote = broken.length ? (
    <Typography.Text type="secondary" className="dm-mock-note">
      {broken.length === 1
        ? `@${broken[0].name} could not be read (${broken[0].broken}) and is not offered.`
        : `${broken.length} placeholder files could not be read and are not offered.`}{' '}
      Settings › Mock placeholders lists them so they can be removed.
    </Typography.Text>
  ) : null

  return (
    // Wide enough for four tiles to a row inside the left tab strip, and the same
    // whether the group tabs are showing or a search's results are not.
    <Modal
      open={open}
      title={field ? `Placeholder for “${field}”` : 'Placeholder'}
      onCancel={onClose}
      footer={
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          A mock is a template: text with placeholders in it, so{' '}
          <span className="mono">user_@natural(1, 999)</span> is a value too. Write{' '}
          <span className="mono">@@</span> for a literal @.
        </Typography.Text>
      }
      width={880}
      destroyOnHidden
    >
      <Input
        allowClear
        autoFocus
        placeholder="Search a placeholder, e.g. email or 邮箱"
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        style={{ marginBottom: 8 }}
      />
      {brokenNote}
      <div ref={bindStrip}>
        {needle ? (
          matches.length === 0 ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No placeholder matches" />
          ) : (
            <div className="dm-mock-pane">
              <div className="dm-mock-tiles">
                {matches.map(({ group: label, item }) => (
                  <PlaceholderTile
                    key={`${label}:${item.value}`}
                    item={item}
                    placeholders={placeholders}
                    tag={label}
                    onPick={onPick}
                  />
                ))}
              </div>
            </div>
          )
        ) : (
          <Tabs
            className="dm-mock-tabs"
            tabPosition="left"
            size="small"
            activeKey={group}
            onChange={setGroup}
            items={groups.map((entry) => ({
              key: entry.key,
              label: entry.label,
              children: (
                <div className="dm-mock-pane">
                  {entry.items.length === 0 ? (
                    <Empty
                      image={Empty.PRESENTED_IMAGE_SIMPLE}
                      description={
                        <span>
                          No custom placeholder yet.
                          <br />
                          Settings › Mock placeholders writes one — a name for a template such as{' '}
                          <span className="mono">SO@date(yyyy)@natural(1000, 9999)</span>.
                        </span>
                      }
                    />
                  ) : (
                    <div className="dm-mock-tiles">
                      {entry.items.map((item) => (
                        <PlaceholderTile
                          key={item.value}
                          item={item}
                          placeholders={placeholders}
                          onPick={onPick}
                        />
                      ))}
                    </div>
                  )}
                </div>
              ),
            }))}
          />
        )}
      </div>
    </Modal>
  )
}
