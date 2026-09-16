import { useMemo } from 'react'
import { Button, Space, Tabs, Tag, Tooltip, Typography } from 'antd'
import type { TabsProps } from 'antd'
import { CodeOutlined, PlusOutlined, TableOutlined } from '@ant-design/icons'

import { useAppStore, type WorkspaceTab } from '../store/appStore'
import { QueryPane } from './QueryPane'
import { TablePane } from './TablePane'
import { WelcomePane } from './WelcomePane'

/** Tab bar + pane host for the whole workspace. */
export function Workspace() {
  const tabs = useAppStore((s) => s.tabs)
  const activeTabId = useAppStore((s) => s.activeTabId)
  const sessions = useAppStore((s) => s.sessions)
  const setActiveTab = useAppStore((s) => s.setActiveTab)
  const closeTab = useAppStore((s) => s.closeTab)
  const openQueryTab = useAppStore((s) => s.openQueryTab)
  const activeSessionId = useAppStore((s) => s.activeSessionId)

  const items = useMemo<TabsProps['items']>(
    () =>
      tabs.map((tab) => ({
        key: tab.id,
        label: <TabLabel tab={tab} sessionName={sessions.find((s) => s.id === tab.sessionId)?.name} />,
        closable: true,
        children:
          tab.kind === 'query' ? <QueryPane tab={tab} /> : <TablePane tab={tab} />,
      })),
    [sessions, tabs],
  )

  if (tabs.length === 0) {
    return <WelcomePane />
  }

  return (
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
        tabBarExtraContent={{
          right: (
            <Space size={4} style={{ paddingInline: 8 }}>
              <Tooltip title="New query tab">
                <Button
                  size="small"
                  type="text"
                  icon={<PlusOutlined />}
                  disabled={!activeSessionId}
                  onClick={() => activeSessionId && openQueryTab(activeSessionId)}
                />
              </Tooltip>
            </Space>
          ),
        }}
        more={{ icon: <span style={{ fontSize: 12 }}>•••</span> }}
      />
    </div>
  )
}

function TabLabel({ tab, sessionName }: { tab: WorkspaceTab; sessionName?: string }) {
  return (
    <Space size={6} className="dm-tab-label">
      {tab.kind === 'query' ? (
        <CodeOutlined style={{ opacity: 0.7 }} />
      ) : (
        <TableOutlined style={{ opacity: 0.7 }} />
      )}
      <span>{tab.title}</span>
      {sessionName ? (
        <Typography.Text type="secondary" style={{ fontSize: 11 }}>
          {sessionName}
        </Typography.Text>
      ) : null}
      {tab.objectKind && tab.objectKind !== 'table' ? (
        <Tag style={{ marginInlineStart: 0, fontSize: 10, lineHeight: '14px' }}>{tab.objectKind}</Tag>
      ) : null}
    </Space>
  )
}
