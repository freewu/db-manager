import type { AreaMessages } from '../types'

// source: frontend/src/components/QueryFavorites.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const queryFavorites = {
  'queryFavorites.give-the-saved-query-a-name': [
    'Give the saved query a name',
    '为已保存的查询命名',
    '為已儲存的查詢命名',
  ],
  'queryFavorites.saved-query-updated': ['Saved query updated', '已保存的查询已更新', '已儲存的查詢已更新'],
  'queryFavorites.saved-to-favourites': ['Saved to favourites', '已保存到收藏', '已儲存至我的最愛'],
  'queryFavorites.remove-from-the-favourites': [
    'Remove “{name}” from the favourites?',
    '从收藏中移除“{name}”？',
    '從我的最愛移除「{name}」？',
  ],
  'queryFavorites.saved-query-deleted': ['Saved query deleted', '已保存的查询已删除', '已儲存的查詢已刪除'],
  'queryFavorites.save-current-sql': ['Save current SQL…', '保存当前 SQL…', '儲存目前 SQL…'],
  'queryFavorites.no-saved-queries-yet': ['No saved queries yet', '暂无已保存的查询', '尚無已儲存的查詢'],
  'queryFavorites.edit': ['Edit', '编辑', '編輯'],
  'queryFavorites.edit-2': ['Edit {name}', '编辑 {name}', '編輯 {name}'],
  'queryFavorites.loaded': ['Loaded “{name}”', '已加载“{name}”', '已載入「{name}」'],
  'queryFavorites.favourites': ['Favourites', '收藏', '我的最愛'],
  'queryFavorites.edit-saved-query': ['Edit saved query', '编辑已保存的查询', '編輯已儲存的查詢'],
  'queryFavorites.save-query-to-favourites': ['Save query to favourites', '将查询保存到收藏', '將查詢儲存至我的最愛'],
  'queryFavorites.save': ['Save', '保存', '儲存'],
  'queryFavorites.name': ['Name', '名称', '名稱'],
  'queryFavorites.e-g-slow-queries': ['e.g. Slow queries', '例如：慢查询', '例如：慢查詢'],
  'queryFavorites.delete-saved-query': ['Delete saved query', '删除已保存的查询', '刪除已儲存的查詢'],
  'queryFavorites.delete': ['Delete', '删除', '刪除'],
  'queryFavorites.delete-2': ['Delete {name}', '删除 {name}', '刪除 {name}'],
} satisfies AreaMessages
