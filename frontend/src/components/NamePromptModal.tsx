import { useCallback, useEffect, useState } from 'react'
import { App as AntApp, Input, Modal, Typography } from 'antd'

import { toMessage } from '../api/client'

interface NamePromptModalProps {
  open: boolean
  title: string
  okText: string
  placeholder: string
  initial: string
  hint?: string
  onSubmit: (name: string) => Promise<void>
  onClose: () => void
}

/**
 * A one-field name window.
 *
 * Create and rename look the same to a user, so they are the same window: the
 * caller says what the title should read and what to do with the answer. A
 * failed submit keeps the window open with the name still in it — losing a typed
 * name to a refused save would be the second mistake in a row. That is also why
 * the backend has the last word on whether a name is usable at all (empty, too
 * long, already taken): the window only rules out the empty one, which it can
 * see on screen.
 */
export function NamePromptModal({
  open,
  title,
  okText,
  placeholder,
  initial,
  hint,
  onSubmit,
  onClose,
}: NamePromptModalProps) {
  const { message } = AntApp.useApp()
  const [name, setName] = useState(initial)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    setName(initial)
  }, [initial, open])

  const submit = useCallback(async () => {
    const trimmed = name.trim()
    if (!trimmed) return
    setSaving(true)
    try {
      await onSubmit(trimmed)
      onClose()
    } catch (error) {
      message.error(toMessage(error))
    } finally {
      setSaving(false)
    }
  }, [message, name, onClose, onSubmit])

  return (
    <Modal
      open={open}
      title={title}
      okText={okText}
      confirmLoading={saving}
      okButtonProps={{ disabled: name.trim() === '' }}
      onOk={() => void submit()}
      onCancel={onClose}
    >
      <Input
        autoFocus
        value={name}
        placeholder={placeholder}
        onChange={(event) => setName(event.target.value)}
        onPressEnter={() => void submit()}
      />
      {hint ? (
        <Typography.Text type="secondary" style={{ fontSize: 12, display: 'block', marginTop: 8 }}>
          {hint}
        </Typography.Text>
      ) : null}
    </Modal>
  )
}
