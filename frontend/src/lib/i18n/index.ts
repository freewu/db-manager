import { createElement, Fragment, useSyncExternalStore, type ReactNode } from 'react'

import { MESSAGES, type MessageKey } from './messages'
import {
  DEFAULT_LANGUAGE,
  LANGUAGES,
  type Language,
  type MessageParams,
} from './types'

export {
  DEFAULT_LANGUAGE,
  LANGUAGE_CHOICES,
  LANGUAGES,
  parseLanguage,
} from './types'
export type { Language, MessageEntry, MessageParams } from './types'
export type { MessageKey } from './messages'

/**
 * What the window is being written in, as of right now.
 *
 * This module owns the answer rather than the store doing it, because most of
 * the words in the interface are not said by a component: a warning string is
 * built in a formatter, a menu label is built while laying out a row, a filename
 * is made while generating code. All of those call `t` directly, and `t` has to
 * answer without a component being involved.
 *
 * The store still *persists* the choice (see `saveUi` in the store) — it just
 * asks this module what to write down.
 */
let current: Language = DEFAULT_LANGUAGE

// The document is told from the start, not only on a change: `lang` chooses the
// font stack and the line breaking rules, and the first frame should already
// have them right (see `setLanguage`).
if (typeof document !== 'undefined') document.documentElement.lang = current

const listeners = new Set<() => void>()

/** The language in use. */
export function getLanguage(): Language {
  return current
}

/**
 * Switches the interface to a language.
 *
 * `document` is told too, because the browser uses `lang` to pick fonts and line
 * breaking rules — a Chinese paragraph set in a Latin-only stack looks wrong
 * even when every string is translated.
 */
export function setLanguage(next: Language): void {
  if (next === current) return
  current = next
  if (typeof document !== 'undefined') document.documentElement.lang = next
  for (const listener of listeners) listener()
}

/** Calls `onChange` whenever the language changes. Used by the React hooks. */
export function subscribe(onChange: () => void): () => void {
  listeners.add(onChange)
  return () => {
    listeners.delete(onChange)
  }
}

/**
 * Looks a message up in the current language.
 *
 * The English column is the fallback for a translation that has not been written
 * yet, so an untranslated message shows the words the code was written with
 * rather than a key or an empty line.
 */
function text(key: MessageKey): string {
  const row = MESSAGES[key]
  return row[LANGUAGES.indexOf(current)] || row[0]
}

/** Fills `{name}` placeholders, leaving an unknown one exactly as it was written. */
function fill(template: string, params?: MessageParams): string {
  if (!params) return template
  return template.replace(/\{(\w+)\}/g, (whole, name: string) =>
    params[name] === undefined ? whole : String(params[name]),
  )
}

/**
 * One message, in the language in use.
 *
 * Callers pass dotted keys (`t('queryPane.run')`), which the type system checks
 * against the message tables: a key that does not exist does not compile.
 */
export function t(key: MessageKey, params?: MessageParams): string {
  return fill(text(key), params)
}

/**
 * One message about a count.
 *
 * English needs both forms (`2 tables`) where Chinese writes one, so the message
 * carries them separated by `|` and `count` picks: the first form is singular,
 * the second everything else. `{n}` is the count, filled in automatically.
 *
 * A `{n}` of its own in `params` wins, so a count the caller has already grouped
 * (`1,024`) can be shown that way while the *form* is still chosen by the number.
 */
export function tn(key: MessageKey, count: number, params?: MessageParams): string {
  const forms = text(key).split('|')
  const template = forms.length > 1 ? forms[count === 1 ? 0 : 1] : forms[0]
  return fill(template, { n: count, ...params })
}

/**
 * One message whose values are markup rather than text.
 *
 * A hint that names a literal in the middle of a sentence (`write @@ for a
 * literal @`) wants that literal styled. Building it out of two messages around
 * a `<span>` would freeze one language's word order into the code, so the whole
 * sentence stays in the message table and the markup is passed as a value.
 */
export function tr(key: MessageKey, params: Record<string, ReactNode>): ReactNode {
  const parts = text(key).split(/(\{\w+\})/)
  return createElement(
    Fragment,
    null,
    ...parts.map((part, index) => {
      const name = index % 2 === 1 ? part.slice(1, -1) : undefined
      const value = name === undefined ? undefined : params[name]
      return createElement(Fragment, { key: index }, value === undefined ? part : value)
    }),
  )
}

/** The language in use, as a value a component re-renders on. */
export function useLanguage(): Language {
  return useSyncExternalStore(subscribe, getLanguage, getLanguage)
}

/**
 * `t` for the rare component that has to re-render on its own.
 *
 * Ordinary components only need to import `t`: the language is switched from one
 * place (`App`), and a change there re-renders the whole tree below it. A
 * component wrapped in `memo`, or holding a label in `useMemo`, does not get that
 * and has to subscribe here.
 */
export function useT(): typeof t {
  useLanguage()
  return t
}
