import { useCallback, useEffect, useMemo, useState } from 'react'
import { Alert, App as AntApp, Button, Select, Spin, Tooltip, Typography } from 'antd'
import { CodeOutlined, CopyOutlined, ReloadOutlined, SaveOutlined } from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type { TableStructure } from '../api/types'
import { qualifiedName } from '../lib/format'
import { CODE_LANGUAGES, codeFileName, codeLanguageById, fieldsOf, generateCode, tokenizeCode } from '../lib/codegen'
import { useAppStore, type WorkspaceTab } from '../store/appStore'
import { t, tn } from '../lib/i18n'

/** The picker's entries, in the order the settings page lists them too. */
const LANGUAGE_OPTIONS = [...CODE_LANGUAGES]
  .sort((a, b) => a.label.localeCompare(b.label))
  .map((language) => ({ value: language.id, label: language.label }))

interface CodegenPaneProps {
  tab: WorkspaceTab
}

/**
 * Code window: one object's fields as a class, a struct or a record.
 *
 * The object's structure is read once, and the text is produced from it here in
 * the window — so the picker switches language as fast as the browser can
 * repaint, with nothing crossing the bridge and no request to wait for. The
 * language the window opens in is the preference from the settings page; picking
 * another one changes what this window shows, which is a look at another
 * language rather than a change to the preference (that stays in Settings).
 *
 * The text is drawn with the same syntax palette the SQL previews use; which
 * word is a keyword there is `lib/codegen/highlight.ts`'s business, and it is
 * asked for the language on screen at that moment, so the colours follow the
 * picker as quickly as the text does.
 */
export function CodegenPane({ tab }: CodegenPaneProps) {
  const session = useAppStore((s) => s.sessionOf(tab.sessionId))
  const preferred = useAppStore((s) => s.codegenLanguage)
  const { message } = AntApp.useApp()

  // The window's own choice. A window already open keeps the language it was
  // showing when the preference changes, the way an editor keeps its cursor.
  const [language, setLanguage] = useState(preferred)
  const [structure, setStructure] = useState<TableStructure | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const object = tab.object ?? ''
  const database = tab.database ?? session?.database ?? ''
  const schema = tab.schema ?? ''
  const qualified = qualifiedName(session?.driver ?? '', database, schema, object)
  const current = codeLanguageById(language)

  const load = useCallback(async () => {
    if (!object) {
      setLoading(false)
      setError(t('codegenPane.window-not-pointed-at-an-object'))
      return
    }
    setLoading(true)
    setError(null)
    try {
      setStructure(await api.getStructure(tab.sessionId, database, schema, object))
    } catch (err) {
      setError(toMessage(err))
    } finally {
      setLoading(false)
    }
  }, [database, object, schema, tab.sessionId])

  useEffect(() => {
    void load()
  }, [load])

  const code = useMemo(
    () =>
      structure
        ? generateCode(language, {
            object: structure.object.name || object,
            qualified,
            comment: structure.object.comment || undefined,
            fields: fieldsOf(structure),
          })
        : '',
    [language, object, qualified, structure],
  )

  const copy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(code)
      message.success(t('codegenPane.code-copied'))
    } catch (err) {
      message.error(toMessage(err))
    }
  }, [code, message])

  const save = useCallback(async () => {
    if (!code) {
      message.info(t('codegenPane.nothing-to-save'))
      return
    }
    try {
      await api.saveTextFile({
        defaultFilename: codeFileName(language, object),
        content: code,
        filters: [{ displayName: current.label, pattern: `*${current.ext}` }],
      })
      message.success(t('codegenPane.code-saved'))
    } catch (err) {
      message.error(toMessage(err))
    }
  }, [code, current.ext, current.label, language, message, object])

  const fieldCount = structure?.columns.length ?? 0
  const tokens = useMemo(() => tokenizeCode(code, language), [code, language])

  return (
    <div className="dm-pane">
      <div className="dm-editor-toolbar">
        <Select
          size="small"
          showSearch
          optionFilterProp="label"
          style={{ width: 180 }}
          value={language}
          options={LANGUAGE_OPTIONS}
          onChange={setLanguage}
          aria-label={t('codegenPane.language')}
        />
        <Tooltip title={t('codegenPane.copy-the-generated-code')}>
          <Button
            size="small"
            icon={<CopyOutlined />}
            disabled={!code}
            onClick={() => void copy()}
          />
        </Tooltip>
        <Tooltip title={t('codegenPane.save-the-generated-code-to-a-file')}>
          <Button
            size="small"
            icon={<SaveOutlined />}
            disabled={!code}
            onClick={() => void save()}
          />
        </Tooltip>
        <Tooltip title={t('codegenPane.read-the-object-s-fields-again')}>
          <Button size="small" icon={<ReloadOutlined />} loading={loading} onClick={() => void load()} />
        </Tooltip>

        <span className="dm-toolbar-sep" />
        <CodeOutlined style={{ opacity: 0.6 }} />
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {qualified}
        </Typography.Text>

        <div className="dm-toolbar-right">
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {tn('codegenPane.fields', fieldCount)}
          </Typography.Text>
        </div>
      </div>

      {error ? (
        <div style={{ padding: 10, flex: '0 0 auto' }}>
          <Alert
            type="error"
            showIcon
            title={t('codegenPane.could-not-read-the-object-s-fields')}
            description={
              <span className="mono" style={{ fontSize: 12 }}>
                {error}
              </span>
            }
            action={
              <Button size="small" onClick={() => void load()}>
                {t('codegenPane.retry')}
              </Button>
            }
          />
        </div>
      ) : null}

      {!loading && !error && fieldCount === 0 ? (
        <div style={{ padding: 10, flex: '0 0 auto' }}>
          <Alert
            type="info"
            showIcon
            title={t('codegenPane.this-object-has-no-fields')}
            description={t('codegenPane.there-is-nothing-to-map-yet-so-the-generated')}
          />
        </div>
      ) : null}

      {loading && !structure ? (
        <div className="dm-codegen-loading">
          <Spin />
          <Typography.Text type="secondary">{t('codegenPane.reading-the-object-s-fields')}</Typography.Text>
        </div>
      ) : (
        // A failed reload keeps the text it already had on screen: the alert
        // above says what went wrong, and an empty page would say less.
        <pre className="mono dm-codegen">
          {tokens.map((token, index) =>
            token.kind === 'plain' ? (
              token.text
            ) : (
              <span key={index} className={`dm-syntax-${token.kind}`}>
                {token.text}
              </span>
            ),
          )}
        </pre>
      )}
    </div>
  )
}
