import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { App as AntApp, Form, Input, Modal, Typography } from 'antd'

import { toMessage } from '../api/client'
import type { ConnectionConfig } from '../api/types'
import { useAppStore } from '../store/appStore'

interface ConnectContextValue {
  /** Opens a session for a saved profile, prompting for a password if needed. */
  connect: (config: ConnectionConfig) => Promise<void>
  /** Id of the profile currently being opened, for per-row spinners. */
  pending: string | null
}

const ConnectContext = createContext<ConnectContextValue | null>(null)

export function useConnect(): ConnectContextValue {
  const value = useContext(ConnectContext)
  if (!value) throw new Error('useConnect must be used inside <ConnectProvider>')
  return value
}

/**
 * Owns the "open a session" flow.
 *
 * Passwords only ever live in this component's local state for the duration of
 * the prompt; the backend decides whether to persist one based on the profile's
 * `savePassword` flag.
 */
export function ConnectProvider({ children }: { children: ReactNode }) {
  const openConnection = useAppStore((s) => s.openConnection)
  const { message } = AntApp.useApp()

  const [pendingId, setPendingId] = useState<string | null>(null)
  const [prompt, setPrompt] = useState<ConnectionConfig | null>(null)
  const [password, setPassword] = useState('')
  const [promptError, setPromptError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const open = useCallback(
    async (config: ConnectionConfig, secret?: string) => {
      setPendingId(config.id)
      try {
        const session = await openConnection({
          connectionId: config.id,
          password: secret,
          readOnly: config.readOnly,
        })
        message.success(`Connected to ${session.name}`)
        return true
      } catch (error) {
        const text = toMessage(error)
        if (config.driver === 'sqlite') {
          message.error(text)
        } else {
          // Offer the credential prompt so the user can retry or fix the secret.
          setPrompt(config)
          setPromptError(text)
        }
        return false
      } finally {
        setPendingId(null)
      }
    },
    [message, openConnection],
  )

  const connect = useCallback(
    async (config: ConnectionConfig) => {
      if (config.driver === 'sqlite' || config.hasPassword) {
        await open(config)
        return
      }
      setPrompt(config)
      setPassword('')
      setPromptError(null)
    },
    [open],
  )

  const closePrompt = useCallback(() => {
    setPrompt(null)
    setPassword('')
    setPromptError(null)
  }, [])

  const submit = useCallback(async () => {
    if (!prompt) return
    setSubmitting(true)
    try {
      const ok = await open(prompt, password)
      if (ok) closePrompt()
    } finally {
      setSubmitting(false)
    }
  }, [closePrompt, open, password, prompt])

  const value = useMemo<ConnectContextValue>(
    () => ({ connect, pending: pendingId }),
    [connect, pendingId],
  )

  return (
    <ConnectContext.Provider value={value}>
      {children}
      <Modal
        open={prompt !== null}
        title={prompt ? `Connect to ${prompt.name}` : 'Connect'}
        okText="Connect"
        confirmLoading={submitting}
        onOk={() => void submit()}
        onCancel={closePrompt}
        destroyOnHidden
      >
        {promptError ? (
          <Typography.Paragraph
            type="danger"
            className="mono"
            style={{ whiteSpace: 'pre-wrap', maxHeight: 160, overflow: 'auto' }}
          >
            {promptError}
          </Typography.Paragraph>
        ) : (
          <Typography.Paragraph type="secondary">
            This profile has no stored password. It is used for this session only unless the
            profile is saved with “remember password”.
          </Typography.Paragraph>
        )}

        <Form layout="vertical" onFinish={() => void submit()}>
          <Form.Item label="Password" style={{ marginBottom: 4 }}>
            <Input.Password
              autoFocus
              value={password}
              placeholder={prompt?.username ? `password for ${prompt.username}` : 'password'}
              onChange={(event) => setPassword(event.target.value)}
              onPressEnter={() => void submit()}
            />
          </Form.Item>
        </Form>
      </Modal>
    </ConnectContext.Provider>
  )
}
