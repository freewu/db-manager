/**
 * The settings page for the user's own mock placeholders.
 *
 * A custom placeholder is a name for a template of the built-in ones: it exists
 * so that a column that always holds `SO@date(yyyy)@natural(1000, 9999)` can be
 * written as `@orderNo` in every mock of a table, with one place to change what
 * it means. The backend keeps one file per placeholder under the `.mock` folder
 * of the data directory (so it travels with the data folder), and this page is
 * the only place that writes them.
 *
 * Two things decide whether a placeholder is any good, and both are answered
 * while it is typed rather than after a save that would then have to be undone:
 * the form refuses a name the engine could not resolve or a template that does
 * not compile, and the debug panel below it renders the template for a handful
 * of rows, with the seed shown, so "it compiles" can be told apart from "it
 * produces what I meant".
 */
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Alert, App as AntApp, Button, Empty, Input, Modal, Space, Tag, Tooltip } from 'antd'
import { DeleteOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons'

import { toMessage } from '../api/client'
import type { MockPlaceholder } from '../api/types'
import {
  bareName,
  checkCustomPlaceholder,
  createRng,
  compileTemplate,
  isClean,
  sampleOf,
  seedOf,
  type CustomDraft,
} from '../lib/mock'
import { useAppStore } from '../store/appStore'
import { t, tr } from '../lib/i18n'

/** How many rows the debug panel renders at a time. */
const DEBUG_ROWS = 5

const EMPTY_DRAFT: CustomDraft = { name: '', template: '', description: '' }

/**
 * Renders a handful of rows from a draft, so a template can be judged by what it
 * produces rather than by how it reads.
 *
 * The rows come from one random source, drawn one after another, which is what a
 * batch fill does — a placeholder whose values are all identical, or all
 * identical to the first, shows up here and nowhere else. The seed is on screen
 * and can be moved, so a set of values that looks wrong can be reproduced.
 */
function DebugRows({
  template,
  placeholders,
  seed,
  onSeed,
}: {
  template: string
  placeholders: MockPlaceholder[]
  seed: number
  onSeed: (seed: number) => void
}) {
  const rendered = useMemo(() => {
    const compiled = compileTemplate(
      template,
      placeholders.map((entry) => ({
        name: entry.name,
        template: entry.template,
        description: entry.description,
      })),
    )
    if (compiled.error) return { error: compiled.error }
    const rng = createRng(seed)
    const values: string[] = []
    for (let i = 0; i < DEBUG_ROWS; i += 1) {
      const value = compiled.render(rng)
      values.push(value === null || value === undefined ? '—' : String(value))
    }
    return { values }
  }, [template, placeholders, seed])

  return (
    <div className="dm-custom-debug">
      <div className="dm-custom-debug-head">
        <span className="dm-custom-label">{t('mockPlaceholderSettings.try-it')}</span>
        <Space size={8}>
          <Tooltip title={t('mockPlaceholderSettings.draw-another-set-of-rows-from-a-different-random')}>
            <Button
              size="small"
              icon={<ReloadOutlined />}
              onClick={() => onSeed((seed + 0x9e3779b1) >>> 0)}
            >
              {t('mockPlaceholderSettings.reroll')}
            </Button>
          </Tooltip>
          <span className="dm-custom-hint">{t('mockPlaceholderSettings.seed')} {seed.toString(16)}</span>
        </Space>
      </div>
      {'error' in rendered ? (
        <Alert type="error" showIcon message={rendered.error} />
      ) : (
        <ol className="dm-custom-rows">
          {rendered.values.map((value, index) => (
            // The rows are a sample, not a list of things with identities: the
            // index is the row number, which is what a reader counts by.
            <li key={index}>
              <span className="dm-custom-row-no">{index + 1}</span>
              <span className="mono">{value}</span>
            </li>
          ))}
        </ol>
      )}
    </div>
  )
}

/** The editor for one placeholder: a modal, because it is a form, not a page. */
function PlaceholderEditor({
  open,
  editing,
  others,
  onClose,
  onSaved,
}: {
  open: boolean
  /** The placeholder being changed; absent while creating one. */
  editing?: MockPlaceholder
  /** Every other custom placeholder: what a name may not collide with. */
  others: MockPlaceholder[]
  onClose: () => void
  onSaved: () => void
}) {
  const save = useAppStore((state) => state.saveMockPlaceholder)
  const [draft, setDraft] = useState<CustomDraft>(EMPTY_DRAFT)
  const [seed, setSeed] = useState(0)
  const [saving, setSaving] = useState(false)
  const [failure, setFailure] = useState<string>()

  useEffect(() => {
    if (!open) return
    const next = editing
      ? {
          name: editing.name,
          template: editing.template,
          description: editing.description ?? '',
        }
      : EMPTY_DRAFT
    setDraft(next)
    // A fixed starting point for the debug rows, so opening the editor twice on
    // the same template shows the same rows — and so a reader can compare two
    // edits without the seed moving underneath them.
    setSeed(seedOf(next.template))
    setFailure(undefined)
  }, [open, editing])

  const engineView = useMemo(
    () =>
      others.map((entry) => ({
        name: entry.name,
        template: entry.template,
        description: entry.description,
      })),
    [others],
  )
  // The draft is laid on top of the others for the debug panel as well, so a
  // template that calls its own name shows the cycle it is rather than an
  // unknown placeholder.
  const debugView = useMemo(
    () => [
      ...others,
      {
        name: bareName(draft.name) || 'draft',
        template: draft.template,
        description: draft.description,
      },
    ],
    [others, draft],
  )
  const errors = useMemo(() => checkCustomPlaceholder(draft, engineView), [draft, engineView])
  const ready = isClean(errors)

  const commit = useCallback(async () => {
    setSaving(true)
    setFailure(undefined)
    try {
      await save({
        // A save is keyed by the name, so the bare one is what has to be sent;
        // `@orderNo` typed into the form means the placeholder the templates
        // write as `@orderNo`.
        name: bareName(draft.name),
        template: draft.template.trim(),
        description: draft.description.trim(),
      })
      onSaved()
    } catch (error) {
      // The backend checks the file's shape, the engine the template, and both
      // messages are worth reading as they came.
      setFailure(toMessage(error))
    } finally {
      setSaving(false)
    }
  }, [draft, onSaved, save])

  return (
    <Modal
      open={open}
      title={editing ? t('mockPlaceholderSettings.edit', { name: editing.name }) : t('mockPlaceholderSettings.new-custom-placeholder')}
      width={640}
      onCancel={onClose}
      destroyOnHidden
      footer={
        <Space>
          <Button onClick={onClose}>{t('mockPlaceholderSettings.cancel')}</Button>
          <Button type="primary" loading={saving} disabled={!ready} onClick={() => void commit()}>
            {t('mockPlaceholderSettings.save')}
          </Button>
        </Space>
      }
    >
      {failure ? (
        <Alert type="error" showIcon message={failure} style={{ marginBottom: 12 }} />
      ) : null}
      <div className="dm-custom-form">
        <label className="dm-custom-field">
          <span className="dm-custom-label">{t('mockPlaceholderSettings.name')}</span>
          <Input
            value={draft.name}
            // The name is the file name, so it cannot move once the file exists;
            // renaming would be a delete and a create, which this form is not.
            disabled={editing !== undefined}
            placeholder={t('mockPlaceholderSettings.orderno')}
            addonBefore="@"
            status={errors.name ? 'error' : undefined}
            onChange={(event) => setDraft({ ...draft, name: event.target.value })}
          />
          <span className="dm-custom-hint">
            {errors.name ??
              (editing
                ? t('mockPlaceholderSettings.a-name-cannot-be-changed-delete-the-placeholder')
                : t('mockPlaceholderSettings.what-templates-write-after-letters-digits-and'))}
          </span>
        </label>

        <label className="dm-custom-field">
          <span className="dm-custom-label">{t('mockPlaceholderSettings.template')}</span>
          <Input
            value={draft.template}
            placeholder={t('mockPlaceholderSettings.so-date-yyyy-natural-1000-9999')}
            status={errors.template ? 'error' : undefined}
            onChange={(event) => setDraft({ ...draft, template: event.target.value })}
          />
          <span className="dm-custom-hint">
            {errors.template ??
              t('mockPlaceholderSettings.any-built-in-placeholder-can-be-used-in-it-and')}
          </span>
        </label>

        <label className="dm-custom-field">
          <span className="dm-custom-label">{t('mockPlaceholderSettings.description')}</span>
          <Input
            value={draft.description}
            placeholder={t('mockPlaceholderSettings.text')}
            status={errors.description ? 'error' : undefined}
            onChange={(event) => setDraft({ ...draft, description: event.target.value })}
          />
          <span className="dm-custom-hint">
            {errors.description ?? t('mockPlaceholderSettings.shown-beside-the-placeholder-in-the-picker-and')}
          </span>
        </label>
      </div>

      <DebugRows template={draft.template} placeholders={debugView} seed={seed} onSeed={setSeed} />
    </Modal>
  )
}

/** The pane itself: what is stored, and the three things that can be done to it. */
export function MockPlaceholderSettings() {
  const placeholders = useAppStore((state) => state.mockPlaceholders)
  const refresh = useAppStore((state) => state.refreshMockPlaceholders)
  const remove = useAppStore((state) => state.deleteMockPlaceholder)

  const { message, modal } = AntApp.useApp()
  const [editing, setEditing] = useState<MockPlaceholder>()
  const [creating, setCreating] = useState(false)
  const [failure, setFailure] = useState<string>()

  const reload = useCallback(() => {
    refresh()
      .then(() => setFailure(undefined))
      .catch((error) => setFailure(toMessage(error)))
  }, [refresh])

  useEffect(reload, [reload])

  const openCreate = useCallback(() => {
    setEditing(undefined)
    setCreating(true)
  }, [])

  const openEdit = useCallback((entry: MockPlaceholder) => {
    setEditing(entry)
    setCreating(true)
  }, [])

  const confirmDelete = useCallback(
    (entry: MockPlaceholder) => {
      modal.confirm({
        title: t('mockPlaceholderSettings.delete', { name: entry.name }),
        content:
          t('mockPlaceholderSettings.mocks-that-use-it-will-report-an-unknown'),
        okText: t('mockPlaceholderSettings.delete-2'),
        okButtonProps: { danger: true },
        onOk: async () => {
          try {
            await remove(entry.name)
            message.success(t('mockPlaceholderSettings.deleted', { name: entry.name }))
          } catch (error) {
            setFailure(toMessage(error))
          }
        },
      })
    },
    [message, modal, remove],
  )

  const healthy = placeholders.filter((entry) => !entry.broken)
  const others = editing
    ? healthy.filter((entry) => entry.name !== editing.name)
    : healthy

  return (
    <div className="dm-custom-pane">
      <div className="dm-custom-head">
        <div>
          <span className="dm-custom-label">{t('mockPlaceholderSettings.custom-placeholders')}</span>
          <span className="dm-custom-hint" style={{ display: 'block' }}>
            {tr('mockPlaceholderSettings.one-file-per-placeholder', {
              extension: <span className="mono">{t('mockPlaceholderSettings.mock')}</span>,
            })}
          </span>
        </div>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={reload}>
            {t('mockPlaceholderSettings.refresh')}
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            {t('mockPlaceholderSettings.new')}
          </Button>
        </Space>
      </div>

      {failure ? (
        <Alert type="error" showIcon message={failure} style={{ marginBottom: 12 }} />
      ) : null}

      {placeholders.length === 0 ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={
            <span>
              {tr('mockPlaceholderSettings.no-custom-placeholder-yet', {
                example: <b>{t('mockPlaceholderSettings.orderno')}</b>,
                syntax: (
                  <span className="mono">
                    {t('mockPlaceholderSettings.so-date-yyyy-natural-1000-9999')}
                  </span>
                ),
                reference: <span className="mono">{t('mockPlaceholderSettings.orderno-2')}</span>,
              })}
            </span>
          }
        >
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            {t('mockPlaceholderSettings.new')}
          </Button>
        </Empty>
      ) : (
        <ul className="dm-custom-list">
          {placeholders.map((entry) => (
            <li key={entry.name} className={entry.broken ? 'is-broken' : undefined}>
              <div className="dm-custom-row-main">
                <span className="mono dm-custom-name">@{entry.name}</span>
                {entry.broken ? (
                  <Tag color="error">{t('mockPlaceholderSettings.unreadable')}</Tag>
                ) : (
                  <span className="mono dm-custom-template">{entry.template}</span>
                )}
              </div>
              <div className="dm-custom-row-sub">
                {entry.broken ? (
                  <span className="dm-custom-bad">
                    {entry.broken} {t('mockPlaceholderSettings.the-file-is-listed-so-it-can-be-deleted')}
                  </span>
                ) : (
                  <>
                    <SampleOf entry={entry} placeholders={healthy} />
                    {entry.description?.trim() ? (
                      <span className="dm-custom-desc">{entry.description}</span>
                    ) : null}
                  </>
                )}
              </div>
              <Space size={4}>
                <Button
                  size="small"
                  disabled={Boolean(entry.broken)}
                  onClick={() => openEdit(entry)}
                >
                  {t('mockPlaceholderSettings.edit-2')}
                </Button>
                <Button
                  size="small"
                  danger
                  icon={<DeleteOutlined />}
                  onClick={() => confirmDelete(entry)}
                />
              </Space>
            </li>
          ))}
        </ul>
      )}

      <PlaceholderEditor
        open={creating}
        editing={editing}
        others={others}
        onClose={() => setCreating(false)}
        onSaved={() => {
          setCreating(false)
          message.success(t('mockPlaceholderSettings.placeholder-saved'))
        }}
      />
    </div>
  )
}

/** One rendered example, beside the template it came from. */
function SampleOf({
  entry,
  placeholders,
}: {
  entry: MockPlaceholder
  placeholders: MockPlaceholder[]
}) {
  const sample = useMemo(
    () =>
      sampleOf(
        `@${entry.name}`,
        placeholders.map((other) => ({
          name: other.name,
          template: other.template,
          description: other.description,
        })),
        seedOf(entry.name),
      ),
    [entry, placeholders],
  )
  if (sample.error) return <span className="dm-custom-bad">{sample.error}</span>
  return <span className="mono dm-custom-sample">{sample.value ?? '—'}</span>
}
