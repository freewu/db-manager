import { useCallback, type ComponentType } from 'react'
import { t } from '../lib/i18n'

import {
  App as AntApp,
  Button,
  Form,
  Input,
  InputNumber,
  Select,
  Space,
  Switch,
  Typography,
} from 'antd'
import type { FormInstance } from 'antd'
import { FolderOpenOutlined, MinusCircleOutlined, PlusOutlined } from '@ant-design/icons'

import { api, toMessage } from '../api/client'
import type { ConnectionConfig, DriverInfo, SSLMode } from '../api/types'

/**
 * Every value the connection form holds, whichever driver page is on screen.
 *
 * The shape is deliberately the union of all drivers: the shell owns the form
 * instance and hands it down, so a page can add fields without the shell — or
 * its save path — knowing anything about them.
 *
 * The driver is deliberately *not* in here: it is picked before the dialog
 * opens, and `validateFields()` only returns fields an input registered, so a
 * value nobody renders comes back empty. The dialog reads it off the draft.
 */
export interface ConnectionValues {
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

/**
 * A profile the editor is working on: either a saved connection (then `id` and
 * `name` are set) or a fresh draft seeded by the type picker, which knows only
 * the driver it was picked for.
 *
 * `groupId` is the exception to that: it rides along when the editor was opened
 * from a group's own "New connection…", so the finished profile is moved into
 * the folder the user asked for instead of appearing at the top level. It is not
 * part of the stored profile — where a connection sits is the arrangement's
 * business, not the profile's.
 */
export type ConnectionDraft = Partial<ConnectionConfig> &
  Pick<ConnectionConfig, 'driver'> & { groupId?: string }

/** What the shell passes to the page of the driver being configured. */
export interface DriverFormProps {
  /** The form that owns the values; fields must register through it. */
  form: FormInstance<ConnectionValues>
  /** Capabilities and defaults, straight from the backend driver registry. */
  driver: DriverInfo
  /** The profile being edited, when this is an edit rather than a create. */
  draft?: ConnectionDraft
}

/**
 * One driver's page, told apart by the tabs the dialog shows.
 *
 * Only `Basic` is mandatory — it is what a connection needs to exist at all.
 * `Security` and `Advanced` are filled by the drivers that have something to
 * put there, and a tab the driver leaves empty never appears: SQLite, a local
 * file, has neither TLS nor driver parameters.
 *
 * Each slot is rendered inside the shell's `<Form>`, so it draws only the
 * driver-specific part of the form; display name, read-only, label colour and
 * the test/save buttons belong to the shell.
 */
export interface DriverForm {
  /** Tab 1: how to reach the engine (address, credentials, or a file path). */
  Basic: ComponentType<DriverFormProps>
  /** The TLS material tab; network drivers only. */
  Security?: ComponentType<DriverFormProps>
  /** The free-form parameters tab, handed to the driver verbatim. */
  Advanced?: ComponentType<DriverFormProps>
  /** One line for the "new connection" picker, in the driver's own terms. */
  summary: string
}

/**
 * The pages a connection form can have, in the order the dialog shows them.
 *
 * The first three are the driver's to fill; `Options` belongs to the shell and
 * is on every form, because read-only and the profile colour mean the same
 * thing whatever you are connecting to.
 *
 * The label is a message key, not a word: this table is built once at import
 * time and read on every render, so the dialog resolves it then.
 */
export const CONNECTION_TABS = [
  { key: 'basic', label: 'connectionDialog.tab-basic' },
  { key: 'security', label: 'connectionDialog.tab-security' },
  { key: 'advanced', label: 'connectionDialog.tab-advanced' },
  { key: 'options', label: 'connectionDialog.tab-options' },
] as const

export type ConnectionTab = (typeof CONNECTION_TABS)[number]['key']

/**
 * The `DriverForm` member that fills a tab, for the shell to look up — or
 * `null` for the tabs the dialog draws itself.
 */
export function connectionTabSlot(
  tab: ConnectionTab,
): 'Basic' | 'Security' | 'Advanced' | null {
  switch (tab) {
    case 'security':
      return 'Security'
    case 'advanced':
      return 'Advanced'
    case 'options':
      return null
    default:
      return 'Basic'
  }
}

export const COLOR_SWATCHES = [
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

/** The name the profile is listed under; the only field every driver needs. */
export function DisplayNameField() {
  return (
    <Form.Item
      name="name"
      label="Display name"
      rules={[{ required: true, message: 'A display name is required' }]}
    >
      <Input placeholder="Local MySQL" />
    </Form.Item>
  )
}

/** Server address: host plus the driver's port. */
export function HostPortFields({ hostPlaceholder = '127.0.0.1' }: { hostPlaceholder?: string }) {
  return (
    <div className="dm-form-row">
      <Form.Item
        name="host"
        label="Host"
        className="dm-form-row-main"
        rules={[{ required: true, message: 'Host is required' }]}
      >
        <Input placeholder={hostPlaceholder} />
      </Form.Item>
      <Form.Item name="port" label="Port" className="dm-form-row-port">
        <InputNumber min={1} max={65535} style={{ width: '100%' }} />
      </Form.Item>
    </div>
  )
}

/**
 * Username and password.
 *
 * The password is never pre-filled from the profile: an empty box means "keep
 * whatever is stored", so editing a connection cannot leak or clobber a secret.
 */
export function CredentialsFields({ storedPassword = false }: { storedPassword?: boolean }) {
  return (
    <div className="dm-form-row">
      <Form.Item name="username" label="Username" className="dm-form-row-half">
        <Input autoComplete="off" placeholder="root" />
      </Form.Item>
      <Form.Item name="password" label="Password" className="dm-form-row-half">
        <Input.Password
          autoComplete="new-password"
          placeholder={storedPassword ? '•••••• (stored)' : 'password'}
        />
      </Form.Item>
    </div>
  )
}

/** Catalogue a tab opens on by default. */
export function DatabaseField({
  label = 'Database',
  extra,
  placeholder,
  required,
  className,
}: {
  label?: string
  extra?: string
  placeholder?: string
  required?: boolean
  className?: string
}) {
  return (
    <Form.Item
      name="database"
      label={label}
      className={className}
      extra={extra}
      rules={required ? [{ required: true, message: `${label} is required` }] : undefined}
    >
      <Input placeholder={placeholder || 'optional'} />
    </Form.Item>
  )
}

/**
 * The default catalogue with the "remember password" switch beside it.
 *
 * The two share a row because neither needs the full width on its own: the
 * catalogue is one name, and the switch is a boolean with a note. Drivers that
 * have no password (SQLite) use `DatabaseField` on its own instead.
 */
export function DatabaseFields(props: {
  label?: string
  extra?: string
  placeholder?: string
  required?: boolean
}) {
  return (
    <div className="dm-form-row">
      <DatabaseField {...props} className="dm-form-row-main" />
      <RememberPasswordField />
    </div>
  )
}

/** TLS mode plus the certificate files it may need (the "Security" tab). */
export function TlsFields() {
  const sslMode = Form.useWatch('sslMode') as SSLMode | undefined
  return (
    <>
      <Form.Item name="sslMode" label="TLS mode">
        <Select options={SSL_MODES} />
      </Form.Item>
      {sslMode && sslMode !== 'disable' ? (
        <>
          <Form.Item name="sslCAFile" label="CA certificate">
            <Input placeholder="C:\\certs\\ca.pem" />
          </Form.Item>
          <div className="dm-form-row">
            <Form.Item name="sslCertFile" label="Client certificate" className="dm-form-row-half">
              <Input placeholder="optional" />
            </Form.Item>
            <Form.Item name="sslKeyFile" label="Client key" className="dm-form-row-half">
              <Input placeholder="optional" />
            </Form.Item>
          </div>
        </>
      ) : null}
    </>
  )
}

/**
 * Free-form key/value pairs handed to the driver as connection parameters
 * (`charset`, `search_path`, …).
 */
export function ExtraParamsField({ hint }: { hint?: string }) {
  return (
    <>
      <Form.Item label="Extra parameters" extra={hint} style={{ marginBottom: 8 }}>
        <Form.List name="params">
          {(fields, { add, remove }) => (
            <div className="dm-param-list">
              {fields.map(({ key, name }) => (
                <div key={key} className="dm-param-row">
                  <Form.Item name={[name, 'key']} noStyle>
                    <Input placeholder="key" className="dm-param-key" />
                  </Form.Item>
                  <Form.Item name={[name, 'value']} noStyle>
                    <Input placeholder="value" className="dm-param-value" />
                  </Form.Item>
                  <Button type="text" icon={<MinusCircleOutlined />} onClick={() => remove(name)} />
                </div>
              ))}
              <Button type="dashed" icon={<PlusOutlined />} onClick={() => add({ key: '', value: '' })}>
                Add parameter
              </Button>
            </div>
          )}
        </Form.List>
      </Form.Item>
    </>
  )
}

/**
 * Whether a typed password is written to the profile file.
 *
 * Only meaningful for drivers that have a password at all, which is why it
 * lives on the driver page (in the database row) rather than in the shell's
 * shared options.
 */
function RememberPasswordField() {
  return (
    <Form.Item
      name="savePassword"
      label="Remember password"
      className="dm-form-row-switch"
      valuePropName="checked"
      extra="Encrypted in your user profile directory."
    >
      <Switch />
    </Form.Item>
  )
}

/** The profile's colour in the explorer tree. */
export function LabelColourField() {
  return (
    <Form.Item name="color" label="Label colour">
      <ColorSwatches />
    </Form.Item>
  )
}

/** Colour picker rendered as a row of swatches (no popover, no extra deps). */
function ColorSwatches({ value, onChange }: { value?: string; onChange?: (value: string) => void }) {
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

/** File-backed drivers: the path plus a native picker behind a button. */
export function FilePathField({ onBrowse }: { onBrowse: () => void }) {
  return (
    <Form.Item
      name="filePath"
      label="Database file"
      rules={[{ required: true, message: 'Pick a database file' }]}
      extra="A missing file is created when the connection is opened."
    >
      <Input
        placeholder="C:\\data\\app.db"
        addonAfter={
          <Button type="text" size="small" icon={<FolderOpenOutlined />} onClick={onBrowse}>
            Browse
          </Button>
        }
      />
    </Form.Item>
  )
}

/** Opens the system file picker and writes the choice back into the form. */
export function useFilePicker(form: FormInstance<ConnectionValues>) {
  const { message } = AntApp.useApp()
  return useCallback(async () => {
    try {
      const picked = await api.pickFile(t('connectionShared.select-a-database-file'), [
        '*.db',
        '*.sqlite',
        '*.sqlite3',
        '*',
      ])
      if (picked) form.setFieldsValue({ filePath: picked })
    } catch (error) {
      message.error(toMessage(error))
    }
  }, [form, message])
}
