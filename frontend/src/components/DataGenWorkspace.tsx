import { useMemo } from 'react'
import { Empty, Space, Tabs, Typography } from 'antd'
import type { TabsProps } from 'antd'
import { ExperimentOutlined } from '@ant-design/icons'

import { useAppStore, type WorkspaceTab } from '../store/appStore'
import { DataGenPane } from './DataGenPane'

/**
 * The data generation page: the windows that write rows.
 *
 * Generation windows are grouped by connection rather than by table — the window
 * has its own list of connections and tables on the left, and its mocks are
 * written per column, so one window per connection is what keeps them — and they
 * are the one kind of window that does not sit beside the explorer. The tree is
 * of no use while writing a mock for one column of one table, and the explorer
 * is where the *other* windows are opened from; so they get a page of their own,
 * which is also what keeps them out of the working area's tab strip.
 *
 * A page of its own means a front window of its own (`datagenTabId`), so that
 * walking over to the change log and back lands on the connection the user was
 * writing mocks in.
 */
export function DataGenWorkspace() {
  const allTabs = useAppStore((s) => s.tabs)
  const tabs = useMemo(() => allTabs.filter((tab) => tab.kind === 'datagen'), [allTabs])
  const activeTabId = useAppStore((s) => s.datagenTabId)
  const sessions = useAppStore((s) => s.sessions)
  const setActiveTab = useAppStore((s) => s.setActiveTab)
  const closeTab = useAppStore((s) => s.closeTab)

  const items = useMemo<TabsProps['items']>(
    () =>
      tabs.map((tab) => ({
        key: tab.id,
        label: <DataGenLabel tab={tab} sessionName={sessions.find((s) => s.id === tab.sessionId)?.name} />,
        closable: true,
        children: <DataGenPane tab={tab} />,
      })),
    [sessions, tabs],
  )

  return (
    <div className="dm-pane">
      <div className="dm-editor-toolbar">
        <ExperimentOutlined style={{ opacity: 0.7 }} />
        <Typography.Text strong>Data generation</Typography.Text>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {tabs.length === 1
            ? 'one open connection'
            : `${tabs.length} open connections`}
        </Typography.Text>
      </div>
      {tabs.length === 0 ? (
        // Only reachable if the last window was closed from somewhere else: the
        // rail opens one rather than showing this page empty, so the answer says
        // how to get one back instead of just being blank.
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          style={{ margin: 'auto' }}
          description={
            <Space direction="vertical" size={2}>
              <Typography.Text>No generation window is open</Typography.Text>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                Right-click a table in the explorer and pick “Data generation…”, or press the flask
                in the page rail.
              </Typography.Text>
            </Space>
          }
        />
      ) : (
        <div className="dm-tabs">
          <Tabs
            type="editable-card"
            hideAdd
            size="small"
            activeKey={activeTabId}
            items={items}
            onChange={setActiveTab}
            onEdit={(target, action) => {
              if (action === 'remove' && typeof target === 'string') closeTab(target)
            }}
            more={{ icon: <span style={{ fontSize: 12 }}>•••</span> }}
          />
        </div>
      )}
    </div>
  )
}

/** The tab label of a generation window: the connection it writes into. */
function DataGenLabel({ tab, sessionName }: { tab: WorkspaceTab; sessionName?: string }) {
  const place = [tab.database, tab.schema].filter(Boolean).join('.')
  return (
    <Space size={6} className="dm-tab-label">
      <ExperimentOutlined style={{ opacity: 0.7 }} />
      <span>{tab.title}</span>
      {sessionName ? (
        <Typography.Text type="secondary" style={{ fontSize: 11 }}>
          {place ? `${sessionName} · ${place}` : sessionName}
        </Typography.Text>
      ) : null}
    </Space>
  )
}
