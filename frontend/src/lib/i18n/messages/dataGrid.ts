import type { AreaMessages } from '../types'

// source: frontend/src/components/DataGrid.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const dataGrid = {
  'dataGrid.true': ['true', 'true', 'true'],
  'dataGrid.false': ['false', 'false', 'false'],
  'dataGrid.double-click-to-edit': ['Double click to edit', '双击编辑', '按兩下編輯'],
  'dataGrid.no-rows': ['No rows', '无数据行', '無資料列'],
  'dataGrid.close-row-detail': ['Close row detail', '关闭行详情', '關閉資料列詳情'],
  'dataGrid.rows': ['rows', '行', '列'],
  'dataGrid.page': ['page', '页', '頁'],
  'dataGrid.page-2': ['/page', '/页', '/頁'],
} satisfies AreaMessages
