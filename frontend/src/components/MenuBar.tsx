import { App as AntApp, Dropdown } from 'antd'
import type { MenuProps } from 'antd'
import {
  CheckOutlined,
  CodeOutlined,
  DisconnectOutlined,
  FileTextOutlined,
  LinkOutlined,
  PlusOutlined,
  ReloadOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'

import { AboutProject } from './AboutProject'
import { useAppStore } from '../store/appStore'
import { useConnect } from '../hooks/useConnect'
import { PROJECT_URL, openExternal } from '../lib/about'
import { driverIconOrLogo } from '../lib/assets'

/**
 * Navicat-style menu bar: 文件 / 编辑 / 查看 / 收藏夹 / 工具 / 窗口 / 帮助.
 *
 * Entries map to real commands that already exist in the store. Commands that
 * need a backend feature we do not have yet are listed but disabled, so the
 * bar mirrors Navicat's structure without pretending to work.
 */

type MenuItem = NonNullable<MenuProps['items']>[number]

/** Wails injects `window.runtime`; the app must survive without it (browser dev). */
interface DesktopRuntime {
  Quit?: () => void
}

function desktopRuntime(): DesktopRuntime | undefined {
  return (window as unknown as { runtime?: DesktopRuntime }).runtime
}

/** Runs a document command against whatever editable element has focus. */
function execCommand(command: string) {
  try {
    document.execCommand(command)
  } catch {
    // Nothing focusable — silently ignore, like a real menu item would.
  }
}

export function MenuBar() {
  const { connect } = useConnect()
  const { modal } = AntApp.useApp()

  const connections = useAppStore((s) => s.connections)
  const sessions = useAppStore((s) => s.sessions)
  const activeSessionId = useAppStore((s) => s.activeSessionId)
  const tabs = useAppStore((s) => s.tabs)
  const activeTabId = useAppStore((s) => s.activeTabId)
  const theme = useAppStore((s) => s.theme)
  const appInfo = useAppStore((s) => s.appInfo)
  const openConnectionEditor = useAppStore((s) => s.openConnectionEditor)
  const closeSession = useAppStore((s) => s.closeSession)
  const setActiveSession = useAppStore((s) => s.setActiveSession)
  const setActiveTab = useAppStore((s) => s.setActiveTab)
  const closeTab = useAppStore((s) => s.closeTab)
  const closeAllTabs = useAppStore((s) => s.closeAllTabs)
  const openQueryTab = useAppStore((s) => s.openQueryTab)
  const openDdlTab = useAppStore((s) => s.openDdlTab)
  const invalidateSession = useAppStore((s) => s.invalidateSession)
  const loadDatabases = useAppStore((s) => s.loadDatabases)
  const setTheme = useAppStore((s) => s.setTheme)

  const activeSession = sessions.find((s) => s.id === activeSessionId)
  const openConnectionIds = new Set(sessions.map((s) => s.connectionId))
  const closedProfiles = connections.filter((c) => !openConnectionIds.has(c.id))
  const runtime = desktopRuntime()

  const refresh = async () => {
    if (!activeSessionId) return
    invalidateSession(activeSessionId)
    await loadDatabases(activeSessionId)
  }

  const reconnect = async () => {
    const profile = connections.find((c) => c.id === activeSession?.connectionId)
    if (!activeSession || !profile) return
    await closeSession(activeSession.id)
    await connect(profile)
  }

  const showAbout = () => {
    modal.info({
      title: `About ${appInfo?.name ?? 'DB Manager'}`,
      width: 520,
      okText: 'Close',
      content: (
        <div className="dm-about">
          <AboutProject />
        </div>
      ),
    })
  }

  const run = (key: string) => {
    if (key.startsWith('file.open.')) {
      const profile = connections.find((c) => c.id === key.slice('file.open.'.length))
      if (profile) void connect(profile)
      return
    }
    if (key.startsWith('fav.')) {
      const profile = connections.find((c) => c.id === key.slice('fav.'.length))
      if (!profile) return
      const session = sessions.find((s) => s.connectionId === profile.id)
      if (session) setActiveSession(session.id)
      else void connect(profile)
      return
    }
    if (key.startsWith('window.tab.')) {
      setActiveTab(key.slice('window.tab.'.length))
      return
    }
    switch (key) {
      case 'file.new':
        openConnectionEditor()
        break
      case 'file.close':
        if (activeSession) void closeSession(activeSession.id)
        break
      case 'file.query':
        if (activeSessionId) openQueryTab(activeSessionId, activeSession?.database)
        break
      case 'file.exit':
        runtime?.Quit?.()
        break
      case 'edit.undo':
        execCommand('undo')
        break
      case 'edit.redo':
        execCommand('redo')
        break
      case 'edit.cut':
        execCommand('cut')
        break
      case 'edit.copy':
        execCommand('copy')
        break
      case 'edit.paste':
        execCommand('paste')
        break
      case 'edit.selectAll':
        execCommand('selectAll')
        break
      case 'view.refresh':
        void refresh()
        break
      case 'view.reconnect':
        void reconnect()
        break
      case 'view.light':
        setTheme('light')
        break
      case 'view.dark':
        setTheme('dark')
        break
      case 'tools.query':
        if (activeSessionId) openQueryTab(activeSessionId, activeSession?.database)
        break
      case 'tools.ddl':
        if (activeSessionId) {
          openDdlTab(activeSessionId, activeSession?.database ?? '', '')
        }
        break
      case 'tools.refresh':
        void refresh()
        break
      case 'window.close':
        if (activeTabId) closeTab(activeTabId)
        break
      case 'window.closeAll':
        closeAllTabs()
        break
      case 'help.about':
        showAbout()
        break
      case 'help.project':
        openExternal(PROJECT_URL)
        break
      default:
        break
    }
  }

  const later = (label: string): MenuItem => ({
    key: `later.${label}`,
    label,
    disabled: true,
  })

  const menus: { key: string; label: string; items: MenuItem[] }[] = [
    {
      key: 'file',
      label: 'File',
      items: [
        { key: 'file.new', label: 'New Connection…', icon: <PlusOutlined /> },
        {
          key: 'file.open',
          label: 'Open Connection',
          icon: <ThunderboltOutlined />,
          children: closedProfiles.length
            ? closedProfiles.map((profile) => ({
                key: `file.open.${profile.id}`,
                label: profile.name,
                icon: <img src={driverIconOrLogo(profile.driver)} alt="" className="dm-menu-icon" />,
              }))
            : [{ key: 'file.open.none', label: 'No closed connections', disabled: true }],
        },
        {
          key: 'file.close',
          label: 'Close Connection',
          icon: <DisconnectOutlined />,
          disabled: !activeSession,
        },
        { type: 'divider' },
        {
          key: 'file.query',
          label: 'New Query',
          icon: <CodeOutlined />,
          disabled: !activeSessionId,
        },
        { type: 'divider' },
        { key: 'file.exit', label: 'Exit', disabled: !runtime?.Quit },
      ],
    },
    {
      key: 'edit',
      label: 'Edit',
      items: [
        { key: 'edit.undo', label: 'Undo' },
        { key: 'edit.redo', label: 'Redo' },
        { type: 'divider' },
        { key: 'edit.cut', label: 'Cut' },
        { key: 'edit.copy', label: 'Copy' },
        { key: 'edit.paste', label: 'Paste' },
        { type: 'divider' },
        { key: 'edit.selectAll', label: 'Select All' },
      ],
    },
    {
      key: 'view',
      label: 'View',
      items: [
        {
          key: 'view.refresh',
          label: 'Refresh',
          icon: <ReloadOutlined />,
          disabled: !activeSessionId,
        },
        {
          key: 'view.reconnect',
          label: 'Reconnect',
          icon: <ThunderboltOutlined />,
          disabled: !activeSession,
        },
        { type: 'divider' },
        {
          key: 'view.light',
          label: 'Light Theme',
          icon: theme === 'light' ? <CheckOutlined /> : undefined,
        },
        {
          key: 'view.dark',
          label: 'Dark Theme',
          icon: theme === 'dark' ? <CheckOutlined /> : undefined,
        },
      ],
    },
    {
      key: 'favorites',
      label: 'Favorites',
      items: connections.length
        ? connections.map((profile) => ({
            key: `fav.${profile.id}`,
            label: profile.name,
            icon: openConnectionIds.has(profile.id) ? (
              <CheckOutlined />
            ) : (
              <img src={driverIconOrLogo(profile.driver)} alt="" className="dm-menu-icon" />
            ),
          }))
        : [{ key: 'fav.none', label: 'No saved connections', disabled: true }],
    },
    {
      key: 'tools',
      label: 'Tools',
      items: [
        {
          key: 'tools.query',
          label: 'New Query',
          icon: <CodeOutlined />,
          disabled: !activeSessionId,
        },
        {
          key: 'tools.ddl',
          label: 'New DDL Script',
          icon: <FileTextOutlined />,
          disabled: !activeSessionId,
        },
        {
          key: 'tools.refresh',
          label: 'Refresh Catalog',
          icon: <ReloadOutlined />,
          disabled: !activeSessionId,
        },
        { type: 'divider' },
        later('Data Transfer…'),
        later('Data Synchronization…'),
        later('Structure Synchronization…'),
        later('Scheduler…'),
        later('Console…'),
      ],
    },
    {
      key: 'window',
      label: 'Window',
      items: [
        ...(tabs.length
          ? tabs.map((tab) => ({
              key: `window.tab.${tab.id}`,
              label: tab.title,
              icon: tab.id === activeTabId ? <CheckOutlined /> : undefined,
            }))
          : [{ key: 'window.none', label: 'No open windows', disabled: true }]),
        { type: 'divider' },
        { key: 'window.close', label: 'Close Window', disabled: !activeTabId },
        { key: 'window.closeAll', label: 'Close All Windows', disabled: !tabs.length },
      ],
    },
    {
      key: 'help',
      label: 'Help',
      items: [
        { key: 'help.about', label: 'About DB Manager' },
        { key: 'help.project', label: 'Project Homepage', icon: <LinkOutlined /> },
        { type: 'divider' },
        later('Keyboard Shortcuts'),
        later('Documentation'),
      ],
    },
  ]

  return (
    <nav className="dm-menubar" aria-label="Main menu">
      {menus.map((menu) => (
        <Dropdown
          key={menu.key}
          menu={{ items: menu.items, onClick: ({ key }) => run(key) }}
          trigger={['click']}
          placement="bottomLeft"
        >
          <button type="button" className="dm-menubar-item">
            {menu.label}
          </button>
        </Dropdown>
      ))}
    </nav>
  )
}
