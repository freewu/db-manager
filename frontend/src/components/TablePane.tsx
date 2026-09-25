import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import {
  Alert,
  App as AntApp,
  Badge,
  Button,
  Dropdown,
  Empty,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  Tooltip,
  Typography,
} from 'antd'
import type { MenuProps } from 'antd'
import {
  AppstoreOutlined,
  CopyOutlined,
  DeleteOutlined,
  DownloadOutlined,
  FileTextOutlined,
  FilterOutlined,
  InfoCircleOutlined,
  KeyOutlined,
  LinkOutlined,
  LockOutlined,
  PlusOutlined,
  ReloadOutlined,
  TableOutlined,
} from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type {
  CellValue,
  FetchResult,
  FilterOperator,
  FilterSpec,
  KeyValue,
  SortSpec,
} from '../api/types'
import { capabilitiesOf } from '../lib/capabilities'
import { downloadText, resultToCSV, resultToJSON, toInsertScript } from '../lib/export'
import { formatDuration, qualifiedName } from '../lib/format'
import { useAppStore, type TableView, type WorkspaceTab } from '../store/appStore'
import { DataGrid } from './DataGrid'
import { RowDetail } from './RowDetail'
import { StructureView } from './StructurePane'
import { t, tn, useLanguage } from '../lib/i18n'

const PAGE_SIZES = [50, 100, 200, 500, 1000]

/**
 * Navicat-style sub-views of a table window, rendered as a bottom tab strip.
 *
 * A document store keeps the same strip minus foreign keys, which do not exist
 * there, and with the definition slice named for what it is.
 */
function subviewsFor(relational: boolean): { value: TableView; label: string; icon: ReactNode }[] {
  const views = [
    { value: 'data' as TableView, label: t('tablePane.data'), icon: <TableOutlined /> },
    { value: 'structure' as TableView, label: t('tablePane.structure'), icon: <AppstoreOutlined /> },
    { value: 'indexes' as TableView, label: t('tablePane.indexes'), icon: <KeyOutlined /> },
  ]
  if (relational) {
    views.push({ value: 'foreignKeys' as TableView, label: t('tablePane.foreign-keys'), icon: <LinkOutlined /> })
  }
  views.push({
    value: 'ddl' as TableView,
    label: relational ? 'DDL' : t('tablePane.definition'),
    icon: <FileTextOutlined />,
  })
  return views
}

const OPERATORS: { value: FilterOperator; label: string; needsValue: boolean; needsSecond?: boolean }[] = [
  { value: 'eq', label: '=', needsValue: true },
  { value: 'ne', label: '≠', needsValue: true },
  { value: 'gt', label: '>', needsValue: true },
  { value: 'gte', label: '≥', needsValue: true },
  { value: 'lt', label: '<', needsValue: true },
  { value: 'lte', label: '≤', needsValue: true },
  { value: 'contains', label: t('tablePane.contains'), needsValue: true },
  { value: 'notContains', label: t('tablePane.does-not-contain'), needsValue: true },
  { value: 'startsWith', label: t('tablePane.starts-with'), needsValue: true },
  { value: 'endsWith', label: t('tablePane.ends-with'), needsValue: true },
  { value: 'in', label: t('tablePane.in-list'), needsValue: true },
  { value: 'notIn', label: t('tablePane.not-in-list'), needsValue: true },
  { value: 'between', label: t('tablePane.between'), needsValue: true, needsSecond: true },
  { value: 'isNull', label: t('tablePane.is-null'), needsValue: false },
  { value: 'isNotNull', label: t('tablePane.is-not-null'), needsValue: false },
]

function operatorLabel(operator: FilterOperator): string {
  return OPERATORS.find((entry) => entry.value === operator)?.label ?? operator
}

function describeFilter(filter: FilterSpec): string {
  const label = operatorLabel(filter.operator)
  switch (filter.operator) {
    case 'isNull':
    case 'isNotNull':
      return `${filter.column} ${label}`
    case 'between':
      return `${filter.column} ${label} ${filter.value ?? ''} … ${filter.value2 ?? ''}`
    default:
      return `${filter.column} ${label} ${filter.value ?? ''}`
  }
}

interface TablePaneProps {
  tab: WorkspaceTab
}

/** Data browser + structure viewer for a single table or view. */
export function TablePane({ tab }: TablePaneProps) {
  const language = useLanguage()
  const session = useAppStore((s) => s.sessionOf(tab.sessionId))
  const { message, modal } = AntApp.useApp()

  const database = tab.database ?? session?.database ?? ''
  const schema = tab.schema ?? ''
  const object = tab.object ?? ''
  const readOnly = Boolean(session?.readOnly)

  const drivers = useAppStore((s) => s.drivers)
  const { relational } = capabilitiesOf(drivers.find((d) => d.type === session?.driver))
  const subviews = useMemo(() => subviewsFor(relational), [relational])

  const setTabView = useAppStore((s) => s.setTabView)
  const view: TableView = tab.view ?? 'data'
  const selectView = useCallback(
    (next: TableView) => setTabView(tab.id, next),
    [setTabView, tab.id],
  )

  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(100)
  const [sort, setSort] = useState<SortSpec[]>([])
  const [filters, setFilters] = useState<FilterSpec[]>([])
  const [countTotal, setCountTotal] = useState(true)
  const [reloadKey, setReloadKey] = useState(0)
  const [data, setData] = useState<FetchResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selected, setSelected] = useState<number[]>([])
  const [detailKey, setDetailKey] = useState<number | null>(null)
  const [structureToken, setStructureToken] = useState(0)

  const sortKey = JSON.stringify(sort)
  const filterKey = JSON.stringify(filters)

  useEffect(() => {
    if (!object) return
    let cancelled = false
    setLoading(true)
    setError(null)
    api
      .fetchRows({
        sessionId: tab.sessionId,
        database,
        schema,
        object,
        limit: pageSize,
        offset: (page - 1) * pageSize,
        orderBy: sort,
        filters,
        countTotal,
      })
      .then((result) => {
        if (cancelled) return
        setData(result)
        setSelected([])
        // A page of rows replaces the last one, so the row that was open is not
        // the row it was any more: index 3 of this page is another record.
        setDetailKey(null)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setError(toMessage(err))
        setData(null)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
    // `sort`/`filters` are compared through their serialised form on purpose.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    tab.sessionId,
    database,
    schema,
    object,
    page,
    pageSize,
    sortKey,
    filterKey,
    countTotal,
    reloadKey,
  ])

  // A document store exports shell, a SQL engine exports INSERT statements.
  const scriptExport = useMemo(
    () =>
      session?.driver === 'mongodb'
        ? {
            menuLabel: t('tablePane.export-insertmany-script'),
            copyLabel: t('tablePane.copy-as-insertmany-script'),
            label: t('tablePane.javascript'),
            extension: 'js',
          }
        : {
            menuLabel: t('tablePane.export-insert-statements'),
            copyLabel: t('tablePane.copy-as-insert'),
            label: 'SQL',
            extension: 'sql',
          },
    [language, session?.driver],
  )

  const primaryKey = useMemo(
    () => (data?.columns ?? []).filter((column) => column.isPrimaryKey).map((column) => column.name),
    [data?.columns],
  )

  const refresh = useCallback(() => setReloadKey((n) => n + 1), [])

  /** Builds the identifying key for a row of the current page. */
  const rowKey = useCallback(
    (rowIndex: number): KeyValue[] => {
      if (!data) return []
      return primaryKey.map((column) => {
        const index = data.columns.findIndex(
          (candidate) => candidate.name.toLowerCase() === column.toLowerCase(),
        )
        const value = index >= 0 ? data.rows[rowIndex]?.[index] : null
        return {
          column,
          value: value === null || value === undefined ? null : String(value),
        }
      })
    },
    [data, primaryKey],
  )

  const editCell = useCallback(
    async (rowIndex: number, column: string, next: string | null) => {
      if (!data) return
      try {
        const affected = await api.updateCell({
          sessionId: tab.sessionId,
          database,
          schema,
          object,
          key: rowKey(rowIndex),
          column,
          value: next,
        })
        if (affected === 0) {
          message.warning(t('tablePane.no-row-matched-it-may-have-been-changed-or'))
        } else {
          message.success(t('tablePane.updated', { column }))
        }
        refresh()
      } catch (err) {
        message.error(toMessage(err))
        throw err
      }
    },
    [data, database, message, object, refresh, rowKey, schema, tab.sessionId],
  )

  const deleteRows = useCallback(
    (indexes: number[]) => {
      if (indexes.length === 0) return
      modal.confirm({
        title: tn('tablePane.delete-rows', indexes.length),
        content: t('tablePane.this-cannot-be-undone'),
        okText: t('tablePane.delete'),
        okButtonProps: { danger: true },
        onOk: async () => {
          let deleted = 0
          for (const index of indexes) {
            try {
              deleted += await api.deleteRow({
                sessionId: tab.sessionId,
                database,
                schema,
                object,
                key: rowKey(index),
              })
            } catch (err) {
              message.error(toMessage(err))
              refresh()
              return
            }
          }
          message.success(tn('tablePane.deleted-rows', deleted))
          setSelected([])
          refresh()
        },
      })
    },
    [database, message, modal, object, refresh, rowKey, schema, tab.sessionId],
  )

  const exportData = useCallback(
    async (format: 'csv' | 'json' | 'sql') => {
      if (!data || data.rows.length === 0) {
        message.info(t('tablePane.there-is-nothing-to-export-on-this-page'))
        return
      }
      const stamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)
      const content =
        format === 'csv'
          ? resultToCSV(data)
          : format === 'json'
            ? resultToJSON(data)
            : toInsertScript(
                { driver: session?.driver ?? '', database, schema, object },
                data.columns,
                data.rows,
              )
      const extension = format === 'sql' ? scriptExport.extension : format
      const filename = `${object}-${stamp}.${extension}`
      try {
        await api.saveTextFile({
          defaultFilename: filename,
          content,
          filters: [
            {
              displayName: format === 'sql' ? scriptExport.label : format.toUpperCase(),
              pattern: `*.${extension}`,
            },
          ],
        })
      } catch (err) {
        const text = toMessage(err)
        if (/cancel/i.test(text)) return
        downloadText(filename, content, 'text/plain')
      }
    },
    [data, database, message, object, schema, scriptExport],
  )

  if (!object) {
    return <Empty description={t('tablePane.no-object-selected')} style={{ marginTop: 80 }} />
  }

  const exportMenu: MenuProps = {
    items: [
      { key: 'csv', label: t('tablePane.export-csv'), onClick: () => void exportData('csv') },
      { key: 'json', label: t('tablePane.export-json'), onClick: () => void exportData('json') },
      { key: 'sql', label: scriptExport.menuLabel, onClick: () => void exportData('sql') },
    ],
  }

  return (
    <div className="dm-pane">
      <div className="dm-editor-toolbar">
        <Typography.Text className="mono dm-object-path" style={{ fontSize: 12 }}>
          {qualifiedName(session?.driver ?? '', database, schema, object)}
        </Typography.Text>

        {view === 'data' ? (
          <>
            <Tooltip title={t('tablePane.reload')}>
              <Button size="small" icon={<ReloadOutlined />} loading={loading} onClick={refresh} />
            </Tooltip>
            <Badge count={filters.length} size="small" offset={[-2, 2]}>
              <FilterButton filters={filters} columns={(data?.columns ?? []).map((c) => c.name)} onChange={(next) => { setFilters(next); setPage(1) }} />
            </Badge>
            <Tooltip title={t('tablePane.count-the-total-number-of-rows-adds-a-count-per')}>
              <Space size={4}>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  {t('tablePane.count')}
                </Typography.Text>
                <Switch size="small" checked={countTotal} onChange={setCountTotal} />
              </Space>
            </Tooltip>
            <Select
              size="small"
              value={pageSize}
              style={{ width: 96 }}
              options={PAGE_SIZES.map((size) => ({ value: size, label: tn('tablePane.page-size-rows', size) }))}
              onChange={(value) => {
                setPageSize(value)
                setPage(1)
              }}
            />

            <div className="dm-toolbar-right">
              {readOnly ? (
                <Typography.Text type="warning" style={{ fontSize: 12 }}>
                  <LockOutlined /> {t('tablePane.read-only')}
                </Typography.Text>
              ) : null}
              <Tooltip title={t('tablePane.show-the-selected-row')}>
                <Button
                  size="small"
                  icon={<InfoCircleOutlined />}
                  disabled={selected.length !== 1}
                  onClick={() => setDetailKey(selected[0] - (page - 1) * pageSize)}
                />
              </Tooltip>
              <Popconfirm
                title={tn('tablePane.delete-rows', selected.length)}
                okText={t('tablePane.delete')}
                okButtonProps={{ danger: true }}
                disabled={selected.length === 0 || readOnly}
                onConfirm={() => deleteRows(selected)}
              >
                <Button
                  size="small"
                  danger
                  icon={<DeleteOutlined />}
                  disabled={selected.length === 0 || readOnly}
                >
                  {t('tablePane.delete')}
                </Button>
              </Popconfirm>
              <Dropdown
                menu={exportMenu}
                trigger={['click']}
                disabled={!data || data.rows.length === 0}
              >
                <Button size="small" icon={<DownloadOutlined />}>
                  {t('tablePane.export')}
                </Button>
              </Dropdown>
            </div>
          </>
        ) : (
          <div className="dm-toolbar-right">
            <Button size="small" icon={<ReloadOutlined />} onClick={() => setStructureToken((n) => n + 1)}>
              {t('tablePane.reload')}
            </Button>
          </div>
        )}
      </div>

      {view !== 'data' ? (
        <StructureView
          tab={tab}
          section={view}
          reloadToken={structureToken}
        />
      ) : (
        <>
          {filters.length > 0 ? (
            <div className="dm-filter-bar">
              {filters.map((filter, index) => (
                <Tag
                  key={`${filter.column}-${index}`}
                  closable
                  className="mono"
                  onClose={() => {
                    setFilters(filters.filter((_, i) => i !== index))
                    setPage(1)
                  }}
                >
                  {describeFilter(filter)}
                </Tag>
              ))}
              <Button size="small" type="link" onClick={() => setFilters([])}>
                {t('tablePane.clear-all')}
              </Button>
            </div>
          ) : null}

          {error ? (
            <div style={{ padding: 10, flex: '0 0 auto' }}>
              <Alert
                type="error"
                showIcon
                title={t('tablePane.could-not-load-rows')}
                description={<span className="mono" style={{ fontSize: 12, whiteSpace: 'pre-wrap' }}>{error}</span>}
                action={
                  <Button size="small" icon={<ReloadOutlined />} onClick={refresh}>
                    {t('tablePane.retry')}
                  </Button>
                }
              />
            </div>
          ) : null}

          {data ? (
            <>
              <DataGrid
                result={data}
                loading={loading}
                rowOffset={(page - 1) * pageSize}
                sort={sort}
                onSortChange={(next) => {
                  setSort(next)
                  setPage(1)
                }}
                page={page}
                pageSize={pageSize}
                total={data.total}
                hasTotal={data.hasTotal}
                onPageChange={(nextPage, nextSize) => {
                  setPage(nextSize !== pageSize ? 1 : nextPage)
                  setPageSize(nextSize)
                }}
                primaryKey={primaryKey}
                editable={!readOnly && primaryKey.length > 0}
                onEditCell={editCell}
                selectedKeys={selected}
                onSelectionChange={setSelected}
                detailRow={detailKey}
                onDetailRowChange={setDetailKey}
                renderRowDetail={(row, index) => (
                  <RowDetail
                    row={row}
                    columns={data.columns}
                    keyValues={rowKey(index)}
                    sessionId={tab.sessionId}
                    database={database}
                    schema={schema}
                    object={object}
                    readOnly={readOnly}
                    title={t('tablePane.row', { index: (page - 1) * pageSize + index + 1 })}
                    onChanged={refresh}
                    actions={
                      <>
                        <Tooltip title={t('tablePane.copy-as-json')}>
                          <Button
                            size="small"
                            icon={<CopyOutlined />}
                            onClick={() => {
                              const record: Record<string, CellValue> = {}
                              data.columns.forEach((column, at) => {
                                record[column.name] = row[at]
                              })
                              void navigator.clipboard
                                .writeText(JSON.stringify(record, null, 2))
                                .then(() => message.success(t('tablePane.row-copied')))
                                .catch((err: unknown) => message.error(toMessage(err)))
                            }}
                          />
                        </Tooltip>
                        <Tooltip title={scriptExport.copyLabel}>
                          <Button
                            size="small"
                            icon={<DownloadOutlined />}
                            onClick={() => {
                              const script = toInsertScript(
                                { driver: session?.driver ?? '', database, schema, object },
                                data.columns,
                                [row],
                              )
                              void navigator.clipboard
                                .writeText(script)
                                .then(() => message.success(t('tablePane.insert-copied')))
                                .catch((err: unknown) => message.error(toMessage(err)))
                            }}
                          />
                        </Tooltip>
                      </>
                    }
                  />
                )}
              />
              <div className="dm-statusbar" style={{ borderTop: '1px solid var(--dm-border)', background: 'transparent' }}>
                <span className="dm-statusbar-item">
                  {tn('tablePane.rows-on-this-page', data.rowCount)}
                  {data.hasTotal
                    ? t('tablePane.of-total', { total: data.total.toLocaleString() })
                    : t('tablePane.total-not-counted')}
                </span>
                <span className="dm-statusbar-item">{formatDuration(data.durationMs)}</span>
                {selected.length > 0 ? (
                  <span className="dm-statusbar-item">
                    {tn('tablePane.selected', selected.length)}
                  </span>
                ) : null}
                <span className="dm-spacer" />
                {!readOnly && primaryKey.length === 0 ? (
                  <span className="dm-statusbar-item">
                    {t('tablePane.no-primary-key-detected-rows-are-read-only')}
                  </span>
                ) : null}
              </div>
            </>
          ) : loading ? (
            <div style={{ padding: 60, textAlign: 'center' }}>
              <Spin />
            </div>
          ) : !error ? (
            <Empty description={t('tablePane.no-rows')} style={{ marginTop: 60 }} />
          ) : null}
        </>
      )}

      <nav className="dm-subtabs" role="tablist" aria-label={t('tablePane.table-views')}>
        {subviews.map((entry) => (
          <button
            key={entry.value}
            type="button"
            role="tab"
            aria-selected={view === entry.value}
            className={`dm-subtab${view === entry.value ? ' is-active' : ''}`}
            onClick={() => selectView(entry.value)}
          >
            {entry.icon}
            <span>{entry.label}</span>
          </button>
        ))}
      </nav>
    </div>
  )
}

/** Button + modal that edits the server-side filter list. */
function FilterButton({
  filters,
  columns,
  onChange,
}: {
  filters: FilterSpec[]
  columns: string[]
  onChange: (next: FilterSpec[]) => void
}) {
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState<FilterSpec[]>(filters)

  const start = () => {
    setDraft(filters.length > 0 ? filters : [{ column: columns[0] ?? '', operator: 'eq', value: '' }])
    setOpen(true)
  }

  const update = (index: number, patch: Partial<FilterSpec>) => {
    setDraft((current) => current.map((entry, i) => (i === index ? { ...entry, ...patch } : entry)))
  }

  return (
    <>
      <Button size="small" icon={<FilterOutlined />} onClick={start}>
        {t('tablePane.filter')}
      </Button>
      <Modal
        title={t('tablePane.filter-rows')}
        open={open}
        width={720}
        onCancel={() => setOpen(false)}
        okText={t('tablePane.apply')}
        onOk={() => {
          onChange(
            draft
              .filter((entry) => entry.column)
              .map((entry) => {
                const spec = OPERATORS.find((candidate) => candidate.value === entry.operator)
                if (!spec?.needsValue) return { column: entry.column, operator: entry.operator }
                if (spec.needsSecond) {
                  return {
                    column: entry.column,
                    operator: entry.operator,
                    value: entry.value ?? '',
                    value2: entry.value2 ?? '',
                  }
                }
                return { column: entry.column, operator: entry.operator, value: entry.value ?? '' }
              }),
          )
          setOpen(false)
        }}
        footer={(_, { OkBtn, CancelBtn }) => (
          <Space>
            <Button
              icon={<PlusOutlined />}
              onClick={() =>
                setDraft((current) => [
                  ...current,
                  { column: columns[0] ?? '', operator: 'eq', value: '' },
                ])
              }
            >
              {t('tablePane.add-condition')}
            </Button>
            <CancelBtn />
            <OkBtn />
          </Space>
        )}
      >
        <Space direction="vertical" style={{ width: '100%' }} size={8}>
          {draft.length === 0 ? (
            <Typography.Text type="secondary">{t('tablePane.no-conditions')}</Typography.Text>
          ) : null}
          {draft.map((entry, index) => {
            const spec = OPERATORS.find((candidate) => candidate.value === entry.operator)
            return (
              <Space key={index} align="start" style={{ width: '100%' }}>
                <Select
                  showSearch
                  style={{ width: 200 }}
                  value={entry.column || undefined}
                  placeholder={t('tablePane.column')}
                  options={columns.map((column) => ({ value: column, label: column }))}
                  onChange={(value) => update(index, { column: value })}
                />
                <Select
                  style={{ width: 168 }}
                  value={entry.operator}
                  options={OPERATORS.map((operator) => ({
                    value: operator.value,
                    label: operator.label,
                  }))}
                  onChange={(value) => update(index, { operator: value })}
                />
                {spec?.needsValue ? (
                  <Input
                    style={{ width: 220 }}
                    placeholder={entry.operator === 'in' || entry.operator === 'notIn' ? t('tablePane.a-b-c') : t('tablePane.value')}
                    value={entry.value ?? ''}
                    onChange={(event) => update(index, { value: event.target.value })}
                  />
                ) : (
                  <Input style={{ width: 220 }} disabled placeholder="—" />
                )}
                {spec?.needsSecond ? (
                  <Input
                    style={{ width: 220 }}
                    placeholder={t('tablePane.upper-bound')}
                    value={entry.value2 ?? ''}
                    onChange={(event) => update(index, { value2: event.target.value })}
                  />
                ) : null}
                <Button
                  danger
                  type="text"
                  icon={<DeleteOutlined />}
                  onClick={() => setDraft((current) => current.filter((_, i) => i !== index))}
                />
              </Space>
            )
          })}
        </Space>
      </Modal>
    </>
  )
}
