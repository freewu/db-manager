import type { AreaMessages } from '../types'

// source: frontend/src/components/overview/MongoOverview.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const overviewMongo = {
  'overviewMongo.counters-come-from': [
    'Counters come from {serverStatus} and reset when mongod restarts; the table above reads {dbStats} per database and costs one round trip per database.',
    '计数器来自 {serverStatus}，mongod 重启后重置；上表会按数据库读取 {dbStats}，每个数据库消耗一次往返。',
    '計數器來自 {serverStatus}，mongod 重新啟動後重置；上表會按資料庫讀取 {dbStats}，每個資料庫耗費一次往返。',
  ],
} satisfies AreaMessages
