import { useState } from 'react'
import { Splitter } from 'antd'

import { ConnectProvider } from '../hooks/useConnect'
import { ConnectionDialog } from './ConnectionDialog'
import { ConnectionSidebar } from './ConnectionSidebar'
import { MainToolbar } from './MainToolbar'
import { SettingsDialog } from './SettingsDialog'
import { StatusBar } from './StatusBar'
import { Workspace } from './Workspace'

/** How wide the connection tree opens, in pixels. */
const SIDEBAR_WIDTH = 310

/**
 * Application chrome, Navicat style: command ribbon, explorer on the left,
 * tabbed workspace on the right and a status bar pinned to the bottom.
 *
 * The settings dialog is owned here rather than by the toolbar: the ribbon is a
 * row of commands, and what a command opens belongs to the shell that holds it.
 * The width of the explorer is owned here for the same reason — folding it away
 * is a fact about this layout, and the ribbon only asks for it.
 */
export function AppShell() {
  const [settingsOpen, setSettingsOpen] = useState(false)
  // How wide the connection tree is, in pixels; 0 means folded away. Size is
  // controlled rather than defaulted because the ribbon has to be able to fold
  // the tree without a remount — the tree holds its own search text and open
  // nodes. Whatever the splitter does, the user dragging the bar or its own
  // arrow, comes back through `onResize`, so the two never disagree.
  const [sidebarWidth, setSidebarWidth] = useState(SIDEBAR_WIDTH)

  return (
    <ConnectProvider>
      <div className="app-shell">
        <div className="app-chrome">
          <MainToolbar
            onOpenSettings={() => setSettingsOpen(true)}
            onCollapseSidebar={() => setSidebarWidth(0)}
          />
        </div>
        <div className="app-body">
          <Splitter
            style={{ height: '100%' }}
            onResize={(sizes) => setSidebarWidth(Math.round(sizes[0] ?? 0))}
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
        </div>
        <StatusBar />
        <ConnectionDialog />
        <SettingsDialog open={settingsOpen} onClose={() => setSettingsOpen(false)} />
      </div>
    </ConnectProvider>
  )
}
