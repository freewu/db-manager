import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  App as AntApp,
  Button,
  Divider,
  Form,
  Modal,
  Select,
  Space,
  Switch,
} from 'antd'
import {
  CheckCircleOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'

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
  DisplayNameField,
  LabelColourField,
  type ConnectionValues,
} from '../connection/shared'
import { driverIcon } from '../lib/assets'
import { useAppStore } from '../store/appStore'

/**
 * The shell around a driver's page.
 *
 * It owns the modal, the form instance, the values that are the same whatever
 * you are connecting to (name, read-only, colour) and the test/save path; the
 * fields that only make sense for one engine live in `src/connection/*.tsx`
 * and are dispatched on the selected driver.
 */
export function ConnectionDialog() {
  const open = useAppStore((s) => s.editorOpen)
  const draft = useAppStore((s) => s.editorDraft)
  const drivers = useAppStore((s) => s.drivers)
  const closeEditor = useAppStore((s) => s.closeConnectionEditor)
  const saveConnection = useAppStore((s) => s.saveConnection)

  const { message } = AntApp.useApp()
  const [form] = Form.useForm<ConnectionValues>()
  const [testing, setTesting] = useState(false)
  const [saving, setSaving] = useState(false)
  const [testResult, setTestResult] = useState<TestResult | null>(null)

  const selectedDriver = Form.useWatch('driver', form)

  const driverInfo = useMemo(
    () => drivers.find((d) => d.type === selectedDriver),
    [drivers, selectedDriver],
  )
  const page = driverForm(driverInfo?.type)
  const isFile = driverInfo?.requiresFile ?? false
  const showNetwork = !isFile
  const showCredentials = !isFile

  // Reset the form whenever the dialog opens with a different profile. A draft
  // coming from the picker carries only a driver, so everything else falls back
  // to that driver's defaults.
  useEffect(() => {
    if (!open) return
    setTestResult(null)
    const driver = driverFor(drivers, draft?.driver) ?? drivers.find((d) => d.implemented)
    const values = blankValues(driver)
    if (draft) {
      Object.assign(values, {
        driver: draft.driver,
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

  const handleDriverChange = useCallback(
    (value: DriverType) => {
      const info = drivers.find((d) => d.type === value)
      const currentName = form.getFieldValue('name')
      form.setFieldsValue({
        port: info?.defaultPort,
        database: info?.defaultDatabase ?? '',
        name:
          currentName ||
          (info ? `${info.displayName} @ ${form.getFieldValue('host') ?? 'localhost'}` : ''),
      })
      setTestResult(null)
    },
    [drivers, form],
  )

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
        driver: values.driver,
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
    [draft, isFile, showCredentials, showNetwork],
  )

  const handleTest = useCallback(async () => {
    let values: ConnectionValues
    try {
      values = await form.validateFields()
    } catch {
      return
    }
    setTesting(true)
    setTestResult(null)
    try {
      const result = await api.testConnection(collect(values))
      setTestResult(result)
    } catch (error) {
      setTestResult({ ok: false, message: toMessage(error), latencyMs: 0 })
    } finally {
      setTesting(false)
    }
  }, [collect, form])

  const handleSave = useCallback(async () => {
    let values: ConnectionValues
    try {
      values = await form.validateFields()
    } catch {
      return
    }
    setSaving(true)
    try {
      const saved = await saveConnection(collect(values))
      message.success(`Saved “${saved.name}”`)
      closeEditor()
    } catch (error) {
      message.error(toMessage(error))
    } finally {
      setSaving(false)
    }
  }, [closeEditor, collect, form, message, saveConnection])

  const Fields = page?.Fields
  const title = draft?.id
    ? `Edit ${draft.name}`
    : driverInfo
      ? `New ${driverInfo.displayName} connection`
      : 'New connection'

  return (
    <Modal
      open={open}
      title={title}
      width={620}
      onCancel={closeEditor}
      destroyOnHidden
      footer={
        <Space>
          <Button onClick={closeEditor}>Cancel</Button>
          <Button icon={<ThunderboltOutlined />} loading={testing} onClick={() => void handleTest()}>
            Test connection
          </Button>
          <Button type="primary" loading={saving} onClick={() => void handleSave()}>
            Save
          </Button>
        </Space>
      }
    >
      <Form<ConnectionValues>
        form={form}
        layout="vertical"
        initialValues={blankValues(drivers.find((d) => d.implemented))}
        onValuesChange={() => setTestResult(null)}
      >
        <Form.Item
          name="driver"
          label="Driver"
          rules={[{ required: true, message: 'Pick a driver' }]}
        >
          <Select
            onChange={handleDriverChange}
            options={drivers.map((d) => ({
              value: d.type,
              label: (
                <Space size={6}>
                  {driverIcon(d.type) ? (
                    <img
                      src={driverIcon(d.type)}
                      alt=""
                      draggable={false}
                      className="dm-driver-icon"
                    />
                  ) : null}
                  <span>{d.implemented ? d.displayName : `${d.displayName} — planned`}</span>
                </Space>
              ),
              disabled: !d.implemented,
            }))}
          />
        </Form.Item>

        {driverInfo && !driverInfo.implemented ? (
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            title="Planned driver"
            description={driverInfo.notes}
          />
        ) : null}

        <DisplayNameField />

        {/* Everything below is the driver's own page: MySQL/PostgreSQL ask for
            an address and credentials, SQLite asks for a file. */}
        {Fields && driverInfo ? <Fields form={form} driver={driverInfo} draft={draft} /> : null}

        <Divider style={{ margin: '4px 0 16px' }}>Options</Divider>
        <div className="dm-form-row">
          <Form.Item name="readOnly" label="Read only" valuePropName="checked">
            <Switch />
          </Form.Item>
          <LabelColourField />
        </div>

        {testResult ? (
          <Alert
            type={testResult.ok ? 'success' : 'error'}
            showIcon
            icon={testResult.ok ? <CheckCircleOutlined /> : undefined}
            title={testResult.ok ? 'Connection succeeded' : 'Connection failed'}
            description={
              <div className="mono" style={{ fontSize: 12, whiteSpace: 'pre-wrap' }}>
                {testResult.message}
                {testResult.ok ? (
                  <>
                    {'\n'}
                    {[
                      testResult.serverVersion,
                      testResult.connectedDatabase ? `db=${testResult.connectedDatabase}` : null,
                      `${testResult.latencyMs} ms`,
                      testResult.databaseCount !== undefined
                        ? `${testResult.databaseCount} databases`
                        : null,
                    ]
                      .filter(Boolean)
                      .join(' · ')}
                  </>
                ) : null}
              </div>
            }
          />
        ) : null}
      </Form>
    </Modal>
  )
}

function driverFor(drivers: DriverInfo[], type: DriverType | undefined): DriverInfo | undefined {
  return type ? drivers.find((d) => d.type === type) : undefined
}

/** A blank profile for a driver: its port, its default database, no secrets. */
function blankValues(driver: DriverInfo | undefined): ConnectionValues {
  return {
    driver: driver?.type ?? 'mysql',
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
