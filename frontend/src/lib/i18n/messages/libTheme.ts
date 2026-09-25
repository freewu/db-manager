import type { AreaMessages } from '../types'

// source: frontend/src/lib/theme.ts
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const libTheme = {
  'libTheme.light': ['Light', '浅色', '淺色'],
  'libTheme.light-hint': ['Always the light theme', '始终使用浅色主题', '一律使用淺色主題'],
  'libTheme.dark': ['Dark', '深色', '深色'],
  'libTheme.dark-hint': ['Always the dark theme', '始终使用深色主题', '一律使用深色主題'],
  'libTheme.system': ['System', '跟随系统', '跟隨系統'],
  'libTheme.system-hint': ['Follow the operating system setting', '跟随操作系统设置', '跟隨作業系統設定'],
} satisfies AreaMessages
