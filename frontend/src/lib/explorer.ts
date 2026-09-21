import type {
  ConnectionConfig,
  ConnectionGroup,
  ConnectionLayout,
  ConnectionPlacement,
} from '../api/types'

/**
 * The connection explorer's arrangement, as the pane draws it.
 *
 * The backend owns the arrangement (`ConnectionLayout`, stored in layout.json)
 * and re-derives it against the profiles that exist on every read and write.
 * This module is the UI's mirror of that shape: it turns the flat
 * groups-and-placements into the tree the pane renders, and turns a drag back
 * into a layout to send. Keeping both directions here — and away from the
 * component — means the tree and the drop handler cannot disagree about what a
 * "position" is.
 *
 * Order is one sequence for the top level, shared by the groups and the
 * ungrouped connections, and one sequence inside each group. That is what makes
 * a connection above a folder, and a folder above a connection, expressible.
 */

/** A top-level entry of the explorer: a group, or a connection that has none. */
export type ExplorerEntry =
  | { t: 'group'; id: string; name: string; members: string[] }
  | { t: 'connection'; id: string }

/**
 * Where a dragged entry may land.
 *
 * `root` and `member` name the neighbour it goes next to, which is what the
 * drop indicator shows; `inside` is a drop onto a group's own row.
 */
export type DropTarget =
  | { t: 'root'; neighborId: string; after: boolean }
  | { t: 'inside'; groupId: string }
  | { t: 'member'; groupId: string; neighborId: string; after: boolean }

/**
 * Builds the top level from the stored arrangement.
 *
 * Everything is defensive, because the pane can be rendering a layout that is a
 * moment behind the profile list: an entry for a profile that is not there is
 * ignored, and a profile with no entry is appended at the bottom instead of
 * going missing. The backend applies the same rules to what it stores, so this
 * only ever papers over the window between two fetches.
 */
export function arrangementOf(
  connections: ConnectionConfig[],
  layout: ConnectionLayout | undefined,
): ExplorerEntry[] {
  const known = new Set(connections.map((profile) => profile.id))
  const groups = (layout?.groups ?? []).filter((group) => group.id)
  const groupIds = new Set(groups.map((group) => group.id))

  const members = new Map<string, ConnectionPlacement[]>()
  const root: ConnectionPlacement[] = []
  const placed = new Set<string>()
  for (const entry of layout?.items ?? []) {
    if (!known.has(entry.id) || placed.has(entry.id)) continue
    placed.add(entry.id)
    const groupId = entry.groupId
    if (groupId && groupIds.has(groupId)) {
      const list = members.get(groupId)
      if (list) list.push(entry)
      else members.set(groupId, [entry])
      continue
    }
    root.push({ ...entry, groupId: undefined })
  }

  const slots: { order: number; seq: number; entry: ExplorerEntry }[] = []
  groups.forEach((group, seq) => {
    slots.push({
      order: group.order,
      seq,
      entry: {
        t: 'group',
        id: group.id,
        name: group.name,
        members: sortedIds(members.get(group.id) ?? []),
      },
    })
  })
  root.forEach((entry, seq) => {
    slots.push({ order: entry.order, seq, entry: { t: 'connection', id: entry.id } })
  })
  slots.sort((left, right) => {
    if (left.order !== right.order) return left.order - right.order
    // A group wins a tie, matching the backend: a folder is drawn next to what
    // is inside it rather than after a connection that shares its number.
    if (left.entry.t !== right.entry.t) return left.entry.t === 'group' ? -1 : 1
    return left.seq - right.seq
  })

  const entries = slots.map((slot) => slot.entry)
  for (const profile of connections) {
    if (placed.has(profile.id)) continue
    placed.add(profile.id)
    entries.push({ t: 'connection', id: profile.id })
  }
  return entries
}

/** Turns the drawn tree back into a layout, numbering both sequences from zero. */
export function layoutOf(entries: ExplorerEntry[]): ConnectionLayout {
  const groups: ConnectionGroup[] = []
  const items: ConnectionPlacement[] = []
  entries.forEach((entry, index) => {
    if (entry.t === 'connection') {
      items.push({ id: entry.id, order: index })
      return
    }
    groups.push({ id: entry.id, name: entry.name, order: index })
    entry.members.forEach((id, position) => {
      items.push({ id, groupId: entry.id, order: position })
    })
  })
  return { groups, items }
}

/** The group holding a connection, or `undefined` when it sits at the top level. */
export function groupOf(entries: ExplorerEntry[], id: string): string | undefined {
  for (const entry of entries) {
    if (entry.t === 'group' && entry.members.includes(id)) return entry.id
  }
  return undefined
}

/**
 * Moves an entry to where the drop asked for and returns the new top level.
 *
 * Everything that cannot be expressed — a folder inside a folder, a folder
 * inside itself, a neighbour that is not there — returns the arrangement
 * untouched, so an odd drop is a no-op rather than a way to lose a connection.
 * That is why every branch checks that the place it is about to write into is
 * really there: an entry that has been taken out is only put back at a place
 * that exists.
 */
export function moveEntry(
  entries: ExplorerEntry[],
  dragId: string,
  target: DropTarget,
): ExplorerEntry[] {
  const { rest, moved } = detach(entries, dragId)
  if (!moved) return entries

  const intoGroup = target.t === 'inside' || target.t === 'member'
  if (moved.t === 'group') {
    // One level deep: a group may not be dropped into a group, and dropping it
    // next to its own name would move it nowhere.
    if (intoGroup) return entries
  }
  if (target.t === 'inside') {
    const into = rest.find((entry) => entry.t === 'group' && entry.id === target.groupId)
    if (!into) return entries
    return rest.map((entry) =>
      entry.t === 'group' && entry.id === target.groupId
        ? { ...entry, members: [...entry.members, moved.id] }
        : entry,
    )
  }
  if (target.t === 'member') {
    const at = rest.findIndex((entry) => entry.t === 'group' && entry.id === target.groupId)
    if (at < 0) return entries
    const members = (rest[at] as Extract<ExplorerEntry, { t: 'group' }>).members
    const slot = members.indexOf(target.neighborId)
    // The neighbour may have gone with the dragged entry (aiming just after
    // itself), which is a drop that moves nothing rather than one that drops it.
    if (slot < 0) return entries
    const next = insert(members, moved.id, target.after ? slot + 1 : slot)
    return rest.map((entry, index) =>
      index === at ? { ...entry, members: next } : entry,
    )
  }

  const at = rest.findIndex((entry) => entry.id === target.neighborId)
  if (at < 0) return entries
  return insert(rest, moved, target.after ? at + 1 : at)
}

/** Takes an entry out of the arrangement, wherever it sits. */
function detach(
  entries: ExplorerEntry[],
  id: string,
): { rest: ExplorerEntry[]; moved?: ExplorerEntry } {
  let moved: ExplorerEntry | undefined
  const rest: ExplorerEntry[] = []
  for (const entry of entries) {
    if (entry.id === id) {
      moved = entry
      continue
    }
    if (entry.t === 'group' && entry.members.includes(id)) {
      moved = { t: 'connection', id }
      rest.push({ ...entry, members: entry.members.filter((member) => member !== id) })
      continue
    }
    rest.push(entry)
  }
  return { rest, moved }
}

function insert<T>(list: T[], value: T, at: number): T[] {
  return [...list.slice(0, at), value, ...list.slice(at)]
}

function sortedIds(entries: ConnectionPlacement[]): string[] {
  return [...entries].sort((left, right) => left.order - right.order).map((entry) => entry.id)
}
