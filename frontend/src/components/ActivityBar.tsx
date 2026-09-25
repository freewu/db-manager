import { Tooltip } from 'antd'
import {
  DatabaseOutlined,
  ExperimentOutlined,
  HistoryOutlined,
  SettingOutlined,
  SwapOutlined,
} from '@ant-design/icons'
import type { ReactNode } from 'react'

import type { AppPage } from '../store/appStore'
import { t } from '../lib/i18n'
import type { MessageKey } from '../lib/i18n'

/**
 * The five pages this application has, in the order they are listed.
 *
 * The name is what the item is called in two places at once: the accessible name,
 * and the tip the pointer shows. A tip that has to explain the icon in a sentence
 * is a tip the icon did not need — the rail is five pictures, and the five things
 * this program can show are five short names. A page that is locked, or whose
 * click does something else right now, still says so through the overrides below.
 */
const RAIL_ITEMS: { page: AppPage; icon: ReactNode; name: MessageKey }[] = [
  { page: 'connections', icon: <DatabaseOutlined />, name: 'activityBar.connections' },
  // Between the things somebody looks at and the things they change: comparing
  // two databases reads them both, and what it produces is a script.
  { page: 'datagen', icon: <ExperimentOutlined />, name: 'activityBar.data-generation' },
  { page: 'compare', icon: <SwapOutlined />, name: 'activityBar.compare' },
  { page: 'changelog', icon: <HistoryOutlined />, name: 'activityBar.change-log' },
  { page: 'settings', icon: <SettingOutlined />, name: 'activityBar.settings' },
]

/**
 * The rail along the far left: the five pages this application has.
 *
 * It is the answer to a window that could be folded away and then not be found
 * again. The explorer is a pane that can be hidden, and a program whose explorer
 * has been hidden has to keep a door to it somewhere that cannot itself be
 * hidden — so the five things the main area can show are named here, always, and
 * each one is one click away. *Connections* is the working area, where the
 * explorer and the windows the connections open live; the other four are pages
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
  return (
    <nav className="dm-rail" aria-label={t('activityBar.pages')}>
      {RAIL_ITEMS.map((item) => (
        <RailItem
          key={item.page}
          page={item.page}
          current={page}
          icon={item.icon}
          name={t(item.name)}
          // A page that cannot be opened has to say so on the item itself: a door
          // that is there but locked reads better than one that has been taken
          // away, and a locked door has to explain itself.
          hint={disabledReason?.[item.page] ?? hints?.[item.page] ?? t(item.name)}
          disabledReason={disabledReason?.[item.page]}
          onSelect={onSelect}
        />
      ))}
    </nav>
  )
}

function RailItem({
  page,
  current,
  icon,
  name,
  hint,
  disabledReason,
  onSelect,
}: {
  page: AppPage
  current: AppPage
  icon: ReactNode
  name: string
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
          aria-label={name}
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
