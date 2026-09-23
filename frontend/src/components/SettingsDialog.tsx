import { useCallback, useEffect, useState, type ReactNode } from 'react'
import {
  Alert,
  App as AntApp,
  Button,
  Descriptions,
  Empty,
  Modal,
  Segmented,
  Select,
  Space,
  Tag,
  Tooltip,
} from 'antd'
import { FolderOpenOutlined, ReloadOutlined, SwapOutlined } from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type { DataDirInfo, DataDirMoveResult } from '../api/types'
import { AboutProject } from './AboutProject'
import { MockPlaceholderSettings } from './MockPlaceholderSettings'
import { useAppStore } from '../store/appStore'
import { CODE_LANGUAGES } from '../lib/codegen'
import { THEME_MODES, type ThemeMode } from '../lib/theme'

/** The pages of the dialog, in the order they are shown. */
type SettingsTab = 'appearance' | 'code' | 'mock' | 'data' | 'about'

const TABS: { key: SettingsTab; label: string }[] = [
  { key: 'appearance', label: 'Appearance' },
  { key: 'code', label: 'Code generation' },
  { key: 'mock', label: 'Mock placeholders' },
  { key: 'data', label: 'Data folder' },
  { key: 'about', label: 'About' },
]

/** The languages a code window can open in, listed by the name they go by. */
const LANGUAGE_OPTIONS = [...CODE_LANGUAGES]
  .sort((a, b) => a.label.localeCompare(b.label))
  .map((language) => ({ value: language.id, label: language.label }))

/**
 * Program settings: how the window looks, where the data is kept, and what this
 * build is.
 *
 * The dialog owns no preference of its own — the theme and the code language go
 * to the state file through the store and the data directory is the backend's —
 * so closing it can never lose a choice, and the values it shows are the ones the
 * app is using.
 * The one operation with a real consequence is the data-directory move, and its
 * answer (which files moved, what was left behind, what could not be deleted) is
 * shown in full rather than being reduced to a success message.
 */
export function SettingsDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const theme = useAppStore((s) => s.theme)
  const setTheme = useAppStore((s) => s.setTheme)
  const codegenLanguage = useAppStore((s) => s.codegenLanguage)
  const setCodegenLanguage = useAppStore((s) => s.setCodegenLanguage)
  const dataDir = useAppStore((s) => s.dataDir)
  const refreshDataDir = useAppStore((s) => s.refreshDataDir)
  const moveDataDir = useAppStore((s) => s.moveDataDir)

  const { message } = AntApp.useApp()
  const [tab, setTab] = useState<SettingsTab>('appearance')
  const [busy, setBusy] = useState<'pick' | 'move' | 'reset' | undefined>()
  const [outcome, setOutcome] = useState<DataDirMoveResult | undefined>()
  const [error, setError] = useState<string | undefined>()

  // Re-read on open: the directory can have changed behind our back (a second
  // window, a pointer file edited by hand), and the listing carries file sizes.
  useEffect(() => {
    if (!open) return
    setOutcome(undefined)
    setError(undefined)
    void refreshDataDir().catch((err) => setError(toMessage(err)))
  }, [open, refreshDataDir])

  const pickAndMove = useCallback(
    async (reset: boolean) => {
      setError(undefined)
      setOutcome(undefined)
      try {
        let target = ''
        if (!reset) {
          setBusy('pick')
          target = await api.pickDataDirectory('Choose where DB Manager keeps its data')
          // Cancelling the chooser is not an error, and must not move anything.
          if (!target) return
        }
        setBusy(reset ? 'reset' : 'move')
        const result = await moveDataDir(target)
        setOutcome(result)
        message.success(
          reset ? 'The data is back in the default folder' : `Data moved to ${result.info.path}`,
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
    <Modal
      open={open}
      title="Settings"
      width="90%"
      onCancel={onClose}
      destroyOnHidden
      footer={
        <Space>
          <Button onClick={onClose}>Close</Button>
        </Space>
      }
    >
      <nav className="dm-form-tabs" role="tablist" aria-label="Settings">
        {TABS.map((entry) => (
          <button
            key={entry.key}
            type="button"
            role="tab"
            aria-selected={tab === entry.key}
            className={`dm-form-tab${tab === entry.key ? ' is-active' : ''}`}
            onClick={() => setTab(entry.key)}
          >
            {entry.label}
          </button>
        ))}
      </nav>

      {tab === 'appearance' ? (
        <div className="dm-settings-pane">
          <SettingRow
            title="Display theme"
            hint="On System the window follows the operating system and switches the moment it does."
          >
            <Segmented
              value={theme}
              options={THEME_MODES.map((mode) => ({
                value: mode.value,
                label: (
                  <Tooltip title={mode.hint}>
                    <span>{mode.label}</span>
                  </Tooltip>
                ),
              }))}
              onChange={(value) => setTheme(value as ThemeMode)}
            />
          </SettingRow>
        </div>
      ) : null}

      {tab === 'code' ? (
        <div className="dm-settings-pane">
          <SettingRow
            title="Default code language"
            hint="What a new code window opens in. Each window can be switched to another language on the spot; this is the one it starts from."
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
        </div>
      ) : null}

      {tab === 'mock' ? <MockPlaceholderSettings /> : null}

      {tab === 'data' ? (
        <div className="dm-settings-pane">
          <SettingRow
            title="Data folder"
            hint="Connections, query favourites, window state and the password key live here; moving the folder takes them all along."
          >
            <DataDirPath info={dataDir} />
            <Space wrap>
              <Button
                icon={<FolderOpenOutlined />}
                onClick={() => void api.revealInExplorer(dataDir?.path ?? '')}
              >
                Open folder
              </Button>
              <Button
                icon={<SwapOutlined />}
                loading={busy === 'pick' || busy === 'move'}
                onClick={() => void pickAndMove(false)}
              >
                Change…
              </Button>
              <Button
                icon={<ReloadOutlined />}
                disabled={dataDir?.isDefault ?? true}
                loading={busy === 'reset'}
                onClick={() => void pickAndMove(true)}
              >
                Use the default
              </Button>
            </Space>
          </SettingRow>

          {error ? (
            <Alert type="error" showIcon title="The move did not finish cleanly" description={error} />
          ) : null}

          {outcome ? <MoveReport result={outcome} /> : null}

          <DataDirFiles info={dataDir} />
        </div>
      ) : null}

      {tab === 'about' ? (
        <div className="dm-settings-pane">
          <AboutProject />
        </div>
      ) : null}
    </Modal>
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
        info.isDefault ? <Tag color="default">default</Tag> : <Tag color="green">custom</Tag>
      ) : null}
    </div>
  )
}

/** What the move did, file by file. */
function MoveReport({ result }: { result: DataDirMoveResult }) {
  return (
    <Alert
      type={result.remaining.length > 0 ? 'warning' : 'success'}
      showIcon
      title={`Moved ${result.moved.length} file${result.moved.length === 1 ? '' : 's'} to ${result.info.path}`}
      description={
        <div className="dm-settings-report">
          {result.moved.length > 0 ? (
            <div>
              <strong>Moved:</strong> <span className="mono">{result.moved.join(', ')}</span>
            </div>
          ) : null}
          {result.leftBehind.length > 0 ? (
            <div>
              <strong>Left where they were</strong> (not files this program writes):{' '}
              <span className="mono">{result.leftBehind.join(', ')}</span>
            </div>
          ) : null}
          {result.remaining.length > 0 ? (
            <div>
              <strong>Copied but not deleted from the old folder</strong> (a lock, a read-only
              folder): <span className="mono">{result.remaining.join(', ')}</span>
            </div>
          ) : null}
          <div className="dm-settings-report-note">
            The new folder is in use from now on, and a restart keeps using it.
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
        description="Nothing written yet: this folder gets its first file once you save a connection or change a setting."
      />
    )
  }
  return (
    <Descriptions
      size="small"
      column={1}
      bordered
      className="dm-settings-files"
      title={`What is in it (${formatBytes(info.totalBytes)})`}
    >
      {info.files.map((file) => (
        <Descriptions.Item key={file.name} label={<span className="mono">{file.name}</span>}>
          {FILE_PURPOSE[file.name] ?? 'Written by this program'} · {formatBytes(file.bytes)}
          {/* A folder says how much is in it as well: its size counts the files
              but not how many there are, which is what a reader wants to know. */}
          {file.dir && file.count ? ` · ${file.count} file${file.count === 1 ? '' : 's'}` : ''}
        </Descriptions.Item>
      ))}
    </Descriptions>
  )
}

/** What each file holds, so the listing reads as an answer rather than a dump. */
const FILE_PURPOSE: Record<string, string> = {
  'connections.json': 'Connection profiles, passwords sealed',
  'queries.json': 'Query favourites',
  'layout.json': 'Groups and order of the connection tree',
  'state.json': 'Window state and preferences',
  'secret.key': 'Key that opens the saved passwords — unreadable on another machine',
  '.mock': 'Custom mock placeholders, one file each',
  '.query': 'Saved scripts, one file each',
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}
