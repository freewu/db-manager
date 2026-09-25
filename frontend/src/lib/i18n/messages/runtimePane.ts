import type { AreaMessages } from '../types'

// source: frontend/src/components/RuntimePane.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const runtimePane = {
  'runtimePane.this-session-refuses-statements-that-write': [
    'This session refuses statements that write',
    '该会话拒绝写语句',
    '此工作階段拒絕寫入語句',
  ],
  'runtimePane.read-only': ['read-only', '只读', '唯讀'],
  'runtimePane.edit-connection': ['Edit connection…', '编辑连接…', '編輯連線…'],
  'runtimePane.collected-ago-in-ms': [
    'collected {age} ago in {elapsedMs} ms',
    '{age}前采集，耗时 {elapsedMs} ms',
    '{age}前收集，耗時 {elapsedMs} ms',
  ],
  'runtimePane.collecting': ['collecting…', '采集中…', '收集中…'],
  'runtimePane.collect-a-fresh-snapshot': ['Collect a fresh snapshot', '采集最新快照', '收集最新快照'],
  'runtimePane.could-not-read-the-server-state': [
    'Could not read the server state',
    '无法读取服务器状态',
    '無法讀取伺服器狀態',
  ],
  'runtimePane.retry': ['Retry', '重试', '重試'],
  'runtimePane.part-of-this-page-could-not-be-read': [
    'Part of this page could not be read',
    '该页面有一部分无法读取',
    '此頁面有一部分無法讀取',
  ],
  'runtimePane.parts-of-this-page-could-not-be-read': [
    '{n} parts of this page could not be read',
    '该页面有 {n} 部分无法读取',
    '此頁面有 {n} 個部分無法讀取',
  ],
  'runtimePane.blocks-of-this-page': ['Blocks of this page', '本页区块', '本頁區塊'],
  'runtimePane.session': ['session · {database}', '会话 · {database}', '工作階段 · {database}'],
  'runtimePane.session-2': ['session', '会话', '工作階段'],
  'runtimePane.connected': ['connected', '已连接', '已連線'],
  'runtimePane.snapshot-press-refresh-for-current-numbers': [
    'snapshot · press refresh for current numbers',
    '快照 · 按刷新获取最新数据',
    '快照 · 按重新整理取得最新資料',
  ],
} satisfies AreaMessages
