import { useCallback, useEffect, useState } from 'react'
import { Alert, App as AntApp, Input, Modal, Radio, Typography } from 'antd'

import { api, toMessage } from '../api/client'
import type { DesignPlan, DesignResult, DriverType } from '../api/types'
import { SqlCode } from './SqlCode'

/** The table being duplicated, and where it lives. */
export interface CopyTableSource {
  sessionId: string
  driver?: DriverType
  database: string
  schema: string
  object: string
}

interface CopyTableModalProps {
  /** The table to copy, or `null` when the window is closed. */
  source: CopyTableSource | null
  onClose: () => void
  /**
   * Called once the script has run, whether or not it ran whole: the catalog
   * moved either way, and a copy that failed halfway may still have left a table
   * behind.
   */
  onFinished: (target: string, withData: boolean, result: DesignResult) => void
}

/** What a copy is called before the user says otherwise. */
export function defaultCopyName(object: string): string {
  return `${object}_copy`
}

/**
 * Duplicating one table into a new one.
 *
 * A copy is a CREATE TABLE — plus, when asked for, one INSERT ... SELECT that
 * moves the rows — so this window is the new-table window with the definition
 * filled in already: the backend reads the structure from the live catalog and
 * renders the script, and the window only asks for the name and whether the rows
 * come along. What is previewed is what runs, because the run plans again from
 * the same request rather than reusing the previewed text, and the window never
 * assembles DDL of its own.
 *
 * The name alone is not enough to render anything useful, so the preview appears
 * once there is one — prefilled with `<table>_copy`, which is what a copy is
 * called nine times out of ten and is also a name the engine will accept.
 */
export function CopyTableModal({ source, onClose, onFinished }: CopyTableModalProps) {
  const { message } = AntApp.useApp()
  const [target, setTarget] = useState('')
  const [withData, setWithData] = useState(false)
  const [plan, setPlan] = useState<DesignPlan | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const object = source?.object ?? ''
  const sessionId = source?.sessionId

  // Every opening starts from the same place: the suggested name, no rows, and
  // nothing remembered from the last table that was copied.
  useEffect(() => {
    setTarget(object ? defaultCopyName(object) : '')
    setWithData(false)
    setPlan(null)
    setError(null)
    setBusy(false)
  }, [object, sessionId])

  const close = useCallback(() => {
    setPlan(null)
    setError(null)
    setBusy(false)
    onClose()
  }, [onClose])

  // The preview is the backend's own script, debounced so a name being typed
  // does not turn into a call per keystroke.
  useEffect(() => {
    const name = target.trim()
    if (!source || !name) {
      setPlan(null)
      setError(null)
      return
    }
    let cancelled = false
    const timer = window.setTimeout(() => {
      api
        .planCopyTable({
          sessionId: source.sessionId,
          database: source.database,
          schema: source.schema,
          object: source.object,
          target: name,
          withData,
        })
        .then((next) => {
          if (cancelled) return
          setPlan(next)
          setError(null)
        })
        .catch((err) => {
          if (cancelled) return
          setPlan(null)
          setError(toMessage(err))
        })
    }, 250)
    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [source, target, withData])

  const submit = useCallback(async () => {
    const name = target.trim()
    if (!source || !name || busy || error) return
    setBusy(true)
    setError(null)
    const request = {
      sessionId: source.sessionId,
      database: source.database,
      schema: source.schema,
      object: source.object,
      target: name,
      withData,
    }
    try {
      const result = await api.copyTable(request)
      onFinished(name, withData, result)
      if (result.error) {
        // The window stays open on a name the engine refused, with the name
        // still in it: retyping it would be the second mistake in a row.
        setError(
          `statement ${result.failedIndex + 1} of ${result.plan.statements.length} failed: ${result.error}`,
        )
        message.error(
          `Copied ${result.executed.length} of ${result.plan.statements.length} statement(s)`,
        )
      } else {
        message.success(
          withData
            ? `Table ${source.object} copied to ${name}, rows included`
            : `Table ${source.object} copied to ${name}`,
        )
        onClose()
      }
    } catch (err) {
      setError(toMessage(err))
    } finally {
      setBusy(false)
    }
  }, [busy, error, message, onClose, onFinished, source, target, withData])

  return (
    <Modal
      open={source !== null}
      title={source ? `Duplicate table ${source.object}` : 'Duplicate table'}
      okText="Create copy"
      confirmLoading={busy}
      okButtonProps={{ disabled: !target.trim() || error !== null }}
      onOk={() => void submit()}
      onCancel={close}
      destroyOnHidden
    >
      <label className="dm-field-label" htmlFor="dm-copy-table-name">
        Name of the copy
      </label>
      <Input
        id="dm-copy-table-name"
        autoFocus
        value={target}
        placeholder={object ? defaultCopyName(object) : 'orders_copy'}
        onChange={(event) => setTarget(event.target.value)}
        onPressEnter={() => void submit()}
      />

      <Radio.Group
        style={{ display: 'flex', flexDirection: 'column', gap: 6, marginTop: 12 }}
        value={withData ? 'data' : 'structure'}
        onChange={(event) => setWithData(event.target.value === 'data')}
      >
        <Radio value="structure">
          Structure only
          <Typography.Text type="secondary" style={{ marginLeft: 6, fontSize: 12 }}>
            the fields, the key and the indexes
          </Typography.Text>
        </Radio>
        <Radio value="data">
          Structure and data
          <Typography.Text type="secondary" style={{ marginLeft: 6, fontSize: 12 }}>
            the same, and the rows with it
          </Typography.Text>
        </Radio>
      </Radio.Group>

      {error ? (
        <Typography.Paragraph type="danger" style={{ marginTop: 12, marginBottom: 0 }}>
          {error}
        </Typography.Paragraph>
      ) : null}

      {plan?.warnings.length ? (
        <Alert
          type="info"
          showIcon
          style={{ marginTop: 12 }}
          title="This engine cannot copy everything as it stands"
          description={
            <ul style={{ margin: 0, paddingLeft: 18 }}>
              {plan.warnings.map((warning, index) => (
                <li key={index}>{warning}</li>
              ))}
            </ul>
          }
        />
      ) : null}

      {plan ? (
        <>
          <SqlCode
            className="dm-ddl"
            style={{ maxHeight: 240, marginTop: 12 }}
            sql={plan.statements.map((statement) => `${statement};`).join('\n')}
            driver={source?.driver}
          />
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {withData
              ? 'The rows are moved by the server, one statement for all of them.'
              : 'The statements run one at a time, in this order.'}
          </Typography.Text>
        </>
      ) : !error ? (
        <Typography.Paragraph type="secondary" style={{ marginTop: 12, marginBottom: 0 }}>
          Name it and the statement appears here.
        </Typography.Paragraph>
      ) : null}
    </Modal>
  )
}
