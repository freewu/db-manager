/**
 * The three languages the interface is written in, and the shape of a message.
 *
 * A message is one line holding all three spellings, in a fixed order
 * (`LANGUAGES`): English first, then 简体中文, then 繁體中文. Keeping them on one
 * line is the whole point — a translation can never drift away from the text it
 * translates, a reviewer sees what changed in both at once, and a language the
 * build does not know about cannot be "half present" (a missing column is a
 * compile error, not a hole in the window).
 *
 * A `|` inside a line separates the two forms of a count, English first:
 * `{n} table|{n} tables`. Only `tn` splits on it, so a message that legitimately
 * contains a pipe is safe as long as it is read with `t`.
 */

/** What the user picked, and what a message's columns are indexed by. */
export type Language = 'en' | 'zh-CN' | 'zh-TW'

/** The columns of a message, in order. The index of a language is its index here. */
export const LANGUAGES: readonly Language[] = ['en', 'zh-CN', 'zh-TW']

/**
 * What a fresh install starts in.
 *
 * English rather than the operating system's language: this is a database tool
 * whose warnings quote SQL as the engine wrote it, and a user who wants 中文
 * should be the one to say so rather than have the interface guess from a locale
 * they may not have chosen.
 */
export const DEFAULT_LANGUAGE: Language = 'en'

/**
 * The choices the settings page offers, each written in its own language.
 *
 * These are the one set of strings that is not in the message tables: a language
 * picker that spells 简体中文 in English is useless to the person who needs it, so
 * each entry is already in the language it selects and translating it would be
 * the bug.
 */
export const LANGUAGE_CHOICES: { value: Language; label: string; hint: string }[] = [
  { value: 'en', label: 'English', hint: 'The interface is written in English' },
  { value: 'zh-CN', label: '简体中文', hint: '界面使用简体中文' },
  { value: 'zh-TW', label: '繁體中文', hint: '介面使用繁體中文' },
]

/**
 * Reads a stored preference back.
 *
 * Anything unrecognised — a value from a build that had other languages, or a
 * file edited by hand — falls back to English, which is also what a fresh
 * install gets.
 */
export function parseLanguage(value: unknown): Language {
  return typeof value === 'string' && (LANGUAGES as readonly string[]).includes(value)
    ? (value as Language)
    : DEFAULT_LANGUAGE
}

/**
 * One message: `[english, 简体中文, 繁體中文]`.
 *
 * An empty entry falls back to the English one when it is read, so a message
 * added a moment ago is readable before its translation arrives.
 */
export type MessageEntry = readonly [string, string, string]

/** What a message table looks like: dotted keys, three columns each. */
export type AreaMessages = Record<string, MessageEntry>

/**
 * The values a `{name}` placeholder can be filled with.
 *
 * `undefined` is allowed because the value often comes from a field that is only
 * sometimes there; it leaves the placeholder in the message rather than printing
 * the word `undefined`, and the type checker stops asking every caller to prove
 * what it already knows.
 */
export type MessageParams = Record<string, string | number | undefined>
