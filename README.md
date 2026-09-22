# db-manager

使用 **Wails v2 + React + Vite + Justfile** 开发的关系型数据库管理工具，打包为 Windows 桌面应用。

支持 **MySQL / TiDB / Apache Doris / PostgreSQL / SQLite / MongoDB**；驱动层不假设关系模型，文档型引擎走的是同一套 `Driver / Conn / Dialect` 契约，service 与 UI 只问能力、不问引擎名。Oracle / SQL Server 的占位仍在（见 `internal/drivers/planned`），但暂不上菜单。

> 在本仓库写代码前先读 [`AGENTS.md`](AGENTS.md)：每次开发完成必须 commit + push，本地打包用 `just release`，发版走 `just publish <x.y.z>` 触发 GitHub Actions 打出三平台免安装可执行文件。

## 功能

- **连接管理**：连接配置的增删改查、连通性测试、SQLite 文件选择、TLS（CA / 证书 / 私钥）、自定义 DSN 参数、只读标记、颜色标签、密码可选保存。新建连接先点出**驱动菜单**（命令条 `Connection`、连接树的 `+`、面板空白处右键，三处挂的是同一份列表，就展开在你刚点的那个东西下面），再落到**这个引擎自己的那一页** —— 走网络的要地址、端口、账号与 TLS，SQLite 只要一个文件加一个附加库别名，两边不会互相看到无关字段（保存下来的配置也照着这一页来，文件型连接不会混进 host / port / ssl）；编辑已有连接同样按它的驱动打开对应那页。
- **对象浏览器**：会话 → 数据库 → Schema → 表 / 视图 / 索引 的懒加载树，**每个文件夹都带数量、空着也画**（`Tables (0)` / `Views (0)` / `Indexes (0)` —— 文件夹一空就消失，跟「这个引擎根本没有这种对象」就分不出来了），该画哪几个由后端按引擎声明（`DriverInfo.objectKinds`），前端不猜；右键菜单支持打开数据、新建查询、复制名称；文档型引擎这里是 database → Collections / Indexes，没有 schema 层，SQL 专属的入口（新建 DDL 脚本、ER 图、设计对象）自动不出现。
- **连接排序与分组**：整行拖着换位置，`+` / 空白处右键菜单里的 `New group…` 建一层深的文件夹，拖进去、拖出来、删掉文件夹（里面的连接会回到顶层，不会被一起删）都行；排法按 id 存在 `layout.json`，读与写都先拿实时配置对一遍，所以配置增删、换版本都不会让排列对不上号（按名字过滤时拖拽关闭 —— 那时候看得见的邻居不是布局里的邻居）。
- **新建数据库**：连接右键 `New database…`，问什么由**服务端**回答 —— MySQL / TiDB 给出字符集与可配的排序规则，PostgreSQL 给出编码与 locale（locale 名单不可能完整，那一栏可以手打），Doris 与 MongoDB 没有可选项、只有一句说明。语句由后端按引擎渲染后**先展示再执行**（MongoDB 下是 `use <db>`，并明说「第一条 collection 写进去之前它什么都不存」）；SQLite 这类文件型引擎不出现这个菜单项，只读会话里它是灰的。
- **MongoDB**：连接（含副本集多主机、`mongodb+srv`、TLS、认证库）、集合浏览（文档数 / 体积 / 索引）、数据网格的过滤排序与分页、双击改标量字段、批量删文档、索引列表与定义脚本、运行情况页（serverStatus + 每库 dbStats）。查询窗口跑的是 **mongosh 风格的 shell**（`db.orders.find({...}).sort({ts: -1}).limit(20)`），不是 SQL。
- **TiDB / Apache Doris**：两个引擎对客户端都讲 MySQL 线协议，但脾气各不相同。TiDB 默认端口 4000，有 TLS 页（`Security`），表单里的 `Database` 只是新标签页的默认库 —— 一个 TiDB 集群就是一份逻辑数据库，树里一次列全所有库，所以这一栏是可选的；运行情况是 MySQL 那一页再加一张 `information_schema.CLUSTER_INFO` 的集群成员表（tidb / tikv / tiflash / ticdc / pd）。Doris 默认端口 9030，前端（FE）与后端（BE）都在集群网内、MySQL 端也不做那套握手，于是**没有 TLS 页**；它是分析型引擎，本工具里**只读浏览** —— 表结构与索引照样能看（字段列表说的是引擎自己的类型拼写，如 `varchar(120)` / `decimal(10,2)`），但没有表设计器，改动走 DDL 编辑器。两者的连接、库表浏览、数据网格、SQL 查询与导出都复用 MySQL 那条代码路径。
- **对象列表**：点击树里的表 / 视图 / 索引文件夹，在右侧开出 Navicat 风格的对象网格（名称 / 类型 / 行数 / 大小 / 引擎 / 注释，索引列还有所属表 / 列 / 唯一性 / 主键 / 方法），支持列排序、列筛选、底部关键字过滤，单击打开对象、双击进入设计视图；打开或切回一个列表窗口时，连接树会跟着展开到它所属的库并选中那个文件夹。
- **数据网格**：分页、服务端排序、服务端过滤（14 种操作符）、列宽自适应、长文本悬浮预览、多选、行详情抽屉（JSON / INSERT 预览）。
- **行编辑**：双击单元格内联编辑、批量删除选中行；所有写操作都以主键为条件并**全部使用参数绑定**。
- **SQL 编辑器**：基于 CodeMirror 6，按驱动切换语言（MongoDB 用 JavaScript，其余用各自方言）、语法高亮与补全、多语句执行、执行历史、`Ctrl/Cmd+Enter` 执行全部、`Ctrl/Cmd+Shift+Enter` 执行选中。
- **查询计划（Explain）**：查询窗口工具条上的 **Explain** 不去执行，而是问引擎「这一条你打算怎么跑」—— 后端按 `Spec.ExplainSQL` 套上各自的包装（MySQL 一族的 `EXPLAIN`、PostgreSQL 的 `EXPLAIN`、SQLite 的 `EXPLAIN QUERY PLAN`），结果铺进和数据网格同一个表格，下半区用 `Results` / `Plan` 两档切换，正文上方原样印出真正发出去的那条语句；包装**永远不带 `ANALYZE`**，所以什么都没被执行 —— 只读连接能解释，`DELETE` / `UPDATE` 也能先看计划再决定跑不跑，面板上同时写明「这是估算、不是实测」。一次只解释**一条**语句：脚本会被回绝并说明找到几条（要解释哪一条就选中哪一条）；`DriverInfo.supportsExplain` 为假的引擎（MongoDB，它的语句是 shell 调用）按钮是灰的并说明原因，而不是点了才报错。
- **SQL 格式化**：工具条上的 **Format**（`Ctrl/Cmd+Shift+F`）按引擎的 SQL 文法重新缩进当前语句 —— 只有「关键字大写」这一个主张，其余只是空白；有选中就只格式化选中、没有就整篇，文法读不出来的脚本**原样不动**并说出错在哪（半格式化的脚本比不格式化更糟）。文法名与高亮一样由驱动决定（`frontend/src/lib/sqlFormat.ts`，走 `sql-formatter` 的文法表）；MongoDB 没有 SQL 可格式化，按钮是灰的。
- **结构查看器**：列、索引、外键、原始 DDL（优先使用引擎原生 DDL），DDL 可复制或导出；这份 DDL 是**带语法高亮的**（关键字 / 类型 / 字符串 / 数字 / 注释 / 引号里的名字各一色），高亮由前端自己扫一遍字符得到，不引第三方词法库，也不把脚本拼成 HTML —— 字符串字面量里的 `<img>` 就只是那几个字符；同一套读法还用在该语句出现的其它只读场合（保存前的语句清单、设计器右侧的预览、新建数据库的语句预览），并按引擎分别处理：PostgreSQL 的 `"…"` 是名字而 MySQL 的是字符串、`$tag$…$tag$` 整段算一个字符串（函数体就这么活下来）、MongoDB 的 shell 按 JavaScript 读（`--` 在那里是自减而不是注释）。文档型引擎的「结构」页是**抽样得到的字段表**（字段名 / 类型 / 是否可能缺失），并说明集合本身没有 schema。
- **表设计器**：表格窗口的「结构」页就是编辑器 —— 直接改字段名 / 类型 / NULL / 默认值 / 主键 / 自增 / 注释，增删索引，右侧实时渲染将要执行的 SQL（同样带语法高亮）与引擎限制警告；保存前无需联网猜测，保存时逐条执行并如实报告「第几条失败」（MySQL / PostgreSQL / SQLite 各自的限制都写在警告里）。**建表走同一条路**：连接树里「表」文件夹右键的 `New table…` 开一个空设计（一个主键字段起步，表名就在工具条上敲），预览与保存用的还是同一个规划器 —— 只是把实时目录换成空基线，`ALTER` 换成 `CREATE`；建完这个窗口会变成刚建好的那张表，停在「结构」页。引擎给不出设计器的（MongoDB、Doris）这一页退化成**只读字段列表**并直说「这个引擎没有表设计器」，不摆一个按下去会失败的按钮。
- **查询收藏**：查询窗口工具条上的「Favourites」可以把当前 SQL 命名保存（默认用第一行非注释文本作名），下拉里一键载入、重命名或删除；收藏存在 `queries.json` 里，与连接配置互不影响，换窗口、换连接都能用。
- **保存的查询**：每个库下面有一个 `Queries` 文件夹（排在 `Tables` / `Views` 前面），右键「New query…」新建脚本、点一下就开在查询窗口里 —— 脚本落成 `<数据目录>/.query/<连接>/<库>/<名字>.sql`，窗口里改了字标签页上出现 `*`，`Ctrl/Cmd-S` 存回文件，关掉没保存的窗口会先问一句；重命名是**纯文件移动**（从不读写内容，所以树里改名不会覆盖窗口里没保存的编辑），删除会先确认并说明「已经打开的那个窗口里的文字会留下」。脚本跟着**数据目录**走，换目录时 `.query/` 整棵树一起搬。
- **ER 图**：在 schema（没有 schema 层的引擎就是 database）节点右键即可打开该命名空间的关系图 —— 一张 `GetSchemaGraph` 就把对象、字段与它们之间的外键取回来；**同一家的表排成一行**，行的归属由名字决定：一个名字的行 key 是它最短的那段「读起来像一家人的」前缀 —— 要么本身就是某张表的完整名字（`t_user_favorite` 归到 `t_user`），要么是至少两张表共同的开头（`xxx_dict_data` 与 `xxx_dict_env` 归到 `xxx_dict`，哪怕库里根本没有 `xxx_dict` 这张表）；于是 `t_user` / `t_user_favorite` / `t_user_profile` 一行，`t_order` / `t_order_payment` 一行，而 `t_product` 这种没人同族的自己占一行 —— 单个词只有当真有表叫这个名字时才算一家（否则库里都叫 `t_…` 的表会被挤成一条长龙；真有一张表就叫 `t` 的话，它们就是这一家）；**行内**再按外键层级从左到右排，所以行内的箭头仍指向被引用的那张表，跨行的外键就随它斜着走（往左上走的那根从左边缘绕出去，不穿过自己这个框）；主键高亮，箭头悬停显示「哪一列引用哪一列」；支持拖动平移、滚轮缩放、按名搜索、隐藏/显示字段、网格开关，点节点直接打开该表，还能把当前这张图导出成自包含的 SVG。跨命名空间的外键画成虚线 stub，读不到字段的对象仍在图里但会列出警告，超过 300 个对象时明确提示只画了前 300 个。
- **DDL 编辑器**：把对象的定义开成可编辑的脚本窗口（MongoDB 下就是集合的定义脚本与 shell 查询） —— 工具条、对象树右键「Edit DDL…」或结构页的「Edit in DDL editor」都能进；编辑器下方是**后端算出来的干跑结果**（逐条语句标出 query / DDL / DML、标红 DROP、TRUNCATE、无 WHERE 的 DELETE/UPDATE，MongoDB 下则是 `drop()` / `dropDatabase()` / 无 filter 的 `deleteMany`，并说明只读连接会拒掉几条），真正点「Run script」时只对破坏性脚本弹二次确认；执行完顺手刷新目录树与索引缓存。
- **窗口即应用**：右键不再弹出 WebView 自带的那套菜单（后退 / 刷新 / 另存为 / 打印 / 检查），
  右键要么什么都不做，要么就是应用自己的菜单（连接树等）；文本框与 SQL 编辑器是例外 —— 那里保留系统菜单，
  右键粘贴照旧可用，其它地方用 `Ctrl+C` / `Ctrl+V`。
- **运行情况**：双击连接节点即可打开该连接的「运行情况」页（未连接会先连上，密码框填完再自动打开）；页面上半部分是会话事实与快照时间，下面按引擎各画各的 —— MySQL 给进程列表、连接数、InnoDB 缓冲池与命中率（TiDB 走同一页，外加集群成员表；Doris 也尽力取这一套，取不到的项挂进警告里），PostgreSQL 给后端/活动会话/数据库体积与提交率、缓存命中率，SQLite 则是「这是一个文件」的视角（路径、落盘大小、pragma、对象清单与 ATTACH 进来的库）。读不到的项一律显示 `—` 并附一条警告（缺权限、缺统计视图），不会拿 0 冒充；工具条的刷新按钮重新取一次快照。
- **项目信息**：没连库时右侧只有一页项目信息，标题就是 `DB Manager` 加当前版本（`v0.1.0`，字号比正文大一档）—— 标题下面一排徽章（`license MIT`、`build just 1.58.0`、`running windows/amd64`），再按组列出 **`Build`**（一枚可点的 `justfile` 徽章，值就是三条常用配方，点开是仓库里的 Justfile）、技术栈（`Runtime`、`Desktop and UI`）与 **`Platforms`**（`windows amd64` / `macos universal` / `linux amd64`，和 `.github/workflows/release.yml` 的构建矩阵一一对应）—— 徽章是 shields 样式的灰标签 + 品牌色值，前端库版本直接读 `frontend/package.json`，Go 版本取自运行中的二进制，再下面依次是项目地址 / Releases / Issues 和开发者（只画一个 GitHub 头像，悬停显示昵称、点一下打开 `github.com/freewu`）。徽章用本地 CSS 画，离线也能渲染；链接交给系统浏览器（`window.runtime.BrowserOpenURL`），不把整个窗口导航走。连库的入口在命令条的 Connection，以及连接树的右键菜单。
- **导出**：CSV / JSON / INSERT 脚本（MongoDB 下是 `insertMany` 脚本，按列的 BSON 类型还原 `$oid` / `$date` / 文档字面量），可写入文件或复制到剪贴板。
- **外观**：Navicat 式窗口骨架（icon-over-label 命令条 + 连接树 + 标签页工作区 + 状态栏）、明暗主题、品牌绿 `#36ab60`、可拖拽分栏、紧凑的表格与状态栏 —— 表格的表头**固定不动**：数据网格、对象列表、设计器的字段表都一样，纵向滚动时表头留在容器顶上，横向滚动也带不走它（钉住的列仍旧钉在原处）。主题有三档：`Light` / `Dark` / `System`，在命令条的 **Settings → Appearance** 里选，状态栏那格点一下也能循环切换（跟随系统时它会写成 `system (dark)`，把当前系统给的那一档一起说出来）；选**跟随系统**时窗口真的跟着操作系统走 —— 操作系统在运行期间切深浅色，界面当场就变，不用重启也不用再点一次。命令条上目前只有 **Connection**、**Open**、**Close**、**Refresh**、**Table**、**View** 与 **Settings** 是活的，其余按钮保持原来的位置但禁用并在提示里说明 —— 摆着的空位比消失的按钮更好认。
- **设置**：命令条上的 **Settings** 开一个三页的窗口 —— `Appearance` 选主题，`Data folder` 看数据存在哪、里面有哪些文件（每个文件写的是干什么的、多大）、从这里**打开目录**、**换一个目录**或**恢复默认**，`About` 就是原来那页项目信息（技术栈徽章、项目地址、开发者）。换目录是真的**把数据搬过去**：连接配置、查询收藏、树的排法、窗口状态与密码密钥一起复制到新目录（逐个读回校验，对不上就不动原文件），然后才改指针、最后才删旧文件；没能删掉的（被占用、只读）会**如实列出来**，而不是回一句「已移动」。目标目录里已经有**非空的**本程序数据时会被拒绝并点名是哪个文件（同名但零字节的不算数据，照常覆盖），选到当前目录本身也会被拒绝；那些**不是本程序写的**文件留在原地并在结果里注明。目录就绪后不需要重启 —— 会话照旧连着，新的保存当场写进新目录。
- **项目信息**：没连库时右侧只有一页项目信息，标题就是 `DB Manager` 加当前版本（`v0.1.0`，字号比正文大一档）—— 标题下面一排徽章（`license MIT`、`build just 1.58.0`、`running windows/amd64`），再按组列出 **`Build`**（一枚可点的 `justfile` 徽章，值就是三条常用配方，点开是仓库里的 Justfile）、技术栈（`Runtime`、`Desktop and UI`）与 **`Platforms`**（`windows amd64` / `macos universal` / `linux amd64`，和 `.github/workflows/release.yml` 的构建矩阵一一对应）—— 徽章是 shields 样式的灰标签 + 品牌色值，前端库版本直接读 `frontend/package.json`，Go 版本取自运行中的二进制，再下面依次是项目地址 / Releases / Issues 和开发者（头像加昵称整个是一条链接，昵称就是显示出来的文字，也是链接的标题，点一下打开 `github.com/freewu`）。徽章用本地 CSS 画，离线也能渲染；链接交给系统浏览器（`window.runtime.BrowserOpenURL`），不把整个窗口导航走。这页在没连库时显示，**Settings → About** 里也是同一份（同一个组件，两处不会各说一套）。连库的入口在命令条的 Connection，以及连接树的右键菜单。
- **托盘**（Windows）：关掉窗口是**收进托盘**，不是退出；托盘图标左键单击等于把窗口叫回来，右键的菜单从上到下是 `Show window` / `Project page` / `Report an issue` / 版本号（灰色，只是给你看的）/ `Quit`。`Quit` 走的是正常退出（连接池照常关），而**图标没能建出来时关窗照旧直接退出** —— 一个没能出现的托盘不该留下一个看不见的进程。其它平台没有通知区域，行为保持原样（关窗即退出）。

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
just release     # 本地打包：构建 + 归档到 release/（带校验和），不碰 git
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
├── tray.go                   # 托盘菜单的内容与动作（文字 / 顺序 / 版本号只有这一处）
├── tray_windows.go           # 纯 Win32 的托盘：隐藏窗口 + 自己的消息循环 + Shell_NotifyIcon
├── tray_other.go             # 非 Windows 的空实现：没有通知区域，关窗照旧退出
├── AGENTS.md                 # 开发约定：提交 / 发版 / 环境坑（动手前必读）
├── Justfile                  # 开发任务（windows-shell = PowerShell）
├── wails.json                # 版本号权威来源（info.productVersion）
├── asserts/                  # 品牌素材的唯一来源：logo.png、icon/<引擎>.png
├── scripts/
│   ├── version.mjs           # 版本号同步与一致性校验
│   ├── package.mjs           # 本地打包：归档到 release/ 并生成 checksums.txt
│   └── release-notes.sh      # 渲染 GitHub Release message
├── .github/workflows/
│   ├── ci.yml                # main / PR：go vet+test、tsc+vite build、素材一致性
│   └── release.yml           # tag v*：三平台可执行文件 + GitHub Release
├── release/                  # `just release` 的产物（已 gitignore）
├── build/                    # 图标、清单、安装包脚本（appicon.png 由 `just icons` 同步）
├── internal/
│   ├── apperr/               # 错误码 + 脱敏（打码 password=... 与 URI userinfo）
│   ├── config/location.go    # 数据目录的指针（location.json）与搬家：复制 → 校验 → 改指针 → 删旧文件
│   ├── config/store.go       # %APPDATA%/db-manager/{connections,queries,state,layout}.json（0600）
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
        │   ├── AboutProject.tsx   # 项目信息（技术栈徽章 / 项目地址 / 开发者），空态页与 Settings → About 共用
        │   ├── SettingsDialog.tsx # 设置窗口：外观 / 数据目录 / 关于
        │   └── overview/     # 运行情况：每个引擎一个视图 + 共用的指标卡片与数据表
        ├── connection/       # 每种驱动一页连接表单（Mysql / Postgres / Sqlite / Mongodb / Tidb / Doris）+ 注册表
        ├── hooks/useConnect  # 先试后问的连接流程
        ├── lib/              # tree key 编解码、格式化、导出、驱动能力（capabilities.ts）、项目信息（about.ts）、主题（theme.ts）、品牌素材（assets.ts，@asserts 别名）
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
查询窗口的 Plan 视图问的是 `drivers.Explainer`：`sqlbase.Conn` 统一实现，**具体发哪句由 `Spec.ExplainSQL`
决定**（没有这个钩子的 spec 直接回 `unsupported`，不必连一次服务器才知道），而它只允许带上不执行语句的
包装（`EXPLAIN` / `EXPLAIN QUERY PLAN` 这些），`EXPLAIN ANALYZE` 那种会真跑一遍的形式不在这里出现。
「建库」则是 `drivers.DatabaseCreator`：两种方法（`DatabaseOptions` 读出这台服务器接受什么、
`CreateDatabase` 渲染语句）在所有 SQL 引擎上都存在，但**支不支持由 `Spec` 里的钩子决定** ——
SQLite 两个钩子都是 nil，`DatabaseOptions` 于是明确回 `unsupported`，
MySQL 一族换成 `SHOW CHARACTER SET` / `SHOW COLLATION` 的读法、PostgreSQL 换成自己的编码与 locale，
Doris 与 MongoDB 只回一句 hint（前者没有库级字符集，后者根本没有 CREATE DATABASE）。

**「引擎能不能做这件事」由后端说，前端只读结果**：`DriverInfo.relational` / `supportsDatabase` /
`supportsSchema` 决定界面上出现什么，`supportsDesign` 说明这个引擎有没有表设计器（MongoDB、Doris 为
`false`）——「结构」页据此换成只读字段列表：有列不等于这个引擎能被这个设计器编辑，如实说「没有设计器」
比给一个点了会失败的按钮好。`supportsExplain` 说的是同一类事实的另一面：它的语句能不能被问出一份计划
（MongoDB 同样为 `false`，`db.orders.find()` 是 shell 调用，没有计划可读），查询窗口据此决定 Plan 那一档
要不要出现。`objectKinds` 是同一类事实的第四项：这个引擎装得下哪几种对象
（MySQL / TiDB / Doris / SQLite 是表与视图，PostgreSQL 多一个物化视图，MongoDB 是集合加视图），
浏览器的文件夹就照着它画。前端读的是同一份结果（`frontend/src/lib/capabilities.ts`），不按引擎名写 if ——
`driver !== 'mongodb'` 这种判断只对一次，再加一个文档型引擎就全是洞。

### `sqlbase` 用一个包实现全部 SQL 引擎

MySQL / PostgreSQL / SQLite 仅声明一份 `Spec`（`DSN` 构造函数、`Dialect`、目录查询实现），
即可获得：连接池（每库一个池）、分页、`COUNT(*)`、多语句脚本执行、值类型归一化
（`[]byte` → UTF-8 或 `0x…`、`time.Time` → RFC3339Nano）以及 DDL 渲染。
`SQLServerDialect` / `OracleDialect` 已预置。

### 讲同一种线协议的引擎共用一个包

TiDB 与 Doris 对客户端而言都是 MySQL，于是 `internal/drivers/mysqlcompat` 收下了这一族的全部共用件：
DSN（含 TLS 白名单与 `interpolateParams`）、一份 `sqlbase.Introspector` 实现（照 `information_schema`
与 `SHOW` 拼目录）、原生 DDL、`SHOW` 输出解析（建库窗口的字符集 / 排序规则也读同一批 `SHOW`，见
`mysqlcompat/database.go`），以及 overview 的公共外壳。引擎自己的包只回答差异：
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
   这份列表挂在三个入口上：命令条 `Connection`、连接树的 `+`、
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
| SQL | shell | `db.<coll>.<cmd>(...)`、`db.getCollection("…")`、`db.<cmd>(...)`、`show dbs\|collections`、`use <db>` |

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
- **`Execute` 的库来自请求**，脚本里还可以用 **`use <name>`** 把后面的语句换到另一个库（就是 shell 里那条，写进脚本能跑，`New database…` 渲染的也是它）；`use` 不到服务端，它只改这段脚本的库，其余语句按顶层 `;` 与换行切分，多条语句都进 `Messages`，返回的表格是最后一条产生结果集的语句。
- **MongoDB 没有 CREATE DATABASE**：库是个命名空间，第一条 collection 写进去时它才真的存在。所以「新建数据库」窗口渲染的是 `use <name>`，并在窗口里直说「此时还没有任何东西落盘」；库名在这一层就按 MongoDB 自己的规则校验（不能含 `/ \ . " $ * < > : | ?` 与空格）——这里的名字没有引号可以躲，`use a; db.dropDatabase()` 不能变成两条语句。
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
| 库下面的 `Queries` 文件夹 | New query… / Reload queries |
| 一个已保存的脚本 | Open / Rename… / Copy name / Show in folder / Delete |

**每个文件夹都带数量，空着也照样画。** 画哪几个文件夹是引擎事实，由后端在 `DriverInfo.objectKinds` 里给
（顺序也是它的：MongoDB 先集合后视图），前端只管画。早先的规则是「对象列表里出现过的种类才画文件夹」，
于是空库展开之后是一片空白 —— 分不清「这个库是空的」和「这个引擎没有这种文件夹」，而「是空的」
恰恰是用户想知道的事。索引装的是整个 namespace 的索引，是**另一次请求**，所以它跟着对象列表一起取：
展开数据库 / schema 时两个请求一起发，索引数量到了就在标题里补上，之后点开索引文件夹（或表的索引页）
是现成的；「Reload index list」仍是强制重取的那个入口。空文件夹在树里是**叶子** —— 标题里的 `(0)`
已经把话说完了，箭头点开只会是空的 —— 但点它照样打开（空的）对象列表。

**每个库下面还有一个 `Queries` 文件夹**（见下一节），它排在这几个文件夹前面：脚本是本地文件，不是库里的
对象，所以它跟着库走而不是跟着 schema 走 —— PostgreSQL 的库即使有多个 schema，同一份脚本也只在这一处
（一条连到库上的脚本，从每个 schema 都该够得着；这是相对「对象挂在 schema 下」的**有意偏差**）。这个
文件夹**必须等对象列表回来之后才挂上去**：rc-tree 只对「没有 children 的节点」调 `loadData`，一个原本
要加载、却突然被塞进了一个（文件夹这个）children 的节点，就再也不会去加载它的表了 —— 所以
`withQueries()` 在没有对象列表时仍然返回 `undefined`（「知道了再问我一次」）。

菜单的浮层是 portal，但 React 的事件依旧顺着**组件树**冒泡，而这个浮层就挂在树节点下面 ——
不拦一下的话，点菜单项会顺手触发这一行的 `onSelect`，刚选的动作当场被「打开这个对象」盖掉
（「Open fields」会落到数据页，文件夹上的「New query」会多加一个对象列表）。`NodeMenu` 在
`menu.onClick` 里 `stopPropagation()`，菜单的点击就只是菜单的点击。

**双击连接节点**是「看它的运行情况」：没连上就先连（服务端要密码时先弹框，填完再自动打开），
已经有会话就直接切到那一页。一次点击（展开）保持原来的行为 —— 只连接、不开页面，
所以「连上了」和「去看看它现在在干什么」是两件事，不会互相打扰。双击是个动作而不是
选中文字，所以**整棵树的标题都不参与文字选择**（`.dm-sidebar-tree .ant-tree-title { user-select: none }`）
—— 树的「选中」由树自己画，双击不该顺手选中半个名字。这条规则一开始只加在连接节点上，
于是轮到文件夹（`Tables` / `Views` / `Indexes`）双击时又得补一遍；要复制名字的话，每行的右键菜单里就有
（错误提示那种行内文字仍可从它自己的悬浮提示里选）。

**命令条上的 `Open` / `Close` / `Refresh` 跟着树里选中的那条连接走。** 点中一个还没打开的配置，
`Open` 亮起来，点一下就用它开一个会话（和右键的「Open connection」、双击节点同一条路径，
要密码就弹密码框）；点中一个已经打开的连接，`Close` 与 `Refresh` 亮起来 —— 前者断开它、
后者重取它的目录。store 里这两个 id 是一起走的：`activeSessionId` 说的是「哪个会话还活着」
（状态栏、查询标签页关心的就是这个），`activeConnectionId` 说的是「资源管理器在看哪条连接」，
后者可以是一个还没有会话的配置 —— 这正是 `Open` 能亮起来的原因。

**同一个焦点也管着 `Table` 与 `View`：点中一个库（或它的 schema、它下面的文件夹、里面的一张表），
这两颗按钮亮起来，点 `Table` 就开出这个 namespace 的 `Tables` 列表、`View` 开出 `Views` 列表**
—— 和树里点开对应文件夹是同一个窗口（同一个 tab id，先开过就直接切过去），数据也走同一份缓存。
store 里存的是 `activeNamespace`：引擎有 schema 时是那一层，没有 schema 时就是库自己
（MySQL / SQLite 的库就是 namespace）。两种情形按钮**不亮**并且在提示里说实话：
选中的是一个 PostgreSQL 的库节点（它的对象在下面的 schema 里，库本身不是一个列表，
硬挑一个 schema 出来只会开错窗口），或者这个引擎根本没有这一类对象
（MongoDB 没有表 —— 这一类判据来自后端 `DriverInfo.objectKinds`，不是在界面上按名字猜的）。
关掉会话时这个记录会一起清掉：库节点已经不在树上了，按钮不能还指着它。

**反过来也通：打开（或切到）一个列表窗口，树会跟着走过去。** 展开这条连接 → 库 →
schema（连接收在分组文件夹里的话，那条文件夹也一并展开），选中对应的那个文件夹
（`Tables` / `Views` / `Indexes` 都算），并把它滚进视野。请求由 `openObjectsTab` 与
`setActiveTab` 发出（命令条上的 `Table` / `View`、标签页的点击与切换都经过它们），所以
「命令条刚开的窗口」和「标签页里早就开着的窗口」行为一样：看哪个列表，树就指着哪个文件夹。
这条请求（store 里的 `reveal`）是**单向**的 —— 只有树自己消费它、改自己的 `expandedKeys` /
`selectedKeys`，它绝不回写 `activeNamespace`，因此不会和「命令条跟着树选中走」互相触发成
死循环。路上哪一层说实话地失败（库 / schema / 对象列表报错，或者库列表里根本没有这个库），
请求就作废，而不是悬在那里等一个永远不会出现的行。

**连接列表可以自己排：整行拖着换位置，分组是一层深的文件夹。** 排法不写在连接配置里，而是
**按 id 记在另一份文件**（`layout.json`）：配置的保存会把 `connections.json` 整份重写，排法搁在
里面早晚被顺手抹掉；`state.json` 也不行 —— Go 那边的 `SaveState` 是整块覆盖，读-改-写必然打架。
读和写都先拿实时配置对一遍（`internal/service/layout.go` 的 `placeProfiles`，读和写**同一条路**，
跟结构页「预览与保存同一条代码路径」一个道理）：布局里没提到的配置**追加在末尾**而不是消失
（旧版本写的配置、别人加进来的配置都还在），布局里提到的、已经没了的配置直接丢掉，指向已删分组的
连接回到顶层而不是跟着一起没。顶层是**一条**顺序，分组和没进分组的连接共用它 —— 这才是
「一个连接在一个文件夹上面、另一个连接在它下面」表达得出来的原因；分组里面是它自己的一条 0..m-1
顺序。前端 `lib/explorer.ts` 是这套规则的镜像，同一份防御：认不出的落点当作没动。拖完先画上新排法
再落盘，后端拒绝就退回去 —— 画出来的永远是真正存下来的那个样子。

几条规则是有意的：**分组只有一层**（「那台服务器在哪」不该变成一次搜索，也没有哪个引擎的目录
需要更深的组织）；**删分组不删里面的连接**（按原相对顺序回到顶层，确认框里会说清楚几个）；
**改名不动位置**（位置是用户选的，打字不是搬家的意思）；**按名字过滤时不许拖**（筛出来的邻居
不是布局里的邻居，那个落点没有意义，宁可关掉拖拽）；**拖出分组要落到分组最后一行的下面** ——
行的下半截仍然「在这一行之后」（那是把成员挪到分组末尾的手势），越过行底边才是出分组，落点是分组
自己所在的这一层：树只报「在这一行的上 / 中 / 下」，不报指针落在行的哪儿，所以在捕获阶段听一次
`dragover` 把指针位置拿住，这条分界交给 `lib/explorer.ts` 的 `dropTargetFor`，提示线也跟着缩进一格
（否则它画在成员那一层，而落点在分组那一层）；**落到最后一行以下的空白 = 顶层末尾**（那儿没有行，
树什么也不报，由树容器自己接）；名字为空或过长由后端发言（`apperror`），
前端不自己发明规则。选中一个分组会把命令条的对象按钮熄掉（文件夹既不是连接也不是库），
但**上次点中的连接仍是焦点**，`Open` / `Close` / `Refresh` 照旧能用。新建入口只有一处：`+` 与
空白处右键菜单里，驱动列表下面跟着一条 `New group…`；分组自己的菜单里也有 `New connection…`
（同一份驱动列表，选完的配置直接落进这个分组）。

「New database…」问的是**这台服务器自己能接受什么**，而这份清单来自服务端：MySQL / TiDB 用 `SHOW CHARACTER SET` /
`SHOW COLLATION` 报出字符集与它可配的排序规则（服务器自己的默认值排在最前并预选），PostgreSQL 报出
编码与 locale（编码是一份引擎常量，locale 只能列出这个集群用过的几个，所以那一栏**可以手打**），
Doris 没有库级字符集、MongoDB 根本没有 `CREATE DATABASE` —— 这两种弹框里就只有库名加一句说明。
语句一律由后端按引擎渲染（MySQL 一族 `CREATE DATABASE … DEFAULT CHARACTER SET … COLLATE …`、
PostgreSQL `… ENCODING '…' LOCALE '…' TEMPLATE template0`、MongoDB `use <name>`），
**在窗口里先展示再执行**，前端只把请求递过去、不拼 SQL。SQLite 这类文件型引擎直接没有这个菜单项，
只读会话里它是灰的。

没有任何标签页时右侧就只有一页**项目信息**（`WelcomePane` 渲染 `AboutProject`；命令条 **Settings → About**
里挂的是同一个组件，两处不会各说一套），标题是应用名 + 版本
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
开发者一栏是头像加昵称，整个是一条去 GitHub 主页的链接，昵称同时也是链接的标题（悬停能看到完整地址）；
无障碍文本走 `aria-label`。**头像随程序发布，不在运行时去 GitHub 拿**
（`asserts/developer.png`，由 `lib/assets.ts` 注册），所以整页断网也能完整渲染，拿不到图时还有 antd
`Avatar` 的首字母灰底兜底；头像换了就按 `about.ts` 里 `avatarSourceUrl` 的注释重新下载一份。项目地址 /
作者 / 许可证 / 构建工具集中在 `frontend/src/lib/about.ts`（仓库地址、Issues、Releases、Justfile 链接
都由 `REPO_OWNER` / `REPO_NAME` 拼出来，改一处即可），其中三处要手动对齐：作者与 `wails.json` 的
`author`、`just` 版本与下面的「环境要求」表、`Platforms` 与 release workflow 的矩阵。

### 保存的查询

每个库的 `Queries` 文件夹下面就是这台连接在这个库里的脚本，一个脚本一个 `.sql` 文件，落在数据目录里：

```
<数据目录>/.query/<连接 uuid>/<库名>/<脚本名>.sql
```

**文件夹名是分隔符安全的**：连接 id 与库名来自外部（用户的库名可能是 `a/b`、`C:temp`、`..`），
文件名又是用户自己起的，所以三段都过一遍 `escapeSegment` —— 可逆的 `%XX` 编码，`%` 自己先被编掉，
控制字符、Windows 不接受的字符（`/ \ : * ? " < > |`）、开头的点、结尾那一串点与空格全部编码；Unicode
**原样保留**（`报表.sql` 就是 `报表.sql`），因为要编码的只是「文件系统会误读」的那些字符。Windows 的保留
设备名（`con`、`nul`、`com1`…）在**每个平台上**都编码第一个字符，这样同一份数据目录换台机器同步过去
名字仍然一致。名字超过 `maxSegmentBytes`（200 字节）是**拒绝**而不是截断 —— 截断会让两个不同的名字悄悄
变成同一个文件。反解（`unescapeSegment`）遇到非法 UTF-8 就把原样的字符串还回去，不去猜。

**重命名是一次纯文件移动，从不读写内容。** 从树里改名只动文件名：那个窗口里可能还有没保存的编辑，
如果后端顺手读一遍再写回去，就等于让树把用户正在写的东西覆盖成它看到的旧版本。同理，「新建查询…」
是**写一个空文件**并且拒绝重名（包括只差大小写的重名），否则在已有脚本上新建会把它清空。

大小写不敏感的文件系统上，`Orders.sql` 与 `orders.sql` 不可能共存，所以保存的路径是：写 `.tmp` → 删掉目标
→ rename → 再删掉只差大小写的那个旧拼写。**最后这一步必须先 `os.SameFile` 确认不是同一个文件** ——
要不然「旧拼写」正好指回刚写好的文件，脚本当场被删掉（这个 bug 是写测试时抓到的）。Windows 的
`os.Rename` 不覆盖已存在的目标，所以中间那一步不能省。

窗口这边：打开就是**读文件**（关掉再开、重启之后看到的都是磁盘上的内容，包括别处编辑器改过的），改了字
标签页上出现 `*`，**Ctrl/Cmd-S** 写回去（`SqlEditor` 的 `onSave`，没有地方可存的时候这个快捷键干脆不
注册，而不是假装保存过）；改名**不重建**窗口（标签页的 key 不含文件名），否则没保存的编辑会跟着窗口一起
没了；关掉一个还没保存的窗口会先问一句。删除是可重入的（文件已经不在也算成功）、只删文件不删文件夹，
已经打开的那个窗口会留着它手里的文本，但下一次保存会把文件重新写出来 —— 菜单里的确认框就是这么说的。

一个脚本对应一个窗口：标签页的 id 由「连接 + 库 + 名字」拼出来，所以同一个脚本打开两次是切回原来那一页，
而不是两个窗口抢同一个文件。

### 设置：主题与数据目录

设置窗口自己**不存**任何偏好：主题经过 store 落进 `state.json`，数据目录本来就是后端的事，所以关掉窗口
不会丢选择，两个窗口看到的也永远是同一份值。主题分成**偏好**（`light` / `dark` / `system`）与**画出来的**
两件事 —— 偏好写进状态文件的东西就是它自己，跟随系统时「现在到底什么颜色」另算（`resolvedTheme`），
页面据此给自己加 `data-theme` 与 `color-scheme`；系统那边的监听器只在这条偏好是 `system` 时才真的会
改变颜色（`resolveTheme` 对另外两档直接忽略系统），所以来回切主题不会积累监听器，旧版本留下的
`light` / `dark` 状态文件也照样读得进来（认不出的值回到 `light`，Navicat 的经典模样）。

**数据目录的指针不在数据目录里**，而在默认位置（`%APPDATA%/db-manager`）的一张
`location.json`：`{"version":1,"dataDir":"…"}`。理由很直接 —— 数据目录本身要先被读出来，指针
不能跟着数据一起搬走，否则读指针的人得先知道指针在哪。文件走 tmp + rename 原子写：半张指针会被读成
「没有指针」，那就等于把用户悄悄送回默认目录。内容读不成（空、坏、相对路径、没有这个字段）一律回落
到默认目录，这个文件坏掉不该让程序打不开。**在指针里写下默认目录 = 清掉指针**，这样以后默认位置换了，
用户仍然跟着走。

搬家的顺序是**先复制再删**：逐个文件读出来、写到新目录、再读回来逐字节比一遍（`copyVerified`），五份
数据（连接 / 收藏 / 排法 / 状态 / `secret.key`）都到位之后才改指针，最后删旧文件；删除是尽力而为，
删不掉的留在原地并出现在结果里（`remaining`），绝不说一句「已移动」了事。所以任何一步失败，原目录都是
完好的（最坏只是多出一份没人用的副本）。拒绝的三种情况：目标就是当前目录（`sameDir` 认符号链接、
Windows 上还认大小写）、相对路径、以及目标里已经有**非空的**本程序数据（点名是哪个文件）。目录**可以
互相嵌套** —— 只搬名单里的文件、从不递归，所以谁在谁里面都行。不是本程序写的文件（`LeftBehind`）留着
不动并如实报告，其中不算指针文件与 `*.tmp`；目标目录如果正好在源目录里面，也不会被误报成「留下没搬」。
名单写死在 `dataFiles`（五个文件）与 `dataDirs`（`Queries` 文件夹那棵树，即 `.query/`），
**新增一份数据文件 / 文件夹必须同时加进对应的名单**，否则搬家会把它落下（并因此在结果里报出来）；
目录是按文件逐个复制校验的（`copyTree` + `copyVerified`，跳过 `*.tmp`），目标目录里已经有一份非空的
`.query` 同样算「已有本程序数据」而被拒绝。不这么搬的话，换一次数据目录就会把用户存的脚本全部丢在原地。

搬完不重启：`MoveDataDir` 会重新 `config.New()` 并把新的 store 换进 `Manager`。整个搬家过程持写锁
（`m.mu.Lock()`，`m.storeRef()` 读锁）—— 否则这一步复制、下一步删除之间落进来的一次保存会丢在旧
目录里；已经连上的会话不受影响，它们手里是 driver 连接不是 store。指针改完但 store 换不过去（目录被
外力弄没了）时错误信息会说**数据已经搬走、重启一下**，而不是谎称失败。

### 运行情况从哪来

`drivers.Overviewer` 是可选接口，`sqlbase` 提供唯一一份实现，把会话事实（名字、驱动、
版本、只读、连接时刻）与耗时交给 service 填，各引擎只往 `Spec.Overview` 里挂一个收集器。
「引擎回答不了这个页面」不是错误，而是一条警告（页面照常渲染会话信息），只有真正取不到
数据（连接断了）才报错 —— 所以 `CodeUnsupported` 在 service 里被降级成 warning。
读不到的指标一律是 `-1` / `—`，绝不用 0 冒充；每一条子查询（pg_stat_activity、
`pg_database_size`、`SHOW FULL PROCESSLIST` …）失败都只降级成自己的那条警告，
一个权限不足不会让整页白掉。

### 托盘只有 Windows 有

Wails 不提供托盘 API（`runtime` 里只有窗口、对话框与事件），所以这一小块是自己写的：
`Shell_NotifyIcon` 只有 Win32 有，走 `golang.org/x/sys/windows` 直接调，不引 cgo，也不引第三方
托盘库（那些库在 Linux 上要么要 cgo + appindicator，要么根本没有统一的通知区域）。

托盘事件是 shell **post 到一个窗口**上的，而应用主窗口归 Wails，所以自己建了一个隐藏窗口
（类名 `DBManagerTray`）；Win32 的窗口属于创建它的那个线程，因此这个窗口连同它的消息循环
都在**自己的线程**上，不去抢 Wails 那条已经有消息循环的主线程。窗口只做两件事：收 `trayMessage`
（左键 = 显示窗口，右键 = 弹菜单）与 `WM_COMMAND`（菜单选中哪一行）。菜单每次右键现建现用，
所以版本号永远跟着二进制走。

菜单文字、顺序与版本号只在 `tray.go` 一处（`trayMenuRows()`），`tray_windows.go` 只负责把它画成
Win32 的菜单；`tray_test.go` 断言这五行、两条分隔线与「只有版本行不可点」，Windows 上再加一组
测试真的建一次菜单、用 `GetMenuString` / `GetMenuState` 读回来，确认标签原样到达 Win32；
手写的 `NOTIFYICONDATAW` 与 `MSG` 也用测试钉住字段偏移量 —— `cbSize` 是 shell 判断结构体版本的
唯一依据，错一个字节就是一句没说出口的错误。

关窗语义在 `app.beforeClose` 里：**图标确实在，才把窗口藏起来**（`tray.running()` 查的就是这一点），
否则让关闭照常发生。托盘里的 `Quit` 先置 `quitting` 再 `runtime.Quit`，同一个钩子因此放行，
`OnShutdown` 照常关掉连接池。图标本身用 `ExtractIconEx` 从自己的可执行文件里取（就是构建时由
`build/appicon.png` 生成、随 `.syso` 嵌进去的那枚资源），取不到就退回系统默认图标 —— `asserts/` 仍然是
品牌素材的唯一来源，不为托盘另存一份 ICO。

## 测试

```sh
just test
```

`internal/drivers/sqlite` 的测试覆盖了完整链路：目录查询、结构 + DDL、
主键顺序分页、`COUNT(*)`、`contains` / `isNull` 过滤、内联更新（含写入 `NULL`）、
过期主键返回 0 行、按主键删除。查询计划那一组测试更较真：它解释完 `INSERT` / `UPDATE` / `DELETE` 之后
**再数一遍行数、比一次字段值**，确认「Explain 只出计划、不执行」是真的，写错的语句按要求报
`query_failed`、没有包装的 spec 不去连服务器就回 `unsupported`；service 层再验证多语句脚本会被回绝
并说出找到几条，以及 `Analyzer` / `Grapher` 之外这道可选的 `Explainer` 拿不到时如实报 `unsupported`。`internal/config` 与 `internal/service` 还分别盯住了
查询收藏的磁盘往返（更新不重复、删不掉别人的文件）与校验/排序/保留 `createdAt`；
`internal/drivers/sqlutil` 则用纯函数盯住脚本干跑的分类与破坏性判定（注释里的 `drop` 不算，
无 `WHERE` 的 `DELETE` 要算），不依赖任何数据库；ER 图在 SQLite 上端到端跑一遍
（外键方向、主键/可空标记、跨命名空间的目标名），service 层再用一个只实现 `Conn`
的包装验证「没有 `Grapher` 时退化成逐对象 `Structure`」这条路；建库同一条路子：
先用 SQLite 确认「没有这个能力就说 `unsupported`」，再加上 `drivers.DatabaseCreator` 的包装确认
服务端给的选项与渲染好的语句原样透传。搬家的规则全在 `internal/config/location_test.go` 里：
默认目录、文件清单、带着数据搬去新目录并对留在原地的文件如实报告、搬回默认目录、
拒绝自己（`sameDir`）与相对路径、拒绝已经有非空数据的目录（零字节同名文件不算数据）、
目标在源目录里面（只搬名单里的文件，不递归）、指针文件读不成时回落默认、目录被删了会重建；
`internal/service/datadir_test.go` 再验证运行中的程序真的换了 store —— 保存、搬家、旧目录里不再
有连接配置、已连上的会话还在、下一次保存落在新目录，以及搬回默认目录时指针会被清掉。
两个测试都先把 `AppData` / `XDG_CONFIG_HOME` 指到临时目录（`os.UserConfigDir()` 读的就是它们），
不会碰真实的用户配置。

`internal/drivers/mongodb` 是纯单元测试加一组**默认跳过**的集成测试：连接串拼装、TLS 三档、
索引信息、shell 的词法 / 参数解析（`SplitStatements` / `ParseStatement` / 各种 JSON 值，`use` 与库名规则在内）、
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
系统库过滤、DSN 参数白名单与 `interpolateParams`、overview 取数，以及建库窗口的 `SHOW CHARACTER SET` /
`SHOW COLLATION` 读数与语句渲染。PostgreSQL 那边用自己的单测盯住 `ENCODING … LOCALE … TEMPLATE template0`
与编码 / locale 清单的组装。TiDB / Doris 各自的包还有表驱动
单测（端口、系统 schema、集群成员标签），真实实例同样要显式给地址才跑：

```sh
DMB_TEST_TIDB_HOST=127.0.0.1 go test ./internal/drivers/tidb/ -run Integration -v
DMB_TEST_DORIS_HOST=127.0.0.1 go test ./internal/drivers/doris/ -run Integration -v
```

两者默认 skip，CI 不需要备 TiDB / Doris。

托盘那一圈不需要桌面也能测：菜单内容按 `trayMenuRows()` 断言，Windows 上再加一组只在 Windows 跑
的测试 —— 手写的 `NOTIFYICONDATAW` / `MSG` 字段偏移量与 `utf16Fill` 的截断 / 终止符、真的建一次
托盘菜单再用 `GetMenuString` 读回来（标签原样、版本行是 `MF_GRAYED`）、图标能从可执行文件或系统
默认图标里拿到。窗口那半边（关窗进托盘、点图标回来、菜单里的 `Quit` 干净退出）是脚本化的手工
烟测：用 `PostMessage` 把 shell 会发的消息发给应用自己的窗口，看进程还在不在、窗口可见不可见，
不放进 CI。

## 发布

本地打包和对外发版是两件事。

### 本地打包

```sh
just release
```

编译当前平台并把产物归档到 `release/`，附 `checksums.txt`：

```
release/db-manager-0.1.0-windows-amd64.exe
 release/checksums.txt
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
  - [x] 查询计划与格式化：Explain 只看不跑（Results / Plan 两档切换），Format 按文法重新缩进
  - [x] ER 图：命名空间的关系图，可搜索 / 缩放 / 导出 SVG，点节点开表
  - [x] 新建数据库：字符集 / 排序规则（MySQL 一族）、编码与 locale（PostgreSQL）由服务端回答，语句先展示再执行
  - [x] 运行情况：双击连接看服务端现状，每个引擎一个视图
  - [x] 连接排序与分组：整行拖动换位置，一层深的分组文件夹，排法存在 `layout.json` 并与实时配置对齐
- [x] Phase 2：MongoDB（连接、集合浏览、shell 查询、增删改、索引、运行情况）
- [x] Phase 2.5：TiDB / Apache Doris（同一种线协议共用 `mysqlcompat`：TiDB 带 TLS 与集群成员概览，Doris 只读浏览 + DDL 编辑）
- [ ] Phase 3：Oracle / SQL Server（占位与 dialect 已在，缺 DSN 与目录查询，`planned.Parked()`）
- [ ] Phase 4：SSH 隧道、导入向导、数据对比、插件式扩展

## 许可证

[MIT](LICENSE)
