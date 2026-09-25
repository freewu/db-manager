import type { AreaMessages } from '../types'

// source: frontend/src/lib/mock/catalog.ts
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const libMockCatalog = {
  'libMockCatalog.person': ['Person', '人物', '人物'],
  'libMockCatalog.web': ['Web', 'Web', 'Web'],
  'libMockCatalog.basic': ['Basic', '基础', '基礎'],
  'libMockCatalog.time': ['Time', '时间', '時間'],
  'libMockCatalog.character': ['Character', '字符', '字元'],
  'libMockCatalog.number': ['Number', '数字', '數字'],
} satisfies AreaMessages
