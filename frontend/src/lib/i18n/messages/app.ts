import type { AreaMessages } from '../types'

// source: frontend/src/App.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const app = {
  'app.starting': ['Starting DB Manager…', '正在启动 DB Manager…', '正在啟動 DB Manager…'],
  'app.cannot-reach-backend': ['Cannot reach the backend', '无法连接到后端', '無法連線到後端'],
  'app.backend-failed': ['The backend failed to start.', '后端启动失败。', '後端啟動失敗。'],
  'app.not-in-shell': [
    'This page is not running inside the desktop shell yet. Use `just dev` (which starts `wails dev`) instead of opening the Vite URL directly.',
    '此页面尚未在桌面外壳中运行。请使用 `just dev`（它会启动 `wails dev`），不要直接打开 Vite URL。',
    '此頁面尚未在桌面殼層中執行。請使用 `just dev`（它會啟動 `wails dev`），不要直接開啟 Vite URL。',
  ],
} satisfies AreaMessages
