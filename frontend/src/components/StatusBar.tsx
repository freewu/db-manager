import { Badge, Tooltip } from 'antd'
import {
  DatabaseOutlined,
  DisconnectOutlined,
  LockOutlined,
  TableOutlined,
} from '@ant-design/icons'

import { useAppStore } from '../store/appStore'
import { driverIcon } from '../lib/assets'

/** Bottom status strip: active session details and global counters. */
export function StatusBar() {
  const sessions = useAppStore((s) => s.sessions)
  const activeSessionId = useAppStore((s) => s.activeSessionId)
  const tabs = useAppStore((s) => s.tabs)
  const connections = useAppStore((s) => s.connections)
  const drivers = useAppStore((s) => s.drivers)
  const appInfo = useAppStore((s) => s.appInfo)
  const theme = useAppStore((s) => s.theme)
  const setTheme = useAppStore((s) => s.setTheme)

  const session = sessions.find((s) => s.id === activeSessionId)
  const driver = session ? drivers.find((d) => d.type === session.driver) : undefined
  const driverLogo = driverIcon(driver?.type)
  const profile = session?.connectionId
    ? connections.find((c) => c.id === session.connectionId)
    : undefined

  return (
    <div className="dm-statusbar">
      {session ? (
        <>
          <span className="dm-statusbar-item">
            <Badge status="processing" />
            {session.name}
          </span>
          {driver ? (
            <span className="dm-statusbar-item" title={driver.displayName}>
              {driverLogo ? (
                <img src={driverLogo} alt="" draggable={false} className="dm-driver-icon is-small" />
              ) : null}
              {driver.displayName}
            </span>
          ) : null}
          {session.serverVersion ? (
            <span className="dm-statusbar-item" title={session.serverVersion}>
              {session.serverVersion.slice(0, 48)}
            </span>
          ) : null}
          {session.database ? (
            <span className="dm-statusbar-item">
              <DatabaseOutlined /> {session.database}
            </span>
          ) : null}
          {session.readOnly ? (
            <Tooltip title="Write statements are rejected for this session">
              <span className="dm-statusbar-item">
                <LockOutlined /> read-only
              </span>
            </Tooltip>
          ) : null}
          {profile?.color ? (
            <span
              className="dm-statusbar-item"
              title="Profile colour"
              style={{
                width: 8,
                height: 8,
                borderRadius: 2,
                background: profile.color,
              }}
            />
          ) : null}
        </>
      ) : (
        <span className="dm-statusbar-item">
          <DisconnectOutlined /> no active connection
        </span>
      )}

      <span className="dm-spacer" />

      <span className="dm-statusbar-item">
        <TableOutlined /> {tabs.length} tab{tabs.length === 1 ? '' : 's'}
      </span>
      <span className="dm-statusbar-item">{sessions.length} connected</span>
      <Tooltip title={`Backend ${appInfo?.version ?? '?'} · ${appInfo?.goVersion ?? ''}`}>
        <span className="dm-statusbar-item">v{appInfo?.version ?? '—'}</span>
      </Tooltip>
      <Tooltip title="Switch theme">
        <span
          className="dm-statusbar-item"
          role="button"
          tabIndex={0}
          style={{ cursor: 'pointer' }}
          onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
          onKeyDown={(event) => {
            if (event.key === 'Enter' || event.key === ' ') {
              setTheme(theme === 'dark' ? 'light' : 'dark')
            }
          }}
        >
          {theme === 'dark' ? 'dark' : 'light'}
        </span>
      </Tooltip>
    </div>
  )
}
