import type { AreaMessages } from '../types'

// source: frontend/src/components/RunSqlFileModal.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const runSqlFile = {
  'runSqlFile.run-sql-file': ['Run SQL file', '运行 SQL 文件', '執行 SQL 檔案'],
  'runSqlFile.run-sql-file-database': [
    'Run SQL file · {database}',
    '运行 SQL 文件 · {database}',
    '執行 SQL 檔案 · {database}',
  ],
  'runSqlFile.choose-a-sql-file': ['Choose a SQL file', '选择 SQL 文件', '選擇 SQL 檔案'],
  'runSqlFile.file': ['File', '文件', '檔案'],
  'runSqlFile.choose-file': ['Choose file', '选择文件', '選擇檔案'],
  'runSqlFile.no-file-chosen-yet': ['No file chosen yet', '尚未选择文件', '尚未選擇檔案'],
  'runSqlFile.reading-the-file': ['Reading the file…', '正在读取文件…', '正在讀取檔案…'],
  'runSqlFile.what-is-in-the-file': ['What is in the file', '文件内容', '檔案內容'],
  'runSqlFile.n-statements': [
    '{n} statement|{n} statements',
    '{n} 条语句|{n} 条语句',
    '{n} 條語句|{n} 條語句',
  ],
  'runSqlFile.n-destructive': [
    '{n} statement can lose schema or data|{n} statements can lose schema or data',
    '{n} 条语句可能丢失结构或数据|{n} 条语句可能丢失结构或数据',
    '{n} 條語句可能遺失結構或資料|{n} 條語句可能遺失結構或資料',
  ],
  'runSqlFile.which-statements': ['Statements {indexes}', '第 {indexes} 条', '第 {indexes} 條'],
  'runSqlFile.n-refused': [
    '{n} statement will be refused on this connection|{n} statements will be refused on this connection',
    '这个连接会拒绝其中的 {n} 条语句|这个连接会拒绝其中的 {n} 条语句',
    '這個連線會拒絕其中的 {n} 條語句|這個連線會拒絕其中的 {n} 條語句',
  ],
  'runSqlFile.a-read-only-connection-refuses-them': [
    'This connection is read-only: a write statement is refused, not run.',
    '这个连接是只读的：写语句会被拒绝，不会执行。',
    '這個連線是唯讀的：寫入語句會被拒絕，不會執行。',
  ],
  'runSqlFile.n-more-not-shown': [
    '{n} more statement is not listed|{n} more statements are not listed',
    '另有 {n} 条语句未列出|另有 {n} 条语句未列出',
    '另有 {n} 條語句未列出|另有 {n} 條語句未列出',
  ],
  'runSqlFile.the-file-is-not-run-in-a-transaction': [
    'The file is not run in a transaction: stopping it does not undo what already ran.',
    '文件不包在事务里：中途停下不会回滚已经执行的语句。',
    '檔案不包在交易裡：中途停下不會回復已經執行的語句。',
  ],
  'runSqlFile.stop-on-error': ['Stop at the first failure', '遇错即停', '遇錯即停'],
  'runSqlFile.the-run-ends-at-the-statement-that-fails': [
    'The run ends at the statement that fails instead of carrying on past it.',
    '遇到失败的语句就停下，不再继续。',
    '遇到失敗的語句就停下，不再繼續。',
  ],
  'runSqlFile.execute': ['Execute', '执行', '執行'],
  'runSqlFile.cancel': ['Cancel', '取消', '取消'],
  'runSqlFile.close': ['Close', '关闭', '關閉'],
  'runSqlFile.stop': ['Stop', '停止', '停止'],
  'runSqlFile.running-the-file': ['Running the file…', '正在执行文件…', '正在執行檔案…'],
  'runSqlFile.n-run': [
    '{n} statement run|{n} statements run',
    '{n} 条已执行|{n} 条已执行',
    '{n} 條已執行|{n} 條已執行',
  ],
  'runSqlFile.n-failed': ['{n} failed', '{n} 条失败', '{n} 條失敗'],
  'runSqlFile.n-rows-changed': [
    '{n} row changed|{n} rows changed',
    '{n} 行已改动|{n} 行已改动',
    '{n} 個資料列已改動|{n} 個資料列已改動',
  ],
  'runSqlFile.n-statements-run': [
    '{n} statement run|{n} statements run',
    '{n} 条语句已执行|{n} 条语句已执行',
    '{n} 條語句已執行|{n} 條語句已執行',
  ],
  'runSqlFile.statement-number': ['statement {index}', '第 {index} 条', '第 {index} 條'],
  'runSqlFile.only-the-first-failures-are-listed': [
    'Only the first failures are listed.',
    '只列出前几条失败。',
    '只列出前幾條失敗。',
  ],
  'runSqlFile.what-already-ran-was-not-undone': [
    'What already ran was not undone.',
    '已经执行的语句没有回滚。',
    '已經執行的語句沒有回復。',
  ],
  'runSqlFile.run-finished': ['File run finished', '文件执行完成', '檔案執行完成'],
  'runSqlFile.finished-with-failures': [
    'Finished with failures',
    '执行完成，但有失败',
    '執行完成，但有失敗',
  ],
  'runSqlFile.run-stopped': ['Run stopped', '已停止执行', '已停止執行'],
  'runSqlFile.stopped-at-a-failed-statement': [
    'Stopped at a failed statement',
    '因语句失败而停止',
    '因語句失敗而停止',
  ],
  'runSqlFile.the-run-failed': ['The run failed', '执行失败', '執行失敗'],
} satisfies AreaMessages
