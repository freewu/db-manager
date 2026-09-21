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
  ReloadOutlined,
  SwapOutlined,
  SyncOutlined,
  TableOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'

import { ConnectionTypeDropdown } from './ConnectionTypeMenu'
import { useAppStore } from '../store/appStore'
import { useConnect } from '../hooks/useConnect'
import { driverIconOrLogo } from '../lib/assets'

/**
 * Tooltip for the buttons that are temporarily parked.
 *
 * The ribbon keeps its shape so the layout still reads like the main window,
 * but only the two commands that always make sense are live: creating or
 * opening a connection, and refreshing the catalog of the active one. The
 * handlers are left wired so putting a group back in service is a one-line
 * change.
 */
const PARKED = 'Temporarily unavailable'

/**
 * Navicat-style ribbon: flat, grouped, icon-over-label buttons.
 *
 * The button set mirrors Navicat's main window one-for-one so the layout reads
 * the same, but only *Connection* and *Refresh* are live. Everything else is
 * disabled and says why — a dead button is worse than an honest gap, but an
 * empty toolbar is not the layout we are after.
 */
export function MainToolbar() {
  const connections = useAppStore((s) => s.connections)
  const sessions = useAppStore((s) => s.sessions)
  const activeSessionId = useAppStore((s) => s.activeSessionId)
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
      <ConnectionTypeDropdown>
        <span className="dm-ribbon-dropdown">
          <RibbonButton
            icon={<ApiOutlined />}
            label="Connection"
            hint="Create a new connection profile"
          />
        </span>
      </ConnectionTypeDropdown>
      <Dropdown menu={connectMenu} trigger={['click']} placement="bottomLeft" disabled>
        <span className="dm-ribbon-dropdown">
          <RibbonButton
            icon={<ThunderboltOutlined />}
            label="Open"
            hint={PARKED}
            disabled
            loading={pending !== null}
          />
        </span>
      </Dropdown>
      <RibbonButton
        icon={<DisconnectOutlined />}
        label="Close"
        hint={PARKED}
        disabled
        onClick={() => activeSession && void closeSession(activeSession.id)}
      />

      <span className="dm-ribbon-sep" />

      <RibbonButton
        icon={<CodeOutlined />}
        label="New Query"
        hint={PARKED}
        disabled
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

      <RibbonButton icon={<TableOutlined />} label="Table" hint={PARKED} disabled />
      <RibbonButton icon={<EyeOutlined />} label="View" hint={PARKED} disabled />

      <span className="dm-ribbon-sep" />

      <RibbonButton icon={<CloudUploadOutlined />} label="Backup" hint={PARKED} disabled />
      <RibbonButton icon={<ClockCircleOutlined />} label="Auto Run" hint={PARKED} disabled />
      <RibbonButton icon={<SwapOutlined />} label="Transfer" hint={PARKED} disabled />
      <RibbonButton icon={<SyncOutlined />} label="Data Sync" hint={PARKED} disabled />
      <RibbonButton icon={<DiffOutlined />} label="Structure Sync" hint={PARKED} disabled />
      <RibbonButton icon={<FileTextOutlined />} label="Report" hint={PARKED} disabled />
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
