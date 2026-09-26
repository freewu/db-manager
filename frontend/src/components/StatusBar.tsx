import { Badge, Tooltip } from 'antd'
import {
  DatabaseOutlined,
  DisconnectOutlined,
  LockOutlined,
  TableOutlined,
} from '@ant-design/icons'

import { useAppStore } from '../store/appStore'
import { driverIcon } from '../lib/assets'
import { themeLabel, type ResolvedTheme } from '../lib/theme'
import { LANGUAGE_CHOICES, t, tn, useLanguage } from '../lib/i18n'

/**
 * Bottom status strip: active session details, global counters, and the two
 * preferences worth reaching without leaving the window — the theme and the
 * interface language.
 *
 * The theme entry is the two-way flip rather than the settings page's three-way
 * choice: it names the theme that is painted, and "follow the system" is not a
 * third colour to show but a reason for the one being shown, so it belongs in
 * the tooltip instead of in the label. Clicking therefore pins the other theme —
 * including when the stored preference is `system`, where the click is what ends
 * the following.
 */
export function StatusBar() {
  const sessions = useAppStore((s) => s.sessions)
  const activeSessionId = useAppStore((s) => s.activeSessionId)
  const tabs = useAppStore((s) => s.tabs)
  const connections = useAppStore((s) => s.connections)
  const drivers = useAppStore((s) => s.drivers)
  const appInfo = useAppStore((s) => s.appInfo)
  const theme = useAppStore((s) => s.theme)
  const resolvedTheme = useAppStore((s) => s.resolvedTheme)
  const setTheme = useAppStore((s) => s.setTheme)
  const setUiLanguage = useAppStore((s) => s.setUiLanguage)
  const language = useLanguage()

  const session = sessions.find((s) => s.id === activeSessionId)
  const driver = session ? drivers.find((d) => d.type === session.driver) : undefined
  const driverLogo = driverIcon(driver?.type)
  const profile = session?.connectionId
    ? connections.find((c) => c.id === session.connectionId)
    : undefined

  // What a click pins: the opposite of what is on screen, which is also the
  // opposite of what the label says. Reading the *resolved* theme here is what
  // makes a click mean something while the preference is following the system.
  const nextTheme: ResolvedTheme = resolvedTheme === 'dark' ? 'light' : 'dark'
  const toggleTheme = () => setTheme(nextTheme)

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
            <Tooltip title={t('statusBar.write-statements-are-rejected-for-this-session')}>
              <span className="dm-statusbar-item">
                <LockOutlined /> {t('statusBar.read-only')}
              </span>
            </Tooltip>
          ) : null}
          {profile?.color ? (
            <span
              className="dm-statusbar-item"
              title={t('statusBar.profile-colour')}
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
          <DisconnectOutlined /> {t('statusBar.no-active-connection')}
        </span>
      )}

      <span className="dm-spacer" />

      <span className="dm-statusbar-item">
        <TableOutlined /> {tn('statusBar.tab-count', tabs.length)}
      </span>
      <span className="dm-statusbar-item">
        {tn('statusBar.connection-count', sessions.length)}
      </span>
      <Tooltip
        title={t('statusBar.backend', {
          version: appInfo?.version ?? '?',
          goVersion: appInfo?.goVersion ?? '',
        })}
      >
        <span className="dm-statusbar-item">v{appInfo?.version ?? '—'}</span>
      </Tooltip>
      {/* The label is the theme on screen, so the tooltip is where the
          preference itself gets to speak: "follow the system" is visible as a
          reason, not as a third state to cycle through. */}
      <Tooltip
        title={
          theme === 'system'
            ? t('statusBar.following-the-system-theme-click-to-switch', {
                resolvedTheme: themeLabel(resolvedTheme),
                next: themeLabel(nextTheme),
              })
            : t('statusBar.switch-between-light-and-dark')
        }
      >
        <span
          className="dm-statusbar-item"
          role="button"
          tabIndex={0}
          style={{ cursor: 'pointer' }}
          onClick={toggleTheme}
          onKeyDown={(event) => {
            if (event.key === 'Enter' || event.key === ' ') {
              toggleTheme()
            }
          }}
        >
          {themeLabel(resolvedTheme)}
        </span>
      </Tooltip>
      {/* The language switch sits next to the theme because the two are the same
          kind of thing — a preference about the window itself — and both are
          wanted without opening the settings page. Three one-word targets, the
          one in use in the accent colour; the full names live in the tooltips,
          each already written in the language it selects. */}
      <span
        className="dm-statusbar-item dm-statusbar-langs"
        role="group"
        aria-label={t('statusBar.interface-language')}
      >
        {LANGUAGE_CHOICES.map((choice) => (
          <button
            key={choice.value}
            type="button"
            className={`dm-statusbar-lang${choice.value === language ? ' is-active' : ''}`}
            title={choice.hint}
            aria-pressed={choice.value === language}
            onClick={() => setUiLanguage(choice.value)}
          >
            {choice.short}
          </button>
        ))}
      </span>
    </div>
  )
}
