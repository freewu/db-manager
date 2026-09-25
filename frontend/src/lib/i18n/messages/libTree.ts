import type { AreaMessages } from '../types'

// source: frontend/src/lib/tree.ts
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const libTree = {
  'libTree.tables': ['Tables', '表', '資料表'],
  'libTree.views': ['Views', '视图', '檢視'],
  'libTree.materialized-views': ['Materialized views', '物化视图', '具體化檢視'],
  'libTree.collections': ['Collections', '集合', '集合'],
  'libTree.sequences': ['Sequences', '序列', '序列'],
  'libTree.procedures': ['Procedures', '存储过程', '預存程序'],
  'libTree.table': ['Table', '表', '資料表'],
  'libTree.view': ['View', '视图', '檢視'],
  'libTree.materialized-view': ['Materialized view', '物化视图', '具體化檢視'],
  'libTree.collection': ['Collection', '集合', '集合'],
  'libTree.sequence': ['Sequence', '序列', '序列'],
  'libTree.procedure': ['Procedure', '存储过程', '預存程序'],
} satisfies AreaMessages
