import type { AreaMessages } from '../types'

// source: frontend/src/components/MainToolbar.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const mainToolbar = {
  'mainToolbar.pick-a-database-in-the-tree-first': [
    'Pick a database in the tree first',
    '请先在树中选择数据库',
    '請先在樹狀結構中選擇資料庫',
  ],
  'mainToolbar.keeps-its-objects-in-schemas-pick-one-of-s': [
    '{driverLabel} keeps its objects in schemas — pick one of {database}\'s',
    '{driverLabel} 的对象存放在 schema 中 —— 请选择一个 {database} 的 schema',
    '{driverLabel} 的物件存放在 schema 中 —— 請選擇一個 {database} 的 schema',
  ],
  'mainToolbar.has-no-tables': [
    '{driverLabel} has no tables — its objects are in {instead}',
    '{driverLabel} 没有表 —— 它的对象在{instead}中',
    '{driverLabel} 沒有資料表 —— 它的物件在{instead}中',
  ],
  'mainToolbar.has-no-views': [
    '{driverLabel} has no views — its objects are in {instead}',
    '{driverLabel} 没有视图 —— 它的对象在{instead}中',
    '{driverLabel} 沒有檢視 —— 它的物件在{instead}中',
  ],
  'mainToolbar.list-every-table-in': [
    'List every table in {database}',
    '列出 {database} 中的所有表',
    '列出 {database} 中的所有資料表',
  ],
  'mainToolbar.list-every-view-in': [
    'List every view in {database}',
    '列出 {database} 中的所有视图',
    '列出 {database} 中的所有檢視',
  ],
  'mainToolbar.the-explorer': ['the explorer', '资源管理器', '總管'],
  'mainToolbar.connection': ['Connection', '连接', '連線'],
  'mainToolbar.open': ['Open', '打开', '開啟'],
  'mainToolbar.open-with-name': ['Open {name}', '打开 {name}', '開啟 {name}'],
  'mainToolbar.pick-a-connection-in-the-tree-first': [
    'Pick a connection in the tree first',
    '请先在树中选择连接',
    '請先在樹狀結構中選擇連線',
  ],
  'mainToolbar.is-already-open': ['{name} is already open', '{name} 已打开', '{name} 已開啟'],
  'mainToolbar.close': ['Close', '关闭', '關閉'],
  'mainToolbar.close-with-name': ['Close {name}', '关闭 {name}', '關閉 {name}'],
  'mainToolbar.pick-an-open-connection-in-the-tree-first': [
    'Pick an open connection in the tree first',
    '请先在树中选择一个已打开的连接',
    '請先在樹狀結構中選擇一個已開啟的連線',
  ],
  'mainToolbar.new-query': ['New Query', '新建查询', '新增查詢'],
  'mainToolbar.new-query-in': [
    'New query in {database}',
    '在 {database} 中新建查询',
    '在 {database} 中新增查詢',
  ],
  'mainToolbar.refresh': ['Refresh', '刷新', '重新整理'],
  'mainToolbar.reload-the-catalog-of': [
    'Reload the catalog of {name}',
    '重新加载 {name} 的目录',
    '重新載入 {name} 的目錄',
  ],
  'mainToolbar.table': ['Table', '表', '資料表'],
  'mainToolbar.view': ['View', '视图', '檢視'],
  'mainToolbar.backup': ['Backup', '备份', '備份'],
  'mainToolbar.auto-run': ['Auto Run', '自动运行', '自動執行'],
  'mainToolbar.transfer': ['Transfer', '传输', '傳輸'],
  'mainToolbar.data-sync': ['Data Sync', '数据同步', '資料同步'],
  'mainToolbar.this-engine': ['This engine', '该引擎', '此引擎'],
  'mainToolbar.temporarily-unavailable': ['Temporarily unavailable', '暂不可用', '暫時無法使用'],
  'mainToolbar.create-a-new-connection-profile': [
    'Create a new connection profile',
    '新建连接配置',
    '新增連線設定檔',
  ],
} satisfies AreaMessages
