import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { App as AntApp, Alert, AutoComplete, Button, Dropdown, Empty, Input, Menu, Modal, Select, Tooltip, Tree, Typography } from 'antd'
import type { MenuProps, TreeDataNode, TreeProps } from 'antd'
import {
  AppstoreOutlined,
  ClearOutlined,
  CodeOutlined,
  CopyOutlined,
  DatabaseOutlined,
  DeleteOutlined,
  DisconnectOutlined,
  EditOutlined,
  ExperimentOutlined,
  FileTextOutlined,
  FolderAddOutlined,
  FolderOutlined,
  FunctionOutlined,
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
import type {
  ConnectionConfig,
  DatabaseOptions,
  DatabasePlan,
  DesignPlan,
  DriverInfo,
  IndexEntry,
  ObjectInfo,
  QueryFile,
  SessionInfo,
  TableOpRequest,
} from '../api/types'
import { useConnect } from '../hooks/useConnect'
import { driverIconOrLogo } from '../lib/assets'
import { capabilitiesOf, findDriver, objectKindsOf } from '../lib/capabilities'
import { generatable } from '../lib/codegen'
import {
  arrangementOf,
  dropTargetFor,
  endOfTopLevelTarget,
  groupOf,
  type DropTarget,
  type ExplorerEntry,
} from '../lib/explorer'
import { useAppStore, type ListScope, type TableView } from '../store/appStore'
import { ConnectionTypeDropdown, connectionTypeItems, driverFromKey } from './ConnectionTypeMenu'
import { CopyTableModal, type CopyTableSource } from './CopyTableModal'
import { NamePromptModal } from './NamePromptModal'
import { objectIcon } from './objectIcon'
import { SqlCode } from './SqlCode'
import {
  databaseKey,
  decodeNode,
  encodeNode,
  FOLDER_LABEL,
  FOLDER_ORDER,
  indexesKey,
  KIND_SINGULAR,
  namespaceKey,
  objectsKey,
  queriesKey,
  type NodeRef,
} from '../lib/tree'

/** Menu key of "New group…", which no driver ever answers to. */
const NEW_GROUP_KEY = 'new-group'

/** Key prefix of the driver list nested under a group, so a pick can be told
 * apart from the group's own items: `group.mysql` on the way to the dialog. */
const GROUP_PREFIX = 'group.'

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
 *
 * The list can be arranged: rows are dragged to reorder them, and groups — one
 * level deep — hold connections the same way a folder holds files. Where things
 * sit is stored per-profile-id in `layout.json` and re-derived against the
 * profiles on both ends, so the tree and the file cannot disagree; see
 * `lib/explorer.ts` for the UI's half of that.
 */
export function ConnectionSidebar() {
  const sessions = useAppStore((s) => s.sessions)
  const drivers = useAppStore((s) => s.drivers)
  const connections = useAppStore((s) => s.connections)
  const connectionLayout = useAppStore((s) => s.connectionLayout)
  const tree = useAppStore((s) => s.tree)
  const openEditor = useAppStore((s) => s.openConnectionEditor)
  const loadDatabases = useAppStore((s) => s.loadDatabases)
  const loadSchemas = useAppStore((s) => s.loadSchemas)
  const loadObjects = useAppStore((s) => s.loadObjects)
  const loadIndexes = useAppStore((s) => s.loadIndexes)
  const loadQueryFiles = useAppStore((s) => s.loadQueryFiles)
  const createQueryFile = useAppStore((s) => s.createQueryFile)
  const renameQueryFile = useAppStore((s) => s.renameQueryFile)
  const deleteQueryFile = useAppStore((s) => s.deleteQueryFile)
  const openQueryFileTab = useAppStore((s) => s.openQueryFileTab)
  const invalidateSession = useAppStore((s) => s.invalidateSession)
  const openTableTab = useAppStore((s) => s.openTableTab)
  const openNewTableTab = useAppStore((s) => s.openNewTableTab)
  const openObjectsTab = useAppStore((s) => s.openObjectsTab)
  const openQueryTab = useAppStore((s) => s.openQueryTab)
  const openDdlTab = useAppStore((s) => s.openDdlTab)
  const openCodegenTab = useAppStore((s) => s.openCodegenTab)
  const openDataGenTab = useAppStore((s) => s.openDataGenTab)
  const openErTab = useAppStore((s) => s.openErTab)
  const openRuntimeTab = useAppStore((s) => s.openRuntimeTab)
  const closeSession = useAppStore((s) => s.closeSession)
  const closeTableTab = useAppStore((s) => s.closeTableTab)
  const setActiveSession = useAppStore((s) => s.setActiveSession)
  const setActiveConnection = useAppStore((s) => s.setActiveConnection)
  const setActiveNamespace = useAppStore((s) => s.setActiveNamespace)
  const reveal = useAppStore((s) => s.reveal)
  const clearReveal = useAppStore((s) => s.clearReveal)
  const moveConnection = useAppStore((s) => s.moveConnection)
  const deleteConnectionGroup = useAppStore((s) => s.deleteConnectionGroup)

  const { connect, pending } = useConnect()
  const { message, modal } = AntApp.useApp()

  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([])
  const [selectedKeys, setSelectedKeys] = useState<React.Key[]>([])
  const [loadedKeys, setLoadedKeys] = useState<React.Key[]>([])
  /** The scrolling pane, for bringing a revealed row into view. */
  const treePaneRef = useRef<HTMLDivElement | null>(null)
  /**
   * Row a reveal asked for, waiting for its turn to be drawn.
   *
   * Pointing the tree at a row means expanding the way down to it first, and a
   * row that was not in the tree a moment ago can only be scrolled to once it
   * has been rendered — one commit later. Keeping the key here lets the scroll
   * happen on the render that finally draws it.
   */
  const revealScroll = useRef<string | null>(null)

  /**
   * Nodes the user opened by expanding them.
   *
   * `loadData` also runs for a node that is merely *still* expanded when the
   * session behind it goes away — that is why Disconnect used to open a fresh
   * session a moment later and the node kept looking connected. Only an expand
   * asks for a connection; the entry is consumed by the first load that sees it.
   */
  const openedByUser = useRef(new Set<string>())
  const [filter, setFilter] = useState('')
  const [manageOpen, setManageOpen] = useState(false)
  /** Group the name window is creating (`group` unset) or renaming. */
  const [groupDialog, setGroupDialog] = useState<{ group?: { id: string; name: string } } | null>(
    null,
  )
  /**
   * The query-name window: creating a script in a database, or renaming one.
   * `rename` is the current name, which is what tells the two apart.
   */
  const [queryName, setQueryName] = useState<
    { sessionId: string; database: string; rename?: string } | null
  >(null)
  /** Where the user asked for the empty-area context menu, if anywhere. */
  const [blankMenu, setBlankMenu] = useState<{ x: number; y: number } | null>(null)
  const blankMenuRef = useRef<HTMLDivElement | null>(null)
  /** Session that the "New database" dialog is creating a database on. */
  const [newDatabase, setNewDatabase] = useState<SessionInfo | null>(null)
  /** Table the "Duplicate table" window is copying, if any. */
  const [copyTable, setCopyTable] = useState<CopyTableSource | null>(null)
  /**
   * Connection whose runtime page a double-click asked for. A profile without a
   * stored password connects through a prompt that finishes later, so the page
   * is opened by the effect that watches the session list, not by the click.
   */
  const pendingRuntime = useRef<string | null>(null)

  const driverOfType = useCallback(
    (type: DriverInfo['type'] | undefined) => findDriver(drivers, type),
    [drivers],
  )

  /** The driver of a live session, for the capability checks in the tree. */
  const driverOfSession = useCallback(
    (sessionId: string) => {
      const session = sessions.find((s) => s.id === sessionId)
      return driverOfType(session?.driver)
    },
    [driverOfType, sessions],
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
        case 'queries':
          return Boolean(tree.loaded[queriesKey(ref.sessionId, ref.database)])
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

  /** The pane's top level as drawn: the groups, and the connections outside them. */
  const arrangement = useMemo(
    () => arrangementOf(connections, connectionLayout),
    [connectionLayout, connections],
  )

  /**
   * Connections with no profile — an ad-hoc session, or one whose profile was
   * deleted. Nothing stores where they sit, so they follow the arrangement in
   * the order the session list hands them over.
   */
  const looseSessions = useMemo(() => roots.filter((root) => !root.profile), [roots])

  const rootById = useMemo(() => new Map(roots.map((root) => [root.id, root])), [roots])
  const needle = filter.trim().toLowerCase()

  /**
   * The arrangement, filtered to what the search box matches.
   *
   * A group survives as a whole when its own name matches — the user asked for
   * the folder — and with only its matching members when one of those matched.
   * A group that ends up with nothing to show is dropped: an empty folder that
   * appeared out of a filter would be a place with nothing in it, not a hit.
   */
  const visibleArrangement = useMemo(() => {
    if (!needle) return arrangement
    const nameOf = (id: string) => rootById.get(id)?.name.toLowerCase() ?? ''
    const out: ExplorerEntry[] = []
    for (const entry of arrangement) {
      if (entry.t === 'connection') {
        if (nameOf(entry.id).includes(needle)) out.push(entry)
        continue
      }
      if (entry.name.toLowerCase().includes(needle)) {
        out.push(entry)
        continue
      }
      const members = entry.members.filter((id) => nameOf(id).includes(needle))
      if (members.length > 0) out.push({ ...entry, members })
    }
    return out
  }, [arrangement, needle, rootById])

  const visibleSessions = useMemo(
    () =>
      needle
        ? looseSessions.filter((root) => root.name.toLowerCase().includes(needle))
        : looseSessions,
    [looseSessions, needle],
  )

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

  /**
   * Empties a table, or removes it, from the explorer's table menu.
   *
   * Neither gets a window of its own, because neither has anything to ask: the
   * statement is rendered by the backend from the live catalog, shown here for
   * confirmation, and the yes/no is the whole interaction. The preview and the run
   * are two calls for the same reason the designer's are — the script that runs is
   * planned again on the other side of the bridge rather than handed back as text.
   *
   * A drop closes the window about that table, if one is open: everything it could
   * show — rows, fields, indexes — is gone, and a window cannot outlive the table
   * it is named after.
   */
  const runTableOp = useCallback(
    async (
      op: 'truncate' | 'drop',
      place: { sessionId: string; database: string; schema: string; object: string },
    ) => {
      const request: TableOpRequest = { ...place }
      const table = place.object
      let plan: DesignPlan
      try {
        plan =
          op === 'drop'
            ? await api.planDropTable(request)
            : await api.planTruncateTable(request)
      } catch (error) {
        message.error(toMessage(error))
        return
      }

      modal.confirm({
        title: op === 'drop' ? `Drop table ${table}?` : `Empty table ${table}?`,
        width: 720,
        okText: op === 'drop' ? 'Drop' : 'Truncate',
        okButtonProps: { danger: true },
        content: (
          <div>
            {plan.warnings.length > 0 ? (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 12 }}
                title="Before this runs"
                description={
                  <ul style={{ margin: 0, paddingInlineStart: 18 }}>
                    {plan.warnings.map((warning) => (
                      <li key={warning}>{warning}</li>
                    ))}
                  </ul>
                }
              />
            ) : null}
            <SqlCode
              className="dm-ddl"
              driver={driverOfSession(place.sessionId)?.type}
              sql={plan.statements.map((s) => `${s};`).join('\n')}
            />
            <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginTop: 8, marginBottom: 0 }}>
              {op === 'drop'
                ? 'The table and every row in it go, and this cannot be undone.'
                : 'Every row goes; the table, its fields and its indexes stay.'}
            </Typography.Paragraph>
          </div>
        ),
        onOk: async () => {
          try {
            const result =
              op === 'drop'
                ? await api.dropTable(request)
                : await api.truncateTable(request)
            if (result.error) {
              message.error(
                `${op === 'drop' ? 'Dropping' : 'Emptying'} ${table} failed: ${result.error}`,
              )
              return
            }
            if (op === 'drop') {
              closeTableTab(place.sessionId, place.database, place.schema, table)
              message.success(`Table ${table} dropped`)
            } else {
              message.success(`Table ${table} is empty`)
            }
            // The tree carries the object list and the index list of this
            // namespace, and one statement can change either.
            void loadIndexes(place.sessionId, place.database, place.schema)
            void loadObjects(place.sessionId, place.database, place.schema)
          } catch (error) {
            message.error(toMessage(error))
          }
        },
      })
    },
    [closeTableTab, driverOfSession, loadIndexes, loadObjects, message, modal],
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

  /**
   * Loads a namespace's objects, and the index list that goes with them.
   *
   * Each folder in the tree carries a count, and the index folder's is built
   * from a request of its own — so it cannot wait for a click on that folder
   * the way it used to. The explorer asks for both as soon as the namespace
   * opens; the index list is cached, so expanding the folder afterwards (or
   * opening a table's Indexes page) costs nothing.
   */
  const loadNamespace = useCallback(
    (sessionId: string, database: string, schema: string) => {
      const key = indexesKey(sessionId, database, schema)
      if (tree.indexes[key] === undefined && !tree.loading[key]) {
        void loadIndexes(sessionId, database, schema)
      }
      return loadObjects(sessionId, database, schema)
    },
    [loadIndexes, loadObjects, tree.indexes, tree.loading],
  )

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
            <span data-tree-key={encodeNode({ t: 'indexFolder', sessionId, database, schema })}>
              {indexes ? `Indexes (${indexes.length})` : 'Indexes'}
            </span>
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
      // A document store has no CREATE TABLE / ALTER TABLE to write, so its
      // object menus stop at the data and the sampled field list.
      const { relational, designable, insertable } = capabilitiesOf(driverOfSession(sessionId))
      // Creating a table is a write, so a read-only session offers no menu item
      // for it rather than one that fails after the whole definition is typed.
      const canCreate = designable && !sessions.find((s) => s.id === sessionId)?.readOnly

      const groups = new Map<string, ObjectInfo[]>()
      for (const object of objects) {
        const list = groups.get(object.kind) ?? []
        list.push(object)
        groups.set(object.kind, list)
      }

      // Which folders to draw: the ones the engine declares, then anything the
      // object list handed us that it did not declare.
      //
      // The declared list leads because it carries the engine's own order (a
      // document store shows collections before its few views), and every one
      // of them is drawn even while empty: "Tables (0)" says the database is
      // there and empty, whereas a folder that only shows up once it holds
      // something cannot be told apart from an engine that has no such folder.
      // A kind nobody declared is still drawn rather than dropped — the
      // explorer shows what it is given — after the declared ones, in the
      // order the UI sorts them.
      const declared = objectKindsOf(driverOfSession(sessionId))
      const undeclared = FOLDER_ORDER.filter(
        (kind) => !declared.includes(kind) && (groups.get(kind)?.length ?? 0) > 0,
      )

      const folders: TreeDataNode[] = []
      for (const kind of [...declared, ...undeclared]) {
        const items = groups.get(kind) ?? []
        const key = encodeNode({ t: 'folder', sessionId, database, schema, kind })
        folders.push({
          key,
          title: (
            <NodeMenu
              items={[
                {
                  key: 'list',
                  icon: <UnorderedListOutlined />,
                  label: 'Open object list',
                  onClick: () => openList(sessionId, database, schema, kind),
                },
                ...(kind === 'table'
                  ? [
                      {
                        key: 'create',
                        icon: <PlusOutlined />,
                        label: 'New table…',
                        disabled: !canCreate,
                        onClick: () => openNewTableTab(sessionId, database, schema),
                      },
                    ]
                  : []),
                ...(relational
                  ? [
                      {
                        key: 'ddl',
                        icon: <CodeOutlined />,
                        label: 'New DDL script…',
                        onClick: () => openDdlTab(sessionId, database, schema),
                      },
                    ]
                  : []),
                { type: 'divider' as const },
                {
                  key: 'refresh',
                  icon: <ReloadOutlined />,
                  label: 'Reload objects',
                  onClick: () => void loadNamespace(sessionId, database, schema),
                },
              ]}
            >
              {/* `data-tree-key` is how a reveal finds the row to scroll to. */}
              <span data-tree-key={key}>{`${FOLDER_LABEL[kind]} (${items.length})`}</span>
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
                    label: designable
                      ? `Design ${KIND_SINGULAR[object.kind]}`
                      : 'Open fields',
                    onClick: () => openObject(sessionId, database, schema, object, 'structure'),
                  },
                  ...(relational
                    ? [
                        {
                          key: 'ddl',
                          icon: <CodeOutlined />,
                          label: 'Edit DDL…',
                          onClick: () => openDdlTab(sessionId, database, schema, object.name),
                        },
                      ]
                    : []),
                  // Only something with fields of its own: a sequence and a
                  // stored procedure have no columns to turn into a class.
                  ...(generatable(object.kind)
                    ? [
                        {
                          key: 'codegen',
                          icon: <FunctionOutlined />,
                          label: 'Generate code…',
                          onClick: () => openCodegenTab(sessionId, database, schema, object),
                        },
                      ]
                    : []),
                  // Only a table can be filled: a view has no INSERT of its own
                  // and a document store has no column list, which is exactly
                  // what the window's own capability check says too.
                  ...(object.kind === 'table' && insertable
                    ? [
                        {
                          key: 'datagen',
                          icon: <ExperimentOutlined />,
                          label: 'Data generation…',
                          onClick: () => openDataGenTab(sessionId, database, schema, object.name),
                        },
                      ]
                    : []),
                  // Only a table can be duplicated: a view has no CREATE of
                  // its own to write, and a document store has no CREATE TABLE
                  // at all — the same capability that gates "New table…".
                  ...(object.kind === 'table' && canCreate
                    ? [
                        {
                          key: 'duplicate',
                          icon: <CopyOutlined />,
                          label: 'Duplicate table',
                          children: [
                            {
                              key: 'duplicate-structure',
                              label: 'Structure only',
                              onClick: () =>
                                setCopyTable({
                                  sessionId,
                                  driver: driverOfSession(sessionId)?.type,
                                  database,
                                  schema,
                                  object: object.name,
                                }),
                            },
                            {
                              key: 'duplicate-data',
                              label: 'Structure and data',
                              onClick: () =>
                                setCopyTable({
                                  sessionId,
                                  driver: driverOfSession(sessionId)?.type,
                                  database,
                                  schema,
                                  object: object.name,
                                }),
                            },
                          ],
                        },
                      ]
                    : []),
                  // Emptying and removing are writes too, so they need the same
                  // capability as creating one: an engine this tool can write
                  // tables for, on a connection that is not read-only. Behind a
                  // divider of their own, because neither can be undone and
                  // neither belongs next to "Copy name".
                  ...(object.kind === 'table' && canCreate
                    ? [
                        { type: 'divider' as const },
                        {
                          key: 'truncate',
                          icon: <ClearOutlined />,
                          label: 'Truncate table',
                          onClick: () =>
                            void runTableOp('truncate', {
                              sessionId,
                              database,
                              schema,
                              object: object.name,
                            }),
                        },
                        {
                          key: 'drop',
                          icon: <DeleteOutlined />,
                          label: 'Drop table',
                          danger: true,
                          onClick: () =>
                            void runTableOp('drop', {
                              sessionId,
                              database,
                              schema,
                              object: object.name,
                            }),
                        },
                      ]
                    : []),
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
          // A folder with nothing in it has nothing to reveal, so it is a leaf:
          // its title already carries the (0), and an arrow opening onto empty
          // space is a click that goes nowhere. Selecting it still opens the
          // (empty) object list, exactly like a folder with contents.
          isLeaf: items.length === 0,
        })
      }

      // Indexes live in their own namespace-wide folder, the way Navicat's
      // object list groups them next to tables and views.
      folders.push(buildIndexFolder(sessionId, database, schema))
      return folders
    },
    [
      buildIndexFolder,
      driverOfSession,
      loadNamespace,
      openCodegenTab,
      openDataGenTab,
      openDdlTab,
      openList,
      openNewTableTab,
      openObject,
      runTableOp,
      sessions,
      tree.objects,
    ],
  )

  const buildNamespace = useCallback(
    (sessionId: string, database: string, schema: string): TreeDataNode[] => {
      const ns = namespaceKey(sessionId, database, schema)
      if (tree.loading[ns]) return [placeholderNode(ns, 'Loading objects…')]
      const error = tree.errors[ns]
      if (error) {
        return [errorNode(ns, error, () => void loadNamespace(sessionId, database, schema))]
      }
      return buildFolders(sessionId, database, schema)
    },
    [buildFolders, loadNamespace, tree.errors, tree.loading],
  )

  /**
   * A namespace's folders, or `undefined` while its object list is on its way.
   *
   * rc-tree calls `loadData` for a node that has no children, and the folders
   * only exist once that load has landed — this is what turns the expand into
   * an actual subtree. Both shapes the explorer browses from need it: a schema,
   * and the database itself on an engine without one.
   */
  const namespaceChildren = useCallback(
    (sessionId: string, database: string, schema: string): TreeDataNode[] | undefined => {
      const ns = namespaceKey(sessionId, database, schema)
      if (tree.loaded[ns] || tree.errors[ns]) {
        return buildNamespace(sessionId, database, schema)
      }
      return undefined
    },
    [buildNamespace, tree.errors, tree.loaded],
  )

  /**
   * The Queries folder of one database.
   *
   * It hangs off the *database* even on an engine whose objects live under
   * schemas (PostgreSQL): a script is written for a database, and the same script
   * has to be reachable from every schema in it. It is drawn for any session that
   * has a saved connection profile, because that is what the folder on disk is
   * named after — an ad-hoc session has nowhere to keep one.
   */
  const buildQueriesFolder = useCallback(
    (session: SessionInfo, database: string): TreeDataNode | undefined => {
      if (!session.connectionId) return undefined
      const key = queriesKey(session.id, database)
      const files = tree.queries[key]
      let children: TreeDataNode[] | undefined
      if (tree.loading[key]) children = [placeholderNode(key, 'Loading queries…')]
      else if (tree.errors[key]) {
        children = [
          errorNode(key, tree.errors[key], () => void loadQueryFiles(session.id, database)),
        ]
      } else if (files) {
        children = files.map((file) => {
          const fileKey = encodeNode({
            t: 'queryFile',
            sessionId: session.id,
            database,
            name: file.name,
          })
          return {
            key: fileKey,
            isLeaf: true,
            icon: <FileTextOutlined />,
            title: (
              <NodeMenu items={queryFileMenuItems(session, database, file)}>
                <span data-tree-key={fileKey}>{file.name}</span>
              </NodeMenu>
            ),
          }
        })
      }

      return {
        key: encodeNode({ t: 'queries', sessionId: session.id, database }),
        title: (
          <NodeMenu
            items={[
              {
                key: 'new',
                icon: <PlusOutlined />,
                label: 'New query…',
                onClick: () => setQueryName({ sessionId: session.id, database }),
              },
              { type: 'divider' as const },
              {
                key: 'refresh',
                icon: <ReloadOutlined />,
                label: 'Reload queries',
                onClick: () => void loadQueryFiles(session.id, database),
              },
            ]}
          >
            <span data-tree-key={key}>{files ? `Queries (${files.length})` : 'Queries'}</span>
          </NodeMenu>
        ),
        icon: <FolderOutlined />,
        isLeaf: false,
        children,
      }
    },
    // The folder and its menu close over the current tree state on purpose.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [loadQueryFiles, tree.errors, tree.loading, tree.queries],
  )

  /**
   * A database's children, with the Queries folder after them.
   *
   * It closes the list — under `Indexes`, which is the last object folder — so
   * the folders the engine actually holds stay together at the top. On an
   * engine with schemas the same rule puts it after the schemas.
   *
   * The folder cannot simply be added while the object list is still on its way:
   * rc-tree only calls `loadData` for a node that has no children, so a node that
   * suddenly had one — the folder — would never load its tables. `undefined`
   * therefore still means "ask me again once it is loaded".
   */
  const withQueries = useCallback(
    (
      session: SessionInfo,
      database: string,
      children?: TreeDataNode[],
    ): TreeDataNode[] | undefined => {
      if (!children) return undefined
      const folder = buildQueriesFolder(session, database)
      return folder ? [...children, folder] : children
    },
    [buildQueriesFolder],
  )

  const buildDatabaseNode = useCallback(
    (session: SessionInfo, database: string): TreeDataNode => {
      const driver = driverOfType(session.driver)

      const { relational } = capabilitiesOf(driver)

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
                  ...(relational
                    ? [
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
                      ]
                    : []),
                  { type: 'divider' as const },
                  {
                    key: 'refresh',
                    icon: <ReloadOutlined />,
                    label: 'Reload objects',
                    onClick: () => void loadNamespace(session.id, database, schema),
                  },
                ]}
              >
                <span>{schema}</span>
              </NodeMenu>
            ),
            icon: <AppstoreOutlined />,
            isLeaf: false,
            // The schema holds the folders, so they only appear once its
            // object list has landed — this is the level PostgreSQL browses
            // from, and without it the tree ended here.
            children: namespaceChildren(session.id, database, schema),
          }))
        }

        return {
          key: encodeNode({ t: 'db', sessionId: session.id, database }),
          title: database,
          icon: <DatabaseOutlined />,
          isLeaf: false,
          children: withQueries(session, database, children),
        }
      }

      // Engines without a schema layer (MySQL, SQLite) read their objects
      // straight from the database node; MySQL uses the database as schema.
      return {
        key: encodeNode({ t: 'db', sessionId: session.id, database }),
        title: (
          <NodeMenu
            items={[
              ...(relational
                ? [
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
                  ]
                : []),
              { type: 'divider' as const },
              ...(session.connectionId
                ? [
                    {
                      key: 'new-query',
                      icon: <PlusOutlined />,
                      label: 'New query…',
                      onClick: () => setQueryName({ sessionId: session.id, database }),
                    },
                  ]
                : []),
              {
                key: 'refresh',
                icon: <ReloadOutlined />,
                label: 'Reload objects',
                onClick: () => void loadNamespace(session.id, database, database),
              },
            ]}
          >
            <span>{database}</span>
          </NodeMenu>
        ),
        icon: <DatabaseOutlined />,
        isLeaf: false,
        children: withQueries(session, database, namespaceChildren(session.id, database, database)),
      }
    },
    [
      driverOfType,
      loadNamespace,
      loadSchemas,
      loadedKeys,
      namespaceChildren,
      openDdlTab,
      openErTab,
      tree.errors,
      tree.loaded,
      tree.loading,
      tree.schemas,
      withQueries,
    ],
  )

  /**
   * One connection row and, when it is open, the databases underneath it.
   *
   * The row is what the arrangement holds, so its key is the profile id — or the
   * session id, for an ad-hoc connection that has no profile.
   */
  const buildConnectionNode = useCallback((root: RootEntry): TreeDataNode => {
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
              : [
                  emptyNode(
                    session.id,
                    'No databases on this server yet',
                    capabilitiesOf(root.driver).relational
                      ? 'Pick “New database…” in this connection’s menu: the window renders the CREATE DATABASE statement before running it, and the database shows up here afterwards.'
                      : 'A database appears here once something is written into it — “New database…” selects one with `use`, and it exists after the first document.',
                  ),
                ]
        }
      }

      const connected = Boolean(session)
      return {
        key: connectionKey,
        title: (
          <NodeMenu items={connectionMenuItems(root, session)}>
            <span
              className="dm-connection-node"
              data-tree-key={connectionKey}
              onDoubleClick={() => openRuntime(root, session)}
            >
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
    // Menu builders close over the current tree/session state on purpose.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    buildDatabaseNode,
    connections,
    driverOfType,
    loadDatabases,
    loadedKeys,
    openQueryTab,
    pending,
    refreshSession,
    sessions,
    tree.databases,
    tree.errors,
    tree.loading,
  ])

  /**
   * Opens or closes a group, the way a click on its drawer arrow would.
   *
   * The arrow is a small target and the rest of the row is its label, so the
   * label answers to a double-click too — the gesture the tree already uses for
   * "show me what is inside" (a connection row opens its runtime page that way).
   * A group has nothing of its own to fetch: its members are already in the
   * arrangement, which is why this touches the open set and nothing else.
   */
  const toggleGroup = useCallback((groupId: string) => {
    const key = encodeNode({ t: 'group', groupId })
    setExpandedKeys((keys) =>
      keys.some((open) => String(open) === key)
        ? keys.filter((open) => String(open) !== key)
        : [...keys, key],
    )
  }, [])

  /**
   * The pane's rows: the arrangement, then whatever the arrangement cannot hold.
   *
   * A group is a row like any other connection, so it is dragged and dropped by
   * the same two handlers — that is what makes "drag a connection into a folder"
   * and "drag it back out" the same gesture in both directions.
   */
  const treeData = useMemo<TreeDataNode[]>(() => {
    const nodes: TreeDataNode[] = []
    for (const entry of visibleArrangement) {
      if (entry.t === 'group') {
        const members = entry.members
          .map((id) => rootById.get(id))
          .filter((root): root is RootEntry => Boolean(root))
          .map(buildConnectionNode)
        nodes.push({
          key: encodeNode({ t: 'group', groupId: entry.id }),
          title: (
            <NodeMenu
              items={groupMenuItems(entry)}
              onClick={({ key }) => handleGroupMenuKey(entry, key)}
            >
              <span
                className="dm-group-node"
                onDoubleClick={() => {
                  // A group with nothing in it has nothing to reveal, so it is a
                  // leaf and a double-click on it says nothing.
                  if (members.length > 0) toggleGroup(entry.id)
                }}
              >
                <span className="dm-truncate">{entry.name}</span>
                {members.length > 0 ? (
                  <span className="dm-group-count">{members.length}</span>
                ) : null}
              </span>
            </NodeMenu>
          ),
          icon: <FolderOutlined />,
          // A group with nothing in it has nothing to reveal, so it is a leaf —
          // the same rule its object folders below follow.
          isLeaf: members.length === 0,
          children: members.length > 0 ? members : undefined,
        })
        continue
      }
      const root = rootById.get(entry.id)
      // An entry whose profile went away between two fetches: the next refresh
      // drops it from the arrangement and the row goes with it, so it is skipped
      // for the moment instead of drawn with nothing behind it.
      if (root) nodes.push(buildConnectionNode(root))
    }
    for (const root of visibleSessions) nodes.push(buildConnectionNode(root))
    return nodes
    // Menu builders close over the current tree/session state on purpose.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [buildConnectionNode, rootById, toggleGroup, visibleArrangement, visibleSessions])

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
          // A node whose session is gone still gets loaded while it is expanded
          // (rc-tree re-runs `loadData` until the load reports back), and
          // reconnecting there silently undoes the Disconnect.
          if (!openedByUser.current.delete(String(node.key))) break
          const profile = connections.find((c) => c.id === ref.connectionId)
          if (profile) await connect(profile)
          break
        }
        case 'db': {
          const session = sessions.find((s) => s.id === ref.sessionId)
          if (session && driverOfType(session.driver)?.supportsSchema) {
            await loadSchemas(ref.sessionId, ref.database)
          } else {
            await loadNamespace(ref.sessionId, ref.database, ref.database)
          }
          break
        }
        case 'schema':
          await loadNamespace(ref.sessionId, ref.database, ref.schema)
          break
        case 'queries':
          await loadQueryFiles(ref.sessionId, ref.database)
          break
        case 'indexFolder': {
          // The namespace load already asked for this list, so an expand that
          // finds it cached (or on its way) does not ask twice. "Reload index
          // list" in the folder's menu is the way to force a fresh read.
          const key = indexesKey(ref.sessionId, ref.database, ref.schema)
          if (tree.indexes[key] === undefined && !tree.loading[key]) {
            await loadIndexes(ref.sessionId, ref.database, ref.schema)
          }
          break
        }
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
      loadNamespace,
      loadQueryFiles,
      loadSchemas,
      sessionForConnection,
      sessions,
      tree.indexes,
      tree.loading,
    ],
  )

  /* ------------------------------------------------------------- dragging */

  /** The id the arrangement uses for a row, or `undefined` for anything else. */
  const entryIdOfKey = useCallback((key: React.Key | undefined): string | undefined => {
    const ref = decodeNode(String(key))
    if (!ref) return undefined
    return ref.t === 'group' ? ref.groupId : ref.t === 'connection' ? ref.connectionId : undefined
  }, [])

  /**
   * Where the pointer is while a row is being dragged.
   *
   * A row's drop handler says "below *this* row" and never says whether the
   * pointer is still on that row's lower half or already past it — and on a
   * folder's last row those are two different drops (see `dropTargetFor`). The
   * pane listens for `dragover` in the capture phase, so this is the position of
   * the event that is about to be turned into a drop decision, not one from
   * before it.
   */
  const dragPoint = useRef<{ x: number; y: number } | null>(null)

  /** The entry a drag carries, so a drop in the empty space knows what moves. */
  const draggedId = useRef<string | null>(null)

  /** The row a key is drawn in, so "on the row" can be told from "under it". */
  const rowElement = useCallback((key: string): HTMLElement | undefined => {
    const rows = treePaneRef.current?.querySelectorAll<HTMLElement>('[data-tree-key]') ?? []
    for (const title of rows) {
      if (title.getAttribute('data-tree-key') === key) {
        return title.closest<HTMLElement>('.ant-tree-treenode') ?? title
      }
    }
    return undefined
  }, [])

  /** Whether the pointer has gone past the bottom edge of the row it is over. */
  const belowRow = useCallback(
    (key: string): boolean => {
      const point = dragPoint.current
      const row = rowElement(key)
      if (!point || !row) return false
      return point.y > row.getBoundingClientRect().bottom
    },
    [rowElement],
  )

  /**
   * Whether the pointer is in the pane's empty space, under its last row.
   *
   * There are no rows down there, so no row handler runs and the tree reports
   * nothing at all: the one gesture that means "out of the list" — let go below
   * it — would do nothing. The pane answers for that space itself.
   */
  const overEmptySpace = useCallback((event: React.DragEvent): boolean => {
    const pane = treePaneRef.current
    if (!pane) return false
    const rows = pane.querySelectorAll<HTMLElement>('.ant-tree-treenode')
    const last = rows[rows.length - 1]
    if (!last || event.clientY <= last.getBoundingClientRect().bottom) return false
    const under = document.elementFromPoint(event.clientX, event.clientY)
    return Boolean(under && pane.contains(under) && !under.closest('.ant-tree-treenode'))
  }, [])

  /**
   * Where a drop would land, or `undefined` when the arrangement cannot express
   * it.
   *
   * `allowDrop` and `onDrop` both come through here, so the hint the user
   * follows is the same decision as the move that happens: a folder inside a
   * folder is refused *before* the drop, rather than shown as allowed and then
   * swallowed. A drop among things the arrangement does not know — a database, a
   * schema, an object — lands nowhere at all.
   */
  const resolveDrop = useCallback(
    (dragId: string, dropKey: string, relative: number, below: boolean): DropTarget | undefined => {
      const dropId = entryIdOfKey(dropKey)
      if (!dropId) return undefined
      return dropTargetFor(arrangement, dragId, dropId, relative, below)
    },
    [arrangement, entryIdOfKey],
  )

  /**
   * Moves the drop hint to the level the drop will really land at.
   *
   * The tree draws its line at the indentation of the row it settled on and has
   * no way to say "one level out", but that is exactly the drop under a folder's
   * last row: the line would sit under the member while the connection lands
   * beside the folder. Pulling it back by the width the tree is really indented
   * with — measured off the tree, not assumed — is what keeps the hint honest.
   */
  const markDropLevel = useCallback(
    (target: DropTarget | undefined, ref: NodeRef | undefined) => {
      const pane = treePaneRef.current
      if (!pane) return
      const outdent =
        target?.t === 'root' &&
        ref?.t === 'connection' &&
        groupOf(arrangement, ref.connectionId) !== undefined
      pane.classList.toggle('is-drop-outdent', outdent)
      if (!outdent) return
      const unit = pane.querySelector<HTMLElement>('.ant-tree-indent-unit')
      if (unit) pane.style.setProperty('--dm-tree-indent', `${unit.offsetWidth}px`)
    },
    [arrangement],
  )

  /** Drops the drag state the pane keeps outside React: hint, pointer, cargo. */
  const clearDragHints = useCallback(() => {
    dragPoint.current = null
    draggedId.current = null
    treePaneRef.current?.classList.remove('is-drop-outdent', 'is-drop-end')
  }, [])

  const allowDrop = useCallback<NonNullable<TreeProps['allowDrop']>>(
    ({ dragNode, dropNode, dropPosition }) => {
      const dragId = entryIdOfKey(dragNode.key)
      const dropKey = String(dropNode.key)
      const target =
        dragId && dragId !== entryIdOfKey(dropNode.key)
          ? resolveDrop(dragId, dropKey, dropPosition, belowRow(dropKey))
          : undefined
      markDropLevel(target, decodeNode(dropKey) ?? undefined)
      return target !== undefined
    },
    [belowRow, entryIdOfKey, markDropLevel, resolveDrop],
  )

  const handleDrop = useCallback<NonNullable<TreeProps['onDrop']>>(
    (info) => {
      const dragId = entryIdOfKey(info.dragNode?.key)
      // rc-tree reports where the drop landed as an index among the parent's
      // children; what the hint meant is the same -1 / 0 / +1 relative to the
      // node it names that `allowDrop` was asked about.
      const pos = String((info.node as TreeDataNode & { pos?: string }).pos ?? '')
      const relative = info.dropToGap
        ? info.dropPosition - Number(pos.split('-').pop())
        : 0
      const dropKey = String(info.node.key)
      const target = dragId ? resolveDrop(dragId, dropKey, relative, belowRow(dropKey)) : undefined
      clearDragHints()
      if (!dragId || !target) return
      void moveConnection(dragId, target).catch((error) => message.error(toMessage(error)))
    },
    [belowRow, clearDragHints, entryIdOfKey, message, moveConnection, resolveDrop],
  )

  const handleDragStart = useCallback<NonNullable<TreeProps['onDragStart']>>(
    ({ node }) => {
      draggedId.current = entryIdOfKey(node.key) ?? null
    },
    [entryIdOfKey],
  )

  /** The pane's own half of dragging: where the pointer is, and what it is over. */
  const handlePaneDrag = useCallback(
    (event: React.DragEvent) => {
      dragPoint.current = { x: event.clientX, y: event.clientY }
      const pane = treePaneRef.current
      // Only a row of this pane can be let go in its empty space; a file dragged
      // in from outside is none of the pane's business.
      const empty = draggedId.current !== null && overEmptySpace(event)
      pane?.classList.toggle('is-drop-end', empty)
      if (empty) pane?.classList.remove('is-drop-outdent')
      // A row prevents the default itself; the empty space has nobody to do it
      // for it, and without that nothing can be dropped there.
      if (empty) event.preventDefault()
    },
    [overEmptySpace],
  )

  const handlePaneDrop = useCallback(
    (event: React.DragEvent) => {
      const dragId = draggedId.current
      if (!dragId || !overEmptySpace(event)) return
      event.preventDefault()
      event.stopPropagation()
      clearDragHints()
      const target = endOfTopLevelTarget(arrangement, dragId)
      if (!target) return
      void moveConnection(dragId, target).catch((error) => message.error(toMessage(error)))
    },
    [arrangement, clearDragHints, message, moveConnection, overEmptySpace],
  )

  /**
   * Only the rows the arrangement holds are draggable.
   *
   * A filtered tree draws just what matched, so the rows around a drop are not
   * the rows the arrangement holds and a position among them means nothing;
   * dragging is off while the filter is on rather than quietly misplaced.
   */
  const nodeDraggable = useCallback(
    (node: TreeDataNode): boolean => needle === '' && entryIdOfKey(node.key) !== undefined,
    [entryIdOfKey, needle],
  )

  const handleSelect = useCallback<NonNullable<TreeProps['onSelect']>>(
    (keys, info) => {
      setSelectedKeys(keys as React.Key[])
      const ref = decodeNode(String(info.node.key))
      if (!ref) return

      if (ref.t === 'connection') {
        // Picking a profile is what hands it to the ribbon: a closed one lights
        // up Open, an open one lights up Close.
        setActiveConnection(ref.connectionId)
        // A connection is not a database, so the ribbon's object buttons return
        // to "nothing picked" — even when there is a live session behind it.
        setActiveNamespace()
        const session = sessionForConnection(ref.connectionId)
        if (session) setActiveSession(session.id)
        return
      }

      if (ref.t === 'group') {
        // A folder names no connection and no database, so the ribbon's object
        // buttons return to "nothing picked" — while the connection picked last
        // stays focused, so Open / Close / Refresh keep working on it.
        setActiveNamespace()
        return
      }

      setActiveSession(ref.sessionId)
      const owner = sessions.find((s) => s.id === ref.sessionId)
      if (owner?.connectionId) setActiveConnection(owner.connectionId)
      // What the ribbon's Table / View buttons act on: the namespace the picked
      // node sits in. Both levels that hold objects count — the schema where the
      // engine has them, the database where it does not — so a database on an
      // engine with a schema layer is noted without one. A node that names no
      // objects at all clears the note.
      if (ref.t === 'session') {
        setActiveNamespace()
      } else if (ref.t === 'db') {
        setActiveNamespace(
          driverOfSession(ref.sessionId)?.supportsSchema
            ? { sessionId: ref.sessionId, database: ref.database }
            : { sessionId: ref.sessionId, database: ref.database, schema: ref.database },
        )
      } else if ('schema' in ref) {
        setActiveNamespace({
          sessionId: ref.sessionId,
          database: ref.database,
          schema: ref.schema,
        })
      } else {
        // The Queries folder and a saved script sit at the database level, the
        // same way a database's own folders do on an engine without schemas.
        setActiveNamespace(
          driverOfSession(ref.sessionId)?.supportsSchema
            ? { sessionId: ref.sessionId, database: ref.database }
            : { sessionId: ref.sessionId, database: ref.database, schema: ref.database },
        )
      }
      if (ref.t === 'folder' || ref.t === 'indexFolder') {
        // An empty folder is a leaf (the (0) in its title is the whole story),
        // so there is nothing to reveal and expanding it is skipped.
        if (!info.node.isLeaf) {
          // Navicat reveals the folder's contents and its object list together.
          const key = info.node.key
          setExpandedKeys((current) =>
            current.some((k) => String(k) === String(key)) ? current : [...current, key],
          )
        }
        openList(ref.sessionId, ref.database, ref.schema, ref.t === 'folder' ? ref.kind : 'index')
      }
      if (ref.t === 'queries') {
        // There is no object list behind this folder — its contents are the rows
        // below it — so selecting it only opens it.
        if (!info.node.isLeaf) {
          const key = info.node.key
          setExpandedKeys((current) =>
            current.some((k) => String(k) === String(key)) ? current : [...current, key],
          )
        }
      }
      if (ref.t === 'queryFile') {
        openQueryFileTab(ref.sessionId, ref.database, ref.name)
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
    [
      driverOfSession,
      openList,
      openObject,
      openQueryFileTab,
      sessionForConnection,
      setActiveConnection,
      setActiveNamespace,
      setActiveSession,
      tree.objects,
    ],
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
      for (const key of fresh) openedByUser.current.add(String(key))
      const retry = new Set(fresh.map(String))
      setLoadedKeys((current) => {
        const kept = current.filter((key) => !retry.has(String(key)) || hasData(String(key)))
        return kept.length === current.length ? current : kept
      })
    },
    [expandedKeys, hasData],
  )

  // A double-click that had to open the connection first: wait for the session
  // to show up, then reveal its runtime page.
  useEffect(() => {
    const wanted = pendingRuntime.current
    if (!wanted) return
    const session = sessionForConnection(wanted)
    if (!session) return
    pendingRuntime.current = null
    openRuntimeTab(session.id)
  }, [openRuntimeTab, sessionForConnection, sessions])

  /**
   * Scrolls the explorer row with this tree key into view, if it is on screen.
   *
   * Answers whether it found the row: a row whose namespace is still loading is
   * not drawn yet, and the caller gets another chance on a later render.
   */
  const scrollToRow = useCallback(
    (key: string) => {
      const row = rowElement(key)
      row?.scrollIntoView({ block: 'nearest' })
      return Boolean(row)
    },
    [rowElement],
  )

  /**
   * Shows the explorer the folder a list window was opened from.
   *
   * The request comes from the ribbon buttons and from the tab strip, never the
   * other way round: only the tree's own selection says where the user stands,
   * so a programmatic pick cannot feed back into it. Every level below the
   * connection loads itself once its node is expanded (`loadData` in the tree),
   * which is why this only has to expand the way down and wait — the folder row
   * cannot exist before the namespace's object list has landed.
   */
  useEffect(() => {
    if (!reveal) return
    const session = sessions.find((s) => s.id === reveal.sessionId)
    // A window whose session is gone has no row to point at.
    if (!session) {
      clearReveal()
      return
    }

    const driver = driverOfSession(session.id)
    const dbKey = encodeNode({ t: 'db', sessionId: session.id, database: reveal.database })
    const nsRef: NodeRef = driver?.supportsSchema
      ? { t: 'schema', sessionId: session.id, database: reveal.database, schema: reveal.schema }
      : { t: 'db', sessionId: session.id, database: reveal.database }
    const nsKey = encodeNode(nsRef)
    const folderKey = encodeNode(
      reveal.kind === 'index'
        ? {
            t: 'indexFolder',
            sessionId: session.id,
            database: reveal.database,
            schema: reveal.schema,
          }
        : {
            t: 'folder',
            sessionId: session.id,
            database: reveal.database,
            schema: reveal.schema,
            kind: reveal.kind,
          },
    )

    const open = new Set(expandedKeys.map(String))
    const connectionId = session.connectionId ?? session.id
    const groupId = groupOf(arrangement, connectionId)
    const path = [
      // A connection in a folder is only drawn once the folder is open.
      ...(groupId ? [encodeNode({ t: 'group', groupId })] : []),
      encodeNode({ t: 'connection', connectionId }),
      dbKey,
      nsKey,
    ].filter((key) => !open.has(String(key)))
    if (path.length > 0) setExpandedKeys([...expandedKeys, ...path])

    // Each level has to report before the one below it exists, so the wait is
    // per level. A level that answers with a "no" — an error, or a list that
    // does not hold what the window is scoped to — ends the request: there will
    // never be a row to point at, and holding on to it would let a stale request
    // hijack the explorer the next time the namespace does load.
    if (tree.errors[session.id] || tree.errors[dbKey]) {
      clearReveal()
      return
    }
    const databases = tree.databases[session.id]
    if (!databases) return
    if (!databases.includes(reveal.database)) {
      clearReveal()
      return
    }
    if (driver?.supportsSchema) {
      const schemas = tree.schemas[dbKey]
      if (!schemas) return
      if (!schemas.includes(reveal.schema)) {
        clearReveal()
        return
      }
    }

    const ns = namespaceKey(session.id, reveal.database, reveal.schema)
    if (tree.errors[ns]) {
      clearReveal()
      return
    }
    if (!tree.loaded[ns]) return

    // A folder for a kind the engine does not declare is only drawn once it
    // holds something (see `buildFolders`), so an empty one is nothing to show.
    const objects = tree.objects[objectsKey(session.id, reveal.database, reveal.schema)] ?? []
    const drawn =
      reveal.kind === 'index' ||
      objectKindsOf(driver).includes(reveal.kind) ||
      objects.some((object) => object.kind === reveal.kind)
    if (!drawn) {
      clearReveal()
      return
    }

    setSelectedKeys([folderKey])
    // The row is usually already drawn (the namespace was open); when it took
    // this request to draw it, the scroll waits for the render that does.
    if (!scrollToRow(String(folderKey))) revealScroll.current = String(folderKey)
    clearReveal()
  }, [
    arrangement,
    clearReveal,
    driverOfSession,
    expandedKeys,
    reveal,
    scrollToRow,
    sessions,
    tree.databases,
    tree.errors,
    tree.loaded,
    tree.objects,
    tree.schemas,
  ])

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

  // Brings a revealed row into view as soon as it is on screen. A row that is
  // still loading stays pending, and is scrolled to by the render that draws it.
  useEffect(() => {
    const key = revealScroll.current
    if (key && scrollToRow(key)) revealScroll.current = null
  }, [scrollToRow, treeData])

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

  // Right-clicking empty space is already the "new connection" gesture, so the
  // menu lists the drivers directly instead of asking a second time. What else
  // this pane can make — a group — rides along at the bottom of the same list,
  // which is why the header `+` shows it too.
  const newMenuItems: MenuProps['items'] = [
    ...(connectionTypeItems(drivers) ?? []),
    { type: 'divider' as const },
    { key: NEW_GROUP_KEY, icon: <FolderAddOutlined />, label: 'New group…' },
  ]

  /** Both anchors dispatch through here, so a key means one thing in each. */
  const handleNewMenuKey = useCallback(
    (key: string) => {
      if (key === NEW_GROUP_KEY) {
        setGroupDialog({})
        return
      }
      const driver = driverFromKey(key)
      if (driver) openEditor({ driver })
    },
    [openEditor],
  )

  return (
    <div className="dm-sidebar" onContextMenu={openBlankMenu}>
      <div className="dm-sidebar-header">
        <span className="dm-sidebar-title">Connections</span>
        <Dropdown
          trigger={['click']}
          placement="bottomLeft"
          rootClassName="dm-type-menu"
          menu={{ items: newMenuItems, onClick: ({ key }) => handleNewMenuKey(key) }}
        >
          <span className="dm-dropdown-anchor">
            <Tooltip title="New connection or group">
              <Button size="small" type="text" icon={<PlusOutlined />} />
            </Tooltip>
          </span>
        </Dropdown>
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

      <div
        className="dm-sidebar-tree"
        ref={treePaneRef}
        onDragEnterCapture={handlePaneDrag}
        onDragOverCapture={handlePaneDrag}
        onDropCapture={handlePaneDrop}
        onDragEndCapture={clearDragHints}
      >
        {arrangement.length === 0 && looseSessions.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={<span style={{ fontSize: 12 }}>No connections yet</span>}
            style={{ marginTop: 40 }}
          >
            <ConnectionTypeDropdown>
              <Button type="primary" size="small" icon={<PlusOutlined />}>
                New connection
              </Button>
            </ConnectionTypeDropdown>
          </Empty>
        ) : treeData.length === 0 ? (
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
            draggable={{ icon: false, nodeDraggable }}
            allowDrop={allowDrop}
            onDragStart={handleDragStart}
            onDragEnd={clearDragHints}
            onDrop={handleDrop}
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

      <CopyTableModal
        source={copyTable}
        onClose={() => setCopyTable(null)}
        onFinished={(target, withData, result) => {
          if (!copyTable) return
          const { sessionId, database, schema } = copyTable
          // The catalog moved either way: a copy that failed halfway may still
          // have left its table (and some of its indexes) behind.
          void loadObjects(sessionId, database, schema)
          void loadIndexes(sessionId, database, schema)
          if (result.error) return
          // The window has done its job, so the copy is what the user is left
          // looking at — on the rows that came with it, or on its structure
          // when all that was asked for was the shape of the table.
          openTableTab(
            sessionId,
            database,
            schema,
            { name: target, kind: 'table', rowEstimate: 0, sizeBytes: 0 },
            withData ? 'data' : 'structure',
          )
        }}
      />

      <GroupNameModal request={groupDialog} onClose={() => setGroupDialog(null)} />

      <QueryNameModal
        request={queryName}
        onClose={() => setQueryName(null)}
        onSubmit={async (name) => {
          if (!queryName) return
          const { sessionId, database, rename } = queryName
          const connectionId = sessions.find((s) => s.id === sessionId)?.connectionId
          if (!connectionId) throw new Error('This connection is no longer open.')
          if (rename) {
            await renameQueryFile({ connectionId, database, from: rename, to: name })
          } else {
            await createQueryFile(sessionId, database, name, '')
          }
          // Opening the window is the point of both: a new script is made to be
          // written in, and a renamed one should not vanish from under the user.
          openQueryFileTab(sessionId, database, name)
        }}
      />

      {blankMenu ? (
        <div
          ref={blankMenuRef}
          className="dm-blank-menu"
          style={{ left: blankMenu.x, top: blankMenu.y }}
        >
          <Menu
            items={newMenuItems}
            selectable={false}
            onClick={({ key }) => {
              setBlankMenu(null)
              handleNewMenuKey(key)
            }}
          />
        </div>
      ) : null}
    </div>
  )

  /* --------------------------------------------------------------- menus */

  /**
   * A saved script's menu.
   *
   * Delete asks first and names what it is about to remove: it is the one item
   * here that cannot be taken back from inside the app (the file is gone, and
   * the editor that had it open would be editing nothing).
   */
  function queryFileMenuItems(
    session: SessionInfo,
    database: string,
    file: QueryFile,
  ): MenuProps['items'] {
    return [
      {
        key: 'open',
        icon: <EditOutlined />,
        label: 'Open',
        onClick: () => openQueryFileTab(session.id, database, file.name),
      },
      {
        key: 'rename',
        icon: <FileTextOutlined />,
        label: 'Rename…',
        onClick: () => setQueryName({ sessionId: session.id, database, rename: file.name }),
      },
      {
        key: 'copy',
        icon: <NumberOutlined />,
        label: 'Copy name',
        onClick: () => void copyText(file.name),
      },
      { type: 'divider' as const },
      {
        key: 'reveal',
        icon: <FolderOutlined />,
        label: 'Show in folder',
        onClick: () => void api.revealInExplorer(file.path).catch(() => undefined),
      },
      {
        key: 'delete',
        icon: <DeleteOutlined />,
        danger: true,
        label: 'Delete',
        onClick: () => {
          modal.confirm({
            title: `Delete “${file.name}”?`,
            content: `The script is removed from ${database}. A window that has it open keeps the text that is in it, but there is no longer a file behind it — saving from there would write it again.`,
            okText: 'Delete',
            okButtonProps: { danger: true },
            onOk: () => deleteQueryFile(session.id, database, file.name),
          })
        },
      },
    ]
  }

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

  /**
   * A double-click on a connection asks for its runtime status.
   *
   * When the session is up this is a plain tab switch. When it is not, the
   * connection is opened first and the page follows once it exists — the click
   * cannot wait for that itself, because a profile without a stored password
   * opens through a password prompt that finishes much later.
   */
  function openRuntime(root: RootEntry, session: SessionInfo | undefined) {
    if (session) {
      openRuntimeTab(session.id)
      return
    }
    if (!root.profile) {
      message.info(`${root.name} has no saved connection to open.`)
      return
    }
    pendingRuntime.current = root.id
    void connect(root.profile)
  }

  /**
   * A group's own items carry their own handlers; the nested driver list is what
   * needs following, and a new connection made there belongs in this folder.
   */
  function handleGroupMenuKey(entry: ExplorerEntry & { t: 'group' }, key: string) {
    if (!key.startsWith(GROUP_PREFIX)) return
    const driver = driverFromKey(key)
    if (driver) openEditor({ driver, groupId: entry.id })
  }

  /**
   * A group's menu: make a connection inside it, rename it, delete it.
   *
   * A user who makes a folder and then makes a connection means it to be in the
   * folder, so the same driver list is nested here. `driverFromKey` reads only
   * the last segment, so the keys reach it as `group.mysql`, and the menu-level
   * handler follows them for this folder alone.
   */
  function groupMenuItems(entry: ExplorerEntry & { t: 'group' }): MenuProps['items'] {
    return [
      {
        key: 'new',
        icon: <PlusOutlined />,
        label: 'New connection…',
        children: connectionTypeItems(drivers, 'group.'),
      },
      { type: 'divider' as const },
      {
        key: 'rename',
        icon: <EditOutlined />,
        label: 'Rename group…',
        onClick: () => setGroupDialog({ group: { id: entry.id, name: entry.name } }),
      },
      {
        key: 'delete',
        icon: <DeleteOutlined />,
        label: 'Delete group',
        danger: true,
        onClick: () => confirmDeleteGroup(entry),
      },
    ]
  }

  /**
   * Deleting a group never deletes what is in it — its connections go back to
   * the top level — so the confirmation says so, and says how many there are
   * instead of leaving the user to count rows.
   */
  function confirmDeleteGroup(entry: ExplorerEntry & { t: 'group' }) {
    const count = entry.members.length
    modal.confirm({
      title: `Delete “${entry.name}”?`,
      content:
        count > 0
          ? `Its ${count} connection${count === 1 ? '' : 's'} move back to the top level. No stored connection is deleted.`
          : 'The group is empty, so nothing else changes.',
      okText: 'Delete',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await deleteConnectionGroup(entry.id)
        } catch (error) {
          message.error(toMessage(error))
        }
      },
    })
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
          disabled: !capabilitiesOf(root.driver).createDatabase || session.readOnly,
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
          // No confirmation: the profile is stored, so connecting again is one
          // click away, and only the tabs of this connection are closed.
          onClick: () => void closeSession(session.id),
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
    case 'tidb':
      return '#d8391b'
    case 'doris':
      return '#3d6ce0'
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

/**
 * One tree row's context menu.
 *
 * The overlay is drawn through a portal, but React keeps bubbling its events up
 * the *component* tree, and this one hangs off a tree row — so without care a
 * click on an item also reaches the row's own click handler, which re-opens the
 * object on its data page and undoes whatever the item just asked for
 * ("Design table" would land on Data, "New query" on a folder would open the
 * object list). Stopping the event at the menu keeps a menu click a menu click.
 */
function NodeMenu({
  items,
  children,
  onClick,
}: {
  items: MenuProps['items']
  children: ReactNode
  /** For menus that hold more than one kind of item — a group's driver submenu. */
  onClick?: MenuProps['onClick']
}) {
  return (
    <Dropdown
      menu={{
        items,
        onClick: (info) => {
          info.domEvent.stopPropagation()
          onClick?.(info)
        },
      }}
      trigger={['contextMenu']}
    >
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
 * What the window asks for depends on the engine, and the engine is asked, not
 * assumed: MySQL and TiDB answer with the character sets and collations their
 * server supports, PostgreSQL with the encodings and locales it accepts, Doris
 * and MongoDB with nothing to choose at all. The statement is rendered by the
 * backend from the very same request — shown verbatim before it runs and then
 * executed as that string — so the preview cannot disagree with what happens,
 * and the frontend never assembles DDL of its own.
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
  const [charset, setCharset] = useState<string | undefined>()
  const [collation, setCollation] = useState<string | undefined>()
  const [options, setOptions] = useState<DatabaseOptions | null>(null)
  const [optionsError, setOptionsError] = useState<string | null>(null)
  const [reading, setReading] = useState(false)
  const [plan, setPlan] = useState<DatabasePlan | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const sessionId = session?.id

  // Every way out clears the dialog, so it never reopens on a stale choice.
  const close = () => {
    setName('')
    setCharset(undefined)
    setCollation(undefined)
    setOptions(null)
    setOptionsError(null)
    setPlan(null)
    setError(null)
    setBusy(false)
    onClose()
  }

  // What this server accepts, read once per opening. A failure here is not
  // fatal — the statement is rendered by the backend anyway — so it is shown
  // next to the form and the window keeps working with the name alone.
  useEffect(() => {
    if (!sessionId) return
    let cancelled = false
    setReading(true)
    setOptionsError(null)
    api
      .databaseOptions(sessionId)
      .then((next) => {
        if (cancelled) return
        setOptions(next)
        const preferred = next.charsets.find((entry) => entry.default) ?? next.charsets[0]
        if (preferred) {
          setCharset(preferred.name)
          setCollation(preferred.collation || next.collations?.[0])
        } else {
          setCollation(next.collations?.[0])
        }
      })
      .catch((err) => {
        if (!cancelled) setOptionsError(toMessage(err))
      })
      .finally(() => {
        if (!cancelled) setReading(false)
      })
    return () => {
      cancelled = true
    }
  }, [sessionId])

  // The preview is the backend's own statement, debounced so a name being typed
  // does not turn into a call per keystroke.
  useEffect(() => {
    const database = name.trim()
    if (!sessionId || !database) {
      setPlan(null)
      setError(null)
      return
    }
    let cancelled = false
    const timer = window.setTimeout(() => {
      api
        .planCreateDatabase(sessionId, { name: database, charset, collation })
        .then((next) => {
          if (cancelled) return
          setPlan(next)
          setError(null)
        })
        .catch((err) => {
          if (cancelled) return
          setPlan(null)
          setError(toMessage(err))
        })
    }, 250)
    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [sessionId, name, charset, collation])

  const charsetEntry = options?.charsets.find((entry) => entry.name === charset)
  const collations = charsetEntry?.collations?.length
    ? charsetEntry.collations
    : options?.collations ?? []
  const showCollation = collations.length > 0 || options?.collationEditable

  async function submit() {
    const database = name.trim()
    if (!session || !database || busy) return
    setBusy(true)
    setError(null)
    try {
      // Planned again on submit rather than reusing the debounced preview: what
      // runs is always the statement rendered from the fields as they are now.
      const rendered = await api.planCreateDatabase(session.id, {
        name: database,
        charset,
        collation,
      })
      await api.executeSql({
        sessionId: session.id,
        sql: rendered.statement,
        timeoutMs: 60000,
      })
      // An engine whose statement does not literally create the database
      // (MongoDB's `use`) explains itself instead of claiming success.
      if (rendered.warnings?.length) {
        message.info(rendered.warnings.join(' '), 8)
      } else {
        message.success(`Database ${database} created`)
      }
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
      okButtonProps={{ disabled: !name.trim() || error !== null }}
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

      {reading || options?.charsets.length ? (
        <>
          <label className="dm-field-label" htmlFor="dm-new-database-charset" style={{ marginTop: 12 }}>
            {options?.charsetLabel || 'Character set'}
          </label>
          <Select
            id="dm-new-database-charset"
            style={{ width: '100%' }}
            value={charset}
            loading={reading}
            placeholder="Server default"
            allowClear
            onChange={(value: string | undefined) => {
              setCharset(value)
              // The collation belongs to the character set, so it follows it,
              // falling back to the new charset's own default.
              const entry = options?.charsets.find((item) => item.name === value)
              setCollation(entry?.collation || entry?.collations?.[0])
            }}
            options={(options?.charsets ?? []).map((entry) => ({
              value: entry.name,
              label: entry.default ? `${entry.name} (server default)` : entry.name,
            }))}
          />
        </>
      ) : null}

      {showCollation ? (
        <>
          <label className="dm-field-label" htmlFor="dm-new-database-collation" style={{ marginTop: 12 }}>
            {options?.collationLabel || 'Collation'}
          </label>
          {options?.collationEditable ? (
            // PostgreSQL locale names come from the server's operating system,
            // so the list can only ever be a suggestion.
            <AutoComplete
              id="dm-new-database-collation"
              style={{ width: '100%' }}
              value={collation}
              options={collations.map((entry) => ({ value: entry }))}
              placeholder="en_US.UTF-8"
              allowClear
              onChange={(value: string) => setCollation(value || undefined)}
            />
          ) : (
            <Select
              id="dm-new-database-collation"
              style={{ width: '100%' }}
              value={collation}
              placeholder="Server default"
              allowClear
              onChange={(value: string | undefined) => setCollation(value)}
              options={collations.map((entry) => ({ value: entry, label: entry }))}
            />
          )}
        </>
      ) : null}

      {optionsError ? (
        <Typography.Paragraph type="secondary" style={{ marginTop: 12, marginBottom: 0 }}>
          {optionsError} The name alone still works.
        </Typography.Paragraph>
      ) : null}

      {error ? (
        <Typography.Paragraph type="danger" style={{ marginTop: 12, marginBottom: 0 }}>
          {error}
        </Typography.Paragraph>
      ) : (
        <Typography.Paragraph type="secondary" style={{ marginTop: 12, marginBottom: 0 }}>
          {plan ? (
            <SqlCode inline sql={plan.statement} driver={session?.driver} />
          ) : (
            'Name it and the statement appears here.'
          )}
        </Typography.Paragraph>
      )}

      {plan?.warnings?.map((warning) => (
        <Typography.Paragraph key={warning} type="warning" style={{ marginTop: 8, marginBottom: 0 }}>
          {warning}
        </Typography.Paragraph>
      ))}
      {options?.hint ? (
        <Typography.Paragraph type="secondary" style={{ marginTop: 8, marginBottom: 0, fontSize: 12 }}>
          {options.hint}
        </Typography.Paragraph>
      ) : null}
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

/**
 * Names a group, for both "New group…" and "Rename group…".
 *
 * The name is the only thing either asks for — a rename keeps the position the
 * folder already has, because typing a name is not a request to move it. What
 * counts as a name is the backend's call (`internal/service/layout.go`), so its
 * answer is what the user sees when one is refused.
 */
function GroupNameModal({
  request,
  onClose,
}: {
  request: { group?: { id: string; name: string } } | null
  onClose: () => void
}) {
  const createConnectionGroup = useAppStore((s) => s.createConnectionGroup)
  const renameConnectionGroup = useAppStore((s) => s.renameConnectionGroup)
  const renaming = request?.group

  return (
    <NamePromptModal
      open={Boolean(request)}
      title={renaming ? 'Rename group' : 'New group'}
      okText={renaming ? 'Rename' : 'Create'}
      placeholder="Group name"
      initial={renaming?.name ?? ''}
      onClose={onClose}
      onSubmit={async (name) => {
        if (renaming) await renameConnectionGroup(renaming.id, name)
        else await createConnectionGroup(name)
      }}
    />
  )
}

/**
 * Names a saved script: a new one, or a rename of one that exists.
 *
 * The window is deliberately thin — it hands the name over and lets the caller
 * decide what that means, because "new" and "rename" differ only in what happens
 * to the name and both end by opening the file.
 */
function QueryNameModal({
  request,
  onSubmit,
  onClose,
}: {
  request: { sessionId: string; database: string; rename?: string } | null
  onSubmit: (name: string) => Promise<void>
  onClose: () => void
}) {
  const renaming = request?.rename

  return (
    <NamePromptModal
      open={Boolean(request)}
      title={renaming ? 'Rename query' : `New query in ${request?.database ?? ''}`}
      okText={renaming ? 'Rename' : 'Create'}
      placeholder="Query name"
      initial={renaming ?? ''}
      hint="The script is saved as a .sql file inside the data folder, so a later rename keeps it in one piece."
      onClose={onClose}
      onSubmit={onSubmit}
    />
  )
}
