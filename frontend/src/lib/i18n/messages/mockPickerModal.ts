import type { AreaMessages } from '../types'

// source: frontend/src/components/MockPickerModal.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const mockPickerModal = {
  'mockPickerModal.custom': ['Custom', '自定义', '自訂'],
  'mockPickerModal.could-not-be-read-and-is-not-offered': [
    '@{name} could not be read ({broken}) and is not offered.',
    '@{name} 无法读取（{broken}），不予提供。',
    '@{name} 無法讀取（{broken}），不予提供。',
  ],
  'mockPickerModal.placeholder-files-could-not-be-read-and-are-not': [
    '{n} placeholder files could not be read and are not offered.',
    '{n} 个占位符文件无法读取，不予提供。',
    '{n} 個佔位符檔案無法讀取，不予提供。',
  ],
  'mockPickerModal.settings-mock-placeholders-lists-them-so-they': [
    'Settings › Mock placeholders lists them so they can be removed.',
    '设置 › Mock 占位符会列出它们，以便移除。',
    '設定 › Mock 佔位符會列出它們，以便移除。',
  ],
  'mockPickerModal.placeholder-for': [
    'Placeholder for “{field}”',
    '“{field}”的占位符',
    '「{field}」的佔位符',
  ],
  'mockPickerModal.placeholder': ['Placeholder', '占位符', '佔位符'],
  'mockPickerModal.footer-hint': [
    'A mock is a template: text with placeholders in it, so {sample} is a value too. Write {literal} for a literal @.',
    'Mock 就是模板：内含占位符的文本，所以 {sample} 也是一个值。写 {literal} 表示字面量 @。',
    'Mock 就是範本：內含佔位符的文字，所以 {sample} 也是一個值。寫 {literal} 表示字面 @。',
  ],
  'mockPickerModal.user-natural-1-999': [
    'user_@natural(1, 999)',
    'user_@natural(1, 999)',
    'user_@natural(1, 999)',
  ],
  'mockPickerModal.settings-writes-one': [
    'Settings › Mock placeholders writes one — a name for a template such as {sample}.',
    '设置 › Mock 占位符可新建一个 —— 为 {sample} 这类模板取的名字。',
    '設定 › Mock 佔位符可新增一個 —— 為 {sample} 這類範本取的名字。',
  ],
  'mockPickerModal.search-a-placeholder-e-g-email-or': [
    'Search a placeholder, e.g. email or 邮箱',
    '搜索占位符，例如 email 或 邮箱',
    '搜尋佔位符，例如 email 或 邮箱',
  ],
  'mockPickerModal.no-placeholder-matches': ['No placeholder matches', '没有匹配的占位符', '沒有符合的佔位符'],
  'mockPickerModal.placeholder-groups': ['Placeholder groups', '占位符分组', '佔位符群組'],
  'mockPickerModal.no-custom-placeholder-yet': [
    'No custom placeholder yet.',
    '尚无自定义占位符。',
    '尚無自訂佔位符。',
  ],
  'mockPickerModal.so-date-yyyy-natural-1000-9999': [
    'SO@date(yyyy)@natural(1000, 9999)',
    'SO@date(yyyy)@natural(1000, 9999)',
    'SO@date(yyyy)@natural(1000, 9999)',
  ],
} satisfies AreaMessages
