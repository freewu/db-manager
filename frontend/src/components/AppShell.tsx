import { Splitter } from 'antd'

import { ConnectProvider } from '../hooks/useConnect'
import { ConnectionDialog } from './ConnectionDialog'
import { ConnectionSidebar } from './ConnectionSidebar'
import { MainToolbar } from './MainToolbar'
import { MenuBar } from './MenuBar'
import { StatusBar } from './StatusBar'
import { Workspace } from './Workspace'

/**
 * Application chrome, Navicat style: menu bar, command ribbon, explorer on the
 * left, tabbed workspace on the right and a status bar pinned to the bottom.
 */
export function AppShell() {
  return (
    <ConnectProvider>
      <div className="app-shell">
        <div className="app-chrome">
          <MenuBar />
          <MainToolbar />
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
      </div>
    </ConnectProvider>
  )
}
