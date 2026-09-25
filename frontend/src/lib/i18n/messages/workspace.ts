import type { AreaMessages } from '../types'

// source: frontend/src/components/Workspace.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const workspace = {
  'workspace.close-without-saving': [
    'Close “{name}” without saving?',
    '不保存就关闭“{name}”？',
    '不儲存就關閉「{name}」？',
  ],
  'workspace.the-window-has-edits-that-were-never-written-to': [
    'The window has edits that were never written to its file. Closing it drops them; there is no copy of them anywhere else.',
    '该窗口中存在从未写入文件的修改。关闭后会丢失，别处也没有副本。',
    '這個視窗中有從未寫入檔案的修改。關閉後就會遺失，其他地方也沒有副本。',
  ],
  'workspace.close-without-saving-2': ['Close without saving', '不保存并关闭', '不儲存並關閉'],
  'workspace.new-query-tab': ['New query tab', '新建查询标签页', '新增查詢分頁'],
} satisfies AreaMessages
