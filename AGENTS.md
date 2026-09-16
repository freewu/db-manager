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
| `just release` | **本地打包**：编译当前平台，产物归档到 `dist/` + `checksums.txt` | 不碰 |
| `just publish <x.y.z> "总结"` | 发版：同步版本号、提交、打 tag、push，触发 GitHub Action 出三平台产物并发布 Release | 会 |

### 本地打包

```sh
just release
# → dist/db-manager-<版本>-<os>-<arch>[.exe]   （macOS 为 .tar.gz）
# → dist/checksums.txt
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
- `Build` 在 Windows / macOS / Linux 三个 runner 上各跑一次 `wails build`，产出**免安装可执行文件**；
- `Publish` 汇总产物、生成 `checksums.txt`，调用 `scripts/release-notes.sh` 渲染 release message 并创建 GitHub Release。

### release message 怎么来的

`scripts/release-notes.sh <tag>` 拼出三段：

1. **附注 tag 的 message** —— 也就是 `just publish` 的第二个参数，人写的本版总结；
2. **两个 tag 之间的提交**，按上表分组；
3. 下载表格 + 各平台运行说明（macOS 未签名、Linux 需要 GTK3/WebKitGTK 等）。

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

- **品牌素材唯一来源是 `asserts/`**（注意目录名就是 `asserts`，不是 `assets`，不要"顺手改正"）。
  `frontend/public/logo.png` 与 `build/appicon.png` 是它的副本，由 `just icons` 生成；
  `build/windows/icon.ico` 被 gitignore —— Wails 只在它**不存在**时才由 `appicon.png` 重新生成，
  所以换了 logo 必须删掉它。CI 会用 `cmp` 检查副本是否漂移。
  新的引擎图标放 `asserts/icon/`，在 `frontend/src/lib/assets.ts` 注册（`@asserts` 别名指向该目录）。
- **主题色 `#36ab60` 写在两处，必须同步**：`frontend/src/App.tsx` 的 `colorPrimary`，
  与 `frontend/src/styles/global.css` 的 `--dm-accent*`。
- **前端不消费 Wails 生成的绑定**：`frontend/src/api/{types,client}.ts` 是手写的，
  `frontend/wailsjs/` 只是构建产物（已 gitignore）。加后端方法时，两边都要手写。
- **`tsconfig` 开了 `noUnusedLocals` / `noUnusedParameters`**：删代码时记得删导入，
  否则 `tsc` 直接失败。
- **antd Tree 的 node key 必须全局唯一**：占位/错误节点用 `placeholder:${scope}` / `error:${scope}` 这种带作用域的前缀。
- 后端新增能力时按 `drivers.Driver` → `Conn` → `Dialect` 契约落地，并在 `init()` 里 `drivers.Register`。

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
scripts/package.mjs     本地打包：把 build/bin 的产物归档到 dist/ + checksums.txt
scripts/release-notes.sh 生成 GitHub Release message
asserts/                品牌素材唯一来源（logo.png、icon/*.png）
build/                  appicon.png、windows/darwin 打包资源（icon.ico 已 gitignore）
dist/                   本地打包产物（just release，已 gitignore）
internal/               后端：驱动契约、sqlbase、服务层
frontend/src/           React 前端（api 手写、store 用 zustand、样式在 styles/global.css）
.github/workflows/      ci.yml（main/PR）、release.yml（tag → 三平台产物 + Release）
```

更细的架构说明见 `README.md`。
