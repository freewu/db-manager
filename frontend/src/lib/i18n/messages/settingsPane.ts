import type { AreaMessages } from '../types'

// source: frontend/src/components/SettingsPane.tsx
//
// Written by `node scripts/i18n.mjs extract`. The first column is the text the
// code was written with and the other two are its translations; a correction
// belongs here. The marker line above is read back by the script, so it stays.

export const settingsPane = {
  'settingsPane.settings': ['Settings', '设置', '設定'],
  'settingsPane.settings-hint': [
    'Theme, code generation, mock placeholders, the data folder and this build',
    '主题、代码生成、模拟占位符、数据目录以及本次构建',
    '主題、程式碼產生、模擬佔位符、資料目錄以及本次建置',
  ],
  'settingsPane.appearance': ['Appearance', '外观', '外觀'],
  'settingsPane.language': ['Language', '语言', '語言'],
  'settingsPane.code-generation': ['Code generation', '代码生成', '程式碼產生'],
  'settingsPane.mock-placeholders': ['Mock placeholders', '模拟占位符', '模擬佔位符'],
  'settingsPane.data-generation': ['Data generation', '数据生成', '資料產生'],
  'settingsPane.data-folder': ['Data folder', '数据目录', '資料目錄'],
  'settingsPane.about': ['About', '关于', '關於'],
  'settingsPane.display-theme': ['Display theme', '显示主题', '顯示主題'],
  'settingsPane.display-theme-hint': [
    'On System the window follows the operating system and switches the moment it does.',
    '选“跟随系统”时，窗口跟随操作系统，系统一变它马上跟着变。',
    '選「跟隨系統」時，視窗跟隨作業系統，系統一變它馬上跟著變。',
  ],
  'settingsPane.interface-language': ['Interface language', '界面语言', '介面語言'],
  'settingsPane.interface-language-hint': [
    'What the menus, the buttons and the messages are written in. A fresh install starts in English.',
    '菜单、按钮和提示信息所用的语言。全新安装时默认是英文。',
    '選單、按鈕和提示訊息所用的語言。全新安裝時預設是英文。',
  ],
  'settingsPane.default-code-language': ['Default code language', '默认代码语言', '預設程式碼語言'],
  'settingsPane.default-code-language-hint': [
    'What a new code window opens in. Each window can be switched to another language on the spot; this is the one it starts from.',
    '新建代码窗口打开时使用的语言。每个窗口都可以随时切换到其他语言，这里只是起始语言。',
    '新建程式碼視窗開啟時使用的語言。每個視窗都可以隨時切換到其他語言，這裡只是起始語言。',
  ],
  'settingsPane.where-the-data-lives': ['Where the data lives', '数据存放位置', '資料存放位置'],
  'settingsPane.where-the-data-lives-hint': [
    'Connections, query favourites, window state and the password key live here; moving the folder takes them all along.',
    '连接、查询收藏、窗口状态和密码密钥都在这里；移动目录会把它们一起带走。',
    '連線、查詢收藏、視窗狀態和密碼金鑰都在這裡；移動目錄會把它們一起帶走。',
  ],
  'settingsPane.open-folder': ['Open folder', '打开文件夹', '開啟資料夾'],
  'settingsPane.change': ['Change…', '更改…', '變更…'],
  'settingsPane.use-the-default': ['Use the default', '使用默认值', '使用預設值'],
  'settingsPane.the-move-did-not-finish-cleanly': [
    'The move did not finish cleanly',
    '移动没有干净完成',
    '移動沒有乾淨完成',
  ],
  'settingsPane.default': ['default', '默认', '預設'],
  'settingsPane.custom': ['custom', '自定义', '自訂'],
  'settingsPane.empty-folder': [
    'Nothing written yet: this folder gets its first file once you save a connection or change a setting.',
    '还没有写入任何内容：保存一个连接或更改一项设置后，这个文件夹才会有第一个文件。',
    '還沒有寫入任何內容：儲存一個連線或變更一項設定後，這個資料夾才會有第一個檔案。',
  ],
  'settingsPane.what-is-in-it': [
    'What is in it ({totalBytes})',
    '里面有什么（{totalBytes}）',
    '裡面有什麼（{totalBytes}）',
  ],
  'settingsPane.file-count': [
    ' · {n} file| · {n} files',
    ' · {n} 个文件| · {n} 个文件',
    ' · {n} 個檔案| · {n} 個檔案',
  ],
  'settingsPane.move-note': [
    'The new folder is in use from now on, and a restart keeps using it.',
    '从现在起使用新文件夹，重启后也继续使用它。',
    '從現在起使用新資料夾，重新啟動後也繼續使用它。',
  ],
  'settingsPane.moved': ['Moved:', '已移动：', '已移動：'],
  'settingsPane.moved-file-to': [
    'Moved {n} file to {path}|Moved {n} files to {path}',
    '已移动 {n} 个文件到 {path}|已移动 {n} 个文件到 {path}',
    '已移動 {n} 個檔案到 {path}|已移動 {n} 個檔案到 {path}',
  ],
  'settingsPane.left-where-they-were': ['Left where they were', '保留在原处', '保留在原處'],
  'settingsPane.not-files-this-program-writes': [
    '(not files this program writes):',
    '（不是本程序写入的文件）：',
    '（不是本程式寫入的檔案）：',
  ],
  'settingsPane.copied-but-not-deleted-from-the-old-folder': [
    'Copied but not deleted from the old folder',
    '已复制，但未从旧文件夹删除',
    '已複製，但未從舊資料夾刪除',
  ],
  'settingsPane.a-lock-a-read-only-folder': [
    '(a lock, a read-only folder):',
    '（被锁定、文件夹只读）：',
    '（被鎖定、資料夾唯讀）：',
  ],
  'settingsPane.choose-where-db-manager-keeps-its-data': [
    'Choose where DB Manager keeps its data',
    '选择 DB Manager 保存数据的位置',
    '選擇 DB Manager 儲存資料的位置',
  ],
  'settingsPane.the-data-is-back-in-the-default-folder': [
    'The data is back in the default folder',
    '数据已回到默认文件夹',
    '資料已回到預設資料夾',
  ],
  'settingsPane.data-moved-to': ['Data moved to {path}', '数据已移动到 {path}', '資料已移動到 {path}'],
  'settingsPane.file-connections-json': [
    'Connection profiles, passwords sealed',
    '连接配置，密码已加密',
    '連線設定，密碼已加密',
  ],
  'settingsPane.file-queries-json': ['Query favourites', '查询收藏', '查詢收藏'],
  'settingsPane.file-layout-json': [
    'Groups and order of the connection tree',
    '连接树的分组和顺序',
    '連線樹的分組和順序',
  ],
  'settingsPane.file-state-json': ['Window state and preferences', '窗口状态和偏好设置', '視窗狀態和偏好設定'],
  'settingsPane.file-secret-key': [
    'Key that opens the saved passwords — unreadable on another machine',
    '用于解开已保存密码的密钥 —— 换一台机器无法读取',
    '用於解開已儲存密碼的金鑰 —— 換一台機器無法讀取',
  ],
  'settingsPane.file-changelog-jsonl': [
    'What this program has run, one statement per line',
    '本程序执行过的语句，每行一条',
    '本程式執行過的語句，每行一條',
  ],
  'settingsPane.file-changelog-json': [
    'How big a log file may get before it is archived',
    '日志文件多大之后会被归档',
    '日誌檔案多大之後會被封存',
  ],
  'settingsPane.file-mock': [
    'Custom mock placeholders, one file each',
    '自定义模拟占位符，每个占一个文件',
    '自訂模擬佔位符，每個佔一個檔案',
  ],
  'settingsPane.file-query': ['Saved scripts, one file each', '已保存的脚本，每个占一个文件', '已儲存的指令碼，每個佔一個檔案'],
  'settingsPane.archived-change-log': ['Archived change log', '已归档的变更日志', '已封存的變更日誌'],
  'settingsPane.written-by-this-program': ['Written by this program', '由本程序写入', '由本程式寫入'],
  'settingsPane.change-log': ['Change log', '变更日志', '變更日誌'],
  'settingsPane.change-log-hint': [
    'Statements are written to changelog.jsonl until it holds this many, then the file is moved aside whole as <date>-<n>.log and a new one is started. Nothing is ever dropped: the archives keep every statement, and the Change log window lists them.',
    '语句会写入 changelog.jsonl，写满这么多条后，整个文件会被改名为 <date>-<n>.log 挪到一边，再新建一个文件。不会有任何丢弃：归档里保留每一条语句，「变更日志」窗口会列出它们。',
    '語句會寫入 changelog.jsonl，寫滿這麼多條後，整個檔案會被改名為 <date>-<n>.log 挪到一旁，再新建一個檔案。不會有任何丟棄：封存裡保留每一條語句，「變更日誌」視窗會列出它們。',
  ],
  'settingsPane.statements': ['statements', '条语句', '條語句'],
  'settingsPane.save': ['Save', '保存', '儲存'],
  'settingsPane.range': ['{min} – {max}', '{min} – {max}', '{min} – {max}'],
  'settingsPane.by-default': ['{n} by default', '默认 {n}', '預設 {n}'],
  'settingsPane.changed-from': ['changed from {n}', '已从 {n} 更改', '已從 {n} 變更'],
  'settingsPane.rotation-size-saved': [
    'A log file now holds {n} statements before it is archived',
    '日志文件现在会保留 {n} 条语句后才归档',
    '日誌檔案現在會保留 {n} 條語句後才封存',
  ],
  'settingsPane.the-rotation-size-could-not-be-saved': [
    'The rotation size could not be saved',
    '无法保存轮转阈值',
    '無法儲存輪替臨界值',
  ],
  'settingsPane.the-rotation-size-could-not-be-read': [
    'The rotation size could not be read',
    '无法读取轮转阈值',
    '無法讀取輪替臨界值',
  ],
  'settingsPane.rows-per-run': ['Rows per run', '每次运行行数', '每次執行列數'],
  'settingsPane.rows-per-run-hint': [
    'The ceiling of the Rows box in the data generation window, kept in datagen.json in the data directory. It is not a batch size: rows still go in, in small batches, and the run reports what landed as it goes.',
    '数据生成窗口中「行数」输入框的上限，保存在数据目录的 datagen.json 里。它并不是批大小：数据行仍会以小批量写入，运行过程中会实时报告已写入的数量。',
    '資料產生視窗中「列數」輸入框的上限，儲存在資料目錄的 datagen.json 裡。它不是批次大小：資料列仍會以小批次寫入，執行過程中會即時回報已寫入的數量。',
  ],
  'settingsPane.rows': ['rows', '行', '列'],
  'settingsPane.a-run-may-now-write-rows': [
    'A run may now write {n} rows',
    '一次运行现在可以写入 {n} 行',
    '一次執行現在可以寫入 {n} 列',
  ],
  'settingsPane.the-row-limit-could-not-be-saved': [
    'The row limit could not be saved',
    '无法保存行数上限',
    '無法儲存列數上限',
  ],
  'settingsPane.the-row-limit-could-not-be-read': [
    'The row limit could not be read',
    '无法读取行数上限',
    '無法讀取列數上限',
  ],
} satisfies AreaMessages
