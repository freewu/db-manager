/**
 * The placeholder picker of the data generation window.
 *
 * It is a catalogue, not an editor: picking a tile sets that field's mock and
 * closes the modal. The field stays editable, so a template that starts from a
 * picked placeholder (`order-@natural(1, 9999)`) is written in the cell, where
 * the description column can explain it.
 *
 * The groups are names down the left and each placeholder is a tile, because the
 * interesting part of a placeholder is not its name but what it produces: every
 * tile renders an example of itself, from the same engine the window uses, so two
 * placeholders that sound alike can be told apart before one is picked. The
 * column and the box are read the way the settings page reads its sections — the
 * highlight follows the scroll, and a click moves the scroll — so the wheel over
 * the list walks the groups instead of doing nothing.
 *
 * The last group is the user's own placeholders — the ones the settings page keeps
 * in the data directory's `.mock` folder. They are shown like the built-ins, with
 * an example rendered from their template, which is what makes a custom
 * placeholder checkable at the moment of use rather than only where it is written.
 * A placeholder file that cannot be read is not offered (picking it could only
 * fail); it is reported instead, with a pointer to the settings page where it can
 * be removed.
 */
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Empty, Input, Modal, Tag, Typography } from 'antd'

import {
  PLACEHOLDER_GROUPS,
  sampleOf,
  type CustomPlaceholder,
  type Placeholder,
  type PlaceholderGroup,
} from '../lib/mock'
import { useSectionScroll } from '../lib/sectionScroll'
import { useAppStore } from '../store/appStore'

/** The key of the group holding the user's own placeholders. */
const CUSTOM_GROUP = 'custom'

/**
 * How long the next scroll notch waits before it walks another group. A wheel
 * fires a burst of events per notch, so without a cooldown one flick would run
 * from the first group to the last.
 */
const WHEEL_COOLDOWN_MS = 150

/** The id a group's element carries, so the column can point at it and the scroll can find it. */
const groupPaneId = (key: string) => `dm-mock-group-${key}`

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
  /** The group's label, shown only in search results, where the groups' headings are not beside them. */
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
  const unbindNav = useRef<(() => void) | null>(null)
  const steppedAt = useRef(0)

  const stored = useAppStore((state) => state.mockPlaceholders)
  const refreshMockPlaceholders = useAppStore((state) => state.refreshMockPlaceholders)
  const open = field !== undefined

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
  const groupKeys = useMemo(() => groups.map((entry) => entry.key), [groups])

  /**
   * The groups are read the way the settings page's sections are: the names down
   * the left, one scroll box beside them, the highlight following the scroll and
   * a click moving it. All the groups are in the box at once, which is what makes
   * a scroll position mean anything: the wheel over the tiles walks the groups
   * because the tiles of the next group are below the ones on screen.
   */
  const { current: group, bodyRef, onScroll, goTo, reset } = useSectionScroll({
    keys: groupKeys,
    idOf: groupPaneId,
    active: open,
  })

  // The list is only as long as the last read, so the modal re-reads the folder
  // as it opens: another window (or a hand-edited file) may have changed it since,
  // which also moves the groups the scroll is read against. A failure leaves the
  // list as it was rather than emptying it.
  useEffect(() => {
    if (!open) return
    setQuery('')
    reset()
    refreshMockPlaceholders().catch(() => undefined)
  }, [open, field, reset, refreshMockPlaceholders])

  /**
   * The wheel over the names walks the groups.
   *
   * The box beside them answers the wheel by scrolling, and a column that answers
   * it by moving two pixels reads as broken: the wheel over the column is how the
   * next group is reached. Bound to the element with a callback ref rather than
   * to a ref read in an effect, because the modal's body is only mounted a render
   * or two after it opens — an effect would run with nothing to listen on.
   *
   * Only the column: the box beside it is what scrolls through the groups, and the
   * wheel over it is left alone to do that.
   */
  const bindNav = useCallback(
    (node: HTMLElement | null) => {
      unbindNav.current?.()
      unbindNav.current = null
      if (!node) return
      const onWheel = (event: WheelEvent) => {
        if (Math.abs(event.deltaY) < 4) return
        const at = groups.findIndex((entry) => entry.key === group)
        const next = at < 0 ? undefined : groups[at + (event.deltaY > 0 ? 1 : -1)]
        // At either end there is no next group, and the wheel is left to the
        // column itself — it may have a notch of its own to give.
        if (!next) return
        event.preventDefault()
        event.stopPropagation()
        // One group per notch: a wheel fires a burst of events per notch, and
        // without the wait a single flick would run from the first group to the
        // last.
        const now = Date.now()
        if (now - steppedAt.current < WHEEL_COOLDOWN_MS) return
        steppedAt.current = now
        goTo(next.key)
      }
      node.addEventListener('wheel', onWheel)
      unbindNav.current = () => node.removeEventListener('wheel', onWheel)
    },
    [groups, group, goTo],
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
    // Wide enough for four tiles to a row beside the group names, and the same
    // whether those names are showing or a search's results are not.
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
        // Every group lives in one scroll box: the column beside it is read back
        // from that scroll, and a click moves it, so the two always agree.
        <div className="dm-settings-page dm-mock-groups">
          <nav
            className="dm-settings-nav"
            role="tablist"
            aria-label="Placeholder groups"
            ref={bindNav}
          >
            {groups.map((entry) => (
              <button
                key={entry.key}
                type="button"
                role="tab"
                aria-selected={group === entry.key}
                aria-controls={groupPaneId(entry.key)}
                className={`dm-settings-nav-item${group === entry.key ? ' is-active' : ''}`}
                onClick={() => goTo(entry.key)}
              >
                {entry.label}
              </button>
            ))}
          </nav>
          <div className="dm-settings-body dm-mock-body" ref={bodyRef} onScroll={onScroll}>
            {groups.map((entry) => (
              <section
                key={entry.key}
                className="dm-settings-section"
                id={groupPaneId(entry.key)}
                role="tabpanel"
                aria-label={entry.label}
              >
                <h2 className="dm-settings-section-title">{entry.label}</h2>
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
              </section>
            ))}
          </div>
        </div>
      )}
    </Modal>
  )
}
