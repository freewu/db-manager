import { useState } from 'react'
import { App as AntApp, Button, Dropdown, Form, Input, Modal } from 'antd'
import type { MenuProps } from 'antd'
import {
  DeleteOutlined,
  EditOutlined,
  SaveOutlined,
  StarOutlined,
} from '@ant-design/icons'

import { toMessage } from '../api/client'
import type { DriverType, SavedQuery } from '../api/types'
import { useAppStore } from '../store/appStore'
import { t } from '../lib/i18n'

interface QueryFavoritesProps {
  /** The statement currently in the editor. */
  sql: string
  driver?: DriverType
  database?: string
  /** Puts a favourite back into the editor. */
  onLoad: (sql: string) => void
}

/**
 * “Save current SQL” + the list of saved snippets, rendered as a dropdown next
 * to the query toolbar's history.
 *
 * Favourites are stored by the backend (config/queries.json) rather than in the
 * UI state blob, so they outlive the window that created them and the same
 * snippet is available in every query tab.
 */
export function QueryFavorites({ sql, driver, database, onLoad }: QueryFavoritesProps) {
  const savedQueries = useAppStore((s) => s.savedQueries)
  const saveSavedQuery = useAppStore((s) => s.saveSavedQuery)
  const deleteSavedQuery = useAppStore((s) => s.deleteSavedQuery)
  const { message, modal } = AntApp.useApp()

  const [open, setOpen] = useState(false)
  const [menuOpen, setMenuOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [name, setName] = useState('')
  const [draftSql, setDraftSql] = useState('')
  /** Non-null when the dialog is editing an existing favourite. */
  const [editing, setEditing] = useState<SavedQuery | null>(null)

  const openCreate = () => {
    setEditing(null)
    setName(suggestName(sql))
    setDraftSql(sql)
    setOpen(true)
  }

  const openEdit = (query: SavedQuery) => {
    // The dialog takes over from the dropdown; leaving it open behind the modal
    // looks like a stuck menu.
    setMenuOpen(false)
    setEditing(query)
    setName(query.name)
    setDraftSql(query.sql)
    setOpen(true)
  }

  const submit = async () => {
    const trimmed = name.trim()
    if (!trimmed) {
      message.warning(t('queryFavorites.give-the-saved-query-a-name'))
      return
    }
    setSaving(true)
    try {
      await saveSavedQuery({
        id: editing?.id ?? '',
        name: trimmed,
        sql: draftSql,
        database: editing?.database ?? database,
        driver: editing?.driver ?? driver,
        createdAt: editing?.createdAt ?? 0,
        updatedAt: 0,
      })
      message.success(editing ? t('queryFavorites.saved-query-updated') : t('queryFavorites.saved-to-favourites'))
      setOpen(false)
    } catch (err) {
      message.error(toMessage(err))
    } finally {
      setSaving(false)
    }
  }

  const confirmDelete = (query: SavedQuery) => {
    setMenuOpen(false)
    modal.confirm({
      title: t('queryFavorites.delete-saved-query'),
      content: t('queryFavorites.remove-from-the-favourites', { name: query.name }),
      okText: t('queryFavorites.delete'),
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await deleteSavedQuery(query.id)
          message.success(t('queryFavorites.saved-query-deleted'))
        } catch (err) {
          message.error(toMessage(err))
        }
      },
    })
  }

  const items: MenuProps['items'] = [
    {
      key: 'save',
      label: t('queryFavorites.save-current-sql'),
      icon: <SaveOutlined />,
      disabled: !sql.trim(),
    },
    { type: 'divider' },
  ]

  if (savedQueries.length === 0) {
    items.push({ key: 'empty', label: t('queryFavorites.no-saved-queries-yet'), disabled: true })
  } else {
    for (const query of savedQueries) {
      items.push({
        key: query.id,
        label: (
          <span className="dm-fav-row">
            <span className="dm-fav-title" title={query.sql}>
              {query.name}
            </span>
            {query.database ? <span className="dm-fav-meta">{query.database}</span> : null}
            <span className="dm-fav-actions">
              <button
                type="button"
                className="dm-fav-action"
                title={t('queryFavorites.edit')}
                aria-label={t('queryFavorites.edit-2', { name: query.name })}
                onClick={(event) => {
                  // Keep the menu open and stop it from loading the snippet.
                  event.stopPropagation()
                  openEdit(query)
                }}
              >
                <EditOutlined />
              </button>
              <button
                type="button"
                className="dm-fav-action is-danger"
                title={t('queryFavorites.delete')}
                aria-label={t('queryFavorites.delete-2', { name: query.name })}
                onClick={(event) => {
                  event.stopPropagation()
                  confirmDelete(query)
                }}
              >
                <DeleteOutlined />
              </button>
            </span>
          </span>
        ),
      })
    }
  }

  const onClick: MenuProps['onClick'] = ({ key }) => {
    setMenuOpen(false)
    if (key === 'save') {
      openCreate()
      return
    }
    if (key === 'empty') return
    const query = savedQueries.find((entry) => entry.id === key)
    if (!query) return
    onLoad(query.sql)
    message.success(t('queryFavorites.loaded', { name: query.name }))
  }

  return (
    <>
      <Dropdown
        menu={{ items, onClick }}
        trigger={['click']}
        open={menuOpen}
        onOpenChange={setMenuOpen}
      >
        <Button size="small" icon={<StarOutlined />}>
          {t('queryFavorites.favourites')}
          {savedQueries.length > 0 ? (
            <span className="dm-fav-count">{savedQueries.length}</span>
          ) : null}
        </Button>
      </Dropdown>

      <Modal
        title={editing ? t('queryFavorites.edit-saved-query') : t('queryFavorites.save-query-to-favourites')}
        open={open}
        okText={t('queryFavorites.save')}
        confirmLoading={saving}
        onOk={() => void submit()}
        onCancel={() => setOpen(false)}
        width={560}
      >
        <Form layout="vertical">
          <Form.Item label={t('queryFavorites.name')} required>
            <Input
              autoFocus
              maxLength={120}
              value={name}
              placeholder={t('queryFavorites.e-g-slow-queries')}
              onChange={(event) => setName(event.target.value)}
              onPressEnter={() => void submit()}
            />
          </Form.Item>
          <Form.Item label="SQL" style={{ marginBottom: 0 }}>
            <Input.TextArea
              className="mono"
              rows={8}
              value={draftSql}
              onChange={(event) => setDraftSql(event.target.value)}
            />
          </Form.Item>
        </Form>
      </Modal>
    </>
  )
}

/**
 * Names an unsaved statement after its first meaningful line, so the dialog is
 * one Enter away from done. Comments are skipped and the result is capped to
 * keep the dropdown readable.
 */
function suggestName(sql: string): string {
  const line = sql
    .split('\n')
    .map((entry) => entry.trim())
    .find((entry) => entry && !entry.startsWith('--') && !entry.startsWith('/*'))
  if (!line) return ''
  return line.replace(/\s+/g, ' ').slice(0, 60)
}
