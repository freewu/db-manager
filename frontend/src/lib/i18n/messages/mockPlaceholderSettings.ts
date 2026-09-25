import type { AreaMessages } from '../types'

// source: frontend/src/components/MockPlaceholderSettings.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const mockPlaceholderSettings = {
  'mockPlaceholderSettings.try-it': ['Try it', '试一试', '試一試'],
  'mockPlaceholderSettings.draw-another-set-of-rows-from-a-different-random': [
    'Draw another set of rows from a different random source',
    '从另一个随机源再抽取一组数据行',
    '從另一個隨機來源再抽取一組資料列',
  ],
  'mockPlaceholderSettings.reroll': ['Reroll', '重新生成', '重新產生'],
  'mockPlaceholderSettings.seed': ['seed', '种子', '種子'],
  'mockPlaceholderSettings.edit': ['Edit @{name}', '编辑 @{name}', '編輯 @{name}'],
  'mockPlaceholderSettings.new-custom-placeholder': [
    'New custom placeholder',
    '新建自定义占位符',
    '新增自訂佔位符',
  ],
  'mockPlaceholderSettings.cancel': ['Cancel', '取消', '取消'],
  'mockPlaceholderSettings.save': ['Save', '保存', '儲存'],
  'mockPlaceholderSettings.name': ['Name', '名称', '名稱'],
  'mockPlaceholderSettings.orderno': ['orderNo', 'orderNo', 'orderNo'],
  'mockPlaceholderSettings.a-name-cannot-be-changed-delete-the-placeholder': [
    'A name cannot be changed — delete the placeholder and write a new one',
    '名称不可修改 —— 请删除该占位符后重新写一个。',
    '名稱無法修改 —— 請刪除該佔位符後重新撰寫一個。',
  ],
  'mockPlaceholderSettings.what-templates-write-after-letters-digits-and': [
    'What templates write after @. Letters, digits and _ only.',
    '模板在 @ 之后写入的内容。仅限字母、数字和 _。',
    '範本在 @ 之後寫入的內容。僅限字母、數字和 _。',
  ],
  'mockPlaceholderSettings.template': ['Template', '模板', '範本'],
  'mockPlaceholderSettings.so-date-yyyy-natural-1000-9999': [
    'SO@date(yyyy)@natural(1000, 9999)',
    'SO@date(yyyy)@natural(1000, 9999)',
    'SO@date(yyyy)@natural(1000, 9999)',
  ],
  'mockPlaceholderSettings.any-built-in-placeholder-can-be-used-in-it-and': [
    'Any built-in placeholder can be used in it, and other custom placeholders too.',
    '其中可以使用任何内置占位符，也可以使用其他自定义占位符。',
    '其中可以使用任何內建佔位符，也可以使用其他自訂佔位符。',
  ],
  'mockPlaceholderSettings.description': ['Description', '描述', '描述'],
  'mockPlaceholderSettings.text': ['Order number', '订单号', '訂單編號'],
  'mockPlaceholderSettings.shown-beside-the-placeholder-in-the-picker-and': [
    'Shown beside the placeholder in the picker and in the mock column.',
    '在选择器和 mock 列中显示在占位符旁。',
    '在選擇器和 mock 欄中顯示在佔位符旁。',
  ],
  'mockPlaceholderSettings.delete': ['Delete @{name}?', '删除 @{name}？', '刪除 @{name}？'],
  'mockPlaceholderSettings.mocks-that-use-it-will-report-an-unknown': [
    'Mocks that use it will report an unknown placeholder until they are changed. The file is removed from the data folder.',
    '使用它的 mock 在修改前会报“未知占位符”。该文件会从数据目录中移除。',
    '使用它的 mock 在修改前會回報「未知佔位符」。該檔案會從資料目錄中移除。',
  ],
  'mockPlaceholderSettings.delete-2': ['Delete', '删除', '刪除'],
  'mockPlaceholderSettings.deleted': ['@{name} deleted', '@{name} 已删除', '@{name} 已刪除'],
  'mockPlaceholderSettings.custom-placeholders': ['Custom placeholders', '自定义占位符', '自訂佔位符'],
  'mockPlaceholderSettings.one-file-per-placeholder': [
    'One file per placeholder in the data folder\'s {extension} subfolder, so they travel with the data folder when it moves.',
    '每个占位符对应数据目录 {extension} 子目录中的一个文件，数据目录移动时会一起搬走。',
    '每個佔位符對應資料目錄 {extension} 子目錄中的一個檔案，資料目錄移動時會一起搬移。',
  ],
  'mockPlaceholderSettings.mock': ['.mock', '.mock', '.mock'],
  'mockPlaceholderSettings.refresh': ['Refresh', '刷新', '重新整理'],
  'mockPlaceholderSettings.new': ['New', '新建', '新增'],
  'mockPlaceholderSettings.no-custom-placeholder-yet': [
    'No custom placeholder yet. One is a name for a template: {example} for {syntax}, written as {reference} in a mock.',
    '还没有自定义占位符。一个占位符就是模板的名字：{example} 对应 {syntax}，在 mock 中写作 {reference}。',
    '還沒有自訂佔位符。一個佔位符就是範本的名字：{example} 對應 {syntax}，在 mock 中寫成 {reference}。',
  ],
  'mockPlaceholderSettings.orderno-2': ['@orderNo', '@orderNo', '@orderNo'],
  'mockPlaceholderSettings.unreadable': ['Unreadable', '无法读取', '無法讀取'],
  'mockPlaceholderSettings.the-file-is-listed-so-it-can-be-deleted': [
    '— the file is listed so it can be deleted.',
    '— 该文件仍会列出，以便删除。',
    '— 該檔案仍會列出，以便刪除。',
  ],
  'mockPlaceholderSettings.edit-2': ['Edit', '编辑', '編輯'],
  'mockPlaceholderSettings.placeholder-saved': ['Placeholder saved', '占位符已保存', '佔位符已儲存'],
} satisfies AreaMessages
