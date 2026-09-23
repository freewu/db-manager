import { Tooltip } from 'antd'
import {
  DatabaseOutlined,
  ExperimentOutlined,
  HistoryOutlined,
  SettingOutlined,
} from '@ant-design/icons'
import type { ReactNode } from 'react'

import type { AppPage } from '../store/appStore'

/** What each item says when the pointer rests on it. */
const RAIL_HINTS: Record<AppPage, string> = {
  connections: 'The connections, and the windows they open',
  datagen: 'Fill a table with generated rows',
  changelog: 'Every change this application has run, newest first',
  settings: 'Theme, code generation, mock placeholders, the data folder',
}

/**
 * The rail along the far left: the four pages this application has.
 *
 * It is the answer to a window that could be folded away and then not be found
 * again. The explorer is a pane that can be hidden, and a program whose explorer
 * has been hidden has to keep a door to it somewhere that cannot itself be
 * hidden — so the four things the main area can show are named here, always, and
 * each one is one click away. *Connections* is the working area, where the
 * explorer and the windows the connections open live; the other three are pages
 * of their own.
 *
 * The rail only says *which* page was asked for. What a page does about being
 * asked — that one is filled with windows and another has to open one — is the
 * shell's business, because the shell is what holds the pages.
 */
export function ActivityBar({
  page,
  onSelect,
  hints,
  disabledReason,
}: {
  page: AppPage
  onSelect: (page: AppPage) => void
  /** Hints that depend on the state of the shell, which the rail cannot see. */
  hints?: Partial<Record<AppPage, string>>
  /** Why a page cannot be opened, or nothing when it can. */
  disabledReason?: Partial<Record<AppPage, string>>
}) {
  // A page that cannot be opened has to say so on the item itself: a door that
  // is there but locked reads better than one that has been taken away.
  const hintOf = (target: AppPage) =>
    disabledReason?.[target] ?? hints?.[target] ?? RAIL_HINTS[target]

  return (
    <nav className="dm-rail" aria-label="Pages">
      <RailItem
        page="connections"
        current={page}
        icon={<DatabaseOutlined />}
        label="Connections"
        hint={hintOf('connections')}
        disabledReason={disabledReason?.connections}
        onSelect={onSelect}
      />
      <RailItem
        page="datagen"
        current={page}
        icon={<ExperimentOutlined />}
        label="Data generation"
        hint={hintOf('datagen')}
        disabledReason={disabledReason?.datagen}
        onSelect={onSelect}
      />
      <RailItem
        page="changelog"
        current={page}
        icon={<HistoryOutlined />}
        label="Change log"
        hint={hintOf('changelog')}
        disabledReason={disabledReason?.changelog}
        onSelect={onSelect}
      />
      <RailItem
        page="settings"
        current={page}
        icon={<SettingOutlined />}
        label="Settings"
        hint={hintOf('settings')}
        disabledReason={disabledReason?.settings}
        onSelect={onSelect}
      />
    </nav>
  )
}

function RailItem({
  page,
  current,
  icon,
  label,
  hint,
  disabledReason,
  onSelect,
}: {
  page: AppPage
  current: AppPage
  icon: ReactNode
  label: string
  hint: string
  disabledReason?: string
  onSelect: (page: AppPage) => void
}) {
  const active = page === current
  return (
    <Tooltip title={hint} placement="right">
      {/* The wrapper is not decoration: a disabled button raises no pointer
          events, so a tooltip attached to it directly would never be asked to
          explain why the item is locked. */}
      <span className="dm-rail-slot">
        <button
          type="button"
          className={active ? 'dm-rail-item is-active' : 'dm-rail-item'}
          aria-label={label}
          aria-current={active ? 'page' : undefined}
          disabled={Boolean(disabledReason)}
          onClick={() => onSelect(page)}
        >
          {icon}
        </button>
      </span>
    </Tooltip>
  )
}
