import type { AreaMessages } from '../types'

// source: frontend/src/lib/mock/defaults.ts
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const libMockDefaults = {
  'libMockDefaults.auto-increment': [
    'Auto-increment — the engine assigns this column',
    '自增 —— 该字段由引擎赋值',
    '自動遞增 —— 該欄位由引擎賦值',
  ],
  'libMockDefaults.no-mock-written': [
    'No mock written — write one, or untick the field',
    '未编写 Mock —— 请编写一个，或取消勾选该字段',
    '未撰寫 Mock —— 請撰寫一個，或取消勾選該欄位',
  ],
  'libMockDefaults.literal-value': ['Literal value, used as is', '字面量值，按原样使用', '字面值，依原樣使用'],
} satisfies AreaMessages
