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
 * PostgreSQL.
 *
 * Same skeleton as MySQL plus the thing that bites people: a pool is opened per
 * database, so the profile has to name the one new tabs should land in. An
 * empty database falls back to `postgres` (see the driver's DSN builder).
 */
function Fields({ driver, draft }: DriverFormProps) {
  return (
    <>
      <HostPortFields />
      <CredentialsFields storedPassword={draft?.hasPassword} />
      <DatabaseField
        label="Database"
        placeholder={driver.defaultDatabase || 'postgres'}
        extra={`PostgreSQL cannot query across databases, so this is the one new tabs open on. Empty connects to “${driver.defaultDatabase || 'postgres'}”.`}
      />
      <TlsFields />
      <ExtraParamsField hint="Passed to the driver verbatim, for example search_path or application_name." />
      <RememberPasswordField />
    </>
  )
}

export const PostgresConnect: DriverForm = {
  Fields,
  summary: 'Host, port, user, password, TLS — one pool per database',
}
