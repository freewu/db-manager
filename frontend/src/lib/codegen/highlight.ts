/**
 * Reading generated code back.
 *
 * The code window draws its text in colour, and this is what decides which word
 * is which. It is the code window's counterpart to `sqlHighlight.ts` — the same
 * reader, one pass over the characters, React elements rather than an HTML
 * string, a word the lists do not know left plain — asked to read nineteen
 * languages instead of one. What each of them is read with lives next door in
 * `lexis.ts`.
 */

import { LEXIS, UNKNOWN_LEXIS } from './lexis'

/** A piece of generated code, tagged with the colour it should be drawn in. */
export type CodeTokenKind =
  | 'plain'
  | 'comment'
  | 'string'
  | 'number'
  | 'keyword'
  | 'type'
  /** `@Data`, `#[derive(Debug)]`, `#include`, `---@field`, `-module`. */
  | 'annotation'
  /** A name behind a sigil: `$id` in PHP and Perl, `@id` in Ruby. */
  | 'ident'
  /** A word a bracket follows — a call, not a declaration. */
  | 'function'

export interface CodeToken {
  kind: CodeTokenKind
  text: string
}

const WORD = /[A-Za-z_][A-Za-z0-9_]*/y
/** Hex, binary, decimal, a fraction, an exponent, and the suffixes C# and Rust add. */
const NUMBER =
  /0[xX][0-9a-fA-F_]+|0[bB][01_]+|\d[\d_]*(?:\.[\d_]+)?(?:[eE][+-]?\d+)?[uUlLfFdDmM]*/y

/** Match a sticky pattern at `index` and return what it matched. */
function matchAt(pattern: RegExp, input: string, index: number): string | null {
  pattern.lastIndex = index
  const match = pattern.exec(input)
  return match ? match[0] : null
}

/** Whether only whitespace stands between `index` and the start of its line. */
function atLineStart(input: string, index: number): boolean {
  for (let i = index - 1; i >= 0; i -= 1) {
    const ch = input[i]
    if (ch === ' ' || ch === '\t') continue
    return ch === '\n'
  }
  return true
}

/**
 * Index just past a run that opens and closes with `marker`.
 *
 * A backslash escapes the character after it, which is how every language here
 * writes a quote inside a string, and an unterminated run swallows the rest of
 * the text rather than leaving the reader to guess where it ended.
 */
function scanQuoted(input: string, start: number, marker: string): number {
  let i = start + marker.length
  while (i < input.length) {
    if (input[i] === '\\') {
      i += 2
      continue
    }
    if (input.startsWith(marker, i)) return i + marker.length
    i += 1
  }
  return input.length
}

/** Index just past the block comment that starts at `start`. */
function scanBlock(input: string, start: number, open: string, close: string): number {
  const found = input.indexOf(close, start + open.length)
  return found === -1 ? input.length : found + close.length
}

/**
 * Split generated code into coloured pieces.
 *
 * Keyword lookups drop the case: one list then covers `String` and `string`,
 * `Integer` and `integer`, which is what the languages themselves do with the
 * words that name their own types. An id this build does not know is read as
 * C-like — comments, strings and numbers still land, only the words stay plain.
 */
export function tokenizeCode(input: string, language: string): CodeToken[] {
  const lexis = LEXIS.get(language) ?? UNKNOWN_LEXIS
  const tokens: CodeToken[] = []

  const push = (kind: CodeTokenKind, text: string) => {
    if (!text) return
    const last = tokens[tokens.length - 1]
    if (kind === 'plain' && last?.kind === 'plain') {
      last.text += text
      return
    }
    tokens.push({ kind, text })
  }

  let i = 0
  while (i < input.length) {
    const ch = input[i]

    // ---- comments -------------------------------------------------------
    // Lua's annotations come first: `---@field` is a comment to the language
    // and a declaration to a reader, and it starts with the two dashes of the
    // comment it also is. Only the marker and its word become one token, so
    // what follows — `integer`, `string`, `table` — reads as the type it is.
    const annotation = lexis.lineAnnotations?.find((marker) => input.startsWith(marker, i))
    if (annotation) {
      const word = matchAt(WORD, input, i + annotation.length) ?? ''
      push('annotation', annotation + word)
      i += annotation.length + word.length
      continue
    }
    const line = lexis.line.find((marker) => input.startsWith(marker, i))
    if (line) {
      const stop = input.indexOf('\n', i)
      // The line break stays inside the comment, so the grey run reaches the
      // margin instead of stopping a character short of it.
      const end = stop === -1 ? input.length : stop + 1
      push('comment', input.slice(i, end))
      i = end
      continue
    }
    const block = lexis.block?.find(([open]) => input.startsWith(open, i))
    if (block) {
      const stop = scanBlock(input, i, block[0], block[1])
      push('comment', input.slice(i, stop))
      i = stop
      continue
    }

    // ---- quoted runs ----------------------------------------------------
    const quote = lexis.quotes.find((marker) => input.startsWith(marker, i))
    if (quote) {
      const stop = scanQuoted(input, i, quote)
      push('string', input.slice(i, stop))
      i = stop
      continue
    }

    // ---- annotations, directives and variables --------------------------
    if (ch === '@' && lexis.at) {
      const word = matchAt(WORD, input, i + 1) ?? ''
      push('annotation', `@${word}`)
      i += 1 + word.length
      continue
    }
    if (lexis.sigils?.includes(ch)) {
      const word = matchAt(WORD, input, i + 1) ?? ''
      if (word) {
        push('ident', ch + word)
        i += 1 + word.length
        continue
      }
    }
    // `#include`, `#[derive(Debug)]`, `-module`: a directive only when the
    // marker opens its line, because `#` is a plain character elsewhere and
    // Erlang writes `->` in the middle of a clause. The marker's first
    // character is compared before the line is walked back, so asking this at
    // every character costs nothing.
    const attribute = lexis.lineAttributes?.find(
      (marker) => marker[0] === ch && atLineStart(input, i) && input.startsWith(marker, i),
    )
    if (attribute) {
      const word = matchAt(WORD, input, i + attribute.length) ?? ''
      push('annotation', attribute + word)
      i += attribute.length + word.length
      continue
    }
    if (ch === '<') {
      const opener = lexis.openers?.find((marker) => input.startsWith(marker, i))
      if (opener) {
        push('keyword', opener)
        i += opener.length
        continue
      }
    }

    // ---- words and numbers ----------------------------------------------
    // A word before a bracket is a call, unless a list already claimed it:
    // `if (` is a keyword and `int64_t (` would still be a type.
    const word = matchAt(WORD, input, i)
    if (word) {
      const lower = word.toLowerCase()
      if (lexis.keywords.has(lower)) push('keyword', word)
      else if (lexis.types.has(lower)) push('type', word)
      else if (input[i + word.length] === '(') push('function', word)
      else push('plain', word)
      i += word.length
      continue
    }
    const number = matchAt(NUMBER, input, i)
    if (number) {
      push('number', number)
      i += number.length
      continue
    }

    // ---- everything else ------------------------------------------------
    push('plain', ch)
    i += 1
  }

  return tokens
}
