import type { ReactNode } from 'react'
import { Dropdown, Tag } from 'antd'
import type { MenuProps } from 'antd'

import type { DriverInfo, DriverType } from '../api/types'
import { driverSummary } from '../connection'
import { driverIconOrLogo } from '../lib/assets'
import { useAppStore } from '../store/appStore'

/**
 * "New connection" is a menu, not a dialog.
 *
 * The driver list opens right under whatever the user just clicked — the
 * ribbon's Connection button, the explorer's `+` (or its empty state), the blank
 * area's context menu, File ▸ New Connection — and picking a driver opens the
 * page built for it. One list, four anchors, no intermediate screen.
 *
 * Drivers without a page stay visible but disabled: the registry decides that,
 * and an honest "planned" beats an entry that leads nowhere.
 */
const PLANNED = 'planned:'
const NONE = 'none'

/**
 * The driver items, keyed `<prefix><type>` so the same list can be nested under
 * a parent menu (`file.new.mysql`) or used on its own (`mysql`).
 */
export function connectionTypeItems(drivers: DriverInfo[], prefix = ''): MenuProps['items'] {
  const sorted = [...drivers].sort((a, b) => a.sortOrder - b.sortOrder)
  const items: NonNullable<MenuProps['items']> = sorted
    .filter((driver) => driver.implemented)
    .map((driver) => ({
      key: `${prefix}${driver.type}`,
      icon: driverIcon(driver.type),
      label: driverLabel(driver, driverSummary(driver.type) ?? driver.notes ?? ''),
    }))

  const planned = sorted.filter((driver) => !driver.implemented)
  if (planned.length) items.push({ type: 'divider' })
  for (const driver of planned) {
    items.push({
      key: `${prefix}${PLANNED}${driver.type}`,
      disabled: true,
      icon: driverIcon(driver.type),
      label: driverLabel(driver, driver.notes ?? 'No driver yet.', true),
    })
  }

  // Bootstrap has not answered yet: say so rather than opening an empty menu.
  if (items.length === 0) {
    items.push({ key: `${prefix}${NONE}`, label: 'Still loading drivers…', disabled: true })
  }
  return items
}

/**
 * The driver a menu key stands for. Only the last segment matters, so nested
 * (`file.new.mysql`) and standalone (`mysql`) keys both work; `undefined` means
 * the item was a disabled placeholder.
 */
export function driverFromKey(key: string): DriverType | undefined {
  if (key.includes(PLANNED)) return undefined
  const type = key.slice(key.lastIndexOf('.') + 1)
  if (!type || type === NONE) return undefined
  return type as DriverType
}

/**
 * The same menu, hung off a button.
 *
 * `children` must be an element that forwards props — an Ant Design `Button`, or
 * a plain DOM node. A component that swallows `onClick` (a `Tooltip`, say) has
 * to be wrapped in a `<span className="dm-dropdown-anchor">` first, or the menu
 * never opens.
 */
export function ConnectionTypeDropdown({
  children,
  disabled,
}: {
  children: ReactNode
  disabled?: boolean
}) {
  const drivers = useAppStore((s) => s.drivers)
  const openEditor = useAppStore((s) => s.openConnectionEditor)

  return (
    <Dropdown
      disabled={disabled}
      trigger={['click']}
      placement="bottomLeft"
      rootClassName="dm-type-menu"
      menu={{
        items: connectionTypeItems(drivers),
        onClick: ({ key }) => {
          const driver = driverFromKey(key)
          if (driver) openEditor({ driver })
        },
      }}
    >
      {children}
    </Dropdown>
  )
}

function driverIcon(type: DriverType) {
  return (
    <img src={driverIconOrLogo(type)} alt="" className="dm-menu-icon" draggable={false} />
  )
}

/** Name on the first line, the driver's one-liner underneath. */
function driverLabel(driver: DriverInfo, summary: string, planned = false) {
  return (
    <span className="dm-type-item">
      <span className="dm-type-name">
        {driver.displayName}
        {planned ? <Tag className="dm-type-tag">planned</Tag> : null}
      </span>
      <span className="dm-type-summary">{summary}</span>
    </span>
  )
}
