import { Typography } from 'antd'

import type { ServerOverview } from '../../api/types'
import { metricSection, tableSection, type OverviewView } from './shared'

/**
 * MongoDB's runtime page.
 *
 * mongod reports through `serverStatus`, which is a single flat document of
 * counters rather than a set of SHOW statements, so the backend has already
 * grouped it and this view is a straight reading: connections first (a mongod
 * that runs out of them stops accepting work), then the operation counters,
 * then memory and the WiredTiger cache, then the replica set if there is one.
 *
 * The per-database block is `dbStats` for every database the user can see. It
 * comes last on purpose: it is the only part that grows with the server's
 * content rather than its activity.
 */
export function mongoView(overview: ServerOverview): OverviewView {
  const mongo = overview.mongodb
  if (!mongo) return { sections: [] }

  return {
    sections: [
      ...mongo.groups.map(metricSection),
      ...(mongo.databases ? [tableSection(mongo.databases)] : []),
    ],
    footnote: (
      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
        Counters come from <span className="mono">serverStatus</span> and reset when mongod
        restarts; the table above reads <span className="mono">dbStats</span> per database and
        costs one round trip per database.
      </Typography.Text>
    ),
  }
}
