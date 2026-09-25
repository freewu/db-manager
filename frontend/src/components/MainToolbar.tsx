import { useState, type ReactNode } from 'react'
import { Tooltip } from 'antd'
import {
  ApiOutlined,
  ClockCircleOutlined,
  CloudUploadOutlined,
  CodeOutlined,
  DisconnectOutlined,
  EyeOutlined,
  ReloadOutlined,
  SwapOutlined,
  SyncOutlined,
  TableOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'

import { ConnectionTypeDropdown } from './ConnectionTypeMenu'
import { useAppStore } from '../store/appStore'
import { useConnect } from '../hooks/useConnect'
import { findDriver, objectKindsOf } from '../lib/capabilities'
import { FOLDER_LABEL } from '../lib/tree'
import { t } from '../lib/i18n'
import type { MessageKey } from '../lib/i18n'

/**
 * Tooltip for the buttons that are temporarily parked.
 *
 * The ribbon keeps its shape so the layout still reads like the main window,
 * but only the commands that act on what the explorer is focused on are live:
 * creating a connection, opening the picked profile, closing the picked
 * session, refreshing its catalog, opening a query window on the picked
 * database, and listing the objects of the picked namespace. The handlers are
 * left wired so putting a group back in service is a one-line change.
 */
const PARKED: MessageKey = 'mainToolbar.temporarily-unavailable'

/**
 * Navicat-style ribbon: flat, grouped, icon-over-label buttons.
 *
 * The button set mirrors Navicat's main window one-for-one so the layout reads
 * the same, but only *Connection*, *Open*, *Close*, *New Query*, *Refresh*,
 * *Table* and *View* are live. Everything else is disabled and says why — a dead
 * button is worse than an honest gap, but an empty toolbar is not the layout we
 * are after.
 *
 * The commands that stand outside the explorer — the connections it lists, the
 * generation windows, the change log, the settings — are not here: they are the
 * pages the rail names (see `AppPage`), and a command that switches page belongs
 * on the far left, next to the page it switches to. This ribbon stops at what
 * acts on the connection the explorer is standing on.
 */
export function MainToolbar() {
  const connections = useAppStore((s) => s.connections)
  const drivers = useAppStore((s) => s.drivers)
  const sessions = useAppStore((s) => s.sessions)
  const activeConnectionId = useAppStore((s) => s.activeConnectionId)
  const activeNamespace = useAppStore((s) => s.activeNamespace)
  const openQueryTab = useAppStore((s) => s.openQueryTab)
  const openObjectsTab = useAppStore((s) => s.openObjectsTab)
  const closeSession = useAppStore((s) => s.closeSession)
  const invalidateSession = useAppStore((s) => s.invalidateSession)
  const loadDatabases = useAppStore((s) => s.loadDatabases)

  const { connect, pending } = useConnect()
  const [refreshing, setRefreshing] = useState(false)

  // What the ribbon acts on: the connection the explorer is focused on. It is
  // either a profile nobody has opened yet (Open lights up) or one with a live
  // session behind it (Close and Refresh do).
  const focusedProfile = connections.find((c) => c.id === activeConnectionId)
  // Matches both kinds of connection node: a saved profile, and a session that
  // stands in for one of its own (ad-hoc, or its profile was deleted).
  const focusedSession = sessions.find(
    (s) => s.id === activeConnectionId || s.connectionId === activeConnectionId,
  )
  // The namespace the ribbon lists objects from. Its session has to still be
  // open — closing one drops the note — so this is also the backstop for a
  // click that lands before the explorer has reacted.
  const namespaceSession = sessions.find((s) => s.id === activeNamespace?.sessionId)
  const focusedNamespace = namespaceSession ? activeNamespace : undefined
  // Which object kinds the engine has is the backend's answer, not ours.
  const namespaceDriver = findDriver(drivers, namespaceSession?.driver)
  const namespaceKinds = objectKindsOf(namespaceDriver)
  const driverLabel = namespaceDriver?.displayName ?? t('mainToolbar.this-engine')

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

  /**
   * One object-list button: what it lists, or the reason it cannot.
   *
   * The explorer hands us the namespace it is focused on, which is not always a
   * list. A PostgreSQL database node names a database but no objects — they live
   * in its schemas — and an engine may not have the kind at all (a document
   * store has collections, not tables). Both are said in the tooltip instead of
   * opening a window that could only ever be empty.
   */
  const listButton = (
    kind: 'table' | 'view',
  ): { disabled: boolean; hint: string; onClick?: () => void } => {
    if (!focusedNamespace) {
      return { disabled: true, hint: t('mainToolbar.pick-a-database-in-the-tree-first') }
    }
    const { sessionId, database, schema } = focusedNamespace
    if (!schema) {
      return {
        disabled: true,
        hint: t('mainToolbar.keeps-its-objects-in-schemas-pick-one-of-s', { driverLabel, database }),
      }
    }
    if (!namespaceKinds.includes(kind)) {
      // Point at the folder this engine does have, so the tooltip leads
      // somewhere instead of just refusing.
      const instead = namespaceKinds[0] ? t(FOLDER_LABEL[namespaceKinds[0]]) : t('mainToolbar.the-explorer')
      // `table` and `view` are two words, not one with an `s` on the end: a
      // message that glued the plural on would be untranslatable.
      return {
        disabled: true,
        hint: t(kind === 'table' ? 'mainToolbar.has-no-tables' : 'mainToolbar.has-no-views', {
          driverLabel,
          instead,
        }),
      }
    }
    return {
      disabled: false,
      hint: t(kind === 'table' ? 'mainToolbar.list-every-table-in' : 'mainToolbar.list-every-view-in', {
        database,
      }),
      onClick: () => openObjectsTab(sessionId, database, schema, kind),
    }
  }

  const tableButton = listButton('table')
  const viewButton = listButton('view')

  return (
    <div className="dm-ribbon">
      <ConnectionTypeDropdown>
        <span className="dm-ribbon-dropdown">
          <RibbonButton
            icon={<ApiOutlined />}
            label={t('mainToolbar.connection')}
            hint={t('mainToolbar.create-a-new-connection-profile')}
          />
        </span>
      </ConnectionTypeDropdown>
      <RibbonButton
        icon={<ThunderboltOutlined />}
        label={t('mainToolbar.open')}
        // Lights up when the tree has a profile picked that is not open yet —
        // the mirror image of Close below, and the same one-click connection the
        // explorer's right-click "Open connection" does (password prompt and
        // all, since both go through the same connect flow).
        hint={
          focusedProfile
            ? focusedSession
              ? t('mainToolbar.is-already-open', { name: focusedProfile.name })
              : t('mainToolbar.open-with-name', { name: focusedProfile.name })
            : t('mainToolbar.pick-a-connection-in-the-tree-first')
        }
        disabled={!focusedProfile || Boolean(focusedSession)}
        loading={pending !== null}
        onClick={() => focusedProfile && void connect(focusedProfile)}
      />
      <RibbonButton
        icon={<DisconnectOutlined />}
        label={t('mainToolbar.close')}
        hint={
          focusedSession
            ? t('mainToolbar.close-with-name', { name: focusedSession.name })
            : t('mainToolbar.pick-an-open-connection-in-the-tree-first')
        }
        disabled={!focusedSession}
        onClick={() => focusedSession && void closeSession(focusedSession.id)}
      />

      <span className="dm-ribbon-sep" />

      <RibbonButton
        icon={<CodeOutlined />}
        label={t('mainToolbar.new-query')}
        // Lights up while the explorer is inside a database, the same way
        // *Table* and *View* do, and opens a window on that very database —
        // carrying the schema it was picked in, which is what the window
        // completes table names from.
        hint={
          focusedNamespace
            ? t('mainToolbar.new-query-in', { database: focusedNamespace.database })
            : t('mainToolbar.pick-a-database-in-the-tree-first')
        }
        disabled={!focusedNamespace}
        onClick={() =>
          focusedNamespace &&
          openQueryTab(
            focusedNamespace.sessionId,
            focusedNamespace.database,
            focusedNamespace.schema,
          )
        }
      />
      <RibbonButton
        icon={<ReloadOutlined />}
        label={t('mainToolbar.refresh')}
        hint={
          focusedSession
            ? t('mainToolbar.reload-the-catalog-of', { name: focusedSession.name })
            : t('mainToolbar.pick-an-open-connection-in-the-tree-first')
        }
        disabled={!focusedSession}
        loading={refreshing}
        onClick={() => void refresh()}
      />

      <span className="dm-ribbon-sep" />

      <RibbonButton
        icon={<TableOutlined />}
        label={t('mainToolbar.table')}
        // Lights up while the explorer is inside a namespace, and opens the
        // very list its Tables folder holds — the same window the folder
        // itself opens, so the tree and the ribbon never disagree.
        disabled={tableButton.disabled}
        hint={tableButton.hint}
        onClick={tableButton.onClick}
      />
      <RibbonButton
        icon={<EyeOutlined />}
        label={t('mainToolbar.view')}
        disabled={viewButton.disabled}
        hint={viewButton.hint}
        onClick={viewButton.onClick}
      />

      <span className="dm-ribbon-sep" />

      <RibbonButton icon={<CloudUploadOutlined />} label={t('mainToolbar.backup')} hint={t(PARKED)} disabled />
      <RibbonButton icon={<ClockCircleOutlined />} label={t('mainToolbar.auto-run')} hint={t(PARKED)} disabled />
      <RibbonButton icon={<SwapOutlined />} label={t('mainToolbar.transfer')} hint={t(PARKED)} disabled />
      <RibbonButton icon={<SyncOutlined />} label={t('mainToolbar.data-sync')} hint={t(PARKED)} disabled />
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
