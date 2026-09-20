# db-manager

使用 **Wails v2 + React + Vite + Justfile** 开发的关系型数据库管理工具，打包为 Windows 桌面应用。

第一阶段支持 **MySQL / PostgreSQL / SQLite**；驱动层已为非关系型引擎预留抽象，第二阶段可直接接入 **MongoDB / Oracle / SQL Server**，无需改动 service 与 UI 层。

> 在本仓库写代码前先读 [`AGENTS.md`](AGENTS.md)：每次开发完成必须 commit + push，本地打包用 `just release`，发版走 `just publish <x.y.z>` 触发 GitHub Actions 打出三平台免安装可执行文件。

## 功能

- **连接管理**：连接配置的增删改查、连通性测试、SQLite 文件选择、TLS（CA / 证书 / 私钥）、自定义 DSN 参数、只读标记、颜色标签、密码可选保存。
- **对象浏览器**：会话 → 数据库 → Schema → 表 / 视图 / 索引 的懒加载树，右键菜单支持打开数据、新建查询、复制名称。
- **对象列表**：点击树里的表 / 视图 / 索引文件夹，在右侧开出 Navicat 风格的对象网格（名称 / 类型 / 行数 / 大小 / 引擎 / 注释，索引列还有所属表 / 列 / 唯一性 / 主键 / 方法），支持列排序、列筛选、底部关键字过滤，单击打开对象、双击进入设计视图。
- **数据网格**：分页、服务端排序、服务端过滤（14 种操作符）、列宽自适应、长文本悬浮预览、多选、行详情抽屉（JSON / INSERT 预览）。
- **行编辑**：双击单元格内联编辑、批量删除选中行；所有写操作都以主键为条件并**全部使用参数绑定**。
- **SQL 编辑器**：基于 CodeMirror 6，按驱动切换方言、SQL 语法高亮与补全、多语句执行、执行历史、`Ctrl/Cmd+Enter` 执行全部、`Ctrl/Cmd+Shift+Enter` 执行选中。
- **结构查看器**：列、索引、外键、原始 DDL（优先使用引擎原生 DDL），DDL 可复制或导出。
- **表设计器**：表格窗口的「结构」页就是编辑器 —— 直接改字段名 / 类型 / NULL / 默认值 / 主键 / 自增 / 注释，增删索引，右侧实时渲染将要执行的 SQL 与引擎限制警告；保存前无需联网猜测，保存时逐条执行并如实报告「第几条失败」（MySQL / PostgreSQL / SQLite 各自的限制都写在警告里）。
- **查询收藏**：查询窗口工具条上的「Favourites」可以把当前 SQL 命名保存（默认用第一行非注释文本作名），下拉里一键载入、重命名或删除；收藏存在 `queries.json` 里，与连接配置互不影响，换窗口、换连接都能用。
- **ER 图**：在 schema（没有 schema 层的引擎就是 database）节点右键即可打开该命名空间的关系图 —— 一张 `GetSchemaGraph` 就把对象、字段与它们之间的外键取回来，节点按外键方向分层，主键高亮，箭头悬停显示「哪一列引用哪一列」；支持拖动平移、滚轮缩放、按名搜索、隐藏/显示字段、网格开关，点节点直接打开该表，还能把当前这张图导出成自包含的 SVG。跨命名空间的外键画成虚线 stub，读不到字段的对象仍在图里但会列出警告，超过 300 个对象时明确提示只画了前 300 个。
- **DDL 编辑器**：把对象的定义开成可编辑的脚本窗口 —— 工具条、对象树右键「Edit DDL…」或结构页的「Edit in DDL editor」都能进；编辑器下方是**后端算出来的干跑结果**（逐条语句标出 query / DDL / DML、标红 DROP、TRUNCATE、无 WHERE 的 DELETE/UPDATE，并说明只读连接会拒掉几条），真正点「Run script」时只对破坏性脚本弹二次确认；执行完顺手刷新目录树与索引缓存。
- **运行情况**：双击连接节点即可打开该连接的「运行情况」页（未连接会先连上，密码框填完再自动打开）；页面上半部分是会话事实与快照时间，下面按引擎各画各的 —— MySQL 给进程列表、连接数、InnoDB 缓冲池与命中率，PostgreSQL 给后端/活动会话/数据库体积与提交率、缓存命中率，SQLite 则是「这是一个文件」的视角（路径、落盘大小、pragma、对象清单与 ATTACH 进来的库）。读不到的项一律显示 `—` 并附一条警告（缺权限、缺统计视图），不会拿 0 冒充；工具条的刷新按钮重新取一次快照。
- **项目信息**：没连库时右侧只有一页项目信息，标题就是 `DB Manager` 加当前版本（`v0.1.0`，字号比正文大一档）—— 标题下面一排徽章（`license MIT`、`build just 1.58.0`、`platform`），下面是技术栈徽章（shields 样式的灰标签 + 品牌色值，前端库版本直接读 `frontend/package.json`，Go 版本取自运行中的二进制），再下面依次是项目地址 / Releases / Issues、开发者（GitHub 头像 + 昵称，点一下打开 `github.com/freewu`）、Build（链接到 Justfile 并列出 `just build` / `just release` / `just publish`）。徽章用本地 CSS 画，离线也能渲染；链接交给系统浏览器（`window.runtime.BrowserOpenURL`），不把整个窗口导航走。连库的入口在命令条的 Connection / Open，以及连接树的右键菜单。
- **导出**：CSV / JSON / INSERT 脚本，可写入文件或复制到剪贴板。
- **外观**：Navicat 式窗口骨架（菜单栏 + icon-over-label 命令条 + 连接树 + 标签页工作区 + 状态栏）、明暗主题、品牌绿 `#36ab60`、可拖拽分栏、紧凑的表格与状态栏。

## 环境要求

| 工具 | 版本 |
| --- | --- |
| Go | 1.24+（开发时使用 1.26.5） |
| Node.js | 20+（开发时使用 24.10.0） |
| Wails CLI | v2.16.0 |
| just | 1.58.0（可选，仅用于便捷脚本） |
| WebView2 | Windows 10/11 自带；旧系统需安装 Runtime |

## 快速开始

```sh
just install     # 安装前端依赖 + 下载 Go 模块
just icons       # 同步品牌素材：asserts/ → 前端 favicon + build/appicon.png
just dev         # 开发模式：Vite 热更新 + Go 热重载
just build       # 生产构建，产物在 build/bin/db-manager.exe
just release     # 本地打包：构建 + 归档到 dist/（带校验和），不碰 git
just doctor      # 打印各个工具链版本
just publish 0.2.0 "本版总结"   # 发版：同步版本号 + 提交 + 打 tag + push
just --list      # 查看全部任务
```

不使用 `just` 时：

```sh
cd frontend && npm install && npm run build && cd ..
wails build
```

> `wails` 构建的是 Windows 桌面程序，因此请使用 **Windows 工具链** 运行（在 Windows 终端或 `just.exe` 中执行），不要用 WSL 里的 Linux 工具链。

## 项目结构

```
.
├── app.go                    # Wails 绑定层：AppInfo / 连接 / 元数据 / 查询 / 文件对话框
├── main.go                   # 应用入口，embed frontend/dist，窗口 1360×860
├── AGENTS.md                 # 开发约定：提交 / 发版 / 环境坑（动手前必读）
├── Justfile                  # 开发任务（windows-shell = PowerShell）
├── wails.json                # 版本号权威来源（info.productVersion）
├── asserts/                  # 品牌素材的唯一来源：logo.png、icon/<引擎>.png
├── scripts/
│   ├── version.mjs           # 版本号同步与一致性校验
│   ├── package.mjs           # 本地打包：归档到 dist/ 并生成 checksums.txt
│   └── release-notes.sh      # 渲染 GitHub Release message
├── .github/workflows/
│   ├── ci.yml                # main / PR：go vet+test、tsc+vite build、素材一致性
│   └── release.yml           # tag v*：三平台可执行文件 + GitHub Release
├── dist/                     # `just release` 的产物（已 gitignore）
├── build/                    # 图标、清单、安装包脚本（appicon.png 由 `just icons` 同步）
├── internal/
│   ├── apperr/               # 错误码 + 脱敏（打码 password=... 与 URI userinfo）
│   ├── config/store.go       # %APPDATA%/db-manager/{connections,queries,state}.json（0600）
│   ├── models/               # 跨层 DTO，时间统一为 int64 unix ms
│   ├── drivers/
│   │   ├── driver.go         # Driver / Conn / Dialect / Grapher / Overviewer 契约 + 注册表（init 注册）
│   │   ├── sqlutil/          # 标识符引用、WHERE / ORDER BY 构造（纯字符串+参数位）
│   │   ├── sqlbase/          # 通用 database/sql 实现：连接池、分页、脚本执行、DDL、行变更、运行情况外壳
│   │   ├── mysql/ postgres/ sqlite/   # 只提供 DSN、Dialect、目录查询与各自的 overview 收集器
│   │   ├── planned/          # MongoDB / Oracle / SQL Server 占位（implemented=false）
│   │   └── all/              # 汇总导入，保证 init 注册
│   └── service/manager.go    # 会话管理、超时、只读校验、审计入口
└── frontend/
    └── src/
        ├── api/              # 手写类型 + 手写 window.go.main.App 桥接（不依赖生成代码）
        ├── components/       # AppShell / Sidebar / Workspace / TablePane / QueryPane / DataGrid …
        │   ├── AboutProject.tsx  # 项目信息（技术栈徽章 / 项目地址 / 开发者），空态页与 Help → About 共用
        │   └── overview/     # 运行情况：每个引擎一个视图 + 共用的指标卡片与数据表
        ├── hooks/useConnect  # 连接 + 密码提示流程
        ├── lib/              # tree key 编解码、格式化、导出、项目信息（about.ts）、品牌素材（assets.ts，@asserts 别名）
        ├── store/            # zustand 全局状态（连接 / 会话 / 标签页 / 浏览器缓存）
        └── styles/global.css
```

## 架构要点

### 驱动抽象不是「SQL 专用」的

`drivers.Driver` → `drivers.Conn` → `drivers.Dialect` 三层契约里没有任何 SQL 词汇：
`Databases / Schemas / Objects / Structure / Fetch / Execute` 都可以由文档型引擎用
「database → collection → field」映射实现，`Execute` 则翻译为自己的查询语言。
新增引擎只需实现 `Driver` 并在 `init()` 中 `drivers.Register`。
少数能力是**可选**的：比如 ER 图需要的 `drivers.Grapher`（一次取回整个命名空间），
`sqlbase` 已经实现，没实现的驱动由 service 退化成逐个对象的 `Structure`，图照样能画。

### `sqlbase` 用一个包实现全部 SQL 引擎

MySQL / PostgreSQL / SQLite 仅声明一份 `Spec`（`DSN` 构造函数、`Dialect`、目录查询实现），
即可获得：连接池（每库一个池）、分页、`COUNT(*)`、多语句脚本执行、值类型归一化
（`[]byte` → UTF-8 或 `0x…`、`time.Time` → RFC3339Nano）以及 DDL 渲染。
`SQLServerDialect` / `OracleDialect` 已预置。

### 前端不消费生成的绑定

`frontend/src/api/types.ts` 与 `client.ts` 是手写的，`wailsjsdir` 生成的
`frontend/wailsjs` 仅作构建产物。这样 `npm run typecheck` / `npm run build`
可以独立于 Go 构建运行，类型也能表达字符串联合（如 `DriverType`）。

### 行变更的安全约定

- 前端把 **所有单元格值都以字符串（或 null）** 传回后端，避免 Wails JSON 桥接把
  `BIGINT` / `NUMERIC` 变成 `float64` 造成精度丢失；类型强转交给数据库引擎。
- 行标识是确定性的 `[]models.KeyValue`（切片而非 map），保证 WHERE 子句与参数顺序稳定。
- 只有标识符经过 `Dialect.Quote` 拼接，值一律走绑定参数；空字符串表示 `NULL`。
- `UpdateCell` / `DeleteRow` 返回受影响行数，为 0 时 UI 提示「行已被他人修改或删除」。

### 密码处理

密码仅在勾选「保存密码」时写入 `connections.json`（文件权限 `0600`）；
其余情况下密码只存在于内存中，由 `useConnect` 在每次打开连接时询问。
前后端之间不会把密码回传到 UI：`ConnectionConfig.Redacted()` 会剥离密码并置
`HasPassword`，`apperr.Sanitize()` 会把日志与错误里的 `password=…`、URI userinfo 打码。

密码提示框是普通受控 `Input.Password`（**不在 antd `Form` 里**：无名 `Form.Item` 的
校验/取值会把表单 store 覆盖成用户输入的那串字符），重复调用 `connect()` 也不会清空
已经敲进去的内容。

### 连接树

展开连接节点就是「连上它」：一次只发起一次加载，失败或取消后把节点**折叠再展开**
即重试一次，可以随时从右键菜单「Open connection」重连。
`loadedKeys` 直接交给 rc-tree（**不能过滤** —— 它会驱动 rc-tree 内部的 loaded 状态，
过滤掉一个 key 就等于让这棵树在每次渲染时重新加载，即死循环），「有没有数据」的判断
只在展开事件里用一次。加载失败的分支都带一个 Retry 按钮。

右键菜单就是全部入口（底部没有 New / Connect 按钮）：

| 右键处 | 菜单 |
| --- | --- |
| 面板空白处 | New connection |
| 未连接的连接 | Open connection / Edit connection… |
| 已连接的连接 | New query / Refresh / New database… / Edit connection… / Disconnect |

**双击连接节点**是「看它的运行情况」：没连上就先连（该弹密码框就弹，填完再自动打开），
已经有会话就直接切到那一页。一次点击（展开）保持原来的行为 —— 只连接、不开页面，
所以「连上了」和「去看看它现在在干什么」是两件事，不会互相打扰。

「New database…」只要一个库名，语句由 `quoteIdent` 按当前引擎拼好并**先展示再执行**
（`CREATE DATABASE …`，同样走 `ExecuteSQL`），SQLite 这类文件型引擎与只读会话直接禁用。

没有任何标签页时右侧就只有一页**项目信息**（`WelcomePane` 渲染 `AboutProject`），标题是应用名 + 版本
（版本只出现在标题里，徽章里不再重复）—— 这一页不再放快捷入口、连接卡片与引擎清单，因为那些在
命令条与连接树里已经有了：技术栈徽章的版本号从
`frontend/package.json` 读（徽章不可能写出包里没有的版本），Go 版本来自后端 `AppInfo`，
项目地址 / 作者 / 许可证 / 构建工具集中在 `frontend/src/lib/about.ts`（仓库地址、Issues、Releases、
Justfile 链接都由 `REPO_OWNER` / `REPO_NAME` 拼出来，改一处即可），其中两处要手动对齐：作者与
`wails.json` 的 `author`、`just` 版本与下面的「环境要求」表。许可证只以徽章形式出现，不再占一行。
徽章是本地 CSS 画的灰标签 + 品牌色值，不依赖 shields.io，断网也照常显示；
链接统一走 `openExternal`，桌面壳里交给系统浏览器，纯浏览器里退化成新标签页。开发者头像是这一页唯一
需要联网的东西（`github.com/<owner>.png`），拿不到时 antd `Avatar` 会退回昵称首字母的灰底头像。

### 运行情况从哪来

`drivers.Overviewer` 是可选接口，`sqlbase` 提供唯一一份实现，把会话事实（名字、驱动、
版本、只读、连接时刻）与耗时交给 service 填，各引擎只往 `Spec.Overview` 里挂一个收集器。
「引擎回答不了这个页面」不是错误，而是一条警告（页面照常渲染会话信息），只有真正取不到
数据（连接断了）才报错 —— 所以 `CodeUnsupported` 在 service 里被降级成 warning。
读不到的指标一律是 `-1` / `—`，绝不用 0 冒充；每一条子查询（pg_stat_activity、
`pg_database_size`、`SHOW FULL PROCESSLIST` …）失败都只降级成自己的那条警告，
一个权限不足不会让整页白掉。

## 测试

```sh
just test
```

`internal/drivers/sqlite` 的测试覆盖了完整链路：目录查询、结构 + DDL、
主键顺序分页、`COUNT(*)`、`contains` / `isNull` 过滤、内联更新（含写入 `NULL`）、
过期主键返回 0 行、按主键删除。`internal/config` 与 `internal/service` 还分别盯住了
查询收藏的磁盘往返（更新不重复、删不掉别人的文件）与校验/排序/保留 `createdAt`；
`internal/drivers/sqlutil` 则用纯函数盯住脚本干跑的分类与破坏性判定（注释里的 `drop` 不算，
无 `WHERE` 的 `DELETE` 要算），不依赖任何数据库；ER 图在 SQLite 上端到端跑一遍
（外键方向、主键/可空标记、跨命名空间的目标名），service 层再用一个只实现 `Conn`
的包装验证「没有 `Grapher` 时退化成逐对象 `Structure`」这条路。

## 发布

本地打包和对外发版是两件事。

### 本地打包

```sh
just release
```

编译当前平台并把产物归档到 `dist/`，附 `checksums.txt`：

```
dist/db-manager-0.1.0-windows-amd64.exe
 dist/checksums.txt
```

只在本机产出文件，**不做任何 git 操作**，脏工作区也能跑。前端资源已 `embed` 进可执行文件，
所以拷走这一个文件就能跑，无需 Node/Go。macOS 上产出 `.tar.gz`（`.app` 是目录，不能只拿文件）。
版本号取自 `wails.json`。

### 发版到 GitHub

发版只需要改版本号、打 tag、push，其余交给 GitHub Actions：

```sh
just publish 0.2.0 "新增 Navicat 风格连接树；索引成为一等资源"
```

`scripts/version.mjs` 会把 `0.2.0` 同步到 `wails.json`、`Justfile`、
`frontend/package.json` 与 `app.go` 的兜底常量，然后提交 `chore(release): v0.2.0`、
打附注标签（第二个参数写进 tag message）并推送。tag 触发
[`.github/workflows/release.yml`](.github/workflows/release.yml)：

1. **Verify** —— 版本号四处一致、tag 与 `wails.json` 一致、品牌素材无漂移、
   `go vet` + `go test` + `tsc --noEmit`；
2. **Build** —— 在 Windows / macOS / Linux 三个 runner 上各跑一次 `wails build`
   （`darwin/universal` 同时覆盖 Intel 与 Apple Silicon，Linux 链接 GTK3 + WebKitGTK 4.0）；
3. **Publish** —— 汇总产物、生成 `checksums.txt`，用 `scripts/release-notes.sh`
   把「tag 附注 + 两次 tag 之间的提交（按 Conventional Commits 分组）+ 下载与运行说明」
   渲染成 release message，创建 GitHub Release。

前端产物已经 `embed` 进可执行文件，三平台产物都是免安装、单独可运行的。
发版前可以先在本地预览 release message：`just notes v0.2.0`。
不用 Actions、只想拿到本机能跑的文件时，用 `just release` 就够了。

## 路线图

- [x] Phase 1：MySQL / PostgreSQL / SQLite（连接、浏览、编辑、SQL、结构、导出）
- [x] Phase 1.5：
  - [x] 表设计器：字段与索引的可视化编辑 + 实时 SQL 预览（Navicat 的「结构」页）
  - [x] 查询收藏：命名 SQL 片段，查询窗口里可载入 / 改名 / 删除
  - [x] DDL 编辑器：对象定义开成可编辑脚本，附逐条语句的干跑预警
  - [x] ER 图：命名空间的关系图，可搜索 / 缩放 / 导出 SVG，点节点开表
  - [x] 运行情况：双击连接看服务端现状，三个引擎各自一个视图
- [ ] Phase 2：MongoDB（文档编辑 + 查询语言）、Oracle、SQL Server
- [ ] Phase 3：SSH 隧道、导入向导、数据对比、插件式扩展

## 许可证

[MIT](LICENSE)
