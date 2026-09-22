import type { ReactNode } from 'react'
import { Avatar, Tooltip, Typography } from 'antd'
import { GithubOutlined } from '@ant-design/icons'

import { developerAvatar } from '../lib/assets'
import { useAppStore } from '../store/appStore'
import {
  BUILD_GROUP,
  DEVELOPER,
  PLATFORM_GROUP,
  JUST_VERSION,
  LICENSE,
  PROJECT_URL,
  REPO_ISSUES_URL,
  REPO_RELEASES_URL,
  openExternal,
  techStack,
  type Shield,
} from '../lib/about'

/**
 * The project's own page: the app name and version as the heading, then what it
 * is built with, where it lives, and who wrote it.
 *
 * Shown on the welcome pane — the right-hand side when nothing is open, which
 * is where a reader who has not connected a database yet is looking. The badges
 * are drawn locally (a grey label + a brand-coloured value) instead of being
 * pulled from shields.io, because the desktop build has to render offline.
 */
export function AboutProject() {
  const appInfo = useAppStore((s) => s.appInfo)

  // The version is in the heading, so it is deliberately not repeated here.
  // `running` is this binary; what the project ships is the Platforms group.
  const top: Shield[] = [
    { label: 'license', value: LICENSE, color: '#97CA00' },
    // The build tool has no brand colour we ship artwork for, so it borrows the
    // neutral grey the other "tooling" badges use.
    { label: 'build', value: `just ${JUST_VERSION}`, color: '#4B5563' },
    { label: 'running', value: appInfo?.platform ?? '—', color: '#007EC6' },
  ]

  return (
    <section className="dm-about-project">
      <Typography.Title level={4} className="dm-about-title">
        {appInfo?.name ?? 'DB Manager'}
        {appInfo?.version ? (
          <>
            {' '}
            <span className="dm-about-version">v{appInfo.version}</span>
          </>
        ) : null}
      </Typography.Title>

      <div className="dm-shield-row">
        {top.map((shield) => (
          <ShieldBadge key={shield.label} {...shield} />
        ))}
      </div>

      <Typography.Paragraph type="secondary" className="dm-about-lead">
        An offline-first MySQL / PostgreSQL / SQLite client: browse, edit, design and query
        without a server-side agent. Connections and favourites stay on this machine, in{' '}
        <span className="mono">{appInfo?.configPath ?? 'the app config folder'}</span>.
      </Typography.Paragraph>

      {[BUILD_GROUP, ...techStack(appInfo?.goVersion), PLATFORM_GROUP].map((group) => (
        <div className="dm-shield-group" key={group.title}>
          <span className="dm-shield-group-title">{group.title}</span>
          <div className="dm-shield-row">
            {group.shields.map((shield) => (
              <ShieldBadge key={shield.label} {...shield} />
            ))}
          </div>
        </div>
      ))}

      <dl className="dm-about-rows">
        <dt>Repository</dt>
        <dd>
          <ExternalLink url={PROJECT_URL} icon={<GithubOutlined />} label={PROJECT_URL} />
        </dd>

        <dt>Releases</dt>
        <dd>
          <ExternalLink url={REPO_RELEASES_URL} label={`${PROJECT_URL}/releases`} />
        </dd>

        <dt>Issues</dt>
        <dd>
          <ExternalLink url={REPO_ISSUES_URL} label={`${PROJECT_URL}/issues`} />
        </dd>

        <dt>Developer</dt>
        <dd>
          {/* The avatar and the nickname are one link to the profile it stands
              for, which is also the anchor's title — the email lives in
              `wails.json` and the repository, not in the UI. */}
          <ExternalLink url={DEVELOPER.url} title={DEVELOPER.name} ariaLabel={`${DEVELOPER.name} on GitHub`}>
            <Avatar size={22} className="dm-about-avatar" src={developerAvatar} alt={DEVELOPER.name}>
              {DEVELOPER.name.slice(0, 1).toUpperCase()}
            </Avatar>
            <span>{DEVELOPER.name}</span>
          </ExternalLink>
        </dd>
      </dl>
    </section>
  )
}

/** One shields.io-style badge, optionally a link (the Justfile is one). */
function ShieldBadge({ label, value, color, dark, href }: Shield) {
  const pill = (
    <span className="dm-shield">
      <span className="dm-shield-label">{label}</span>
      <span className={`dm-shield-value${dark ? ' is-dark' : ''}`} style={{ background: color }}>
        {value || 'n/a'}
      </span>
    </span>
  )

  return (
    <Tooltip title={`${label} ${value}`}>
      {href ? (
        <a
          className="dm-shield-link"
          href={href}
          onClick={(event) => {
            event.preventDefault()
            openExternal(href)
          }}
        >
          {pill}
        </a>
      ) : (
        pill
      )}
    </Tooltip>
  )
}

/** A link that leaves the webview for the real browser. */
function ExternalLink({
  url,
  label,
  icon,
  children,
  ariaLabel,
  title,
}: {
  url: string
  /** Plain text to show; use `children` instead for richer content. */
  label?: string
  icon?: ReactNode
  children?: ReactNode
  /** Required when the link shows no text of its own (a bare avatar). */
  ariaLabel?: string
  /** Native tooltip; also what a screen reader reads out with the URL. */
  title?: string
}) {
  return (
    <a
      href={url}
      aria-label={ariaLabel}
      title={title}
      onClick={(event) => {
        event.preventDefault()
        openExternal(url)
      }}
    >
      {icon ? <span className="dm-link-icon">{icon}</span> : null}
      {children ?? label}
    </a>
  )
}
