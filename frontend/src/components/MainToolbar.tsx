import { useState, type ReactNode } from 'react'
import { Button, Dropdown, Tooltip } from 'antd'
import type { MenuProps } from 'antd'
import {
  BulbOutlined,
  CodeOutlined,
  DisconnectOutlined,
  PlusOutlined,
  ReloadOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'

import { useAppStore } from '../store/appStore'
import { useConnect } from '../hooks/useConnect'
import { appLogo, driverIconOrLogo } from '../lib/assets'

/**
 * Navicat-style ribbon: a flat row of grouped commands above the workspace.
 *
 * Only commands that map to something real are shown — no decorative menus —
 * so the toolbar stays honest about what the app can do today.
 */
export function MainToolbar() {
  const connections = useAppStore((s) => s.connections)
  const sessions = useAppStore((s) => s.sessions)
  const activeSessionId = useAppStore((s) => s.activeSessionId)
  const openEditor = useAppStore((s) => s.openConnectionEditor)
  const openQueryTab = useAppStore((s) => s.openQueryTab)
  const closeSession = useAppStore((s) => s.closeSession)
  const invalidateSession = useAppStore((s) => s.invalidateSession)
  const loadDatabases = useAppStore((s) => s.loadDatabases)
  const theme = useAppStore((s) => s.theme)
  const setTheme = useAppStore((s) => s.setTheme)
  const appInfo = useAppStore((s) => s.appInfo)

  const { connect, pending } = useConnect()
  const [refreshing, setRefreshing] = useState(false)

  const activeSession = sessions.find((s) => s.id === activeSessionId)
  const closedProfiles = connections.filter(
    (c) => !sessions.some((s) => s.connectionId === c.id),
  )

  const connectMenu: MenuProps = {
    items:
      closedProfiles.length === 0
        ? [{ key: 'none', label: 'Every saved connection is open', disabled: true }]
        : closedProfiles.map((profile) => ({
            key: profile.id,
            label: profile.name,
            icon: (
              <img
                src={driverIconOrLogo(profile.driver)}
                alt=""
                className="dm-menu-icon"
                draggable={false}
              />
            ),
            onClick: () => void connect(profile),
          })),
  }

  const refresh = async () => {
    if (!activeSessionId) return
    setRefreshing(true)
    try {
      invalidateSession(activeSessionId)
      await loadDatabases(activeSessionId)
    } finally {
      setRefreshing(false)
    }
  }

  return (
    <div className="dm-toolbar">
      <div className="dm-toolbar-brand" title={appInfo ? `v${appInfo.version}` : undefined}>
        <img src={appLogo} alt="" className="dm-brand-logo" draggable={false} />
        <span>{appInfo?.name ?? 'DB Manager'}</span>
      </div>

      <span className="dm-toolbar-sep" />

      <ToolbarButton
        icon={<PlusOutlined />}
        label="Connection"
        hint="Create a new connection profile"
        onClick={() => openEditor()}
      />
      <Dropdown menu={connectMenu} trigger={['click']} placement="bottomLeft">
        <span>
          <ToolbarButton
            icon={<ThunderboltOutlined />}
            label="Open"
            hint="Open a saved connection"
            loading={pending !== null}
          />
        </span>
      </Dropdown>
      <ToolbarButton
        icon={<DisconnectOutlined />}
        label="Close"
        hint="Close the active connection"
        disabled={!activeSession}
        onClick={() => activeSession && void closeSession(activeSession.id)}
      />

      <span className="dm-toolbar-sep" />

      <ToolbarButton
        icon={<CodeOutlined />}
        label="Query"
        hint="New query tab"
        disabled={!activeSession}
        onClick={() => activeSession && openQueryTab(activeSession.id, activeSession.database)}
      />
      <ToolbarButton
        icon={<ReloadOutlined />}
        label="Refresh"
        hint="Reload the active connection's catalog"
        disabled={!activeSession}
        loading={refreshing}
        onClick={() => void refresh()}
      />

      <div className="dm-toolbar-spacer" />

      <div className="dm-toolbar-status">
        {activeSession ? `${activeSession.name} · ${activeSession.driver}` : 'not connected'}
      </div>

      <Tooltip title={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}>
        <Button
          size="small"
          type="text"
          className="dm-toolbar-icon"
          icon={<BulbOutlined />}
          onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
        />
      </Tooltip>
    </div>
  )
}

function ToolbarButton({
  icon,
  label,
  hint,
  disabled,
  loading,
  onClick,
}: {
  icon: ReactNode
  label: string
  hint?: string
  disabled?: boolean
  loading?: boolean
  onClick?: () => void
}) {
  const button = (
    <Button
      size="small"
      type="text"
      className="dm-toolbar-button"
      icon={icon}
      disabled={disabled}
      loading={loading}
      onClick={onClick}
    >
      {label}
    </Button>
  )
  return hint ? <Tooltip title={hint}>{button}</Tooltip> : button
}
