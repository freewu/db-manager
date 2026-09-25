import type { AreaMessages } from '../types'

// source: frontend/src/connection/PostgresConnect.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const connectionPostgres = {
  'connectionPostgres.the-one-new-tabs-open-on': [
    'PostgreSQL cannot query across databases, so this is the one new tabs open on. Empty connects to “{database}”.',
    'PostgreSQL 无法跨数据库查询，因此新标签页打开的就是这一个。留空则连接到“{database}”。',
    'PostgreSQL 無法跨資料庫查詢，因此新分頁開啟的就是這一個。留空則連線到「{database}」。',
  ],
} satisfies AreaMessages
