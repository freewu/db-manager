import type { ServerOverview } from '../../api/types'
import { DataTable, MetricGroup } from './shared'

/**
 * PostgreSQL's runtime page.
 *
 * The order follows how a PostgreSQL admin actually works: who am I connected to
 * and for how long, then the shape of the cluster (which databases, how big),
 * then who is doing what in `pg_stat_activity`, and only then the accumulated
 * counters. The database and activity tables are absent when the role may not
 * read them — a missing table here means "not permitted", never "nothing".
 */
export function PostgresOverview({ overview }: { overview: ServerOverview }) {
  const postgres = overview.postgres
  if (!postgres) return null

  const [server, ...counters] = postgres.groups

  return (
    <>
      {server ? <MetricGroup group={server} /> : null}
      {postgres.databases ? <DataTable table={postgres.databases} /> : null}
      {postgres.activity ? <DataTable table={postgres.activity} /> : null}
      {counters.map((group) => (
        <MetricGroup key={group.title} group={group} />
      ))}
    </>
  )
}
