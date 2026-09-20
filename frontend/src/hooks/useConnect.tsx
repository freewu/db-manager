import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { App as AntApp, Input, Modal, Typography } from 'antd'

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
 * Connecting is optimistic: the profile is tried with whatever it already has —
 * a stored password, or nothing at all — because plenty of servers (a local
 * `root`, a freshly created PostgreSQL role) want no password, and a prompt for
 * them is pure friction. The dialog only shows up once the server has refused
 * an attempt without a secret; a password typed there goes to the backend, which
 * stores it when the profile opted into "remember password", so the same profile
 * is never asked twice.
 */
export function ConnectProvider({ children }: { children: ReactNode }) {
  const openConnection = useAppStore((s) => s.openConnection)
  const refreshConnections = useAppStore((s) => s.refreshConnections)
  const { message } = AntApp.useApp()

  const [pendingId, setPendingId] = useState<string | null>(null)
  const [prompt, setPrompt] = useState<ConnectionConfig | null>(null)
  const [password, setPassword] = useState('')
  const [promptError, setPromptError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  // The password field submits on Enter through both `onPressEnter` and the
  // dialog's OK button; a ref keeps the two from opening two connections.
  const busy = useRef(false)
  // Which profile the prompt is showing. Re-opening the prompt for the profile
  // that is already on screen (the explorer can ask again while the dialog is
  // up) must not wipe what the user has typed into it.
  const prompted = useRef<string | null>(null)
  // Profiles the server has already turned away once when we asked without a
  // password. Remembered so the next attempt opens the dialog straight away
  // instead of paying for another failed round trip.
  const needPassword = useRef<Set<string>>(new Set())

  const showPrompt = useCallback((config: ConnectionConfig) => {
    if (prompted.current !== config.id) setPassword('')
    prompted.current = config.id
    setPrompt(config)
  }, [])

  /**
   * One attempt at opening a session. Nothing is asked of the user here: the
   * caller decides whether the failure deserves a prompt. Returns the error
   * message, or `null` when the session is up.
   */
  const attempt = useCallback(
    async (config: ConnectionConfig, secret?: string) => {
      setPendingId(config.id)
      try {
        const session = await openConnection({
          connectionId: config.id,
          password: secret,
          readOnly: config.readOnly,
        })
        message.success(`Connected to ${session.name}`)
        needPassword.current.delete(config.id)
        // A secret typed into the prompt may have just been written to the
        // profile; re-reading the list is what flips `hasPassword`, so the next
        // click already knows there is nothing to ask for.
        if (secret) await refreshConnections()
        return null
      } catch (error) {
        return toMessage(error)
      } finally {
        setPendingId(null)
      }
    },
    [message, openConnection, refreshConnections],
  )

  const connect = useCallback(
    async (config: ConnectionConfig) => {
      // A file needs no credentials, and a profile that already holds a password
      // has everything the server can ask for: go straight at it and report what
      // the server says — asking again would be asking the user to retype a
      // secret the app already has.
      if (config.driver === 'sqlite' || config.hasPassword) {
        const failure = await attempt(config)
        if (failure) message.error(failure)
        return
      }

      // No stored secret. If this profile already refused an anonymous attempt,
      // ask right away; otherwise try it first and only ask once it says no.
      if (needPassword.current.has(config.id)) {
        setPromptError(null)
        showPrompt(config)
        return
      }
      const failure = await attempt(config)
      if (!failure) return
      needPassword.current.add(config.id)
      showPrompt(config)
      setPromptError(failure)
    },
    [attempt, message, showPrompt],
  )

  const closePrompt = useCallback(() => {
    prompted.current = null
    setPrompt(null)
    setPassword('')
    setPromptError(null)
  }, [])

  const submit = useCallback(async () => {
    if (!prompt || busy.current) return
    busy.current = true
    setSubmitting(true)
    try {
      const failure = await attempt(prompt, password)
      if (!failure) closePrompt()
      else setPromptError(failure)
    } finally {
      busy.current = false
      setSubmitting(false)
    }
  }, [attempt, closePrompt, password, prompt])

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

        {/* Deliberately not an antd Form: the field is not part of a form, and a
            nameless `Form.Item` pushes every keystroke into a form store keyed by
            an empty name path. */}
        <label className="dm-field-label" htmlFor="dm-connect-password">
          Password
        </label>
        <Input.Password
          id="dm-connect-password"
          autoFocus
          value={password}
          placeholder={prompt?.username ? `password for ${prompt.username}` : 'password'}
          onChange={(event) => setPassword(event.target.value)}
          onPressEnter={() => void submit()}
        />
      </Modal>
    </ConnectContext.Provider>
  )
}
