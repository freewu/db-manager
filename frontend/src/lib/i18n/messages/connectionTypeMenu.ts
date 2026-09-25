import type { AreaMessages } from '../types'

// source: frontend/src/components/ConnectionTypeMenu.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const connectionTypeMenu = {
  'connectionTypeMenu.still-loading-drivers': ['Still loading drivers…', '仍在加载驱动…', '仍在載入驅動程式…'],
  'connectionTypeMenu.planned': ['planned', '计划中', '計畫中'],
  'connectionTypeMenu.no-driver-yet': ['No driver yet.', '尚无驱动。', '尚無驅動程式。'],
} satisfies AreaMessages
