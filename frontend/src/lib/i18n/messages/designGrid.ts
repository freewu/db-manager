import type { AreaMessages } from '../types'

// source: frontend/src/components/DesignGrid.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const designGrid = {
  'designGrid.drag-to-reorder': ['Drag to reorder', '拖动以重新排序', '拖曳以重新排序'],
  'designGrid.name': ['Name', '名称', '名稱'],
  'designGrid.type': ['Type', '类型', '型別'],
  'designGrid.varchar-255': ['varchar(255)', 'varchar(255)', 'varchar(255)'],
  'designGrid.allow-null-values': ['Allow NULL values', '允许 NULL 值', '允許 NULL 值'],
  'designGrid.null': ['Null', 'Null', 'Null'],
  'designGrid.default': ['Default', '默认值', '預設值'],
  'designGrid.none': ['none', '无', '無'],
  'designGrid.primary-key': ['Primary key', '主键', '主鍵'],
  'designGrid.part-of-the-primary-key': ['Part of the primary key', '主键的一部分', '主鍵的一部分'],
  'designGrid.make-this-field-part-of-the-primary-key': [
    'Make this field part of the primary key',
    '将该字段设为主键的一部分',
    '將此欄位設為主鍵的一部分',
  ],
  'designGrid.auto-increment': ['Auto increment', '自增', '自動遞增'],
  'designGrid.auto': ['Auto', '自增', '自動'],
  'designGrid.comment': ['Comment', '注释', '註解'],
} satisfies AreaMessages
