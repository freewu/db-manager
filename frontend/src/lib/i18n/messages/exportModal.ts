import type { AreaMessages } from '../types'

// source: frontend/src/components/ExportModal.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const exportModal = {
  'exportModal.export-database-database': [
    'Export database {database}',
    '导出数据库 {database}',
    '匯出資料庫 {database}',
  ],
  'exportModal.export-database': ['Export database', '导出数据库', '匯出資料庫'],
  'exportModal.structure-only': ['Structure only', '仅结构', '僅結構'],
  'exportModal.the-fields-keys-and-indexes': [
    'the fields, the keys and the indexes',
    '字段、主键和索引',
    '欄位、主鍵和索引',
  ],
  'exportModal.structure-and-data': ['Structure and data', '结构和数据', '結構和資料'],
  'exportModal.structure-and-rows': [
    'the same, followed by the rows',
    '同上，后接数据行',
    '同上，後接資料列',
  ],
  'exportModal.data-only': ['Data only', '仅数据', '僅資料'],
  'exportModal.one-tables-rows-only': [
    'one table’s rows, for a table that already exists',
    '单表数据行，用于已存在的表',
    '單一資料表的資料列，用於已存在的資料表',
  ],
  'exportModal.table': ['Table', '表', '資料表'],
  'exportModal.tables': ['Tables', '表', '資料表'],
  'exportModal.select-all': ['Select all', '全选', '全選'],
  'exportModal.select-none': ['Select none', '全不选', '全部取消'],
  'exportModal.search-tables': ['Search tables', '搜索表', '搜尋資料表'],
  'exportModal.reading-the-object-list': ['Reading the object list…', '正在读取对象列表…', '正在讀取物件清單…'],
  'exportModal.no-tables': ['No tables here', '这里没有表', '這裡沒有資料表'],
  'exportModal.skipped-not-tables': [
    '{n} object that is not a table is left out|{n} objects that are not tables are left out',
    '{n} 个非表对象已略过|{n} 个非表对象已略过',
    '{n} 個非資料表物件已略過|{n} 個非資料表物件已略過',
  ],
  'exportModal.fields': ['Fields', '字段', '欄位'],
  'exportModal.fields-picked': [
    '{picked} of {total} field|{picked} of {total} fields',
    '{total} 个字段中的 {picked} 个|{total} 个字段中的 {picked} 个',
    '{total} 個欄位中的 {picked} 個|{total} 個欄位中的 {picked} 個',
  ],
  'exportModal.format': ['Format', '格式', '格式'],
  'exportModal.a-whole-database-is-written-as-sql': [
    'A whole database is written as SQL.',
    '整个数据库写成 SQL。',
    '整個資料庫寫成 SQL。',
  ],
  'exportModal.destination': ['Destination', '目标文件', '目標檔案'],
  'exportModal.choose-file': ['Choose file', '选择文件', '選擇檔案'],
  'exportModal.no-file-chosen-yet': ['No file chosen yet', '尚未选择文件', '尚未選擇檔案'],
  'exportModal.writing-the-file': ['Writing the file…', '正在写入文件…', '正在寫入檔案…'],
  'exportModal.stop': ['Stop', '停止', '停止'],
  'exportModal.cancel': ['Cancel', '取消', '取消'],
  'exportModal.export': ['Export', '导出', '匯出'],
  'exportModal.close': ['Close', '关闭', '關閉'],
  'exportModal.show-in-folder': ['Show in folder', '在文件夹中显示', '在資料夾中顯示'],
  'exportModal.tables-written': [
    '{done} of {total} table|{done} of {total} tables',
    '{total} 个表中的 {done} 个|{total} 个表中的 {done} 个',
    '{total} 個資料表中的 {done} 個|{total} 個資料表中的 {done} 個',
  ],
  'exportModal.rows-written': ['{n} row written|{n} rows written', '{n} 行已写入|{n} 行已写入', '{n} 個資料列已寫入|{n} 個資料列已寫入'],
  'exportModal.n-tables-written': ['{n} table|{n} tables', '{n} 个表|{n} 个表', '{n} 個資料表|{n} 個資料表'],
  'exportModal.n-rows-written': ['{n} row|{n} rows', '{n} 行|{n} 行', '{n} 個資料列|{n} 個資料列'],
  'exportModal.the-export-failed': ['The export failed', '导出失败', '匯出失敗'],
  'exportModal.export-finished': ['Export finished', '导出完成', '匯出完成'],
  'exportModal.export-stopped': ['Export stopped', '导出已停止', '匯出已停止'],
  'exportModal.finished-with-warnings': [
    'Finished, with tables it could not read',
    '已完成，但有些表读不出来',
    '已完成，但有些資料表讀不出來',
  ],
} satisfies AreaMessages
