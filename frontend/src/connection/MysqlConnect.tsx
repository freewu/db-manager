import {
  CredentialsFields,
  DatabaseField,
  ExtraParamsField,
  HostPortFields,
  RememberPasswordField,
  TlsFields,
  type DriverForm,
  type DriverFormProps,
} from './shared'

/**
 * MySQL / MariaDB.
 *
 * A network server: address, credentials, an optional default schema and the
 * TLS material. MariaDB speaks the same protocol, so it shares the page.
 */
function Fields({ draft }: DriverFormProps) {
  return (
    <>
      <HostPortFields />
      <CredentialsFields storedPassword={draft?.hasPassword} />
      <DatabaseField
        label="Database"
        placeholder="optional"
        extra="Optional. Used as the default schema for new tabs; the explorer always lists every database."
      />
      <TlsFields />
      <ExtraParamsField hint="Passed to the driver verbatim, for example charset=utf8mb4." />
      <RememberPasswordField />
    </>
  )
}

export const MysqlConnect: DriverForm = {
  Fields,
  summary: 'Host, port, user, password, TLS — schemas are databases here',
}
