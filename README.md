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
│   │   ├── driver.go         # Driver / Conn / Dialect 契约 + 注册表（init 注册）
│   │   ├── sqlutil/          # 标识符引用、WHERE / ORDER BY 构造（纯字符串+参数位）
│   │   ├── sqlbase/          # 通用 database/sql 实现：连接池、分页、脚本执行、DDL、行变更
│   │   ├── mysql/ postgres/ sqlite/   # 只提供 DSN、Dialect 与目录查询
│   │   ├── planned/          # MongoDB / Oracle / SQL Server 占位（implemented=false）
│   │   └── all/              # 汇总导入，保证 init 注册
│   └── service/manager.go    # 会话管理、超时、只读校验、审计入口
└── frontend/
    └── src/
        ├── api/              # 手写类型 + 手写 window.go.main.App 桥接（不依赖生成代码）
        ├── components/       # AppShell / Sidebar / Workspace / TablePane / QueryPane / DataGrid …
        ├── hooks/useConnect  # 连接 + 密码提示流程
        ├── lib/              # tree key 编解码、格式化、导出、品牌素材（assets.ts，@asserts 别名）
        ├── store/            # zustand 全局状态（连接 / 会话 / 标签页 / 浏览器缓存）
        └── styles/global.css
```

## 架构要点

### 驱动抽象不是「SQL 专用」的

`drivers.Driver` → `drivers.Conn` → `drivers.Dialect` 三层契约里没有任何 SQL 词汇：
`Databases / Schemas / Objects / Structure / Fetch / Execute` 都可以由文档型引擎用
「database → collection → field」映射实现，`Execute` 则翻译为自己的查询语言。
新增引擎只需实现 `Driver` 并在 `init()` 中 `drivers.Register`。

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

## 测试

```sh
just test
```

`internal/drivers/sqlite` 的测试覆盖了完整链路：目录查询、结构 + DDL、
主键顺序分页、`COUNT(*)`、`contains` / `isNull` 过滤、内联更新（含写入 `NULL`）、
过期主键返回 0 行、按主键删除。`internal/config` 与 `internal/service` 还分别盯住了
查询收藏的磁盘往返（更新不重复、删不掉别人的文件）与校验/排序/保留 `createdAt`。

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
- [ ] Phase 1.5：
  - [x] 表设计器：字段与索引的可视化编辑 + 实时 SQL 预览（Navicat 的「结构」页）
  - [x] 查询收藏：命名 SQL 片段，查询窗口里可载入 / 改名 / 删除
  - [ ] DDL 编辑器、ER 图
- [ ] Phase 2：MongoDB（文档编辑 + 查询语言）、Oracle、SQL Server
- [ ] Phase 3：SSH 隧道、导入向导、数据对比、插件式扩展

## 许可证

[MIT](LICENSE)
