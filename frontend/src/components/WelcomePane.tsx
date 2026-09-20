import { Button, Card, Empty, Space, Tag, Typography } from 'antd'
import { PlusOutlined, RocketOutlined, TableOutlined } from '@ant-design/icons'

import { useAppStore } from '../store/appStore'
import { useConnect } from '../hooks/useConnect'
import { appLogo, driverIconOrLogo } from '../lib/assets'
import { AboutProject } from './AboutProject'
import { describeProfile } from './ConnectionSidebar'

/** Shown when no tab is open. */
export function WelcomePane() {
  const appInfo = useAppStore((s) => s.appInfo)
  const connections = useAppStore((s) => s.connections)
  const drivers = useAppStore((s) => s.drivers)
  const openConnectionEditor = useAppStore((s) => s.openConnectionEditor)
  const { connect } = useConnect()

  const recent = connections.slice(0, 6)
  const implemented = drivers.filter((driver) => driver.implemented)
  const planned = drivers.filter((driver) => !driver.implemented)

  return (
    <div className="dm-welcome">
      <div className="dm-welcome-inner">
        <Space direction="vertical" size={4} style={{ marginBottom: 24 }}>
          <Typography.Title level={3} style={{ margin: 0 }}>
            <img src={appLogo} alt="" className="dm-brand-logo is-large" draggable={false} />{' '}
            {appInfo?.name ?? 'DB Manager'}
          </Typography.Title>
          <Typography.Text type="secondary">
            {appInfo ? `v${appInfo.version} · ${appInfo.platform}` : ''}
          </Typography.Text>
        </Space>

        <Space wrap size={8} style={{ marginBottom: 24 }}>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => openConnectionEditor()}
          >
            New connection
          </Button>
          <Button icon={<RocketOutlined />} onClick={() => openConnectionEditor()}>
            Quick connect
          </Button>
        </Space>

        {recent.length > 0 ? (
          <div style={{ marginBottom: 28 }}>
            <Typography.Title level={5}>Saved connections</Typography.Title>
            <div className="dm-welcome-grid">
              {recent.map((profile) => {
                const driver = drivers.find((d) => d.type === profile.driver)
                return (
                  <Card
                    key={profile.id}
                    size="small"
                    hoverable
                    className="dm-welcome-card"
                    onClick={() => void connect(profile)}
                  >
                    <Space direction="vertical" size={2} style={{ width: '100%' }}>
                      <Space size={6}>
                        <img
                          src={driverIconOrLogo(profile.driver)}
                          alt=""
                          draggable={false}
                          className="dm-driver-icon is-large"
                        />
                        <Typography.Text strong>{profile.name}</Typography.Text>
                      </Space>
                      <Typography.Text type="secondary" className="mono" style={{ fontSize: 12 }}>
                        {describeProfile(profile)}
                      </Typography.Text>
                      <Space size={4}>
                        <Tag style={{ marginInlineEnd: 0 }}>{driver?.displayName ?? profile.driver}</Tag>
                        {profile.readOnly ? <Tag color="gold">read-only</Tag> : null}
                      </Space>
                    </Space>
                  </Card>
                )
              })}
            </div>
          </div>
        ) : (
          <Empty
            style={{ marginBottom: 28 }}
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              <span>
                No connections yet. Create one to get started.
                <br />
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  Everything is stored locally in{' '}
                  <span className="mono">{appInfo?.configPath ?? 'connections.json'}</span>.
                </Typography.Text>
              </span>
            }
          />
        )}

        <Typography.Title level={5}>
          <TableOutlined /> Supported engines
        </Typography.Title>
        <Space wrap size={6}>
          {implemented.map((driver) => (
            <Tag key={driver.type} color="green">
              {driver.displayName}
            </Tag>
          ))}
          {planned.map((driver) => (
            <Tag key={driver.type} color="default">
              {driver.displayName} · planned
            </Tag>
          ))}
        </Space>

        <div style={{ marginTop: 28 }}>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            Shortcuts: <span className="mono">Ctrl/Cmd+Enter</span> runs the whole editor,{' '}
            <span className="mono">Ctrl/Cmd+Shift+Enter</span> runs the selection, double click a
            cell to edit it.
          </Typography.Text>
        </div>

        <hr className="dm-welcome-divider" />
        <AboutProject />
      </div>
    </div>
  )
}
