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
 * Both are one-line flip-throughs rather than lists of what is on offer: the
 * strip is 26px of readouts, and three lit-up alternatives would be the only
 * thing there asking to be read. So each entry names its *current* value — the
 * theme being painted, the language being spoken — and clicking moves to the
 * next one. The tooltips carry the part a single word cannot: for the theme that
 * "follow the system" is why it says what it says, for the language that the
 * next click lands on 繁體中文.
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

  // Three languages, so the cycle wraps. The label stays the two-character form
  // the strip has room for; the tooltip spells the language out in full, since
  // `简` is not a language to anyone who cannot read it.
  const languageIndex = LANGUAGE_CHOICES.findIndex((choice) => choice.value === language)
  const nextLanguage = LANGUAGE_CHOICES[(languageIndex + 1) % LANGUAGE_CHOICES.length]
  const spoken = LANGUAGE_CHOICES[languageIndex]

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
      {/* The language entry is the theme entry's twin: the word shown is the
          language in use, written in that language, and a click moves to the
          next of the three. The tooltip names it in full, because `简` would not
          be a language to anyone who cannot read it. */}
      <Tooltip
        title={t('statusBar.interface-language-click-to-switch', {
          language: spoken.label,
          next: nextLanguage.label,
        })}
      >
        <span
          className="dm-statusbar-item"
          role="button"
          tabIndex={0}
          aria-label={t('statusBar.interface-language')}
          style={{ cursor: 'pointer' }}
          onClick={() => setUiLanguage(nextLanguage.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter' || event.key === ' ') {
              setUiLanguage(nextLanguage.value)
            }
          }}
        >
          {spoken.short}
        </span>
      </Tooltip>
    </div>
  )
}
