import type { ServerOverview } from '../../api/types'
import { isMySQLFamily } from '../../lib/sqlFlavor'
import { mongoView } from './MongoOverview'
import { mysqlView } from './MysqlOverview'
import { postgresView } from './PostgresOverview'
import { sqliteView } from './SqliteOverview'
import { unsupportedView } from './UnsupportedOverview'
import type { OverviewView } from './shared'

export type { OverviewSection, OverviewView } from './shared'

/**
 * Which page a session gets, and how its blocks are named.
 *
 * The payload decides, not the driver list: TiDB and Doris are MySQL-family
 * engines and report through the MySQL payload, so they read the MySQL page —
 * the same page, with the blocks their server actually fills in. A driver that
 * reports nothing gets the page that says so, instead of an empty screen.
 */
export function overviewView(overview: ServerOverview): OverviewView {
  if (isMySQLFamily(overview.driver) && overview.mysql) return mysqlView(overview)
  if (overview.driver === 'postgres' && overview.postgres) return postgresView(overview)
  if (overview.driver === 'sqlite' && overview.sqlite) return sqliteView(overview)
  if (overview.driver === 'mongodb' && overview.mongodb) return mongoView(overview)
  return unsupportedView(overview)
}
