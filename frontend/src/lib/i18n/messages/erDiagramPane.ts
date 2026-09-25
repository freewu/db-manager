import type { AreaMessages } from '../types'

// source: frontend/src/components/ErDiagramPane.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const erDiagramPane = {
  'erDiagramPane.e8e8ea': ['#e8e8ea', '#e8e8ea', '#e8e8ea'],
  'erDiagramPane.1f1f22': ['#1f1f22', '#1f1f22', '#1f1f22'],
  'erDiagramPane.xml-version-1-0-encoding-utf-8': [
    '<?xml version="1.0" encoding="UTF-8"?> {content}',
    '<?xml version="1.0" encoding="UTF-8"?> {content}',
    '<?xml version="1.0" encoding="UTF-8"?> {content}',
  ],
  'erDiagramPane.diagram-exported': ['Diagram exported', '图表已导出', '圖表已匯出'],
  'erDiagramPane.could-not-read-the-schema': ['Could not read the schema', '无法读取结构', '無法讀取結構'],
  'erDiagramPane.retry': ['Retry', '重试', '重試'],
  'erDiagramPane.find-a-table': ['Find a table', '查找表', '尋找資料表'],
  'erDiagramPane.zoom-out': ['Zoom out', '缩小', '縮小'],
  'erDiagramPane.zoom-in': ['Zoom in', '放大', '放大'],
  'erDiagramPane.fit-to-window': ['Fit to window', '适应窗口', '符合視窗'],
  'erDiagramPane.show-the-object-s-columns-in-each-box': [
    'Show the object\'s columns in each box',
    '在每个框中显示对象的字段',
    '在每個框中顯示物件的欄位',
  ],
  'erDiagramPane.grid': ['Grid', '网格', '格線'],
  'erDiagramPane.plain': ['Plain', '纯色', '純色'],
  'erDiagramPane.objects-and-relations': [
    '{objects} · {relations}',
    '{objects} · {relations}',
    '{objects} · {relations}',
  ],
  'erDiagramPane.n-objects': ['{n} object|{n} objects', '{n} 个对象|{n} 个对象', '{n} 個物件|{n} 個物件'],
  'erDiagramPane.n-relations': ['{n} relation|{n} relations', '{n} 个关系|{n} 个关系', '{n} 個關聯|{n} 個關聯'],
  'erDiagramPane.refresh': ['Refresh', '刷新', '重新整理'],
  'erDiagramPane.export-the-diagram-as-svg': [
    'Export the diagram as SVG',
    '将图表导出为 SVG',
    '將圖表匯出為 SVG',
  ],
  'erDiagramPane.only-the-first-300-objects-are-shown': [
    'Only the first 300 objects are shown',
    '仅显示前 300 个对象',
    '僅顯示前 300 個物件',
  ],
  'erDiagramPane.open-the-namespace-in-a-narrower-scope-a-schema': [
    'Open the namespace in a narrower scope (a schema, or a database per schema) to see the rest.',
    '用更小的范围打开该命名空间（某个 schema，或按 schema 拆分的某个数据库）即可看到其余对象。',
    '用更小的範圍開啟該命名空間（某個 schema，或按 schema 拆分的某個資料庫）即可看到其餘物件。',
  ],
  'erDiagramPane.objects-not-read-in-full': [
    '{n} object could not be read in full|{n} objects could not be read in full',
    '{n} 个对象未能完整读取|{n} 个对象未能完整读取',
    '{n} 個物件未能完整讀取|{n} 個物件未能完整讀取',
  ],
  'erDiagramPane.this-namespace-has-no-objects': [
    'This namespace has no objects',
    '该命名空间没有任何对象',
    '該命名空間沒有任何物件',
  ],
  'erDiagramPane.focused': ['focused:', '聚焦：', '聚焦：'],
  'erDiagramPane.click-a-box-to-open-it-drag-to-pan-wheel-to-zoom': [
    'click a box to open it · drag to pan · wheel to zoom',
    '单击方框打开 · 拖动平移 · 滚轮缩放',
    '按一下方框開啟 · 拖曳平移 · 滾輪縮放',
  ],
  'erDiagramPane.external': ['external', '外部', '外部'],
  'erDiagramPane.more-columns': [
    '{n} more column|{n} more columns',
    '另有 {n} 个字段|另有 {n} 个字段',
    '另有 {n} 個欄位|另有 {n} 個欄位',
  ],
} satisfies AreaMessages
