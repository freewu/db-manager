import { useState } from 'react'
import { Splitter } from 'antd'

import { ConnectProvider } from '../hooks/useConnect'
import { ConnectionDialog } from './ConnectionDialog'
import { ConnectionSidebar } from './ConnectionSidebar'
import { MainToolbar } from './MainToolbar'
import { SettingsDialog } from './SettingsDialog'
import { StatusBar } from './StatusBar'
import { Workspace } from './Workspace'

/**
 * Application chrome, Navicat style: command ribbon, explorer on the left,
 * tabbed workspace on the right and a status bar pinned to the bottom.
 *
 * The settings dialog is owned here rather than by the toolbar: the ribbon is a
 * row of commands, and what a command opens belongs to the shell that holds it.
 */
export function AppShell() {
  const [settingsOpen, setSettingsOpen] = useState(false)

  return (
    <ConnectProvider>
      <div className="app-shell">
        <div className="app-chrome">
          <MainToolbar onOpenSettings={() => setSettingsOpen(true)} />
        </div>
        <div className="app-body">
          <Splitter style={{ height: '100%' }}>
            <Splitter.Panel
              defaultSize={310}
              min={220}
              max={640}
              collapsible={{ showCollapsibleIcon: 'auto' }}
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
