import type { AreaMessages } from '../types'

// source: frontend/src/components/AppShell.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const appShell = {
  'appShell.pick-an-open-connection-in-the-tree-first': [
    'Pick an open connection in the tree first',
    '请先在树中选择一个已打开的连接',
    '請先在樹中選擇一個已開啟的連線',
  ],
  'appShell.n-cannot-insert-rows': [
    '{driver} cannot insert rows',
    '{driver} 无法插入数据行',
    '{driver} 無法插入資料列',
  ],
  'appShell.this-engine': ['This engine', '此引擎', '此引擎'],
  'appShell.hide-the-connection-tree': ['Hide the connection tree', '隐藏连接树', '隱藏連線樹'],
} satisfies AreaMessages
