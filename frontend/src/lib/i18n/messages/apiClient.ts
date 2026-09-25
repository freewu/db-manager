import type { AreaMessages } from '../types'

// source: frontend/src/lib/i18n/messages/apiClient.ts
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const apiClient = {
  'apiClient.unknown-error': ['Unknown error', '未知错误', '未知錯誤'],
  'apiClient.bridge-unavailable': [
    'Backend bridge is unavailable. Start the app with `wails dev` or `wails build` instead of opening the page directly.',
    '后端桥接不可用。请用 `wails dev` 或 `wails build` 启动应用，不要直接打开页面。',
    '後端橋接不可用。請用 `wails dev` 或 `wails build` 啟動應用程式，不要直接開啟頁面。',
  ],
  'apiClient.unknown-method': [
    'Unknown backend method: {method}',
    '未知的后端方法：{method}',
    '未知的後端方法：{method}',
  ],
} satisfies AreaMessages
