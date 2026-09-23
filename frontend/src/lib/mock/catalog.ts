/**
 * The placeholder catalogue the picker shows.
 *
 * The value of every entry is written in mock.js syntax — `@name`, or
 * `@name(args)` — because that is what the column stores either way: a template
 * is literal text with placeholders in it, so `user_@natural(1, 999)@tld` is a
 * perfectly good value. The engine that reads those templates lives in
 * ./engine.ts, and every entry here is one it can render; a name it does not
 * know is refused rather than quietly left in the output, which is where this
 * differs from mock.js (see the module comment there).
 *
 * The descriptions stay in Chinese: they are the catalogue's own labels, and
 * they are shown next to the value in the picker and in the window's
 * "Description" column.
 */

/** One placeholder the picker offers. */
export interface Placeholder {
  /** The text the column gets, as mock.js would write it. */
  value: string
  /** What it produces. */
  desc: string
}

/** A group of placeholders, drawn as one tab of the picker. */
export interface PlaceholderGroup {
  key: string
  label: string
  items: Placeholder[]
}

export const PLACEHOLDER_GROUPS: PlaceholderGroup[] = [
  {
    key: 'person',
    label: 'Person',
    items: [
      { value: '@cname', desc: '中文姓名' },
      { value: '@cfirst', desc: '中文姓' },
      { value: '@clast', desc: '中文名' },
      { value: '@name', desc: '英文姓名' },
      { value: '@first', desc: '英文名' },
      { value: '@last', desc: '英文姓' },
      { value: '@id', desc: '身份证号（带校验位）' },
      { value: '@phone', desc: '手机号' },
      { value: '@province', desc: '省份' },
      { value: '@city', desc: '城市' },
      { value: '@county', desc: '区县' },
      { value: '@address', desc: '完整地址（省市区 + 街道 + 门牌号）' },
      { value: '@zip', desc: '邮政编码' },
      { value: '@company', desc: '公司名称' },
      { value: '@bankcard', desc: '银行卡号（Luhn 校验位）' },
      { value: '@plate', desc: '车牌号' },
    ],
  },
  {
    key: 'web',
    label: 'Web',
    items: [
      { value: '@email', desc: '邮箱地址' },
      { value: '@url', desc: 'URL（含随机协议）' },
      { value: '@domain', desc: '域名' },
      { value: '@ip', desc: 'IPv4 地址' },
      { value: '@tld', desc: '顶级域名' },
      { value: '@protocol', desc: '协议（https、ftp …）' },
      { value: '@ua', desc: 'User-Agent' },
    ],
  },
  {
    key: 'basic',
    label: 'Basic',
    items: [
      { value: '@boolean', desc: '布尔值 true / false' },
      { value: '@guid', desc: 'UUID（v4 形状）' },
      { value: '@character(lower)', desc: '随机字符，可给字符池' },
      { value: '@string(lower, 5, 10)', desc: '随机字符串（池、最小、最大长度）' },
      { value: '@increment(1)', desc: '自增整数，每行加 step（默认 1）' },
      { value: '@isbn', desc: 'ISBN-13 书号（带校验位）' },
    ],
  },
  {
    key: 'time',
    label: 'Time',
    items: [
      { value: '@date(yyyy-MM-dd)', desc: '随机日期，可给格式' },
      { value: '@time(HH:mm:ss)', desc: '随机时间，可给格式' },
      { value: '@datetime(yyyy-MM-dd HH:mm:ss)', desc: '随机日期时间，可给格式' },
      { value: '@now(day)', desc: '当前时间；给单位（year…second）则随机落在该单位内' },
    ],
  },
  {
    key: 'character',
    label: 'Character',
    items: [
      { value: '@word', desc: '随机英文单词' },
      { value: '@sentence(3, 8)', desc: '英文句子（最少、最多词数）' },
      { value: '@cword(2, 5)', desc: '随机汉字串（最少、最多字数）' },
      { value: '@csentence(3, 10)', desc: '中文句子（最少、最多字数）' },
    ],
  },
  {
    key: 'number',
    label: 'Number',
    items: [
      { value: '@integer(1, 100)', desc: '整数，闭区间（默认 0 ~ 1000000）' },
      { value: '@natural(1, 100)', desc: '自然数，即非负整数（默认 0 ~ 1000000）' },
      { value: '@float(0, 100, 2)', desc: '浮点数（最小、最大、小数位，可给第四位）' },
    ],
  },
]

/**
 * What each placeholder produces, by name.
 *
 * Built from the catalogue above so the picker and the window's description
 * column cannot disagree: an entry added there shows up here, and the first
 * entry for a name wins (a name appears once anyway).
 */
export const PLACEHOLDER_NOTES: Map<string, string> = (() => {
  const notes = new Map<string, string>()
  for (const group of PLACEHOLDER_GROUPS) {
    for (const item of group.items) {
      const name = /^@([A-Za-z_][A-Za-z0-9_]*)/.exec(item.value)?.[1]
      if (name && !notes.has(name)) notes.set(name, item.desc)
    }
  }
  return notes
})()
