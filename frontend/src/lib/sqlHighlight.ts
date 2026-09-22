import type { DriverType } from '../api/types'
import { isMySQLFamily } from './sqlFlavor'

/**
 * A piece of a script, tagged with the colour it should be drawn in.
 *
 * `plain` is the catch-all — whitespace, punctuation, operators and every word
 * this highlighter has no opinion about — and it is emitted in runs, so a
 * script the highlighter understands nothing about still comes back as a
 * handful of tokens.
 */
export type SqlTokenKind =
  | 'plain'
  | 'comment'
  | 'string'
  | 'number'
  | 'keyword'
  | 'type'
  | 'ident'
  | 'function'

export interface SqlToken {
  kind: SqlTokenKind
  text: string
}

/**
 * How one engine's scripts have to be read.
 *
 * Everything the highlighter can see is a comment, a quoted run or a word, and
 * the engines disagree about the first two:
 *
 * - MySQL takes `#` comments, backslash escapes inside strings, `"…"` as a
 *   string, and backtick-quoted names;
 * - PostgreSQL reads `"…"` as a name and `$tag$…$tag$` as a string (that is how
 *   a function body survives the parser), has no backslash escapes, and nests
 *   block comments where MySQL ends them at the first one;
 * - SQLite and SQL Server also bracket names as `[…]`;
 * - the MongoDB shell is JavaScript, so `//` opens a comment, `--` is a
 *   decrement rather than a comment, and the SQL word lists have no say at all:
 *   `$set` is a document key there, not a statement;
 * - Oracle and anything this build has not been taught yet get the standard
 *   reading, which is also the one that colours the least.
 *
 * A dialect only decides between those readings. The word lists below are the
 * union of all eight engines on purpose: a keyword one engine does not have is
 * still a keyword to read.
 */
type Dialect = 'sql' | 'mysql' | 'postgres' | 'sqlite' | 'sqlserver' | 'mongodb'

/** The dialect a driver's scripts are written in. */
function dialectOf(driver: DriverType | undefined): Dialect {
  if (isMySQLFamily(driver)) return 'mysql'
  switch (driver) {
    case 'postgres':
      return 'postgres'
    case 'sqlite':
      return 'sqlite'
    case 'sqlserver':
      return 'sqlserver'
    case 'mongodb':
      return 'mongodb'
    default:
      return 'sql'
  }
}

/**
 * Statements, clauses and attributes — uppercase, compared case-insensitively.
 *
 * A word that is missing here stays plain, which is the quiet failure mode: the
 * script still reads, it just loses one colour. A word that is here and should
 * not be is louder, so the list stays with the words that shape a statement
 * rather than growing into a dictionary.
 */
const SQL_KEYWORDS = new Set(
  (
    'ADD ALL ALTER ANALYZE AND AS ASC AUTO_INCREMENT BEGIN BETWEEN BY CASCADE CASE CHANGE ' +
    'CHARACTER CHARACTERISTICS CHARSET CHECK COALESCE COLLATE COLLATION COLUMN COMMENT COMMIT ' +
    'CONCURRENTLY CONFLICT CONSTRAINT CREATE CROSS CURRENT_TEMP CURSOR DATABASE DECLARE DEFAULT ' +
    'DEFINER DELAYED DELETE DESC DESCRIBE DETACH DISTINCT DO DROP DUPLICATE ELSE END ENGINE ' +
    'ESCAPE EXCEPT EXCLUDE EXCLUSIVE EXISTS EXPLAIN FALSE FETCH FILTER FIRST FOLLOWING FOR FOREIGN ' +
    'FULL FUNCTION GENERATED GRANT GROUP HAVING IDENTITY IF IGNORE ILIKE IMMUTABLE IN INDEX ' +
    'INHERITS INNER INSERT INTERSECT INTO INVOKER IS JOIN KEY LANGUAGE LAST LATERAL LEFT LIKE ' +
    'LIMIT LOCAL LOCK LOGIN LOOP MATERIALIZED MERGE MODIFY NATURAL NO NOT NOTHING NULL NULLS ' +
    'OF OFFSET ON ONLY OPEN OR ORDER OUTER OVER OWNER PARTIAL PARTITION PASSWORD PLPGSQL PRAGMA ' +
    'PRECEDING PRIMARY PRIVILEGES PROCEDURE PUBLIC QUICK RANGE RECURSIVE REFERENCES REGEXP ' +
    'REINDEX RENAME REPEAT REPLACE RESTRICT RETURN RETURNING RETURNS REVOKE RIGHT ROLE ROLLBACK ' +
    'ROW ROW_FORMAT ROWS SAVEPOINT SCHEMA SECURITY SELECT SEQUENCE SESSION SET SHOW SIGNED ' +
    'STABLE START STORED STRICT TABLE TABLESPACE TEMP TEMPORARY THEN TO TRANSACTION TRIGGER TRUE ' +
    'TRUNCATE UNBOUNDED UNDO UNION UNIQUE UNLOCK UNLOGGED UNSIGNED UPDATE UPSERT USAGE USE USER ' +
    'USING VACUUM VALUES VIEW VIRTUAL WHEN WHERE WHILE WINDOW WITH WORK XOR ZEROFILL'
  ).split(' '),
)

/**
 * Data types, drawn apart from the keywords.
 *
 * A DDL script is mostly types and names, and telling them apart is most of
 * what makes it readable at a glance. `SET` and `INTERVAL` live in the keyword
 * list above because they are statements more often than they are types here.
 */
const SQL_TYPES = new Set(
  (
    'ARRAY BFILE BIGINT BIGSERIAL BINARY BIT BITMAP BLOB BOOL BOOLEAN BOX BYTEA CHAR CIDR ' +
    'CIRCLE CLOB DATE DATETIME DATETIME2 DATEV2 DATETIMEV2 DECIMAL DECIMALV3 DOUBLE ENUM FLOAT ' +
    'FLOAT4 FLOAT8 GEOGRAPHY GEOMETRY HLL HSTORE IMAGE INET INT INT2 INT4 INT8 INTEGER IPV4 ' +
    'IPV6 JSON JSONB LARGEINT LSEG LINESTRING LONG LONGBLOB LONGTEXT MACADDR MEDIUMBLOB ' +
    'MEDIUMINT MEDIUMTEXT MONEY MULTIPOLYGON NCHAR NCLOB NTEXT NUMBER NUMERIC NVARCHAR ' +
    'NVARCHAR2 PATH POINT POLYGON PRECISION RAW REAL ROWID SERIAL SMALLDATETIME SMALLINT ' +
    'SMALLMONEY SMALLSERIAL SQL_VARIANT STRING TEXT TIME TIMESTAMP TIMESTAMPTZ TIMETZ TINYBLOB ' +
    'TINYINT TINYTEXT TSQUERY TSVECTOR UNIQUEIDENTIFIER UROWID UUID VARBINARY VARCHAR VARCHAR2 ' +
    'VARIANT VECTOR XML YEAR'
  ).split(' '),
)

/**
 * The words JavaScript adds, for the MongoDB shell. Only the language and the
 * two globals a shell script actually starts from — `db` is the namespace root
 * there, not a variable. The SQL lists are not consulted for the shell, which
 * is what keeps `$set` and `Date` from being drawn as a statement and a type.
 */
const JS_KEYWORDS = new Set(
  (
    'ASYNC AWAIT BREAK CASE CATCH CLASS CONST CONTINUE DB DELETE DO ELSE EXTENDS ' +
    'FALSE FINALLY FOR FUNCTION IF IN INSTANCEOF LET NEW NULL OF PRINT RETURN SUPER ' +
    'SWITCH THIS THROW TRUE TRY TYPEOF UNDEFINED USE VAR WHILE YIELD'
  ).split(' '),
)

const WORD = /[A-Za-z_][A-Za-z0-9_$]*/y
const NUMBER = /0[xX][0-9a-fA-F]+|\d+(?:\.\d+)?(?:[eE][+-]?\d+)?/y
const DOLLAR_TAG = /\$[A-Za-z_][A-Za-z0-9_]*\$|\$\$/y

/** Whether `\` escapes the next character inside a string on this engine. */
function backslashEscapes(dialect: Dialect): boolean {
  return dialect === 'mysql' || dialect === 'mongodb'
}

/**
 * Match a sticky pattern at `index` and return what it matched.
 *
 * Sticky means the pattern either matches right here or not at all, so the
 * length of the result is also how far the caller has to move.
 */
function matchAt(pattern: RegExp, input: string, index: number): string | null {
  pattern.lastIndex = index
  const match = pattern.exec(input)
  return match ? match[0] : null
}

/**
 * Index just past a quoted run that starts at `start`, or the end of the input
 * when the quote is never closed.
 *
 * `'…'`, `"…"`, `` `…` `` and `[…]` all end the same way: at the closing quote,
 * where a doubled quote is the escape every engine agrees on.
 */
function scanQuoted(input: string, start: number, quote: string, backslash: boolean): number {
  let i = start + 1
  while (i < input.length) {
    const ch = input[i]
    if (backslash && ch === '\\') {
      // MySQL and the shell read `\` as an escape; standard SQL does not, so
      // `'a\'` is a complete string there and an unfinished one here.
      i += 2
      continue
    }
    if (ch === quote) {
      if (input[i + 1] === quote) {
        i += 2
        continue
      }
      return i + 1
    }
    i += 1
  }
  return input.length
}

/** Index just past a block comment that starts at `start`. */
function scanBlockComment(input: string, start: number, nested: boolean): number {
  let depth = 1
  let i = start + 2
  while (i < input.length && depth > 0) {
    if (nested && input[i] === '/' && input[i + 1] === '*') {
      depth += 1
      i += 2
      continue
    }
    if (input[i] === '*' && input[i + 1] === '/') {
      depth -= 1
      i += 2
      continue
    }
    i += 1
  }
  return i
}

/**
 * Split a script into coloured pieces.
 *
 * The reader is a single pass over the characters rather than a pile of
 * regular expressions: what a quote or a comment does to the rest of the script
 * depends on where it is, and that is exactly the state a tokeniser is for.
 *
 * It never throws and never gives up — an unterminated string, comment or
 * bracket swallows the rest of the script instead, because a highlighter that
 * fails on half-typed input is worse than one that colours it oddly.
 */
export function tokenizeSql(input: string, driver: DriverType | undefined): SqlToken[] {
  const dialect = dialectOf(driver)
  const tokens: SqlToken[] = []

  const push = (kind: SqlTokenKind, text: string) => {
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
    const next = input[i + 1]

    // ---- comments -------------------------------------------------------
    // `--` opens one everywhere except the shell, which is JavaScript. MySQL
    // additionally wants whitespace after it; without that it is a minus sign
    // in front of a negative value.
    const dashes =
      ch === '-' &&
      next === '-' &&
      dialect !== 'mongodb' &&
      (dialect !== 'mysql' || input[i + 2] === undefined || /\s/.test(input[i + 2]))
    const hash = ch === '#' && dialect === 'mysql'
    const slashes = ch === '/' && next === '/' && dialect === 'mongodb'
    if (dashes || hash || slashes) {
      const end = input.indexOf('\n', i)
      // The line break stays inside the comment: a grey run that reaches the
      // margin reads better than one that stops a character short of it.
      const stop = end === -1 ? input.length : end + 1
      push('comment', input.slice(i, stop))
      i = stop
      continue
    }
    if (ch === '/' && next === '*') {
      // PostgreSQL nests block comments, MySQL ends at the first `*/`, and a
      // script written for one of them reads fine under the other.
      const stop = scanBlockComment(input, i, dialect === 'postgres')
      push('comment', input.slice(i, stop))
      i = stop
      continue
    }

    // ---- quoted runs ----------------------------------------------------
    if (ch === "'") {
      const stop = scanQuoted(input, i, "'", backslashEscapes(dialect))
      push('string', input.slice(i, stop))
      i = stop
      continue
    }
    if (ch === '"') {
      const stop = scanQuoted(input, i, '"', backslashEscapes(dialect))
      // MySQL reads `"…"` as a string, PostgreSQL and the rest as a name. The
      // shell is JavaScript, where it is a string again.
      const kind = dialect === 'mysql' || dialect === 'mongodb' ? 'string' : 'ident'
      push(kind, input.slice(i, stop))
      i = stop
      continue
    }
    if (ch === '`') {
      const stop = scanQuoted(input, i, '`', false)
      push('ident', input.slice(i, stop))
      i = stop
      continue
    }
    if (ch === '[' && (dialect === 'sqlserver' || dialect === 'sqlite')) {
      const stop = scanQuoted(input, i, ']', false)
      push('ident', input.slice(i, stop))
      i = stop
      continue
    }
    if (ch === '$' && dialect === 'postgres') {
      const tag = matchAt(DOLLAR_TAG, input, i)
      if (tag) {
        const close = input.indexOf(tag, i + tag.length)
        const stop = close === -1 ? input.length : close + tag.length
        push('string', input.slice(i, stop))
        i = stop
        continue
      }
    }

    // ---- words and numbers ----------------------------------------------
    const word = matchAt(WORD, input, i)
    if (word) {
      const upper = word.toUpperCase()
      // A name that runs straight into `(` is a call: `COUNT(`, `HASH(id)`, and
      // `getCollection(` in the shell. A space before the bracket means a
      // definition instead — `CREATE TABLE t (` — so object names stay plain.
      // Types and keywords were asked first, so `VARCHAR(120)` and `VALUES(`
      // keep what they are.
      const called = input[i + word.length] === '('
      if (dialect === 'mongodb') {
        if (JS_KEYWORDS.has(upper)) push('keyword', word)
        else if (called) push('function', word)
        else push('plain', word)
      } else if (SQL_TYPES.has(upper)) push('type', word)
      else if (SQL_KEYWORDS.has(upper)) push('keyword', word)
      else if (called) push('function', word)
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

    // ---- everything else: whitespace, punctuation, operators -------------
    push('plain', ch)
    i += 1
  }

  return tokens
}
