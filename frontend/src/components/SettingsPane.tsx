import { useCallback, useEffect, useState, type ReactNode } from 'react'
import {
  Alert,
  App as AntApp,
  Button,
  Descriptions,
  Empty,
  InputNumber,
  Segmented,
  Select,
  Space,
  Tag,
  Tooltip,
  Typography,
} from 'antd'
import { FolderOpenOutlined, ReloadOutlined, SettingOutlined, SwapOutlined } from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type { ChangeLogSettings, DataDirInfo, DataDirMoveResult } from '../api/types'
import { AboutProject } from './AboutProject'
import { MockPlaceholderSettings } from './MockPlaceholderSettings'
import { useAppStore } from '../store/appStore'
import { CODE_LANGUAGES } from '../lib/codegen'
import { useSectionScroll } from '../lib/sectionScroll'
import { t, tn, useLanguage, LANGUAGE_CHOICES } from '../lib/i18n'
import type { Language, MessageKey } from '../lib/i18n'
import { THEME_MODES, themeHint, themeLabel, type ThemeMode } from '../lib/theme'

/** The pages of the settings, in the order they are shown. */
type SettingsTab = 'appearance' | 'language' | 'code' | 'mock' | 'generation' | 'data' | 'about'

// The names are held as keys rather than as words: this list is built once, when
// the module is loaded, and a word read here would freeze the page in whichever
// language happened to be in force at that moment.
const TABS: { key: SettingsTab; labelKey: MessageKey }[] = [
  { key: 'appearance', labelKey: 'settingsPane.appearance' },
  { key: 'language', labelKey: 'settingsPane.language' },
  { key: 'code', labelKey: 'settingsPane.code-generation' },
  { key: 'mock', labelKey: 'settingsPane.mock-placeholders' },
  { key: 'generation', labelKey: 'settingsPane.data-generation' },
  { key: 'data', labelKey: 'settingsPane.data-folder' },
  { key: 'about', labelKey: 'settingsPane.about' },
]

/** The id a section carries, so the nav can point at it and the scroll can find it. */
const sectionId = (key: SettingsTab) => `dm-settings-${key}`

/** The sections' keys, in the order they are shown; the scroll is read in this order. */
const SECTION_KEYS = TABS.map((entry) => entry.key)

/** The languages a code window can open in, listed by the name they go by. */
const LANGUAGE_OPTIONS = [...CODE_LANGUAGES]
  .sort((a, b) => a.label.localeCompare(b.label))
  .map((language) => ({ value: language.id, label: language.label }))

/**
 * Program settings: how the window looks, what the code windows open in, what
 * mock placeholders exist, how much one data generation run may write, where the
 * data is kept, and what this build is.
 *
 * This is a page rather than a dialog, like the other pages the rail names: it is
 * somewhere the user goes and comes back from, not something that opens over
 * what they were doing. Everything on it is applied as it is changed — the theme
 * and the code language go to the state file through the store and the data
 * directory is the backend's — so there is nothing to cancel and no choice can be
 * lost by walking away from it.
 *
 * The one operation with a real consequence is the data-directory move, and its
 * answer (which files moved, what was left behind, what could not be deleted) is
 * shown in full rather than being reduced to a success message.
 */
export function SettingsPane() {
  // The page is mounted once and then kept behind the others, so reading it again
  // is tied to coming back to the front rather than to being built: the directory
  // can have changed behind our back (a second window, a pointer file edited by
  // hand), and the listing carries file sizes.
  const active = useAppStore((s) => s.page === 'settings')
  const theme = useAppStore((s) => s.theme)
  const setTheme = useAppStore((s) => s.setTheme)
  // The interface language is not kept in the store — every string is read
  // through `t`, and this is how a component follows a switch.
  const language = useLanguage()
  const setUiLanguage = useAppStore((s) => s.setUiLanguage)
  const codegenLanguage = useAppStore((s) => s.codegenLanguage)
  const setCodegenLanguage = useAppStore((s) => s.setCodegenLanguage)
  const dataDir = useAppStore((s) => s.dataDir)
  const refreshDataDir = useAppStore((s) => s.refreshDataDir)
  const moveDataDir = useAppStore((s) => s.moveDataDir)

  const { message } = AntApp.useApp()
  const [busy, setBusy] = useState<'pick' | 'move' | 'reset' | undefined>()
  const [outcome, setOutcome] = useState<DataDirMoveResult | undefined>()
  const [error, setError] = useState<string | undefined>()

  // The nav and the scroll box are one thing: the highlight follows the scroll
  // and a click moves the scroll, so the two always agree about which section is
  // being read. See `useSectionScroll`.
  const { current: tab, bodyRef, onScroll, goTo } = useSectionScroll({
    keys: SECTION_KEYS,
    idOf: sectionId,
    active,
  })
  // Re-read on coming back to the front, and whenever a move left something to
  // report: what the folder holds is what the page is about.
  useEffect(() => {
    if (!active) return
    setOutcome(undefined)
    setError(undefined)
    void refreshDataDir().catch((err) => setError(toMessage(err)))
  }, [active, refreshDataDir])

  const pickAndMove = useCallback(
    async (reset: boolean) => {
      setError(undefined)
      setOutcome(undefined)
      try {
        let target = ''
        if (!reset) {
          setBusy('pick')
          target = await api.pickDataDirectory(
            t('settingsPane.choose-where-db-manager-keeps-its-data'),
          )
          // Cancelling the chooser is not an error, and must not move anything.
          if (!target) return
        }
        setBusy(reset ? 'reset' : 'move')
        const result = await moveDataDir(target)
        setOutcome(result)
        message.success(
          reset ? t('settingsPane.the-data-is-back-in-the-default-folder') : t('settingsPane.data-moved-to', { path: result.info.path }),
        )
      } catch (err) {
        // The data may well have moved even when this fires: the backend says so
        // when it could not switch the running app over. The listing is re-read
        // either way, and the message is shown as it came.
        setError(toMessage(err))
        await refreshDataDir().catch(() => undefined)
      } finally {
        setBusy(undefined)
      }
    },
    [message, moveDataDir, refreshDataDir],
  )

  return (
    <div className="dm-pane">
      <div className="dm-editor-toolbar">
        <SettingOutlined style={{ opacity: 0.7 }} />
        <Typography.Text strong>{t('settingsPane.settings')}</Typography.Text>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {t('settingsPane.settings-hint')}
        </Typography.Text>
      </div>

      <div className="dm-settings-page">
        <nav className="dm-settings-nav" role="tablist" aria-label={t('settingsPane.settings')}>
          {TABS.map((entry) => (
            <button
              key={entry.key}
              type="button"
              role="tab"
              aria-selected={tab === entry.key}
              aria-controls={sectionId(entry.key)}
              className={`dm-settings-nav-item${tab === entry.key ? ' is-active' : ''}`}
              onClick={() => goTo(entry.key)}
            >
              {t(entry.labelKey)}
            </button>
          ))}
        </nav>

        {/* Every section lives in one scroll box: the nav beside it is read back
            from that scroll, and a click moves it, so the two always agree. */}
        <div className="dm-settings-body" ref={bodyRef} onScroll={onScroll}>
          <section
            className="dm-settings-section"
            id={sectionId('appearance')}
            role="tabpanel"
            aria-label={t('settingsPane.appearance')}
          >
            <h2 className="dm-settings-section-title">{t('settingsPane.appearance')}</h2>
            <SettingRow
              title={t('settingsPane.display-theme')}
              hint={t('settingsPane.display-theme-hint')}
            >
              <Segmented
                value={theme}
                options={THEME_MODES.map((mode) => ({
                  value: mode.value,
                  label: (
                    <Tooltip title={themeHint(mode.value)}>
                      <span>{themeLabel(mode.value)}</span>
                    </Tooltip>
                  ),
                }))}
                onChange={(value) => setTheme(value as ThemeMode)}
              />
            </SettingRow>
          </section>

          <section
            className="dm-settings-section"
            id={sectionId('language')}
            role="tabpanel"
            aria-label={t('settingsPane.language')}
          >
            <h2 className="dm-settings-section-title">{t('settingsPane.language')}</h2>
            <SettingRow
              title={t('settingsPane.interface-language')}
              hint={t('settingsPane.interface-language-hint')}
            >
              <Segmented
                value={language}
                options={LANGUAGE_CHOICES.map((choice) => ({
                  value: choice.value,
                  label: (
                    <Tooltip title={choice.hint}>
                      <span>{choice.label}</span>
                    </Tooltip>
                  ),
                }))}
                onChange={(value) => setUiLanguage(value as Language)}
              />
            </SettingRow>
          </section>

          <section
            className="dm-settings-section"
            id={sectionId('code')}
            role="tabpanel"
            aria-label={t('settingsPane.code-generation')}
          >
            <h2 className="dm-settings-section-title">{t('settingsPane.code-generation')}</h2>
            <SettingRow
              title={t('settingsPane.default-code-language')}
              hint={t('settingsPane.default-code-language-hint')}
            >
              <Select
                showSearch
                optionFilterProp="label"
                style={{ width: 240 }}
                value={codegenLanguage}
                options={LANGUAGE_OPTIONS}
                onChange={setCodegenLanguage}
              />
            </SettingRow>
          </section>

          <section
            className="dm-settings-section"
            id={sectionId('mock')}
            role="tabpanel"
            aria-label={t('settingsPane.mock-placeholders')}
          >
            <h2 className="dm-settings-section-title">{t('settingsPane.mock-placeholders')}</h2>
            <MockPlaceholderSettings />
          </section>

          <section
            className="dm-settings-section"
            id={sectionId('generation')}
            role="tabpanel"
            aria-label={t('settingsPane.data-generation')}
          >
            <h2 className="dm-settings-section-title">{t('settingsPane.data-generation')}</h2>
            <DataGenRows />
          </section>

          <section
            className="dm-settings-section"
            id={sectionId('data')}
            role="tabpanel"
            aria-label={t('settingsPane.data-folder')}
          >
            <h2 className="dm-settings-section-title">{t('settingsPane.data-folder')}</h2>
            <SettingRow
              title={t('settingsPane.where-the-data-lives')}
              hint={t('settingsPane.where-the-data-lives-hint')}
            >
              <DataDirPath info={dataDir} />
              <Space wrap>
                <Button
                  icon={<FolderOpenOutlined />}
                  onClick={() => void api.revealInExplorer(dataDir?.path ?? '')}
                >
                  {t('settingsPane.open-folder')}
                </Button>
                <Button
                  icon={<SwapOutlined />}
                  loading={busy === 'pick' || busy === 'move'}
                  onClick={() => void pickAndMove(false)}
                >
                  {t('settingsPane.change')}
                </Button>
                <Button
                  icon={<ReloadOutlined />}
                  disabled={dataDir?.isDefault ?? true}
                  loading={busy === 'reset'}
                  onClick={() => void pickAndMove(true)}
                >
                  {t('settingsPane.use-the-default')}
                </Button>
              </Space>
            </SettingRow>

            {error ? (
              <Alert type="error" showIcon title={t('settingsPane.the-move-did-not-finish-cleanly')} description={error} />
            ) : null}

            {outcome ? <MoveReport result={outcome} /> : null}

            <ChangeLogSize />

            <DataDirFiles info={dataDir} />
          </section>

          <section
            className="dm-settings-section"
            id={sectionId('about')}
            role="tabpanel"
            aria-label={t('settingsPane.about')}
          >
            <h2 className="dm-settings-section-title">{t('settingsPane.about')}</h2>
            <AboutProject />
          </section>
        </div>
      </div>
    </div>
  )
}

/** One labelled block: a title, an explanation, then the controls. */
function SettingRow({
  title,
  hint,
  children,
}: {
  title: string
  hint?: string
  children: ReactNode
}) {
  return (
    <section className="dm-settings-row">
      <div className="dm-settings-row-title">{title}</div>
      {hint ? <div className="dm-settings-row-hint">{hint}</div> : null}
      <div className="dm-settings-row-body">{children}</div>
    </section>
  )
}

/** The path in use, tagged with whether it is the default or a choice. */
function DataDirPath({ info }: { info?: DataDirInfo }) {
  return (
    <div className="dm-settings-path">
      <span className="mono dm-settings-path-value" title={info?.path}>
        {info?.path ?? '…'}
      </span>
      {info ? (
        info.isDefault ? (
          <Tag color="default">{t('settingsPane.default')}</Tag>
        ) : (
          <Tag color="green">{t('settingsPane.custom')}</Tag>
        )
      ) : null}
    </div>
  )
}

/** How big one log file gets before it is moved aside as an archive. */
function ChangeLogSize() {
  const { message } = AntApp.useApp()
  const [settings, setSettings] = useState<ChangeLogSettings | undefined>()
  const [value, setValue] = useState<number | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | undefined>()
  // Read when the page comes to the front rather than kept in the app store:
  // this is the only place that shows it, and the threshold is the backend's to
  // hold — it reads it at append time, so what is on screen has to be read back
  // rather than remembered.
  const active = useAppStore((s) => s.page === 'settings')
  useEffect(() => {
    if (!active) return
    void api
      .changeLogSettings()
      .then((current) => {
        setSettings(current)
        setValue(current.maxEntries)
      })
      .catch((err) => setError(toMessage(err)))
  }, [active])

  const save = async (maxEntries: number) => {
    if (!settings) return
    setBusy(true)
    setError(undefined)
    try {
      // What comes back is what is in force, which is the backend's decision and
      // not something this page gets to assume.
      const saved = await api.saveChangeLogSettings({ ...settings, maxEntries })
      setSettings(saved)
      setValue(saved.maxEntries)
      message.success(
        t('settingsPane.rotation-size-saved', {
          n: saved.maxEntries.toLocaleString(),
        }),
      )
    } catch (err) {
      setError(toMessage(err))
    } finally {
      setBusy(false)
    }
  }

  const changed = settings !== undefined && value !== null && value !== settings.maxEntries

  return (
    <SettingRow
      title={t('settingsPane.change-log')}
      hint={t('settingsPane.change-log-hint')}
    >
      {error ? (
        <Alert
          type="error"
          showIcon
          title={
            settings
              ? t('settingsPane.the-rotation-size-could-not-be-saved')
              : t('settingsPane.the-rotation-size-could-not-be-read')
          }
          description={error}
        />
      ) : null}
      <Space wrap>
        <InputNumber
          min={settings?.min ?? 100}
          max={settings?.max ?? 100000}
          step={100}
          disabled={settings === undefined}
          value={value}
          onChange={setValue}
          addonAfter={t('settingsPane.statements')}
          style={{ width: 220 }}
        />
        <Button
          type="primary"
          loading={busy}
          disabled={!changed || value === null}
          onClick={() => value !== null && void save(value)}
        >
          {t('settingsPane.save')}
        </Button>
        {settings ? (
          <Tooltip
            title={t('settingsPane.range', {
              min: settings.min.toLocaleString(),
              max: settings.max.toLocaleString(),
            })}
          >
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {settings.maxEntries === settings.default
                ? t('settingsPane.by-default', { n: settings.default.toLocaleString() })
                : t('settingsPane.changed-from', { n: settings.default.toLocaleString() })}
            </Typography.Text>
          </Tooltip>
        ) : null}
      </Space>
    </SettingRow>
  )
}

/** How many rows one data generation run may write. */
function DataGenRows() {
  const { message } = AntApp.useApp()
  const settings = useAppStore((s) => s.dataGenSettings)
  const refresh = useAppStore((s) => s.refreshDataGenSettings)
  const store = useAppStore((s) => s.saveDataGenSettings)
  const [value, setValue] = useState<number | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | undefined>()
  // Read when the page comes to the front, like the rotation size below: the
  // number belongs to the backend and the data generation window reads it back
  // from the store, so what is on screen has to be what would be used rather than
  // what this page last saw.
  const active = useAppStore((s) => s.page === 'settings')
  useEffect(() => {
    if (!active) return
    void refresh()
      .then((current) => {
        setValue(current.maxRows)
        setError(undefined)
      })
      .catch((err) => setError(toMessage(err)))
  }, [active, refresh])

  const apply = async (maxRows: number) => {
    if (!settings) return
    setBusy(true)
    setError(undefined)
    try {
      // What comes back is what is in force, which is the backend's decision and
      // not something this page gets to assume.
      const saved = await store({ ...settings, maxRows })
      setValue(saved.maxRows)
      message.success(
        t('settingsPane.a-run-may-now-write-rows', { n: saved.maxRows.toLocaleString() }),
      )
    } catch (err) {
      setError(toMessage(err))
    } finally {
      setBusy(false)
    }
  }

  const changed = settings !== undefined && value !== null && value !== settings.maxRows

  return (
    <SettingRow
      title={t('settingsPane.rows-per-run')}
      hint={t('settingsPane.rows-per-run-hint')}
    >
      {error ? (
        <Alert
          type="error"
          showIcon
          title={
            settings
              ? t('settingsPane.the-row-limit-could-not-be-saved')
              : t('settingsPane.the-row-limit-could-not-be-read')
          }
          description={error}
        />
      ) : null}
      <Space wrap>
        <InputNumber
          min={settings?.min ?? 100}
          max={settings?.max ?? 100000000}
          step={1000}
          disabled={settings === undefined}
          value={value}
          onChange={setValue}
          addonAfter={t('settingsPane.rows')}
          style={{ width: 220 }}
        />
        <Button
          type="primary"
          loading={busy}
          disabled={!changed || value === null}
          onClick={() => value !== null && void apply(value)}
        >
          {t('settingsPane.save')}
        </Button>
        {settings ? (
          <Tooltip
            title={t('settingsPane.range', {
              min: settings.min.toLocaleString(),
              max: settings.max.toLocaleString(),
            })}
          >
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {settings.maxRows === settings.default
                ? t('settingsPane.by-default', { n: settings.default.toLocaleString() })
                : t('settingsPane.changed-from', { n: settings.default.toLocaleString() })}
            </Typography.Text>
          </Tooltip>
        ) : null}
      </Space>
    </SettingRow>
  )
}

/** What the move did, file by file. */
function MoveReport({ result }: { result: DataDirMoveResult }) {
  return (
    <Alert
      type={result.remaining.length > 0 ? 'warning' : 'success'}
      showIcon
      title={tn('settingsPane.moved-file-to', result.moved.length, { path: result.info.path })}
      description={
        <div className="dm-settings-report">
          {result.moved.length > 0 ? (
            <div>
              <strong>{t('settingsPane.moved')}</strong> <span className="mono">{result.moved.join(', ')}</span>
            </div>
          ) : null}
          {result.leftBehind.length > 0 ? (
            <div>
              <strong>{t('settingsPane.left-where-they-were')}</strong> {t('settingsPane.not-files-this-program-writes')}{' '}
              <span className="mono">{result.leftBehind.join(', ')}</span>
            </div>
          ) : null}
          {result.remaining.length > 0 ? (
            <div>
              <strong>{t('settingsPane.copied-but-not-deleted-from-the-old-folder')}</strong>{' '}
              {t('settingsPane.a-lock-a-read-only-folder')}{' '}
              <span className="mono">{result.remaining.join(', ')}</span>
            </div>
          ) : null}
          <div className="dm-settings-report-note">
            {t('settingsPane.move-note')}
          </div>
        </div>
      }
    />
  )
}

/** What is in the directory right now. */
function DataDirFiles({ info }: { info?: DataDirInfo }) {
  if (!info) return null
  if (info.files.length === 0) {
    return (
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description={t('settingsPane.empty-folder')}
      />
    )
  }
  return (
    <Descriptions
      size="small"
      column={1}
      bordered
      className="dm-settings-files"
      title={t('settingsPane.what-is-in-it', { totalBytes: formatBytes(info.totalBytes) })}
    >
      {info.files.map((file) => (
        <Descriptions.Item key={file.name} label={<span className="mono">{file.name}</span>}>
          {purposeOf(file.name)} · {formatBytes(file.bytes)}
          {/* A folder says how much is in it as well: its size counts the files
              but not how many there are, which is what a reader wants to know. */}
          {file.dir && file.count ? <> {tn('settingsPane.file-count', file.count)}</> : ''}
        </Descriptions.Item>
      ))}
    </Descriptions>
  )
}

/**
 * What each file holds, so the listing reads as an answer rather than a dump.
 *
 * The names are message keys, not words: this map is built once, when the module
 * is loaded, and a word read here would freeze the listing in whichever language
 * was in force at that moment.
 */
const FILE_PURPOSE: Record<string, MessageKey> = {
  'connections.json': 'settingsPane.file-connections-json',
  'queries.json': 'settingsPane.file-queries-json',
  'layout.json': 'settingsPane.file-layout-json',
  'state.json': 'settingsPane.file-state-json',
  'secret.key': 'settingsPane.file-secret-key',
  'datagen.json': 'settingsPane.file-datagen-json',
  'changelog.jsonl': 'settingsPane.file-changelog-jsonl',
  'changelog.json': 'settingsPane.file-changelog-json',
  '.mock': 'settingsPane.file-mock',
  '.query': 'settingsPane.file-query',
  log: 'settingsPane.file-log',
}

/**
 * An archived log — `<date>-<n>.log` — is named when it is rotated out, so it
 * cannot be listed above by name. The rule mirrors the backend's, which is the
 * one that decides what counts as an archive. The log folder the app writes in
 * now is listed as one row, so a name that matches here is one an older build
 * left in the data directory itself.
 */
const ARCHIVED_LOG = /^\d{8}-[1-9]\d*\.log$/

/** What one file holds, so the listing reads as an answer rather than a dump. */
function purposeOf(name: string): string {
  const key = FILE_PURPOSE[name]
  if (key) return t(key)
  return t(
    ARCHIVED_LOG.test(name)
      ? 'settingsPane.archived-change-log-by-an-older-build'
      : 'settingsPane.written-by-this-program',
  )
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}
