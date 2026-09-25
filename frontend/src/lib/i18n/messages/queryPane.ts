import type { AreaMessages } from '../types'

// source: frontend/src/components/QueryPane.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const queryPane = {
  'queryPane.no-saved-profile-to-keep-a-script-in': [
    'This connection has no saved profile, so there is nowhere to keep a script',
    '该连接没有已保存的配置，无处存放脚本',
    '此連線沒有已儲存的設定，無處存放指令碼',
  ],
  'queryPane.no-database-picked-so-no-folder-to-save-into': [
    'This session has no database picked, so there is no folder to save into',
    '该会话未选择数据库，没有可保存的文件夹',
    '此工作階段未選擇資料庫，沒有可儲存的資料夾',
  ],
  'queryPane.export-insertmany-script': [
    'Export insertMany script',
    '导出 insertMany 脚本',
    '匯出 insertMany 指令碼',
  ],
  'queryPane.export-insert-statements': [
    'Export INSERT statements',
    '导出 INSERT 语句',
    '匯出 INSERT 語句',
  ],
  'queryPane.this-engine': ['This engine', '当前引擎', '目前引擎'],
  'queryPane.saved': ['Saved {name}', '已保存 {name}', '已儲存 {name}'],
  'queryPane.javascript': ['JavaScript', 'JavaScript', 'JavaScript'],
  'queryPane.nothing-to-run': ['Nothing to run', '没有可执行的内容', '沒有可執行的內容'],
  'queryPane.nothing-to-explain': ['Nothing to explain', '没有可执行 Explain 的语句', '沒有可執行 Explain 的語句'],
  'queryPane.nothing-to-format': ['Nothing to format', '没有可格式化的内容', '沒有可格式化的內容'],
  'queryPane.this-engine-s-statements-are-not-sql-so-there-is': [
    'This engine’s statements are not SQL, so there is no SQL formatting for them',
    '该引擎的语句不是 SQL，无法对其进行 SQL 格式化',
    '此引擎的語句不是 SQL，無法對其進行 SQL 格式化',
  ],
  'queryPane.could-not-format': ['Could not format: {error}', '无法格式化：{error}', '無法格式化：{error}'],
  'queryPane.there-is-nothing-to-export': ['There is nothing to export', '没有可导出的内容', '沒有可匯出的內容'],
  'queryPane.export-written': ['Export written', '导出文件已写入', '匯出檔案已寫入'],
  'queryPane.copied-result-as-csv': ['Copied result as CSV', '已复制结果为 CSV', '已複製結果為 CSV'],
  'queryPane.no-statements-yet': ['No statements yet', '暂无语句', '尚無語句'],
  'queryPane.export-csv': ['Export CSV', '导出 CSV', '匯出 CSV'],
  'queryPane.export-json': ['Export JSON', '导出 JSON', '匯出 JSON'],
  'queryPane.run': ['Run', '运行', '執行'],
  'queryPane.ctrl-cmd-shift-enter': [
    'Ctrl/Cmd+Shift+Enter',
    'Ctrl/Cmd+Shift+Enter',
    'Ctrl/Cmd+Shift+Enter',
  ],
  'queryPane.run-selection': ['Run selection', '运行选中内容', '執行選取內容'],
  'queryPane.plan-the-current-statement-without-running-it': [
    'Plan the current statement without running it',
    '只生成当前语句的执行计划，不实际运行',
    '只產生目前語句的執行計畫，不實際執行',
  ],
  'queryPane.has-no-plan-to-read': [
    '{engine} has no plan to read',
    '{engine} 没有可读取的执行计划',
    '{engine} 沒有可讀取的執行計畫',
  ],
  'queryPane.explain': ['Explain', 'Explain', 'Explain'],
  'queryPane.history': ['History', '历史记录', '歷史記錄'],
  'queryPane.this-engine-s-statements-are-not-sql': [
    'This engine’s statements are not SQL',
    '该引擎的语句不是 SQL',
    '此引擎的語句不是 SQL',
  ],
  'queryPane.re-indent-the-selection-or-the-whole-script-ctrl': [
    'Re-indent the selection, or the whole script (Ctrl/Cmd+Shift+F)',
    '重新缩进选中内容或整个脚本（Ctrl/Cmd+Shift+F）',
    '重新縮排選取內容或整個指令碼（Ctrl/Cmd+Shift+F）',
  ],
  'queryPane.format': ['Format', '格式化', '格式化'],
  'queryPane.clear-editor': ['Clear editor', '清空编辑器', '清空編輯器'],
  'queryPane.ctrl-cmd-s': ['Ctrl/Cmd+S', 'Ctrl/Cmd+S', 'Ctrl/Cmd+S'],
  'queryPane.name-this-script-and-save-it-to-the-data-folder': [
    'Name this script and save it to the data folder (Ctrl/Cmd+S)',
    '为脚本命名并保存到数据文件夹（Ctrl/Cmd+S）',
    '為指令碼命名並儲存到資料資料夾（Ctrl/Cmd+S）',
  ],
  'queryPane.save': ['Save *', '保存 *', '儲存 *'],
  'queryPane.save-2': ['Save', '保存', '儲存'],
  'queryPane.max-rows': ['max rows', '最大行数', '最大資料列數'],
  'queryPane.15s-timeout': ['15s timeout', '15s 超时', '15s 逾時'],
  'queryPane.60s-timeout': ['60s timeout', '60s 超时', '60s 逾時'],
  'queryPane.5m-timeout': ['5m timeout', '5m 超时', '5m 逾時'],
  'queryPane.15m-timeout': ['15m timeout', '15m 超时', '15m 逾時'],
  'queryPane.write-statements-are-rejected-on-this-session': [
    'Write statements are rejected on this session',
    '该会话会拒绝写语句',
    '此工作階段會拒絕寫入語句',
  ],
  'queryPane.read-only': ['read-only', '只读', '唯讀'],
  'queryPane.copy-result-as-csv': ['Copy result as CSV', '复制结果为 CSV', '複製結果為 CSV'],
  'queryPane.export': ['Export', '导出', '匯出'],
  'queryPane.could-not-be-read': ['{name} could not be read', '无法读取 {name}', '無法讀取 {name}'],
  'queryPane.statement-n-of-m': [
    'statement {index} of {total}',
    '第 {index} 条语句，共 {total} 条',
    '第 {index} 條語句，共 {total} 條',
  ],
  'queryPane.statement-failed': ['Statement failed', '语句执行失败', '語句執行失敗'],
  'queryPane.results': ['Results', '结果', '結果'],
  'queryPane.plan': ['Plan', '执行计划', '執行計畫'],
  'queryPane.estimated-not-measured': ['estimated, not measured', '估算值，非实测', '估算值，非實測'],
  'queryPane.rows-affected-in': [
    '{n} row affected in {durationMs}|{n} rows affected in {durationMs}',
    '{n} 行受影响，耗时 {durationMs}|{n} 行受影响，耗时 {durationMs}',
    '{n} 個資料列受影響，耗時 {durationMs}|{n} 個資料列受影響，耗時 {durationMs}',
  ],
  'queryPane.result-truncated-at-rows-max-rows': [
    'Result truncated at {toLocaleString} rows (max rows = {maxRows})',
    '结果已在 {toLocaleString} 行处截断（最大行数 = {maxRows}）',
    '結果已在 {toLocaleString} 個資料列處截斷（最大資料列數 = {maxRows}）',
  ],
  'queryPane.running': ['Running…', '运行中…', '執行中…'],
  'queryPane.run-a-statement-to-see-results-here': [
    'Run a statement to see results here.',
    '运行语句后在此查看结果。',
    '執行語句後在此查看結果。',
  ],
  'queryPane.ctrl-cmd-enter-runs-everything-ctrl-cmd-shift': [
    'Ctrl/Cmd+Enter runs everything, Ctrl/Cmd+Shift+Enter runs the selection.',
    'Ctrl/Cmd+Enter 运行全部，Ctrl/Cmd+Shift+Enter 运行选中内容。',
    'Ctrl/Cmd+Enter 執行全部，Ctrl/Cmd+Shift+Enter 執行選取內容。',
  ],
  'queryPane.this-session-is-no-longer-open': [
    'This session is no longer open.',
    '该会话已关闭。',
    '此工作階段已關閉。',
  ],
  'queryPane.reload': ['Reload', '重新加载', '重新載入'],
  'queryPane.rows': ['{n} row|{n} rows', '{n} 行|{n} 行', '{n} 個資料列|{n} 個資料列'],
  'queryPane.plan-steps': ['plan step|plan steps', '执行计划步骤|执行计划步骤', '執行計畫步驟|執行計畫步驟'],
  'queryPane.the-engine-returned-no-plan-steps-for-this': [
    'The engine returned no plan steps for this statement.',
    '该引擎未返回此语句的执行计划步骤。',
    '此引擎未傳回此語句的執行計畫步驟。',
  ],
  'queryPane.press-explain-to-see-how-the-engine-would-run': [
    'Press Explain to see how the engine would run the current statement.',
    '点击 Explain 查看引擎将如何执行当前语句。',
    '點選 Explain 查看引擎將如何執行目前語句。',
  ],
  'queryPane.nothing-is-executed-explaining-a-statement-that': [
    'Nothing is executed. Explaining a statement that writes is safe.',
    '不会执行任何内容。对写语句执行 Explain 是安全的。',
    '不會執行任何內容。對寫入語句執行 Explain 是安全的。',
  ],
  'queryPane.save-this-script': ['Save this script', '保存脚本', '儲存指令碼'],
  'queryPane.query-name': ['Query name', '查询名称', '查詢名稱'],
  'queryPane.saved-as-a-sql-file-in-the-data-folder-where-the': [
    'Saved as a .sql file in the data folder, where the tree\'s Queries folder lists it. A name that is already taken is refused.',
    '保存为数据文件夹中的 .sql 文件，树中的「查询」文件夹会列出它。名称重复会被拒绝。',
    '儲存為資料資料夾中的 .sql 檔案，樹狀目錄中的「查詢」資料夾會列出它。名稱重複會被拒絕。',
  ],
} satisfies AreaMessages
