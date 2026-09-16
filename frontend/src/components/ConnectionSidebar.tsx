import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { App as AntApp, Button, Dropdown, Empty, Input, Modal, Space, Tooltip, Tree, Typography } from 'antd'
import type { MenuProps, TreeDataNode, TreeProps } from 'antd'
import {
  AppstoreOutlined,
  ContainerOutlined,
  DatabaseOutlined,
  DeleteOutlined,
  DisconnectOutlined,
  EditOutlined,
  EyeOutlined,
  FolderOutlined,
  FunctionOutlined,
  KeyOutlined,
  MinusSquareOutlined,
  NumberOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
  TableOutlined,
  ThunderboltOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons'

import type { ConnectionConfig, DriverInfo, IndexEntry, ObjectInfo, SessionInfo } from '../api/types'
import { useConnect } from '../hooks/useConnect'
import { driverIconOrLogo } from '../lib/assets'
import { useAppStore, type TableView } from '../store/appStore'
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
  const openQueryTab = useAppStore((s) => s.openQueryTab)
  const closeSession = useAppStore((s) => s.closeSession)
  const setActiveSession = useAppStore((s) => s.setActiveSession)

  const { connect, pending } = useConnect()
  const { modal } = AntApp.useApp()

  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([])
  const [selectedKeys, setSelectedKeys] = useState<React.Key[]>([])
  const [loadedKeys, setLoadedKeys] = useState<React.Key[]>([])
  const [filter, setFilter] = useState('')
  const [manageOpen, setManageOpen] = useState(false)

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
   * A node only counts as "loaded" once its data actually arrived, so a failed
   * or cancelled load can be retried by expanding the node again.
   */
  const effectiveLoadedKeys = useMemo(
    () =>
      loadedKeys.filter((key) => {
        const ref = decodeNode(String(key))
        if (!ref) return false
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
      }),
    [driverOfType, loadedKeys, sessionForConnection, sessions, tree.loaded],
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
        title: indexes ? `Indexes (${indexes.length})` : 'Indexes',
        icon: <KeyOutlined />,
        selectable: false,
        isLeaf: false,
        children,
      }
    },
    [loadIndexes, tree.errors, tree.indexes, tree.loading],
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
          title: `${FOLDER_LABEL[kind]} (${items.length})`,
          icon: <FolderOutlined />,
          selectable: false,
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
    [buildIndexFolder, openObject, openQueryTab, tree.objects],
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
            title: schema,
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
        title: database,
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
      loadSchemas,
      tree.errors,
      tree.loaded,
      tree.loading,
      tree.schemas,
    ],
  )

  const treeData = useMemo<TreeDataNode[]>(() => {
    return visibleRoots.map((root) => {
      const session = root.session

      let children: TreeDataNode[] | undefined
      if (!session) {
        if (pending === root.id) children = [placeholderNode(root.id, 'Connecting…')]
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
          children = databases.map((database) => buildDatabaseNode(session, database))
        }
      }

      const connected = Boolean(session)
      return {
        key: encodeNode({ t: 'connection', connectionId: root.id }),
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
    [openObject, sessionForConnection, setActiveSession, tree.objects],
  )

  const handleExpand = useCallback<NonNullable<TreeProps['onExpand']>>((keys) => {
    setExpandedKeys(keys as React.Key[])
  }, [])

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

  const savedProfiles = useMemo(
    () => connections.filter((c) => !sessions.some((s) => s.connectionId === c.id)),
    [connections, sessions],
  )

  const connectMenu: MenuProps = {
    items:
      savedProfiles.length === 0
        ? [{ key: 'none', label: 'Every saved connection is open', disabled: true }]
        : savedProfiles.map((profile) => ({
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

  return (
    <div className="dm-sidebar">
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
            loadedKeys={effectiveLoadedKeys}
            loadData={handleLoadData}
            onExpand={handleExpand}
            onSelect={handleSelect}
            onLoad={handleLoad}
            style={{ background: 'transparent' }}
          />
        )}
      </div>

      <div className="dm-sidebar-footer">
        <Space.Compact style={{ width: '100%' }}>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            style={{ flex: 1 }}
            onClick={() => openEditor()}
          >
            New
          </Button>
          <Dropdown menu={connectMenu} trigger={['click']} placement="topRight">
            <Button icon={<ThunderboltOutlined />}>Connect</Button>
          </Dropdown>
        </Space.Compact>
      </div>

      <ManageConnectionsModal
        open={manageOpen}
        onClose={() => setManageOpen(false)}
        onEdit={(profile) => {
          setManageOpen(false)
          openEditor(profile)
        }}
      />
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
    } else if (root.profile) {
      const profile = root.profile
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
          label: 'Edit connection',
          onClick: () => openEditor(profile),
        },
      )
    }
    return items
  }
}

/* ------------------------------------------------------------------ helpers */

function objectIcon(kind: ObjectInfo['kind']): ReactNode {
  switch (kind) {
    case 'view':
    case 'materialized_view':
      return <EyeOutlined />
    case 'collection':
      return <ContainerOutlined />
    case 'sequence':
      return <NumberOutlined />
    case 'procedure':
      return <FunctionOutlined />
    default:
      return <TableOutlined />
  }
}

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
