import type { AreaMessages } from '../types'

// source: frontend/src/components/RowDetail.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const rowDetail = {
  'rowDetail.change': ['Change {name}?', '修改 {name}？', '修改 {name}？'],
  'rowDetail.change-columns': ['Change {n} columns?', '修改 {n} 个字段？', '修改 {n} 個欄位？'],
  'rowDetail.this-is-the-statement-the-engine-will-run': [
    'This is the statement the engine will run.',
    '这是引擎将要执行的语句。',
    '這是引擎將要執行的語句。',
  ],
  'rowDetail.apply': ['Apply', '应用', '套用'],
  'rowDetail.no-row-matched-it-may-have-been-changed-or': [
    'No row matched: it may have been changed or deleted by someone else',
    '没有匹配的数据行：可能已被他人修改或删除',
    '沒有符合的資料列：可能已被他人修改或刪除',
  ],
  'rowDetail.updated-columns': [
    'Updated {n} column|Updated {n} columns',
    '已更新 {n} 个字段|已更新 {n} 个字段',
    '已更新 {n} 個欄位|已更新 {n} 個欄位',
  ],
  'rowDetail.changed': ['changed', '已修改', '已變更'],
  'rowDetail.no-changes': ['No changes', '无修改', '無變更'],
  'rowDetail.columns-changed': [
    '{n} column changed|{n} columns changed',
    '{n} 个字段已修改|{n} 个字段已修改',
    '{n} 個欄位已變更|{n} 個欄位已變更',
  ],
  'rowDetail.reset': ['Reset', '重置', '重設'],
} satisfies AreaMessages
