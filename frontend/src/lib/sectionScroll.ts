/**
 * The left column and the one scroll box beside it.
 *
 * The settings page reads a long list of sections as one scroll, with the names
 * down the left: the highlight follows the scroll, and a click moves the scroll.
 * The placeholder picker reads its groups the same way. Both use this, because
 * the rules below are not obvious enough to be written out twice — and a pair
 * that behaved differently in the two places would be a bug in one of them.
 *
 * The sections are all present at once, which is what makes a scroll position
 * mean something: the section being read is the last one whose heading has
 * reached the top of the box.
 */
import { useCallback, useEffect, useRef, useState, type RefObject } from 'react'

export interface SectionScrollOptions<T extends string> {
  /** The sections, in the order they are shown. */
  keys: readonly T[]
  /** The element id a section carries, which is also what its nav button points at. */
  idOf: (key: T) => string
  /** Whether the pair is on screen; a box that is hidden cannot be measured. */
  active: boolean
  /**
   * How far below the top of the scroll box a heading has to be for the section
   * to count as the one being read. A small offset rather than zero, because a
   * heading exactly level with the edge is half cut off and does not read as
   * "here".
   */
  marker?: number
  /**
   * How long a click's smooth scroll is given before scrolling is read again.
   * Those scroll events are the click's own doing, and reading them back would
   * run the highlight through every section on the way there.
   */
  settleMs?: number
}

export interface SectionScroll<T extends string> {
  /** The section being read. */
  current: T
  /** The scroll box; its `onScroll` has to be wired to `onScroll` below. */
  bodyRef: RefObject<HTMLDivElement | null>
  onScroll: () => void
  /** Moves the scroll to a section, and the highlight with it. */
  goTo: (key: T) => void
  /** Puts the pair back at the first section, for a box that is opened afresh. */
  reset: () => void
}

export function useSectionScroll<T extends string>({
  keys,
  idOf,
  active,
  marker = 24,
  settleMs = 420,
}: SectionScrollOptions<T>): SectionScroll<T> {
  const [current, setCurrent] = useState<T>(keys[0])
  const bodyRef = useRef<HTMLDivElement | null>(null)
  // The section a click asked for, held while its smooth scroll is animating.
  const scrollingTo = useRef<T | undefined>(undefined)
  const settleTimer = useRef<number | undefined>(undefined)

  const sectionOf = useCallback(
    (key: T): HTMLElement | null => bodyRef.current?.querySelector<HTMLElement>(`#${idOf(key)}`) ?? null,
    [idOf],
  )

  /**
   * Reads the scroll position back into the column.
   *
   * The end of the list is the exception to the rule above: the final section can
   * be too short to ever reach the top, so the bottom of the scroll means the
   * bottom of the list.
   */
  const sync = useCallback(() => {
    const body = bodyRef.current
    if (!body || scrollingTo.current) return
    const top = body.getBoundingClientRect().top
    let next = keys[0]
    for (const key of keys) {
      const section = sectionOf(key)
      if (section && section.getBoundingClientRect().top - top <= marker) next = key
    }
    if (body.scrollTop + body.clientHeight >= body.scrollHeight - 2) {
      next = keys[keys.length - 1]
    }
    setCurrent(next)
  }, [keys, marker, sectionOf])

  const goTo = useCallback(
    (key: T) => {
      const body = bodyRef.current
      const section = sectionOf(key)
      setCurrent(key)
      if (!body || !section) return
      scrollingTo.current = key
      const top = body.scrollTop + section.getBoundingClientRect().top - body.getBoundingClientRect().top
      body.scrollTo({ top, behavior: 'smooth' })
      window.clearTimeout(settleTimer.current)
      settleTimer.current = window.setTimeout(() => {
        scrollingTo.current = undefined
        sync()
      }, settleMs)
    },
    [sectionOf, settleMs, sync],
  )

  useEffect(() => () => window.clearTimeout(settleTimer.current), [])

  // Coming to the front re-measures: a section can have grown while it was
  // hidden, which moves every heading below it.
  useEffect(() => {
    if (active) sync()
  }, [active, sync])

  const reset = useCallback(() => setCurrent(keys[0]), [keys])

  return { current, bodyRef, onScroll: sync, goTo, reset }
}
