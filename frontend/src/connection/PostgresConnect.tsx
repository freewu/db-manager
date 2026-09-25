import { t } from '../lib/i18n'
import {
  CredentialsFields,
  DatabaseFields,
  ExtraParamsField,
  HostPortFields,
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
function Basic({ driver, draft }: DriverFormProps) {
  return (
    <>
      <HostPortFields />
      <CredentialsFields storedPassword={draft?.hasPassword} />
      <DatabaseFields
        label="Database"
        placeholder={driver.defaultDatabase || 'postgres'}
        extra={t('connectionPostgres.the-one-new-tabs-open-on', {
          database: driver.defaultDatabase || 'postgres',
        })}
      />
    </>
  )
}

/** TLS: off by default, which is why it is its own tab rather than more fields. */
function Security() {
  return <TlsFields />
}

/** Whatever the server needs beyond the DSN — search_path, application_name, … */
function Advanced() {
  return (
    <ExtraParamsField hint="Passed to the driver verbatim, for example search_path or application_name." />
  )
}

export const PostgresConnect: DriverForm = {
  Basic,
  Security,
  Advanced,
  summary: 'Host, port, user, password, TLS — one pool per database',
}
