import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Splitter } from 'antd'

import { ConnectProvider } from '../hooks/useConnect'
import { capabilitiesOf, findDriver } from '../lib/capabilities'
import { datagenTarget, useAppStore, type AppPage } from '../store/appStore'
import { ActivityBar } from './ActivityBar'
import { ChangeLogPane } from './ChangeLogPane'
import { ConnectionDialog } from './ConnectionDialog'
import { ConnectionSidebar } from './ConnectionSidebar'
import { DataGenWorkspace } from './DataGenWorkspace'
import { MainToolbar } from './MainToolbar'
import { SettingsPane } from './SettingsPane'
import { StatusBar } from './StatusBar'
import { Workspace } from './Workspace'

/** How wide the connection tree opens, in pixels. */
const SIDEBAR_WIDTH = 310

/**
 * Application chrome, Navicat style: command ribbon, page rail, explorer and
 * status bar — with the four pages' main areas stacked in one place.
 *
 * The main area is not one pane that swaps its content: each page is a layer
 * that is mounted the first time it is shown and then kept, so leaving a page
 * never throws away what is on it (see `PageLayer`). What holds them together is
 * the width of the explorer: folding it away is a fact about this layout, so it
 * is owned here rather than by the rail that asks for it.
 */
export function AppShell() {
  const page = useAppStore((s) => s.page)
  const setPage = useAppStore((s) => s.setPage)
  const openDataGenTab = useAppStore((s) => s.openDataGenTab)
  const target = useAppStore(datagenTarget)
  const activeNamespace = useAppStore((s) => s.activeNamespace)
  const drivers = useAppStore((s) => s.drivers)
  // Whether there is already a generation window to go back to: a page that
  // holds windows is not a page whose command can be unavailable.
  const hasDataGenWindow = useAppStore((s) => s.tabs.some((t) => t.kind === 'datagen'))

  // How wide the connection tree is, in pixels; 0 means folded away. Size is
  // controlled rather than defaulted because the rail has to be able to fold the
  // tree without a remount — the tree holds its own search text and open nodes.
  // Whatever the splitter does, the user dragging the bar or its own arrow, comes
  // back through `onResize`, so the two never disagree.
  const [sidebarWidth, setSidebarWidth] = useState(SIDEBAR_WIDTH)
  // What the tree was before it was folded. The splitter remembers only the width
  // *it* collapsed, which is not the same number: a tree folded from the rail is
  // handed back by the splitter's arrow at its minimum instead (see the README).
  const lastWidth = useRef(SIDEBAR_WIDTH)
  const treeOpen = sidebarWidth > 0

  const dataGenDriver = findDriver(drivers, target?.driver)
  // A page that cannot be opened says why on the item itself, and only when it
  // has nothing to show: an existing window is always worth going back to.
  const dataGenReason = hasDataGenWindow
    ? undefined
    : !target
      ? 'Pick an open connection in the tree first'
      : !capabilitiesOf(dataGenDriver).insertable
        ? `${dataGenDriver?.displayName ?? 'This engine'} cannot insert rows`
        : undefined

  /**
   * The rail was asked for a page.
   *
   * Two of the four need more than a page switch. *Connections* is the page the
   * explorer belongs to, so it is also the explorer's own door — the one thing
   * that cannot be folded away — and asking for it while it is already showing
   * is how the tree is folded and unfolded. *Data generation* is a page of
   * windows, and opening a window is what the command means when there is none.
   */
  const selectPage = (next: AppPage) => {
    if (next === 'connections') {
      setPage('connections')
      if (page === 'connections' && treeOpen) {
        lastWidth.current = sidebarWidth
        setSidebarWidth(0)
        return
      }
      setSidebarWidth(lastWidth.current > 0 ? lastWidth.current : SIDEBAR_WIDTH)
      return
    }
    if (next === 'datagen' && !hasDataGenWindow) {
      if (!target || dataGenReason) return
      // The window opens standing on the database the explorer is in, when that
      // is the connection it is for: the window picks its own table, and the
      // namespace is only a starting point.
      const scope = activeNamespace?.sessionId === target.id ? activeNamespace : undefined
      openDataGenTab(target.id, scope?.database, scope?.schema)
      return
    }
    setPage(next)
  }

  return (
    <ConnectProvider>
      <div className="app-shell">
        <div className="app-chrome">
          <MainToolbar />
        </div>
        <div className="app-body">
          <ActivityBar
            page={page}
            onSelect={selectPage}
            hints={
              page === 'connections' && treeOpen
                ? { connections: 'Hide the connection tree' }
                : undefined
            }
            disabledReason={dataGenReason ? { datagen: dataGenReason } : undefined}
          />
          <div className="app-main">
            <PageLayer active={page === 'connections'}>
              <Splitter
                style={{ height: '100%' }}
                onResize={(sizes) => {
                  const width = Math.round(sizes[0] ?? 0)
                  setSidebarWidth(width)
                  if (width > 0) lastWidth.current = width
                }}
              >
                <Splitter.Panel
                  size={sidebarWidth}
                  min={220}
                  max={640}
                  collapsible={{ start: true, end: true, showCollapsibleIcon: 'auto' }}
                >
                  <ConnectionSidebar />
                </Splitter.Panel>
                <Splitter.Panel min={420}>
                  <Workspace />
                </Splitter.Panel>
              </Splitter>
            </PageLayer>
            <PageLayer active={page === 'datagen'}>
              <DataGenWorkspace />
            </PageLayer>
            <PageLayer active={page === 'changelog'}>
              <ChangeLogPane />
            </PageLayer>
            <PageLayer active={page === 'settings'}>
              <SettingsPane />
            </PageLayer>
          </div>
        </div>
        <StatusBar />
        <ConnectionDialog />
      </div>
    </ConnectProvider>
  )
}

/**
 * One page of the main area.
 *
 * A page is mounted the first time it is shown and then kept, hidden: a
 * generation window holds the mock templates being written for one connection and
 * the change log holds what the user was reading, and a page that was rebuilt on
 * every visit would throw both away. Hidden is not unmounted, which is why the
 * pages that read from the data directory re-read when they come back to the
 * front — they ask the store which page is showing (see `ChangeLogPane`).
 */
function PageLayer({ active, children }: { active: boolean; children: ReactNode }) {
  const [seen, setSeen] = useState(active)
  useEffect(() => {
    if (active) setSeen(true)
  }, [active])
  if (!seen) return null
  return (
    <div className="app-layer" hidden={!active}>
      {children}
    </div>
  )
}
