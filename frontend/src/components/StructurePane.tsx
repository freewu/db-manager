import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Alert,
  App as AntApp,
  Button,
  Empty,
  Input,
  Space,
  Spin,
  Splitter,
  Table,
  Tag,
  Tooltip,
  Typography,
} from 'antd'
import type { TableColumnsType } from 'antd'
import {
  CaretRightOutlined,
  CopyOutlined,
  DeleteOutlined,
  DownloadOutlined,
  EditOutlined,
  PlusOutlined,
  ReloadOutlined,
  WarningOutlined,
} from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type {
  ColumnInfo,
  DesignPlan,
  DriverType,
  ForeignKeyInfo,
  IndexInfo,
  TableStructure,
} from '../api/types'
import { capabilitiesOf } from '../lib/capabilities'
import { emptyColumn, emptyIndex, isDirty, primaryKeyRow } from '../lib/design'
import { useAppStore, type WorkspaceTab } from '../store/appStore'
import { FieldGrid, IndexGrid } from './DesignGrid'
import { SqlCode } from './SqlCode'

/** Which slice of the structure a table window is showing. */
export type StructureSection = 'structure' | 'indexes' | 'foreignKeys' | 'ddl'

interface StructureViewProps {
  tab: WorkspaceTab
  /** Which slice to render. */
  section: StructureSection
  /** Bumping this value forces a reload. */
  reloadToken?: number
  /**
   * Design a table that does not exist yet.
   *
   * There is no catalog structure to fetch and nothing to compare the draft
   * against; the draft *is* the whole definition and Save runs CREATE TABLE
   * instead of ALTER TABLE.
   */
  creating?: boolean
}

/**
 * One slice of an object's definition.
 *
 * The whole structure is fetched once and cached for the lifetime of the
 * window, so switching between Columns / Indexes / Foreign keys / DDL is
 * instant and costs no extra round trip.
 *
 * The Columns slice is the table designer: it edits a draft of the table, and
 * the SQL that would turn the live table into that draft is previewed next to
 * it. Nothing runs until Save is pressed.
 */
export function StructureView({ tab, section, reloadToken = 0, creating = false }: StructureViewProps) {
  const { message, modal } = AntApp.useApp()
  const appInfo = useAppStore((s) => s.appInfo)
  const session = useAppStore((s) => s.sessionOf(tab.sessionId))
  const drivers = useAppStore((s) => s.drivers)
  const design = useAppStore((s) => s.designs[tab.id])
  const ensureDesign = useAppStore((s) => s.ensureDesign)
  const updateDesign = useAppStore((s) => s.updateDesign)
  const openDdlTab = useAppStore((s) => s.openDdlTab)

  const database = tab.database ?? session?.database ?? ''
  const schema = tab.schema ?? ''
  // A new table has no catalog name yet: the name being typed in the designer
  // is the one every statement will be rendered with.
  const object = creating ? (design?.draft.object ?? '') : (tab.object ?? '')
  const readOnly = Boolean(session?.readOnly)
  // A document store has nothing to design: its fields are whatever the
  // documents happen to contain, so the Columns slice degrades to the sampled
  // field list the backend reports and no design is ever planned. `designable`
  // is the narrower question — an engine can have columns and still not be
  // editable by this designer (Doris needs a data model and a distribution
  // clause), and then the same read-only field list is the honest answer.
  const { relational, designable } = capabilitiesOf(
    drivers.find((d) => d.type === session?.driver),
  )

  const [fetched, setFetched] = useState<TableStructure | null>(null)
  // Creating compares the draft against an empty baseline instead of the live
  // catalog, so there is nothing to fetch and nothing to reload.
  const structure = creating ? (design?.baseline ?? null) : fetched
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [attempt, setAttempt] = useState(0)
  const [plan, setPlan] = useState<DesignPlan | null>(null)
  const [planError, setPlanError] = useState<string | null>(null)
  const [applying, setApplying] = useState(false)
  const [applyError, setApplyError] = useState<string | null>(null)
  const [selectedField, setSelectedField] = useState<number | null>(null)
  const [selectedIndex, setSelectedIndex] = useState<number | null>(null)
  // Set right before a reload that must throw the draft away (after a save).
  const rebase = useRef(false)

  useEffect(() => {
    if (creating) {
      // Nothing to read and no draft to prefill: the store opened this window
      // with an empty baseline and a fresh draft.
      setLoading(false)
      return undefined
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    api
      .getStructure(tab.sessionId, database, schema, object)
      .then((value) => {
        if (cancelled) return
        setFetched(value)
        if (designable) ensureDesign(tab.id, value, rebase.current)
        rebase.current = false
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setError(toMessage(err))
        setFetched(null)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [
    attempt,
    creating,
    database,
    designable,
    ensureDesign,
    object,
    reloadToken,
    schema,
    tab.id,
    tab.sessionId,
  ])

  const draft = design?.draft
  const draftKey = draft ? JSON.stringify(draft) : ''

  // Plan the draft as it stands, debounced: every keystroke in the grid would
  // otherwise be a catalog round trip.
  useEffect(() => {
    if (!designable || !draft || section !== 'structure') return
    // A table needs a name before anything can be rendered, and asking the
    // backend without one would only repeat “the new table needs a name”.
    if (creating && !draft.object.trim()) {
      setPlan(null)
      setPlanError(null)
      return
    }
    let cancelled = false
    const timer = window.setTimeout(() => {
      const planned = creating ? api.planCreateTable(draft) : api.planTableDesign(draft)
      planned
        .then((value) => {
          if (cancelled) return
          setPlan(value)
          setPlanError(null)
        })
        .catch((err: unknown) => {
          if (cancelled) return
          setPlan(null)
          setPlanError(toMessage(err))
        })
    }, 350)
    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [creating, draftKey, section])

  const dirty = useMemo(
    () => (creating ? Boolean(draft) : draft && structure ? isDirty(draft, structure) : false),
    [creating, draft, structure],
  )

  const reload = useCallback(() => setAttempt((n) => n + 1), [])

  const saveDDL = useCallback(async () => {
    if (!structure) return
    try {
      // The definition of a collection is a shell script, not SQL.
      const extension = relational ? 'sql' : 'js'
      await api.saveTextFile({
        defaultFilename: `${object}.${extension}`,
        content: structure.ddl,
        filters: [
          {
            displayName: relational ? 'SQL' : 'JavaScript',
            pattern: `*.${extension}`,
          },
        ],
      })
    } catch (err) {
      const text = toMessage(err)
      if (!/cancel/i.test(text)) message.error(text)
    }
  }, [message, object, relational, structure])

  const copy = useCallback(
    (text: string, what: string) => {
      void navigator.clipboard
        .writeText(text)
        .then(() => message.success(`${what} copied`))
        .catch((err: unknown) => message.error(toMessage(err)))
    },
    [message],
  )

  const apply = useCallback(
    (statements: string[]) => {
      if (!draft) return
      const name = draft.object.trim()
      modal.confirm({
        title: creating ? `Create table ${name}?` : 'Apply changes to this table?',
        width: 760,
        okText: creating ? 'Create' : 'Apply',
        content: (
          <div>
            <SqlCode
              className="dm-ddl"
              style={{ maxHeight: 280 }}
              sql={statements.map((statement) => `${statement};`).join('\n')}
              driver={session?.driver}
            />
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              The statements run one at a time, in this order.
            </Typography.Text>
          </div>
        ),
        onOk: async () => {
          setApplying(true)
          setApplyError(null)
          try {
            const result = creating
              ? await api.applyCreateTable(draft)
              : await api.applyTableDesign(draft)
            if (result.error) {
              setApplyError(
                `statement ${result.failedIndex + 1} of ${result.plan.statements.length} failed: ${result.error}`,
              )
              message.error(
                `Applied ${result.executed.length} of ${result.plan.statements.length} statement(s)`,
              )
            } else if (creating) {
              message.success(`Table ${name} created`)
            } else {
              message.success(
                `Table ${object} updated (${result.executed.length} statement(s))`,
              )
            }

            const state = useAppStore.getState()
            // The catalog moved either way, and after a partial CREATE the
            // table may exist with only some of its indexes.
            void state.loadObjects(tab.sessionId, database, schema)
            void state.loadIndexes(tab.sessionId, database, schema)

            if (creating) {
              if (result.error) return
              // The window has done its job, so it turns into the table it just
              // made — which is where the user wanted to end up.
              state.closeTab(tab.id)
              state.openTableTab(
                tab.sessionId,
                database,
                schema,
                { name, kind: 'table', rowEstimate: 0, sizeBytes: 0 },
                // The new table is empty, so its Structure tab is the useful
                // place to land — it shows what the CREATE just produced.
                'structure',
              )
              return
            }
            // The catalog may have moved: reload and rebase the draft.
            rebase.current = true
            setAttempt((n) => n + 1)
          } catch (err) {
            setApplyError(toMessage(err))
            message.error(toMessage(err))
          } finally {
            setApplying(false)
          }
        },
      })
    },
    [creating, database, draft, message, modal, object, schema, tab.id, tab.sessionId],
  )

  if (loading && !structure) {
    return (
      <div style={{ padding: 40, textAlign: 'center' }}>
        <Spin />
      </div>
    )
  }

  if (error) {
    return (
      <div style={{ padding: 16 }}>
        <Alert
          type="error"
          showIcon
          title="Could not read the structure"
          description={<span className="mono">{error}</span>}
          action={
            <Button size="small" icon={<ReloadOutlined />} onClick={reload}>
              Retry
            </Button>
          }
        />
      </div>
    )
  }

  if (!structure) {
    return <Empty description="No structure available" style={{ marginTop: 60 }} />
  }

  if (section === 'indexes') {
    return (
      <div className="dm-pane-body" style={{ padding: 12 }}>
        {structure.indexes.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No indexes" />
        ) : (
          <Table
            className="dm-grid"
            size="small"
            rowKey="name"
            columns={indexColumns}
            dataSource={structure.indexes}
            pagination={false}
            scroll={{ x: 'max-content' }}
          />
        )}
      </div>
    )
  }

  if (!designable && section === 'structure') {
    return <FieldList structure={structure} driver={session?.driver} />
  }

  if (section === 'foreignKeys') {
    return (
      <div className="dm-pane-body" style={{ padding: 12 }}>
        {structure.foreignKeys.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No foreign keys" />
        ) : (
          <Table
            className="dm-grid"
            size="small"
            rowKey="name"
            columns={fkColumns}
            dataSource={structure.foreignKeys}
            pagination={false}
            scroll={{ x: 'max-content' }}
          />
        )}
      </div>
    )
  }

  if (section === 'ddl') {
    return (
      <div className="dm-pane-body" style={{ padding: 12 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
          <Typography.Title level={5} style={{ margin: 0 }}>
            {relational ? 'DDL' : 'Definition'}
          </Typography.Title>
          <span style={{ flex: 1 }} />
          <Tooltip title={relational ? 'Copy DDL' : 'Copy the definition script'}>
            <Button
              size="small"
              icon={<CopyOutlined />}
              onClick={() => copy(structure.ddl, relational ? 'DDL' : 'Definition')}
            >
              Copy
            </Button>
          </Tooltip>
          <Tooltip title="Open this definition in the DDL editor, where it can be changed and run">
            <Button
              size="small"
              icon={<EditOutlined />}
              onClick={() => openDdlTab(tab.sessionId, database, schema, object)}
            >
              Edit in DDL editor
            </Button>
          </Tooltip>
          <Button size="small" icon={<DownloadOutlined />} onClick={() => void saveDDL()}>
            Save
          </Button>
        </div>
        <SqlCode className="dm-ddl" sql={structure.ddl} driver={session?.driver} />
        <Typography.Paragraph type="secondary" style={{ fontSize: 11, marginTop: 12 }}>
          {appInfo ? `${appInfo.name} ${appInfo.version} · ` : ''}
          {/^(--|\/\/)/.test(structure.ddl)
            ? relational
              ? 'DDL reconstructed from catalog metadata.'
              : 'Definition script reconstructed from the sampled documents and the index list.'
            : ''}
        </Typography.Paragraph>
      </div>
    )
  }

  // --- section === 'structure': the table designer -------------------------

  const rowCount = draft?.columns.length ?? 0
  const statements = plan?.statements ?? []
  const hasChanges = dirty && !planError && statements.length > 0

  return (
    <div className="dm-pane-body dm-designer">
      <div className="dm-designer-toolbar">
        {creating ? (
          <Input
            size="small"
            className="mono"
            style={{ width: 200 }}
            placeholder="Table name"
            value={draft?.object ?? ''}
            disabled={readOnly}
            onChange={(event) => {
              if (!draft) return
              updateDesign(tab.id, { ...draft, object: event.target.value })
            }}
          />
        ) : null}
        <Tooltip title="Add a field at the end of the list">
          <Button
            size="small"
            icon={<PlusOutlined />}
            disabled={readOnly || !draft}
            onClick={() => {
              if (!draft) return
              const next = [...draft.columns, emptyColumn(draft, session?.driver)]
              updateDesign(tab.id, { ...draft, columns: next })
              setSelectedField(next.length - 1)
            }}
          >
            Add field
          </Button>
        </Tooltip>
        <Tooltip title="Drop the selected field and its data">
          <Button
            size="small"
            danger
            icon={<DeleteOutlined />}
            disabled={readOnly || selectedField === null || !draft}
            onClick={() => {
              if (!draft || selectedField === null) return
              const field = draft.columns[selectedField]
              const applied = Boolean(field.originalName)
              modal.confirm({
                title: `Drop field ${field.name}?`,
                content: applied
                  ? 'The column and everything stored in it is dropped when you save.'
                  : 'The field has not been created yet, so it is simply removed from the design.',
                okText: 'Drop',
                okButtonProps: { danger: true },
                onOk: () => {
                  const columns = draft.columns.filter((_, i) => i !== selectedField)
                  const names = new Set(columns.map((c) => c.name.trim().toLowerCase()))
                  const indexes = draft.indexes
                    .map((ix) => ({
                      ...ix,
                      columns: ix.columns.filter((c) => names.has(c.trim().toLowerCase())),
                    }))
                    .filter((ix) => ix.columns.length > 0)
                  updateDesign(tab.id, { ...draft, columns, indexes })
                  setSelectedField(null)
                },
              })
            }}
          >
            Delete field
          </Button>
        </Tooltip>

        <span className="dm-toolbar-sep" />

        <Tooltip title="Index the current fields">
          <Button
            size="small"
            icon={<PlusOutlined />}
            disabled={readOnly || !draft}
            onClick={() => {
              if (!draft) return
              const fields = draft.columns.map((c) => c.name.trim()).filter(Boolean)
              const next = [...draft.indexes, emptyIndex(draft, fields.slice(0, 1))]
              updateDesign(tab.id, { ...draft, indexes: next })
              setSelectedIndex(next.length - 1)
            }}
          >
            Add index
          </Button>
        </Tooltip>
        <Tooltip title="Drop the selected index">
          <Button
            size="small"
            danger
            icon={<DeleteOutlined />}
            disabled={readOnly || selectedIndex === null || !draft}
            onClick={() => {
              if (!draft || selectedIndex === null) return
              const index = draft.indexes[selectedIndex]
              const indexes = draft.indexes.filter((_, i) => i !== selectedIndex)
              updateDesign(tab.id, { ...draft, indexes })
              setSelectedIndex(null)
              message.info(`Index ${index.name} will be dropped when you save`)
            }}
          >
            Delete index
          </Button>
        </Tooltip>

        <div className="dm-toolbar-right">
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {rowCount} field{rowCount === 1 ? '' : 's'}
            {draft ? ` · ${draft.indexes.length} index${draft.indexes.length === 1 ? '' : 'es'}` : ''}
          </Typography.Text>
          {readOnly ? (
            <Tag color="warning">read-only</Tag>
          ) : (
            <>
              {/* Reverting a new table would only throw the name away; closing
                  the window is the way out of it. */}
              {creating ? null : (
                <Button
                  size="small"
                  icon={<ReloadOutlined />}
                  disabled={!dirty}
                  onClick={() => {
                    ensureDesign(tab.id, structure, true)
                    setSelectedField(null)
                    setSelectedIndex(null)
                  }}
                >
                  Revert
                </Button>
              )}
              <Button
                size="small"
                type="primary"
                loading={applying}
                disabled={!hasChanges}
                onClick={() => apply(statements)}
              >
                {creating ? 'Create' : 'Save'}
              </Button>
            </>
          )}
        </div>
      </div>

      {applyError ? (
        <Alert
          type="error"
          showIcon
          closable
          style={{ margin: '8px 12px 0' }}
          title="The script stopped in the middle"
          description={<span className="mono">{applyError}</span>}
          onClose={() => setApplyError(null)}
        />
      ) : null}

      <Splitter layout="vertical" className="dm-designer-split">
        <Splitter.Panel defaultSize="58%" min="25%">
          <div className="dm-designer-panel">
            <div className="dm-designer-heading">Fields</div>
            <FieldGrid
              design={draft ?? { sessionId: tab.sessionId, object, columns: [], indexes: [] }}
              driver={session?.driver}
              readOnly={readOnly}
              onChange={(next) => updateDesign(tab.id, next)}
              selection={{ selected: selectedField, onChange: setSelectedField }}
            />
          </div>
        </Splitter.Panel>
        <Splitter.Panel min="15%">
          <div className="dm-designer-panel">
            <div className="dm-designer-heading">Indexes</div>
            <IndexGrid
              design={draft ?? { sessionId: tab.sessionId, object, columns: [], indexes: [] }}
              driver={session?.driver}
              readOnly={readOnly}
              onChange={(next) => updateDesign(tab.id, next)}
              selection={{ selected: selectedIndex, onChange: setSelectedIndex }}
              primary={primaryKeyRow(structure)}
            />
          </div>
        </Splitter.Panel>
      </Splitter>

      <div className="dm-design-preview">
        <div className="dm-design-preview-head">
          <CaretRightOutlined style={{ fontSize: 10 }} />
          <Typography.Text strong style={{ fontSize: 12 }}>
            SQL preview
          </Typography.Text>
          {plan?.destructive ? <Tag color="red">drops data</Tag> : null}
          <span className="dm-spacer" />
          <Typography.Text type="secondary" style={{ fontSize: 11 }}>
            {planError
              ? 'the design cannot be rendered'
              : statements.length === 0
                ? creating
                  ? 'nothing to create yet'
                  : 'no changes'
                : `${statements.length} statement${statements.length === 1 ? '' : 's'}`}
          </Typography.Text>
          {statements.length > 0 ? (
            <Tooltip title="Copy the script">
              <Button
                size="small"
                icon={<CopyOutlined />}
                onClick={() => copy(statements.map((s) => `${s};`).join('\n'), 'SQL')}
              />
            </Tooltip>
          ) : null}
        </div>
        <div className="dm-design-preview-body">
          {planError ? (
            <Alert type="warning" showIcon title={<span className="mono">{planError}</span>} />
          ) : null}
          {plan && plan.warnings.length > 0 ? (
            <Alert
              type="info"
              showIcon
              icon={<WarningOutlined />}
              style={{ marginBottom: 6 }}
              title="This engine cannot do everything the design asks for"
              description={
                <ul style={{ margin: 0, paddingLeft: 18 }}>
                  {plan.warnings.map((warning, index) => (
                    <li key={index}>{warning}</li>
                  ))}
                </ul>
              }
            />
          ) : null}
          {statements.length === 0 && !planError ? (
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {creating
                ? 'Name the table, then add the fields it should have.'
                : 'The table matches the design; nothing to run.'}
            </Typography.Text>
          ) : (
            <SqlCode
              className="dm-ddl"
              style={{ margin: 0 }}
              sql={statements.map((statement) => `${statement};`).join('\n')}
              driver={session?.driver}
            />
          )}
        </div>
      </div>
    </div>
  )
}

/**
 * The read-only field list of an object this build cannot design.
 *
 * Two engines land here for different reasons, so the note says which one it
 * is: a collection has no schema to design, while a Doris table has one that
 * this designer does not model (its DDL needs a data model and a
 * `DISTRIBUTED BY` clause). Either way the fields come from the catalog and
 * are reported as they are.
 */
function FieldList({
  structure,
  driver,
}: {
  structure: TableStructure
  driver: DriverType | undefined
}) {
  const mongo = driver === 'mongodb'
  return (
    <div className="dm-pane-body" style={{ padding: 12 }}>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        title={mongo ? 'Collections have no schema' : 'This engine has no table designer'}
        description={
          mongo
            ? 'The fields below were inferred from a sample of the documents in this collection. Any document may carry other fields, or the same field with another type, so there is nothing to design here.'
            : 'The fields below come from the catalog and are read-only. Doris DDL needs a data model and a distribution clause, so changes go through the DDL editor, where the engine\u2019s own statement is edited and run.'
        }
      />
      {structure.columns.length === 0 ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={mongo ? 'No documents to sample' : 'No fields'}
        />
      ) : (
        <Table
          className="dm-grid"
          size="small"
          rowKey="name"
          columns={fieldColumns}
          dataSource={structure.columns}
          pagination={false}
          scroll={{ x: 'max-content' }}
        />
      )}
    </div>
  )
}

/* ------------------------------------------------------------- column sets */

const fieldColumns: TableColumnsType<ColumnInfo> = [
  {
    title: 'Field',
    dataIndex: 'name',
    render: (name: string, row) => (
      <Space size={6}>
        <span className="mono">{name}</span>
        {row.primaryKey ? <Tag color="gold">key</Tag> : null}
      </Space>
    ),
  },
  {
    title: 'Type',
    dataIndex: 'dataType',
    // `columnType` is the engine's own spelling (including length and
    // precision) and is what a SQL engine reports; the sampled document fields
    // of a collection only have the inferred `dataType`.
    render: (value: string, row) => <Tag className="mono">{row.columnType || value}</Tag>,
  },
  {
    title: 'May be missing',
    dataIndex: 'nullable',
    width: 140,
    render: (value: boolean) => (value ? 'yes' : 'no'),
  },
]

const indexColumns: TableColumnsType<IndexInfo> = [
  {
    title: 'Name',
    dataIndex: 'name',
    render: (name: string) => <span className="mono">{name}</span>,
  },
  {
    title: 'Columns',
    dataIndex: 'columns',
    render: (columns: string[]) => (
      <Space size={4} wrap>
        {columns.map((column, i) => (
          <Tag key={`${column}-${i}`} className="mono">
            {column}
          </Tag>
        ))}
      </Space>
    ),
  },
  {
    title: 'Unique',
    dataIndex: 'unique',
    width: 90,
    render: (unique: boolean, row) =>
      row.primary ? <Tag color="gold">PRIMARY</Tag> : unique ? <Tag color="green">unique</Tag> : '—',
  },
  {
    title: 'Method',
    dataIndex: 'method',
    width: 110,
    render: (value: string) => value || <span className="dm-null">—</span>,
  },
]

const fkColumns: TableColumnsType<ForeignKeyInfo> = [
  {
    title: 'Name',
    dataIndex: 'name',
    render: (name: string) => <span className="mono">{name}</span>,
  },
  {
    title: 'Columns',
    dataIndex: 'columns',
    render: (columns: string[]) => <span className="mono">{columns.join(', ')}</span>,
  },
  {
    title: 'References',
    key: 'references',
    render: (_value, row) => (
      <span className="mono">
        {[row.referencedSchema, row.referencedTable].filter(Boolean).join('.')}(
        {row.referencedColumns.join(', ')})
      </span>
    ),
  },
  {
    title: 'On delete',
    dataIndex: 'onDelete',
    width: 110,
    render: (value: string) => value || <span className="dm-null">—</span>,
  },
  {
    title: 'On update',
    dataIndex: 'onUpdate',
    width: 110,
    render: (value: string) => value || <span className="dm-null">—</span>,
  },
]
