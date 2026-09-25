import type { AreaMessages } from '../types'

// source: frontend/src/store/appStore.ts
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const storeApp = {
  'storeApp.query': ['Query', '查询', '查詢'],
  'storeApp.indexes': ['Indexes', '索引', '索引'],
  'storeApp.new-table': ['New table', '新建表', '新增資料表'],
  'storeApp.ddl': ['{object} DDL', '{object} DDL', '{object} DDL'],
  'storeApp.ddl-script': ['DDL script', 'DDL 脚本', 'DDL 指令碼'],
  'storeApp.code': ['{name} code', '{name} 代码', '{name} 程式碼'],
  'storeApp.data-generation': ['Data generation', '数据生成', '資料產生'],
  'storeApp.er': ['ER · {schema}', 'ER · {schema}', 'ER · {schema}'],
  'storeApp.er-diagram': ['ER diagram', 'ER 图', 'ER 圖'],
  'storeApp.runtime': ['Runtime', '运行时', '執行階段'],
} satisfies AreaMessages
