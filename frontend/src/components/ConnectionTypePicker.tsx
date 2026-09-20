import { Drawer, Tag } from 'antd'

import { driverIconOrLogo } from '../lib/assets'
import { driverSummary } from '../connection'
import { useAppStore } from '../store/appStore'
import type { DriverInfo } from '../api/types'

/**
 * "New connection" starts here: a side panel listing the drivers we can talk
 * to, so the user picks the kind of database first and lands on the form built
 * for it, rather than on a generic form that has to hide half its fields.
 *
 * Drivers without a page yet stay visible but disabled — the registry decides,
 * not this component, and an honest "planned" beats an empty menu.
 */
export function ConnectionTypePicker() {
  const open = useAppStore((s) => s.pickerOpen)
  const drivers = useAppStore((s) => s.drivers)
  const closePicker = useAppStore((s) => s.closeConnectionPicker)
  const openEditor = useAppStore((s) => s.openConnectionEditor)

  const sorted = [...drivers].sort((a, b) => a.sortOrder - b.sortOrder)
  const available = sorted.filter((d) => d.implemented)
  const planned = sorted.filter((d) => !d.implemented)

  const pick = (driver: DriverInfo) => openEditor({ driver: driver.type })

  return (
    <Drawer
      open={open}
      onClose={closePicker}
      placement="right"
      width={330}
      title="New connection"
      rootClassName="dm-type-picker"
      destroyOnHidden
    >
      <p className="dm-type-picker-hint">
        Pick the kind of database to connect to — the form follows the driver.
      </p>

      <div className="dm-type-list">
        {available.map((driver) => (
          <button
            key={driver.type}
            type="button"
            className="dm-type-card"
            onClick={() => pick(driver)}
          >
            <img className="dm-type-icon" src={driverIconOrLogo(driver.type)} alt="" draggable={false} />
            <span className="dm-type-body">
              <span className="dm-type-name">{driver.displayName}</span>
              <span className="dm-type-summary">{driverSummary(driver.type) ?? driver.notes ?? ''}</span>
              <span className="dm-type-facts">{factsOf(driver)}</span>
            </span>
          </button>
        ))}
      </div>

      {planned.length ? (
        <>
          <div className="dm-type-planned-title">Planned</div>
          <div className="dm-type-list">
            {planned.map((driver) => (
              <div key={driver.type} className="dm-type-card is-planned">
                <img
                  className="dm-type-icon"
                  src={driverIconOrLogo(driver.type)}
                  alt=""
                  draggable={false}
                />
                <span className="dm-type-body">
                  <span className="dm-type-name">
                    {driver.displayName} <Tag className="dm-type-tag">planned</Tag>
                  </span>
                  <span className="dm-type-summary">{driver.notes ?? 'No driver yet.'}</span>
                </span>
              </div>
            ))}
          </div>
        </>
      ) : null}
    </Drawer>
  )
}

/** The one-line facts that tell the drivers apart at a glance. */
function factsOf(driver: DriverInfo): string {
  const facts: string[] = []
  if (driver.requiresFile) facts.push('local file')
  else if (driver.defaultPort) facts.push(`port ${driver.defaultPort}`)
  if (driver.supportsSchema) facts.push('schemas')
  else if (driver.supportsDatabase && !driver.requiresFile) facts.push('databases')
  return facts.join(' · ')
}
