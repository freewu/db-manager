# db-manager

[English](README.md) · **简体中文** · [繁體中文](README_TC.md)

一个支持六种引擎 —— **MySQL、PostgreSQL、SQLite、MongoDB、TiDB、Apache Doris** —— 的桌面数据库客户端，基于 Wails v2 + React，在 Windows / macOS / Linux 上都以单个自包含可执行文件分发。

它不只是个浏览器：能导出整个数据库、逐条执行 `.sql` 文件、用 mock.js 模板造数据、比较两个库并生成同步脚本、设计表、按 18 种语言生成代码，还会把它被要求执行的每一条写语句记进变更日志 —— 连影响行数一起。任何会改数据的操作都先弹确认，且确认框里显示的语句就是真正发给引擎的那一条。

驱动层不预设关系模型：文档型引擎走的是同一套 `Driver / Conn / Dialect` 契约，界面问的是「能力」而不是引擎名字 —— 所以某个引擎伺候不了的入口（MongoDB、Doris 的表设计器，造数据，Explain）是**不出现**，而不是点了才报错。Oracle 与 SQL Server 停在 `internal/drivers/planned` 里。

> 要在本仓库里干活？先读 [`AGENTS.md`](AGENTS.md)：每轮任务都以一次提交和推送收尾。

## 下载

每个版本都产出三个**免安装可执行文件**：前端资源已经编译进去，那一个文件就是整个程序。到 [最新发布](https://github.com/freewu/db-manager/releases/latest) 取自己平台的那个（下表 `<version>` 是你下载的版本号，比如 `0.1.0`）：

| 平台 | 文件 | 运行方式 |
| --- | --- | --- |
| Windows x64 | `db-manager-<version>-windows-amd64.exe` | 双击即可。需要 WebView2 运行时（Windows 10/11 自带） |
| macOS（Intel + Apple Silicon 通用） | `db-manager-<version>-macos-universal.zip` | 解压后把 `db-manager.app` 拖进「应用程序」 |
| Linux x64 | `db-manager-<version>-linux-amd64` | `chmod +x db-manager-<version>-linux-amd64 && ./db-manager-<version>-linux-amd64` |

macOS 包没有签名与公证，首次打开会被 Gatekeeper 拦下：右键「打开」，或执行 `xattr -cr /Applications/db-manager.app`。Linux 需要 GTK3 与 WebKitGTK 4.0（Debian/Ubuntu 上是 `libgtk-3-0 libwebkit2gtk-4.0-37`）。下载旁边还有一份 `checksums.txt`，可以 `sha256sum -c checksums.txt` 校验。

## 功能

- **连接管理** —— 新建、编辑、测试、颜色标记、只读锁定；TLS（CA / 证书 / 私钥）与自定义 DSN 参数；每种驱动一张表单，文件型连接不会出现主机和端口。保存密码是可选项，且加密存放。
- **对象浏览器** —— 懒加载的会话 → 数据库 → schema → 表 / 视图 / 索引树，空文件夹也照常画出来（`Tables (0)`），有哪些文件夹由引擎决定。右键可以打开数据、设计表、复制名字、复制 / 清空 / 删除表、运行 SQL 文件、导出数据库、生成代码。
- **数据表格** —— 分页、服务端排序与筛选（14 个操作符）、可拖动列宽、多选、长文本悬停预览，单击一行会滑出该行详情（字段、主键、NULL、复制为 JSON 或 INSERT，以及就地编辑）。
- **行编辑** —— 单元格就地编辑与批量删除，语句一律用主键生成并走参数绑定，执行前把渲染好的语句摆给你看。
- **表设计器** —— 改字段名、类型、可空、默认值、主键、自增、注释，拖动手柄调整行序（行序就是列序）。保存时把将要逐条执行的语句原样列出；新建表是同一条路径，只是基线是空的。
- **SQL 编辑器** —— CodeMirror 6、按驱动切换方言、多语句执行、历史记录、只补全本窗口所属命名空间下的表与视图，`Ctrl/Cmd+Enter` 执行，`Ctrl/Cmd+Shift+F` 格式化。
- **执行计划** —— Explain 问的是引擎「打算怎么做」，包装层绝不加 `ANALYZE`，所以它不执行任何语句；只读连接也能先看清一条 `DELETE` 再决定要不要跑。
- **结构与 DDL** —— 字段、索引、外键，以及引擎原生的 DDL，带语法高亮、可复制。文档型引擎给的是抽样字段表，因为集合本来就没有结构。
- **ER 图** —— 把一个命名空间画成图：有关联的表排在同一行，外键层级从左到右，可拖动、缩放、搜索、隐藏字段，并导出 SVG。
- **数据库比较** —— 同一引擎的两个库并排比，逐表给出「仅在左 / 仅在右 / 有差异 / 一致」以及差异字段和索引。**生成脚本**给的就是这份差异本身（先建、再改、后删），唯一要你决定的是「只在右边存在的表要不要删掉」。
- **数据生成** —— 每列一个 mock.js 模板，占位符选择器里有 40 多个内置项，也能自定义，并按列名和类型给出建议。数据按 200 行一批插入，带进度、可停止，最后给一句「实际写入多少行、花了多久」。
- **数据库导出** —— 在数据库节点上选「结构 / 结构+数据 / 仅数据」：整库导出成一个 `.sql` 脚本（引擎原生的 `CREATE` 加 `INSERT`），或把单表导成 CSV、JSON、JSONL、SQL 并自选列。读写全在后端完成、直接落盘，带进度和停止。
- **运行 SQL 文件** —— 把一个磁盘上的 `.sql` 文件交给后端，它逐条执行，因此每条写语句都会单独记下影响行数。执行前先告诉你文件里有什么（破坏性语句标红、只读连接会拒绝哪些、`USE` 与识别不了的语句列为警告），然后才是进度、停止，以及一份会点出「第几条失败」的总结。整个文件**不在事务里**，界面上也是这么写的。
- **代码生成** —— 把一张表的字段生成 18 种语言的类或结构体（Java 分带 Lombok 和不带两种），可空按语言各自处理，命名按惯例，带高亮，可复制或保存。
- **变更日志** —— 本程序被要求执行过的每一条写语句，含时间、连接、库 / schema、表、来源窗口和引擎报告的影响行数。每天一个文件放在 `<数据目录>/log/`，超限整文件轮转而不是截断，不连库也能看。
- **新建数据库** —— 服务器需要什么就问什么（字符集与排序规则、编码与 locale），语句先给你看再执行。
- **概览** —— 双击连接看它的服务端实时数据：进程列表与 InnoDB 命中率、后端与提交速率，或 SQLite 文件的 pragma 视图。读不到的指标显示 `—` 并附一条警告，而不是显示 0。
- **设置** —— 一页管住主题（浅色 / 深色 / 跟随系统）、界面语言、代码生成语言、mock 占位符、数据目录（打开、搬家、设置单个日志文件记多少条）以及项目信息。改完立即生效。
- **界面语言** —— English / 简体中文 / 繁體中文，默认英文；状态栏与 Windows 托盘菜单切的是同一份偏好。
- **托盘（Windows）** —— 关窗口只是隐藏；菜单可以把窗口叫回来、切换显示主题与界面语言、打开项目主页、退出。

## 支持的引擎

| 引擎 | 默认端口 | 说明 |
| --- | --- | --- |
| MySQL | 3306 | TLS、按字符集与排序规则建库、完整的表设计器 |
| PostgreSQL | 5432 | 有 schema 层、认 `serial` / identity、编码与 locale |
| SQLite | 文件 | ATTACH 别名、pragma 概览、没有 `TRUNCATE`（用 `DELETE`） |
| MongoDB | 27017 | 副本集、`mongodb+srv`、TLS、mongosh 风格命令行、无结构 |
| TiDB | 4000 | MySQL 线协议、TLS、集群成员表 |
| Apache Doris | 9030 | MySQL 线协议、只读浏览、改结构走 DDL 编辑器 |

MySQL、TiDB、Doris 共用同一个线协议包，因此也共用连接、浏览、表格、查询、导出这些代码。接入新引擎就是实现 `Driver` / `Conn` / `Dialect` 并在 `init()` 里注册。

## 环境要求

| 工具 | 版本 |
| --- | --- |
| Go | 1.24+（开发机上是 1.26.5） |
| Node.js | 20+（开发机上是 24.10.0） |
| Wails CLI | v2.16.0 |
| just | 1.58.0（可选，只是让命令短一点） |
| WebView2 | Windows 10/11 自带；更老的系统需要装运行时 |

## 从源码构建

```sh
just install     # npm install + go mod download
just dev         # 开发模式：Vite 热更新 + Go 热重载
just build       # 生产构建，产物在 build/bin/db-manager.exe
just release     # 构建 + 归档到 release/ 并生成校验和（不碰 git）
just publish 0.2.0 "这次改了什么"   # 同步版本号、提交、打标签、推送
just --list      # 看所有配方
```

不用 `just` 的话：

```sh
cd frontend && npm install && npm run build && cd ..
wails build
```

`wails build` 产出的是桌面程序，所以在 Windows 上要用 **Windows 工具链**（Windows 终端或 `just.exe`）跑，别在 WSL 里跑。

## 数据放在哪

连接配置、保存的查询、连接顺序、窗口状态、密码密钥和变更日志都放在同一个数据目录里：Windows 是 `%APPDATA%\db-manager`，macOS 是 `~/Library/Application Support/db-manager`，Linux 是 `~/.config/db-manager`。它的位置记在 `location.json` 里，设置页的「数据目录」负责搬家：复制 → 逐字节校验 → 改指针 → 才删旧文件。

密码用 AES-256-GCM 加密，密钥首次运行时生成（`secret.key`，权限 0600）。不往任何地方发数据：没有埋点，程序只连你自己配的那些数据库。

## 代码结构

Wails v2 把 React 19 + Vite 的前端装进 WebView2 / WebKit 窗口，`app.go` 是前端调用的绑定层。前端是照着手写的 `frontend/src/api/{types,client}.ts` 写的（不用生成的绑定），状态集中在一个 zustand store 里。界面文案放在 `frontend/src/lib/i18n/messages/<area>.ts`，每条形如 `[English, 简体, 繁體]`，由 `scripts/i18n.mjs verify` 把关；后端 Go 的文案目前还是英文。

```
app.go, main.go          Wails 绑定、窗口、embed 进去的 frontend/dist
tray*.go                 Windows 托盘菜单（主题 + 语言）及其空实现桩
internal/drivers/        Driver / Conn / Dialect 契约、sqlbase、每个引擎一个包
internal/service/        manager、设计器、导出、比较、变更日志、数据生成
internal/config/         JSON 存储、数据目录指针、变更日志文件
frontend/src/            React 界面：components、lib、i18n、store、styles
docs/                    介绍页（静态，没有构建步骤）
scripts/                 version.mjs、package.mjs、i18n.mjs、docs-check.mjs、release-notes.sh
```

动手前值得知道的四条约定：

- **表设计器发的是「完整目标定义」，不是 diff。** 后端拿实时 catalog 结构去 plan，所以预览与保存走的是同一条代码；引擎表达不了的变更写成 `Plan.Warnings`，而不是静默跳过。
- **每次写操作都先问、后记。** 确认框里显示的语句就是真正发出去的那条，变更日志记的是引擎报告的影响行数。
- **不在自己兜不住的事务里跑。** DDL 本来就不可回滚，运行 SQL 文件那条路径会直说，而不是假装安全。
- **品牌素材只有一个来源** `asserts/`；`frontend/public/logo.png` 与 `build/appicon.png` 都是 `just icons` 生成的副本。

## 测试

```sh
just test                        # go test ./...
npm --prefix frontend run build  # tsc --noEmit + vite build
node scripts/i18n.mjs verify     # 译文与英文原文对得上
node scripts/docs-check.mjs      # 介绍页的文案、截图与相对路径
node scripts/readme-check.mjs    # 三份 README：结构一致、链接有效、下载文件名一致
```

Go 测试不需要 cgo，也不需要起任何服务容器 —— SQLite 是纯 Go 的。MongoDB、TiDB、Doris 的集成测试在不给它服务器地址时会自己跳过：

```sh
DMB_TEST_MONGODB_HOST=127.0.0.1 go test ./internal/drivers/mongodb/ -run Integration -v
DMB_TEST_TIDB_HOST=127.0.0.1 go test ./internal/drivers/tidb/ -run Integration -v
DMB_TEST_DORIS_HOST=127.0.0.1 go test ./internal/drivers/doris/ -run Integration -v
```

## 发版

推一个 `v*` 标签会触发 [`.github/workflows/release.yml`](.github/workflows/release.yml)：先校验版本号镜像是否一致、标签与 `wails.json` 是否一致、品牌素材有没有漂移，然后在 Windows、macOS、Linux 三个 runner 上各自构建，发布三个可执行文件加一份 `checksums.txt`。

Release 说明由 [`scripts/release-notes.sh`](scripts/release-notes.sh) 拼出来：附注标签的 message 加上两个标签之间的提交（按 Conventional Commits 分组）—— 所以每条提交的标题都得写成用户看得懂的一句话。

```sh
just notes v0.2.0   # 预览某个标签的 release message
```

`just release` 只在本机干活：编译当前平台并归档到 `release/`，不碰 git。

## 介绍页

[`docs/index.html`](docs/index.html) 是一张静态介绍页：一个 HTML、一个样式表、一个脚本，再加上装着同样三档语言的 `i18n.js`。没有构建步骤，双击就能打开；`docs/images/` 里那六张截图同时供轮播和画廊使用。`scripts/docs-check.mjs` 盯着三份词典、截图引用和相对路径别走歪，CI 里也会跑。

## 路线图

- [x] MySQL / PostgreSQL / SQLite：浏览、编辑、查询、结构、导出
- [x] 表设计器、保存的查询、DDL 编辑器、执行计划与格式化、ER 图、新建数据库、概览、变更日志、数据生成、代码生成
- [x] MongoDB，以及后来的 TiDB 与 Apache Doris
- [x] 数据库比较与同步脚本；每次写入前确认；逐条记录影响行数
- [x] 数据库导出；运行 SQL 文件
- [x] 三档界面语言；Windows 托盘切换主题与语言
- [ ] Oracle / SQL Server（方言桩已经就位）
- [ ] SSH 隧道、表数据导入（CSV / Excel）、插件

## 许可证

[MIT](LICENSE)
