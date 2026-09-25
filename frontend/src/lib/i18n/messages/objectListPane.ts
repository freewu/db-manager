import type { AreaMessages } from '../types'

// source: frontend/src/components/ObjectListPane.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const objectListPane = {
  'objectListPane.design': ['Design {kind}', '设计 {kind}', '設計 {kind}'],
  'objectListPane.copied': ['Copied {name}', '已复制 {name}', '已複製 {name}'],
  'objectListPane.clipboard-is-not-available': ['Clipboard is not available', '剪贴板不可用', '剪貼簿無法使用'],
  'objectListPane.open-data': ['Open data', '打开数据', '開啟資料'],
  'objectListPane.generate-code': ['Generate code…', '生成代码…', '產生程式碼…'],
  'objectListPane.data-generation': ['Data generation…', '数据生成…', '資料產生…'],
  'objectListPane.copy-name': ['Copy name', '复制名称', '複製名稱'],
  'objectListPane.name': ['Name', '名称', '名稱'],
  'objectListPane.type': ['Type', '类型', '型別'],
  'objectListPane.rows': ['Rows', '行数', '資料列數'],
  'objectListPane.size': ['Size', '大小', '大小'],
  'objectListPane.engine': ['Engine', '引擎', '引擎'],
  'objectListPane.comment': ['Comment', '注释', '註解'],
  'objectListPane.table': ['Table', '表', '資料表'],
  'objectListPane.columns': ['Columns', '列数', '欄數'],
  'objectListPane.unique': ['Unique', '唯一', '唯一'],
  'objectListPane.not-unique': ['Not unique', '非唯一', '非唯一'],
  'objectListPane.primary': ['Primary', '主键', '主鍵'],
  'objectListPane.method': ['Method', '方法', '方法'],
  'objectListPane.no-namespace-selected': ['No namespace selected', '未选择命名空间', '未選擇命名空間'],
  'objectListPane.reload-this-list': ['Reload this list', '重新加载此列表', '重新載入此清單'],
  'objectListPane.could-not-load': ['Could not load', '加载失败', '載入失敗'],
  'objectListPane.try-again': ['Try again', '重试', '重試'],
  'objectListPane.no-in': ['No {kind} in {path}', '{path} 中没有{kind}', '{path} 中沒有{kind}'],
  'objectListPane.filter': ['Filter', '筛选', '篩選'],
  'objectListPane.of': [
    '{visible} of {total}',
    '{total} 项中的 {visible} 项',
    '{total} 項中的 {visible} 項',
  ],
  'objectListPane.item': ['item', '项', '項'],
  'objectListPane.items': ['items', '项', '項'],
  'objectListPane.click-to-open-double-click-to-design': [
    'Click to open · double-click to design',
    '单击打开 · 双击设计',
    '按一下開啟 · 按兩下設計',
  ],
} satisfies AreaMessages
