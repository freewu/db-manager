/**
 * What a field's mock starts as.
 *
 * Two questions decide it, in this order: what the column is *called*, and what
 * it *is*. The name is the better hint — a `varchar(64)` called `email` wants an
 * address, not a random string — but it is only trusted when it can be: a hint
 * that does not fit the column's length is dropped, because a `@cname` in a
 * `char(2)` would fail the insert rather than fill it. That is also why a
 * name hint never applies to a numeric, boolean or JSON column: `id` on an
 * integer primary key is a key, not an 身份证号.
 *
 * A column the engine fills itself is left empty instead, which is what makes
 * the generated rows keep their keys (see the window for how an empty mock is
 * treated).
 */
import type { ColumnInfo } from '../../api/types'
import { kindOfColumn, type ColumnKind } from '../codegen'
import type { CompiledTemplate } from './engine'

/** A name hint, and the shortest column it may be used in. */
interface NameHint {
  match: RegExp
  mock: string
  /** The `charMaxLength` below which this hint is dropped. Omitted means any. */
  minLength?: number
}

/**
 * The hints, tried in order; the first match wins.
 *
 * `id` deliberately only matches the spellings that mean "identity card" —
 * `idcard`, `id_card`, `idno`, `identity` — because a bare `id` column is a key
 * far more often than it is a person, and an 18-character value in a `char(6)`
 * is a failed insert. Keys are handled by the primary-key rules below instead.
 */
const NAME_HINTS: NameHint[] = [
  { match: /idcard|id_card|id_no|idno|identity|身份证/i, mock: '@id', minLength: 18 },
  { match: /email|e_mail|mail/i, mock: '@email', minLength: 12 },
  { match: /phone|mobile|tel|手机|电话/i, mock: '@phone', minLength: 11 },
  { match: /bank|card_no|cardno|银行卡|cardnumber/i, mock: '@bankcard', minLength: 16 },
  { match: /plate|车牌/i, mock: '@plate', minLength: 7 },
  { match: /address|addr|地址/i, mock: '@address', minLength: 24 },
  { match: /province|省份|省/i, mock: '@province' },
  { match: /city|城市|市/i, mock: '@city' },
  { match: /county|district|区县|县/i, mock: '@county' },
  { match: /zip|postcode|postal|邮编/i, mock: '@zip', minLength: 6 },
  { match: /company|corp|公司/i, mock: '@company', minLength: 10 },
  { match: /url|link|website|homepage|网址/i, mock: '@url', minLength: 12 },
  { match: /domain|域名/i, mock: '@domain', minLength: 6 },
  { match: /^ip$|ip_addr|ipaddress|\bip\b/i, mock: '@ip', minLength: 7 },
  { match: /guid|uuid/i, mock: '@guid', minLength: 36 },
  { match: /password|passwd|密码/i, mock: '@string(alpha+number, 8, 12)', minLength: 8 },
  { match: /user_?name|account|login|nick|账号|用户名|昵称/i, mock: '@string(lower, 5, 10)', minLength: 5 },
  { match: /remark|note|comment|desc|description|备注|说明/i, mock: '@csentence(3, 10)' },
  { match: /name|姓名|名字/i, mock: '@cname', minLength: 3 },
]

/** The mock a column's type asks for, when its name says nothing. */
function mockForKind(kind: ColumnKind, column: ColumnInfo): string {
  switch (kind) {
    case 'int':
      return '@integer(1, 100)'
    case 'long':
      return '@natural(1, 1000000)'
    case 'float':
      return '@float(0, 100, 2)'
    case 'double':
      return '@float(0, 10000, 2)'
    case 'decimal':
      return '@float(0, 10000, 2)'
    case 'bool':
      return '@boolean'
    case 'date':
      return '@date(yyyy-MM-dd)'
    case 'time':
      return '@time(HH:mm:ss)'
    case 'datetime':
      return '@datetime(yyyy-MM-dd HH:mm:ss)'
    case 'json':
      // Valid JSON on its own, and still a template: the quoted placeholder is
      // rendered inside the string.
      return '{"key": "@word"}'
    case 'uuid':
      return '@guid'
    case 'bytes':
      return '@string(lower, 4, 8)'
    default:
      // A string, and any type this build cannot map — a text value is the
      // least wrong guess, and it is visible in the window before it is used.
      return stringMock(column)
  }
}

/** A string mock that fits the column's declared length. */
function stringMock(column: ColumnInfo): string {
  const width = column.charMaxLength && column.charMaxLength > 0 ? column.charMaxLength : 8
  const size = Math.max(1, Math.min(width, 16))
  return `@string(lower, ${size}, ${size})`
}

/** The name hint for a column, when one applies. */
function hintFor(column: ColumnInfo, kind: ColumnKind): string | undefined {
  // A name only ever describes text. What an `int` or a `json` column holds is
  // settled by its type, and a hint there would be worse than no hint.
  if (kind !== 'string' && kind !== 'unknown') return undefined
  const name = column.name ?? ''
  for (const hint of NAME_HINTS) {
    if (!hint.match.test(name)) continue
    const width = column.charMaxLength ?? 0
    if (hint.minLength && width > 0 && width < hint.minLength) continue
    return hint.mock
  }
  return undefined
}

/**
 * The mock a column starts with.
 *
 * Empty means "leave this column out of the INSERT", which is what every
 * auto-increment column gets — the engine has to be the one to hand out those
 * keys, or a run would collide with itself.
 */
export function defaultMock(column: ColumnInfo): string {
  if (column.autoIncrement) return ''
  const kind = kindOfColumn(column)

  if (column.primaryKey) {
    // A key that the engine does not fill has to be unique across the run, so
    // it gets the one placeholder that counts rather than draws.
    if (kind === 'int' || kind === 'long' || kind === 'decimal') return '@increment(1)'
    if (kind === 'uuid') return '@guid'
    if (kind === 'string' || kind === 'unknown') return stringMock(column)
  }

  return hintFor(column, kind) ?? mockForKind(kind, column)
}

/**
 * The line the window's description column shows for a field.
 *
 * It answers "what will this column get?", which is not always the same
 * question as "what does this placeholder do" — an auto-increment column's key
 * is the engine's business, and a ticked column with no mock is a run that
 * cannot start rather than a value that is somehow empty. Whether the column is
 * sent at all is the tick's decision, not this line's (see the window).
 */
export function mockDescription(
  column: ColumnInfo,
  template: string,
  compiled: CompiledTemplate,
): string {
  if (compiled.error) return compiled.error
  if (template.trim() === '') {
    return column.autoIncrement
      ? 'Auto-increment — the engine assigns this column'
      : 'No mock written — write one, or untick the field'
  }
  if (compiled.note) return compiled.note
  return 'Literal value, used as is'
}
