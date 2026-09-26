import type { AreaMessages } from '../types'

// source: frontend/src/components/AboutProject.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const aboutProject = {
  'aboutProject.lead': [
    'A MySQL / PostgreSQL / SQLite / MongoDB / Apache Doris / TiDB client with data generation, database comparison, code generation and other tools, and a log of every statement it runs. Connections and favourites stay on this machine, in {path}.',
    'MySQL / PostgreSQL / SQLite / MongoDB / Doris / TiDB 客户端，提供数据生成、数据库比对、代码生成等工具，支持操作日志记录和查询。连接和收藏都保存在本机，位于 {path}。',
    'MySQL / PostgreSQL / SQLite / MongoDB / Doris / TiDB 用戶端，提供資料生成、資料庫比對、程式碼產生等工具，支援操作日誌記錄與查詢。連線和收藏都保存在本機，位於 {path}。',
  ],
  'aboutProject.the-app-config-folder': ['the app config folder', '应用配置目录', '應用程式設定目錄'],
  'aboutProject.repository': ['Repository', '仓库', '儲存庫'],
  'aboutProject.releases': ['Releases', '版本发布', '版本發佈'],
  'aboutProject.issues': ['Issues', '问题反馈', '問題回報'],
  'aboutProject.developer': ['Developer', '开发者', '開發者'],
  'aboutProject.developer-on-github': ['{name} on GitHub', '{name} 在 GitHub', '{name} 在 GitHub'],
  'aboutProject.not-available': ['n/a', '不适用', '不適用'],
  'aboutProject.license': ['license', '许可证', '授權條款'],
  'aboutProject.build': ['build', '构建', '建置'],
  'aboutProject.running': ['running', '运行中', '執行中'],
} satisfies AreaMessages
