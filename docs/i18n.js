/*
 * Copy for the introduction site, in three languages.
 *
 *   en      English — the default, and the language every fallback lands on
 *   zh-CN   简体中文
 *   zh-TW   繁體中文
 *
 * `index.html` addresses a key from the markup (`data-i18n` for text,
 * `data-i18n-html` for the two strings that carry an `<em>`,
 * `data-i18n-attr="attr:key;attr:key"` for attributes) and `site.js` through
 * `t()`, so adding a language is a matter of adding a dictionary here — the
 * markup never changes. Brand names (MySQL, PostgreSQL, SQLite, MongoDB, TiDB,
 * Apache Doris, mock.js, Wails) and code (`chmod +x db-manager`) stay as they
 * are in every language; that is why a few values repeat.
 *
 * `scripts/docs-check.mjs` fails when the three dictionaries drift apart or
 * when the markup asks for a key that does not exist, so a typo is a red build
 * rather than an empty line on the page.
 */
(function () {
  'use strict'

  var EN = {
    'meta.title': 'db-manager — a desktop database client for six engines',
    'meta.description':
      'An offline-first desktop database client for six engines: export a whole database, run a .sql file statement by statement, generate test data, compare two databases, design tables, generate code — and log every statement the app runs. MIT licensed, for Windows, macOS and Linux.',

    'nav.screens': 'Screens',
    'nav.features': 'Features',
    'nav.engines': 'Engines',
    'nav.download': 'Download',
    'top.sections': 'Sections',
    'lang.group': 'Interface language',

    'hero.eyebrow': 'MySQL · MariaDB · PostgreSQL · SQLite · MongoDB · TiDB · Apache Doris',
    'hero.title': 'Six engines, one desktop client, and the tools that <em>actually do the work</em>',
    'hero.lead':
      'Browse and edit data, design tables, export a whole database, run a .sql file statement by statement, generate test data, compare two databases, turn a table into code — and keep a log of every statement the app runs. Connections, favourites and logs stay on this machine: no account, no telemetry, no cloud.',
    'hero.download': 'Download the latest release',
    'hero.source': 'Source on GitHub',
    'hero.screens': 'See the screens',
    'hero.pill.mit': 'MIT licensed',
    'hero.pill.platforms': 'Windows · macOS · Linux',
    'hero.pill.offline': 'Offline-first',
    'hero.pill.six': 'Six engines',

    'screens.title': 'Screens',
    'screens.lead':
      'Six pages of the app, playing by themselves — hover to pause, or jump with the thumbnails below.',
    'screens.all': 'Every screen',

    'carousel.prev': 'Previous screenshot',
    'carousel.next': 'Next screenshot',
    'carousel.play': 'Play',
    'carousel.pause': 'Pause',
    'carousel.close': 'Close',
    'carousel.zoom': 'Open this screenshot larger',
    'carousel.go-to': 'Screenshot {n}',

    'shot.main.title': 'Workspace',
    'shot.main.desc':
      'Connections on the left, objects and data on the right. Pages stay mounted when you switch away, so coming back keeps your place — a half-written mock or an unsaved editor included.',
    'shot.main.chips': 'connection tree · object list · data grid · tabs',

    'shot.connections.title': 'Connections',
    'shot.connections.desc':
      'Pick a driver first, then fill in the page that belongs to it: host, port, account, TLS and DSN options for a server; a single file and attach aliases for SQLite. Groups, colours and read-only marks live in the same tree.',
    'shot.connections.chips': '6 drivers · TLS · groups · read-only',

    'shot.data-generation.title': 'Data generation',
    'shot.data-generation.desc':
      'One window per connection, one mock.js template per column, auto-increment columns left unchecked. Each run ends with a sentence: how many rows went in, how many were skipped, and how long it took.',
    'shot.data-generation.chips': 'mock.js templates · placeholders · batches · stop',

    'shot.database-compare.title': 'Database comparison',
    'shot.database-compare.desc':
      'Two databases of the same engine, side by side: only left, only right, differs, same. Open a table to see which column or index disagrees, then generate the script that turns the right one into the left one.',
    'shot.database-compare.chips': 'per-table diff · sync script · skipped objects warned',

    'shot.change-log.title': 'Change log',
    'shot.change-log.desc':
      'Every write statement the app was asked to run, with the connection it ran on and the number of rows the engine reported. One plain-text file per day, rotated rather than trimmed — readable with nothing connected.',
    'shot.change-log.chips': 'one file per day · affected rows · works offline',

    'shot.settings.title': 'Settings',
    'shot.settings.desc':
      'Appearance, interface language, default code language, mock placeholders, data generation limits, the data folder and About — seven sections sharing one scrolling area, each one taking effect as you change it.',
    'shot.settings.chips': 'theme · language · mock · data folder',

    'features.title': 'What is in the box',
    'features.lead':
      'Every tool is built on the same driver contract, so it is offered when the engine can serve it — an entry point that would fail is not drawn at all.',

    'feat.explorer.title': 'Connections and object tree',
    'feat.explorer.desc':
      'Six engines behind one lazy tree: sessions, databases, schemas, then tables, views and indexes — every folder with its count, empty or not. Sort and group the list, tag a connection with a colour, mark it read-only.',

    'feat.grid.title': 'Data grid',
    'feat.grid.desc':
      'Server-side paging, sorting and fourteen filter operators. Drag a column edge to resize it, click a row to slide its detail panel in, edit one cell or several fields at once — every write is parameter bound and shown before it runs.',

    'feat.designer.title': 'Table designer',
    'feat.designer.desc':
      'The structure page is the editor: rename, retype, flag or drag-reorder the columns and the backend plans the DDL against the live catalogue. Preview and save go through the same planner, so what you read is what runs.',

    'feat.export.title': 'Database export',
    'feat.export.desc':
      "Structure, structure and data, or a single table's rows as CSV / JSON / JSONL / SQL. The backend reads the table and writes the file at the same time, so the progress bar, the byte count and Stop are all real.",

    'feat.sqlfile.title': 'Run SQL file',
    'feat.sqlfile.desc':
      'Hand a .sql file to the backend: it reads it from disk, tells you what is inside — the statements that lose data, the ones a read-only connection will refuse, the USE that re-targets the rest — then runs it statement by statement with progress and Stop. No transaction wraps the file, and the window says so.',

    'feat.datagen.title': 'Data generation',
    'feat.datagen.desc':
      'Fill a table from mock.js templates, one per column, with a placeholder picker that renders a sample value for every entry. A rejected row stops the run, or gets counted and skipped — your call.',

    'feat.compare.title': 'Database comparison',
    'feat.compare.desc':
      'Compare two databases of the same engine table by table and generate the script that turns the right one into the left one, ordered create, alter, drop. Only the right side may be read-only — the left is often production.',

    'feat.codegen.title': 'Code generation',
    'feat.codegen.desc':
      "Turn a table's columns into a class, struct or record in one of eighteen languages, with nullability written the way each language writes it. Java comes with or without Lombok.",

    'feat.explain.title': 'Explain and format',
    'feat.explain.desc':
      "Ask the engine how it plans to run a statement without running it, and re-indent a script with the dialect's own grammar. Highlighting is done by the app, not by a lexer library, because quoting rules differ per engine.",

    'feat.er.title': 'ER diagram',
    'feat.er.desc':
      'One namespace on a canvas: tables of the same family in one row, ordered by foreign keys, with the referenced column pair on hover. Search, pan, zoom and export the drawing as a standalone SVG.',

    'feat.changelog.title': 'Change log',
    'feat.changelog.desc':
      'Every write statement the app ran, with its affected rows and the connection it ran on. The log lives in the data folder — one file per day, rotated, never trimmed — so it opens with no database connected.',

    'feat.i18n.title': 'English, 简体中文, 繁體中文',
    'feat.i18n.desc':
      'The whole interface is translated, antd components included. Switch it from the settings page, the status bar at the bottom, or — on Windows — the tray menu, and nothing restarts.',

    'engines.title': 'Engines',
    'engines.lead':
      'The driver layer does not assume a relational model, so a document engine goes through the same contract — and an entry point the engine cannot serve is simply not drawn.',
    'engines.planned.title': 'On the roadmap',
    'engines.planned':
      'Oracle and SQL Server: the dialect and the placeholder are already in the tree, the DSN and the catalogue queries are not — so they stay off the menu until they are.',

    'engine.mysql.name': 'MySQL & MariaDB',
    'engine.mysql.desc':
      'The home turf: TLS, charset and collation when a database is created, partitions, and the InnoDB buffer pool plus process list on the overview page.',
    'engine.postgres.name': 'PostgreSQL',
    'engine.postgres.desc':
      'Several schemas in one database, serial and identity columns, and encoding plus locale when a database is created.',
    'engine.sqlite.name': 'SQLite',
    'engine.sqlite.desc':
      'A single file: no server, no account. Attach extra databases, read the pragma values, and browse the file the way the server engines are browsed.',
    'engine.mongodb.name': 'MongoDB',
    'engine.mongodb.desc':
      'Collections, documents, indexes and a mongosh-style shell for queries. Document counts, sizes and dbStats on the overview page.',
    'engine.tidb.name': 'TiDB',
    'engine.tidb.desc':
      'The MySQL wire protocol with TLS, plus a cluster member table from information_schema.CLUSTER_INFO.',
    'engine.doris.name': 'Apache Doris',
    'engine.doris.desc':
      'Analytical, and read-only here on purpose: structure, indexes, the data grid, SQL and export all work; there is no table designer, and the app says why instead of failing.',

    'download.title': 'Download',
    'download.lead': 'Every release is built by CI on three runners and attached to the release:',
    'dl.windows.name': 'Windows x64',
    'dl.windows.note': 'Portable — no installer. WebView2 is required (Windows 10/11 already has it).',
    'dl.macos.name': 'macOS (Intel + Apple Silicon)',
    'dl.macos.note':
      'Unzip and drag db-manager.app into Applications. The build is not signed or notarised: right-click → Open the first time, or run xattr -cr.',
    'dl.linux.name': 'Linux x64',
    'dl.linux.note': 'chmod +x db-manager && ./db-manager — needs GTK3 and WebKitGTK 4.0.',
    'dl.checksums.title': 'Checksums',
    'dl.checksums.note': 'Each release carries a checksums.txt next to the downloads:',
    'dl.build.title': 'Build from source',
    'dl.build.note':
      'Go 1.24+, Node 20+, the Wails CLI 2.16 and just. `just release` compiles the current platform into release/ with a checksums file; `just publish <x.y.z> "summary"` tags a release.',
    'dl.build.link': 'Read the build instructions',
    'dl.notes.title': 'Release notes',
    'dl.notes.note':
      'Each release body is rendered from the annotated tag and the Conventional Commit subjects since the previous one — the commit subjects are the release notes.',

    'data.title': 'Where your data lives',
    'data.lead':
      'Connections, favourites, saved query files, the tree layout, the window state, the encryption key and the change log all live in one data folder — %APPDATA%/db-manager on Windows, the usual config folder (~/.config, ~/Library/Application Support) elsewhere.',
    'data.point.local':
      'No account, no telemetry, nothing uploaded — the app talks to your databases and to nothing else.',
    'data.point.passwords':
      'Passwords are saved only when you ask for it: sealed with AES-256-GCM, with the key in its own file next to the store.',
    'data.point.log-text':
      'The change log is plain text. Grep it, archive it, read it in another editor.',
    'data.point.move':
      'Moving the folder is a copy, a byte-for-byte verification, then a pointer switch — and sessions stay connected while it happens.',

    'footer.tagline': 'An offline-first database client for six engines.',
    'footer.project': 'Project',
    'footer.repo': 'Repository',
    'footer.releases': 'Releases',
    'footer.issues': 'Issues',
    'footer.developer': 'Developer',
    'footer.readme': 'README',
    'footer.license': 'MIT licensed',
    'footer.built': 'Built with Wails, Go and React.',
  }

  var ZH_CN = {
    'meta.title': 'db-manager —— 六引擎桌面数据库客户端',
    'meta.description':
      '离线优先的桌面数据库客户端，支持六种引擎：整库导出、按语句跑一份 .sql 文件、数据生成、数据库比对、表设计、代码生成，并把程序跑过的每一句写语句记进变更日志。MIT 许可，Windows / macOS / Linux。',

    'nav.screens': '界面',
    'nav.features': '功能',
    'nav.engines': '支持的引擎',
    'nav.download': '下载',
    'top.sections': '页面区域',
    'lang.group': '界面语言',

    'hero.eyebrow': 'MySQL · MariaDB · PostgreSQL · SQLite · MongoDB · TiDB · Apache Doris',
    'hero.title': '六种引擎、一个桌面客户端，和一套<em>真能干活的</em>工具',
    'hero.lead':
      '浏览和编辑数据、设计表、导出整个库、把一份 .sql 文件按语句跑下去、生成测试数据、比较两个库、把一张表翻成代码，并把程序跑过的每一句写语句记成日志。连接、收藏和日志都留在本机：不需要账号、不上报、不联网也能用。',
    'hero.download': '下载最新版本',
    'hero.source': 'GitHub 源码',
    'hero.screens': '看看界面',
    'hero.pill.mit': 'MIT 许可',
    'hero.pill.platforms': 'Windows · macOS · Linux',
    'hero.pill.offline': '离线优先',
    'hero.pill.six': '六种引擎',

    'screens.title': '界面预览',
    'screens.lead': '应用的六页，自动轮播 —— 鼠标移上去就暂停，也可以点下面的缩略图直接跳过去。',
    'screens.all': '全部界面',

    'carousel.prev': '上一张',
    'carousel.next': '下一张',
    'carousel.play': '播放',
    'carousel.pause': '暂停',
    'carousel.close': '关闭',
    'carousel.zoom': '放大看这一张',
    'carousel.go-to': '第 {n} 张',

    'shot.main.title': '工作区',
    'shot.main.desc':
      '左边是连接，右边是对象与数据。切走的页面不会被卸载，切回来还在原处 —— 连写了一半的 mock、没保存的编辑器都还在。',
    'shot.main.chips': '连接树 · 对象列表 · 数据网格 · 标签页',

    'shot.connections.title': '连接管理',
    'shot.connections.desc':
      '先挑驱动，再落到这个引擎自己的那一页：走网络的填地址、端口、账号、TLS 与 DSN 参数，SQLite 只要一个文件加附加库别名。分组、颜色和只读标记都在同一棵树里。',
    'shot.connections.chips': '六种驱动 · TLS · 分组 · 只读',

    'shot.data-generation.title': '数据生成',
    'shot.data-generation.desc':
      '一个连接一个窗口，每列一段 mock.js 模板，自增列默认不勾。每一轮跑完都留下一句话：进去多少行、跳过多少行、用了多久。',
    'shot.data-generation.chips': 'mock.js 模板 · 占位符 · 分批 · 停止',

    'shot.database-compare.title': '数据库比较',
    'shot.database-compare.desc':
      '两个同引擎的库摆在一起：只有左边有、只有右边有、不一样、一样。点开一条看是哪个字段或索引不同，再生成「把右边改成左边」的脚本。',
    'shot.database-compare.chips': '逐表差异 · 同步脚本 · 跳过的进警告',

    'shot.change-log.title': '变更日志',
    'shot.change-log.desc':
      '这个程序被要求跑过的每一句写语句，带着它在哪个连接上跑的、引擎报了多少行。一天一个纯文本文件，写满就整份留档、从不丢弃 —— 没连库也能打开来看。',
    'shot.change-log.chips': '一天一个文件 · 影响行数 · 离线可读',

    'shot.settings.title': '设置',
    'shot.settings.desc':
      '外观、界面语言、默认代码语言、Mock 占位符、数据生成上限、数据目录和关于 —— 七段共用一条滚动区，改了就生效，没有「取消」要按。',
    'shot.settings.chips': '主题 · 语言 · Mock · 数据目录',

    'features.title': '工具箱里有什么',
    'features.lead':
      '所有工具都长在同一套驱动契约上：引擎能做到的才给入口，做不到的干脆不摆出来，而不是点了才报错。',

    'feat.explorer.title': '连接与对象树',
    'feat.explorer.desc':
      '六种引擎共用一棵懒加载树：会话 → 库 → schema → 表 / 视图 / 索引，每个文件夹都带数量、空着也画。列表可以拖动排序、建分组，连接可以打颜色标签、标成只读。',

    'feat.grid.title': '数据网格',
    'feat.grid.desc':
      '分页、排序与 14 种过滤操作符都由服务端做。拖表头边缘改列宽，点一行从右侧滑入这一行的详情，改一个单元格或一次改多个字段 —— 每条写语句都走参数绑定，而且先摆出来给你看过。',

    'feat.designer.title': '表设计器',
    'feat.designer.desc':
      '「结构」页就是编辑器：改名字、改类型、加标记、拖行首抓手排序，后端拿实时目录规划 DDL。预览与保存走同一个规划器，看到什么就跑什么。',

    'feat.export.title': '数据库导出',
    'feat.export.desc':
      '结构、结构与数据、或单表的行导出成 CSV / JSON / JSONL / SQL。整个导出在后端一边读一边直接写盘，所以进度条、字节数与 Stop 都是真的。',

    'feat.sqlfile.title': '运行 SQL 文件',
    'feat.sqlfile.desc':
      '把一份 .sql 交给后端：它从磁盘上读，先告诉你里面有什么 —— 哪几条会丢数据、哪几条会被只读连接拒掉、USE 会把后面重定向到哪 —— 然后一条语句一次调用地跑，带进度与 Stop。文件没有事务包着，窗口上直说这一点。',

    'feat.datagen.title': '数据生成',
    'feat.datagen.desc':
      '按 mock.js 模板填表，一列一段模板，取色器会给每个占位符渲染一行示例值。引擎拒掉一行时是停下、还是跳过并数出来，由你决定。',

    'feat.compare.title': '数据库比较',
    'feat.compare.desc':
      '两个同引擎的库逐表对比，生成「把右边改成左边」的脚本，按先建、再改、最后删排。只有右边可以是只读的 —— 左边常常是拿来当参照的生产库。',

    'feat.codegen.title': '代码生成',
    'feat.codegen.desc':
      '把一张表的字段翻成 18 种语言里的类 / 结构体 / 记录，可空按各语言的写法给。Java 分带 Lombok 与素写两档。',

    'feat.explain.title': '查询计划与格式化',
    'feat.explain.desc':
      '问引擎「这条你打算怎么跑」，而不真的跑；再按这个引擎自己的文法重新缩进脚本。高亮由程序自己扫字符得到，不引第三方词法库 —— 因为各引擎的引号规则不一样。',

    'feat.er.title': 'ER 图',
    'feat.er.desc':
      '把一个命名空间画在一张图上：同一家的表排成一行，行内按外键层级排，箭头悬停显示哪一列引用哪一列。可以搜索、拖动、缩放，也能导出成自包含的 SVG。',

    'feat.changelog.title': '变更日志',
    'feat.changelog.desc':
      '程序跑过的每一句写语句，带着影响行数与它所在的连接。日志就在数据目录里 —— 一天一个文件、只轮转不丢弃 —— 所以没连库也能打开。',

    'feat.i18n.title': 'English、简体中文、繁體中文',
    'feat.i18n.desc':
      '整个界面都译了，antd 组件也在内。设置页、底部状态栏、以及 Windows 的托盘菜单都能切，切完不用重启。',

    'engines.title': '支持的引擎',
    'engines.lead':
      '驱动层不假设关系模型，所以文档型引擎走的是同一套契约；引擎给不出的入口（MongoDB 的表设计器、数据生成与 Explain，Doris 的表设计器）不摆出来，而不是点了才报错。',
    'engines.planned.title': '规划中',
    'engines.planned':
      'Oracle 与 SQL Server：方言与占位已经在树里，缺 DSN 与目录查询 —— 在那之前不上菜单。',

    'engine.mysql.name': 'MySQL & MariaDB',
    'engine.mysql.desc':
      '最常用的一档：TLS、建库时的字符集与排序规则、分区表，以及运行情况页上的 InnoDB 缓冲池与进程列表。',
    'engine.postgres.name': 'PostgreSQL',
    'engine.postgres.desc':
      '一个库里几个 schema，serial 与 identity 列，建库时的编码与 locale。',
    'engine.sqlite.name': 'SQLite',
    'engine.sqlite.desc':
      '一个文件就是一个库：不用服务端、不用账号。可以附加别的库、读 pragma，像浏览服务端引擎那样浏览这个文件。',
    'engine.mongodb.name': 'MongoDB',
    'engine.mongodb.desc':
      '集合、文档、索引，查询窗口跑的是 mongosh 风格的 shell；运行情况页给出文档数、体积与 dbStats。',
    'engine.tidb.name': 'TiDB',
    'engine.tidb.desc':
      '讲 MySQL 线协议、有 TLS，运行情况页多一张 information_schema.CLUSTER_INFO 的集群成员表。',
    'engine.doris.name': 'Apache Doris',
    'engine.doris.desc':
      '分析型引擎，在这里有意做成只读浏览：结构、索引、数据网格、SQL 与导出都可用；没有表设计器，而且会说明原因，而不是点了才失败。',

    'download.title': '下载',
    'download.lead': '每个版本都由 CI 在三个 runner 上构建，产物直接挂在 Release 上：',
    'dl.windows.name': 'Windows x64',
    'dl.windows.note': '免安装，双击即可。需要 WebView2 运行时（Windows 10/11 自带）。',
    'dl.macos.name': 'macOS（Intel + Apple Silicon 通用）',
    'dl.macos.note':
      '解压后把 db-manager.app 拖进「应用程序」。包未做签名与公证：首次打开请右键「打开」，或执行 xattr -cr。',
    'dl.linux.name': 'Linux x64',
    'dl.linux.note': 'chmod +x db-manager && ./db-manager —— 需要 GTK3 与 WebKitGTK 4.0。',
    'dl.checksums.title': '校验',
    'dl.checksums.note': '每个 Release 都带一份 checksums.txt：',
    'dl.build.title': '从源码构建',
    'dl.build.note':
      'Go 1.24+、Node 20+、Wails CLI 2.16 和 just。`just release` 把当前平台的产物打到 release/ 并附校验和；`just publish <x.y.z> "本版总结"` 发版。',
    'dl.build.link': '看构建说明',
    'dl.notes.title': '版本说明',
    'dl.notes.note':
      '每个 Release 的正文由附注标签与本版提交信息渲染而成 —— 所以提交信息写清楚，release notes 就清楚。',

    'data.title': '数据放在哪',
    'data.lead':
      '连接配置、查询收藏、保存的脚本、树的排法、窗口状态、密码密钥和变更日志都在同一个数据目录里 —— Windows 上是 %APPDATA%/db-manager，其它平台是各自惯例的位置（~/.config、~/Library/Application Support）。',
    'data.point.local': '不需要账号、没有遥测、什么都不上传 —— 只跟你的数据库说话。',
    'data.point.passwords':
      '密码只有你要求才保存：AES-256-GCM 封装，密钥单独放一个文件，和配置放在一起。',
    'data.point.log-text': '变更日志是纯文本：可以 grep、可以归档、可以拿别的编辑器打开。',
    'data.point.move':
      '换目录是真的把数据搬过去：先逐个复制、逐字节校验，然后才改指针 —— 搬的时候会话照旧连着。',

    'footer.tagline': '离线优先的六引擎数据库客户端。',
    'footer.project': '项目',
    'footer.repo': '仓库',
    'footer.releases': '版本发布',
    'footer.issues': '问题反馈',
    'footer.developer': '开发者',
    'footer.readme': 'README',
    'footer.license': 'MIT 许可',
    'footer.built': '用 Wails + Go + React 构建。',
  }

  var ZH_TW = {
    'meta.title': 'db-manager —— 六引擎桌面資料庫用戶端',
    'meta.description':
      '離線優先的桌面資料庫用戶端，支援六種引擎：整庫匯出、按語句跑一份 .sql 檔案、資料生成、資料庫比對、表設計、程式碼產生，並把程式跑過的每一句寫語句記進變更日誌。MIT 授權，Windows / macOS / Linux。',

    'nav.screens': '介面',
    'nav.features': '功能',
    'nav.engines': '支援的引擎',
    'nav.download': '下載',
    'top.sections': '頁面區域',
    'lang.group': '介面語言',

    'hero.eyebrow': 'MySQL · MariaDB · PostgreSQL · SQLite · MongoDB · TiDB · Apache Doris',
    'hero.title': '六種引擎、一個桌面用戶端，和一套<em>真能幹活的</em>工具',
    'hero.lead':
      '瀏覽和編輯資料、設計表、匯出整個資料庫、把一份 .sql 檔案按語句跑下去、產生測試資料、比較兩個資料庫、把一張表翻成程式碼，並把程式跑過的每一句寫語句記成日誌。連線、收藏和日誌都留在本機：不需要帳號、不上報、不連網也能用。',
    'hero.download': '下載最新版本',
    'hero.source': 'GitHub 原始碼',
    'hero.screens': '看看介面',
    'hero.pill.mit': 'MIT 授權',
    'hero.pill.platforms': 'Windows · macOS · Linux',
    'hero.pill.offline': '離線優先',
    'hero.pill.six': '六種引擎',

    'screens.title': '介面預覽',
    'screens.lead': '應用程式的六頁，自動輪播 —— 滑鼠移上去就暫停，也可以點下面的縮圖直接跳過去。',
    'screens.all': '全部介面',

    'carousel.prev': '上一張',
    'carousel.next': '下一張',
    'carousel.play': '播放',
    'carousel.pause': '暫停',
    'carousel.close': '關閉',
    'carousel.zoom': '放大看這一張',
    'carousel.go-to': '第 {n} 張',

    'shot.main.title': '工作區',
    'shot.main.desc':
      '左邊是連線，右邊是物件與資料。切走的頁面不會被卸載，切回來還在原處 —— 連寫了一半的 mock、沒儲存的編輯器都還在。',
    'shot.main.chips': '連線樹 · 物件清單 · 資料網格 · 分頁標籤',

    'shot.connections.title': '連線管理',
    'shot.connections.desc':
      '先挑驅動，再落到這個引擎自己的那一頁：走網路的填位址、連接埠、帳號、TLS 與 DSN 參數，SQLite 只要一個檔案加附加資料庫別名。分組、顏色和唯讀標記都在同一棵樹裡。',
    'shot.connections.chips': '六種驅動 · TLS · 分組 · 唯讀',

    'shot.data-generation.title': '資料生成',
    'shot.data-generation.desc':
      '一個連線一個視窗，每欄一段 mock.js 範本，自增欄位預設不勾。每一輪跑完都留下一句話：進去多少列、跳過多少列、用了多久。',
    'shot.data-generation.chips': 'mock.js 範本 · 佔位符 · 分批 · 停止',

    'shot.database-compare.title': '資料庫比對',
    'shot.database-compare.desc':
      '兩個同引擎的資料庫擺在一起：只有左邊有、只有右邊有、不一樣、一樣。點開一條看是哪個欄位或索引不同，再產生「把右邊改成左邊」的腳本。',
    'shot.database-compare.chips': '逐表差異 · 同步腳本 · 跳過的進警告',

    'shot.change-log.title': '變更日誌',
    'shot.change-log.desc':
      '這個程式被要求跑過的每一句寫語句，帶著它在哪個連線上跑的、引擎報了多少列。一天一個純文字檔案，寫滿就整份留檔、從不丟棄 —— 沒連資料庫也能打開來看。',
    'shot.change-log.chips': '一天一個檔案 · 影響列數 · 離線可讀',

    'shot.settings.title': '設定',
    'shot.settings.desc':
      '外觀、介面語言、預設程式碼語言、Mock 佔位符、資料生成上限、資料目錄和關於 —— 七段共用一條捲動區，改了就地生效，沒有「取消」要按。',
    'shot.settings.chips': '主題 · 語言 · Mock · 資料目錄',

    'features.title': '工具箱裡有什麼',
    'features.lead':
      '所有工具都長在同一套驅動契約上：引擎能做到的才給入口，做不到的乾脆不擺出來，而不是點了才報錯。',

    'feat.explorer.title': '連線與物件樹',
    'feat.explorer.desc':
      '六種引擎共用一棵懶載入樹：連線 → 資料庫 → schema → 表 / 檢視 / 索引，每個資料夾都帶數量、空著也畫。清單可以拖曳排序、建分組，連線可以打顏色標籤、標成唯讀。',

    'feat.grid.title': '資料網格',
    'feat.grid.desc':
      '分頁、排序與 14 種過濾運算子都由伺服器端做。拖表頭邊緣改欄寬，點一列從右側滑入這一列的詳情，改一個儲存格或一次改多個欄位 —— 每條寫語句都走參數綁定，而且先擺出來給你看過。',

    'feat.designer.title': '表設計器',
    'feat.designer.desc':
      '「結構」頁就是編輯器：改名字、改型別、加標記、拖列首把手排序，後端拿即時目錄規劃 DDL。預覽與儲存走同一個規劃器，看到什麼就跑什麼。',

    'feat.export.title': '資料庫匯出',
    'feat.export.desc':
      '結構、結構與資料、或單表的列匯出成 CSV / JSON / JSONL / SQL。整個匯出在後端一邊讀一邊直接寫入磁碟，所以進度列、位元組數與 Stop 都是真的。',

    'feat.sqlfile.title': '執行 SQL 檔案',
    'feat.sqlfile.desc':
      '把一份 .sql 交給後端：它從磁碟上讀，先告訴你裡面有什麼 —— 哪幾條會丟資料、哪幾條會被唯讀連線拒掉、USE 會把後面重定向到哪 —— 然後一條語句一次呼叫地跑，帶進度與 Stop。檔案沒有交易包著，視窗上直說這一點。',

    'feat.datagen.title': '資料生成',
    'feat.datagen.desc':
      '按 mock.js 範本填表，一欄一段範本，取色器會給每個佔位符渲染一列範例值。引擎拒掉一列時是停下、還是跳過並數出來，由你決定。',

    'feat.compare.title': '資料庫比對',
    'feat.compare.desc':
      '兩個同引擎的資料庫逐表比對，產生「把右邊改成左邊」的腳本，按先建、再改、最後刪排。只有右邊可以是唯讀的 —— 左邊常常是拿來當參照的正式環境。',

    'feat.codegen.title': '程式碼產生',
    'feat.codegen.desc':
      '把一張表的欄位翻成 18 種語言裡的類別 / 結構 / 記錄，可空按各語言的寫法給。Java 分帶 Lombok 與素寫兩檔。',

    'feat.explain.title': '查詢計畫與格式化',
    'feat.explain.desc':
      '問引擎「這條你打算怎麼跑」，而不真的跑；再按這個引擎自己的文法重新縮排腳本。高亮由程式自己掃字元得到，不引第三方詞法庫 —— 因為各引擎的引號規則不一樣。',

    'feat.er.title': 'ER 圖',
    'feat.er.desc':
      '把一個命名空間畫在一張圖上：同一家的表排成一列，列內按外鍵層級排，箭頭懸停顯示哪一欄引用哪一欄。可以搜尋、拖曳、縮放，也能匯出成自包含的 SVG。',

    'feat.changelog.title': '變更日誌',
    'feat.changelog.desc':
      '程式跑過的每一句寫語句，帶著影響列數與它所在的連線。日誌就在資料目錄裡 —— 一天一個檔案、只輪轉不丟棄 —— 所以沒連資料庫也能打開。',

    'feat.i18n.title': 'English、简体中文、繁體中文',
    'feat.i18n.desc':
      '整個介面都譯了，antd 元件也在內。設定頁、底部狀態列、以及 Windows 的托盤選單都能切，切完不用重啟。',

    'engines.title': '支援的引擎',
    'engines.lead':
      '驅動層不假設關聯式模型，所以文件型引擎走的是同一套契約；引擎給不出的入口（MongoDB 的表設計器、資料生成與 Explain，Doris 的表設計器）不擺出來，而不是點了才報錯。',
    'engines.planned.title': '規劃中',
    'engines.planned':
      'Oracle 與 SQL Server：方言與佔位已經在樹裡，缺 DSN 與目錄查詢 —— 在那之前不上選單。',

    'engine.mysql.name': 'MySQL & MariaDB',
    'engine.mysql.desc':
      '最常用的一檔：TLS、建庫時的字元集與排序規則、分割表，以及運行情況頁上的 InnoDB 緩衝池與行程清單。',
    'engine.postgres.name': 'PostgreSQL',
    'engine.postgres.desc': '一個資料庫裡幾個 schema，serial 與 identity 欄位，建庫時的編碼與 locale。',
    'engine.sqlite.name': 'SQLite',
    'engine.sqlite.desc':
      '一個檔案就是一個資料庫：不用伺服器端、不用帳號。可以附加別的資料庫、讀 pragma，像瀏覽伺服器端引擎那樣瀏覽這個檔案。',
    'engine.mongodb.name': 'MongoDB',
    'engine.mongodb.desc':
      '集合、文件、索引，查詢視窗跑的是 mongosh 風格的 shell；運行情況頁給出文件數、體積與 dbStats。',
    'engine.tidb.name': 'TiDB',
    'engine.tidb.desc':
      '講 MySQL 線協定、有 TLS，運行情況頁多一張 information_schema.CLUSTER_INFO 的叢集成員表。',
    'engine.doris.name': 'Apache Doris',
    'engine.doris.desc':
      '分析型引擎，在這裡有意做成唯讀瀏覽：結構、索引、資料網格、SQL 與匯出都可用；沒有表設計器，而且會說明原因，而不是點了才失敗。',

    'download.title': '下載',
    'download.lead': '每個版本都由 CI 在三個 runner 上建置，產物直接掛在 Release 上：',
    'dl.windows.name': 'Windows x64',
    'dl.windows.note': '免安裝，雙擊即可。需要 WebView2 執行階段（Windows 10/11 內建）。',
    'dl.macos.name': 'macOS（Intel + Apple Silicon 通用）',
    'dl.macos.note':
      '解壓後把 db-manager.app 拖進「應用程式」。封裝未做簽章與公證：首次打開請右鍵「打開」，或執行 xattr -cr。',
    'dl.linux.name': 'Linux x64',
    'dl.linux.note': 'chmod +x db-manager && ./db-manager —— 需要 GTK3 與 WebKitGTK 4.0。',
    'dl.checksums.title': '校驗',
    'dl.checksums.note': '每個 Release 都帶一份 checksums.txt：',
    'dl.build.title': '從原始碼建置',
    'dl.build.note':
      'Go 1.24+、Node 20+、Wails CLI 2.16 和 just。`just release` 把當前平台的產物打到 release/ 並附校驗和；`just publish <x.y.z> "本版總結"` 發版。',
    'dl.build.link': '看建置說明',
    'dl.notes.title': '版本說明',
    'dl.notes.note':
      '每個 Release 的正文由附註標籤與本版提交資訊渲染而成 —— 所以提交資訊寫清楚，release notes 就清楚。',

    'data.title': '資料放在哪',
    'data.lead':
      '連線設定、查詢收藏、儲存的腳本、樹的排法、視窗狀態、密碼金鑰和變更日誌都在同一個資料目錄裡 —— Windows 上是 %APPDATA%/db-manager，其它平台是各自慣例的位置（~/.config、~/Library/Application Support）。',
    'data.point.local': '不需要帳號、沒有遙測、什麼都不上傳 —— 只跟你的資料庫說話。',
    'data.point.passwords':
      '密碼只有你要求才儲存：AES-256-GCM 封裝，金鑰單獨放一個檔案，和設定放在一起。',
    'data.point.log-text': '變更日誌是純文字：可以 grep、可以歸檔、可以拿別的編輯器打開。',
    'data.point.move':
      '換目錄是真的把資料搬過去：先逐個複製、逐位元組校驗，然後才改指標 —— 搬的時候連線照舊連著。',

    'footer.tagline': '離線優先的六引擎資料庫用戶端。',
    'footer.project': '專案',
    'footer.repo': '儲存庫',
    'footer.releases': '版本發佈',
    'footer.issues': '問題回報',
    'footer.developer': '開發者',
    'footer.readme': 'README',
    'footer.license': 'MIT 授權',
    'footer.built': '用 Wails + Go + React 建置。',
  }

  window.DM_DOCS = {
    defaultLang: 'en',
    fallbackLang: 'en',
    storageKey: 'dm-docs-lang',
    languages: [
      { code: 'en', short: 'EN', label: 'English', htmlLang: 'en' },
      { code: 'zh-CN', short: '简', label: '简体中文', htmlLang: 'zh-Hans-CN' },
      { code: 'zh-TW', short: '繁', label: '繁體中文', htmlLang: 'zh-Hant-TW' },
    ],
    messages: { en: EN, 'zh-CN': ZH_CN, 'zh-TW': ZH_TW },
  }
})()
