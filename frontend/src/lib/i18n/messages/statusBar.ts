import type { AreaMessages } from '../types'

// source: frontend/src/components/StatusBar.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const statusBar = {
  'statusBar.write-statements-are-rejected-for-this-session': [
    'Write statements are rejected for this session',
    '本会话拒绝执行写语句',
    '本次工作階段會拒絕執行寫入語句',
  ],
  'statusBar.read-only': ['read-only', '只读', '唯讀'],
  'statusBar.profile-colour': ['Profile colour', '配置颜色', '設定檔色彩'],
  'statusBar.no-active-connection': ['no active connection', '无活动连接', '沒有使用中的連線'],
  'statusBar.tab-count': ['{n} tab|{n} tabs', '{n} 个标签页|{n} 个标签页', '{n} 個分頁|{n} 個分頁'],
  'statusBar.connection-count': ['{n} connected', '已连接 {n} 个', '已連線 {n} 個'],
  'statusBar.backend': [
    'Backend {version} · {goVersion}',
    '后端 {version} · {goVersion}',
    '後端 {version} · {goVersion}',
  ],
  'statusBar.following-the-system-theme-click-to-switch': [
    'Following the system theme ({resolvedTheme}) — click to switch',
    '跟随系统主题（{resolvedTheme}）—— 点击切换',
    '跟隨系統主題（{resolvedTheme}）—— 按一下切換',
  ],
  'statusBar.switch-theme-light-dark-or-follow-the-system': [
    'Switch theme: light, dark, or follow the system',
    '切换主题：浅色、深色或跟随系统',
    '切換主題：淺色、深色或跟隨系統',
  ],
  'statusBar.system-theme-suffix': [
    ' ({resolvedTheme})',
    ' （{resolvedTheme}）',
    ' （{resolvedTheme}）',
  ],
} satisfies AreaMessages
