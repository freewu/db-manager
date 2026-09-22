/**
 * How each language the picker offers has to be read.
 *
 * `highlight.ts` is the reader; this is what it reads with. Everything it can
 * see is a comment, a quoted run, a word or a run of punctuation, and the
 * languages agree on all of those except the first two and the marks around a
 * name:
 *
 * - `line` holds the markers that comment out the rest of a line, longest
 *   first, so `%%` is Erlang's doc comment and `%` its plain one.
 * - `lineAnnotations` are markers that open a comment the language then reads
 *   as a declaration — Lua's `---@field` — and are asked before `line`,
 *   because they are also line comments.
 * - `quotes` are runs that end at the marker they started with, longest first,
 *   so Python's `"""` is one string rather than three empty ones.
 * - `block` is asked for a comment body's opening and closing marker; nothing
 *   here nests, because nothing the generators write nests.
 *
 * The word lists are deliberately short: this reads what the templates next
 * door write, not the languages as a whole. The lists are matched without
 * regard to case, so one entry covers `String` and `string` both — which is
 * also why a column named after a type (`date`, `value`) takes the type's
 * colour in the languages that keep column names as they are. That is the cost
 * of colouring a type by name, and it is cheaper than a parser. The same
 * bargain in reverse: `id` is a type in Objective-C and a column name in every
 * generated file, so it stays plain and only the unknown-kind column that
 * leans on it goes without a colour.
 */

/** One language's reading. */
export interface Lexis {
  line: string[]
  lineAnnotations?: string[]
  block?: Array<[string, string]>
  quotes: string[]
  /** A word behind one of these is a variable, not a name. */
  sigils?: string[]
  /** `@Name` is a decorator, an annotation or an ObjC directive. */
  at?: boolean
  /** `#` or `-` opens a directive when it stands at the head of a line. */
  lineAttributes?: string[]
  /** A marker that reads as a keyword where it stands: PHP's `<?php`. */
  openers?: string[]
  keywords: Set<string>
  types: Set<string>
}

/** A word list as a set, matched without regard to case. */
const words = (list: string) => new Set(list.split(' '))

const PYTHON: Lexis = {
  line: ['#'],
  quotes: ['"""', "'''", '"', "'"],
  at: true,
  keywords: words(
    'and as assert async await break class continue def del elif else except finally for from ' +
      'global if import in is lambda nonlocal not or pass raise return try while with yield none ' +
      'true false self',
  ),
  types: words(
    'any bool bytes complex date datetime decimal dict float frozenset int iterable list mapping ' +
      'object optional path sequence set str timedelta time tuple union uuid',
  ),
}

const C: Lexis = {
  line: ['//'],
  block: [['/*', '*/']],
  quotes: ['"', "'"],
  lineAttributes: ['#'],
  keywords: words(
    'auto break case const continue default do else enum extern for goto if inline register ' +
      'restrict return sizeof static struct switch typedef union void volatile while',
  ),
  types: words(
    'bool char clock_t double file float int int16_t int32_t int64_t int8_t intptr_t long short ' +
      'size_t ssize_t time_t uint16_t uint32_t uint64_t uint8_t uintptr_t',
  ),
}

const CPP: Lexis = {
  line: ['//'],
  block: [['/*', '*/']],
  quotes: ['"', "'"],
  lineAttributes: ['#'],
  keywords: words(
    'asm break case catch class const constexpr const_cast continue decltype default delete do ' +
      'dynamic_cast else enum explicit export extern false for friend goto if inline mutable ' +
      'namespace new noexcept nullptr operator override private protected public register ' +
      'reinterpret_cast return sizeof static static_assert static_cast struct switch template ' +
      'this throw true try typedef typename union using virtual void volatile while',
  ),
  types: words(
    'any bool char double float int int16_t int32_t int64_t int8_t long map optional pair set ' +
      'shared_ptr short size_t string tuple uint16_t uint32_t uint64_t uint8_t unique_ptr ' +
      'unordered_map unordered_set variant vector weak_ptr wstring',
  ),
}

/** Both Java flavours are read the same way; only what they write differs. */
const JAVA: Lexis = {
  line: ['//'],
  block: [['/*', '*/']],
  quotes: ['"', "'"],
  at: true,
  keywords: words(
    'abstract assert break case catch class const continue default do else enum extends final ' +
      'finally for goto if implements import instanceof interface native new package permits ' +
      'private protected public record return sealed static strictfp super switch synchronized ' +
      'this throw throws transient try var void volatile while true false null yield',
  ),
  types: words(
    'arrays bigdecimal biginteger boolean byte character collection double duration float instant ' +
      'int integer jsonnode list localdate localdatetime localtime long map object offsetdatetime ' +
      'optional set short string uuid zoneddatetime',
  ),
}

const CSHARP: Lexis = {
  line: ['//'],
  block: [['/*', '*/']],
  quotes: ['"', "'"],
  keywords: words(
    'abstract as async await base break case catch checked class const continue default delegate ' +
      'do else event explicit extern false finally fixed for foreach get goto if implicit in init ' +
      'interface internal is lock namespace new null operator out override params private ' +
      'protected public readonly record ref return sealed set sizeof stackalloc static struct ' +
      'switch this throw true try typeof unchecked unsafe using var virtual void volatile when ' +
      'where while yield',
  ),
  types: words(
    'bool byte char dateonly datetime datetimeoffset decimal dictionary double float guid ' +
      'ienumerable ienumerator int jsondocument jsonelement jsonnode list long object sbyte short ' +
      'string timeonly timespan uint ulong ushort',
  ),
}

const JAVASCRIPT: Lexis = {
  line: ['//'],
  block: [['/*', '*/']],
  quotes: ['`', '"', "'"],
  keywords: words(
    'async await break case catch class const constructor continue debugger default delete do ' +
      'else export extends finally for from function get if import in instanceof let new null of ' +
      'return set static super switch this throw true false try typeof undefined var void while ' +
      'with yield',
  ),
  types: words(
    'array arraybuffer bigint boolean date error function int8array json map number object ' +
      'promise regexp set string symbol uint8array weakmap weakset',
  ),
}

const TYPESCRIPT: Lexis = {
  line: ['//'],
  block: [['/*', '*/']],
  quotes: ['`', '"', "'"],
  keywords: words(
    'abstract as async await break case catch class const constructor continue declare default ' +
      'delete do else enum export extends finally for from function get if implements import in ' +
      'instanceof interface is keyof let module namespace new null of readonly return satisfies set ' +
      'static super switch this throw true false try type typeof undefined var void while with yield',
  ),
  types: words(
    'any array arraybuffer bigint boolean date error int8array json map never number object ' +
      'partial pick promise record regexp required set string symbol uint8array unknown void',
  ),
}

const RUST: Lexis = {
  line: ['//'],
  block: [['/*', '*/']],
  quotes: ['"', "'"],
  lineAttributes: ['#[', '#'],
  keywords: words(
    'as async await break const continue crate dyn else enum extern false fn for if impl in let ' +
      'loop match mod move mut pub ref return self static struct super trait true type unsafe use ' +
      'where while',
  ),
  types: words(
    'bool box char datetime duration f32 f64 hashmap i128 i16 i32 i64 i8 isize naivedate ' +
      'naivetime option result set str string u128 u16 u32 u64 u8 usize utc uuid value vec',
  ),
}

const PHP: Lexis = {
  line: ['//', '#'],
  block: [['/*', '*/']],
  quotes: ['"', "'"],
  sigils: ['$'],
  openers: ['<?php', '<?='],
  keywords: words(
    'abstract and as break case catch class clone const continue declare default do echo else ' +
      'elseif empty extends final finally fn for foreach function global goto if implements ' +
      'include include_once instanceof insteadof interface isset list match namespace new or print ' +
      'private protected public readonly require require_once return static switch throw trait try ' +
      'unset use var while xor yield true false null this parent',
  ),
  types: words(
    'array bool callable datetime datetimeimmutable double false float int integer iterable mixed ' +
      'null object self static stdclass string true void',
  ),
}

const GO: Lexis = {
  line: ['//'],
  block: [['/*', '*/']],
  quotes: ['`', '"', "'"],
  keywords: words(
    'break case chan const continue default defer else fallthrough for func go goto if import ' +
      'interface map package range return select struct switch type var nil true false',
  ),
  types: words(
    'any big bool byte complex64 complex128 duration error float32 float64 int int16 int32 int64 ' +
      'int8 json rawmessage rune string time uint uint16 uint32 uint64 uint8 uintptr uuid',
  ),
}

const RUBY: Lexis = {
  line: ['#'],
  quotes: ['"', "'"],
  sigils: ['@', '$'],
  keywords: words(
    'alias and attr_accessor attr_reader attr_writer begin break case class def defined do else ' +
      'elsif end ensure false for if in lambda module new next nil not or private proc protected ' +
      'public raise redo require require_relative rescue retry return self super then true undef ' +
      'unless until when while yield',
  ),
  types: words(
    'array bigdecimal bignum date datetime falseclass fixnum float hash integer nilclass object ' +
      'range string struct symbol time trueclass',
  ),
}

const SWIFT: Lexis = {
  line: ['//'],
  block: [['/*', '*/']],
  quotes: ['"""', '"'],
  at: true,
  keywords: words(
    'associatedtype break case catch class continue default defer deinit do else enum extension ' +
      'fallthrough false fileprivate final for func guard if import indirect init inout internal ' +
      'is lazy let mutating nil open operator override private protocol public repeat required ' +
      'return self static struct subscript super switch throw throws true try typealias unowned ' +
      'var weak where while',
  ),
  types: words(
    'any anyobject array bool cgfloat character codable data date decimal dictionary double float ' +
      'int int16 int32 int64 int8 nsnumber nsstring set string uikit uint uint16 uint32 uint64 ' +
      'uint8 url uuid',
  ),
}

const PERL: Lexis = {
  line: ['#'],
  quotes: ['"', "'"],
  sigils: ['$', '@', '%'],
  keywords: words(
    'and bless cmp continue cpan delete do else elsif eq exists for foreach ge goto grep gt if ' +
      'last le local lt m my ne next no not or our package print q qq qr quit redo ref require ' +
      'return scalar sort strict sub undef unless until use warnings while xor',
  ),
  types: words('array av glob hash hv int iv nv pv ref scalar str sv'),
}

const OBJECTIVEC: Lexis = {
  line: ['//'],
  block: [['/*', '*/']],
  quotes: ['"', "'"],
  at: true,
  lineAttributes: ['#'],
  keywords: words(
    'assign atomic auto break case const continue copy default do else enum extern for goto if ' +
      'inline instancetype nonatomic nonnull readonly readwrite register restrict return sizeof ' +
      'static struct switch typedef union void volatile weak while self super nil yes no nullable ' +
      'strong',
  ),
  types: words(
    'bool cgfloat char double float int int16_t int32_t int64_t int8_t long nsarray nsdata nsdate ' +
      'nsdecimalnumber nsdictionary nsinteger nsmutablearray nsmutabledictionary nsmutablestring ' +
      'nsnumber nsobject nsstring nsuinteger nsuuid nsvalue short signed size_t uint16_t uint32_t ' +
      'uint64_t uint8_t unsigned',
  ),
}

const JULIA: Lexis = {
  line: ['#'],
  quotes: ['"""', '"'],
  at: true,
  keywords: words(
    'abstract baremodule begin break catch const continue do else elseif end export finally for ' +
      'function global if import in isa let local macro module mutable primitive quote return ' +
      'struct try using where while true false nothing missing',
  ),
  types: words(
    'any array base bool char date dates datetime dict float32 float64 int int16 int32 int64 int8 ' +
      'matrix missing nothing set string symbol time tuple uint uint16 uint32 uint64 uint8 union ' +
      'uuid vector',
  ),
}

const KOTLIN: Lexis = {
  line: ['//'],
  block: [['/*', '*/']],
  quotes: ['"""', '"'],
  at: true,
  keywords: words(
    'abstract actual annotation as by catch class companion const constructor continue data do ' +
      'dynamic else enum expect external final finally for fun get if import in infix init inline ' +
      'inner interface internal is lateinit noinline null object open operator out override ' +
      'package private protected public reified return sealed set super suspend tailrec this throw ' +
      'true false try typealias val var vararg when where while',
  ),
  types: words(
    'any array arraylist bigdecimal biginteger boolean byte bytearray char double float int list ' +
      'localdate localdatetime localtime long map mutablelist mutablemap mutableset nothing number ' +
      'set short string uint unit uuid',
  ),
}

const ERLANG: Lexis = {
  line: ['%%', '%'],
  quotes: ['"', "'"],
  lineAttributes: ['-'],
  keywords: words(
    'after andalso begin band bnot bor bsl bsr bxor case catch div end fun if not of orelse ' +
      'receive rem try undefined when xor',
  ),
  types: words(
    'atom binary boolean calendar date datetime dict float integer list map number pid port ' +
      'proplists queue reference sets string term time tuple',
  ),
}

const LUA: Lexis = {
  line: ['--'],
  lineAnnotations: ['---@'],
  quotes: ['"', "'"],
  keywords: words(
    'and break do else elseif end false for function goto if in local nil not or repeat return ' +
      'then true until while self',
  ),
  types: words('any boolean integer number string table thread userdata'),
}

/** One reading per language the picker offers; Java's two entries share one. */
export const LEXIS = new Map<string, Lexis>([
  ['python', PYTHON],
  ['c', C],
  ['cpp', CPP],
  ['java', JAVA],
  ['java_plain', JAVA],
  ['csharp', CSHARP],
  ['javascript', JAVASCRIPT],
  ['typescript', TYPESCRIPT],
  ['rust', RUST],
  ['php', PHP],
  ['go', GO],
  ['ruby', RUBY],
  ['swift', SWIFT],
  ['perl', PERL],
  ['objectivec', OBJECTIVEC],
  ['julia', JULIA],
  ['kotlin', KOTLIN],
  ['erlang', ERLANG],
  ['lua', LUA],
])

/**
 * What an id this table does not know is read as.
 *
 * A language from a later build still gets its comments, its strings and its
 * numbers right; only the words go uncoloured, which is the same quiet failure
 * a word missing from a list has.
 */
export const UNKNOWN_LEXIS: Lexis = {
  line: ['//'],
  block: [['/*', '*/']],
  quotes: ['"', "'"],
  keywords: words(''),
  types: words(''),
}
