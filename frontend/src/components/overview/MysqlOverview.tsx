import type { ServerOverview } from '../../api/types'
import { metricSection, tableSection, type OverviewView } from './shared'

/**
 * MySQL's runtime page.
 *
 * MySQL is thread-per-connection with one shared InnoDB buffer pool, so after
 * the one-line server block the page answers "what is running right now" — the
 * process list is the panel you act on, so it is the second block. The counters
 * come last, ordered the way a MySQL DBA reads them: who is connected, how much
 * work the server has done, and whether the buffer pool is earning its memory.
 *
 * `SHOW FULL PROCESSLIST` needs the PROCESS privilege. Without it the backend
 * returns no list at all and this view simply has no process block, rather than
 * a section whose table looks like a hung server.
 *
 * TiDB and Doris are served by this view too: they report through the MySQL
 * payload, and its blocks are the ones they fill.
 */
export function mysqlView(overview: ServerOverview): OverviewView {
  const mysql = overview.mysql
  if (!mysql) return { sections: [] }

  const [server, ...rest] = mysql.groups

  return {
    sections: [
      ...(server ? [metricSection(server)] : []),
      ...(mysql.processes ? [tableSection(mysql.processes)] : []),
      ...rest.map(metricSection),
    ],
  }
}
