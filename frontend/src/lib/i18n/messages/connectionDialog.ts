import type { AreaMessages } from '../types'

// source: frontend/src/components/ConnectionDialog.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const connectionDialog = {
  'connectionDialog.saved': ['Saved “{name}”', '已保存“{name}”', '已儲存「{name}」'],
  'connectionDialog.cancel': ['Cancel', '取消', '取消'],
  'connectionDialog.test-connection': ['Test connection', '测试连接', '測試連線'],
  'connectionDialog.save': ['Save', '保存', '儲存'],
  'connectionDialog.no-form-yet': [
    'No {displayName} form yet',
    '尚无 {displayName} 表单',
    '尚無 {displayName} 表單',
  ],
  'connectionDialog.the-driver-is-registered-but-its-connection-page': [
    'The driver is registered, but its connection page has not been written.',
    '该驱动已注册，但其连接页面尚未实现。',
    '這個驅動程式已註冊，但其連線頁面尚未實作。',
  ],
  'connectionDialog.connection-settings': ['Connection settings', '连接设置', '連線設定'],
  'connectionDialog.connection-succeeded': ['Connection succeeded', '连接成功', '連線成功'],
  'connectionDialog.connection-failed': ['Connection failed', '连接失败', '連線失敗'],
  'connectionDialog.read-only': ['Read only', '只读', '唯讀'],
  'connectionDialog.edit-connection': ['Edit {name}', '编辑 {name}', '編輯 {name}'],
  'connectionDialog.new-connection-for': [
    'New {driver} connection',
    '新建 {driver} 连接',
    '新增 {driver} 連線',
  ],
  'connectionDialog.new-connection': ['New connection', '新建连接', '新增連線'],
  'connectionDialog.tab-basic': ['Basic', '基本', '基本'],
  'connectionDialog.tab-security': ['Security', '安全', '安全性'],
  'connectionDialog.tab-advanced': ['Advanced', '高级', '進階'],
  'connectionDialog.tab-options': ['Options', '选项', '選項'],
} satisfies AreaMessages
