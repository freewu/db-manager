import type { AreaMessages } from '../types'

// source: frontend/src/components/DataGenWorkspace.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const dataGenWorkspace = {
  'dataGenWorkspace.data-generation': ['Data generation', '数据生成', '資料產生'],
  'dataGenWorkspace.one-open-connection': ['one open connection', '1 个打开的连接', '1 個開啟的連線'],
  'dataGenWorkspace.open-connections': ['{n} open connections', '{n} 个打开的连接', '{n} 個開啟的連線'],
  'dataGenWorkspace.no-generation-window-is-open': [
    'No generation window is open',
    '没有打开的数据生成窗口',
    '沒有開啟的資料產生視窗',
  ],
  'dataGenWorkspace.right-click-a-table-in-the-explorer-and-pick': [
    'Right-click a table in the explorer and pick “Data generation…”, or press the flask in the page rail.',
    '在资源管理器中右键单击表并选择“数据生成…”，或按页面侧栏上的烧瓶图标。',
    '在檔案總管中對資料表按右鍵並選擇「資料產生…」，或按頁面側欄上的燒瓶圖示。',
  ],
} satisfies AreaMessages
