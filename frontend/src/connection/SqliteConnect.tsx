import { Alert } from 'antd'

import {
  DatabaseField,
  FilePathField,
  useFilePicker,
  type DriverForm,
  type DriverFormProps,
} from './shared'

/**
 * SQLite.
 *
 * No host, no credentials: the profile is a path on this machine, which is why
 * this page shares nothing with the network drivers but the shared shell.
 */
function Fields({ form, driver }: DriverFormProps) {
  const browse = useFilePicker(form)
  return (
    <>
      <FilePathField onBrowse={() => void browse()} />
      <DatabaseField
        label="Attached database alias"
        placeholder={driver.defaultDatabase || 'main'}
        extra="Leave as “main” unless you attach extra files to the same connection."
      />
      <Alert
        type="info"
        showIcon
        title="Local file only"
        description="Nothing leaves this machine, and the file is opened as-is — use Read only below to browse a database you do not want to write to."
      />
    </>
  )
}

export const SqliteConnect: DriverForm = {
  Fields,
  summary: 'A local .db file — no server, no credentials, works offline',
}
