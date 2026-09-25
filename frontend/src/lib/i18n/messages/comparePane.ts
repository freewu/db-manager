import type { AreaMessages } from '../types'

// source: frontend/src/components/ComparePane.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const comparePane = {
  'comparePane.only-left': ['only left', '仅左侧', '僅左側'],
  'comparePane.only-right': ['only right', '仅右侧', '僅右側'],
  'comparePane.differs': ['differs', '有差异', '有差異'],
  'comparePane.same': ['same', '一致', '一致'],
  'comparePane.left-the-reference': ['Left · the reference', '左侧 · 基准', '左側 · 基準'],
  'comparePane.swap-the-two-sides': ['Swap the two sides', '交换两侧', '交換兩側'],
  'comparePane.right-the-one-the-script-changes': [
    'Right · the one the script changes',
    '右侧 · 脚本要修改的一方',
    '右側 · 指令碼要修改的一方',
  ],
  'comparePane.differences': ['Differences', '差异', '差異'],
  'comparePane.differ': ['differ', '有差异', '有差異'],
  'comparePane.compare-the-two-databases-to-see-what-differs': [
    'Compare the two databases to see what differs',
    '对比两个数据库，看看哪里有差异',
    '對比兩個資料庫，看看哪裡有差異',
  ],
  'comparePane.pick-a-connection-database-and-schema-on-each': [
    'Pick a connection, database and schema on each side',
    '在两侧分别选择连接、数据库和 schema',
    '在兩側分別選擇連線、資料庫和 schema',
  ],
  'comparePane.only-differences': ['Only differences', '只看差异', '只看差異'],
  'comparePane.compare': ['Compare', '对比', '對比'],
  'comparePane.generate-script': ['Generate script', '生成脚本', '產生指令碼'],
  'comparePane.the-comparison-could-not-be-run': [
    'The comparison could not be run',
    '无法执行对比',
    '無法執行對比',
  ],
  'comparePane.what-this-comparison-did-not-look-at': [
    'What this comparison did not look at',
    '本次对比没有检查的内容',
    '本次對比沒有檢查的內容',
  ],
  'comparePane.neither-database-has-a-table-to-compare': [
    'Neither database has a table to compare',
    '两个数据库都没有可对比的表',
    '兩個資料庫都沒有可對比的資料表',
  ],
  'comparePane.every-table-is-the-same-on-both-sides': [
    'Every table is the same on both sides',
    '两侧的每个表都一致',
    '兩側的每個資料表都一致',
  ],
  'comparePane.nothing-to-show': ['Nothing to show', '没有内容可显示', '沒有內容可顯示'],
  'comparePane.open-two-connections-to-compare-their-databases': [
    'Open two connections to compare their databases',
    '打开两个连接，对比它们的数据库',
    '開啟兩個連線，對比它們的資料庫',
  ],
  'comparePane.compare-two-databases-of-the-same-engine': [
    'Compare two databases of the same engine',
    '对比同一引擎的两个数据库',
    '對比同一引擎的兩個資料庫',
  ],
  'comparePane.connection': ['Connection', '连接', '連線'],
  'comparePane.database': ['Database', '数据库', '資料庫'],
  'comparePane.schema': ['Schema', 'Schema', 'Schema'],
  'comparePane.only-engine-connections-are-offered': [
    'Only {engine} connections are offered: both sides must be the same engine.',
    '只提供 {engine} 连接：两侧必须是同一引擎。',
    '只提供 {engine} 連線：兩側必須是同一引擎。',
  ],
  'comparePane.read-only-connections-are-not-offered-here-this': [
    'Read-only connections are not offered here: this is the side the script runs against.',
    '这里不提供只读连接：这一侧是脚本执行的对象。',
    '這裡不提供唯讀連線：這一側是指令碼執行的對象。',
  ],
  'comparePane.show-unchanged': ['Show unchanged', '显示未变更项', '顯示未變更項'],
  'comparePane.fields': ['Fields', '字段', '欄位'],
  'comparePane.every-field-is-the-same-on-both-sides': [
    'Every field is the same on both sides.',
    '两侧的每个字段都一致。',
    '兩側的每個欄位都一致。',
  ],
  'comparePane.indexes': ['Indexes', '索引', '索引'],
  'comparePane.neither-side-has-a-secondary-index': [
    'Neither side has a secondary index.',
    '两侧都没有二级索引。',
    '兩側都沒有次要索引。',
  ],
  'comparePane.every-index-is-the-same-on-both-sides': [
    'Every index is the same on both sides.',
    '两侧的每个索引都一致。',
    '兩側的每個索引都一致。',
  ],
  'comparePane.script-copied': ['Script copied', '脚本已复制', '指令碼已複製'],
  'comparePane.the-script-stopped-at-statement': [
    'The script stopped at statement {failedIndex}: {error}',
    '脚本在第 {failedIndex} 条语句处停止：{error}',
    '指令碼在第 {failedIndex} 條語句處停止：{error}',
  ],
  'comparePane.statements-applied-to': [
    '{n} statement applied to {rightLabel}|{n} statements applied to {rightLabel}',
    '已对 {rightLabel} 执行 {n} 条语句|已对 {rightLabel} 执行 {n} 条语句',
    '已對 {rightLabel} 執行 {n} 條語句|已對 {rightLabel} 執行 {n} 條語句',
  ],
  'comparePane.update-script-for': [
    'Update script for {rightLabel}',
    '{rightLabel} 的更新脚本',
    '{rightLabel} 的更新指令碼',
  ],
  'comparePane.cancel': ['Cancel', '取消', '取消'],
  'comparePane.copy': ['Copy', '复制', '複製'],
  'comparePane.run-script': ['Run script', '运行脚本', '執行指令碼'],
  'comparePane.the-script-could-not-be-generated': [
    'The script could not be generated',
    '无法生成脚本',
    '無法產生指令碼',
  ],
  'comparePane.also-drop-tables-that-only-the-right-database': [
    'Also drop tables that only the right database has',
    '同时删除只有右侧数据库才有的表',
    '同時刪除只有右側資料庫才有的資料表',
  ],
  'comparePane.before-this-runs': ['Before this runs', '执行前须知', '執行前須知'],
  'comparePane.nothing-to-do': ['Nothing to do', '无需执行', '無需執行'],
  'comparePane.the-right-database-already-matches-the-left-one': [
    'The right database already matches the left one; no statement would run.',
    '右侧数据库已与左侧一致，不会执行任何语句。',
    '右側資料庫已與左側一致，不會執行任何語句。',
  ],
  'comparePane.runs-against-one-at-a-time': [
    'This runs against {rightLabel}, one statement at a time. There is no transaction around it, so a statement that fails stops the rest and reports how far it got.',
    '这会针对 {rightLabel} 逐条执行。整个过程没有事务包裹，因此某条语句失败就会中止后续语句，并报告已执行到哪一步。',
    '這會針對 {rightLabel} 逐條執行。整個過程沒有交易包裹，因此某條語句失敗就會中止後續語句，並回報已執行到哪一步。',
  ],
  'comparePane.name-read-only': ['{name} · read-only', '{name} · 只读', '{name} · 唯讀'],
} satisfies AreaMessages
