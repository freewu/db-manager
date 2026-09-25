import type { AreaMessages } from '../types'

// source: frontend/src/components/ChangeLogPane.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const changeLogPane = {
  'changeLogPane.file-size': ['{size}, {statements}', '{size}，{statements}', '{size}，{statements}'],
  'changeLogPane.a-script-that-was-run': ['A script that was run', '运行的脚本', '執行的指令碼'],
  'changeLogPane.the-structure-page': ['The structure page', '结构页', '結構頁'],
  'changeLogPane.the-table-designer': ['The table designer', '表设计器', '資料表設計器'],
  'changeLogPane.the-data-grid': ['The data grid', '数据网格', '資料網格'],
  'changeLogPane.the-duplicate-table-window': ['The duplicate-table window', '复制表窗口', '複製資料表視窗'],
  'changeLogPane.the-object-explorer': ['The object explorer', '对象浏览器', '物件總管'],
  'changeLogPane.the-database-comparison': ['The database comparison', '数据库对比', '資料庫對比'],
  'changeLogPane.change-log': ['Change Log', '变更日志', '變更日誌'],
  'changeLogPane.no-log-yet': ['No log yet', '暂无日志', '尚無日誌'],
  'changeLogPane.rotated-out-on-archived-logs-are-kept-as-they': [
    'Rotated out on {at}. Archived logs are kept as they were and never written to again.',
    '已于 {at} 轮转。归档日志保持原样，不再写入。',
    '已於 {at} 輪替。封存日誌保持原樣，不再寫入。',
  ],
  'changeLogPane.archived': ['archived', '已归档', '已封存'],
  'changeLogPane.nothing-has-been-run-yet': ['Nothing has been run yet', '尚未运行任何内容', '尚未執行任何內容'],
  'changeLogPane.the-newest-of-statements': [
    'the newest {shown} of {total} statements',
    '共 {total} 条语句，最新 {shown} 条',
    '共 {total} 條語句，最新 {shown} 條',
  ],
  'changeLogPane.statement': [
    '{total} statement|{total} statements',
    '{total} 条语句|{total} 条语句',
    '{total} 條語句|{total} 條語句',
  ],
  'changeLogPane.filter-by-table-database-or-statement': [
    'Filter by table, database or statement',
    '按表、数据库或语句筛选',
    '按資料表、資料庫或語句篩選',
  ],
  'changeLogPane.read-the-log-again': ['Read the log again', '重新读取日志', '重新讀取日誌'],
  'changeLogPane.the-change-log-could-not-be-read': [
    'The change log could not be read',
    '无法读取变更日志',
    '無法讀取變更日誌',
  ],
  'changeLogPane.nothing-has-been-run-that-changed-anything': [
    'Nothing has been run that changed anything',
    '尚未运行任何会改变数据的操作',
    '尚未執行任何會變更資料的操作',
  ],
  'changeLogPane.no-entry-matches-the-filter': [
    'No entry matches the filter',
    '没有符合筛选条件的记录',
    '沒有符合篩選條件的記錄',
  ],
  'changeLogPane.nothing-to-show': ['Nothing to show', '没有内容可显示', '沒有內容可顯示'],
  'changeLogPane.statement-copied': ['Statement copied', '语句已复制', '語句已複製'],
  'changeLogPane.time': ['Time', '时间', '時間'],
  'changeLogPane.connection': ['Connection', '连接', '連線'],
  'changeLogPane.database': ['Database', '数据库', '資料庫'],
  'changeLogPane.none': ['none', '无', '無'],
  'changeLogPane.table': ['Table', '表', '資料表'],
  'changeLogPane.run-from': ['Run from', '运行来源', '執行來源'],
  'changeLogPane.no-table-is-recorded-for-this-statement-either': [
    'No table is recorded for this statement: either it is the statement that creates the table — nothing points at it yet, and the name it introduces is in the statement — or the window it was run from was not open on one, as a query window is not.',
    '该语句没有记录任何表：要么它是创建表的语句——此时还没有任何东西指向该表，它引入的名字就写在语句里——要么执行它的窗口当时没有打开在任何表上，比如查询窗口就是如此。',
    '該語句沒有記錄任何資料表：要麼它是建立資料表的語句——此時還沒有任何東西指向該資料表，它引入的名稱就寫在語句裡——要麼執行它的視窗當時沒有開啟在任何資料表上，比如查詢視窗就是如此。',
  ],
  'changeLogPane.the-run-ended-with-this': ['The run ended with this', '本次运行以该错误结束', '本次執行以該錯誤結束'],
  'changeLogPane.executed-statement': ['Executed statement', '已执行的语句', '已執行的語句'],
  'changeLogPane.copy-the-statement': ['Copy the statement', '复制该语句', '複製該語句'],
} satisfies AreaMessages
