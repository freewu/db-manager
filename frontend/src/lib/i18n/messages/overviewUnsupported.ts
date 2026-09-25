import type { AreaMessages } from '../types'

// source: frontend/src/components/overview/UnsupportedOverview.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const overviewUnsupported = {
  'overviewUnsupported.does-not-report-runtime-state-yet': [
    '{driver} does not report runtime state yet',
    '{driver} 尚未支持上报运行时状态',
    '{driver} 尚未支援回報執行階段狀態',
  ],
  'overviewUnsupported.the-connection-is-open-and-usable-queries-the': [
    'The connection is open and usable — queries, the object tree and the designer all work. This page is the only thing this engine cannot fill in.',
    '连接已打开且可用 —— 查询、对象树和设计器都能正常工作。这个引擎唯一无法填充的就是这一页。',
    '連線已開啟且可用 —— 查詢、物件樹和設計器都能正常運作。這個引擎唯一無法填補的就是這一頁。',
  ],
  'overviewUnsupported.session': ['Session', '会话', '工作階段'],
  'overviewUnsupported.connection': ['Connection', '连接', '連線'],
  'overviewUnsupported.engine': ['Engine', '引擎', '引擎'],
  'overviewUnsupported.server-version': ['Server version', '服务端版本', '伺服器版本'],
  'overviewUnsupported.database': ['Database', '数据库', '資料庫'],
  'overviewUnsupported.read-only': ['Read-only', '只读', '唯讀'],
  'overviewUnsupported.yes': ['yes', '是', '是'],
  'overviewUnsupported.no': ['no', '否', '否'],
  'overviewUnsupported.connected': ['Connected', '已连接', '已連線'],
} satisfies AreaMessages
