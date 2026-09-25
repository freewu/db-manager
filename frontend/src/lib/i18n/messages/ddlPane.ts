import type { AreaMessages } from '../types'

// source: frontend/src/components/DdlPane.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const ddlPane = {
  'ddlPane.nothing-to-run': ['Nothing to run', '没有可执行的语句', '沒有可執行的陳述式'],
  'ddlPane.rows-returned': [
    '{n} row returned|{n} rows returned',
    '返回 {n} 行|返回 {n} 行',
    '傳回 {n} 列|傳回 {n} 列',
  ],
  'ddlPane.rows-affected': [
    '{n} row affected|{n} rows affected',
    '影响 {n} 行|影响 {n} 行',
    '影響 {n} 列|影響 {n} 列',
  ],
  'ddlPane.run-a-script-that-destroys-data-or-schema': [
    'Run a script that destroys data or schema?',
    '要运行会破坏数据或结构的脚本吗？',
    '要執行會破壞資料或結構的指令碼嗎？',
  ],
  'ddlPane.run-anyway': ['Run anyway', '仍然运行', '仍要執行'],
  'ddlPane.ddl-cannot-be-rolled-back-on-every-engine': [
    'DDL cannot be rolled back on every engine.',
    'DDL 并非在所有引擎上都能回滚。',
    'DDL 並非在所有引擎上都能復原。',
  ],
  'ddlPane.copied': ['{what} copied', '已复制 {what}', '已複製 {what}'],
  'ddlPane.nothing-to-save': ['Nothing to save', '没有可保存的内容', '沒有可儲存的內容'],
  'ddlPane.script-saved': ['Script saved', '脚本已保存', '指令碼已儲存'],
  'ddlPane.run-script': ['Run script', '运行脚本', '執行指令碼'],
  'ddlPane.run-the-selected-statements-only': [
    'Run the selected statements only',
    '仅运行选中的语句',
    '僅執行選取的陳述式',
  ],
  'ddlPane.run-selection': ['Run selection', '运行选中项', '執行選取範圍'],
  'ddlPane.replace-the-editor-with-the-object-s-current': [
    'Replace the editor with the object\'s current definition',
    '用对象的当前定义替换编辑器内容',
    '以物件的目前定義取代編輯器內容',
  ],
  'ddlPane.reload': ['Reload', '重新加载', '重新載入'],
  'ddlPane.start-over-from-a-template': ['Start over from a template', '从模板重新开始', '從範本重新開始'],
  'ddlPane.template': ['Template', '模板', '範本'],
  'ddlPane.copy-the-script': ['Copy the script', '复制脚本', '複製指令碼'],
  'ddlPane.script': ['Script', '脚本', '指令碼'],
  'ddlPane.save-the-script-to-a-file': ['Save the script to a file', '将脚本保存到文件', '將指令碼儲存至檔案'],
  'ddlPane.clear-the-editor': ['Clear the editor', '清空编辑器', '清除編輯器'],
  'ddlPane.statements': ['{n} statement|{n} statements', '{n} 条语句|{n} 条语句', '{n} 個陳述式|{n} 個陳述式'],
  'ddlPane.destructive': ['destructive', '破坏性', '破壞性'],
  'ddlPane.write-statements-are-rejected-on-this-session': [
    'Write statements are rejected on this session',
    '本会话拒绝写入语句',
    '本工作階段拒絕寫入陳述式',
  ],
  'ddlPane.read-only': ['read-only', '只读', '唯讀'],
  'ddlPane.could-not-load-the-object-definition': [
    'Could not load the object definition',
    '无法加载对象定义',
    '無法載入物件定義',
  ],
  'ddlPane.retry': ['Retry', '重试', '重試'],
  'ddlPane.will-be-refused-read-only': ['will be refused (read-only)', '将被拒绝（只读）', '將被拒絕（唯讀）'],
  'ddlPane.dry-run': ['Dry run', '试运行', '試執行'],
  'ddlPane.statement-failed': ['Statement failed', '语句执行失败', '陳述式執行失敗'],
  'ddlPane.rows-affected-in': [
    '{n} row affected in {durationMs}|{n} rows affected in {durationMs}',
    '{durationMs} 内影响 {n} 行|{durationMs} 内影响 {n} 行',
    '{durationMs} 內影響 {n} 列|{durationMs} 內影響 {n} 列',
  ],
  'ddlPane.statement-n-of-m': [
    'Statement {index} of {total}',
    '第 {index} 条语句，共 {total} 条',
    '第 {index} 個陳述式，共 {total} 個',
  ],
  'ddlPane.loading-the-object-definition': [
    'Loading the object definition…',
    '正在加载对象定义…',
    '正在載入物件定義…',
  ],
  'ddlPane.write-a-script-the-dry-run-appears-here-before': [
    'Write a script — the dry run appears here before anything runs.',
    '编写脚本 —— 试运行结果会先显示在这里。',
    '撰寫指令碼 —— 試執行結果會先顯示在這裡。',
  ],
  'ddlPane.rows': ['{n} row|{n} rows', '{n} 行|{n} 行', '{n} 列|{n} 列'],
  'ddlPane.new-object': ['new object', '新对象', '新物件'],
} satisfies AreaMessages
