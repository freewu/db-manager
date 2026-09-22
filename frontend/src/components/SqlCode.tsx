import { useMemo } from 'react'
import type { CSSProperties } from 'react'

import type { DriverType } from '../api/types'
import { tokenizeSql } from '../lib/sqlHighlight'

interface SqlCodeProps {
  /** The script to draw. */
  sql: string
  /** The engine it was written for; it decides how quotes and comments read. */
  driver?: DriverType
  /** `dm-ddl` in most places — this is the box the old `<pre>` used to be. */
  className?: string
  style?: CSSProperties
  /** Draw a `<code>` instead of a `<pre>`, for a statement quoted in a sentence. */
  inline?: boolean
}

/**
 * A read-only script with SQL syntax highlighting.
 *
 * The tokens become React elements rather than an HTML string on purpose: a
 * comment or a string literal can contain anything at all — including something
 * that looks like markup — and nothing in a script should ever be able to
 * become part of the page. There is no `dangerouslySetInnerHTML` here and there
 * does not need to be.
 *
 * Highlighting is a reading aid, never a check: a script that is not even SQL
 * still comes back from the tokeniser, just mostly plain.
 */
export function SqlCode({ sql, driver, className, style, inline = false }: SqlCodeProps) {
  const tokens = useMemo(() => tokenizeSql(sql, driver), [driver, sql])
  const body = tokens.map((token, index) =>
    token.kind === 'plain' ? (
      token.text
    ) : (
      <span key={index} className={`dm-sql-${token.kind}`}>
        {token.text}
      </span>
    ),
  )
  if (inline) {
    return (
      <code className={className} style={style}>
        {body}
      </code>
    )
  }
  return (
    <pre className={className} style={style}>
      {body}
    </pre>
  )
}
