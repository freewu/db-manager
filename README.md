# db-manager

使用 **Wails v2 + React + Vite + Justfile** 开发的关系型数据库管理工具，打包为 Windows 桌面应用。

支持 **MySQL / TiDB / Apache Doris / PostgreSQL / SQLite / MongoDB**；驱动层不假设关系模型，文档型引擎走的是同一套 `Driver / Conn / Dialect` 契约，service 与 UI 只问能力、不问引擎名。Oracle / SQL Server 的占位仍在（见 `internal/drivers/planned`），但暂不上菜单。

> 在本仓库写代码前先读 [`AGENTS.md`](AGENTS.md)：每次开发完成必须 commit + push，本地打包用 `just release`，发版走 `just publish <x.y.z>` 触发 GitHub Actions 打出三平台免安装可执行文件。

## 功能

- **连接管理**：连接配置的增删改查、连通性测试、SQLite 文件选择、TLS（CA / 证书 / 私钥）、自定义 DSN 参数、只读标记、颜色标签、密码可选保存。新建连接先点出**驱动菜单**（命令条 `Connection`、连接树的 `+`、`File ▸ New Connection`、面板空白处右键，四处挂的是同一份列表，就展开在你刚点的那个东西下面），再落到**这个引擎自己的那一页** —— 走网络的要地址、端口、账号与 TLS，SQLite 只要一个文件加一个附加库别名，两边不会互相看到无关字段（保存下来的配置也照着这一页来，文件型连接不会混进 host / port / ssl）；编辑已有连接同样按它的驱动打开对应那页。
- **对象浏览器**：会话 → 数据库 → Schema → 表 / 视图 / 索引 的懒加载树，右键菜单支持打开数据、新建查询、复制名称；文档型引擎这里是 database → Collections / Indexes，没有 schema 层，SQL 专属的入口（新建 DDL 脚本、ER 图、设计对象）自动不出现。
- **MongoDB**：连接（含副本集多主机、`mongodb+srv`、TLS、认证库）、集合浏览（文档数 / 体积 / 索引）、数据网格的过滤排序与分页、双击改标量字段、批量删文档、索引列表与定义脚本、运行情况页（serverStatus + 每库 dbStats）。查询窗口跑的是 **mongosh 风格的 shell**（`db.orders.find({...}).sort({ts: -1}).limit(20)`），不是 SQL。
- **TiDB / Apache Doris**：两个引擎对客户端都讲 MySQL 线协议，但脾气各不相同。TiDB 默认端口 4000，有 TLS 页（`Security`），表单里的 `Database` 只是新标签页的默认库 —— 一个 TiDB 集群就是一份逻辑数据库，树里一次列全所有库，所以这一栏是可选的；运行情况是 MySQL 那一页再加一张 `information_schema.CLUSTER_INFO` 的集群成员表（tidb / tikv / tiflash / ticdc / pd）。Doris 默认端口 9030，前端（FE）与后端（BE）都在集群网内、MySQL 端也不做那套握手，于是**没有 TLS 页**；它是分析型引擎，本工具里**只读浏览** —— 表结构与索引照样能看（字段列表说的是引擎自己的类型拼写，如 `varchar(120)` / `decimal(10,2)`），但没有表设计器，改动走 DDL 编辑器。两者的连接、库表浏览、数据网格、SQL 查询与导出都复用 MySQL 那条代码路径。
- **对象列表**：点击树里的表 / 视图 / 索引文件夹，在右侧开出 Navicat 风格的对象网格（名称 / 类型 / 行数 / 大小 / 引擎 / 注释，索引列还有所属表 / 列 / 唯一性 / 主键 / 方法），支持列排序、列筛选、底部关键字过滤，单击打开对象、双击进入设计视图。
- **数据网格**：分页、服务端排序、服务端过滤（14 种操作符）、列宽自适应、长文本悬浮预览、多选、行详情抽屉（JSON / INSERT 预览）。
- **行编辑**：双击单元格内联编辑、批量删除选中行；所有写操作都以主键为条件并**全部使用参数绑定**。
- **SQL 编辑器**：基于 CodeMirror 6，按驱动切换语言（MongoDB 用 JavaScript，其余用各自方言）、语法高亮与补全、多语句执行、执行历史、`Ctrl/Cmd+Enter` 执行全部、`Ctrl/Cmd+Shift+Enter` 执行选中。
- **结构查看器**：列、索引、外键、原始 DDL（优先使用引擎原生 DDL），DDL 可复制或导出；文档型引擎的「结构」页是**抽样得到的字段表**（字段名 / 类型 / 是否可能缺失），并说明集合本身没有 schema。
- **表设计器**：表格窗口的「结构」页就是编辑器 —— 直接改字段名 / 类型 / NULL / 默认值 / 主键 / 自增 / 注释，增删索引，右侧实时渲染将要执行的 SQL 与引擎限制警告；保存前无需联网猜测，保存时逐条执行并如实报告「第几条失败」（MySQL / PostgreSQL / SQLite 各自的限制都写在警告里）。引擎给不出设计器的（MongoDB、Doris）这一页退化成**只读字段列表**并直说「这个引擎没有表设计器」，不摆一个按下去会失败的按钮。
- **查询收藏**：查询窗口工具条上的「Favourites」可以把当前 SQL 命名保存（默认用第一行非注释文本作名），下拉里一键载入、重命名或删除；收藏存在 `queries.json` 里，与连接配置互不影响，换窗口、换连接都能用。
- **ER 图**：在 schema（没有 schema 层的引擎就是 database）节点右键即可打开该命名空间的关系图 —— 一张 `GetSchemaGraph` 就把对象、字段与它们之间的外键取回来，节点按外键方向分层，主键高亮，箭头悬停显示「哪一列引用哪一列」；支持拖动平移、滚轮缩放、按名搜索、隐藏/显示字段、网格开关，点节点直接打开该表，还能把当前这张图导出成自包含的 SVG。跨命名空间的外键画成虚线 stub，读不到字段的对象仍在图里但会列出警告，超过 300 个对象时明确提示只画了前 300 个。
- **DDL 编辑器**：把对象的定义开成可编辑的脚本窗口（MongoDB 下就是集合的定义脚本与 shell 查询） —— 工具条、对象树右键「Edit DDL…」或结构页的「Edit in DDL editor」都能进；编辑器下方是**后端算出来的干跑结果**（逐条语句标出 query / DDL / DML、标红 DROP、TRUNCATE、无 WHERE 的 DELETE/UPDATE，MongoDB 下则是 `drop()` / `dropDatabase()` / 无 filter 的 `deleteMany`，并说明只读连接会拒掉几条），真正点「Run script」时只对破坏性脚本弹二次确认；执行完顺手刷新目录树与索引缓存。
- **窗口即应用**：右键不再弹出 WebView 自带的那套菜单（后退 / 刷新 / 另存为 / 打印 / 检查），
  右键要么什么都不做，要么就是应用自己的菜单（连接树等）；文本框与 SQL 编辑器是例外 —— 那里保留系统菜单，
  右键粘贴照旧可用，其它地方用 `Ctrl+C` / `Ctrl+V`。
- **运行情况**：双击连接节点即可打开该连接的「运行情况」页（未连接会先连上，密码框填完再自动打开）；页面上半部分是会话事实与快照时间，下面按引擎各画各的 —— MySQL 给进程列表、连接数、InnoDB 缓冲池与命中率（TiDB 走同一页，外加集群成员表；Doris 也尽力取这一套，取不到的项挂进警告里），PostgreSQL 给后端/活动会话/数据库体积与提交率、缓存命中率，SQLite 则是「这是一个文件」的视角（路径、落盘大小、pragma、对象清单与 ATTACH 进来的库）。读不到的项一律显示 `—` 并附一条警告（缺权限、缺统计视图），不会拿 0 冒充；工具条的刷新按钮重新取一次快照。
- **项目信息**：没连库时右侧只有一页项目信息，标题就是 `DB Manager` 加当前版本（`v0.1.0`，字号比正文大一档）—— 标题下面一排徽章（`license MIT`、`build just 1.58.0`、`running windows/amd64`），再按组列出 **`Build`**（一枚可点的 `justfile` 徽章，值就是三条常用配方，点开是仓库里的 Justfile）、技术栈（`Runtime`、`Desktop and UI`）与 **`Platforms`**（`windows amd64` / `macos universal` / `linux amd64`，和 `.github/workflows/release.yml` 的构建矩阵一一对应）—— 徽章是 shields 样式的灰标签 + 品牌色值，前端库版本直接读 `frontend/package.json`，Go 版本取自运行中的二进制，再下面依次是项目地址 / Releases / Issues 和开发者（只画一个 GitHub 头像，悬停显示昵称、点一下打开 `github.com/freewu`）。徽章用本地 CSS 画，离线也能渲染；链接交给系统浏览器（`window.runtime.BrowserOpenURL`），不把整个窗口导航走。连库的入口在命令条的 Connection / Open，以及连接树的右键菜单。
- **导出**：CSV / JSON / INSERT 脚本（MongoDB 下是 `insertMany` 脚本，按列的 BSON 类型还原 `$oid` / `$date` / 文档字面量），可写入文件或复制到剪贴板。
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

### 开发时窗口一片空白（黑屏）

`wails dev` 的窗口要 React 挂载之后才有内容，所以**模块加载失败的样子就是一片空白**（露出来的是窗口底色）。
最常见的原因不是代码写错了，而是**开发服务器手里那份模块是旧的**：源码在 WSL 里改，Windows 侧 Vite 的
文件监听并不总能收到这些写入，于是它一直发着「文件当时还是空的」那份 transform，页面在
`does not provide an export named …` 上停住。

判断与恢复：

- 用浏览器（或 `curl`）打开 `wails dev` 打印的地址对应的源码 URL，例如
  `http://127.0.0.1:34115/src/components/AboutProject.tsx`：**返回空内容**就是这个问题；
- 在 **Windows 侧**碰一下那个文件让 Vite 重新读（`copy /b file+,,`，或用 PowerShell 原样重写一遍），
  页面会自己恢复；仍然不行就重启 `wails dev`；
- 现在有两层防护：`vite.config.ts` 里开了 `server.watch.usePolling`，开发服务器改为轮询源码，
  WSL 侧写入也能看见；`index.html` 里有一段启动看门狗 —— 十秒后 `#root` 还是空的，就把错误
  （暗底、原因、`Ctrl+R` 提示）画出来，不再只留一个空白窗口。看控制台按 **F12**（右键菜单已经关掉了，
  见下面「窗口即应用」）。

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
│   ├── secret/               # AES-256-GCM 封装连接的密码；密钥 secret.key（0600，首次用时生成）
│   ├── models/               # 跨层 DTO，时间统一为 int64 unix ms
│   ├── drivers/
│   │   ├── driver.go         # Driver / Conn / Dialect / Grapher / Overviewer / Analyzer 契约 + 注册表（init 注册）
│   │   ├── format/           # 指标格式化（字节 / 计数 / 时长 / 百分比），两个 overview 实现共用
│   │   ├── sqlutil/          # 标识符引用、WHERE / ORDER BY 构造（纯字符串+参数位）、脚本干跑
│   │   ├── sqlbase/          # 通用 database/sql 实现：连接池、分页、脚本执行、DDL、行变更、运行情况外壳
│   │   ├── mysqlcompat/      # MySQL 家族共用件：DSN / TLS、目录查询、原生 DDL、SHOW 解析、overview 外壳
│   │   ├── mysql/ postgres/ sqlite/   # 只提供 DSN、Dialect、目录查询与各自的 overview 收集器
│   │   ├── tidb/ doris/      # 同样讲 MySQL 线协议的两个引擎：各报端口、系统 schema 与 overview 差异
│   │   ├── sqltest/          # 假 database/sql/driver：给「照 SHOW / information_schema 结果拼结构」写单测
│   │   ├── mongodb/          # 官方 v2 驱动：文档 ↔ 表格映射、shell 解析与执行、索引、运行情况
│   │   ├── planned/          # Oracle / SQL Server 占位（`Infos()` 暂不返回，`Parked()` 留着路线图）
│   │   └── all/              # 汇总导入，保证 init 注册
│   └── service/manager.go    # 会话管理、超时、只读校验、审计入口
└── frontend/
    └── src/
        ├── api/              # 手写类型 + 手写 window.go.main.App 桥接（不依赖生成代码）
        ├── components/       # AppShell / Sidebar / Workspace / TablePane / QueryPane / DataGrid …
        │   ├── AboutProject.tsx  # 项目信息（技术栈徽章 / 项目地址 / 开发者），空态页与 Help → About 共用
        │   └── overview/     # 运行情况：每个引擎一个视图 + 共用的指标卡片与数据表
        ├── connection/       # 每种驱动一页连接表单（Mysql / Postgres / Sqlite / Mongodb / Tidb / Doris）+ 注册表
        ├── hooks/useConnect  # 先试后问的连接流程
        ├── lib/              # tree key 编解码、格式化、导出、驱动能力（capabilities.ts）、项目信息（about.ts）、品牌素材（assets.ts，@asserts 别名）
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
`sqlbase` 已经实现，没实现的驱动由 service 退化成逐个对象的 `Structure`，图照样能画；
DDL 编辑器要的 `drivers.Analyzer`（脚本干跑）同理 —— SQL 引擎用 `sqlutil` 的关键字启发式，
MongoDB 用自己的解析器回答，因为 `db.orders.drop()` 认不出任何 SQL 关键字。

**「引擎能不能做这件事」由后端说，前端只读结果**：`DriverInfo.relational` / `supportsDatabase` /
`supportsSchema` 决定界面上出现什么，`supportsDesign` 说明这个引擎有没有表设计器（MongoDB、Doris 为
`false`）——「结构」页据此换成只读字段列表：有列不等于这个引擎能被这个设计器编辑，如实说「没有设计器」
比给一个点了会失败的按钮好。前端读的是同一份结果（`frontend/src/lib/capabilities.ts`），不按引擎名写 if ——
`driver !== 'mongodb'` 这种判断只对一次，再加一个文档型引擎就全是洞。

### `sqlbase` 用一个包实现全部 SQL 引擎

MySQL / PostgreSQL / SQLite 仅声明一份 `Spec`（`DSN` 构造函数、`Dialect`、目录查询实现），
即可获得：连接池（每库一个池）、分页、`COUNT(*)`、多语句脚本执行、值类型归一化
（`[]byte` → UTF-8 或 `0x…`、`time.Time` → RFC3339Nano）以及 DDL 渲染。
`SQLServerDialect` / `OracleDialect` 已预置。

### 讲同一种线协议的引擎共用一个包

TiDB 与 Doris 对客户端而言都是 MySQL，于是 `internal/drivers/mysqlcompat` 收下了这一族的全部共用件：
DSN（含 TLS 白名单与 `interpolateParams`）、一份 `sqlbase.Introspector` 实现（照 `information_schema`
与 `SHOW` 拼目录）、原生 DDL、`SHOW` 输出解析，以及 overview 的公共外壳。引擎自己的包只回答差异：
默认端口、要藏掉哪些系统库、有没有设计器，以及 overview 多给哪几张表 —— Doris 的 introspector 用
`newIntrospector()` 构造，免得零值悄悄退回 MySQL 的默认系统库清单。`sqlbase.MySQLDialect` 带一个
`Driver` 字段，`mysqlFamily()` 是「是不是这一族」的唯一判断；前端对应的是 `lib/sqlFlavor.ts` 的
`isMySQLFamily()`，标识符引用、DDL 模板与编辑器方言都问它。

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

打开连接是**先试后问**的：`useConnect` 先拿配置里已有的东西（存下的密码，或干脆什么都没有）
去连一次。存了密码的连接、SQLite 文件、以及服务端本来就允许空密码的连接（本地 `root`、刚建的
PostgreSQL 角色）全程不弹框；**只有这次尝试被服务端拒绝**，才弹出密码框，并把服务端给的那句
错误原样放进框里 —— 是密码不对还是根本连不上，一眼能分清。同一个连接被拒过一次就记在内存里，
下次直接弹框，不再白跑一趟。

框里敲下的密码只用于本次会话（后端在 `resolveConfig` 里复用已有会话与已存密码）；
勾了「保存密码」的连接会在**连接成功之后**顺手落盘一次 —— 失败就不写，免得把一个打错的
密码存进配置。

**落盘的密码是密文**：`internal/secret` 用一个 32 字节随机密钥（`secret.key`，权限 `0600`，
与 `connections.json` 同目录）做 AES-256-GCM，配置里存的是 `enc:v1:` + base64(nonce‖密文) 这串
令牌，内存里才还原成明文（`config.Store` 在存/取时封装与解封，其它层拿到的永远是明文）。
所以一个被拷走、被同步到网盘、被贴进工单的配置文件里没有可读的密码，手改过的密文也认证不过。
密钥第一次真要写密码时才生成 —— 从不保存密码的安装根本不会有这个文件。它挡不住能同时读到
这两个文件的人，要再上一层就得把密钥交给系统钥匙串（DPAPI / Keychain / libsecret），那是
后续的事。旧版本直接写成明文的密码不会失效：读的时候照用，下一次保存时自动变成密文；
密钥丢了或被换掉时，解不开的令牌按「没有存密码」处理（弹框重新问，而不是打不开配置），
并且原样留在文件里 —— 免得在这台机器上一次无关的保存把别的机器还能读的密钥抹掉。

前后端之间不会把密码回传到 UI：`ConnectionConfig.Redacted()`
会剥离密码并置 `HasPassword`，`apperr.Sanitize()` 会把日志与错误里的 `password=…`、
URI userinfo 打码。`HasPassword` 为真的连接要是连不上，只报错、不弹框：密码已经在库里了，
再问一遍没有意义。

密码提示框是普通受控 `Input.Password`（**不在 antd `Form` 里**：无名 `Form.Item` 的
校验/取值会把表单 store 覆盖成用户输入的那串字符），连试失败**不关闭**：错误显示在框里，
已经敲进去的内容照旧留着，改完再按 Connect；同一个连接的框重复打开也不会被清空。

### 新建连接：先选驱动，再填那一页

「新建连接」不是先弹一个通用表单、再靠 Driver 下拉换页，而是两步：

1. `connectionTypeItems()`（`ConnectionTypeMenu.tsx`）把 `ListDrivers` 按 `sortOrder` 排成一份两行的
   菜单项 —— 第一行引擎名，第二行是这个引擎的路数（「A local .db file — no server, no credentials」）；
   `implemented=false` 的驱动在分隔线下面，看得见但点不动 —— 空菜单比禁用按钮更让人困惑。
   这份列表挂在四个入口上：命令条 `Connection`、连接树的 `+`、`File ▸ New Connection`（子菜单）、
   面板空白处右键（`dm-blank-menu` 直接列引擎，不再多一次「New connection」跳转）。
   按钮上挂菜单要小心：`Dropdown` 的 `onClick` 会注入给子元素，中间隔一个 `Tooltip` 就被吞掉，
   这类触发点外面套一层 `<span class="dm-dropdown-anchor">`。
2. 选中后 `openConnectionEditor({ driver })` 只带一个**草稿**（`ConnectionDraft = Partial<ConnectionConfig> &
   Pick<ConnectionConfig, 'driver'>`），`ConnectionDialog` 拿到草稿后按 `driverForm(type)` 查出该驱动的页面
   渲染，其余（显示名、只读、颜色标签、连通性测试、保存）留在外壳里。
   **驱动不再在弹窗里选**：草稿里已经定了，弹窗只在标题左边摆一个该引擎的图标，换引擎就回菜单里点另一个。
   菜单项的 key 带前缀（`file.new.mysql`），`driverFromKey()` 只取最后一段。

页面在 `frontend/src/connection/`：`shared.tsx` 放各页共用的字段零件（`HostPortFields`、`CredentialsFields`、
`TlsFields`、`FilePathField`…），`MysqlConnect.tsx` / `PostgresConnect.tsx` / `SqliteConnect.tsx` /
`MongodbConnect.tsx` 各导出一个
`DriverForm = { Basic, Security?, Advanced?, summary }`，`index.ts` 的 `DRIVER_FORMS` 是唯一注册点 ——
加引擎就是加一个文件加一行注册。
`Basic` / `Security` / `Advanced` 就是弹窗里那三条分割线 tab（默认 `Basic`），驱动没填的 tab 不出现
（SQLite 是本地文件，既没有 TLS 也没有驱动参数，于是一个 tab 都不显示，弹窗直接就是那张表单）。
外壳不按驱动名写 if：`requiresFile` 决定 `collect()` 往配置里放 filePath 还是 host/port/username/ssl，
页面决定这些字段长什么样。

三个 tab 的字段**一起挂载**（非当前 tab 只是 `hidden`），所以按保存时 `validateFields()` 一次校验全部；
某个必填项出错就跳回它所在的 tab（`FIELD_TAB` 那张表），否则红字停在看不见的地方。

「Test connection」的结果是**贴着按钮上方浮出来的一小块 `Alert`**（`.dm-test-slot` / `.dm-test-notice`），
不是表单里的横幅，也不是窗口角落的提示：测试是对已经填好的值做一次校验，答案就该长在按它的那颗按钮上，
跟着弹窗走，也不该把字段挤得跳来跳去、或在值被改过之后还挂在那里说「连接成功」。面板按 `bottom: 100% + 8px`
贴着按钮定位（不用量、也不会指向一个过期坐标），`TEST_TOAST_MS` 秒后自己消失，鼠标悬停时留着不关，
再点一次是**替换**同一条而不是叠一屏。文字一律靠左 —— 面板长在 footer 里，而 footer 为了排按钮
设了 `text-align: right`，不显式改回来的话版本号与报错全贴着右边。

### 文档型引擎怎么接进来

MongoDB 不从 `sqlbase` 继承任何东西（那个包是 `database/sql` 专用），`internal/drivers/mongodb`
是官方 `go.mongodb.org/mongo-driver/v2` 之上的一层映射：

| 关系型 | MongoDB | 说明 |
| --- | --- | --- |
| database | database | 同名，`Databases()` 隐藏 `config` / `local`，`admin` 留着 |
| schema | — | `SupportsSchema=false`，集合直接挂在库下 |
| table | collection | `Objects()` 返回 `KindCollection`，行数 / 体积来自 `$collStats`（有界并发，超上限就跳过） |
| column | field | 没有 schema：`Structure()` 抽样最多 100 条文档推断字段与类型并集（`int32 \| string` 照实写） |
| index | index | `listIndexes`，`_id_` 排最前，文本 / 地理索引各有类型标注 |
| DDL | 定义脚本 | 可重放的 `db.createCollection(...)` + `createIndex(...)`，集合名不是普通标识符时用 `db.getCollection("…")` |
| SQL | shell | `db.<coll>.<cmd>(...)`、`db.getCollection("…")`、`db.<cmd>(...)`、`show dbs\|collections` |

几条必须记住的取舍：

- **数据网格里的值都是文本**，因为 Wails 桥只能递 JSON：`ObjectID` 走 24 位 hex、时间走 ISO 文本、
  文档与数组走扩展 JSON。`UpdateCell` / `DeleteRow` 时再把 hex 还原成真正的 `ObjectID`，
  非标量字段（文档、数组）在网格里**只读** —— 用一个字符串覆盖一个文档不是编辑，是删数据。
- **filter 值只有是合法 JSON 才保类型**，其余按裸字符串；`_id` 列上的 24 位 hex 自动升级为 `ObjectID`
  （这里踩过坑：`json.Decoder` 只读第一个值，`"507f1f77bcf86cd799439011"` 会被解析成数字 `507`，
  过滤器于是静默匹配不到任何东西 —— 现在用 `json.Unmarshal`，尾随垃圾直接报错）。
- **扩展 JSON 的 `$` 外壳会被还原成真正的 BSON 类型**（`unwrapExtJSON`）：驱动把 `{"$oid": "…"}` 解成
  单键文档而不是 `ObjectID`，原样发到服务端就变成「拿子文档去匹配 ObjectID」—— 又是静默零命中。
  现在 `{"$oid": …}` / `{"$date": …}` / `$numberLong` / `$numberInt` / `$numberDecimal` / `$timestamp` /
  `$binary` / `$regularExpression` / `$minKey` / `$maxKey` 都会重建，但**同时是查询操作符的键不碰**
  （`{"$regex": "…"}` 仍旧是操作符，它的类型写法是 `$regularExpression`）。外壳里的值不合法时直接报错
  （网格里退化成「这是字面文本」），不会静默当子文档用。
- **shell 认 mongosh 的字面量**：`ObjectId(...)` / `ISODate(...)` / `new Date(...)` / `NumberLong` / `NumberInt` /
  `NumberDecimal` / `Timestamp` / `RegExp` / `UUID` / `BinData` / `MinKey` / `MaxKey`，以及 JS 裸键对象
  `{sku: "a"}`。字面量层只做「拼写重写」，值的解析与重建交给 `bson` 的扩展 JSON 解析加上
  `unwrapExtJSON`（外壳里的值合不合法由后者说了算）；`true` / `false` / `null` 与写错的裸标识符不会被
  顺手变成字符串。零参数的非确定写法（`ObjectId()`、`ISODate()`）报错而不是就地生成 —— 每次执行结果
  都不一样不是好事。
- **`createIndex` 不写名字时按 shell 规则补名**（`{"sku": 1}` → `sku_1`，`{a: 1, b: -1}` → `a_1_b_-1`）：
  `createIndexes` 命令要求 `name`，而 shell 是自动推出来的。
- **`Execute` 的库来自请求**，整段脚本共用一个库（shell 里没有 `use`），语句按顶层 `;` 与换行切分，
  多条语句都进 `Messages`，返回的表格是最后一条产生结果集的语句。
- **只读会话**由驱动自己拦：写命令表（`insert*` / `update*` / `delete*` / `drop*` / `createIndex*` …）命中即拒，
  干跑面板也会提前说「这几条会被拒」。

### 连接树

展开连接节点就是「连上它」：一次只发起一次加载，失败或取消后把节点**折叠再展开**
即重试一次，可以随时从右键菜单「Open connection」重连。
`loadedKeys` 直接交给 rc-tree（**不能过滤** —— 它会驱动 rc-tree 内部的 loaded 状态，
过滤掉一个 key 就等于让这棵树在每次渲染时重新加载，即死循环），「有没有数据」的判断
只在展开事件里用一次。加载失败的分支都带一个 Retry 按钮。

**只有「用户展开节点」才会建会话。** `loadData` 还会被一个「会话刚没了、节点恰好还开着」
的节点调到（断开之后就是这种状态），在那里顺手连接过 —— SQLite 的「Disconnect 没反应」
就是这么来的：会话确实关了，紧接着又被树重新拉起来一个甚至两个，节点看上去一直连着。
`openedByUser` 记下展开事件里的新 key，`handleLoadData` 只在命中它时才调 `connect()`，
一次消费一个。断开后节点会确实变成「Not connected — expand this node again to retry」，
再展开一次才是重连（右键的「Open connection」不用展开也照样直接连）。

右键菜单里的「Disconnect」**不再问一次**：配置存在本地，再连回来只是一下，被关掉的也只有这条连接的标签页。

右键菜单就是全部入口（底部没有 New / Connect 按钮）：

| 右键处 | 菜单 |
| --- | --- |
| 面板空白处 | 引擎列表（MySQL / MariaDB、PostgreSQL、SQLite、MongoDB、TiDB、Apache Doris） |
| 未连接的连接 | Open connection / Edit connection… |
| 已连接的连接 | New query / Refresh / New database… / Edit connection… / Disconnect |

菜单的浮层是 portal，但 React 的事件依旧顺着**组件树**冒泡，而这个浮层就挂在树节点下面 ——
不拦一下的话，点菜单项会顺手触发这一行的 `onSelect`，刚选的动作当场被「打开这个对象」盖掉
（「Open fields」会落到数据页，文件夹上的「New query」会多加一个对象列表）。`NodeMenu` 在
`menu.onClick` 里 `stopPropagation()`，菜单的点击就只是菜单的点击。

**双击连接节点**是「看它的运行情况」：没连上就先连（服务端要密码时先弹框，填完再自动打开），
已经有会话就直接切到那一页。一次点击（展开）保持原来的行为 —— 只连接、不开页面，
所以「连上了」和「去看看它现在在干什么」是两件事，不会互相打扰。

「New database…」只要一个库名，语句由 `quoteIdent` 按当前引擎拼好并**先展示再执行**
（`CREATE DATABASE …`，同样走 `ExecuteSQL`），SQLite 这类文件型引擎与只读会话直接禁用。

没有任何标签页时右侧就只有一页**项目信息**（`WelcomePane` 渲染 `AboutProject`），标题是应用名 + 版本
（版本只出现在标题里，徽章里不再重复）—— 这一页不再放快捷入口、连接卡片与引擎清单，因为那些在命令条与
连接树里已经有了。徽章分四组：顶部三枚说明**手上这个程序**（`license MIT`、`build just 1.58.0`、
`running windows/amd64`，最后一项来自后端 `AppInfo.platform`）；下面按组列出 `Build`（
一枚 `justfile` 徽章，值 `just build · just release · just publish`，点开是仓库里的 Justfile ——
它原来是右侧的一行文字，现在就是徽章组里的第一组，链接与排版都跟其它徽章一致）、技术栈（`Runtime` /
`Desktop and UI`，前端库版本直接读 `frontend/package.json`，徽章不可能写出包里没有的版本，Go 版本
同样来自 `AppInfo`）与 `Platforms` 说的是**项目发出去支持哪些平台** —— `windows amd64` /
`macos universal` / `linux amd64`，与 `.github/workflows/release.yml` 的构建矩阵一一对应，一套 Wails
代码三个平台。徽章是本地 CSS 画的灰标签 + 品牌色值（Linux 的黄底浅，值用深色字），不依赖 shields.io，
断网也照常显示；链接统一走 `openExternal`，桌面壳里交给系统浏览器，纯浏览器里退化成新标签页。
开发者一栏只画头像、不写名字，昵称放在悬停提示里（`Tooltip` 得挂在 DOM 元素上，包在 `ExternalLink`
外面会被组件吃掉 hover 事件），无障碍文本走 `aria-label`。**头像随程序发布，不在运行时去 GitHub 拿**
（`asserts/developer.png`，由 `lib/assets.ts` 注册），所以整页断网也能完整渲染，拿不到图时还有 antd
`Avatar` 的首字母灰底兜底；头像换了就按 `about.ts` 里 `avatarSourceUrl` 的注释重新下载一份。项目地址 /
作者 / 许可证 / 构建工具集中在 `frontend/src/lib/about.ts`（仓库地址、Issues、Releases、Justfile 链接
都由 `REPO_OWNER` / `REPO_NAME` 拼出来，改一处即可），其中三处要手动对齐：作者与 `wails.json` 的
`author`、`just` 版本与下面的「环境要求」表、`Platforms` 与 release workflow 的矩阵。

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

`internal/drivers/mongodb` 是纯单元测试加一组**默认跳过**的集成测试：连接串拼装、TLS 三档、
索引信息、shell 的词法 / 参数解析（`SplitStatements` / `ParseStatement` / 各种 JSON 值）、
BSON 值到单元格文本的映射、filter / sort 构造、定义脚本的往返、干跑分类都在不连库的情况下跑。
要在真实实例上跑那组集成测试时：

```sh
DMB_TEST_MONGODB_HOST=127.0.0.1 go test ./internal/drivers/mongodb/ -run Integration -v
```

它会建一个 `dmb_test_<纳秒>` 库、塞几条订单文档，跑完自己 `dropDatabase`，
覆盖连接与版本、集合与结构、分页与过滤、单元格更新与删行、脚本执行（多语句、`show`、
`createIndex`、只读拒绝）、运行情况页的每个分组。没设 `DMB_TEST_MONGODB_HOST` 时全部 skip，
所以 CI 不需要装 mongod。

`internal/drivers/mysqlcompat` 用自写的假 SQL driver（`internal/drivers/sqltest`，不引第三方 mock）
盯住「照 `SHOW` / `information_schema` 的结果拼出结构」这类纯映射：列类型与长度、可空、索引列、
系统库过滤、DSN 参数白名单与 `interpolateParams`、overview 取数。TiDB / Doris 各自的包还有表驱动
单测（端口、系统 schema、集群成员标签），真实实例同样要显式给地址才跑：

```sh
DMB_TEST_TIDB_HOST=127.0.0.1 go test ./internal/drivers/tidb/ -run Integration -v
DMB_TEST_DORIS_HOST=127.0.0.1 go test ./internal/drivers/doris/ -run Integration -v
```

两者默认 skip，CI 不需要备 TiDB / Doris。

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
  - [x] 运行情况：双击连接看服务端现状，每个引擎一个视图
- [x] Phase 2：MongoDB（连接、集合浏览、shell 查询、增删改、索引、运行情况）
- [x] Phase 2.5：TiDB / Apache Doris（同一种线协议共用 `mysqlcompat`：TiDB 带 TLS 与集群成员概览，Doris 只读浏览 + DDL 编辑）
- [ ] Phase 3：Oracle / SQL Server（占位与 dialect 已在，缺 DSN 与目录查询，`planned.Parked()`）
- [ ] Phase 4：SSH 隧道、导入向导、数据对比、插件式扩展

## 许可证

[MIT](LICENSE)
