import {
  CredentialsFields,
  DatabaseFields,
  ExtraParamsField,
  HostPortFields,
  type DriverForm,
  type DriverFormProps,
} from './shared'

/**
 * Apache Doris.
 *
 * A MySQL-compatible analytical database: the front end answers the MySQL
 * protocol on 9030, so browsing and querying work like MySQL's. Two differences
 * shape this page:
 *
 * - there is no TLS tab, because a Doris front end is normally reached inside a
 *   cluster network and its MySQL protocol endpoint does not do the handshake
 *   this build would have to configure;
 * - a table needs a data model (duplicate/aggregate/unique key) and a
 *   distribution clause, so the table designer stays off and the DDL editor is
 *   where a table is changed. The driver's notes say so on the structure page.
 */
function Basic({ draft }: DriverFormProps) {
  return (
    <>
      <HostPortFields hostPlaceholder="127.0.0.1, or the Doris front end (FE) address" />
      <CredentialsFields storedPassword={draft?.hasPassword} />
      <DatabaseFields
        label="Database"
        placeholder="optional"
        extra="Optional default database for new tabs; the explorer always lists every database the account can see."
      />
    </>
  )
}

/** Session options the front end accepts beyond the ones set from the fields above. */
function Advanced() {
  return (
    <ExtraParamsField hint="Passed to the driver verbatim, for example a session variable the FE accepts. Interpolation is always on for Doris and is not overridable here." />
  )
}

export const DorisConnect: DriverForm = {
  Basic,
  Advanced,
  summary: 'Host, port, user, password — MySQL-compatible analytical database',
}
