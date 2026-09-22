import { useMemo } from 'react'
import { App as AntApp, Button, Space, Tabs, Tag, Tooltip, Typography } from 'antd'
import type { TabsProps } from 'antd'
import { CodeOutlined, DashboardOutlined, FileTextOutlined, FolderOutlined, PartitionOutlined, PlusOutlined, TableOutlined } from '@ant-design/icons'

import { useAppStore, type WorkspaceTab } from '../store/appStore'
import { DdlPane } from './DdlPane'
import { ErDiagramPane } from './ErDiagramPane'
import { NewTablePane } from './NewTablePane'
import { ObjectListPane } from './ObjectListPane'
import { QueryPane } from './QueryPane'
import { RuntimePane } from './RuntimePane'
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
  const { modal } = AntApp.useApp()

  /**
   * Closing a window, asking first when it holds edits that are not in its file.
   *
   * Only a saved script can be dirty: a scratchpad tab is not written anywhere,
   * so there is nothing to lose except the text on screen, which the user can
   * see is about to go.
   */
  const requestClose = (id: string) => {
    const tab = tabs.find((t) => t.id === id)
    if (!tab?.dirty || !tab.queryFile) {
      closeTab(id)
      return
    }
    modal.confirm({
      title: `Close “${tab.queryFile.name}” without saving?`,
      content:
        'The window has edits that were never written to its file. Closing it drops them; there is no copy of them anywhere else.',
      okText: 'Close without saving',
      okButtonProps: { danger: true },
      onOk: () => closeTab(id),
    })
  }

  const items = useMemo<TabsProps['items']>(
    () =>
      tabs.map((tab) => ({
        key: tab.id,
        label: <TabLabel tab={tab} sessionName={sessions.find((s) => s.id === tab.sessionId)?.name} />,
        closable: true,
        children:
          tab.kind === 'query' ? (
            <QueryPane tab={tab} />
          ) : tab.kind === 'newtable' ? (
            <NewTablePane tab={tab} />
          ) : tab.kind === 'objects' ? (
            <ObjectListPane tab={tab} />
          ) : tab.kind === 'ddl' ? (
            <DdlPane tab={tab} />
          ) : tab.kind === 'er' ? (
            <ErDiagramPane tab={tab} />
          ) : tab.kind === 'runtime' ? (
            <RuntimePane tab={tab} />
          ) : (
            <TablePane tab={tab} />
          ),
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
          if (action === 'remove' && typeof target === 'string') requestClose(target)
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
  // A new table has no catalog name yet, so the tab follows the name being
  // typed in the designer instead of staying "New table" while it is written.
  const draftName = useAppStore((s) => (tab.kind === 'newtable' ? s.designs[tab.id]?.draft.object : ''))
  const title = draftName?.trim() ? draftName.trim() : tab.title
  return (
    <Space size={6} className="dm-tab-label">
      {tab.kind === 'query' ? (
        // A saved script is a file, so it gets the file icon; a scratchpad stays
        // the code glyph. The two read differently at a glance, which matters
        // when only one of them is written anywhere.
        tab.queryFile ? (
          <FileTextOutlined style={{ opacity: 0.7 }} />
        ) : (
          <CodeOutlined style={{ opacity: 0.7 }} />
        )
      ) : tab.kind === 'newtable' ? (
        <PlusOutlined style={{ opacity: 0.7 }} />
      ) : tab.kind === 'objects' ? (
        <FolderOutlined style={{ opacity: 0.7 }} />
      ) : tab.kind === 'ddl' ? (
        <FileTextOutlined style={{ opacity: 0.7 }} />
      ) : tab.kind === 'er' ? (
        <PartitionOutlined style={{ opacity: 0.7 }} />
      ) : tab.kind === 'runtime' ? (
        <DashboardOutlined style={{ opacity: 0.7 }} />
      ) : (
        <TableOutlined style={{ opacity: 0.7 }} />
      )}
      <span>
        {title}
        {tab.dirty ? ' *' : ''}
      </span>
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
