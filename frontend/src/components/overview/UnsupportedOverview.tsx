import { Alert, Descriptions, Typography } from 'antd'

import type { ServerOverview } from '../../api/types'
import type { OverviewView } from './shared'
import { t } from '../../lib/i18n'

/**
 * The page for an engine that has no runtime reporting.
 *
 * This is not an error state: the session works, it just cannot answer "how are
 * you doing". Saying exactly that, and showing what is known about the session
 * itself, is more useful than an empty page or a fabricated set of zeroes — and
 * it is the honest place for a driver that has not been taught to report yet.
 *
 * The alert leads the page rather than becoming a block: it explains the whole
 * page, so it must not look like one of the things the page has to say.
 */
export function unsupportedView(overview: ServerOverview): OverviewView {
  return {
    notice: (
      <Alert
        type="info"
        showIcon
        title={t('overviewUnsupported.does-not-report-runtime-state-yet', { driver: overview.driver })}
        description={
          <Typography.Text style={{ fontSize: 12 }}>
            {t('overviewUnsupported.the-connection-is-open-and-usable-queries-the')}
          </Typography.Text>
        }
      />
    ),
    sections: [
      {
        label: t('overviewUnsupported.session'),
        node: (
          <section className="dm-metric-group">
            <header className="dm-metric-group-title">
              <span>{t('overviewUnsupported.session')}</span>
            </header>
            <Descriptions size="small" column={1} bordered>
              <Descriptions.Item label={t('overviewUnsupported.connection')}>{overview.name || '—'}</Descriptions.Item>
              <Descriptions.Item label={t('overviewUnsupported.engine')}>{overview.driver}</Descriptions.Item>
              <Descriptions.Item label={t('overviewUnsupported.server-version')}>
                {overview.serverVersion || '—'}
              </Descriptions.Item>
              <Descriptions.Item label={t('overviewUnsupported.database')}>{overview.database || '—'}</Descriptions.Item>
              <Descriptions.Item label={t('overviewUnsupported.read-only')}>
                {overview.readOnly ? t('overviewUnsupported.yes') : t('overviewUnsupported.no')}
              </Descriptions.Item>
              <Descriptions.Item label={t('overviewUnsupported.connected')}>
                {overview.connectedAt ? new Date(overview.connectedAt).toLocaleString() : '—'}
              </Descriptions.Item>
            </Descriptions>
          </section>
        ),
      },
    ],
  }
}
