import { Alert, Descriptions, Typography } from 'antd'

import type { ServerOverview } from '../../api/types'

/**
 * The page for an engine that has no runtime reporting.
 *
 * This is not an error state: the session works, it just cannot answer "how are
 * you doing". Saying exactly that, and showing what is known about the session
 * itself, is more useful than an empty page or a fabricated set of zeroes — and
 * it is the honest place for a driver that has not been taught to report yet.
 */
export function UnsupportedOverview({ overview }: { overview: ServerOverview }) {
  return (
    <>
      <Alert
        type="info"
        showIcon
        title={`${overview.driver} does not report runtime state yet`}
        description={
          <Typography.Text style={{ fontSize: 12 }}>
            The connection is open and usable — queries, the object tree and the designer all work.
            This page is the only thing this engine cannot fill in.
          </Typography.Text>
        }
      />
      <section className="dm-metric-group">
        <header className="dm-metric-group-title">
          <span>Session</span>
        </header>
        <Descriptions size="small" column={1} bordered>
          <Descriptions.Item label="Connection">{overview.name || '—'}</Descriptions.Item>
          <Descriptions.Item label="Engine">{overview.driver}</Descriptions.Item>
          <Descriptions.Item label="Server version">
            {overview.serverVersion || '—'}
          </Descriptions.Item>
          <Descriptions.Item label="Database">{overview.database || '—'}</Descriptions.Item>
          <Descriptions.Item label="Read-only">{overview.readOnly ? 'yes' : 'no'}</Descriptions.Item>
          <Descriptions.Item label="Connected">
            {overview.connectedAt ? new Date(overview.connectedAt).toLocaleString() : '—'}
          </Descriptions.Item>
        </Descriptions>
      </section>
    </>
  )
}
