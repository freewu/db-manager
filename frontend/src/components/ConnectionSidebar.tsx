import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { App as AntApp, Button, Dropdown, Empty, Input, Menu, Modal, Tooltip, Tree, Typography } from 'antd'
import type { MenuProps, TreeDataNode, TreeProps } from 'antd'
import {
  AppstoreOutlined,
  CodeOutlined,
  DatabaseOutlined,
  DeleteOutlined,
  DisconnectOutlined,
  EditOutlined,
  FolderOutlined,
  KeyOutlined,
  MinusSquareOutlined,
  NumberOutlined,
  PartitionOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
  TableOutlined,
  ThunderboltOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type { ConnectionConfig, DriverInfo, IndexEntry, ObjectInfo, SessionInfo } from '../api/types'
import { useConnect } from '../hooks/useConnect'
import { driverIconOrLogo } from '../lib/assets'
import { quoteIdent } from '../lib/format'
import { useAppStore, type ListScope, type TableView } from '../store/appStore'
import { objectIcon } from './objectIcon'
import {
  databaseKey,
  decodeNode,
  encodeNode,
  FOLDER_LABEL,
  FOLDER_ORDER,
  indexesKey,
  namespaceKey,
  objectsKey,
} from '../lib/tree'

/** One entry of the connection pane: a saved profile and/or live session. */
interface RootEntry {
  /** Stable id used as the tree key (profile id, or session id for ad-hoc). */
  id: string
  name: string
  profile?: ConnectionConfig
  session?: SessionInfo
  driver: DriverInfo | undefined
}

/**
 * Navicat-style connection pane.
 *
 * Every saved profile is listed; open ones expand into
 * `database → schemas → [Tables | Views | Indexes | …]`. Expanding a closed
 * profile connects it, mirroring Navicat's double-click-to-open behaviour.
 */
export function ConnectionSidebar() {
  const sessions = useAppStore((s) => s.sessions)
  const drivers = useAppStore((s) => s.drivers)
  const connections = useAppStore((s) => s.connections)
  const tree = useAppStore((s) => s.tree)
  const openEditor = useAppStore((s) => s.openConnectionEditor)
  const loadDatabases = useAppStore((s) => s.loadDatabases)
  const loadSchemas = useAppStore((s) => s.loadSchemas)
  const loadObjects = useAppStore((s) => s.loadObjects)
  const loadIndexes = useAppStore((s) => s.loadIndexes)
  const invalidateSession = useAppStore((s) => s.invalidateSession)
  const openTableTab = useAppStore((s) => s.openTableTab)
  const openObjectsTab = useAppStore((s) => s.openObjectsTab)
  const openQueryTab = useAppStore((s) => s.openQueryTab)
  const openDdlTab = useAppStore((s) => s.openDdlTab)
  const openErTab = useAppStore((s) => s.openErTab)
  const closeSession = useAppStore((s) => s.closeSession)
  const setActiveSession = useAppStore((s) => s.setActiveSession)

  const { connect, pending } = useConnect()
  const { modal } = AntApp.useApp()

  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([])
  const [selectedKeys, setSelectedKeys] = useState<React.Key[]>([])
  const [loadedKeys, setLoadedKeys] = useState<React.Key[]>([])
  const [filter, setFilter] = useState('')
  const [manageOpen, setManageOpen] = useState(false)
  /** Where the user asked for the empty-area context menu, if anywhere. */
  const [blankMenu, setBlankMenu] = useState<{ x: number; y: number } | null>(null)
  const blankMenuRef = useRef<HTMLDivElement | null>(null)
  /** Session that the "New database" dialog is creating a database on. */
  const [newDatabase, setNewDatabase] = useState<SessionInfo | null>(null)

  const driverOfType = useCallback(
    (type: DriverInfo['type'] | undefined) => drivers.find((d) => d.type === type),
    [drivers],
  )

  const sessionForConnection = useCallback(
    (connectionId: string): SessionInfo | undefined =>
      sessions.find((s) => s.connectionId === connectionId || s.id === connectionId),
    [sessions],
  )

  /**
   * True once a node actually has data behind it.
   *
   * rc-tree re-triggers `loadData` on *every* render for an expanded node that
   * is not listed in `loadedKeys`, so this must never be used to filter that
   * prop — a filtered key makes the tree reload, which re-renders, which
   * reloads… Only consult it while handling an expand, so a node whose load
   * failed or was cancelled gets one fresh attempt per expand.
   */
  const hasData = useCallback(
    (key: string): boolean => {
      const ref = decodeNode(key)
      if (!ref) return true
      switch (ref.t) {
        case 'connection':
          return Boolean(sessionForConnection(ref.connectionId))
        case 'db': {
          const session = sessions.find((s) => s.id === ref.sessionId)
          const ns = driverOfType(session?.driver)?.supportsSchema
            ? databaseKey(ref.sessionId, ref.database)
            : namespaceKey(ref.sessionId, ref.database, ref.database)
          return Boolean(tree.loaded[ns])
        }
        case 'schema':
          return Boolean(tree.loaded[namespaceKey(ref.sessionId, ref.database, ref.schema)])
        case 'indexFolder':
          return Boolean(tree.loaded[indexesKey(ref.sessionId, ref.database, ref.schema)])
        default:
          return true
      }
    },
    [driverOfType, sessionForConnection, sessions, tree.loaded],
  )

  /** Merges profiles and ad-hoc sessions into the pane's root list. */
  const roots = useMemo<RootEntry[]>(() => {
    const openByConnection = new Map<string, SessionInfo>()
    for (const session of sessions) {
      if (session.connectionId) openByConnection.set(session.connectionId, session)
    }

    const out: RootEntry[] = []
    for (const profile of connections) {
      const session = openByConnection.get(profile.id)
      openByConnection.delete(profile.id)
      out.push({
        id: profile.id,
        name: profile.name,
        profile,
        session,
        driver: driverOfType(profile.driver),
      })
    }
    // Ad-hoc sessions and sessions whose profile has been deleted still show up.
    for (const session of [...openByConnection.values(), ...sessions.filter((s) => !s.connectionId)]) {
      if (out.some((entry) => entry.id === session.id)) continue
      out.push({ id: session.id, name: session.name, session, driver: driverOfType(session.driver) })
    }
    return out
  }, [connections, driverOfType, sessions])

  const visibleRoots = useMemo(() => {
    const needle = filter.trim().toLowerCase()
    if (!needle) return roots
    return roots.filter((entry) => entry.name.toLowerCase().includes(needle))
  }, [filter, roots])

  const openObject = useCallback(
    (sessionId: string, database: string, schema: string, object: ObjectInfo, view?: TableView) => {
      setActiveSession(sessionId)
      openTableTab(sessionId, database, schema, object, view)
    },
    [openTableTab, setActiveSession],
  )

  /** Opens the Navicat-style object list window for one explorer folder. */
  const openList = useCallback(
    (sessionId: string, database: string, schema: string, list: ListScope) => {
      setActiveSession(sessionId)
      openObjectsTab(sessionId, database, schema, list)
    },
    [openObjectsTab, setActiveSession],
  )

  const refreshSession = useCallback(
    (sessionId: string) => {
      invalidateSession(sessionId)
      setLoadedKeys((keys) =>
        keys.filter((key) => {
          const ref = decodeNode(String(key))
          return !ref || !('sessionId' in ref) || ref.sessionId !== sessionId
        }),
      )
      void loadDatabases(sessionId)
    },
    [invalidateSession, loadDatabases],
  )

  /* ----------------------------------------------------------- tree build */

  const buildIndexFolder = useCallback(
    (sessionId: string, database: string, schema: string): TreeDataNode => {
      const key = indexesKey(sessionId, database, schema)
      const indexes = tree.indexes[key]
      const loading = tree.loading[key]
      const error = tree.errors[key]

      let children: TreeDataNode[] | undefined
      if (loading) children = [placeholderNode(key, 'Loading indexes…')]
      else if (error) {
        children = [
          errorNode(key, error, () => void loadIndexes(sessionId, database, schema)),
        ]
      } else if (indexes) {
        children = indexes.map((index) => ({
          key: encodeNode({
            t: 'index',
            sessionId,
            database,
            schema,
            index: index.name,
            table: index.table,
          }),
          isLeaf: true,
          icon: index.primary ? (
            <KeyOutlined style={{ color: '#d4a017' }} />
          ) : (
            <KeyOutlined style={{ opacity: 0.75 }} />
          ),
          title: (
            <NodeMenu items={indexMenuItems(sessionId, database, schema, index)}>
              <span title={indexTooltip(index)}>
                <span className="dm-index-name">{index.name}</span>
                <span className="dm-index-table">{index.table}</span>
              </span>
            </NodeMenu>
          ),
        }))
      }

      return {
        key: encodeNode({ t: 'indexFolder', sessionId, database, schema }),
        title: (
          <NodeMenu
            items={[
              {
                key: 'list',
                icon: <UnorderedListOutlined />,
                label: 'Open object list',
                onClick: () => openList(sessionId, database, schema, 'index'),
              },
              {
                key: 'refresh',
                icon: <ReloadOutlined />,
                label: 'Reload index list',
                onClick: () => void loadIndexes(sessionId, database, schema),
              },
            ]}
          >
            <span>{indexes ? `Indexes (${indexes.length})` : 'Indexes'}</span>
          </NodeMenu>
        ),
        icon: <KeyOutlined />,
        isLeaf: false,
        children,
      }
    },
    [loadIndexes, openList, tree.errors, tree.indexes, tree.loading],
  )
  const buildFolders = useCallback(
    (sessionId: string, database: string, schema: string): TreeDataNode[] => {
      const objects = tree.objects[objectsKey(sessionId, database, schema)]
      if (!objects) return []

      const groups = new Map<string, ObjectInfo[]>()
      for (const object of objects) {
        const list = groups.get(object.kind) ?? []
        list.push(object)
        groups.set(object.kind, list)
      }

      const folders: TreeDataNode[] = []
      for (const kind of FOLDER_ORDER) {
        const items = groups.get(kind)
        if (!items || items.length === 0) continue
        folders.push({
          key: encodeNode({ t: 'folder', sessionId, database, schema, kind }),
          title: (
            <NodeMenu
              items={[
                {
                  key: 'list',
                  icon: <UnorderedListOutlined />,
                  label: 'Open object list',
                  onClick: () => openList(sessionId, database, schema, kind),
                },
                {
                  key: 'query',
                  icon: <EditOutlined />,
                  label: 'New query',
                  onClick: () => openQueryTab(sessionId, database),
                },
                {
                  key: 'ddl',
                  icon: <CodeOutlined />,
                  label: 'New DDL script…',
                  onClick: () => openDdlTab(sessionId, database, schema),
                },
                { type: 'divider' as const },
                {
                  key: 'refresh',
                  icon: <ReloadOutlined />,
                  label: 'Reload objects',
                  onClick: () => void loadObjects(sessionId, database, schema),
                },
              ]}
            >
              <span>{`${FOLDER_LABEL[kind]} (${items.length})`}</span>
            </NodeMenu>
          ),
          icon: <FolderOutlined />,
          children: items.map((object) => ({
            key: encodeNode({
              t: 'object',
              sessionId,
              database,
              schema,
              object: object.name,
              kind: object.kind,
            }),
            isLeaf: true,
            icon: objectIcon(object.kind),
            title: (
              <NodeMenu
                items={[
                  {
                    key: 'data',
                    icon: <UnorderedListOutlined />,
                    label: 'Open data',
                    onClick: () => openObject(sessionId, database, schema, object, 'data'),
                  },
                  {
                    key: 'structure',
                    icon: <AppstoreOutlined />,
                    label: 'Design object',
                    onClick: () => openObject(sessionId, database, schema, object, 'structure'),
                  },
                  {
                    key: 'query',
                    icon: <EditOutlined />,
                    label: 'New query',
                    onClick: () => openQueryTab(sessionId, database),
                  },
                  {
                    key: 'ddl',
                    icon: <CodeOutlined />,
                    label: 'Edit DDL…',
                    onClick: () => openDdlTab(sessionId, database, schema, object.name),
                  },
                  { type: 'divider' as const },
                  {
                    key: 'copy',
                    icon: <NumberOutlined />,
                    label: 'Copy name',
                    onClick: () => void copyText(object.name),
                  },
                ]}
              >
                <span title={objectTooltip(object)}>{object.name}</span>
              </NodeMenu>
            ),
          })),
        })
      }

      // Indexes live in their own namespace-wide folder, the way Navicat's
      // object list groups them next to tables and views.
      folders.push(buildIndexFolder(sessionId, database, schema))
      return folders
    },
    [buildIndexFolder, loadObjects, openDdlTab, openList, openObject, openQueryTab, tree.objects],
  )

  const buildNamespace = useCallback(
    (sessionId: string, database: string, schema: string): TreeDataNode[] => {
      const ns = namespaceKey(sessionId, database, schema)
      if (tree.loading[ns]) return [placeholderNode(ns, 'Loading objects…')]
      const error = tree.errors[ns]
      if (error) {
        return [errorNode(ns, error, () => void loadObjects(sessionId, database, schema))]
      }
      return buildFolders(sessionId, database, schema)
    },
    [buildFolders, loadObjects, tree.errors, tree.loading],
  )

  const buildDatabaseNode = useCallback(
    (session: SessionInfo, database: string): TreeDataNode => {
      const driver = driverOfType(session.driver)

      if (driver?.supportsSchema) {
        const key = databaseKey(session.id, database)
        const schemas = tree.schemas[key]
        let children: TreeDataNode[] | undefined
        if (tree.loading[key]) children = [placeholderNode(key, 'Loading schemas…')]
        else if (tree.errors[key]) {
          children = [
            errorNode(key, tree.errors[key], () => void loadSchemas(session.id, database)),
          ]
        } else if (schemas) {
          children = schemas.map((schema) => ({
            key: encodeNode({ t: 'schema', sessionId: session.id, database, schema }),
            title: (
              <NodeMenu
                items={[
                  {
                    key: 'er',
                    icon: <PartitionOutlined />,
                    label: 'ER diagram',
                    onClick: () => openErTab(session.id, database, schema),
                  },
                  {
                    key: 'ddl',
                    icon: <CodeOutlined />,
                    label: 'New DDL script…',
                    onClick: () => openDdlTab(session.id, database, schema),
                  },
                  { type: 'divider' as const },
                  {
                    key: 'refresh',
                    icon: <ReloadOutlined />,
                    label: 'Reload objects',
                    onClick: () => void loadObjects(session.id, database, schema),
                  },
                ]}
              >
                <span>{schema}</span>
              </NodeMenu>
            ),
            icon: <AppstoreOutlined />,
            isLeaf: false,
          }))
        }

        return {
          key: encodeNode({ t: 'db', sessionId: session.id, database }),
          title: database,
          icon: <DatabaseOutlined />,
          isLeaf: false,
          children,
        }
      }

      // Engines without a schema layer (MySQL, SQLite) read their objects
      // straight from the database node; MySQL uses the database as schema.
      const ns = namespaceKey(session.id, database, database)
      return {
        key: encodeNode({ t: 'db', sessionId: session.id, database }),
        title: (
          <NodeMenu
            items={[
              {
                key: 'er',
                icon: <PartitionOutlined />,
                label: 'ER diagram',
                onClick: () => openErTab(session.id, database, database),
              },
              {
                key: 'ddl',
                icon: <CodeOutlined />,
                label: 'New DDL script…',
                onClick: () => openDdlTab(session.id, database, database),
              },
              { type: 'divider' as const },
              {
                key: 'refresh',
                icon: <ReloadOutlined />,
                label: 'Reload objects',
                onClick: () => void loadObjects(session.id, database, database),
              },
            ]}
          >
            <span>{database}</span>
          </NodeMenu>
        ),
        icon: <DatabaseOutlined />,
        isLeaf: false,
        children: tree.loaded[ns] || tree.errors[ns]
          ? buildNamespace(session.id, database, database)
          : undefined,
      }
    },
    [
      buildNamespace,
      driverOfType,
      loadObjects,
      loadSchemas,
      loadedKeys,
      openDdlTab,
      openErTab,
      tree.errors,
      tree.loaded,
      tree.loading,
      tree.schemas,
    ],
  )

  const treeData = useMemo<TreeDataNode[]>(() => {
    return visibleRoots.map((root) => {
      const session = root.session
      const connectionKey = encodeNode({ t: 'connection', connectionId: root.id })

      let children: TreeDataNode[] | undefined
      if (!session) {
        // A node that carries children is never handed to `loadData` again, so
        // the hint below may only appear once the key is known to be loaded —
        // otherwise the connection could never be opened from here.
        if (pending === root.id) {
          children = [placeholderNode(root.id, 'Connecting…')]
        } else if (loadedKeys.some((key) => String(key) === connectionKey)) {
          children = [
            emptyNode(
              root.id,
              'Not connected — expand this node again to retry',
              'Use “Open connection” in the context menu, or collapse and expand this node, to try again.',
            ),
          ]
        }
      } else if (tree.loading[session.id]) {
        children = [placeholderNode(session.id, 'Loading databases…')]
      } else if (tree.errors[session.id]) {
        children = [
          errorNode(session.id, tree.errors[session.id], () => void loadDatabases(session.id)),
        ]
      } else {
        // Every engine keeps its database node, SQLite's `main` included — that
        // is the level Navicat shows, and it keeps the load/expand state simple.
        const databases = tree.databases[session.id]
        if (databases) {
          children =
            databases.length > 0
              ? databases.map((database) => buildDatabaseNode(session, database))
              : [emptyNode(session.id, 'No databases on this server yet')]
        }
      }

      const connected = Boolean(session)
      return {
        key: connectionKey,
        title: (
          <NodeMenu items={connectionMenuItems(root, session)}>
            <span className="dm-connection-node">
              <span
                className="dm-connection-dot"
                style={{ background: root.profile?.color ?? driverColor(root.driver?.type) }}
              />
              <span className="dm-truncate">{root.name}</span>
              {connected ? null : (
                <span className="dm-connection-state" title="Not connected">
                  <DisconnectOutlined />
                </span>
              )}
            </span>
          </NodeMenu>
        ),
        icon: (
          <img
            src={driverIconOrLogo(root.driver?.type)}
            alt=""
            draggable={false}
            className={`dm-driver-icon${connected ? '' : ' is-offline'}`}
          />
        ),
        isLeaf: false,
        children,
      }
    })
    // Menu builders close over the current tree/session state on purpose.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    buildDatabaseNode,
    buildNamespace,
    connections,
    driverOfType,
    loadDatabases,
    modal,
    openQueryTab,
    pending,
    refreshSession,
    roots,
    sessions,
    tree.databases,
    tree.errors,
    tree.loading,
    visibleRoots,
  ])

  /* ------------------------------------------------------------ callbacks */

  const handleLoadData = useCallback<NonNullable<TreeProps['loadData']>>(
    async (node) => {
      const ref = decodeNode(String(node.key))
      if (!ref) return
      switch (ref.t) {
        case 'connection': {
          const session = sessionForConnection(ref.connectionId)
          if (session) {
            await loadDatabases(session.id)
            return
          }
          const profile = connections.find((c) => c.id === ref.connectionId)
          if (profile) await connect(profile)
          break
        }        case 'db': {
          const session = sessions.find((s) => s.id === ref.sessionId)
          if (session && driverOfType(session.driver)?.supportsSchema) {
            await loadSchemas(ref.sessionId, ref.database)
          } else {
            await loadObjects(ref.sessionId, ref.database, ref.database)
          }
          break
        }
        case 'schema':
          await loadObjects(ref.sessionId, ref.database, ref.schema)
          break
        case 'indexFolder':
          await loadIndexes(ref.sessionId, ref.database, ref.schema)
          break
        default:
          break
      }
    },
    [
      connect,
      connections,
      driverOfType,
      loadDatabases,
      loadIndexes,
      loadObjects,
      loadSchemas,
      sessionForConnection,
      sessions,
    ],
  )

  const handleSelect = useCallback<NonNullable<TreeProps['onSelect']>>(
    (keys, info) => {
      setSelectedKeys(keys as React.Key[])
      const ref = decodeNode(String(info.node.key))
      if (!ref) return

      if (ref.t === 'connection') {
        const session = sessionForConnection(ref.connectionId)
        if (session) setActiveSession(session.id)
        return
      }

      setActiveSession(ref.sessionId)
      if (ref.t === 'folder') {
        // Navicat reveals the folder's contents and its object list together.
        const key = info.node.key
        setExpandedKeys((current) =>
          current.some((k) => String(k) === String(key)) ? current : [...current, key],
        )
        openList(ref.sessionId, ref.database, ref.schema, ref.kind)
      }
      if (ref.t === 'indexFolder') {
        const key = info.node.key
        setExpandedKeys((current) =>
          current.some((k) => String(k) === String(key)) ? current : [...current, key],
        )
        openList(ref.sessionId, ref.database, ref.schema, 'index')
      }
      if (ref.t === 'object') {
        const objects = tree.objects[objectsKey(ref.sessionId, ref.database, ref.schema)] ?? []
        const object = objects.find((o) => o.name === ref.object)
        if (object) openObject(ref.sessionId, ref.database, ref.schema, object, 'data')
      }
      if (ref.t === 'index') {
        const objects = tree.objects[objectsKey(ref.sessionId, ref.database, ref.schema)] ?? []
        const object = objects.find((o) => o.name === ref.table)
        if (object) openObject(ref.sessionId, ref.database, ref.schema, object, 'indexes')
      }
    },
    [openList, openObject, sessionForConnection, setActiveSession, tree.objects],
  )

  const handleExpand = useCallback<NonNullable<TreeProps['onExpand']>>(
    (keys) => {
      const next = keys as React.Key[]
      const open = new Set(expandedKeys.map(String))
      const fresh = next.filter((key) => !open.has(String(key)))
      setExpandedKeys(next)

      // Expanding a node that never received its data is how the user asks for
      // another try. Drop those keys from `loadedKeys` once, so rc-tree runs a
      // single fresh load instead of retrying on every render.
      if (fresh.length === 0) return
      const retry = new Set(fresh.map(String))
      setLoadedKeys((current) => {
        const kept = current.filter((key) => !retry.has(String(key)) || hasData(String(key)))
        return kept.length === current.length ? current : kept
      })
    },
    [expandedKeys, hasData],
  )

  const handleLoad = useCallback<NonNullable<TreeProps['onLoad']>>((keys) => {
    setLoadedKeys(keys as React.Key[])
  }, [])

  const collapseAll = useCallback(() => {
    setExpandedKeys([])
  }, [])

  // Keep the pane in sync when a session disappears (disconnect, window close).
  useEffect(() => {
    const live = new Set(roots.map((root) => root.id))
    setExpandedKeys((keys) =>
      keys.filter((key) => {
        const ref = decodeNode(String(key))
        return !ref || ref.t !== 'connection' || live.has(ref.connectionId)
      }),
    )
  }, [roots])

  // Navicat reveals the object list as soon as a connection comes up.
  const knownSessions = useRef<Set<string> | null>(null)
  useEffect(() => {
    const current = new Set(sessions.map((s) => s.id))
    const previous = knownSessions.current
    knownSessions.current = current
    if (!previous) return
    const fresh = sessions.filter((s) => !previous.has(s.id))
    if (fresh.length === 0) return
    setExpandedKeys((keys) => {
      const next = new Set(keys.map(String))
      for (const session of fresh) {
        next.add(encodeNode({ t: 'connection', connectionId: session.connectionId ?? session.id }))
      }
      return [...next]
    })
  }, [sessions])

  /* ---------------------------------------------------------------- menus */

  /**
   * Right-click on the empty part of the pane. Rows bring their own menus, and
   * typing surfaces keep the browser menu, so both are left alone.
   */
  const openBlankMenu = useCallback((event: React.MouseEvent) => {
    const target = event.target as HTMLElement
    if (target.closest('.ant-tree-treenode, input, textarea, .ant-btn, a')) return
    event.preventDefault()
    setBlankMenu({ x: event.clientX, y: event.clientY })
  }, [])

  // A hand-positioned menu needs its own dismissal rules: antd only manages the
  // ones it anchors itself.
  useEffect(() => {
    if (!blankMenu) return undefined
    const dismiss = (event: Event) => {
      if (event.target instanceof Node && blankMenuRef.current?.contains(event.target)) return
      setBlankMenu(null)
    }
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setBlankMenu(null)
    }
    document.addEventListener('mousedown', dismiss)
    document.addEventListener('keydown', onKey)
    // The menu is pinned to a viewport point, so anything that scrolls moves the
    // tree out from under it.
    window.addEventListener('scroll', dismiss, true)
    window.addEventListener('resize', dismiss)
    return () => {
      document.removeEventListener('mousedown', dismiss)
      document.removeEventListener('keydown', onKey)
      window.removeEventListener('scroll', dismiss, true)
      window.removeEventListener('resize', dismiss)
    }
  }, [blankMenu])

  const blankMenuItems: MenuProps['items'] = [
    { key: 'new', icon: <PlusOutlined />, label: 'New connection' },
  ]

  return (
    <div className="dm-sidebar" onContextMenu={openBlankMenu}>
      <div className="dm-sidebar-header">
        <span className="dm-sidebar-title">Connections</span>
        <Tooltip title="New connection">
          <Button size="small" type="text" icon={<PlusOutlined />} onClick={() => openEditor()} />
        </Tooltip>
        <Tooltip title="Collapse all">
          <Button size="small" type="text" icon={<MinusSquareOutlined />} onClick={collapseAll} />
        </Tooltip>
        <Tooltip title="Manage connections">
          <Button
            size="small"
            type="text"
            icon={<UnorderedListOutlined />}
            onClick={() => setManageOpen(true)}
          />
        </Tooltip>
      </div>

      <div className="dm-sidebar-search">
        <Input
          size="small"
          allowClear
          value={filter}
          prefix={<SearchOutlined style={{ opacity: 0.5 }} />}
          placeholder="Filter connections"
          onChange={(event) => setFilter(event.target.value)}
        />
      </div>

      <div className="dm-sidebar-tree">
        {roots.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={<span style={{ fontSize: 12 }}>No connections yet</span>}
            style={{ marginTop: 40 }}
          >
            <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => openEditor()}>
              New connection
            </Button>
          </Empty>
        ) : visibleRoots.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={<span style={{ fontSize: 12 }}>No match for “{filter}”</span>}
            style={{ marginTop: 40 }}
          />
        ) : (
          <Tree
            showIcon
            blockNode
            treeData={treeData}
            expandedKeys={expandedKeys}
            selectedKeys={selectedKeys}
            loadedKeys={loadedKeys}
            loadData={handleLoadData}
            onExpand={handleExpand}
            onSelect={handleSelect}
            onLoad={handleLoad}
            style={{ background: 'transparent' }}
          />
        )}
      </div>

      <ManageConnectionsModal
        open={manageOpen}
        onClose={() => setManageOpen(false)}
        onEdit={(profile) => {
          setManageOpen(false)
          openEditor(profile)
        }}
      />

      <NewDatabaseModal session={newDatabase} onClose={() => setNewDatabase(null)} />

      {blankMenu ? (
        <div
          ref={blankMenuRef}
          className="dm-blank-menu"
          style={{ left: blankMenu.x, top: blankMenu.y }}
        >
          <Menu
            items={blankMenuItems}
            selectable={false}
            onClick={() => {
              setBlankMenu(null)
              openEditor()
            }}
          />
        </div>
      ) : null}
    </div>
  )

  /* --------------------------------------------------------------- menus */

  function indexMenuItems(
    sessionId: string,
    database: string,
    schema: string,
    index: IndexEntry,
  ): MenuProps['items'] {
    return [
      {
        key: 'open',
        icon: <TableOutlined />,
        label: `Open ${index.table}`,
        onClick: () => {
          const objects = tree.objects[objectsKey(sessionId, database, schema)] ?? []
          const object = objects.find((o) => o.name === index.table)
          if (object) openObject(sessionId, database, schema, object, 'indexes')
        },
      },
      {
        key: 'copy',
        icon: <NumberOutlined />,
        label: 'Copy name',
        onClick: () => void copyText(index.name),
      },
    ]
  }

  function connectionMenuItems(root: RootEntry, session: SessionInfo | undefined): MenuProps['items'] {
    const items: MenuProps['items'] = []
    const profile = root.profile
    if (session) {
      items.push(
        {
          key: 'query',
          icon: <EditOutlined />,
          label: 'New query',
          onClick: () => openQueryTab(session.id, session.database),
        },
        {
          key: 'refresh',
          icon: <ReloadOutlined />,
          label: 'Refresh',
          onClick: () => refreshSession(session.id),
        },
        { type: 'divider' as const },
        {
          key: 'newDatabase',
          icon: <DatabaseOutlined />,
          label: 'New database…',
          disabled: !canCreateDatabase(root.driver) || session.readOnly,
          onClick: () => setNewDatabase(session),
        },
        {
          key: 'edit',
          icon: <EditOutlined />,
          label: 'Edit connection…',
          disabled: !profile,
          onClick: () => profile && openEditor(profile),
        },
        { type: 'divider' as const },
        {
          key: 'disconnect',
          icon: <DisconnectOutlined />,
          label: 'Disconnect',
          danger: true,
          onClick: () =>
            modal.confirm({
              title: `Disconnect from ${session.name}?`,
              content: 'Tabs belonging to this connection will be closed.',
              okText: 'Disconnect',
              okButtonProps: { danger: true },
              onOk: () => closeSession(session.id),
            }),
        },
      )
    } else if (profile) {
      items.push(
        {
          key: 'connect',
          icon: <ThunderboltOutlined />,
          label: 'Open connection',
          onClick: () => void connect(profile),
        },
        {
          key: 'edit',
          icon: <EditOutlined />,
          label: 'Edit connection…',
          onClick: () => openEditor(profile),
        },
      )
    }
    return items
  }
}

/* ------------------------------------------------------------------ helpers */

function objectTooltip(object: ObjectInfo): string {
  const parts = [object.name]
  if (object.rowEstimate > 0) parts.push(`~${object.rowEstimate.toLocaleString()} rows`)
  if (object.sizeBytes > 0) parts.push(`${(object.sizeBytes / 1024).toFixed(1)} KiB`)
  if (object.comment) parts.push(object.comment)
  return parts.join(' · ')
}

function indexTooltip(index: IndexEntry): string {
  const parts = [index.name, index.table]
  if (index.columns.length > 0) parts.push(`(${index.columns.join(', ')})`)
  if (index.primary) parts.push('PRIMARY')
  else if (index.unique) parts.push('UNIQUE')
  if (index.method) parts.push(index.method)
  return parts.join(' · ')
}

/**
 * Engine that keeps several databases in one server, so `CREATE DATABASE` makes
 * sense. File-backed engines (SQLite) and schema-only engines say no.
 */
function canCreateDatabase(driver: DriverInfo | undefined): boolean {
  return Boolean(driver && driver.supportsDatabase && !driver.requiresFile)
}

/** Colour used when a profile has no explicit colour. */
function driverColor(type: DriverInfo['type'] | undefined): string {
  switch (type) {
    case 'mysql':
      return '#00758f'
    case 'postgres':
      return '#336791'
    case 'sqlite':
      return '#0f80cc'
    case 'mongodb':
      return '#13aa52'
    case 'oracle':
      return '#c74634'
    case 'sqlserver':
      return '#a91b0d'
    default:
      return '#8c8c8c'
  }
}

function placeholderNode(scope: string, text: string): TreeDataNode {
  return {
    key: `placeholder:${scope}`,
    title: <span style={{ opacity: 0.6, fontSize: 12 }}>{text}</span>,
    isLeaf: true,
    selectable: false,
    icon: <ReloadOutlined spin />,
  }
}

/** Terminal hint for a namespace that legitimately has nothing in it. */
function emptyNode(scope: string, text: string, tip?: string): TreeDataNode {
  return {
    key: `empty:${scope}`,
    title: (
      <Tooltip
        title={
          tip ?? 'Create one with CREATE DATABASE … in a query tab, then reload the catalog.'
        }
      >
        <span style={{ opacity: 0.6, fontSize: 12 }}>{text}</span>
      </Tooltip>
    ),
    isLeaf: true,
    selectable: false,
  }
}

function errorNode(scope: string, text: string, onRetry?: () => void): TreeDataNode {
  return {
    key: `error:${scope}`,
    title: (
      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 12 }}>
        <Tooltip title={text}>
          <span style={{ color: '#ff7875' }}>{text.split('\n')[0]}</span>
        </Tooltip>
        {onRetry ? (
          <Button
            type="link"
            size="small"
            style={{ padding: 0, height: 'auto', fontSize: 12 }}
            onClick={onRetry}
          >
            Retry
          </Button>
        ) : null}
      </span>
    ),
    isLeaf: true,
    selectable: false,
  }
}

/** Wraps a tree node title in a right-click menu. */
function NodeMenu({ items, children }: { items: MenuProps['items']; children: ReactNode }) {
  return (
    <Dropdown menu={{ items }} trigger={['contextMenu']}>
      {children}
    </Dropdown>
  )
}

async function copyText(text: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    // Clipboard access can be denied inside the webview; ignore.
  }
}

/* ------------------------------------------------- manage connections modal */

/**
 * Creates a database on a live session.
 *
 * The name is the only input: the statement is rendered from it with the
 * session driver's quoting rules and shown verbatim before it runs, so there is
 * no hidden SQL and no hand-typed statement to get wrong.
 */
function NewDatabaseModal({
  session,
  onClose,
}: {
  session: SessionInfo | null
  onClose: () => void
}) {
  const loadDatabases = useAppStore((s) => s.loadDatabases)
  const { message } = AntApp.useApp()
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const sql = session && name.trim() ? `CREATE DATABASE ${quoteIdent(name.trim(), session.driver)}` : ''

  // Every way out clears the dialog, so it never reopens on a stale name.
  const close = () => {
    setName('')
    setError(null)
    setBusy(false)
    onClose()
  }

  const submit = async () => {
    const database = name.trim()
    if (!session || !database || busy) return
    setBusy(true)
    setError(null)
    try {
      await api.executeSql({ sessionId: session.id, sql, timeoutMs: 60000 })
      message.success(`Database ${database} created`)
      // A new namespace invalidates nothing but the database list itself.
      void loadDatabases(session.id)
      close()
    } catch (err) {
      setError(toMessage(err))
      setBusy(false)
    }
  }

  return (
    <Modal
      open={session !== null}
      title={session ? `New database on ${session.name}` : 'New database'}
      okText="Create"
      confirmLoading={busy}
      okButtonProps={{ disabled: !sql }}
      onOk={() => void submit()}
      onCancel={close}
      destroyOnHidden
    >
      <label className="dm-field-label" htmlFor="dm-new-database-name">
        Database name
      </label>
      <Input
        id="dm-new-database-name"
        autoFocus
        value={name}
        placeholder="analytics"
        onChange={(event) => setName(event.target.value)}
        onPressEnter={() => void submit()}
      />
      {error ? (
        <Typography.Paragraph type="danger" style={{ marginTop: 10, marginBottom: 0 }}>
          {error}
        </Typography.Paragraph>
      ) : (
        <Typography.Paragraph type="secondary" style={{ marginTop: 10, marginBottom: 0 }}>
          {sql ? <code>{sql}</code> : 'Name it and the statement appears here.'}
        </Typography.Paragraph>
      )}
    </Modal>
  )
}

function ManageConnectionsModal({
  open,
  onClose,
  onEdit,
}: {
  open: boolean
  onClose: () => void
  onEdit: (profile: ConnectionConfig) => void
}) {
  const connections = useAppStore((s) => s.connections)
  const sessions = useAppStore((s) => s.sessions)
  const deleteConnection = useAppStore((s) => s.deleteConnection)
  const closeSession = useAppStore((s) => s.closeSession)
  const { connect, pending } = useConnect()
  const { message, modal } = AntApp.useApp()

  return (
    <Modal
      open={open}
      title="Saved connections"
      footer={null}
      width={620}
      onCancel={onClose}
      destroyOnHidden
    >
      {connections.length === 0 ? (
        <Empty description="No saved connections yet" />
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {connections.map((profile) => {
            const session = sessions.find((s) => s.connectionId === profile.id)
            return (
              <div
                key={profile.id}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 10,
                  padding: '8px 10px',
                  border: '1px solid var(--dm-border)',
                  borderRadius: 6,
                }}
              >
                <img
                  src={driverIconOrLogo(profile.driver)}
                  alt=""
                  draggable={false}
                  className="dm-driver-icon is-large"
                />
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontWeight: 500 }}>{profile.name}</div>
                  <Typography.Text type="secondary" style={{ fontSize: 11 }} className="mono">
                    {describeProfile(profile)}
                  </Typography.Text>
                </div>
                {session ? (
                  <Button size="small" onClick={() => void closeSession(session.id)}>
                    Disconnect
                  </Button>
                ) : (
                  <Button
                    size="small"
                    type="primary"
                    loading={pending === profile.id}
                    onClick={() => void connect(profile)}
                  >
                    Connect
                  </Button>
                )}
                <Tooltip title="Edit">
                  <Button
                    size="small"
                    type="text"
                    icon={<EditOutlined />}
                    onClick={() => onEdit(profile)}
                  />
                </Tooltip>
                <Tooltip title="Delete">
                  <Button
                    size="small"
                    type="text"
                    danger
                    icon={<DeleteOutlined />}
                    onClick={() =>
                      modal.confirm({
                        title: `Delete “${profile.name}”?`,
                        content:
                          'The stored profile and its saved password are removed. The database itself is not touched.',
                        okText: 'Delete',
                        okButtonProps: { danger: true },
                        onOk: async () => {
                          try {
                            if (session) await closeSession(session.id)
                            await deleteConnection(profile.id)
                            message.success('Connection deleted')
                          } catch (error) {
                            message.error(error instanceof Error ? error.message : String(error))
                          }
                        },
                      })
                    }
                  />
                </Tooltip>
              </div>
            )
          })}
        </div>
      )}
    </Modal>
  )
}

/** One-line summary of a profile for list views. */
export function describeProfile(profile: ConnectionConfig): string {
  if (profile.filePath) return `${profile.driver} · ${profile.filePath}`
  const host = profile.host ?? ''
  const port = profile.port ? `:${profile.port}` : ''
  const db = profile.database ? `/${profile.database}` : ''
  const user = profile.username ? `${profile.username}@` : ''
  return `${profile.driver} · ${user}${host}${port}${db}`
}
