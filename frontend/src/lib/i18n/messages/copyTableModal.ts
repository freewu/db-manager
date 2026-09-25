import type { AreaMessages } from '../types'

// source: frontend/src/components/CopyTableModal.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const copyTableModal = {
  'copyTableModal.copied-statements': [
    'Copied {done} of {total} statement|Copied {done} of {total} statements',
    '已复制 {total} 条语句中的 {done} 条|已复制 {total} 条语句中的 {done} 条',
    '已複製 {total} 個陳述式中的 {done} 個|已複製 {total} 個陳述式中的 {done} 個',
  ],
  'copyTableModal.table-copied-to-rows-included': [
    'Table {object} copied to {name}, rows included',
    '已将表 {object} 复制为 {name}，包含数据行',
    '已將資料表 {object} 複製為 {name}，包含資料列',
  ],
  'copyTableModal.table-copied-to': [
    'Table {object} copied to {name}',
    '已将表 {object} 复制为 {name}',
    '已將資料表 {object} 複製為 {name}',
  ],
  'copyTableModal.duplicate-table': ['Duplicate table {object}', '复制表 {object}', '複製資料表 {object}'],
  'copyTableModal.duplicate-table-2': ['Duplicate table', '复制表', '複製資料表'],
  'copyTableModal.name-of-the-copy': ['Name of the copy', '副本名称', '副本名稱'],
  'copyTableModal.orders-copy': ['orders_copy', 'orders_copy', 'orders_copy'],
  'copyTableModal.structure-only': ['Structure only', '仅结构', '僅結構'],
  'copyTableModal.the-fields-the-key-and-the-indexes': [
    'the fields, the key and the indexes',
    '字段、主键和索引',
    '欄位、主鍵和索引',
  ],
  'copyTableModal.structure-and-data': ['Structure and data', '结构和数据', '結構和資料'],
  'copyTableModal.the-same-and-the-rows-with-it': [
    'the same, and the rows with it',
    '同上，并带上数据行',
    '同上，並帶上資料列',
  ],
  'copyTableModal.this-engine-cannot-copy-everything-as-it-stands': [
    'This engine cannot copy everything as it stands',
    '此引擎无法按原样完整复制',
    '此引擎無法依原樣完整複製',
  ],
  'copyTableModal.the-rows-are-moved-by-the-server-one-statement': [
    'The rows are moved by the server, one statement for all of them.',
    '数据行由服务器搬运，所有行合并为一条语句。',
    '資料列由伺服器搬移，所有列合併為一個陳述式。',
  ],
  'copyTableModal.the-statements-run-one-at-a-time-in-this-order': [
    'The statements run one at a time, in this order.',
    '语句按此顺序逐条执行。',
    '陳述式依此順序逐一執行。',
  ],
  'copyTableModal.name-it-and-the-statement-appears-here': [
    'Name it and the statement appears here.',
    '命名后语句会显示在这里。',
    '命名後陳述式會顯示在這裡。',
  ],
  'copyTableModal.create-copy': ['Create copy', '创建副本', '建立副本'],
} satisfies AreaMessages
