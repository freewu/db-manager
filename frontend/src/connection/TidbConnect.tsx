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
 * TiDB.
 *
 * A MySQL-compatible distributed cluster: the wire protocol, the catalog and
 * the SQL dialect are MySQL's, so this page is MySQL's with the two things a
 * TiDB user looks for said out loud — the default port (4000, not 3306) and the
 * fact that "database" here means the same thing it means in MySQL, with
 * `information_schema`, `mysql` and `metrics_schema` filtered out of the tree.
 */
function Basic({ draft }: DriverFormProps) {
  return (
    <>
      <HostPortFields hostPlaceholder="127.0.0.1, or the PD/TiDB service address" />
      <CredentialsFields storedPassword={draft?.hasPassword} />
      <DatabaseFields
        label="Database"
        placeholder="optional, for example test"
        extra="Optional default database for new tabs. Any TiDB server is a single logical database cluster, so the explorer lists every database at once."
      />
    </>
  )
}

/** TLS: the same handshake MySQL speaks, so the same fields. */
function Security() {
  return <TlsFields />
}

/** Whatever the session needs beyond the DSN. */
function Advanced() {
  return (
    <ExtraParamsField hint="Passed to the driver verbatim. charset, tls and the statement timeouts are set from the fields above and cannot be overridden here." />
  )
}

export const TidbConnect: DriverForm = {
  Basic,
  Security,
  Advanced,
  summary: 'Host, port, user, password, TLS — MySQL-compatible, clustered storage',
}
