import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  App as AntApp,
  Button,
  Divider,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Switch,
  Typography,
} from 'antd'
import {
  CheckCircleOutlined,
  FolderOpenOutlined,
  MinusCircleOutlined,
  PlusOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type {
  ConnectionConfig,
  DriverInfo,
  DriverType,
  SSLConfig,
  SSLMode,
  TestResult,
} from '../api/types'
import { driverIcon } from '../lib/assets'
import { useAppStore } from '../store/appStore'

const COLOR_SWATCHES = [
  '#36ab60',
  '#1677ff',
  '#13c2c2',
  '#faad14',
  '#fa541c',
  '#eb2f96',
  '#722ed1',
  '#8c8c8c',
]

const SSL_MODES: { value: SSLMode; label: string }[] = [
  { value: 'disable', label: 'Disabled' },
  { value: 'require', label: 'Required (no verification)' },
  { value: 'verify-ca', label: 'Verify CA' },
  { value: 'verify-full', label: 'Verify full (CA + hostname)' },
]

interface FormValues {
  driver: DriverType
  name: string
  host?: string
  port?: number
  username?: string
  password?: string
  database?: string
  filePath?: string
  params: { key: string; value: string }[]
  sslMode: SSLMode
  sslCAFile?: string
  sslCertFile?: string
  sslKeyFile?: string
  readOnly: boolean
  savePassword: boolean
  color?: string
}

function blankValues(driver: DriverInfo | undefined): FormValues {
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

/** Create / edit dialog for a saved connection profile. */
export function ConnectionDialog() {
  const open = useAppStore((s) => s.editorOpen)
  const draft = useAppStore((s) => s.editorDraft)
  const drivers = useAppStore((s) => s.drivers)
  const closeEditor = useAppStore((s) => s.closeConnectionEditor)
  const saveConnection = useAppStore((s) => s.saveConnection)

  const { message } = AntApp.useApp()
  const [form] = Form.useForm<FormValues>()
  const [testing, setTesting] = useState(false)
  const [saving, setSaving] = useState(false)
  const [testResult, setTestResult] = useState<TestResult | null>(null)

  const selectedDriver = Form.useWatch('driver', form)
  const sslMode = Form.useWatch('sslMode', form)

  const driverInfo = useMemo(
    () => drivers.find((d) => d.type === selectedDriver),
    [drivers, selectedDriver],
  )

  const isFile = driverInfo?.requiresFile ?? false
  const showNetwork = !isFile
  const showSSL = showNetwork && !isFile
  const showCredentials = !isFile

  // Reset the form whenever the dialog opens with a different profile.
  useEffect(() => {
    if (!open) return
    setTestResult(null)
    if (draft) {
      form.setFieldsValue({
        driver: draft.driver,
        name: draft.name,
        host: draft.host ?? 'localhost',
        port: draft.port ?? driverFor(drivers, draft.driver)?.defaultPort,
        username: draft.username ?? '',
        // Never pre-fill the stored secret; leaving it blank keeps it as-is.
        password: '',
        database: draft.database ?? '',
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
    } else {
      form.setFieldsValue(blankValues(drivers.find((d) => d.implemented)))
    }
  }, [draft, drivers, form, open])

  const handleDriverChange = useCallback(
    (value: DriverType) => {
      const info = drivers.find((d) => d.type === value)
      const currentPort = form.getFieldValue('port')
      const currentName = form.getFieldValue('name')
      form.setFieldsValue({
        port: info?.defaultPort,
        database: form.getFieldValue('database') || info?.defaultDatabase || '',
        name:
          currentName ||
          (info ? `${info.displayName} @ ${form.getFieldValue('host') ?? 'localhost'}` : ''),
      })
      if (currentPort === undefined) {
        form.setFieldsValue({ port: info?.defaultPort })
      }
      setTestResult(null)
    },
    [drivers, form],
  )

  const collect = useCallback(
    (values: FormValues): ConnectionConfig => {
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
    let values: FormValues
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
    let values: FormValues
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

  const browse = useCallback(async () => {
    try {
      const picked = await api.pickFile('Select a SQLite database', ['*.db', '*.sqlite', '*.sqlite3', '*'])
      if (picked) form.setFieldsValue({ filePath: picked })
    } catch (error) {
      message.error(toMessage(error))
    }
  }, [form, message])

  return (
    <Modal
      open={open}
      title={draft ? `Edit ${draft.name}` : 'New connection'}
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
      <Form<FormValues>
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
            message="Planned driver"
            description={driverInfo.notes}
          />
        ) : null}

        <Form.Item
          name="name"
          label="Display name"
          rules={[{ required: true, message: 'A display name is required' }]}
        >
          <Input placeholder="Local MySQL" />
        </Form.Item>

        {isFile ? (
          <Form.Item
            name="filePath"
            label="Database file"
            rules={[{ required: true, message: 'Pick a database file' }]}
          >
            <Input
              placeholder="C:\\data\\app.db"
              addonAfter={
                <Button
                  type="text"
                  size="small"
                  icon={<FolderOpenOutlined />}
                  onClick={() => void browse()}
                >
                  Browse
                </Button>
              }
            />
          </Form.Item>
        ) : null}

        {showNetwork ? (
          <div style={{ display: 'flex', gap: 12, alignItems: 'flex-start' }}>
            <Form.Item
              name="host"
              label="Host"
              style={{ flex: '1 1 auto' }}
              rules={[{ required: true, message: 'Host is required' }]}
            >
              <Input placeholder="127.0.0.1" />
            </Form.Item>
            <Form.Item name="port" label="Port" style={{ flex: '0 0 110px' }}>
              <InputNumber min={1} max={65535} style={{ width: '100%' }} />
            </Form.Item>
          </div>
        ) : null}

        {showCredentials ? (
          <div style={{ display: 'flex', gap: 12, alignItems: 'flex-start' }}>
            <Form.Item name="username" label="Username" style={{ flex: '1 1 50%' }}>
              <Input autoComplete="off" placeholder="root" />
            </Form.Item>
            <Form.Item name="password" label="Password" style={{ flex: '1 1 50%' }}>
              <Input.Password
                autoComplete="new-password"
                placeholder={draft?.hasPassword ? '•••••• (stored)' : 'password'}
              />
            </Form.Item>
          </div>
        ) : null}

        <Form.Item
          name="database"
          label={isFile ? 'Attached database alias' : 'Database'}
          extra={
            isFile
              ? 'Leave as “main” unless you attach additional files.'
              : 'Optional. Used as the default schema/catalog for new tabs.'
          }
        >
          <Input placeholder={driverInfo?.defaultDatabase || 'optional'} />
        </Form.Item>

        {showSSL ? (
          <>
            <Divider style={{ margin: '4px 0 16px' }}>Security</Divider>
            <Form.Item name="sslMode" label="TLS mode">
              <Select options={SSL_MODES} />
            </Form.Item>
            {sslMode && sslMode !== 'disable' ? (
              <>
                <Form.Item name="sslCAFile" label="CA certificate">
                  <Input placeholder="C:\\certs\\ca.pem" />
                </Form.Item>
                <div style={{ display: 'flex', gap: 12, alignItems: 'flex-start' }}>
                  <Form.Item name="sslCertFile" label="Client certificate" style={{ flex: 1 }}>
                    <Input placeholder="optional" />
                  </Form.Item>
                  <Form.Item name="sslKeyFile" label="Client key" style={{ flex: 1 }}>
                    <Input placeholder="optional" />
                  </Form.Item>
                </div>
              </>
            ) : null}
          </>
        ) : null}

        <Divider style={{ margin: '4px 0 16px' }}>Options</Divider>

        <Form.Item label="Extra parameters" style={{ marginBottom: 8 }}>
          <Form.List name="params">
            {(fields, { add, remove }) => (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {fields.map(({ key, name }) => (
                  <div key={key} style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <Form.Item name={[name, 'key']} noStyle>
                      <Input placeholder="key" style={{ width: 200 }} />
                    </Form.Item>
                    <Form.Item name={[name, 'value']} noStyle>
                      <Input placeholder="value" style={{ width: 240 }} />
                    </Form.Item>
                    <Button
                      type="text"
                      icon={<MinusCircleOutlined />}
                      onClick={() => remove(name)}
                    />
                  </div>
                ))}
                <Button
                  type="dashed"
                  icon={<PlusOutlined />}
                  onClick={() => add({ key: '', value: '' })}
                >
                  Add parameter
                </Button>
              </div>
            )}
          </Form.List>
        </Form.Item>

        <div style={{ display: 'flex', gap: 28, flexWrap: 'wrap' }}>
          <Form.Item name="readOnly" label="Read only" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item
            name="savePassword"
            label="Remember password"
            valuePropName="checked"
            extra="Stored in plain text in your user profile directory."
          >
            <Switch />
          </Form.Item>
          <Form.Item name="color" label="Label colour">
            <ColorSwatches />
          </Form.Item>
        </div>

        {testResult ? (
          <Alert
            type={testResult.ok ? 'success' : 'error'}
            showIcon
            icon={testResult.ok ? <CheckCircleOutlined /> : undefined}
            message={testResult.ok ? 'Connection succeeded' : 'Connection failed'}
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

function driverFor(drivers: DriverInfo[], type: DriverType): DriverInfo | undefined {
  return drivers.find((d) => d.type === type)
}

/** Colour picker rendered as a row of swatches (no popover, no extra deps). */
function ColorSwatches({
  value,
  onChange,
}: {
  value?: string
  onChange?: (value: string) => void
}) {
  return (
    <Space size={6}>
      {COLOR_SWATCHES.map((color) => (
        <div
          key={color}
          role="button"
          tabIndex={0}
          title={color}
          onClick={() => onChange?.(color)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') onChange?.(color)
          }}
          style={{
            width: 22,
            height: 22,
            borderRadius: 5,
            background: color,
            cursor: 'pointer',
            outline: value === color ? `2px solid ${color}` : 'none',
            outlineOffset: 2,
          }}
        />
      ))}
      <Typography.Text type="secondary" style={{ fontSize: 11 }}>
        {value}
      </Typography.Text>
    </Space>
  )
}
