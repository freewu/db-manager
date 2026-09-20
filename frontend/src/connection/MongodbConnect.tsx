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
 * MongoDB.
 *
 * A document store, so this page is a network page like MySQL's rather than a
 * file page: address, credentials, TLS. Two things are specific to it, and both
 * are said in the fields themselves rather than hidden behind defaults:
 *
 * - a host may be a comma-separated member list ("a:27017,b:27018") or a
 *   replica set / Atlas hostname with `srv=true` in Advanced;
 * - the database field is also the authentication source, which is what a user
 *   hitting an "Authentication failed" against a server whose users live in
 *   `admin` needs to know.
 */
function Basic({ draft }: DriverFormProps) {
  return (
    <>
      <HostPortFields hostPlaceholder="127.0.0.1, or a:27017,b:27018 for a replica set" />
      <CredentialsFields storedPassword={draft?.hasPassword} />
      <DatabaseField
        label="Database"
        placeholder="admin"
        extra="Used as the authentication source and as the default database for new tabs. Leave empty to authenticate against admin."
      />
      <RememberPasswordField />
    </>
  )
}

/** TLS: mongod has it off by default, so it stays a separate tab. */
function Security() {
  return <TlsFields />
}

/** Connection-string options mongod understands, passed through verbatim. */
function Advanced() {
  return (
    <ExtraParamsField hint="Written into the connection string as-is — for example srv=true, replicaSet=rs0 or readPreference=secondaryPreferred." />
  )
}

export const MongodbConnect: DriverForm = {
  Basic,
  Security,
  Advanced,
  summary: 'Host, port, user, password, TLS — collections instead of tables',
}
