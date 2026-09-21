import type { ServerOverview } from '../../api/types'
import { metricSection, tableSection, type OverviewView } from './shared'

/**
 * PostgreSQL's runtime page.
 *
 * The order follows how a PostgreSQL admin actually works: who am I connected to
 * and for how long, then the shape of the cluster (which databases, how big),
 * then who is doing what in `pg_stat_activity`, and only then the accumulated
 * counters. The database and activity blocks are absent when the role may not
 * read them — a missing block here means "not permitted", never "nothing".
 */
export function postgresView(overview: ServerOverview): OverviewView {
  const postgres = overview.postgres
  if (!postgres) return { sections: [] }

  const [server, ...counters] = postgres.groups

  return {
    sections: [
      ...(server ? [metricSection(server)] : []),
      ...(postgres.databases ? [tableSection(postgres.databases)] : []),
      ...(postgres.activity ? [tableSection(postgres.activity)] : []),
      ...counters.map(metricSection),
    ],
  }
}
