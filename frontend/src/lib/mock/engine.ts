/**
 * The placeholder engine behind the data generation window's "mock" column.
 *
 * A mock is a *template*: literal text with `@placeholders` in it, in mock.js
 * syntax — `@cname`, `@integer(1, 100)`, or `user_@natural(1, 999)@tld`. That
 * syntax is what people already have in their fixtures, and it is expressive
 * enough that a column with a format (`@date(yyyy-MM-dd)`) or a prefix needs no
 * second mechanism.
 *
 * Two things are deliberately *not* mock.js' behaviour:
 *
 *   - A name this module does not know is an error, not a literal. mock.js
 *     leaves `@nope` in the output, which turns a typo into a column full of
 *     the same wrong string; here the cell turns red and Generate refuses, so
 *     the mistake is visible before anything is written.
 *   - The catalogue is closed and implemented locally. mock.js ships Chinese
 *     data behind an optional extension, and the placeholders this app offers
 *     for it (身份证、车牌、银行卡 …) are not in mock.js at all.
 *
 * Everything here is pure and seeded: `createRng` can be given a seed, so a run
 * can be repeated in a test, and rendering a row touches nothing but the
 * functions below.
 *
 * Beside the built-in catalogue there is a second, user-defined one: the
 * placeholders kept in the settings page (see CustomPlaceholder). They are not a
 * second engine — a custom placeholder is a name for a template of the built-in
 * ones, and it is expanded where it is written, so a template that is exactly one
 * custom placeholder keeps that value's own type just as a built-in would.
 */
import type { ColumnKind } from '../codegen'
import { PLACEHOLDER_NOTES } from './catalog'
import {
  BANK_PREFIXES,
  CITIES,
  CJK_CHARS,
  COUNTIES,
  COMPANY_PREFIXES,
  COMPANY_SUFFIXES,
  GIVEN_CHARS,
  ID_AREAS,
  LATIN_WORDS,
  MOBILE_PREFIXES,
  PLATE_PROVINCES,
  PROTOCOLS,
  PROVINCES,
  STREETS,
  SURNAMES,
  TLDS,
  USER_AGENTS,
} from './words'

/** What a placeholder can produce. A template that mixes text always produces a
 * string; a lone placeholder keeps its own type, so an integer column gets an
 * integer. */
export type MockValue = string | number | boolean | null

/** The random source every generator is handed. */
export type Rng = () => number

/**
 * One placeholder the user defined, as the settings page stores it and the
 * engine reads it.
 *
 * It is a whole template under a name: `orderNo` may be
 * `SO@date(yyyy)@natural(1000, 9999)`, and a mock then writes `@orderNo`. It
 * takes no arguments — the arguments belong to the built-in placeholder at the
 * bottom of it — so a custom placeholder cannot surprise a reader with a second
 * kind of call syntax.
 */
export interface CustomPlaceholder {
  /** The bare name: what a template writes after `@`. */
  name: string
  /** The template it stands for. */
  template: string
  /** What it produces, shown next to it in the picker and the settings page. */
  description?: string
}

/** A placeholder argument: a number, or text (quoted or bare). */
type Arg = string | number

/* --- random primitives ---------------------------------------------------- */

/**
 * A seeded random source (mulberry32).
 *
 * Seeded rather than `Math.random` so a probe can pin a run down: the same seed
 * and the same templates have to produce the same rows, which is the only way
 * to test a generator without asserting on a range.
 */
export function createRng(seed: number = Date.now()): Rng {
  let state = seed >>> 0
  return () => {
    state = (state + 0x6d2b79f5) >>> 0
    let t = state
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

/** A whole number in [min, max]; the ends are swapped if they arrive backwards. */
function int(rng: Rng, min: number, max: number): number {
  const lo = Math.ceil(Math.min(min, max))
  const hi = Math.floor(Math.max(min, max))
  if (!Number.isFinite(lo) || !Number.isFinite(hi) || hi < lo) return Number.isFinite(lo) ? lo : 0
  return lo + Math.floor(rng() * (hi - lo + 1))
}

function pick<T>(rng: Rng, list: readonly T[]): T {
  return list[int(rng, 0, list.length - 1)]
}

/** `n` characters drawn from a pool. */
function chars(rng: Rng, pool: string, n: number): string {
  if (pool === '' || n <= 0) return ''
  let out = ''
  for (let i = 0; i < n; i += 1) out += pool[int(rng, 0, pool.length - 1)]
  return out
}

function digits(rng: Rng, n: number): string {
  return chars(rng, '0123456789', n)
}

function pad(value: number, width: number): string {
  return String(value).padStart(width, '0')
}

const LOWER = 'abcdefghijklmnopqrstuvwxyz'
const UPPER = LOWER.toUpperCase()
const SYMBOLS = '!@#$%^&*()_+-=[]{}|;:,.<>?'

/** The character pools mock.js knows by name. */
const POOLS: Record<string, string> = {
  lower: LOWER,
  upper: UPPER,
  number: '0123456789',
  alpha: LOWER + UPPER,
  symbol: SYMBOLS,
  'lower+number': LOWER + '0123456789',
  'alpha+number': LOWER + UPPER + '0123456789',
}

/**
 * A pool argument: a known name, a `a+b` combination of them, or a literal
 * character pool (`@string(abc, 3, 3)` uses a, b and c).
 */
function poolOf(value: Arg | undefined, fallback: string): string {
  if (typeof value !== 'string' || value.trim() === '') return fallback
  const text = value.trim()
  const parts = text.split('+').map((part) => POOLS[part.trim()] ?? part)
  return parts.join('') || fallback
}

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1)
}

/* --- dates ---------------------------------------------------------------- */

/**
 * mock.js' date format tokens, longest first.
 *
 * One pass, not a chain of replaces: `yyyy` expands to `2024`, and a later
 * `yy` rule would happily match inside that and turn it into `2424`.
 */
const DATE_FORMAT = /yyyy|yy|SSS|MM|dd|HH|mm|ss|M|d|H|m|s/g

/** Formats a date the way mock.js' `@date('yyyy-MM-dd')` does. */
export function formatDate(date: Date, pattern: string): string {
  return pattern.replace(DATE_FORMAT, (token) => {
    switch (token) {
      case 'yyyy':
        return String(date.getFullYear())
      case 'yy':
        return pad(date.getFullYear() % 100, 2)
      case 'MM':
        return pad(date.getMonth() + 1, 2)
      case 'M':
        return String(date.getMonth() + 1)
      case 'dd':
        return pad(date.getDate(), 2)
      case 'd':
        return String(date.getDate())
      case 'HH':
        return pad(date.getHours(), 2)
      case 'H':
        return String(date.getHours())
      case 'mm':
        return pad(date.getMinutes(), 2)
      case 'm':
        return String(date.getMinutes())
      case 'ss':
        return pad(date.getSeconds(), 2)
      case 's':
        return String(date.getSeconds())
      case 'SSS':
        return pad(date.getMilliseconds(), 3)
      default:
        return token
    }
  })
}

/** A date between two moments, drawn at second resolution. */
function dateBetween(rng: Rng, from: Date, to: Date): Date {
  const span = Math.max(0, Math.floor((to.getTime() - from.getTime()) / 1000))
  return new Date(from.getTime() + int(rng, 0, span) * 1000)
}

const RANDOM_DATE_FROM = new Date(1970, 0, 1).getTime()
const RANDOM_DATE_TO = new Date(2030, 11, 31, 23, 59, 59).getTime()

/** Milliseconds a `@now(unit)` offsets backwards by; anything else is a format. */
const NOW_UNITS: Record<string, number> = {
  year: 365 * 24 * 3600 * 1000,
  month: 30 * 24 * 3600 * 1000,
  week: 7 * 24 * 3600 * 1000,
  day: 24 * 3600 * 1000,
  hour: 3600 * 1000,
  minute: 60 * 1000,
  second: 1000,
}

/* --- checksums ------------------------------------------------------------ */

/**
 * Appends the Luhn check digit to a payload.
 *
 * Doubling starts at the rightmost payload digit — the one immediately left of
 * the digit being computed — which is what every card validator does.
 */
function withLuhnDigit(payload: string): string {
  let sum = 0
  for (let i = 0; i < payload.length; i += 1) {
    let value = Number(payload[payload.length - 1 - i])
    if (i % 2 === 0) {
      value *= 2
      if (value > 9) value -= 9
    }
    sum += value
  }
  return payload + String((10 - (sum % 10)) % 10)
}

/** A mainland ID number: area, birthday, sequence, then the GB 11643 digit. */
function idCard(rng: Rng): string {
  const weights = [7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2]
  const checks = ['1', '0', 'X', '9', '8', '7', '6', '5', '4', '3', '2']
  const year = int(rng, 1940, 2005)
  const body =
    pick(rng, ID_AREAS) +
    String(year) +
    pad(int(rng, 1, 12), 2) +
    pad(int(rng, 1, 28), 2) +
    digits(rng, 3)
  let sum = 0
  for (let i = 0; i < body.length; i += 1) sum += Number(body[i]) * weights[i]
  return body + checks[sum % 11]
}

/** An ISBN-13: the 978 prefix, nine digits, then the weighted check digit. */
function isbn13(rng: Rng): string {
  const body = '978' + digits(rng, 9)
  let sum = 0
  for (let i = 0; i < body.length; i += 1) sum += Number(body[i]) * (i % 2 === 0 ? 1 : 3)
  return body + String((10 - (sum % 10)) % 10)
}

/** A card number: a real issuer prefix, filler digits, Luhn's check digit. */
function bankCard(rng: Rng): string {
  let payload = pick(rng, BANK_PREFIXES)
  while (payload.length < 15) payload += digits(rng, 1)
  return withLuhnDigit(payload)
}

/* --- arguments ------------------------------------------------------------ */

/**
 * Reads a placeholder's argument list.
 *
 * Arguments are comma separated and either quoted (`'text'`, `"text"`) or bare;
 * a bare argument that is a number becomes one, anything else stays text — so
 * `@date(yyyy-MM-dd)` and `@date('yyyy-MM-dd')` mean the same thing, and a
 * format with spaces in it needs no quotes.
 *
 * Returns the arguments, or the message explaining what is wrong with them.
 */
function parseArgs(raw: string): Arg[] | string {
  const args: Arg[] = []
  let i = 0
  while (i < raw.length) {
    while (i < raw.length && /\s/.test(raw[i])) i += 1
    if (i >= raw.length) break

    const quote = raw[i]
    if (quote === "'" || quote === '"') {
      const end = raw.indexOf(quote, i + 1)
      if (end < 0) return `an argument is missing its closing ${quote}`
      args.push(raw.slice(i + 1, end))
      i = end + 1
    } else {
      let end = i
      while (end < raw.length && raw[end] !== ',') end += 1
      args.push(atom(raw.slice(i, end)))
      i = end
    }

    while (i < raw.length && /\s/.test(raw[i])) i += 1
    if (i >= raw.length) break
    if (raw[i] !== ',') return 'arguments are separated by commas'
    i += 1
  }
  return args
}

/** A bare argument: a number when it is one, text otherwise. */
function atom(token: string): Arg {
  const text = token.trim()
  if (text === '') return ''
  const number = Number(text)
  return Number.isFinite(number) ? number : text
}

/** Whether an argument reads as a number, quoted or bare. */
function readsAsNumber(value: Arg | undefined): boolean {
  if (typeof value === 'number') return Number.isFinite(value)
  if (typeof value === 'string' && value.trim() !== '') return Number.isFinite(Number(value))
  return false
}

function numArg(value: Arg | undefined, fallback: number): number {
  if (typeof value === 'number') return value
  if (typeof value === 'string' && value.trim() !== '') {
    const number = Number(value)
    if (Number.isFinite(number)) return number
  }
  return fallback
}

function strArg(value: Arg | undefined, fallback: string): string {
  if (value === undefined) return fallback
  return typeof value === 'string' ? value : String(value)
}

/** The index of the `)` that closes the `(` at `open`, or -1. */
function matchingParen(text: string, open: number): number {
  let depth = 0
  for (let i = open; i < text.length; i += 1) {
    const char = text[i]
    if (char === "'" || char === '"') {
      const end = text.indexOf(char, i + 1)
      if (end < 0) return -1
      i = end
      continue
    }
    if (char === '(') depth += 1
    else if (char === ')') {
      depth -= 1
      if (depth === 0) return i
    }
  }
  return -1
}

/* --- generators ----------------------------------------------------------- */

/**
 * One placeholder: how many arguments it accepts, and what it produces.
 *
 * `create` is for the placeholders that count rather than draw — one state
 * object per compiled template, so `@increment(1)` keeps counting across the
 * rows of a run instead of restarting in every row.
 */
interface Generator {
  arity: [number, number]
  /**
   * Argument positions that have to read as a number.
   *
   * A length, a bound or a step is a number; a pool, a unit and a date format
   * are text. Saying which is which is what lets `@integer(1;100)` — a stray
   * separator, and a range nobody meant — be refused instead of quietly
   * falling back to the placeholder's default.
   */
  numeric?: number[]
  create?: () => unknown
  make: (rng: Rng, args: Arg[], state: unknown) => MockValue
}

const GENERATORS = new Map<string, Generator>([
  /* person --------------------------------------------------------------- */
  [
    'cname',
    {
      arity: [0, 0],
      make: (rng) => pick(rng, SURNAMES) + chars(rng, GIVEN_CHARS, int(rng, 1, 2)),
    },
  ],
  ['cfirst', { arity: [0, 0], make: (rng) => pick(rng, SURNAMES) }],
  [
    'clast',
    {
      arity: [0, 1],
      numeric: [0],
      make: (rng, args) => chars(rng, GIVEN_CHARS, int(rng, 1, numArg(args[0], 2))),
    },
  ],
  [
    'name',
    {
      arity: [0, 0],
      make: (rng) => `${capitalize(pick(rng, LATIN_WORDS))} ${capitalize(pick(rng, LATIN_WORDS))}`,
    },
  ],
  ['first', { arity: [0, 0], make: (rng) => capitalize(pick(rng, LATIN_WORDS)) }],
  ['last', { arity: [0, 0], make: (rng) => capitalize(pick(rng, LATIN_WORDS)) }],
  ['id', { arity: [0, 0], make: (rng) => idCard(rng) }],
  ['phone', { arity: [0, 0], make: (rng) => pick(rng, MOBILE_PREFIXES) + digits(rng, 8) }],
  ['province', { arity: [0, 0], make: (rng) => pick(rng, PROVINCES) }],
  ['city', { arity: [0, 0], make: (rng) => pick(rng, CITIES) }],
  ['county', { arity: [0, 0], make: (rng) => pick(rng, COUNTIES) }],
  [
    'address',
    {
      arity: [0, 0],
      make: (rng) =>
        pick(rng, PROVINCES) +
        pick(rng, CITIES) +
        pick(rng, COUNTIES) +
        pick(rng, STREETS) +
        String(int(rng, 1, 999)) +
        '号',
    },
  ],
  ['zip', { arity: [0, 0], make: (rng) => String(int(rng, 1, 8)) + digits(rng, 5) }],
  [
    'company',
    {
      arity: [0, 0],
      make: (rng) => pick(rng, COMPANY_PREFIXES) + pick(rng, COMPANY_SUFFIXES),
    },
  ],
  ['bankcard', { arity: [0, 0], make: (rng) => bankCard(rng) }],
  [
    'plate',
    {
      arity: [0, 0],
      make: (rng) =>
        pick(rng, PLATE_PROVINCES) +
        chars(rng, 'ABCDEFGHJKLMNPQRSTUVWXYZ', 1) +
        chars(rng, 'ABCDEFGHJKLMNPQRSTUVWXYZ0123456789', 5),
    },
  ],

  /* web ------------------------------------------------------------------ */
  [
    'email',
    {
      arity: [0, 1],
      make: (rng, args) =>
        `${chars(rng, LOWER + '0123456789', int(rng, 5, 10))}@${strArg(args[0], domain(rng))}`,
    },
  ],
  [
    'url',
    {
      arity: [0, 0],
      make: (rng) => `${pick(rng, PROTOCOLS)}://${domain(rng)}/${chars(rng, LOWER, int(rng, 3, 8))}`,
    },
  ],
  ['domain', { arity: [0, 0], make: (rng) => domain(rng) }],
  [
    'ip',
    {
      arity: [0, 0],
      make: (rng) =>
        [int(rng, 1, 254), int(rng, 1, 254), int(rng, 1, 254), int(rng, 1, 254)].join('.'),
    },
  ],
  ['tld', { arity: [0, 0], make: (rng) => pick(rng, TLDS) }],
  ['protocol', { arity: [0, 0], make: (rng) => pick(rng, PROTOCOLS) }],
  ['ua', { arity: [0, 0], make: (rng) => pick(rng, USER_AGENTS) }],

  /* basic ---------------------------------------------------------------- */
  ['boolean', { arity: [0, 0], make: (rng) => rng() < 0.5 }],
  ['guid', { arity: [0, 0], make: (rng) => uuid(rng) }],
  [
    'character',
    {
      arity: [0, 1],
      make: (rng, args) => chars(rng, poolOf(args[0], LOWER + UPPER + '0123456789'), 1),
    },
  ],
  [
    'string',
    {
      arity: [0, 3],
      numeric: [1, 2],
      make: (rng, args) => {
        const pool = poolOf(args[0], LOWER)
        const min = numArg(args[1], 3)
        const max = numArg(args[2], Math.max(min, 7))
        return chars(rng, pool, int(rng, min, max))
      },
    },
  ],
  [
    'increment',
    {
      arity: [0, 1],
      numeric: [0],
      create: () => ({ value: 0 }),
      make: (_rng, args, state) => {
        const counter = state as { value: number }
        const step = numArg(args[0], 1)
        counter.value += Number.isFinite(step) && step !== 0 ? step : 1
        return counter.value
      },
    },
  ],
  ['isbn', { arity: [0, 0], make: (rng) => isbn13(rng) }],

  /* time ----------------------------------------------------------------- */
  [
    'date',
    {
      arity: [0, 1],
      make: (rng, args) => formatDate(randomMoment(rng), strArg(args[0], 'yyyy-MM-dd')),
    },
  ],
  [
    'time',
    {
      arity: [0, 1],
      make: (rng, args) => {
        const moment = randomMoment(rng)
        moment.setFullYear(2000, 0, 1)
        return formatDate(moment, strArg(args[0], 'HH:mm:ss'))
      },
    },
  ],
  [
    'datetime',
    {
      arity: [0, 1],
      make: (rng, args) =>
        formatDate(randomMoment(rng), strArg(args[0], 'yyyy-MM-dd HH:mm:ss')),
    },
  ],
  [
    'now',
    {
      arity: [0, 2],
      make: (rng, args) => {
        // mock.js' first argument is a unit, and the second a format — but a
        // format is the more useful thing to pass on its own, so a first
        // argument that is not a unit is read as the format.
        const unit = strArg(args[0], '')
        const span = NOW_UNITS[unit]
        const pattern = span === undefined ? args[0] : args[1]
        const moment = new Date()
        if (span !== undefined) moment.setTime(moment.getTime() - int(rng, 0, span))
        return formatDate(moment, strArg(pattern, 'yyyy-MM-dd HH:mm:ss'))
      },
    },
  ],

  /* character ------------------------------------------------------------ */
  ['word', { arity: [0, 0], make: (rng) => pick(rng, LATIN_WORDS) }],
  [
    'sentence',
    {
      arity: [0, 2],
      numeric: [0, 1],
      make: (rng, args) => {
        const min = numArg(args[0], 3)
        const max = numArg(args[1], Math.max(min, 8))
        const count = int(rng, Math.max(1, min), Math.max(1, max))
        const words = Array.from({ length: count }, () => pick(rng, LATIN_WORDS))
        words[0] = capitalize(words[0])
        return `${words.join(' ')}.`
      },
    },
  ],
  [
    'cword',
    {
      arity: [0, 2],
      numeric: [0, 1],
      make: (rng, args) => {
        const min = numArg(args[0], 2)
        const max = numArg(args[1], Math.max(min, 5))
        return chars(rng, CJK_CHARS, int(rng, Math.max(1, min), Math.max(1, max)))
      },
    },
  ],
  [
    'csentence',
    {
      arity: [0, 2],
      numeric: [0, 1],
      make: (rng, args) => {
        const min = numArg(args[0], 3)
        const max = numArg(args[1], Math.max(min, 10))
        return `${chars(rng, CJK_CHARS, int(rng, Math.max(1, min), Math.max(1, max)))}。`
      },
    },
  ],

  /* number --------------------------------------------------------------- */
  [
    'integer',
    {
      arity: [0, 2],
      numeric: [0, 1],
      make: (rng, args) => int(rng, numArg(args[0], 0), numArg(args[1], 1000000)),
    },
  ],
  [
    'natural',
    {
      arity: [0, 2],
      numeric: [0, 1],
      make: (rng, args) => Math.abs(int(rng, numArg(args[0], 0), numArg(args[1], 1000000))),
    },
  ],
  [
    'float',
    {
      arity: [0, 4],
      numeric: [0, 1, 2, 3],
      make: (rng, args) => {
        const min = numArg(args[0], 0)
        const max = numArg(args[1], 1000)
        const least = Math.max(0, numArg(args[2], 2))
        const most = Math.max(least, numArg(args[3], least))
        const places = Math.min(10, int(rng, least, most))
        return Number((min + rng() * (max - min)).toFixed(places))
      },
    },
  ],
])

/**
 * Every name the engine knows by itself.
 *
 * The settings page refuses a custom placeholder that takes one of them: a
 * built-in is what a template means by that name (the engine looks there first),
 * so a custom copy would be an entry the picker offers and no mock ever reaches.
 * Derived from the generators rather than from the catalogue, because this is
 * about what the engine can resolve, not about what the picker happens to show.
 */
export const BUILT_IN_NAMES: ReadonlySet<string> = new Set(GENERATORS.keys())

function randomMoment(rng: Rng): Date {
  return dateBetween(rng, new Date(RANDOM_DATE_FROM), new Date(RANDOM_DATE_TO))
}

/** A host name: a word, sometimes with digits, and a top level domain. */
function domain(rng: Rng): string {
  const stem = pick(rng, LATIN_WORDS) + (rng() < 0.3 ? digits(rng, 1) : '')
  return `${stem}.${pick(rng, TLDS)}`
}

/** A v4-shaped UUID, drawn from the seeded source rather than from crypto. */
function uuid(rng: Rng): string {
  const body = chars(rng, '0123456789abcdef', 32).split('')
  body[12] = '4'
  body[16] = pick(rng, ['8', '9', 'a', 'b'])
  const text = body.join('')
  return [
    text.slice(0, 8),
    text.slice(8, 12),
    text.slice(12, 16),
    text.slice(16, 20),
    text.slice(20),
  ].join('-')
}

/* --- templates ------------------------------------------------------------ */

interface TextNode {
  kind: 'text'
  text: string
}

interface PlaceholderNode {
  kind: 'placeholder'
  name: string
  args: Arg[]
  generator: Generator
  state: unknown
}

type Node = TextNode | PlaceholderNode

/** A template, read once and rendered per row. */
export interface CompiledTemplate {
  /** The template as written. */
  source: string
  /**
   * Why this template cannot be rendered. When set, `render` produces null and
   * the window refuses to generate, naming the field.
   */
  error?: string
  /** What the placeholders in it produce, for the window's description column. */
  note: string
  /** Produce one value. Never throws. */
  render: (rng: Rng) => MockValue
}

const NAME_PATTERN = /^[A-Za-z_][A-Za-z0-9_]*/

/** A seed that depends only on a string: FNV-1a over its code units. */
export function seedOf(text: string): number {
  let hash = 2166136261
  for (let i = 0; i < text.length; i += 1) hash = Math.imul(hash ^ text.charCodeAt(i), 16777619)
  return hash >>> 0
}

/** What one level of a template turned into. */
interface Parsed {
  /** Why it could not be read, if it could not. */
  error?: string
  /** The catalogue lines of the placeholders at this level, in the order met. */
  notes: string[]
}

/** What a parse needs besides the text: the custom placeholders, and the chain
 * of them currently being expanded, which is what a cycle looks like. */
interface ParseContext {
  custom: Map<string, CustomPlaceholder>
  chain: string[]
}

/**
 * Reads one template into nodes, appending to `nodes` as it goes.
 *
 * Recursive, because a custom placeholder is expanded in place: its own nodes
 * take its place, so the surrounding text keeps its order and a template that
 * is nothing but one custom placeholder still ends up as one node (which is
 * what lets it keep the underlying value's type).
 *
 * The notes of a custom placeholder are its own description rather than the
 * ones inside it — "订单号" says more in the description column than
 * "@date + @natural" — so the notes of a nested parse are only used as the
 * fallback when the user wrote no description.
 */
function parseInto(text: string, ctx: ParseContext, nodes: Node[]): Parsed {
  const notes: string[] = []
  const seen = new Set<string>()
  let literal = ''
  let i = 0

  const note = (name: string) => {
    if (seen.has(name)) return
    seen.add(name)
    notes.push(PLACEHOLDER_NOTES.get(name) ?? `@${name}`)
  }

  // Text held back until something is known to follow it: pushing it early would
  // put a custom placeholder's first node after text that came before it.
  const flush = () => {
    if (!literal) return
    nodes.push({ kind: 'text', text: literal })
    literal = ''
  }

  while (i < text.length) {
    const char = text[i]
    if (char !== '@') {
      literal += char
      i += 1
      continue
    }
    if (text[i + 1] === '@') {
      literal += '@'
      i += 2
      continue
    }

    const name = NAME_PATTERN.exec(text.slice(i + 1))?.[0]
    if (!name) return { error: "'@' must start a placeholder name — write @@ for a literal @", notes }
    i += 1 + name.length

    let args: Arg[] = []
    if (text[i] === '(') {
      const close = matchingParen(text, i)
      if (close < 0) return { error: `the arguments of @${name} have no closing ')'`, notes }
      const parsed = parseArgs(text.slice(i + 1, close))
      if (typeof parsed === 'string') return { error: parsed, notes }
      args = parsed
      i = close + 1
    }

    const generator = GENERATORS.get(name)
    if (!generator) {
      const custom = ctx.custom.get(name)
      if (!custom) return { error: `@${name} is not a placeholder this app knows`, notes }
      if (ctx.chain.includes(name)) {
        const path = [...ctx.chain, name].map((part) => `@${part}`).join(' → ')
        return { error: `@${name} expands into itself (${path})`, notes }
      }
      if (args.length > 0) {
        return { error: `@${name} takes no arguments — a custom placeholder is a whole template`, notes }
      }
      const body = custom.template.trim()
      if (body === '') return { error: `@${name} has no template to render`, notes }

      flush()
      const nested = parseInto(body, { ...ctx, chain: [...ctx.chain, name] }, nodes)
      if (nested.error) return { error: `in @${name}: ${nested.error}`, notes }
      // The description says more than the parts it is made of; with no
      // description, the parts are the next best answer.
      if (!seen.has(name)) {
        seen.add(name)
        notes.push(custom.description?.trim() || nested.notes.join(' + ') || `@${name}`)
      }
      continue
    }

    const [least, most] = generator.arity
    if (args.length < least || args.length > most) {
      return { error: `@${name} ${arityText(generator.arity)}`, notes }
    }
    const badIndex = (generator.numeric ?? []).find(
      (index) => args[index] !== undefined && !readsAsNumber(args[index]),
    )
    if (badIndex !== undefined) {
      return {
        error: `@${name} expects a number as argument ${badIndex + 1}, not “${String(args[badIndex])}”`,
        notes,
      }
    }

    flush()
    nodes.push({ kind: 'placeholder', name, args, generator, state: generator.create?.() })
    note(name)
  }

  flush()
  return { notes }
}

/** Compiles a template, reporting what is wrong with it instead of throwing. */
export function compileTemplate(
  source: string,
  placeholders?: readonly CustomPlaceholder[],
): CompiledTemplate {
  const text = source.trim()
  const custom = new Map<string, CustomPlaceholder>()
  for (const placeholder of placeholders ?? []) custom.set(placeholder.name, placeholder)

  const nodes: Node[] = []
  const parsed = parseInto(text, { custom, chain: [] }, nodes)
  const note = parsed.notes.filter(Boolean).join(' + ')

  if (parsed.error) {
    return { source: text, error: parsed.error, note, render: () => null }
  }

  // A template that is exactly one placeholder keeps that value's own type, so
  // an integer column is handed an integer. Anything around it makes the result
  // text — there is nowhere else for a mixed value to go.
  const single = nodes.length === 1 && nodes[0].kind === 'placeholder' ? nodes[0] : undefined
  return {
    source: text,
    note,
    render: (rng) =>
      single
        ? single.generator.make(rng, single.args, single.state)
        : nodes
            .map((node) =>
              node.kind === 'text'
                ? node.text
                : String(node.generator.make(rng, node.args, node.state) ?? ''),
            )
            .join(''),
  }
}

/** One rendered example of a template, for the picker's tiles and the settings
 * page's debug panel. */
export interface Sample {
  /** What the template produced, as text. Missing when it produced nothing. */
  value?: string
  /** Why it produced nothing. */
  error?: string
  /** The catalogue line: what the placeholders in it are. */
  note: string
}

/**
 * Renders one example of a template.
 *
 * The seed is the caller's (by default a hash of the template itself), so the
 * same template gives the same example every time the window redraws, while a
 * different one gets a different example. An example that flickered while the
 * user read it would be worse than none at all, and a template that cannot be
 * rendered reports the error rather than an empty box.
 */
export function sampleOf(
  source: string,
  placeholders?: readonly CustomPlaceholder[],
  seed: number = seedOf(source),
): Sample {
  const compiled = compileTemplate(source, placeholders)
  if (compiled.error) return { error: compiled.error, note: compiled.note }
  if (source.trim() === '') return { note: compiled.note }
  const value = compiled.render(createRng(seed))
  if (value === null || value === undefined || value === '') return { note: compiled.note }
  return { value: String(value), note: compiled.note }
}

function arityText([least, most]: [number, number]): string {
  if (least === most) return least === 0 ? 'takes no arguments' : `takes ${least} argument(s)`
  return `takes between ${least} and ${most} arguments`
}

/* --- coercion ------------------------------------------------------------- */

/**
 * Makes a rendered value fit the column it is bound for.
 *
 * Only two things happen here, and neither of them invents a value: a numeric
 * column given numeric text becomes that number (`@string(number, 4, 4)` on an
 * `int` column is still meant to be an integer), and a boolean column given
 * `'true'`/`'1'`/`'no'` becomes the boolean it spells. Anything else travels as
 * it was rendered, so a value that cannot fit is refused by the engine with its
 * own message rather than silently replaced here.
 */
export function coerceMockValue(value: MockValue, kind: ColumnKind): MockValue {
  if (kind === 'int' || kind === 'long' || kind === 'float' || kind === 'double' || kind === 'decimal') {
    if (typeof value === 'number') return value
    if (typeof value === 'string' && value.trim() !== '') {
      const number = Number(value.trim())
      if (Number.isFinite(number)) return number
    }
    return value
  }
  if (kind === 'bool' && typeof value === 'string') {
    const text = value.trim().toLowerCase()
    if (['true', 't', '1', 'yes', 'y'].includes(text)) return true
    if (['false', 'f', '0', 'no', 'n'].includes(text)) return false
  }
  return value
}
