import type { AreaMessages } from '../types'

// source: frontend/src/lib/about.ts
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const libAbout = {
  'libAbout.build': ['Build', '构建', '建置'],
  'libAbout.platforms': ['Platforms', '平台', '平台'],
  'libAbout.runtime': ['Runtime', '运行时', '執行階段'],
  'libAbout.desktop-and-ui': ['Desktop and UI', '桌面与 UI', '桌面與 UI'],
} satisfies AreaMessages
