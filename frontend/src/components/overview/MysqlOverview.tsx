import type { ServerOverview } from '../../api/types'
import { DataTable, MetricGroup } from './shared'

/**
 * MySQL's runtime page.
 *
 * MySQL is thread-per-connection with one shared InnoDB buffer pool, so after
 * the one-line server block this page answers "what is running right now" —
 * the process list is the panel you act on, so it sits directly under the
 * build line. The counters come last, ordered the way a MySQL DBA reads them:
 * who is connected, how much work the server has done, and whether the buffer
 * pool is earning its memory.
 *
 * `SHOW FULL PROCESSLIST` needs the PROCESS privilege. Without it the backend
 * returns no list at all and this view simply does not draw the panel, rather
 * than showing an empty table that looks like a hung server.
 */
export function MysqlOverview({ overview }: { overview: ServerOverview }) {
  const mysql = overview.mysql
  if (!mysql) return null

  const [server, ...rest] = mysql.groups

  return (
    <>
      {server ? <MetricGroup group={server} /> : null}
      {mysql.processes ? <DataTable table={mysql.processes} /> : null}
      {rest.map((group) => (
        <MetricGroup key={group.title} group={group} />
      ))}
    </>
  )
}
