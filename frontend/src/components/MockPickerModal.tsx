/**
 * The placeholder picker of the data generation window.
 *
 * It is a catalogue, not an editor: picking a row sets that field's mock and
 * closes the modal. The field stays editable, so a template that starts from a
 * picked placeholder (`order-@natural(1, 9999)`) is written in the cell, where
 * the description column can explain it.
 *
 * Searching looks at both the placeholder and its description, and flattens the
 * categories while it is active — the user who types "邮箱" does not want to be
 * told which tab that is on.
 */
import { useEffect, useMemo, useState } from 'react'
import { Empty, Input, Modal, Tabs, Tag, Typography } from 'antd'

import { PLACEHOLDER_GROUPS, type Placeholder } from '../lib/mock'

interface MockPickerModalProps {
  /** The field being filled; the modal is closed while this is undefined. */
  field?: string
  /** Called with the placeholder text, which becomes the field's mock. */
  onPick: (value: string) => void
  onClose: () => void
}

export function MockPickerModal({ field, onPick, onClose }: MockPickerModalProps) {
  const [query, setQuery] = useState('')
  const [group, setGroup] = useState(PLACEHOLDER_GROUPS[0].key)
  const open = field !== undefined

  // Every field starts from a clean picker: carrying the last search over to
  // another column would answer a question nobody asked.
  useEffect(() => {
    if (!open) return
    setQuery('')
    setGroup(PLACEHOLDER_GROUPS[0].key)
  }, [open, field])

  const needle = query.trim().toLowerCase()
  const matches = useMemo(() => {
    if (!needle) return []
    const found: { group: string; item: Placeholder }[] = []
    for (const entry of PLACEHOLDER_GROUPS) {
      for (const item of entry.items) {
        if (item.value.toLowerCase().includes(needle) || item.desc.toLowerCase().includes(needle)) {
          found.push({ group: entry.label, item })
        }
      }
    }
    return found
  }, [needle])

  return (
    <Modal
      open={open}
      title={field ? `Placeholder for “${field}”` : 'Placeholder'}
      onCancel={onClose}
      footer={
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          A mock is a template: text with placeholders in it, so{' '}
          <span className="mono">user_@natural(1, 999)</span> is a value too. Write{' '}
          <span className="mono">@@</span> for a literal @.
        </Typography.Text>
      }
      width={640}
      destroyOnHidden
    >
      <Input
        allowClear
        autoFocus
        placeholder="Search a placeholder, e.g. email or 邮箱"
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        style={{ marginBottom: 8 }}
      />
      {needle ? (
        matches.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No placeholder matches" />
        ) : (
          <div className="dm-mock-list">
            {matches.map(({ group: label, item }) => (
              <button
                key={item.value}
                type="button"
                className="dm-mock-item"
                onClick={() => onPick(item.value)}
              >
                <span className="mono dm-mock-value">{item.value}</span>
                <Tag style={{ marginInlineStart: 0, fontSize: 10, lineHeight: '14px' }}>{label}</Tag>
                <span className="dm-mock-desc">{item.desc}</span>
              </button>
            ))}
          </div>
        )
      ) : (
        <Tabs
          size="small"
          activeKey={group}
          onChange={setGroup}
          items={PLACEHOLDER_GROUPS.map((entry) => ({
            key: entry.key,
            label: entry.label,
            children: (
              <div className="dm-mock-list">
                {entry.items.map((item) => (
                  <button
                    key={item.value}
                    type="button"
                    className="dm-mock-item"
                    onClick={() => onPick(item.value)}
                  >
                    <span className="mono dm-mock-value">{item.value}</span>
                    <span className="dm-mock-desc">{item.desc}</span>
                  </button>
                ))}
              </div>
            ),
          }))}
        />
      )}
    </Modal>
  )
}
