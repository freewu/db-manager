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
 * this page shares nothing with the network drivers but the shared shell. It is
 * also the one driver with no Security or Advanced page — the dialog's Options
 * tab is all it gets on top of Basic.
 */
function Basic({ form, driver }: DriverFormProps) {
  const browse = useFilePicker(form)
  return (
    <>
      <FilePathField onBrowse={() => void browse()} />
      <DatabaseField
        label="Attached database alias"
        placeholder={driver.defaultDatabase || 'main'}
        extra="Leave as “main” unless you attach extra files to the same connection."
      />
    </>
  )
}

export const SqliteConnect: DriverForm = {
  Basic,
  summary: 'A local .db file — no server, no credentials, works offline',
}
