import type { AreaMessages } from '../types'

// source: frontend/src/components/overview/SqliteOverview.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const overviewSqlite = {
  'overviewSqlite.database-file': ['Database file', '数据库文件', '資料庫檔案'],
  'overviewSqlite.no-such-file-on-disk-either-an-in-memory': [
    'No such file on disk: either an in-memory database, or one opened through a path this process cannot see.',
    '磁盘上没有这个文件：要么是内存数据库，要么是通过本进程看不到的路径打开的。',
    '磁碟上沒有這個檔案：可能是記憶體資料庫，或是透過本行程看不到的路徑開啟的。',
  ],
  'overviewSqlite.sqlite-has-no-server-process-every-number-above': [
    'SQLite has no server process: every number above describes this file at the moment the snapshot was taken.',
    'SQLite 没有服务端进程：上面的每个数字描述的都是快照生成那一刻该文件的状态。',
    'SQLite 沒有伺服器端行程：上面的每個數字描述的都是快照產生那一刻該檔案的狀態。',
  ],
} satisfies AreaMessages
