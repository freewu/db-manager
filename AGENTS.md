# AGENTS.md

给在本仓库里干活的 AI 代理 / 协作者的约定。**动手前先读这里，再读 `README.md` 与源码。**

---

## 0. 每次开发完成的收尾动作（硬性要求）

一轮任务做完、验证全绿之后，必须依次执行：

```sh
# 1) 验证（WSL 里必须带 .exe，见 §3；前端命令要在 frontend/ 下跑）
cd frontend && /mnt/d/env/nodejs/node.exe node_modules/typescript/bin/tsc --noEmit && cd ..
/mnt/d/env/go/go1.26.5/bin/go.exe vet ./... && /mnt/d/env/go/go1.26.5/bin/go.exe test ./...

# 2) 提交并推送
git add -A
git commit -m "<type>(<scope>): <一句话摘要>"
git push origin HEAD
```

**不允许留下未提交的改动。** 任务结束时工作区必须是干净的（`git status --short` 无输出）。

git 操作要在 **Windows 侧**跑（或在 Windows 终端里执行）：WSL 里的 `git push` 到不了外网，原因见 §3。
如果 `/mnt/e/work/github/db-manager` 已经在 Windows 那边跑过 `git config user.name/user.email`，提交可以直接在 WSL 里做，只把 `push` 交给 Windows。

`just typecheck` / `just test` / `just sync` 等价于上面这些命令，但它们调用的是裸的
`npm` / `go` / `git`，而 WSL 的 PATH 里只有 `*.exe`，所以**从 WSL 跑会直接找不到命令**；
要用 just 就得到 Windows 侧跑（`just` ↔ `just.exe`）。

### 提交信息

遵循 Conventional Commits，因为 **提交信息会直接变成下一个版本的 release message**：

```
<type>(<scope>): <摘要>
```

| type | 用途 | 在 release notes 里的分组 |
| --- | --- | --- |
| `feat` | 新功能 | 新增 |
| `fix` | 缺陷修复 | 修复 |
| `perf` | 性能 | 性能 |
| `refactor` | 不改变行为的重构 | 重构 |
| `docs` | 文档 | 文档 |
| `test` | 测试 | 测试 |
| `chore` / `build` / `ci` / `style` | 杂项 | 其他 |

摘要写成**用户能看懂的一句话**，不要写 "update code" / "fix bug" 这类空话。
语言跟 release notes 保持一致（本仓库的 release message 是中文，所以摘要也用中文）。
`chore(release): v*` 会被 release notes 自动过滤掉。

---

## 1. 本地打包与发版

两件事分得很开：

| 命令 | 做什么 | 碰 git 吗 |
| --- | --- | --- |
| `just release` | **本地打包**：编译当前平台，产物归档到 `release/` + `checksums.txt` | 不碰 |
| `just publish <x.y.z> "总结"` | 发版：同步版本号、提交、打 tag、push，触发 GitHub Action 出三平台产物并发布 Release | 会 |

### 本地打包

```sh
just release
# → release/db-manager-<版本>-<os>-<arch>[.exe]   （macOS 为 .tar.gz）
# → release/checksums.txt
```

只在本机产出文件，**不做任何 git 操作**，脏工作区也能跑。版本号取自 `wails.json`；
要换版本先跑 `node scripts/version.mjs <x.y.z>` 或等发版时统一改。
实现见 `scripts/package.mjs`：Windows / Linux 直接给可执行文件（前端资源已 embed，本身就是单文件），
macOS 给 `.tar.gz`（`.app` 是目录）。

### 发版（触发 GitHub Action）

```sh
just publish 0.2.0 "新增 Navicat 风格连接树；索引成为一等资源；支持 PostgreSQL 分区表"
```

这条命令会：

1. `node scripts/version.mjs 0.2.0` —— 把版本号同步到所有镜像位置（见下）；
2. 提交 `chore(release): v0.2.0`；
3. 打**附注**标签 `v0.2.0`，把第二个参数（版本总结）写进 tag message；
4. push 分支与 tag。

push tag 会触发 [`.github/workflows/release.yml`](.github/workflows/release.yml)：

- `Verify` 校验版本镜像一致、tag 与 `wails.json` 一致、品牌素材无漂移、`go vet`/`go test`/`tsc`；
- `Build` 在 Windows / macOS / Linux 三个 runner 上各跑一次 `wails build`，把产物改名成
  `db-manager-<版本>-windows-amd64.exe` / `-linux-amd64` / `-macos-universal.zip` 后上传（macOS 的 `.app` 是目录，打包成 zip；前端资源已 embed，三个文件都是免安装可执行文件）；
- `Publish` 汇总产物、**先核对三个平台都到齐了**、生成 `checksums.txt`（`sha256sum db-manager-*`），
  调用 `scripts/release-notes.sh` 渲染 release message 并创建 GitHub Release。

这套命名在 `release.yml` 的改名步骤和 `scripts/release-notes.sh` 的下载表格里各写了一次，
**改一处要改两处**（`docs/i18n.js` 的下载说明里不写具体文件名，所以不用跟着改）。

### release message 怎么来的

`scripts/release-notes.sh <tag>` 拼出三段：

1. **附注 tag 的 message** —— 也就是 `just publish` 的第二个参数，人写的本版总结；
2. **两个 tag 之间的提交**，按上表分组；
3. 下载表格 + 各平台运行说明（macOS 未签名、Linux 需要 GTK3/WebKitGTK 等）。表格里的文件名由版本号拼出来，
   跟 `release.yml` 的改名步骤是同一套命名。

所以：**写清楚提交信息 = 写好 release notes**。发版前可以本地预览：

```sh
just notes v0.2.0      # tag 还不存在时自动回退到 HEAD
```

### 版本号的唯一来源

`wails.json` 的 `info.productVersion` 是权威值，`scripts/version.mjs` 负责同步镜像：

| 文件 | 字段 | 说明 |
| --- | --- | --- |
| `wails.json` | `info.productVersion` | 权威值 |
| `frontend/package.json` | `version` | |
| `Justfile` | `version := "…"` | `just build` / `just release` 的 ldflags 来源 |
| `app.go` | `var Version = "…-dev"` | 开发期兜底；发布构建用 ldflags 覆盖 |

`just check-version`（CI 里也跑）会在四处不一致时报错。
**不要手改这些字段**，一律走 `just publish` 或 `node scripts/version.mjs <x.y.z>`
（`just release` 只读取它，不修改）。

---

## 2. 动手前必须知道的约定

- **前端是「四页 + 窗口」两层**：主区域分 `connections`（工作区）/ `datagen` / `changelog` / `settings`
  四页（`AppPage`），由最左边那条图标栏（`ActivityBar`）切；页面**先挂载、之后一直挂着（只隐藏）**，
  所以别靠卸载去重置一页内的状态，也别指望 effect 会在每次切回来时重跑（要重新读的话用
  `page === '…'` 当依赖，见 `ChangeLogPane` / `SettingsPane`）。**开窗口必须走 store 的
  `front(kind, id)`**（把窗口和它的页面一起切过去），`activeTabId` 只属于工作区页、`datagenTabId`
  只属于数据生成页 —— 新增一种窗口时两者都要顺着这套走，否则会出现「窗口开了但看不见」。
- **三份 README 是同一份文档的三种语言**：`README.md`（英文，主文档）、`README_CN.md`、`README_TC.md`。
  **加功能就三份一起改**：英文是原文，另外两份是它的镜像。繁中用台湾习惯（資料庫 / 資料表 / 欄位 /
  資料列），简中沿用本仓库既有的用词（数据库 / 表 / 字段 / 行）。README 只留「用户视角」的一句话说明，
  深挖设计取舍的内容写进 `AGENTS.md` 或代码注释，不再往 README 里堆。
  `node scripts/readme-check.mjs`（CI 里也跑）会比对三份的结构、相对链接、下载文件名，以及
  `docs/images/` 里的每张截图是否三份都展示了 —— 但它只保证「形状一致」，措辞是否准确还得自己看。
  截图表格里的图与标题直接沿用 `docs/images/` 与介绍页的 `shot.*.title`，换图或改标题时两边一起改。
- **介绍页 `docs/` 有两副面孔：仓库里那份，和发到 GitHub Pages 的那份。** 仓库里那份是按「从仓库根目录
  提供服务、或者直接双击打开」写的，所以引擎图标与版本清单写成 `../asserts/icon/…`、`../wails.json`；
  而项目站点在子路径下（<https://freewu.github.io/db-manager/>），`../` 会直接爬出站点根目录。
  因此 `scripts/pages-build.sh` 会把 `docs/` 复制一份、把 `asserts/` 与 `wails.json` 摆到页面旁边、
  再抹掉那两处前缀，`.github/workflows/pages.yml` 发的是这份副本（CI 里也会拼一遍）。
  **新增一个 `../` 引用就同时改这个脚本**，否则线上就是 404；`node scripts/docs-check.mjs --site _site`
  （在 `_site` 上跑）会抓住漏改。页面上不要再出现指向仓库里 markdown 的相对链接 —— README 那类
  一律写成 GitHub 绝对地址。启用 Pages 只需一次：Settings → Pages → Source 选 GitHub Actions
  （`configure-pages` 想代为开启需要非 `GITHUB_TOKEN` 的令牌，所以这一步得手工做一次）。
- **品牌素材唯一来源是 `asserts/`**（注意目录名就是 `asserts`，不是 `assets`，不要"顺手改正"）。
  `frontend/public/logo.png`、`build/appicon.png` 与 `docs/logo.png` 是它的副本，由 `just icons` 生成；
  `build/windows/icon.ico` 被 gitignore —— Wails 只在它**不存在**时才由 `appicon.png` 重新生成，
  所以换了 logo 必须删掉它。CI 会用 `cmp` 检查三份副本是否漂移。
  新的引擎图标放 `asserts/icon/`，在 `frontend/src/lib/assets.ts` 注册（`@asserts` 别名指向该目录）。
- **主题色 `#36ab60` 写在两处，必须同步**：`frontend/src/App.tsx` 的 `colorPrimary`，
  与 `frontend/src/styles/global.css` 的 `--dm-accent*`。
- **托盘的菜单只在 `tray.go` 一处**：文字 / 顺序 / 勾在哪一行 / 版本号都出自 `trayMenuRows(prefs)`，
  `tray_windows.go` 只负责把它递归画成 Win32 菜单（子菜单用 `MF_POPUP` 挂）。菜单里那两档设置
  （显示主题 / 界面语言）的**偏好仍然只归前端一份**：Go 只把选中的值当事件发出去
  （`trayThemeEvent` / `trayLanguageEvent`，前端 `lib/tray.ts` 里各写一份同名常量，`tray_test.go`
  会去读那个文件对一遍），前端用 `setTheme` / `setUiLanguage` 应用并落盘 —— **托盘不写 `state.json`**，
  否则同一份偏好就有了第二个写者。要勾哪一行读的是 `trayPrefsStore` 里的副本（`startup` 与每次
  `SaveState` 之后刷新），不是每次右键去读一遍可能写了一半的文件。`trayWordsByLanguage` 是 Go 侧
  **唯一**翻译过的文案，用词必须与设置页对齐（`libTheme.*`、`settingsPane.*`）；三档语言名自身不翻译。
- **前端不消费 Wails 生成的绑定**：`frontend/src/api/{types,client}.ts` 是手写的，
  `frontend/wailsjs/` 只是构建产物（已 gitignore）。加后端方法时，两边都要手写。
- **`tsconfig` 开了 `noUnusedLocals` / `noUnusedParameters`**：删代码时记得删导入，
  否则 `tsc` 直接失败。
- **antd Tree 的 node key 必须全局唯一**：占位/错误节点用 `placeholder:${scope}` / `error:${scope}` 这种带作用域的前缀。
- **表格列宽由 `components/ResizableHeader.tsx` 提供**：新的 `Table` 要接上 `useColumnResize`
  （`{...grid.tableProps}` + `columns={grid.columns}`，拖动过就给 `className` 加 `dm-grid-resized`），
  否则表头拖不动；装饰列（行号、拖动抓手、行操作）写 `resizable: false`。
  第一次拖动会把**当前屏幕上所有列**的宽度量下来定死并改用 `table-layout: fixed`（列少、内容长时的
  「拖不动 / 打滑」都是这一条在管），宽度不落盘；列多了重复的 `key`/`dataIndex` 会被自动区分，
  编辑表（`DataGenPane`）里同 `dataIndex` 的两列务必自己给 `key`。
  定死宽度之后就由 `FILLER_KEY` 那一列（宽度 `auto`、内容为空）去接剩下的空间，所以表格仍旧填满窗格 ——
  **不要**再加 `.dm-grid-resized table { min-width: 0 }` 这种覆盖，那会连 antd 的 `min-width: 100%` 一起废掉，
  变成所有列平分剩余宽度（拖动就不准了）。
- **行详情是网格自己的一层，不是 antd `Drawer`**：`DataGrid` 管壳（绝对定位在 `.dm-result-area` 上、
  `transform` 滑入滑出、右上角关闭、`Esc`、`dm-grid-row is-detail` 高亮），内容由调用方用
  `renderRowDetail(row, rowIndex)` 给（`detailRow` / `onDetailRowChange` 控开关）。触发是**单击一行**（不是双击、
  也不是勾选框 —— 看一行不该顺手把它勾上）；`.dm-result-area` 必须有 `position: relative; overflow: hidden`，
  层要盖住钉住的表头（表头 `z-index: 100`，层是 `200`）。
- **表设计器（结构页）发送的是「完整目标定义」，不是 diff**：`TableDesign` 是用户想要的样子，
  后端 `sqlbase.PlanAlter` 拿实时 catalog 结构对比后渲染语句 —— **预览与保存走同一条代码路径**。
  由此派生几条硬规则：
  - `ApplyDesign` 只接收设计、不接收 SQL，前端无法借此发任意 SQL；它先重新 plan 一次，再**逐条**执行。
  - 没有事务包裹（`Conn.Execute` 无 Tx 接口，DDL 也不可回滚）：失败时返回 `FailedIndex`/`Error`，
    如实报告「第几条失败、已执行几条」，不假装全部成功。
  - 默认值**逐字输出**为原始 SQL（用户自己写 `'text'` / `0` / `CURRENT_TIMESTAMP`）；
    `nil` 表示「无默认」，与 `DEFAULT ''` 不是一回事。
  - 引擎表达不了的变更写进 `Plan.Warnings` **而不是静默跳过**（尤其 SQLite）。
  - 字段改名时索引跟着改：前端在重命名时同步索引列，后端把只写了旧名字的索引列也重映射到新名字。
- 后端新增能力时按 `drivers.Driver` → `Conn` → `Dialect` 契约落地，并在 `init()` 里 `drivers.Register`。
- **数据目录不由「写死的路径」决定，而是由默认位置里的指针 `location.json` 决定**（见
  `internal/config/location.go`）。**新增一份数据文件必须同时加进 `dataFiles`**，否则用户换目录时它会被
  落在原处。搬家的顺序是复制 → 逐字节校验 → 改指针 → 删旧文件，不要改成先删后拷。

---

## 3. 环境（WSL + Windows 混合）

本机开发在 WSL 里编辑、用 Windows 工具链执行。几个坑：

- `go` / `node` 必须带 `.exe` 后缀调用：
  `/mnt/d/env/go/go1.26.5/bin/go.exe`、`/mnt/d/env/nodejs/node.exe`。
  WSL 的 PATH 里只有 `*.exe`（没有裸名），因此 **Justfile 里裸调 `go`/`node`/`npm`/`wails`
  的配方在 WSL 里会 "command not found"**：`just typecheck` / `just test` / `just build`
  得在 Windows 侧跑，或者在 WSL 里按下面“验证命令”手敲一遍。
- `wails`、`git`（推送）要用 Windows 侧的可执行文件：
  WSL 是 NAT 模式，`~/.gitconfig` 里的 `http.proxy=http://127.0.0.1:7897` 指向 Windows 宿主，
  WSL 里访问不到 → `git push` 会超时。推送走 `cmd.exe /c "cd /d E:\work\github\db-manager && git push"`。
  本机 WSL 里的 `just` 是 Linux ELF，并不存在 `just.exe`（别对着它干等）。
- `node scripts/*.mjs` 用 Windows 的 node 跑也完全没问题（`process.platform` 会正确报 `win32`）。
- 每条 bash 命令都会带一行 WSL NAT 警告，噪音，忽略即可。
- 构建用的是 Windows 工具链，所以 `just build` / `just release` / `wails build` 必须在 Windows 侧执行。
  从 cmd.exe 传含空格的参数（如 `-ldflags "-X main.Version=…"`）容易被引号兑死，
  写个临时 `.bat` 再 `cmd.exe /c` 跑它最稳。

### 验证命令（提交前必须全绿）

```sh
# 前端（必须在 frontend/ 下跑：vite 要在那里找 index.html，tsc 要在那里找 tsconfig）
cd /mnt/e/work/github/db-manager/frontend
/mnt/d/env/nodejs/node.exe node_modules/typescript/bin/tsc --noEmit
/mnt/d/env/nodejs/node.exe node_modules/vite/bin/vite.js build

# 后端（在仓库根目录）
cd /mnt/e/work/github/db-manager
/mnt/d/env/go/go1.26.5/bin/go.exe build ./... && \
/mnt/d/env/go/go1.26.5/bin/go.exe vet ./... && \
/mnt/d/env/go/go1.26.5/bin/go.exe test ./...

# 端到端（改了 wails.json / 图标 / main.go 时）
cmd.exe /c "cd /d E:\work\github\db-manager && C:\Users\24358\go\bin\wails.exe build -platform windows/amd64"
```

---

## 4. 目录速查

```
app.go                  Wails 绑定层（前端调用的入口就是这里的方法名）
main.go                 入口：embed frontend/dist、窗口参数
scripts/version.mjs     版本号同步 / 校验（wails.json 是权威值）
scripts/package.mjs     本地打包：把 build/bin 的产物归档到 release/ + checksums.txt
scripts/release-notes.sh 生成 GitHub Release message
scripts/docs-check.mjs  校验 docs/ 介绍页的词典、截图引用与相对路径
                        （`--site <目录>` 改校验 Pages 发布出来的那份副本）
scripts/readme-check.mjs 校验三份 README 的结构、相对链接、截图与下载文件名
scripts/pages-build.sh  拼出 GitHub Pages 的发布目录（默认 _site）
README.md               英文主文档；README_CN.md / README_TC.md 是简繁镜像（三份同改）
docs/                   介绍页（静态、无构建步骤），也是 Pages 的源
docs/logo.png           介绍页自己的 logo（`asserts/logo.png` 的副本，`just icons` 同步）
docs/images/            六张截图（README 截图表格、轮播与画廊共用）
asserts/                品牌素材唯一来源（logo.png、icon/*.png）
build/                  appicon.png、windows/darwin 打包资源（icon.ico 已 gitignore）
release/                本地打包产物（just release，已 gitignore）
internal/               后端：驱动契约、sqlbase、服务层
frontend/src/           React 前端（api 手写、store 用 zustand、样式在 styles/global.css）
.github/workflows/      ci.yml（main/PR）、release.yml（tag → 三平台产物 + Release）、
                        pages.yml（docs/ → GitHub Pages）
```

更细的架构说明见 `README.md`（三份 README 里只有英文那份带完整细节）与各处的代码注释。
