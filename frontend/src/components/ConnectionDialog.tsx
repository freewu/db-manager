import { useCallback, useEffect, useState } from 'react'
import { Alert, App as AntApp, Button, Form, Modal, Space, Switch } from 'antd'
import { ThunderboltOutlined } from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type {
  ConnectionConfig,
  DriverInfo,
  DriverType,
  SSLConfig,
  TestResult,
} from '../api/types'
import { driverForm } from '../connection'
import {
  COLOR_SWATCHES,
  CONNECTION_TABS,
  DisplayNameField,
  LabelColourField,
  connectionTabSlot,
  type ConnectionTab,
  type ConnectionValues,
} from '../connection/shared'
import { driverIcon } from '../lib/assets'
import { useAppStore } from '../store/appStore'

/**
 * Which tab owns a field, so a failed validation can jump to the one it is on.
 * The dialog mounts every tab (hidden), which is what lets one Save press
 * check all of them at once.
 */
const FIELD_TAB: Record<string, ConnectionTab> = {
  name: 'basic',
  host: 'basic',
  port: 'basic',
  username: 'basic',
  password: 'basic',
  database: 'basic',
  filePath: 'basic',
  savePassword: 'basic',
  readOnly: 'options',
  color: 'options',
  params: 'advanced',
  sslMode: 'security',
  sslCAFile: 'security',
  sslCertFile: 'security',
  sslKeyFile: 'security',
}

/**
 * The shell around a driver's page.
 *
 * It owns the modal, the form instance, the values that are the same whatever
 * you are connecting to (name, read-only, colour) and the test/save path; the
 * fields that only make sense for one engine live in `src/connection/*.tsx`
 * and are dispatched on the driver the draft carries.
 *
 * The driver itself is not editable here, and not a form field either: it was
 * picked from the "new connection" menu, which is the only thing that decides
 * it, and the dialog reads it off the draft (see `driverType`). Switching
 * engines means picking a different entry there — the dialog only says which
 * engine it is, with the vendor logo next to the title.
 */
export function ConnectionDialog() {
  const open = useAppStore((s) => s.editorOpen)
  const draft = useAppStore((s) => s.editorDraft)
  const drivers = useAppStore((s) => s.drivers)
  const closeEditor = useAppStore((s) => s.closeConnectionEditor)
  const saveConnection = useAppStore((s) => s.saveConnection)

  const { message } = AntApp.useApp()
  const [form] = Form.useForm<ConnectionValues>()
  const [tab, setTab] = useState<ConnectionTab>('basic')
  const [testing, setTesting] = useState(false)
  const [saving, setSaving] = useState(false)
  /**
   * The last test outcome, shown above the button that asked for it.
   *
   * `seq` counts the attempts: pressing Test again with the same outcome has to
   * restart the auto-dismiss timer, and the panel has to stay mounted while it
   * does so (remounting it would blink).
   */
  const [test, setTest] = useState<{ result: TestResult; seq: number } | null>(null)

  // The draft is the only source of the driver; fall back to the first
  // implemented one so a malformed draft still lands on a usable page.
  const driverInfo =
    driverFor(drivers, draft?.driver) ?? drivers.find((d) => d.implemented)
  const page = driverForm(driverInfo?.type)
  const isFile = driverInfo?.requiresFile ?? false
  const showNetwork = !isFile
  const showCredentials = !isFile

  // One tab per page: Basic and the shell's Options are always there, the
  // driver adds Security (TLS) and Advanced (extra parameters) when it has
  // something to put in them.
  const tabs = CONNECTION_TABS.filter((entry) => {
    if (entry.key === 'basic' || entry.key === 'options') return true
    const slot = connectionTabSlot(entry.key)
    return slot ? Boolean(page?.[slot]) : false
  })

  // Reset the form whenever the dialog opens with a different profile. A draft
  // coming from the menu carries only a driver, so everything else falls back
  // to that driver's defaults.
  useEffect(() => {
    if (!open) return
    setTab('basic')
    const driver = driverFor(drivers, draft?.driver) ?? drivers.find((d) => d.implemented)
    const values = blankValues(driver)
    if (draft) {
      Object.assign(values, {
        name: draft.name ?? '',
        host: draft.host ?? values.host,
        port: draft.port ?? values.port,
        username: draft.username ?? '',
        // Never pre-fill the stored secret; leaving it blank keeps it as-is.
        password: '',
        database: draft.database ?? values.database,
        filePath: draft.filePath ?? '',
        params: Object.entries(draft.params ?? {}).map(([key, value]) => ({ key, value })),
        sslMode: draft.ssl?.mode ?? 'disable',
        sslCAFile: draft.ssl?.caFile ?? '',
        sslCertFile: draft.ssl?.certFile ?? '',
        sslKeyFile: draft.ssl?.keyFile ?? '',
        readOnly: draft.readOnly ?? false,
        savePassword: draft.savePassword ?? false,
        color: draft.color ?? COLOR_SWATCHES[0],
      })
    }
    form.setFieldsValue(values)
  }, [draft, drivers, form, open])

  const collect = useCallback(
    (values: ConnectionValues): ConnectionConfig => {
      const params: Record<string, string> = {}
      for (const entry of values.params ?? []) {
        const key = entry?.key?.trim()
        if (key) params[key] = entry?.value ?? ''
      }
      const ssl: SSLConfig | undefined =
        values.sslMode && values.sslMode !== 'disable'
          ? {
              mode: values.sslMode,
              caFile: values.sslCAFile || undefined,
              certFile: values.sslCertFile || undefined,
              keyFile: values.sslKeyFile || undefined,
            }
          : undefined

      return {
        id: draft?.id ?? '',
        name: values.name.trim(),
        // Read off the draft, never out of `values`: the driver is picked in the
        // "new connection" menu before this dialog opens, `validateFields()`
        // only returns registered fields, and a `driver` that no input owns came
        // back empty — which the backend answers with "driver  is not available
        // yet" for both Test connection and Save.
        driver: driverType(driverInfo),
        host: showNetwork ? values.host?.trim() : undefined,
        port: showNetwork ? values.port : undefined,
        username: showCredentials ? values.username : undefined,
        password: values.password ? values.password : undefined,
        database: values.database?.trim() || undefined,
        filePath: isFile ? values.filePath?.trim() : undefined,
        params: Object.keys(params).length > 0 ? params : undefined,
        ssl,
        readOnly: values.readOnly,
        savePassword: values.savePassword,
        color: values.color,
        hasPassword: draft?.hasPassword,
      }
    },
    [driverInfo, draft, isFile, showCredentials, showNetwork],
  )

  /**
   * Validate every tab at once.
   *
   * All panes stay mounted (the inactive ones are hidden), so a rule on a tab
   * you are not looking at is still enforced; when something is wrong we switch
   * to the tab holding the first offending field, otherwise its red text would
   * be off screen.
   */
  const validateAll = useCallback(async (): Promise<ConnectionValues | undefined> => {
    try {
      return await form.validateFields()
    } catch (error) {
      const [first] = (error as { errorFields?: { name: unknown[] }[] }).errorFields ?? []
      const field = typeof first?.name?.[0] === 'string' ? (first.name[0] as string) : undefined
      const owner = field ? FIELD_TAB[field] : undefined
      if (owner) setTab(owner)
      return undefined
    }
  }, [form])

  const handleTest = useCallback(async () => {
    const values = await validateAll()
    if (!values) return
    setTesting(true)
    const show = (result: TestResult) =>
      setTest((current) => ({ result, seq: (current?.seq ?? 0) + 1 }))
    try {
      show(await api.testConnection(collect(values)))
    } catch (error) {
      show({ ok: false, message: toMessage(error), latencyMs: 0 })
    } finally {
      setTesting(false)
    }
  }, [collect, validateAll])

  const handleSave = useCallback(async () => {
    const values = await validateAll()
    if (!values) return
    setSaving(true)
    try {
      const saved = await saveConnection(collect(values), draft?.groupId)
      message.success(`Saved “${saved.name}”`)
      closeEditor()
    } catch (error) {
      message.error(toMessage(error))
    } finally {
      setSaving(false)
    }
  }, [closeEditor, collect, draft, message, saveConnection, validateAll])

  const icon = driverIcon(driverInfo?.type)
  const title = draft?.id
    ? `Edit ${draft.name}`
    : driverInfo
      ? `New ${driverInfo.displayName} connection`
      : 'New connection'

  return (
    <Modal
      open={open}
      title={
        <span className="dm-dialog-title">
          {icon ? (
            <img src={icon} alt="" draggable={false} className="dm-driver-icon is-large" />
          ) : null}
          {title}
        </span>
      }
      width={620}
      onCancel={closeEditor}
      destroyOnHidden
      footer={
        <Space>
          <Button onClick={closeEditor}>Cancel</Button>
          {/* The answer to a test belongs over the button that asked for it:
              the values it judges are the ones in this dialog. */}
          <div className="dm-test-slot">
            {test ? (
              <TestNotice
                result={test.result}
                seq={test.seq}
                onClose={() => setTest(null)}
              />
            ) : null}
            <Button icon={<ThunderboltOutlined />} loading={testing} onClick={() => void handleTest()}>
              Test connection
            </Button>
          </div>
          <Button type="primary" loading={saving} onClick={() => void handleSave()}>
            Save
          </Button>
        </Space>
      }
    >
      <Form<ConnectionValues>
        form={form}
        layout="vertical"
        initialValues={blankValues(driverInfo)}
      >
        {driverInfo && !page ? (
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            title={`No ${driverInfo.displayName} form yet`}
            description="The driver is registered, but its connection page has not been written."
          />
        ) : null}

        {/* Which engine this is was decided by the menu, so the dialog only
            states it: tabs, not a driver picker. */}
        {tabs.length > 1 ? (
          <nav className="dm-form-tabs" role="tablist" aria-label="Connection settings">
            {tabs.map((entry) => (
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
        ) : null}

        {/* The driver's own pages, one pane each. Inactive panes stay mounted
            but hidden: their rules still run on save, and their values stay in
            the form store, so switching tabs never loses what was typed. */}
        {tabs.map((entry) => {
          const slot = connectionTabSlot(entry.key)
          const Fields = slot ? page?.[slot] : undefined
          return (
            <div key={entry.key} hidden={tab !== entry.key}>
              {entry.key === 'basic' ? <DisplayNameField /> : null}
              {Fields && driverInfo ? (
                <Fields form={form} driver={driverInfo} draft={draft} />
              ) : null}
              {entry.key === 'options' ? <OptionsFields /> : null}
            </div>
          )
        })}
      </Form>
    </Modal>
  )
}

/**
 * How long a test result stays on screen, in seconds. Hovering holds it open,
 * which is what a long refusal needs.
 */
const TEST_TOAST_MS = 6

/**
 * The test result, floating directly above the "Test connection" button.
 *
 * A panel pinned to the button rather than a toast in the corner of the window:
 * the answer is about the values in this dialog, and it has to travel with it
 * (the window can be moved, and the dialog is the only thing on screen that
 * matters while it is open). It dismisses itself after `TEST_TOAST_MS`, or on
 * hover it waits until the pointer leaves, so a long server message can be read
 * to the end.
 */
function TestNotice({
  result,
  seq,
  onClose,
}: {
  result: TestResult
  seq: number
  onClose: () => void
}) {
  const [held, setHeld] = useState(false)

  useEffect(() => {
    if (held) return undefined
    const timer = window.setTimeout(onClose, TEST_TOAST_MS * 1000)
    return () => window.clearTimeout(timer)
  }, [held, onClose, seq])

  const detail = [
    result.serverVersion,
    result.connectedDatabase ? `db=${result.connectedDatabase}` : null,
    `${result.latencyMs} ms`,
    result.databaseCount !== undefined ? `${result.databaseCount} databases` : null,
  ]
    .filter(Boolean)
    .join(' · ')

  return (
    <div
      className="dm-test-notice"
      role="status"
      onMouseEnter={() => setHeld(true)}
      onMouseLeave={() => setHeld(false)}
    >
      <Alert
        type={result.ok ? 'success' : 'error'}
        showIcon
        closable
        onClose={onClose}
        title={result.ok ? 'Connection succeeded' : 'Connection failed'}
        description={
          <div className="mono" style={{ fontSize: 12, whiteSpace: 'pre-wrap' }}>
            {[result.message, result.ok ? detail : ''].filter(Boolean).join('\n')}
          </div>
        }
      />
    </div>
  )
}

function driverFor(drivers: DriverInfo[], type: DriverType | undefined): DriverInfo | undefined {
  return type ? drivers.find((d) => d.type === type) : undefined
}

/**
 * The driver name the backend's registry knows the draft by.
 *
 * MySQL is only the fallback for a registry that offers nothing at all (the
 * dialog has no page to show then and says so); with a real registry the draft's
 * own driver is always found.
 */
function driverType(driver: DriverInfo | undefined): DriverType {
  return driver?.type ?? 'mysql'
}

/**
 * The Options page: what a profile says about itself rather than about the
 * server. Every driver gets it, so the dialog draws it for all of them.
 */
function OptionsFields() {
  return (
    <div className="dm-form-row">
      <Form.Item name="readOnly" label="Read only" valuePropName="checked">
        <Switch />
      </Form.Item>
      <LabelColourField />
    </div>
  )
}

/** A blank profile for a driver: its port, its default database, no secrets. */
function blankValues(driver: DriverInfo | undefined): Omit<ConnectionValues, 'driver'> {
  return {
    name: '',
    host: 'localhost',
    port: driver?.defaultPort,
    username: '',
    password: '',
    database: driver?.defaultDatabase ?? '',
    filePath: '',
    params: [],
    sslMode: 'disable',
    readOnly: false,
    savePassword: false,
    color: COLOR_SWATCHES[0],
  }
}
