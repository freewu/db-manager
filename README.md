# db-manager

使用 **Wails v2 + React + Vite + Justfile** 开发的关系型数据库管理工具，打包为 Windows 桌面应用。

支持 **MySQL / TiDB / Apache Doris / PostgreSQL / SQLite / MongoDB**；驱动层不假设关系模型，文档型引擎走的是同一套 `Driver / Conn / Dialect` 契约，service 与 UI 只问能力、不问引擎名。Oracle / SQL Server 的占位仍在（见 `internal/drivers/planned`），但暂不上菜单。

> 在本仓库写代码前先读 [`AGENTS.md`](AGENTS.md)：每次开发完成必须 commit + push，本地打包用 `just release`，发版走 `just publish <x.y.z>` 触发 GitHub Actions 打出三平台免安装可执行文件。

## 功能

- **连接管理**：连接配置的增删改查、连通性测试、SQLite 文件选择、TLS（CA / 证书 / 私钥）、自定义 DSN 参数、只读标记、颜色标签、密码可选保存。新建连接先点出**驱动菜单**（命令条 `Connection`、连接树的 `+`、面板空白处右键，三处挂的是同一份列表，就展开在你刚点的那个东西下面），再落到**这个引擎自己的那一页** —— 走网络的要地址、端口、账号与 TLS，SQLite 只要一个文件加一个附加库别名，两边不会互相看到无关字段（保存下来的配置也照着这一页来，文件型连接不会混进 host / port / ssl）；编辑已有连接同样按它的驱动打开对应那页。
- **对象浏览器**：会话 → 数据库 → Schema → 表 / 视图 / 索引 的懒加载树，**每个文件夹都带数量、空着也画**（`Tables (0)` / `Views (0)` / `Indexes (0)` —— 文件夹一空就消失，跟「这个引擎根本没有这种对象」就分不出来了），该画哪几个由后端按引擎声明（`DriverInfo.objectKinds`），前端不猜；右键菜单支持打开数据、设计表（设计器给不出的引擎是「看字段」）、复制名称 —— **新建查询不在对象自己的菜单里**：那个窗口跟这张表没有关系（不带上表名、也不落在这张表上），它的入口是命令条上的 `New Query` / 标签页的 `+`（都落在树里选中的那个库 / schema 上）、库节点与 `Queries` 文件夹的 `New query…`（后者是给脚本起名字）。文档型引擎这里是 database → Collections / Indexes，没有 schema 层，SQL 专属的入口（新建 DDL 脚本、ER 图、设计表）自动不出现。
- **连接排序与分组**：整行拖着换位置，`+` / 空白处右键菜单里的 `New group…` 建一层深的文件夹，拖进去、拖出来、双击分组展开或收起、删掉文件夹（里面的连接会回到顶层，不会被一起删）都行；排法按 id 存在 `layout.json`，读与写都先拿实时配置对一遍，所以配置增删、换版本都不会让排列对不上号（按名字过滤时拖拽关闭 —— 那时候看得见的邻居不是布局里的邻居）。
- **新建数据库**：连接右键 `New database…`，问什么由**服务端**回答 —— MySQL / TiDB 给出字符集与可配的排序规则，PostgreSQL 给出编码与 locale（locale 名单不可能完整，那一栏可以手打），Doris 与 MongoDB 没有可选项、只有一句说明。语句由后端按引擎渲染后**先展示再执行**（MongoDB 下是 `use <db>`，并明说「第一条 collection 写进去之前它什么都不存」）；SQLite 这类文件型引擎不出现这个菜单项，只读会话里它是灰的。
- **MongoDB**：连接（含副本集多主机、`mongodb+srv`、TLS、认证库）、集合浏览（文档数 / 体积 / 索引）、数据网格的过滤排序与分页、双击改标量字段、批量删文档、索引列表与定义脚本、运行情况页（serverStatus + 每库 dbStats）。查询窗口跑的是 **mongosh 风格的 shell**（`db.orders.find({...}).sort({ts: -1}).limit(20)`），不是 SQL。
- **TiDB / Apache Doris**：两个引擎对客户端都讲 MySQL 线协议，但脾气各不相同。TiDB 默认端口 4000，有 TLS 页（`Security`），表单里的 `Database` 只是新标签页的默认库 —— 一个 TiDB 集群就是一份逻辑数据库，树里一次列全所有库，所以这一栏是可选的；运行情况是 MySQL 那一页再加一张 `information_schema.CLUSTER_INFO` 的集群成员表（tidb / tikv / tiflash / ticdc / pd）。Doris 默认端口 9030，前端（FE）与后端（BE）都在集群网内、MySQL 端也不做那套握手，于是**没有 TLS 页**；它是分析型引擎，本工具里**只读浏览** —— 表结构与索引照样能看（字段列表说的是引擎自己的类型拼写，如 `varchar(120)` / `decimal(10,2)`），但没有表设计器，改动走 DDL 编辑器。两者的连接、库表浏览、数据网格、SQL 查询与导出都复用 MySQL 那条代码路径。
- **对象列表**：点击树里的表 / 视图 / 索引文件夹，在右侧开出 Navicat 风格的对象网格（名称 / 类型 / 行数 / 大小 / 引擎 / 注释，索引列还有所属表 / 列 / 唯一性 / 主键 / 方法），支持列排序、列筛选、底部关键字过滤，单击打开对象、双击进入设计视图；打开或切回一个列表窗口时，连接树会跟着展开到它所属的库并选中那个文件夹。
- **数据网格**：分页、服务端排序、服务端过滤（14 种操作符）、**列宽可拖动**（表头右边缘按住左右拖，拖过一次之后宽度就按拖出来的算，多出来的空间留给一列空列、表格照样铺满窗格）、长文本悬浮预览、多选、**点一行从右侧滑入这一行的详情**（字段清单 / PK / NULL、复制 JSON、复制 INSERT，**还能直接改字段** —— 改完点 Apply 先看引擎渲染出的那条语句，确认后才写下去）。
- **行编辑**：双击单元格内联编辑、批量删除选中行，也可以在**行详情里一次改多个字段**（改完先看后端渲染出的那条 `UPDATE`，确认才执行 —— 预览与真正执行的语句由同一段渲染代码产出，所以确认框里就是发出去的那条）；所有写操作都以主键为条件、**全部使用参数绑定**，删行同样先渲染出 `DELETE` 再执行。这些改动会进**变更日志**（来源标为 `grid`），见下。
- **SQL 编辑器**：基于 CodeMirror 6，按驱动切换语言（MongoDB 用 JavaScript，其余用各自方言）、语法高亮、多语句执行、执行历史、`Ctrl/Cmd+Enter` 执行全部、`Ctrl/Cmd+Shift+Enter` 执行选中；补全除了关键字，还列出**这个窗口所属命名空间里的表与视图**，名字后面用斜体小字标着它是表还是视图（那是 CodeMirror 补全项的 `detail`）。清单取的是资源管理器已经缓存的目录，窗口自己再把**它点名的那一个** namespace 取一次 —— 所以不必先在树里把文件夹点开才能补全，也不会因为开一个窗口就替每个 schema 跑一次目录查询（规则写在 `lib/tree.ts` 的 `catalogOf` 里）。只补名字、不补字段：字段是另一份目录（`GetTableStructure`），编辑器不去偷偷取它。
- **查询计划（Explain）**：查询窗口工具条上的 **Explain** 不去执行，而是问引擎「这一条你打算怎么跑」—— 后端按 `Spec.ExplainSQL` 套上各自的包装（MySQL 一族的 `EXPLAIN`、PostgreSQL 的 `EXPLAIN`、SQLite 的 `EXPLAIN QUERY PLAN`），结果铺进和数据网格同一个表格，下半区用 `Results` / `Plan` 两档切换，正文上方原样印出真正发出去的那条语句；包装**永远不带 `ANALYZE`**，所以什么都没被执行 —— 只读连接能解释，`DELETE` / `UPDATE` 也能先看计划再决定跑不跑，面板上同时写明「这是估算、不是实测」。一次只解释**一条**语句：脚本会被回绝并说明找到几条（要解释哪一条就选中哪一条）；`DriverInfo.supportsExplain` 为假的引擎（MongoDB，它的语句是 shell 调用）按钮是灰的并说明原因，而不是点了才报错。
- **SQL 格式化**：工具条上的 **Format**（`Ctrl/Cmd+Shift+F`）按引擎的 SQL 文法重新缩进当前语句 —— 只有「关键字大写」这一个主张，其余只是空白；有选中就只格式化选中、没有就整篇，文法读不出来的脚本**原样不动**并说出错在哪（半格式化的脚本比不格式化更糟）。文法名与高亮一样由驱动决定（`frontend/src/lib/sqlFormat.ts`，走 `sql-formatter` 的文法表）；MongoDB 没有 SQL 可格式化，按钮是灰的。
- **结构查看器**：列、索引、外键、原始 DDL（优先使用引擎原生 DDL），DDL 可复制或导出；这份 DDL 是**带语法高亮的**（关键字 / 类型 / 字符串 / 数字 / 注释 / 引号里的名字各一色），高亮由前端自己扫一遍字符得到，不引第三方词法库，也不把脚本拼成 HTML —— 字符串字面量里的 `<img>` 就只是那几个字符；同一套读法还用在该语句出现的其它只读场合（保存前的语句清单、新建数据库的语句预览），并按引擎分别处理：PostgreSQL 的 `"…"` 是名字而 MySQL 的是字符串、`$tag$…$tag$` 整段算一个字符串（函数体就这么活下来）、MongoDB 的 shell 按 JavaScript 读（`--` 在那里是自减而不是注释）。文档型引擎的「结构」页是**抽样得到的字段表**（字段名 / 类型 / 是否可能缺失），并说明集合本身没有 schema。这套颜色服务的是所有**只读代码**（变量名因此是 `--dm-syntax-*` 而不是 `--dm-sql-*`）：代码生成窗口画出的文本用同一个调色板，那边只是换了读法（见下）。
- **表设计器**：表格窗口的「结构」页就是编辑器 —— 直接改字段名 / 类型 / NULL / 默认值 / 主键 / 自增 / 注释，字段行还能用行首的抓手**拖动排序**（行序就是列序，后端据此渲染 DDL）；索引不在这页改，隔壁「Indexes」页把主键与索引只读列出来。保存前无需联网猜测，按 Save 弹出的确认框里就是要逐条执行的语句（同样带语法高亮）与引擎限制警告，保存时逐条执行并如实报告「第几条失败」（MySQL / PostgreSQL / SQLite 各自的限制都写在警告里）。**建表走同一条路**：连接树里「表」文件夹右键的 `New table…` 开一个空设计（一个主键字段起步，表名就在工具条上敲），确认框与保存用的还是同一个规划器 —— 只是把实时目录换成空基线，`ALTER` 换成 `CREATE`；建完这个窗口会变成刚建好的那张表，停在「结构」页。引擎给不出设计器的（MongoDB、Doris）这一页退化成**只读字段列表**并直说「这个引擎没有表设计器」，不摆一个按下去会失败的按钮。
- **查询收藏**：查询窗口工具条上的「Favourites」可以把当前 SQL 命名保存（默认用第一行非注释文本作名），下拉里一键载入、重命名或删除；收藏存在 `queries.json` 里，与连接配置互不影响，换窗口、换连接都能用。
- **保存的查询**：每个库下面有一个 `Queries` 文件夹（排在 `Tables` / `Views` / `Indexes` 后面），右键「New query…」新建脚本、点一下就开在查询窗口里 —— 脚本落成 `<数据目录>/.query/<连接>/<库>/<名字>.sql`，窗口里改了字标签页上出现 `*`，`Ctrl/Cmd-S` 存回文件，关掉没保存的窗口会先问一句；**没名字的草稿窗口也有 Save** —— 点它（或 `Ctrl/Cmd-S`）先问名字，名字定下来就把窗口里现有的正文当场写进去（后端一次 `CreateQueryFile` 写完正文，不做「先建空文件、再填内容」两步，免得中间失败只剩一个空文件），窗口随即变成「这个文件的窗口」：标题换成名字、树里 `Queries` 下多一行，之后的保存都落在它上面；名字已被占用时后端拒绝并点名是哪一个，而不是悄悄覆盖掉别人的脚本（没有配置的临时会话、连库都没定的会话无处可存，按钮是灰的并说明原因）。重命名是**纯文件移动**（从不读写内容，所以树里改名不会覆盖窗口里没保存的编辑），删除会先确认并说明「已经打开的那个窗口里的文字会留下」。脚本跟着**数据目录**走，换目录时 `.query/` 整棵树一起搬。
- **ER 图**：在 schema（没有 schema 层的引擎就是 database）节点右键即可打开该命名空间的关系图 —— 一张 `GetSchemaGraph` 就把对象、字段与它们之间的外键取回来；**同一家的表排成一行**，行的归属由名字决定：一个名字的行 key 是它最短的那段「读起来像一家人的」前缀 —— 要么本身就是某张表的完整名字（`t_user_favorite` 归到 `t_user`），要么是至少两张表共同的开头（`xxx_dict_data` 与 `xxx_dict_env` 归到 `xxx_dict`，哪怕库里根本没有 `xxx_dict` 这张表）；于是 `t_user` / `t_user_favorite` / `t_user_profile` 一行，`t_order` / `t_order_payment` 一行，而 `t_product` 这种没人同族的自己占一行 —— 单个词只有当真有表叫这个名字时才算一家（否则库里都叫 `t_…` 的表会被挤成一条长龙；真有一张表就叫 `t` 的话，它们就是这一家）；**行内**再按外键层级从左到右排，所以行内的箭头仍指向被引用的那张表，跨行的外键就随它斜着走（往左上走的那根从左边缘绕出去，不穿过自己这个框）；主键高亮，箭头悬停显示「哪一列引用哪一列」；支持拖动平移、滚轮缩放、按名搜索、隐藏/显示字段、网格开关，点节点直接打开该表，还能把当前这张图导出成自包含的 SVG。跨命名空间的外键画成虚线 stub，读不到字段的对象仍在图里但会列出警告，超过 300 个对象时明确提示只画了前 300 个。
- **DDL 编辑器**：把对象的定义开成可编辑的脚本窗口（MongoDB 下就是集合的定义脚本与 shell 查询） —— 工具条、对象树右键「Edit DDL…」或结构页的「Edit in DDL editor」都能进；编辑器下方是**后端算出来的干跑结果**（逐条语句标出 query / DDL / DML、标红 DROP、TRUNCATE、无 WHERE 的 DELETE/UPDATE，MongoDB 下则是 `drop()` / `dropDatabase()` / 无 filter 的 `deleteMany`，并说明只读连接会拒掉几条），真正点「Run script」时只对破坏性脚本弹二次确认；执行完顺手刷新目录树与索引缓存。
- **代码生成**：表的右键菜单里多了 `Generate code…`（表 / 视图 / 物化视图 / 集合都有，序列与存储过程没有字段，所以菜单里不摆），开出来的窗口把这个对象的字段与类型翻成这个语言的类 / 结构体 / 记录 —— 一共 18 种（python、c、cpp、java、csharp、javascript、rust、php、go、ruby、swift、perl、objectivec、julia、kotlin、typescript、erlang、lua），Java 分两档：`Java (Lombok)` 用 `@Data`，素写的那个把 getter / setter 展开。**生成在前端做**（`frontend/src/lib/codegen/`），所以工具条上的语言下拉一换就当场重画 —— 不过桥、不发请求（结构只读一次，跟 DDL 生成不同：那是引擎自己的事，`PlanAlter` 在后端），窗口里换语言也只换这个窗口，要换新窗口开出来的那一档就去 **设置页 → Code generation** 改（默认 `Java (Lombok)`）。类型映射是一张**有序的规则表**收成 14 类（`tinyint(1)` 是布尔、`character varying` 与 `nvarchar` 都是字符串、`numeric` / `money` 是定点数、MongoDB 的 `int32 | string` 这种联合类型取读得懂的那一半），不认识的落到该语言自己的兜底类型而不是瞎猜；**可空按各语言的写法给**（Java 换包装类型、Python `Optional[T]`、Kotlin `T?`、Rust `Option<T>`、Go 指针、TS `T | null`、Erlang `T | undefined`…），字段名按该语言的命名习惯收（Go / C# 用 Pascal、Java 一族用 camel、Python 一族用 snake），Go 的字段名还按 Go 的规矩把缩写拼全（`user_id` → `UserID`）并带上 `json` / `db` 两个 tag（可空加 `,omitempty`）—— 生成的 Go 是 `gofmt` 之后一个字节不长不短的样子（有注释的字段会打断对齐组，所以补对齐是按段算的）。字段注释只来自这个列自己的注释，主键与自增不另加标记；文件名与后缀按语言给（`Users.java` / `users.py` / `users.go` / `users.erl`…），Copy 与 Save 在工具条上。生成的文本是**带语法高亮的**：每种语言在 `frontend/src/lib/codegen/lexis.ts` 里写一行「行注释拿什么标、引号怎么配对、哪些词算关键字 / 类型」，`codegen/highlight.ts` 拿这行扫一遍字符（跟 SQL 那个一样：不引第三方词法库、也不把代码拼成 HTML —— 注释或字符串里的 `<img>` 就只是那几个字符），换语言时连颜色一起换。颜色就是 DDL / SQL 预览那套 `--dm-syntax-*`，所以同一个关键字在哪个窗口都是同一个紫；那边的读法按**引擎**分（PostgreSQL 的 `"…"` 是名字而 MySQL 的是字符串），这边的读法按**语言**分，两件事互不干扰。
- **数据生成**：图标栏上的 **数据生成**、或表右键的 `Data generation…`，开一个「填表」窗口 —— 左边挑一张表（连接 → 库 → schema → 表，只列**表**：视图没有自己的 INSERT，集合没有列清单），右边就是那张表的字段表（Field / Type / Mock / Description）。**每行左侧有一个勾选框，勾上的列才会进 INSERT** —— 默认全勾，**自增列默认不勾**（那个键得由引擎发，自己编的值一轮写完就撞上了），表头那个框是全选 / 全不选。**空 mock 与不勾是两件事**：不勾 = 这一列不参与（描述列会写明），勾了却没写 mock 会被拦下来并点名是哪个字段，而不是拿空字符串冒充一个值 —— 勾中的行带品牌色底纹、未勾的行变淡（但 mock 仍可读可改）。**一个连接一个窗口**：表里写好的 mock 在同一个连接里换表不丢，从别的连接选一张表会切到**那个连接自己的窗口**（每个窗口只写它自己连上的那个库）。
  - **Mock 是模板，不是固定值**：写的是 mock.js 语法 —— 字面文本里夹 `@` 占位符，`user_@natural(1, 999)@tld` 也是一个值。窗口里的取色器把分组摆在**左边**、**可滚动**（Person / Web / Basic / Time / Character / Number，`@cname`、`@id` 带校验位、`@bankcard` 走 Luhn、`@address` 是省市区 + 街道 + 门牌号、`@guid`、`@now(day)`… 一共 40 多个；最后那一页 **Custom** 是用户自己攒的），每个占位符是**一块平铺的卡片**而不是一行 —— 卡片上除了名字，直接用同一个引擎渲染一行**示例值**（同一种模板给同一种样本，重开窗口也不会乱跳）：光看名字分不出两个相似的占位符，看样本就分得出来。也可以直接手打；**认不出的 `@name` 是错误而不是原样输出**（mock.js 会把 `@nope` 留下来，于是打错一个字就得到一列同样的错字符串），单元格当场标红、Generate 拒绝执行并点名是哪个字段，要写一个真正的 `@` 就用 `@@`。模板引擎在 `frontend/src/lib/mock/`（词表、校验位、参数解析都是本地实现，一处纯函数、可给种子复现）。
  - **自定义占位符**：取色器最后那一页 **Custom** 里是用户自己的占位符 —— 一个名字换一段模板（`orderNo` = `SO@date(yyyy)@natural(1000, 9999)`，mock 里就写 `@orderNo`），在 **设置页 → Mock placeholders** 里增删改。存法是**一个占位符一个文件**：`<数据目录>/.mock/<名字>.json`（`{"version","name","template","description"}`），名字就是文件名（过的是查询树那套 `escapeSegment`），所以它跟着**数据目录**一起搬。自定义占位符是**无参数的整段模板别名**（参数属于底下那些内置的），与内置重名时**内置赢**（表单直接不让存），写错名字仍是错误；展开在编译期**就地**完成并带**循环检测**（`@a` → `@b` → `@a` 会把整条链报出来），于是「整段就是一个占位符」的自定义保持着那个值的类型（`@dice` 给整数列的是整数，不是字符串）。设置页的编辑器**边写边判**（名字形状、与内置或别的自定义重名、模板能不能编译），下面还有一个**调试面板**：拿一个显示在面板上的种子渲染 5 行（`Reroll` 换一组），一眼看出「能编译」与「生成了我想要的东西」是两回事。手改坏了、读不出来的文件不会被取色器摆出来（点了只会失败），但在设置页里以 `Unreadable` 列出并可删除 —— 消失得无影无踪才是最难查的。
  - **初值从列名与列型来**：列名先说话（`email` / `phone` / `身份证` / `address`… 对不上列的长度就把提示丢掉，免得 `@cname` 塞进 `char(2)`），列名说不出话时按类型给（`int` → `@integer(1, 100)`、`datetime` → `@datetime(…)`、`json` → `{"key": "@word"}`）；**自增列留空且默认不勾**（这个键必须由引擎发，否则一轮写完自己就撞上了），非自增的整数主键拿 `@increment(1)`（唯一值靠「会数数」的那个占位符，不靠运气）。Description 列会写出这个字段会得到什么、并给一行**样本** —— 样本的随机源只跟字段名有关，所以在别的行里打字时它不会乱跳。工具条上的 **Reset mocks** 把这张表的 mock 与勾选一起恢复成初值。
  - **写下去之前先问一次**：确认框说明要写多少行、发哪几列、进的是哪张表，并写明**没有 undo**（撤不回来，要删就按普通行删）。生成在前端、分批过桥：每批 200 行（后端上限 500），一个批次一条多行 `INSERT`，进度条在两批之间走、**Stop 也在两批之间生效**，所以停下来时报的数就是真的写进去的数。后端 `InsertRows` 只把标识符（表名、列名）按方言引号拼进语句，**每一个值都是绑定参数**，前端生成的任何文本都不可能变成 SQL；JSON 过桥后整数都成了 `float64`，落库前把**整数值折回 int64**（PostgreSQL 会把 `float64` 绑到 `int4` 上直接拒收，MySQL 则默默四舍五入），小数原样交给列去判断。引擎拒掉一批时，后端**逐行重放**找出是哪一行并把引擎自己的话带回来，窗口如实报「第 N 行被拒，前面 m 行已进表」，不会把一半说成全部。
- **变更日志**：图标栏上的 **变更日志** 开一页「这个程序对数据库做过什么」—— 左边按**最新在前**列出每条记录（时间、语句的首关键字、作用在哪个对象上、语句摘要），右边是选中那条的全部内容：**日志时间 / 连接信息（连接名 + 引擎 + 用户与地址）/ 数据库（`库 · schema`）/ 表名称 / 从哪个窗口执行的 / 执行语句**（按引擎高亮），失败的那条还带一段红色说明。顶部有计数（`the newest 500 of 1,248 statements`）、过滤框与重新读取。它读的是**数据目录里的文件**而不是任何一个服务端，所以**没连库也能打开** —— 而变更通常正是连接已经关掉之后才要查的东西。记录的是**这个程序被要求执行过的写语句**：结构页保存（一条一条执行，所以每条都有自己的结果）、DDL 编辑器 / 查询窗口 / 新建库里跑过的脚本（按语句切开，只留 `ddl` / `dml`，读语句与认不出的语句不记）。**网格里的改与删现在也记**（来源那一栏是 `grid`）：行详情会把引擎渲染出的那条语句摆出来给人看过再执行，内联改单元格与删行走的是同一段渲染 —— 日志里那行就是真正发出去的那个字符串，不是为日志另拼一次，发出去又失败的语句也照记（日志回答的是「这个数据库被要求做过什么」）。**数据生成仍然不记**：它是成批的多行 `INSERT`（一批 200 行），一批一条地写进日志只会把它淹掉。日志**只轮转、从不丢弃**：写满一个文件（**默认 2000 条，可在 设置 → 数据目录 里改**）就整份改名成 `20260214-1.log` 留档、另起一个新的，所以上面那个下拉框里能选出**历史日志**来读（每个文件都标着里面有多少条、多大），右边读的就是选中的那一份。
- **窗口即应用**：右键不再弹出 WebView 自带的那套菜单（后退 / 刷新 / 另存为 / 打印 / 检查），
  右键要么什么都不做，要么就是应用自己的菜单（连接树等）；文本框与 SQL 编辑器是例外 —— 那里保留系统菜单，
  右键粘贴照旧可用，其它地方用 `Ctrl+C` / `Ctrl+V`。
- **运行情况**：双击连接节点即可打开该连接的「运行情况」页（未连接会先连上，密码框填完再自动打开）；页面上半部分是会话事实与快照时间，下面按引擎各画各的 —— MySQL 给进程列表、连接数、InnoDB 缓冲池与命中率（TiDB 走同一页，外加集群成员表；Doris 也尽力取这一套，取不到的项挂进警告里），PostgreSQL 给后端/活动会话/数据库体积与提交率、缓存命中率，SQLite 则是「这是一个文件」的视角（路径、落盘大小、pragma、对象清单与 ATTACH 进来的库）。读不到的项一律显示 `—` 并附一条警告（缺权限、缺统计视图），不会拿 0 冒充；工具条的刷新按钮重新取一次快照。
- **项目信息**：没连库时右侧只有一页项目信息，标题就是 `DB Manager` 加当前版本（`v0.1.0`，字号比正文大一档）—— 标题下面一排徽章（`license MIT`、`build just 1.58.0`、`running windows/amd64`），再按组列出 **`Build`**（一枚可点的 `justfile` 徽章，值就是三条常用配方，点开是仓库里的 Justfile）、技术栈（`Runtime`、`Desktop and UI`）与 **`Platforms`**（`windows amd64` / `macos universal` / `linux amd64`，和 `.github/workflows/release.yml` 的构建矩阵一一对应）—— 徽章是 shields 样式的灰标签 + 品牌色值，前端库版本直接读 `frontend/package.json`，Go 版本取自运行中的二进制，再下面依次是项目地址 / Releases / Issues 和开发者（只画一个 GitHub 头像，悬停显示昵称、点一下打开 `github.com/freewu`）。徽章用本地 CSS 画，离线也能渲染；链接交给系统浏览器（`window.runtime.BrowserOpenURL`），不把整个窗口导航走。连库的入口在命令条的 Connection，以及连接树的右键菜单。
- **导出**：CSV / JSON / INSERT 脚本（MongoDB 下是 `insertMany` 脚本，按列的 BSON 类型还原 `$oid` / `$date` / 文档字面量），可写入文件或复制到剪贴板。
- **外观**：Navicat 式窗口骨架（icon-over-label 命令条 + 最左那条四页图标栏 + 连接树 + 标签页工作区 + 状态栏）、明暗主题、品牌绿 `#36ab60`、可拖拽分栏、紧凑的表格与状态栏 —— 表格的表头**固定不动**：数据网格、对象列表、设计器的字段表都一样，纵向滚动时表头留在容器顶上，横向滚动也带不走它（钉住的列仍旧钉在原处）。主题有三档：`Light` / `Dark` / `System`，在设置页的 **Appearance** 里选，状态栏那格点一下也能循环切换（跟随系统时它会写成 `system (dark)`，把当前系统给的那一档一起说出来）；选**跟随系统**时窗口真的跟着操作系统走 —— 操作系统在运行期间切深浅色，界面当场就变，不用重启也不用再点一次。命令条上目前只有 **Connection**、**Open**、**Close**、**New Query**、**Refresh**、**Table** 与 **View** 是活的，其余按钮保持原来的位置但禁用并在提示里说明 —— 摆着的空位比消失的按钮更好认。切**页面**的命令不在命令条上，而在最左边那条图标栏里（见「页面栏与四个页面」）：命令条只管**作用在当前连接上的事**。
- **设置**：最左边图标栏上的 **设置** 开一整页（不再是弹窗 —— 它是「去一趟再回来」的地方，不是盖在手上的东西）—— 左栏是栏目、右边是这五个栏目共用的一条滚动区，于是栏目名**跟着滚动高亮**（滚到哪一段就是哪一段），点栏目也真的把内容滚过去 —— `Appearance` 选主题，`Code generation` 选代码窗口默认用哪种语言，`Mock placeholders` 管自己的占位符（见上），`Data folder` 看数据存在哪、里面有哪些文件（每个文件写的是干什么的、多大 —— 连接配置、查询收藏、树的排法、窗口状态、密码密钥、**变更日志**、**日志的轮转设置**，还有 `.query` / `.mock` 这两个文件夹与每个 `20260214-1.log` 留档；后两者按「一个文件夹、里面几个文件」列出来）、**顺手改日志一个文件写多少条**（到数就整份留档，见下）、从这里**打开目录**、**换一个目录**或**恢复默认**，`About` 就是原来那页项目信息（技术栈徽章、项目地址、开发者）。每一页都是**改了就生效**，所以没有「取消」可点，也没有哪个选择会因为走开而丢掉。换目录是真的**把数据搬过去**：连接配置、查询收藏、树的排法、窗口状态与密码密钥一起复制到新目录（逐个读回校验，对不上就不动原文件），然后才改指针、最后才删旧文件；没能删掉的（被占用、只读）会**如实列出来**，而不是回一句「已移动」。目标目录里已经有**非空的**本程序数据时会被拒绝并点名是哪个文件（同名但零字节的不算数据，照常覆盖），选到当前目录本身也会被拒绝；那些**不是本程序写的**文件留在原地并在结果里注明。目录就绪后不需要重启 —— 会话照旧连着，新的保存当场写进新目录。
- **项目信息**：没连库时右侧只有一页项目信息，标题就是 `DB Manager` 加当前版本（`v0.1.0`，字号比正文大一档）—— 标题下面一排徽章（`license MIT`、`build just 1.58.0`、`running windows/amd64`），再按组列出 **`Build`**（一枚可点的 `justfile` 徽章，值就是三条常用配方，点开是仓库里的 Justfile）、技术栈（`Runtime`、`Desktop and UI`）与 **`Platforms`**（`windows amd64` / `macos universal` / `linux amd64`，和 `.github/workflows/release.yml` 的构建矩阵一一对应）—— 徽章是 shields 样式的灰标签 + 品牌色值，前端库版本直接读 `frontend/package.json`，Go 版本取自运行中的二进制，再下面依次是项目地址 / Releases / Issues 和开发者（头像加昵称整个是一条链接，昵称就是显示出来的文字，也是链接的标题，点一下打开 `github.com/freewu`）。徽章用本地 CSS 画，离线也能渲染；链接交给系统浏览器（`window.runtime.BrowserOpenURL`），不把整个窗口导航走。这页在没连库时显示，设置页的 **About** 里也是同一份（同一个组件，两处不会各说一套）。连库的入口在命令条的 Connection，以及连接树的右键菜单。
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
│   ├── config/changelog.go   # 变更日志：<数据目录>/changelog.jsonl + 满 2000 条整份留档成 <yyyymmdd>-N.log + changelog.json 记轮转阈值
│   ├── config/mockplaceholders.go # 自定义 mock 占位符：<数据目录>/.mock/<名字>.json，一个占位符一个文件
│   ├── secret/               # AES-256-GCM 封装连接的密码；密钥 secret.key（0600，首次用时生成）
│   ├── models/               # 跨层 DTO，时间统一为 int64 unix ms
│   ├── drivers/
│   │   ├── driver.go         # Driver / Conn / Dialect / Grapher / Overviewer / Analyzer 契约 + 注册表（init 注册）
│   │   ├── format/           # 指标格式化（字节 / 计数 / 时长 / 百分比），两个 overview 实现共用
│   │   ├── sqlutil/          # 标识符引用、WHERE / ORDER BY 构造（纯字符串+参数位）、脚本干跑
│   │   ├── sqlbase/          # 通用 database/sql 实现：连接池、分页、脚本执行、DDL、行变更（含 literal.go 的语句预览字面量）、运行情况外壳
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
        ├── components/       # AppShell / ActivityBar / Workspace / TablePane / QueryPane / DataGrid …
        │   ├── AboutProject.tsx   # 项目信息（技术栈徽章 / 项目地址 / 开发者），空态页与设置页的 About 共用
        │   ├── ActivityBar.tsx # 最左边的图标栏：连接 / 数据生成 / 变更日志 / 设置四页唯一的门
        │   ├── SettingsPane.tsx # 设置页（不再是弹窗）：外观 / 代码生成 / Mock 占位符 / 数据目录（含日志轮转阈值）/ 关于
        │   ├── DataGenWorkspace.tsx # 数据生成页：一个连接一个窗口的标签栏，窗口本体在 DataGenPane.tsx
        │   ├── MockPickerModal.tsx # 占位符取色器（左侧分组 + 平铺卡片，卡片带示例值）
        │   ├── MockPlaceholderSettings.tsx # 自定义占位符：列表 + 编辑器（带种子的调试面板）
        │   ├── ChangeLogPane.tsx # 变更日志：顶部选文件（当前 / 留档）、左边最新的记录、右边那条的全部内容（连接 / 库 / 表 / 语句）
        │   ├── ResizableHeader.tsx # 可拖动的列宽：useColumnResize 给每张表接上表头拖动（一次拖动把所有列宽量下来定死，之后 table-layout: fixed）
        │   ├── RowDetail.tsx # 行详情：字段表单 + 引擎渲染的语句预览，改多个字段合成一条 UPDATE 一次写下
        │   └── overview/     # 运行情况：每个引擎一个视图 + 共用的指标卡片与数据表
        ├── connection/       # 每种驱动一页连接表单（Mysql / Postgres / Sqlite / Mongodb / Tidb / Doris）+ 注册表
        ├── hooks/useConnect  # 先试后问的连接流程
        ├── lib/              # tree key 编解码、格式化、导出、代码生成（codegen/，见下）、驱动能力（capabilities.ts）、项目信息（about.ts）、主题（theme.ts）、品牌素材（assets.ts，@asserts 别名）
        │   ├── codegen/      # 代码生成：types.ts（类型归类与渲染契约）/ languages.ts（每种语言的数据）/ lexis.ts（每种语言怎么读）/ highlight.ts（生成代码的高亮）/ index.ts（入口）
        │   └── mock/         # 模板引擎：engine.ts（解析、渲染、种子）/ words.ts（词表与校验位）/ catalog.ts（取色器目录）/ defaults.ts（列名、列型猜初值）/ custom.ts（自定义占位符的校验）
        ├── store/            # zustand 全局状态（连接 / 会话 / 标签页 / 页面 / 浏览器缓存）
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
浏览器的文件夹就照着它画。`supportsInsert` 是同一类事实的第五项：这个引擎能不能把生成的行写进表里（MongoDB 为 `false`），
图标栏上的**数据生成**与表右键的 `Data generation…` 据此决定露不露面 —— 它和 `drivers.Inserter` 接口是同一件事的两面
：service 是拿 `Conn` 去**断言这个接口**（不是看引擎名），所以一个没实现它、却把 `supportsInsert` 写成 `true` 的驱动会在点下去之前
就报「写不了」，而 `DriverInfo` 只是把同一事实告诉界面。前端读的是同一份结果（`frontend/src/lib/capabilities.ts`），不按引擎名写 if ——
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

### 页面栏与四个页面

窗口最左边是一条**图标栏**：**连接**（工作区 —— 连接树与连接开出来的各个窗口）、**数据生成**、**变更日志**、
**设置**。它不是装饰品，而是这四页唯一的门：连接树这一格窗格是可以收起来的，收起来之后就再没有地方把它叫回来，
所以得有一条**自己不能被藏起来**的东西来给它们命名，而这条栏也不可能被任何一页带着消失（四页都是它的内容）。

切页面**不是重建页面，而是先挂载、之后一直挂着（只是隐藏）**：数据生成窗口里写了一半的 mock、变更日志里
正读着的那一条，都不该因为走开一眼就没掉 —— 跟标签页把非当前页留在 DOM 里是一样的理由。所以主区域不是
「一个窗格换内容」，而是四层叠在一起、只显示其中一层（`AppShell` 的 `PageLayer`）；每次切回来时两页会重新读一遍
数据目录（它们的 `active` 就是 `page === …`），因为隐藏期间数据可能被别的窗口改过。

由此**数据生成 / 变更日志 / 设置不再出现在工作区的标签栏里**：它们不是「某个连接上的一个窗口」（数据生成是
**一个连接一个窗口**，按连接而不是按表分组；变更日志与设置根本不属于任何连接），所以它们各自成页，
从标签栏里被滤掉（`Workspace`）。命令条上也随之**没有**这三枚按钮 —— 切页面的命令就待在页面旁边。
图标栏上的**数据生成**还兼着命令：已经有窗口就直接切过去，没有就拿**当前连着的那个会话**开一个（跟表右键
开的是同一个窗口，只是没带具体哪张表），没有可用会话、或这个引擎写不了行时它是灰的、提示里说的是哪一种 ——
锁着的门比不存在的门好认。

### 连接树

分隔条可以拖（宽 220–640 px），鼠标移上去会出现一枚箭头，点一下就把整棵树**收起来**（面板宽度归 0，
内容裁掉但组件不卸载 —— 搜索框里的字与展开的节点都留着），再点一下展开。展开时给的是**最小宽度 220**：
收起那一刻宽度就不再有地方放了，所以要让树回到比这更宽，得再拖一次 —— **图标栏上的「连接」是另一条路**：
它记着收起前的宽度，点一下按那个宽度还给你。这份记忆在 `AppShell` 里而不是分隔条里：分隔条只知道自己
收起来那一刻的大小，不知道它本来有多宽。

点开 **数据生成** / **变更日志** / **设置**都是**切页面**，树于是跟着整层隐藏 —— 它们都是「要宽度」的页面
（变更日志左边列表右边语句，数据生成左边挑表右边一整张字段表，设置是一整页表单），树在那一刻帮不上忙。
想把它要回来点图标栏上的**连接**即可：从表右键开数据生成窗口也一样，窗口开在它自己的页面上，树随时一键就回来。

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
| 一个对象文件夹（`Tables` / `Views` …） | Open object list / New table…（`Tables` 才有）/ New DDL script…（关系型才有）/ Reload objects |
| 一个对象（表 / 视图…） | Open data / Design table（给不出设计器的引擎是 Open fields）/ Edit DDL…（关系型才有）/ Generate code…（有字段的对象才有）/ Data generation…（表 + 能 INSERT 的引擎才有）/ Copy name |
| 一个已保存的脚本 | Open / Rename… / Copy name / Show in folder / Delete |

**每个文件夹都带数量，空着也照样画。** 画哪几个文件夹是引擎事实，由后端在 `DriverInfo.objectKinds` 里给
（顺序也是它的：MongoDB 先集合后视图），前端只管画。早先的规则是「对象列表里出现过的种类才画文件夹」，
于是空库展开之后是一片空白 —— 分不清「这个库是空的」和「这个引擎没有这种文件夹」，而「是空的」
恰恰是用户想知道的事。索引装的是整个 namespace 的索引，是**另一次请求**，所以它跟着对象列表一起取：
展开数据库 / schema 时两个请求一起发，索引数量到了就在标题里补上，之后点开索引文件夹（或表的索引页）
是现成的；「Reload index list」仍是强制重取的那个入口。空文件夹在树里是**叶子** —— 标题里的 `(0)`
已经把话说完了，箭头点开只会是空的 —— 但点它照样打开（空的）对象列表。

**每个库下面还有一个 `Queries` 文件夹**（见下一节），它排在这几个文件夹后面（`Indexes` 之后）：脚本是本地文件，不是库里的
对象，所以它跟着库走而不是跟着 schema 走 —— PostgreSQL 的库即使有多个 schema，同一份脚本也只在这一处
（一条连到库上的脚本，从每个 schema 都该够得着；这是相对「对象挂在 schema 下」的**有意偏差**）。有 schema 的
库，同一套规则让 `Queries` 落在 schema 之后。这个
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

**`New Query` 认的是同一个焦点，门槛比 `Table` / `View` 低一档：只要焦点落在一个库上它就亮，
包括 PostgreSQL 的库节点** —— 库节点上 `Table` / `View` 不亮，是因为「这个库有哪些表」得先挑一个 schema，
而「开一个查询窗口」只是选定了这个库，不必替用户挑 schema。点下去开的窗口会把**当时站在哪个 schema 里**
一起带走（`openQueryTab(sessionId, database, schema)`），补全于是知道去哪张目录里找表名；
从库节点 / 收藏的脚本 / 标签页 `+` 开的窗口没有这一层信息，就只补已经缓存过的那些 schema ——
开窗口不该变成一次全库扫描，这一点在 `lib/tree.ts` 的 `catalogOf` 里写明了。提示语跟着焦点走
（`New query in <库>`，焦点不在库上时说「先在树里选一个库」），不是灰按钮加一句固定的话。

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
（同一份驱动列表，选完的配置直接落进这个分组）。展开与收起除了那个小三角，**双击分组这一行**也算 ——
图标旁边那个名字才是整行的落点，而「双击一行看里面有什么」本来就是这个树的手势（连接行双击开的是运行情况页）。
分组自己没有数据要取（成员早就在布局里了），所以这条手势只动「哪些行是开的」这一件事；空分组是叶子，
双击它不做任何事，小三角也不会因为这个手势而跟状态打架。

「New database…」问的是**这台服务器自己能接受什么**，而这份清单来自服务端：MySQL / TiDB 用 `SHOW CHARACTER SET` /
`SHOW COLLATION` 报出字符集与它可配的排序规则（服务器自己的默认值排在最前并预选），PostgreSQL 报出
编码与 locale（编码是一份引擎常量，locale 只能列出这个集群用过的几个，所以那一栏**可以手打**），
Doris 没有库级字符集、MongoDB 根本没有 `CREATE DATABASE` —— 这两种弹框里就只有库名加一句说明。
语句一律由后端按引擎渲染（MySQL 一族 `CREATE DATABASE … DEFAULT CHARACTER SET … COLLATE …`、
PostgreSQL `… ENCODING '…' LOCALE '…' TEMPLATE template0`、MongoDB `use <name>`），
**在窗口里先展示再执行**，前端只把请求递过去、不拼 SQL。SQLite 这类文件型引擎直接没有这个菜单项，
只读会话里它是灰的。

没有任何标签页时右侧就只有一页**项目信息**（`WelcomePane` 渲染 `AboutProject`；设置页的 **About**
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

### 表格列宽可以拖

每一张表里的表头都能在**右边缘**按住左右拖（数据网格、对象列表、结构页的字段 / 索引 / 外键、设计器的字段表、
数据生成的字段表都是同一套）：**没拖过的时候一切照旧** —— 宽度还是各表自己给的那份估计值，表格仍旧铺满窗格；
**拖过一次之后**，那一次拖动开始时屏幕上**每一列**的宽度都被记下来定死，表格改用 `table-layout: fixed`，
于是光标移动多少、列宽就变多少（最小 48px、最大 1200px），列里的长文本按列宽裁掉、以省略号收尾。

**为什么要「一次全冻住」而不是只定住被拖的那一列**：表格铺满窗格时，每列的实际宽度是浏览器分配出来的 ——
自动布局把窗格剩余的空间按各列**声明**的宽度成比例放大（一张只有三列的小表里，那个只装一个复选框的勾选列
也会被撑到一百多像素，就是这么来的）；你按住的只是其中一列，但拖宽它会让别的列当场被挤窄，
光标下那条边于是按另一种速度移动，手感是「打滑」。先把当前这一刻的宽度照单全收、再只动一列，
拖动才与光标的位移严格相等（量到的宽度之和本来就等于窗格宽度，所以冻住的那一刻屏幕上不会有什么东西挪位），
而且量到的就是真话：像对象列表的「名称 / 注释」或设计器的「描述」这类本来没有宽度的列，量到多少就是多少，
不必替它们猜一个默认值（只有「冻结之后才冒出来的新列」才落到 160px 这个兜底）。
**唯一的例外是勾选列**：它是 antd 自己插进来的、不归这套宽度管，改用固定布局后它回到自己声明的 36px ——
于是第一次拖动的瞬间整张表会往左挪这一点点（表越宽、剩的空间越少，挪得越少，多数时候看不出来）。

**拖过之后表格照样铺满窗格**：宽度既然是拖出来的，就不该再随窗口变化 —— 但右边也不该空出一条（那看着就像表格
被框在窗格的一角）。所以拖过之后会追加**一列没有内容的列**（`FILLER_KEY`，宽度写 `auto`）：固定布局里「剩下的空间」
只分给**没有宽度**的列，再叠上 antd 自己那句内联的 `min-width: 100%`，表格就正好等于窗格宽度。这不是偷偷又拉伸回去了 ——
空间永远落在那一列空列上、从不落到真列上，所以拖动仍然精确：把某列拉宽 10px，宽的就是这 10px，空列同时缩掉 10px，
光标下那条边不会跑。列加起来的宽度真超过窗格时，空列被压到只剩自己的内边距，表格横向滚动（而不是把列挤窄）。
这一路**不需要量任何东西**（没有 `ResizeObserver`、没有列宽记账），它只是让 CSS 自己去分那点剩余空间。

实现见
`frontend/src/components/ResizableHeader.tsx`（`useColumnResize`）：拖动把手是通过 `components.header.cell`
塞进 `<th>` 里的 —— 只有单元格自己知道右边缘在哪、自己有多宽，也顺带保证那个额外的 `dmResize` 属性不会跑到 DOM 上；
按住时用 `setPointerCapture` 接管指针，不必往 `window` 上挂监听，也就没有「拖到一半失去焦点」这种收尾路径要善后
（`onLostPointerCapture` 一条路就把状态与整个窗口的拖动光标一起收回）。行号、拖动抓手、行操作这类**装饰列**明确退出
（`resizable: false`）—— 在那里摆一条能拖的边只有噪音。

宽度是**窗口自己的事**：记在组件的状态里，切标签页再回来还在（这些面板挂上就不再卸载），关掉窗口就忘了 ——
那时候表很可能已经换了一张。也**不落盘**，不会有「上个版本拖窄了、这个版本怎么看不清」这种事。

### 点一行看这一行

数据网格里**点一下某一行**（点哪一格都行，勾选框也算），这一行的详情就从右边**滑进来**：每个字段按「字段名 / 值」
列出来，主键带 `PK` 标、`NULL` 显示成灰的 `NULL`，长文本折行。层里两个按钮把这一行复制走 —— **Copy as JSON**
（按字段名拼成对象）与**复制 INSERT**（`toInsertScript` 生成，跟导出走的是同一条路）。

- **它是滑进来的层，不是抽屉**：没用 antd 的 `Drawer` —— 那会盖一层整窗的遮罩、还会按固定宽度把内容挤开。
  这一层是网格自己的兄弟节点，绝对定位在网格区域上（`.dm-row-detail-layer`，`transform` 从 `translateX(100%)` 到 `0`），
  所以表格一列都不会被推走；关上（右上角的 × 或 `Esc`）也是滑回去的。层从挂载起就一直在（不打开时 `visibility: hidden`），
  否则一个「一出现就是打开状态」的元素不会滑动，只会闪一下。
- **它盖住表格，包括钉住的表头**：所以 `z-index` 比表头那层还高（表头是 `100`，它是 `200`）。
- **看一行不等于选中一行**：点行**不会**顺手把勾选框勾上（高亮走 `dm-grid-row is-detail` 与底色，不走 `rowSelection`）——
  否则「看一眼」与「批量删除的前一秒」就成了一件事。勾选框照旧只管批量操作；工具条上那个信息按钮（恰好勾中一行时可用）
  是它的另一个入口。
- **壳归 `DataGrid`，内容归调用方**：位置、动画、关闭按钮、`Esc` 与高亮都在网格里，而「这一行的字段是什么意思」
  只有调用方知道，所以内容是一个 `renderRowDetail(row, rowIndex)` 渲染函数（`TablePane` 传的就是上面那份字段清单）。
  查询窗口将来要接这一层，接上十来行就够。

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

### 自定义 Mock 占位符

数据生成窗口的 mock 列是一段 mock.js 模板，能用的占位符由构建定死。但「订单号 = 前缀 + 日期 + 4 位流水」
这种片段是每个库都有的，人手抄一遍不算什么，抄错一个字（`@nope`）就是当场一条错误 —— 所以用户自己可
以攒一批：`<数据目录>/.mock/<名字>.json`，一个占位符一个文件：

```json
{"version":1,"name":"orderNo","template":"SO@date(yyyy)@natural(1000, 9999)","description":"订单号"}
```

一个占位符一个文件的理由和脚本文件一样：这东西是一个人写的、可能要改、可能想用别的工具看，一文件夹的
小 JSON 能 diff，一大份不能。名字**就是文件名**（过 `escapeSegment`，跟查询树共用一把转义），所以名字
跑不出文件夹、撞不上 Windows 的保留设备名，也不会因为 `a/b` 落到别处；反过来说，**改名就是换文件**，
所以编辑已有占位符时名字那一栏是灰的（要改名字就删了重建），大小写只差一个的文件在 Windows / macOS
上本来就是同一个，保存时会把旧拼写删掉（同一条路径，同一个 `caseVariant`）。一份数据文件多出来就得多
进名单：`.mock` 已经加进 `dataDirs`，换数据目录时整棵树跟着搬、读不回来就报错，这个在
「数据目录」那节里说。

**自定义占位符是无参数的整段模板别名**，不是第二个引擎：它由内置占位符组成，不接参数（参数属于底下那
个内置的），既不能与内置重名（内置赢，表单直接不让存），也不能与自己兜圈。展开在**编译期就地**完成 ——
它的节点直接顶到父模板里，所以前后文字的顺序不变，而「整段就是一个占位符」的自定义仍旧只产生一个节
点，于是那个值的类型不变（`@dice` 给整数列一个整数）。循环靠一条**展开链**发现，报出来的是整条路径
（`in @a: in @b: @a expands into itself (@a → @b → @a)`），而不是一句「出错了」。描述栏里的说明来自自
定义的**描述**（没写描述时退回它内部那些占位符的说明）—— 在描述列里写「订单号」比写「@date + @natural」
有用。

校验是分开的：后端只管**形状**（名字能不能写成 `@name`、模板非空、长度、最多 200 个），因为模板语法
的唯一实现是前端那个引擎，在 Go 里再写一个差一些的解析器只会让两边对同一段模板有不同看法；**能不能
编译**由编辑器当场判（`checkCustomPlaceholder`，把草稿自己叠进去编译，所以 `@loop` 里写 `@loop` 会被
抓成循环），**会不会生成想要的东西**则由调试面板回答：一个显示在面板上的种子、一次渲染 5 行、`Reroll`
换一组。另有一个上限 200 条的理由很实际：取色器要把它们全画出来。

### 变更日志

**日志由执行语句的那一层写，不由窗口写** —— 这是它能有意义的前提。写入点就是能执行写语句的那几处：
`Manager.applyPlan`（结构页保存与新建表，它的 plan 是一条一条跑的，所以每条语句都有自己的结果）、
`Manager.Execute`（DDL 编辑器 / 查询窗口 / 新建库，**按这个引擎自己的解析器**切开 —— 有 `Analyzer` 的驱动
自己说，没有的走 `sqlutil` —— 只留 `ddl` / `dml`，
读语句与认不出的语句不记），以及 `Manager.UpdateRow` / `Manager.DeleteRow`（网格的行编辑与删行：语句由驱动渲染成文本，**同一份文本**既是预览也是日志那行，所以日志里写的就是发出去的那条）。窗口只负责把「这段脚本是写在哪个对象上的」告诉服务层（`ExecRequest` 多了
`schema` / `object` 两个字段），不负责写日志。列表上那个标签是**语句自己的首关键字**
（`create` / `alter` / `insert`…）—— 读日志是拿眼睛扫「做了什么」。关键字表读不出来的语句
（文档库的 `db.orders.insertOne(…)`，它的第一个词是 `db`）就用驱动自己给出的分类（`ddl` / `dml`）：
说 `db` 还不如不说。

**表名来自上下文，从不从 SQL 里解出来。** 结构页知道自己保存的是哪张表，DDL 编辑器知道自己在开哪张表的脚本，
查询窗口只知道自己站在哪个库上（于是表名是空的）—— 去 SQL 里找「这句话动过谁」得先写一个真的解析器，
而它一定会在第一个子查询上猜错。唯一例外是用户点名的那条规则：**`CREATE TABLE` 的记录里表名是空的**
（`sqlutil.CreatesTable`），因为那张表还不存在，而它的名字就写在语句里 —— 一条记录里的「表」回答的是
「这条语句作用在谁身上」，而不是「它提到了谁」。新建表计划里的 `CREATE INDEX` 反过来**是写着表名的**：
它是作用在那张新表上的语句，不是创建它的那句。

存法是**一行一条 JSON**（`<数据目录>/changelog.jsonl`），而不是跟其他文件一样的「一个文件一份 JSON 数组」。
理由是这个文件的使用方式跟它们不同：它被追加的次数远多于被读的次数，方向永远是往末尾加、从开头读，
而且它是这里唯一一个不需要用户同意就会长的文件。于是——一行读不出来的 JSON 只丢那一条（崩溃留下的半行、
手敲坏的一行），而不是整个日志打不开；读取走 `readChangeLogTail`（只看最后 8 MiB，且丢掉被截断的
第一行），所以别人往里塞过东西也不会把整个文件拉进内存。

**写满了是留档，不是丢弃**（`rotateChangeLogLocked`）。一个文件写到用户设的条数（默认 **2000**，存在
`<数据目录>/changelog.json` 的 `maxEntries` 里，设置页的 `Data folder` 页改它）就**整份改名**成
`<yyyymmdd>-N.log` —— 轮到它的那天，加上那天第几次轮转 —— 然后另起一个新的 `changelog.jsonl`。
用改名而不是重写/截断，是因为改名是**一步原子操作**：不复制、不重写，正好在读它的人要么拿到完完整整的旧文件、
要么拿到完完整整的新文件；于是「删掉最旧的几条」这件事根本不需要存在，用户见过的每一条都还在。一天里
轮转第二次就顺着取 `-2`，不会覆盖上一份。轮转阈值那个文件是**单独一份**而不是塞进 `state.json`：
轮转发生在后端的一次 append 里，那里的后端得能读它，而 `state.json` 是前端整份替换的、后端不该去读别人
拥有的文件。设回默认值就把这份文件删掉 —— 没选过和选成默认不是两种状态。`AppendChangeLog` 现在是
**追加**（不是整份重写）：该重写的理由（截断）已经随轮转一起消失了，剩下的一次小写反而更不容易留下半行。

于是日志不只是一份文件，而是一**架子**文件，读日志就是先选哪一份：`Store.ChangeLog` 会在一次调用里
同时给出选中的那份（最新在前）与全部文件（当前那份在前、留档按新旧排，每份带条数、大小、留档时间），
窗口顶部的下拉框就是这个架子 —— 分成两次调用的话，两次之间刚好发生一次轮转，列表与内容就会互相矛盾。
留档文件名是**轮转的那一刻**定下来的，所以它不可能事先写在 `dataFiles` 里：凡是需要知道数据目录里有
什么的地方（设置页的清单、搬家、拒绝目标目录、以及「还剩下什么不是本程序的」）都走同一个
`dataFileNames`，而它把目录扫描一遍取出 `isChangeLogArchive` 认得的名字 —— 一份留档会被列出来、被搬走、
且不会被当成别人落下的文件。名字是读留档的**唯一一道门**（`ChangeLogName` 白名单）：只认
`changelog.jsonl` 与那个形状的留档名，其余（路径、`connections.json`、`changelog.json`）一律拒绝，
不能被「洗干净」成恰好安全的东西。

**连接信息在写的时候拷一份**（名字、引擎、用户、地址），不在读的时候去查连接配置：配置会被改名、
改地址、删掉，而一个会改变主意的日志不是日志。写入是**尽力而为**的（`appendChange` 不看错误）：语句
在那之前已经跑完了，写不了日志说成「没改成」会让用户去找一张就在那儿的表 —— 丢的是历史，丢历史比丢事实轻。
脚本是一次调用交给引擎的，所以脚本跑到一半断掉时，这次运行的每条语句都带着同一句 `error`：记录说的是
引擎说过的话，不比引擎说得更多（这也是为什么界面上写的是「这次运行以这句话结束」而不是「这条语句失败了」）。

### 设置：主题、代码语言、Mock 占位符与数据目录

页面的形状是**左栏 + 一条滚动区**，而不是「卸掉上一段、挂上下一段」：五个栏目（外观 / 代码生成 / Mock 占位符 /
数据目录 / 关于）同时在场，栏目名跟着滚动位置高亮，点一下栏目就真的把内容滚到那一段去。所以「现在是第几段」
只有一个来源 —— 滚动位置 —— 点击只是去改它，而不是另存一份会跟滚动打架的状态。两处细节：点击自己引起的
滚动期间不读滚动位置（否则高亮会在动画途中把路过的几段都报一遍），以及最后一段可能短到永远滚不到顶部，
所以滚到底就当到底。也正因为五段同时在场，离开这一页再回来时它们都还在原处 —— 包括 Mock 占位符里正在写
的那张表单。

设置页自己**不存**任何偏好：主题与代码窗口的默认语言经过 store 落进 `state.json`，数据目录本来就是后端的事，所以走去别的页面
不会丢选择，两个窗口看到的也永远是同一份值。它也**不是弹窗**：弹窗是盖在手上的东西，总要有一个「关掉」，
而这里是去一趟再回来 —— 改了就生效，没有需要确认的动作，也没有哪个选择会因为走开而丢掉。`state.json` 是**整份替换**而不是合并写，所以落盘只有
一个出口（`saveUi`）—— 改主题时把语言一起写进去，反过来也一样，不然改一个偏好会把另一个抹掉。主题分成**偏好**（`light` / `dark` / `system`）与**画出来的**
两件事 —— 偏好写进状态文件的东西就是它自己，跟随系统时「现在到底什么颜色」另算（`resolvedTheme`），
页面据此给自己加 `data-theme` 与 `color-scheme`；系统那边的监听器只在这条偏好是 `system` 时才真的会
改变颜色（`resolveTheme` 对另外两档直接忽略系统），所以来回切主题不会积累监听器，旧版本留下的
`light` / `dark` 状态文件也照样读得进来（认不出的值回到 `light`，Navicat 的经典模样）。
设置里唯一**不进 `state.json`** 的是日志轮转阈值：它存在 `<数据目录>/changelog.json`，跟它管的那份
日志待在一起。理由是读它的人不是界面而是后端 —— 每次 append 都要知道写满没写满 —— 而 `state.json`
是前端整份替换的文件，后端去读一份别人拥有的文件迟早会被覆盖；何况把那份日志搬到另一台机器时，跟着
它走的应该是它自己的规矩。没改过（或改成默认值）时这份文件根本不存在：「没选过」与「选成默认」不该是
两种状态。

**数据目录的指针不在数据目录里**，而在默认位置（`%APPDATA%/db-manager`）的一张
`location.json`：`{"version":1,"dataDir":"…"}`。理由很直接 —— 数据目录本身要先被读出来，指针
不能跟着数据一起搬走，否则读指针的人得先知道指针在哪。文件走 tmp + rename 原子写：半张指针会被读成
「没有指针」，那就等于把用户悄悄送回默认目录。内容读不成（空、坏、相对路径、没有这个字段）一律回落
到默认目录，这个文件坏掉不该让程序打不开。**在指针里写下默认目录 = 清掉指针**，这样以后默认位置换了，
用户仍然跟着走。

搬家的顺序是**先复制再删**：逐个文件读出来、写到新目录、再读回来逐字节比一遍（`copyVerified`），七份
数据（连接 / 收藏 / 排法 / 状态 / `secret.key` / 变更日志 / 日志轮转设置）都到位之后才改指针，最后删旧文件；删除是尽力而为，
删不掉的留在原地并出现在结果里（`remaining`），绝不说一句「已移动」了事。所以任何一步失败，原目录都是
完好的（最坏只是多出一份没人用的副本）。拒绝的三种情况：目标就是当前目录（`sameDir` 认符号链接、
Windows 上还认大小写）、相对路径、以及目标里已经有**非空的**本程序数据（点名是哪个文件）。目录**可以
互相嵌套** —— 只搬名单里的文件、从不递归，所以谁在谁里面都行。不是本程序写的文件（`LeftBehind`）留着
不动并如实报告，其中不算指针文件与 `*.tmp`；目标目录如果正好在源目录里面，也不会被误报成「留下没搬」。
名单写死在 `dataFiles`（七个文件：连接配置、收藏、排法、状态、`secret.key`、变更日志、日志轮转设置）与 `dataDirs`
（`Queries` 文件夹那棵树即 `.query/`，加上自定义占位符那个 `.mock/`），
**新增一份数据文件 / 文件夹必须同时加进对应的名单**，否则搬家会把它落下（并因此在结果里报出来）；
唯一的例外是**留档的日志**，它们的名字要到轮转那一刻才存在，所以靠目录扫描认（见「变更日志」，同一个
`dataFileNames` 供清单、搬家与「剩下什么」共用）。
目录是按文件逐个复制校验的（`copyTree` + `copyVerified`，跳过 `*.tmp`），目标目录里已经有一份非空的
`.query` 或 `.mock` 同样算「已有本程序数据」而被拒绝。不这么搬的话，换一次数据目录就会把用户存的脚本
与占位符全部丢在原地。

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
无 `WHERE` 的 `DELETE` 要算），以及变更日志用的那两个判断（`CreatesTable`、`LeadingWord` ——
「`CREATE TABLE` 的记录不写表名」就是前者），不依赖任何数据库；ER 图在 SQLite 上端到端跑一遍
（外键方向、主键/可空标记、跨命名空间的目标名），service 层再用一个只实现 `Conn`
的包装验证「没有 `Grapher` 时退化成逐对象 `Structure`」这条路；建库同一条路子：
先用 SQLite 确认「没有这个能力就说 `unsupported`」，再加上 `drivers.DatabaseCreator` 的包装确认
服务端给的选项与渲染好的语句原样透传。搬家的规则全在 `internal/config/location_test.go` 里：
默认目录、文件清单、带着数据搬去新目录并对留在原地的文件如实报告、搬回默认目录、
拒绝自己（`sameDir`）与相对路径、拒绝已经有非空数据的目录（零字节同名文件不算数据）、
目标在源目录里面（只搬名单里的文件，不递归）、指针文件读不成时回落默认、目录被删了会重建；
`internal/service/datadir_test.go` 再验证运行中的程序真的换了 store —— 保存、搬家、旧目录里不再
有连接配置、已连上的会话还在、下一次保存落在新目录，以及搬回默认目录时指针会被清掉。
变更日志同样是两层：`internal/config/changelog_test.go` 盯住格式、寿命与轮转（往返、最新在前、
**写满一个文件是整份改名留档而不是丢掉最旧的几条**、同一天第二次轮转取 `-2`、读不出来的那一行只丢自己、
只认自己写过的文件名 —— `connections.json` 与 `../connections.json` 一律拒、留档名的形状要真的是一天
（`20260230-1.log` / `2026-1.log` / `-0` 不算）、阈值文件往返与坏掉时回落默认、搬家把当前日志与留档
一起带走、目标目录里只躺着一份留档也会被认出来），`internal/service/changelog_test.go` 则用真 SQLite
会话跑一遍「哪些语句会进日志」（写语句进、`SELECT` 与认不出的语句不进、`CREATE TABLE` 不带表名、
`CREATE INDEX` 带着、结构页每条都有自己的结果、失败时这次运行的每条都带同一句 `error`），
外加「选一份留档读得到它的记录，并且文件清单里两份都在」，以及阈值只能是数字且在上下限里。
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
  - [x] 表设计器：字段的可视化编辑（增删、行首抓手拖动排序）+ 保存前的语句确认（Navicat 的「结构」页）
  - [x] 查询收藏：命名 SQL 片段，查询窗口里可载入 / 改名 / 删除
  - [x] DDL 编辑器：对象定义开成可编辑脚本，附逐条语句的干跑预警
  - [x] 查询窗口：在库上直接新开（命令条 `New Query` 与标签页 `+`），编辑器按窗口所在库 / schema 补全表与视图，草稿窗口的 Save 先问名字再落成文件
  - [x] 查询计划与格式化：Explain 只看不跑（Results / Plan 两档切换），Format 按文法重新缩进
  - [x] ER 图：命名空间的关系图，可搜索 / 缩放 / 导出 SVG，点节点开表
  - [x] 新建数据库：字符集 / 排序规则（MySQL 一族）、编码与 locale（PostgreSQL）由服务端回答，语句先展示再执行
  - [x] 运行情况：双击连接看服务端现状，每个引擎一个视图
  - [x] 变更日志：图标栏的「变更日志」一页看这个程序跑过的每一句写语句（时间 / 连接 / 库 / 表 / 语句），存在数据目录里，没连库也能打开；写满了就整份留档成 `20260214-1.log`（阈值可设，默认 2000 条），历史日志在同一个窗口里选着看
  - [x] 数据生成：按 mock.js 模板填表 —— 每列一段模板（可勾选、自增列默认不勾）、一个连接一个窗口、占位符取色器与自定义占位符（`<数据目录>/.mock`）
  - [x] 页面栏：连接 / 数据生成 / 变更日志 / 设置四页各占一页，切走不丢状态（数据生成里写一半的 mock 仍在）；分隔条收起后用图标栏上的「连接」就能回来
  - [x] 连接排序与分组：整行拖动换位置，一层深的分组文件夹，排法存在 `layout.json` 并与实时配置对齐
- [x] Phase 2：MongoDB（连接、集合浏览、shell 查询、增删改、索引、运行情况）
- [x] Phase 2.5：TiDB / Apache Doris（同一种线协议共用 `mysqlcompat`：TiDB 带 TLS 与集群成员概览，Doris 只读浏览 + DDL 编辑）
- [ ] Phase 3：Oracle / SQL Server（占位与 dialect 已在，缺 DSN 与目录查询，`planned.Parked()`）
- [ ] Phase 4：SSH 隧道、导入向导、数据对比、插件式扩展

## 许可证

[MIT](LICENSE)
