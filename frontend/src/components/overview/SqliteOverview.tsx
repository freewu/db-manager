import { Typography } from 'antd'

import type { ServerOverview } from '../../api/types'
import { metricSection, tableSection, type OverviewView } from './shared'

/**
 * SQLite's runtime page.
 *
 * There is no server here, so the page is about the file: where it is, how big
 * it is, how its format is configured, and what is inside it. The path leads,
 * because it is the one piece of information a SQLite user always needs and the
 * one thing a server-oriented page never has to show.
 *
 * The size is not repeated here: the backend renders it once, in the File block,
 * and this view reads its numbers rather than reformatting them.
 */
export function sqliteView(overview: ServerOverview): OverviewView {
  const sqlite = overview.sqlite
  if (!sqlite) return { sections: [] }

  return {
    sections: [
      {
        label: 'Database file',
        node: (
          <section className="dm-metric-group">
            <header className="dm-metric-group-title">
              <span>Database file</span>
            </header>
            <div className="dm-sqlite-path mono">{sqlite.path}</div>
            {sqlite.fileSize < 0 ? (
              <div className="dm-metric-note">
                No such file on disk: either an in-memory database, or one opened through a path
                this process cannot see.
              </div>
            ) : null}
          </section>
        ),
      },
      ...sqlite.groups.map(metricSection),
      ...(sqlite.objects ? [tableSection(sqlite.objects)] : []),
      ...(sqlite.attached ? [tableSection(sqlite.attached)] : []),
    ],
    footnote: (
      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
        SQLite has no server process: every number above describes this file at the moment the
        snapshot was taken.
      </Typography.Text>
    ),
  }
}
