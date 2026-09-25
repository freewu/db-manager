import type { AreaMessages } from '../types'

// source: frontend/src/components/ActivityBar.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const activityBar = {
  'activityBar.pages': ['Pages', '页面', '頁面'],
  'activityBar.connections': ['Connections', '连接', '連線'],
  'activityBar.data-generation': ['Data generation', '数据生成', '資料產生'],
  'activityBar.compare': ['Compare', '对比', '比對'],
  'activityBar.change-log': ['Change log', '变更日志', '變更日誌'],
  'activityBar.settings': ['Settings', '设置', '設定'],
} satisfies AreaMessages
