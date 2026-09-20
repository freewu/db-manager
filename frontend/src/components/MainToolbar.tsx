import { useState, type ReactNode } from 'react'
import { Dropdown, Tooltip } from 'antd'
import type { MenuProps } from 'antd'
import {
  ApiOutlined,
  ClockCircleOutlined,
  CloudUploadOutlined,
  CodeOutlined,
  DiffOutlined,
  DisconnectOutlined,
  EyeOutlined,
  FileTextOutlined,
  FunctionOutlined,
  ReloadOutlined,
  SwapOutlined,
  SyncOutlined,
  TableOutlined,
  ThunderboltOutlined,
  UserOutlined,
} from '@ant-design/icons'

import { useAppStore } from '../store/appStore'
import { useConnect } from '../hooks/useConnect'
import { driverIconOrLogo } from '../lib/assets'

/** Tooltip for commands whose backend is not written yet. */
const SOON = 'Not available yet — the editor behind this command is still to come'

/**
 * Navicat-style ribbon: flat, grouped, icon-over-label buttons.
 *
 * The button set mirrors Navicat's main window one-for-one so the layout reads
 * the same. Commands we cannot honour yet stay visible but disabled and carry
 * a tooltip explaining why — a dead button is worse than an honest gap, but an
 * empty toolbar is not the layout we are after.
 */
export function MainToolbar() {
  const connections = useAppStore((s) => s.connections)
  const sessions = useAppStore((s) => s.sessions)
  const activeSessionId = useAppStore((s) => s.activeSessionId)
  const openPicker = useAppStore((s) => s.openConnectionPicker)
  const openQueryTab = useAppStore((s) => s.openQueryTab)
  const closeSession = useAppStore((s) => s.closeSession)
  const invalidateSession = useAppStore((s) => s.invalidateSession)
  const loadDatabases = useAppStore((s) => s.loadDatabases)

  const { connect, pending } = useConnect()
  const [refreshing, setRefreshing] = useState(false)

  const activeSession = sessions.find((s) => s.id === activeSessionId)
  const openConnectionIds = new Set(sessions.map((s) => s.connectionId))
  const closedProfiles = connections.filter((c) => !openConnectionIds.has(c.id))

  const connectMenu: MenuProps = {
    items: closedProfiles.length
      ? closedProfiles.map((profile) => ({
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
        }))
      : [{ key: 'none', label: 'Every saved connection is open', disabled: true }],
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
    <div className="dm-ribbon">
      <RibbonButton
        icon={<ApiOutlined />}
        label="Connection"
        hint="Create a new connection profile"
        onClick={openPicker}
      />
      <Dropdown menu={connectMenu} trigger={['click']} placement="bottomLeft">
        <span className="dm-ribbon-dropdown">
          <RibbonButton
            icon={<ThunderboltOutlined />}
            label="Open"
            hint="Open a saved connection"
            loading={pending !== null}
          />
        </span>
      </Dropdown>
      <RibbonButton
        icon={<DisconnectOutlined />}
        label="Close"
        hint="Close the active connection"
        disabled={!activeSession}
        onClick={() => activeSession && void closeSession(activeSession.id)}
      />

      <span className="dm-ribbon-sep" />

      <RibbonButton
        icon={<CodeOutlined />}
        label="New Query"
        hint="Open a new SQL editor"
        disabled={!activeSession}
        onClick={() => activeSession && openQueryTab(activeSession.id, activeSession.database)}
      />
      <RibbonButton
        icon={<ReloadOutlined />}
        label="Refresh"
        hint="Reload the active connection's catalog"
        disabled={!activeSession}
        loading={refreshing}
        onClick={() => void refresh()}
      />

      <span className="dm-ribbon-sep" />

      <RibbonButton icon={<TableOutlined />} label="Table" hint={SOON} />
      <RibbonButton icon={<EyeOutlined />} label="View" hint={SOON} />
      <RibbonButton icon={<FunctionOutlined />} label="Function" hint={SOON} />
      <RibbonButton icon={<UserOutlined />} label="User" hint={SOON} />

      <span className="dm-ribbon-sep" />

      <RibbonButton icon={<CloudUploadOutlined />} label="Backup" hint={SOON} />
      <RibbonButton icon={<ClockCircleOutlined />} label="Auto Run" hint={SOON} />
      <RibbonButton icon={<SwapOutlined />} label="Transfer" hint={SOON} />
      <RibbonButton icon={<SyncOutlined />} label="Data Sync" hint={SOON} />
      <RibbonButton icon={<DiffOutlined />} label="Structure Sync" hint={SOON} />
      <RibbonButton icon={<FileTextOutlined />} label="Report" hint={SOON} />
    </div>
  )
}

function RibbonButton({
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
    <button
      type="button"
      className={`dm-ribbon-button${disabled ? ' is-disabled' : ''}`}
      disabled={disabled}
      onClick={onClick}
    >
      <span className={`dm-ribbon-glyph${loading ? ' is-loading' : ''}`}>{icon}</span>
      <span className="dm-ribbon-label">{label}</span>
    </button>
  )
  return hint ? (
    <Tooltip title={hint}>
      <span className="dm-ribbon-cell">{button}</span>
    </Tooltip>
  ) : (
    button
  )
}
