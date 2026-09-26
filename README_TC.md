# db-manager

[English](README.md) · [简体中文](README_CN.md) · **繁體中文**

一套支援六種引擎 —— **MySQL、PostgreSQL、SQLite、MongoDB、TiDB、Apache Doris** —— 的桌面資料庫客戶端，以 Wails v2 + React 打造，在 Windows / macOS / Linux 上都以單一自帶的可執行檔發佈。

它不只是瀏覽器：能匯出整個資料庫、逐條執行 `.sql` 檔案、用 mock.js 範本產生測試資料、比較兩個資料庫並產生同步腳本、設計資料表、依 18 種語言產生程式碼，還會把它被要求執行的每一道寫入語句記進變更日誌 —— 連影響資料列數一起。任何會改資料的操作都先跳確認，而且確認框裡顯示的語句，就是真正送給引擎的那一道。

驅動層不預設關聯式模型：文件型引擎走的是同一套 `Driver / Conn / Dialect` 契約，介面問的是「能力」而不是引擎名稱 —— 所以某個引擎伺候不了的入口（MongoDB、Doris 的資料表設計器，資料產生，Explain）是**不出現**，而不是按了才報錯。Oracle 與 SQL Server 停在 `internal/drivers/planned` 裡。

> 要在這個專案裡動手？先讀 [`AGENTS.md`](AGENTS.md)：每一輪任務都以一次提交與推送作結。

## 介面截圖

六個視窗，從工作區到設定 —— 和[介紹頁](docs/index.html)用的是同一批圖。全部是真實視窗，不是效果圖。

| 工作區 | 連線管理 |
| --- | --- |
| ![工作區](docs/images/main.png) | ![連線管理](docs/images/connections.png) |
| **資料產生** | **資料庫比對** |
| ![資料產生](docs/images/data-generation.png) | ![資料庫比對](docs/images/database-compare.png) |
| **變更日誌** | **設定** |
| ![變更日誌](docs/images/change-log.png) | ![設定](docs/images/settings.png) |

## 下載

每個版本都產出三個**免安裝可執行檔**：前端資源已經編譯進去，那一個檔案就是整個程式。到 [最新發佈](https://github.com/freewu/db-manager/releases/latest) 取自己平臺的那一個（下表 `<version>` 是你下載的版本號，例如 `0.2.0`）：

| 平臺 | 檔案 | 執行方式 |
| --- | --- | --- |
| Windows x64 | `db-manager-<version>-windows-amd64.exe` | 雙擊即可。需要 WebView2 執行階段（Windows 10/11 內建） |
| macOS（Intel + Apple Silicon 通用） | `db-manager-<version>-macos-universal.zip` | 解壓後把 `db-manager.app` 拖進「應用程式」 |
| Linux x64 | `db-manager-<version>-linux-amd64` | `chmod +x db-manager-<version>-linux-amd64 && ./db-manager-<version>-linux-amd64` |

macOS 版沒有簽章與公證，第一次開啟會被 Gatekeeper 攔下：按右鍵「打開」，或執行 `xattr -cr /Applications/db-manager.app`。Linux 需要 GTK3 與 WebKitGTK 4.0（Debian/Ubuntu 上是 `libgtk-3-0 libwebkit2gtk-4.0-37`）。下載旁邊還有一份 `checksums.txt`，可以 `sha256sum -c checksums.txt` 驗證。

## 功能

- **連線管理** —— 新增、編輯、測試、顏色標記、唯讀鎖定；TLS（CA / 憑證 / 私鑰）與自訂 DSN 參數；每種驅動一張表單，檔案型連線不會出現主機與連接埠。儲存密碼是選項，而且加密存放。
- **物件瀏覽器** —— 延遲載入的工作階段 → 資料庫 → schema → 資料表 / 檢視表 / 索引樹，空資料夾也照樣畫出來（`Tables (0)`），有哪些資料夾由引擎決定。按右鍵可以開啟資料、設計資料表、複製名稱、複製 / 清空 / 刪除資料表、執行 SQL 檔案、匯出資料庫、產生程式碼。
- **資料表格** —— 分頁、伺服器端排序與篩選（14 個運算子）、可拖曳欄寬、多選、長文字滑鼠停留預覽，點一下資料列會滑出該列詳情（欄位、主鍵、NULL、複製為 JSON 或 INSERT，以及就地編輯）。
- **資料列編輯** —— 儲存格就地編輯與批次刪除，語句一律用主鍵產生並走參數綁定，執行前把算好的語句攤給你看。
- **資料表設計器** —— 改欄位名稱、型別、可為空、預設值、主鍵、自動遞增、註解，拖曳握把調整資料列順序（列序就是欄序）。儲存時把將要逐條執行的語句原樣列出；新增資料表是同一條路徑，只是起點是空的。
- **SQL 編輯器** —— CodeMirror 6、依驅動切換方言、多語句執行、歷史記錄、只補齊本視窗所屬命名空間下的資料表與檢視表，`Ctrl/Cmd+Enter` 執行，`Ctrl/Cmd+Shift+F` 格式化。
- **執行計畫** —— Explain 問的是引擎「打算怎麼做」，包裝層絕不加 `ANALYZE`，所以它不執行任何語句；唯讀連線也能先看清楚一道 `DELETE` 再決定要不要跑。
- **結構與 DDL** —— 欄位、索引、外部鍵，以及引擎原生的 DDL，帶語法高亮、可複製。文件型引擎給的是取樣欄位表，因為集合本來就沒有結構。
- **ER 圖** —— 把一個命名空間畫成圖：有關聯的資料表排在同一列，外部鍵層級由左到右，可拖曳、縮放、搜尋、隱藏欄位，並匯出 SVG。
- **資料庫比較** —— 同一引擎的兩個資料庫並排比，逐一資料表給出「僅在左 / 僅在右 / 有差異 / 一致」以及差異欄位與索引。**產生命令稿**給的就是這份差異本身（先建立、再修改、後刪除），唯一要你決定的是「只在右邊存在的資料表要不要刪掉」。
- **資料產生** —— 每個欄位一個 mock.js 範本，佔位符選擇器裡有 40 多個內建項目，也能自訂，並依欄位名稱與型別給建議。資料以每批 200 列寫入，帶進度、可停止，最後給一句「實際寫入幾列、花了多久」。
- **資料庫匯出** —— 在資料庫節點上選「結構 / 結構+資料 / 僅資料」：整庫匯出成一份 `.sql` 腳本（引擎原生的 `CREATE` 加 `INSERT`），或把單一資料表匯出成 CSV、JSON、JSONL、SQL 並自選欄位。讀寫全在後端完成、直接寫入磁碟，帶進度與停止。
- **執行 SQL 檔案** —— 把磁碟上的一個 `.sql` 檔交給後端，它逐條執行，因此每一道寫入語句都會單獨記下影響資料列數。執行前先告訴你檔案裡有什麼（破壞性語句標紅、唯讀連線會拒絕哪些、`USE` 與認不得的語句列為警告），然後才是進度、停止，以及一份會點出「第幾條失敗」的總結。整個檔案**不在交易裡**，介面上也是這樣寫的。
- **程式碼產生** —— 把一張資料表的欄位產生 18 種語言的類別或結構（Java 分帶 Lombok 與不帶兩種），可為空依語言各自處理，命名依慣例，帶高亮，可複製或儲存。
- **變更日誌** —— 本程式被要求執行過的每一道寫入語句，含時間、連線、資料庫 / schema、資料表、來源視窗與引擎回報的影響資料列數。每天一個檔案放在 `<資料目錄>/log/`，超過上限時整檔輪替而不是截斷，不連線也能看。
- **新增資料庫** —— 伺服器需要什麼就問什麼（字元集與排序規則、編碼與 locale），語句先給你看再執行。
- **總覽** —— 雙擊連線看它的伺服器端即時資料：處理程序清單與 InnoDB 命中率、後端與提交速率，或 SQLite 檔案的 pragma 檢視。讀不到的指標顯示 `—` 並附一則警告，而不是顯示 0。
- **設定** —— 一頁管住主題（淺色 / 深色 / 跟隨系統）、介面語言、程式碼產生語言、mock 佔位符、資料目錄（開啟、搬移、設定單一日誌檔記幾筆）以及專案資訊。改完立即生效。
- **介面語言** —— English / 简体中文 / 繁體中文，預設英文；狀態列與 Windows 系統匣選單切的是同一份偏好。
- **系統匣（Windows）** —— 關視窗只是隱藏；選單可以把視窗叫回來、切換顯示主題與介面語言、開啟專案首頁、結束。

## 支援的引擎

| 引擎 | 預設連接埠 | 說明 |
| --- | --- | --- |
| MySQL | 3306 | TLS、依字元集與排序規則建立資料庫、完整的資料表設計器 |
| PostgreSQL | 5432 | 有 schema 層、認 `serial` / identity、編碼與 locale |
| SQLite | 檔案 | ATTACH 別名、pragma 概覽、沒有 `TRUNCATE`（用 `DELETE`） |
| MongoDB | 27017 | 副本集、`mongodb+srv`、TLS、mongosh 風格命令列、無結構 |
| TiDB | 4000 | MySQL 線協定、TLS、叢集成員表 |
| Apache Doris | 9030 | MySQL 線協定、唯讀瀏覽、改結構走 DDL 編輯器 |

MySQL、TiDB、Doris 共用同一個線協定套件，因此也共用連線、瀏覽、表格、查詢、匯出這些程式碼。接進新引擎就是實作 `Driver` / `Conn` / `Dialect` 並在 `init()` 裡註冊。

## 環境需求

| 工具 | 版本 |
| --- | --- |
| Go | 1.24+（開發機上是 1.26.5） |
| Node.js | 20+（開發機上是 24.10.0） |
| Wails CLI | v2.16.0 |
| just | 1.58.0（選用，只是讓指令短一點） |
| WebView2 | Windows 10/11 內建；更舊的系統需要安裝執行階段 |

## 從原始碼建置

```sh
just install     # npm install + go mod download
just dev         # 開發模式：Vite 熱更新 + Go 熱重載
just build       # 正式建置，產物在 build/bin/db-manager.exe
just release     # 建置 + 封存到 release/ 並產生檢查碼（不碰 git）
just publish 0.2.0 "這次改了什麼"   # 同步版本號、提交、打標籤、推送
just --list      # 看所有配方
```

不用 `just` 的話：

```sh
cd frontend && npm install && npm run build && cd ..
wails build
```

`wails build` 產出的是桌面程式，所以在 Windows 上要用 **Windows 工具鏈**（Windows 終端機或 `just.exe`）執行，別在 WSL 裡跑。

## 資料放在哪

連線設定、已儲存的查詢、連線順序、視窗狀態、密碼金鑰與變更日誌都放在同一個資料目錄裡：Windows 是 `%APPDATA%\db-manager`，macOS 是 `~/Library/Application Support/db-manager`，Linux 是 `~/.config/db-manager`。它的位置記在 `location.json` 裡，設定頁的「資料目錄」負責搬移：複製 → 逐位元組驗證 → 改指標 → 才刪舊檔案。

密碼用 AES-256-GCM 加密，金鑰在第一次執行時產生（`secret.key`，權限 0600）。不往任何地方送資料：沒有追蹤，程式只連你自己設定的那些資料庫。

## 程式碼結構

Wails v2 把 React 19 + Vite 的前端裝進 WebView2 / WebKit 視窗，`app.go` 是前端呼叫的綁定層。前端是照著手寫的 `frontend/src/api/{types,client}.ts` 寫的（不用產生的綁定），狀態集中在一個 zustand store 裡。介面文案放在 `frontend/src/lib/i18n/messages/<area>.ts`，每筆形如 `[English, 简体, 繁體]`，由 `scripts/i18n.mjs verify` 把關；後端 Go 的文案目前還是英文。

```
app.go, main.go          Wails 綁定、視窗、embed 進去的前端 frontend/dist
tray*.go                 Windows 系統匣選單（主題 + 語言）及其空實作樁
internal/drivers/        Driver / Conn / Dialect 契約、sqlbase、每個引擎一個套件
internal/service/        manager、設計器、匯出、比較、變更日誌、資料產生
internal/config/         JSON 儲存、資料目錄指標、變更日誌檔案
frontend/src/            React 介面：components、lib、i18n、store、styles
docs/                    介紹頁（靜態，沒有建置步驟）
scripts/                 version.mjs、package.mjs、i18n.mjs、docs-check.mjs、
                         readme-check.mjs、pages-build.sh、release-notes.sh
```

動手前值得知道的四條約定：

- **資料表設計器送的是「完整目標定義」，不是 diff。** 後端拿即時 catalog 結構去 plan，所以預覽與儲存走的是同一條程式碼；引擎表達不了的變更寫成 `Plan.Warnings`，而不是靜默略過。
- **每次寫入都先問、後記。** 確認框裡顯示的語句就是真正送出去的那一道，變更日誌記的是引擎回報的影響資料列數。
- **不在自己承擔不了的交易裡跑。** DDL 本來就不可回滾，執行 SQL 檔案那條路徑會直說，而不是假裝安全。
- **品牌素材只有一個來源** `asserts/`；`frontend/public/logo.png` 與 `build/appicon.png` 都是 `just icons` 產生的副本。

## 測試

```sh
just test                        # go test ./...
npm --prefix frontend run build  # tsc --noEmit + vite build
node scripts/i18n.mjs verify     # 譯文與英文原文對得上
node scripts/docs-check.mjs      # 介紹頁的文案、截圖與相對路徑
node scripts/readme-check.mjs    # 三份 README：結構一致、連結、截圖與下載檔名一致
bash scripts/pages-build.sh _site          # 拼出 GitHub Pages 會發佈的那份副本
node scripts/docs-check.mjs --site _site # …再檢查這份副本裡的引用沒跑出網站
```

Go 測試不需要 cgo，也不需要起任何服務容器 —— SQLite 是純 Go 的。MongoDB、TiDB、Doris 的整合測試在不給它伺服器位址時會自己略過：

```sh
DMB_TEST_MONGODB_HOST=127.0.0.1 go test ./internal/drivers/mongodb/ -run Integration -v
DMB_TEST_TIDB_HOST=127.0.0.1 go test ./internal/drivers/tidb/ -run Integration -v
DMB_TEST_DORIS_HOST=127.0.0.1 go test ./internal/drivers/doris/ -run Integration -v
```

## 發佈

推一個 `v*` 標籤會觸發 [`.github/workflows/release.yml`](.github/workflows/release.yml)：先驗證版本號各處是否一致、標籤與 `wails.json` 是否相符、品牌素材有沒有走樣，然後在 Windows、macOS、Linux 三個 runner 上各自建置，發佈三個執行檔加一份 `checksums.txt`。

Release 說明由 [`scripts/release-notes.sh`](scripts/release-notes.sh) 拼出來：附註標籤的 message 加上兩個標籤之間的提交（依 Conventional Commits 分組）—— 所以每一筆提交的標題都得寫成使用者看得懂的一句話。

```sh
just notes v0.2.0   # 預覽某個標籤的 release message
```

`just release` 只在本機做事：編譯目前平臺並封存到 `release/`，不碰 git。

## 介紹頁

[`docs/index.html`](docs/index.html) 是一張靜態介紹頁：一個 HTML、一份樣式表、一支腳本，再加上裝著同樣三種語言的 `i18n.js`。沒有建置步驟，雙擊就能開啟；`docs/images/` 裡那六張截圖同時供輪播與相簿使用。`scripts/docs-check.mjs` 盯著三份字典、截圖引用與相對路徑別走偏，CI 裡也會跑。

同一張頁面也作為專案首頁發佈：<https://freewu.github.io/db-manager/>，由 [`.github/workflows/pages.yml`](.github/workflows/pages.yml) 負責。專案網站掛在子路徑下，頁面裡那些 `../asserts/…`、`../wails.json` 引用會爬出網站根目錄，所以 `scripts/pages-build.sh` 會拼出一份發佈副本，把品牌素材與版本清單放到頁面旁邊；上傳前由 `node scripts/docs-check.mjs --site _site` 檢查這份副本。CI 裡也會拼一遍，讓「引用跑到網站外面」這種事在 PR 階段就被發現，而不是等上線之後。

只需啟用一次：**Settings → Pages → Build and deployment → Source 選 GitHub Actions**。

## 發展藍圖

- [x] MySQL / PostgreSQL / SQLite：瀏覽、編輯、查詢、結構、匯出
- [x] 資料表設計器、已儲存的查詢、DDL 編輯器、執行計畫與格式化、ER 圖、新增資料庫、總覽、變更日誌、資料產生、程式碼產生
- [x] MongoDB，以及後來的 TiDB 與 Apache Doris
- [x] 資料庫比較與同步腳本；每次寫入前確認；逐條記錄影響資料列數
- [x] 資料庫匯出；執行 SQL 檔案
- [x] 三種介面語言；Windows 系統匣切換主題與語言
- [ ] Oracle / SQL Server（方言樁已經就位）
- [ ] SSH 通道、資料表資料匯入（CSV / Excel）、外掛

## 授權條款

[MIT](LICENSE)
