import { useState, type ReactNode } from 'react'
import { Tooltip } from 'antd'
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

/**
 * Tooltip for the buttons that are temporarily parked.
 *
 * The ribbon keeps its shape so the layout still reads like the main window,
 * but only the commands that act on the connection the explorer is focused on
 * are live: creating one, opening the picked profile, closing the picked
 * session, and refreshing its catalog. The handlers are left wired so putting a
 * group back in service is a one-line change.
 */
const PARKED = 'Temporarily unavailable'

/**
 * Navicat-style ribbon: flat, grouped, icon-over-label buttons.
 *
 * The button set mirrors Navicat's main window one-for-one so the layout reads
 * the same, but only *Connection*, *Open*, *Close* and *Refresh* are live.
 * Everything else is disabled and says why — a dead button is worse than an
 * honest gap, but an empty toolbar is not the layout we are after.
 */
export function MainToolbar() {
  const connections = useAppStore((s) => s.connections)
  const sessions = useAppStore((s) => s.sessions)
  const activeSessionId = useAppStore((s) => s.activeSessionId)
  const activeConnectionId = useAppStore((s) => s.activeConnectionId)
  const openQueryTab = useAppStore((s) => s.openQueryTab)
  const closeSession = useAppStore((s) => s.closeSession)
  const invalidateSession = useAppStore((s) => s.invalidateSession)
  const loadDatabases = useAppStore((s) => s.loadDatabases)

  const { connect, pending } = useConnect()
  const [refreshing, setRefreshing] = useState(false)

  const activeSession = sessions.find((s) => s.id === activeSessionId)
  // What the ribbon acts on: the connection the explorer is focused on. It is
  // either a profile nobody has opened yet (Open lights up) or one with a live
  // session behind it (Close and Refresh do).
  const focusedProfile = connections.find((c) => c.id === activeConnectionId)
  // Matches both kinds of connection node: a saved profile, and a session that
  // stands in for one of its own (ad-hoc, or its profile was deleted).
  const focusedSession = sessions.find(
    (s) => s.id === activeConnectionId || s.connectionId === activeConnectionId,
  )

  const refresh = async () => {
    if (!focusedSession) return
    setRefreshing(true)
    try {
      invalidateSession(focusedSession.id)
      await loadDatabases(focusedSession.id)
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
      <RibbonButton
        icon={<ThunderboltOutlined />}
        label="Open"
        // Lights up when the tree has a profile picked that is not open yet —
        // the mirror image of Close below, and the same one-click connection the
        // explorer's right-click "Open connection" does (password prompt and
        // all, since both go through the same connect flow).
        hint={
          focusedProfile
            ? focusedSession
              ? `${focusedProfile.name} is already open`
              : `Open ${focusedProfile.name}`
            : 'Pick a connection in the tree first'
        }
        disabled={!focusedProfile || Boolean(focusedSession)}
        loading={pending !== null}
        onClick={() => focusedProfile && void connect(focusedProfile)}
      />
      <RibbonButton
        icon={<DisconnectOutlined />}
        label="Close"
        hint={
          focusedSession
            ? `Close ${focusedSession.name}`
            : 'Pick an open connection in the tree first'
        }
        disabled={!focusedSession}
        onClick={() => focusedSession && void closeSession(focusedSession.id)}
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
        hint={
          focusedSession
            ? `Reload the catalog of ${focusedSession.name}`
            : 'Pick an open connection in the tree first'
        }
        disabled={!focusedSession}
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
