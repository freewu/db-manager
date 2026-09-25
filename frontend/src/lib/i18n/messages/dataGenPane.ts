import type { AreaMessages } from '../types'

// source: frontend/src/components/DataGenPane.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const dataGenPane = {
  'dataGenPane.no-tables': ['No tables', '没有表', '沒有資料表'],
  'dataGenPane.no-schemas': ['No schemas', '没有 schema', '沒有 schema'],
  'dataGenPane.cannot-insert': ['(cannot insert)', '（无法插入）', '（無法插入）'],
  'dataGenPane.no-databases': ['No databases', '没有数据库', '沒有資料庫'],
  'dataGenPane.generate-rows-into': [
    'Generate {n} row into “{object}”?|Generate {n} rows into “{object}”?',
    '生成 {n} 行到“{object}”？|生成 {n} 行到“{object}”？',
    '產生 {n} 列到「{object}」？|產生 {n} 列到「{object}」？',
  ],
  'dataGenPane.n-of-m-columns-are-sent': [
    '{sent} of {total} columns are sent:',
    '已发送 {sent}/{total} 个字段：',
    '已傳送 {sent}/{total} 個欄位：',
  ],
  'dataGenPane.batches-no-undo': [
    'The rows go straight into the table in batches of {batch}. There is no undo — delete them the way you would delete any other row.',
    '数据行按每批 {batch} 条直接写入表。无法撤销 —— 删除它们的方式和删除其他任何数据行一样。',
    '資料列按每批 {batch} 筆直接寫入資料表。無法復原 —— 刪除它們的方式和刪除其他任何資料列一樣。',
  ],
  'dataGenPane.a-row-the-engine-refuses-is-left-out-and-counted': [
    'A row the engine refuses is left out and counted; the run carries on without it.',
    '引擎拒绝的数据行会被跳过并计数；运行会继续。',
    '引擎拒絕的資料列會被略過並計數；執行會繼續。',
  ],
  'dataGenPane.the-run-stops-at-the-first-row-the-engine': [
    'The run stops at the first row the engine refuses and says which one it was.',
    '运行会在引擎拒绝的第一行处停止，并指出是哪一行。',
    '執行會在引擎拒絕的第一列處停止，並指出是哪一列。',
  ],
  'dataGenPane.generate': ['Generate', '生成', '產生'],
  'dataGenPane.field': ['Field', '字段', '欄位'],
  'dataGenPane.key': ['key', '键', '鍵'],
  'dataGenPane.type': ['Type', '类型', '型別'],
  'dataGenPane.mock': ['Mock', 'Mock', 'Mock'],
  'dataGenPane.no-mock': ['(no mock)', '（无 Mock）', '（無 Mock）'],
  'dataGenPane.pick-a-placeholder': ['Pick a placeholder', '选择一个占位符', '選擇一個佔位符'],
  'dataGenPane.description': ['Description', '描述', '描述'],
  'dataGenPane.this-engine-cannot-insert-generated-rows': [
    'This engine cannot insert generated rows',
    '此引擎无法插入生成的数据行',
    '此引擎無法插入產生的資料列',
  ],
  'dataGenPane.a-collection-has-no-column-list-so-there-is': [
    'A collection has no column list, so there is nothing for a batch of generated values to line up with. Generate documents with the query window instead.',
    '集合没有字段列表，生成的一批值无从对应。请改用查询窗口生成文档。',
    '集合沒有欄位清單，產生的一批值無從對應。請改用查詢視窗產生文件。',
  ],
  'dataGenPane.the-driver-for-this-connection-does-not': [
    'The driver for this connection does not implement row inserts.',
    '此连接的驱动未实现数据行插入。',
    '此連線的驅動程式未實作資料列插入。',
  ],
  'dataGenPane.connection-database-tables': [
    'Connection → database → tables',
    '连接 → 数据库 → 表',
    '連線 → 資料庫 → 資料表',
  ],
  'dataGenPane.connection-database-schema-tables': [
    'Connection → database → schema → tables',
    '连接 → 数据库 → schema → 表',
    '連線 → 資料庫 → schema → 資料表',
  ],
  'dataGenPane.no-table-picked': ['No table picked', '未选择表', '未選擇資料表'],
  'dataGenPane.read-only': ['read-only', '只读', '唯讀'],
  'dataGenPane.rows': ['Rows', '数据行', '資料列'],
  'dataGenPane.at-most-rows-per-run': [
    'At most {toLocaleString} rows per run',
    '每次运行最多 {toLocaleString} 行',
    '每次執行最多 {toLocaleString} 列',
  ],
  'dataGenPane.max': ['max', '最大', '最大'],
  'dataGenPane.a-row-the-engine-refuses-is-left-out-and-counted-2': [
    'A row the engine refuses is left out and counted, and the run carries on',
    '引擎拒绝的数据行会被跳过并计数，运行继续',
    '引擎拒絕的資料列會被略過並計數，執行繼續',
  ],
  'dataGenPane.the-run-stops-at-the-first-row-the-engine-2': [
    'The run stops at the first row the engine refuses (tick to carry on instead)',
    '运行会在引擎拒绝的第一行处停止（勾选则改为继续）',
    '執行會在引擎拒絕的第一列處停止（勾選則改為繼續）',
  ],
  'dataGenPane.skip-bad-rows': ['Skip bad rows', '跳过错误数据行', '略過錯誤資料列'],
  'dataGenPane.reload': ['Reload', '重新加载', '重新載入'],
  'dataGenPane.put-every-column-back-to-the-mock-this-table-s': [
    'Put every column back to the mock this table\'s types suggest, and tick them the way they start',
    '把所有字段恢复为该表类型建议的 Mock，并按初始方式勾选',
    '把所有欄位還原為該資料表型別建議的 Mock，並依初始方式勾選',
  ],
  'dataGenPane.reset-mocks': ['Reset mocks', '重置 Mock', '重設 Mock'],
  'dataGenPane.finish-the-batch-in-flight-then-stop': [
    'Finish the batch in flight, then stop',
    '完成当前批次后停止',
    '完成目前批次後停止',
  ],
  'dataGenPane.stop': ['Stop', '停止', '停止'],
  'dataGenPane.insert-the-generated-rows': ['Insert the generated rows', '插入生成的数据行', '插入產生的資料列'],
  'dataGenPane.this-session-is-read-only': ['This session is read-only', '此会话为只读', '此工作階段為唯讀'],
  'dataGenPane.connect-without-the-read-only-flag-to-write-rows': [
    'Connect without the read-only flag to write rows into this table.',
    '以不带只读标志的方式连接，即可向此表写入数据行。',
    '以不帶唯讀旗標的方式連線，即可向此資料表寫入資料列。',
  ],
  'dataGenPane.could-not-be-read': ['{object} could not be read', '{object} 无法读取', '{object} 無法讀取'],
  'dataGenPane.inserted-done-of-total-rows': [
    'Inserted {done} / {total} rows',
    '已插入 {done} / {total} 行',
    '已插入 {done} / {total} 列',
  ],
  'dataGenPane.skipped': [' · {skipped} skipped', ' · 已跳过 {skipped}', ' · 已略過 {skipped}'],
  'dataGenPane.n-rows': ['{n} row|{n} rows', '{n} 行|{n} 行', '{n} 列|{n} 列'],
  'dataGenPane.rows-skipped': [
    ' — {n} row skipped| — {n} rows skipped',
    ' — 跳过 {n} 行| — 跳过 {n} 行',
    ' — 略過 {n} 列| — 略過 {n} 列',
  ],
  'dataGenPane.inserted-in': [
    'Inserted {rows} in {elapsed}{skipped}',
    '已插入 {rows}，用时 {elapsed}{skipped}',
    '已插入 {rows}，耗時 {elapsed}{skipped}',
  ],
  'dataGenPane.stopped-after-in': [
    'Stopped after {rows} in {elapsed}{skipped}',
    '已停止：{rows}，用时 {elapsed}{skipped}',
    '已停止：{rows}，耗時 {elapsed}{skipped}',
  ],
  'dataGenPane.row-was-refused': [
    'Row {row} was refused after {elapsed} — {rows} are in the table',
    '第 {row} 行在 {elapsed} 后被拒绝 —— 表中已有 {rows} 行',
    '第 {row} 列在 {elapsed} 後被拒絕 —— 資料表中已有 {rows} 列',
  ],
  'dataGenPane.insert-failed-after-in': [
    'Insert failed after {rows} in {elapsed}',
    '插入失败：已插入 {rows}，用时 {elapsed}',
    '插入失敗：已插入 {rows}，耗時 {elapsed}',
  ],
  'dataGenPane.this-engine-cannot-insert-rows': [
    'This engine cannot insert rows',
    '此引擎无法插入数据行',
    '此引擎無法插入資料列',
  ],
  'dataGenPane.pick-a-table-on-the-left': ['Pick a table on the left', '请在左侧选择表', '請在左側選擇資料表'],
  'dataGenPane.fix-the-mock-in': [
    'Fix the mock in {names}',
    '请修正 {names} 中的 Mock',
    '請修正 {names} 中的 Mock',
  ],
  'dataGenPane.write-a-mock-for-or-untick': [
    'Write a mock for {names} or untick the field',
    '请为 {names} 编写 Mock，或取消勾选该字段',
    '請為 {names} 撰寫 Mock，或取消勾選該欄位',
  ],
  'dataGenPane.no-field-is-ticked': ['No field is ticked', '未勾选任何字段', '未勾選任何欄位'],
  'dataGenPane.first-refusal': ['first refusal:', '首次拒绝：', '首次拒絕：'],
  'dataGenPane.unknown-placeholder-in': [
    'Unknown placeholder in {names}',
    '{names} 中存在未知占位符',
    '{names} 中存在未知佔位符',
  ],
  'dataGenPane.a-mock-is-mock-js-syntax-an-name-this-app-does': [
    'A mock is mock.js syntax: an @name this app does not implement cannot be rendered, and is never written as text. Pick one from the catalogue, or write @@ for a literal @.',
    'Mock 是 mock.js 语法：本应用未实现的 @name 无法渲染，也不会作为文本写出。请从目录中选择，或写 @@ 表示字面量 @。',
    'Mock 是 mock.js 語法：本應用程式未實作的 @name 無法渲染，也不會作為文字寫出。請從目錄中選擇，或寫 @@ 表示字面 @。',
  ],
  'dataGenPane.pick-a-table-to-fill': ['Pick a table to fill', '选择要填充的表', '選擇要填入的資料表'],
  'dataGenPane.reading-the-table': ['Reading the table…', '正在读取表…', '正在讀取資料表…'],
  'dataGenPane.this-table-has-no-columns': ['This table has no columns', '此表没有字段', '此資料表沒有欄位'],
  'dataGenPane.auto-increment-the-engine-assigns-this-column': [
    'Auto-increment — the engine assigns this column',
    '自增 —— 该字段由引擎赋值',
    '自動遞增 —— 該欄位由引擎賦值',
  ],
  'dataGenPane.not-sent-this-column-is-left-out-of-the-insert': [
    'Not sent — this column is left out of the INSERT',
    '未发送 —— 该字段不出现在 INSERT 中',
    '未傳送 —— 該欄位不會出現在 INSERT 中',
  ],
  'dataGenPane.e-g': ['e.g.', '例如', '例如'],
} satisfies AreaMessages
