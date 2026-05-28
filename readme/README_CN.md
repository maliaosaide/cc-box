<div align="center">
  <img src="../gui/assets/icons/generated/appicon.png" alt="CC-Box" width="96" height="96" />

  <h1>CC-Box</h1>

  <p><strong>同步、加密、版本化、可回滚你的 Claude Code 完整环境。</strong></p>

  <p>
    <img src="https://img.shields.io/badge/版本-v0.4.0-ok?style=flat-square&color=c4704e" alt="Version" />
    <img src="https://img.shields.io/badge/平台-Windows%20%7C%20macOS%20%7C%20Linux-blue?style=flat-square" alt="Platform" />
    <img src="https://img.shields.io/badge/构建-Wails-09f?style=flat-square&color=df2929" alt="Built with Wails" />
    <img src="https://img.shields.io/github/downloads/maliaosaide/cc-box/total?style=flat-square&color=6b9080" alt="Downloads" />
  </p>

  <p>
    <a href="../README.md">English</a> | <b>中文</b>
  </p>
</div>

---

## CC-Box 是什么？

CC-Box 把你的 Claude Code 完整环境——`CLAUDE.md` 中的全局指令、`settings.json` 中的 API 密钥和权限、自定义的 `skills/`、`commands/`、`agents/`、每个项目的 `.claude.json` MCP 配置，甚至 Claude 二进制本身——全部装进一个**基于 WebDAV 的类 Git 快照系统**中，带**端到端加密**。

换电脑？开新虚拟机？CC-Box 一键恢复你的 Claude Code 环境。不用再拿 U 盘拷 `~/.claude/`，不用再担心云盘同步把配置搞乱。CC-Box 给你和 Git 一样的信心：**每次变更都被记录，每个版本都可以恢复，不会有任何丢失。**

## 为什么需要 CC-Box

Claude Code 越用越顺手，配置也越来越多：

- 全局指令写在 `CLAUDE.md`
- API 密钥、权限、环境变量在 `settings.json`
- 自定义 skills、commands、agents、plugins 散落在子目录中
- 每个项目的 `.claude.json` 带有项目级 MCP 服务器和工具权限
- 某个 Claude 二进制版本你想保留或回滚

换一台电脑，每一样都变成一个手动迁移步骤。两台机器同时改配置，你面临静默覆盖的风险。CC-Box 端到端解决这个问题。

| 能力 | 没有 CC-Box | 有了 CC-Box |
|------|------------|-----------|
| 迁移配置到新机器 | 手动复制 `~/.claude/`，容易遗漏 | 一个 `pull` 命令或 GUI 点击 |
| 历史和回滚 | 无。覆盖了就没了。 | 每次推送生成快照。可回滚到任意时间点。 |
| 多机冲突 | 手动合并，祈祷不出错 | 并排对比，选本地/远程，或 Git 风格内联合并 |
| 传输安全 | WebDAV 上存的是明文 | Argon2id + AES-256-GCM 加密后上传 |
| Claude 二进制版本管理 | 从 GitHub 重新下载，希望版本还在 | 备份到 WebDAV，随时恢复，不依赖 GitHub |
| 项目 MCP 配置 | 每台机器重新配一遍 | 通过 git remote 发现，按项目同步 `.claude.json` |

## 同步原理

CC-Box 不只是上传文件。它构建一个**内容寻址的快照**，涵盖整个 `~/.claude/` 目录和 `~/.claude.json`，加密后推送到你的 WebDAV 服务器。

```
机器 A                                       WebDAV 服务器                          机器 B
──────                                     ────────────                         ──────
~/.claude/                                 snapshots/                            ~/.claude/
  settings.json  ──┐                      ├── abc123.json.enc  ──┐                settings.json
  CLAUDE.md       │  扫描→哈希→加密       │── def456.json.enc     │  解密→写入      CLAUDE.md
  skills/         │                       │── ...                │                 skills/
  agents/         │  objects/             │── HEAD ── "abc123"   │                 agents/
~/.claude.json   ─┘  ├── a1b2c3...        │                      └─ ~/.claude.json
                     ├── d4e5f6...        │
                     └── ...              │
```

### 详细步骤

1. **扫描与哈希** — 递归遍历 `~/.claude/`。对每个文件计算内容哈希。排除规则（正则/通配符）匹配的文件会被跳过。大小写不敏感文件系统上的路径冲突会被检测并报告。符号链接会被拒绝（跨 OS 无法可靠迁移）。

2. **与父快照对比** — 将当前文件哈希与上次快照对比。文件被分类为 `added`（新增）、`modified`（修改）、`deleted`（删除）或 `synced`（一致）。只有变更的文件才上传。

3. **加密** — 加密密码通过 Argon2id 密钥派生。派生密钥用 AES-256-GCM 加密快照元数据和文件内容。唯一的随机 salt（存储在 `salt.bin`）确保相同密码在不同设备上产生不同密文。

4. **上传对象** — 每个文件作为内容寻址对象存储（`objects/{hash}`）。跨快照的相同文件共享同一对象——无重复上传。

5. **推送快照** — 快照 JSON（文件路径 → 哈希 + 大小 + 修改时间 的映射）加密后上传到 `snapshots/{id}.json.enc`。

6. **更新 HEAD（原子操作）** — `HEAD` 指针通过基于 ETag 的 Compare-And-Swap 更新为新快照 ID。如果另一台设备在你推送期间也推送了，CAS 会失败，CC-Box 提示你先拉取。这防止了"最后写入者静默胜出"的问题。

### 拉取与合并

拉取时，CC-Box 在共享父快照、你的本地状态和远程状态之间做**三路合并**。如果你和另一台设备都修改了同一个文件，CC-Box 会创建冲突条目，而不是静默选一个胜者。然后你可以选择本地、远程或手动合并。

### 冲突处理

CLI 和 GUI 都会展示冲突。GUI 显示并排对比视图，带有元数据（哪边较新、修改时间），以及切换到 **Git 风格内联标记**（`<<<<<<< 本地` / `=======` / `>>>>>>> 远程`）的按钮。逐个解决，或批量处理。

### 对象存储与去重

每个文件都是内容寻址的。两个快照中相同内容的文件共享服务器上的同一对象。这意味着：

- 无重复上传。
- 二进制文件和大配置文件只存一次。
- 缺失的对象可以从本地副本修复——CC-Box 自动检查并提供补传。

## 加密详解

CC-Box 不信任 WebDAV 服务器能保护你的明文配置。

```
用户密码 ──→ Argon2id ──→ 256 位密钥
                              │
                ┌─────────────┴─────────────┐
                ▼                           ▼
          AES-256-GCM                  AES-256-GCM
       （快照元数据）                  （文件对象）
```

- **密钥派生**：Argon2id，参数偏向安全性而非速度（内存密集、多轮、多线程）。每台设备使用 WebDAV 上的共享 salt（`salt.bin`），相同密码派生出相同密钥。
- **加密**：每个快照 JSON 和每个文件对象独立用 AES-256-GCM 加密，提供机密性和真实性（防篡改）。
- **Salt**：通过 WebDAV 共享。第一台初始化的设备上传随机 salt。后续加入的设备下载它。如果远程 salt 不匹配，CC-Box 会警告——你可能指向了错误的 WebDAV 根路径或使用了不同的密码。

**重要**：加密密码由你选择和记忆。CC-Box 不存储它、无法恢复它、没有它也无法解密你的数据。如果你丢了密码，数据无法恢复。

## 界面截图

<p align="center">
  <img src="../docs/screenshots/01-dashboard.png" alt="概览" width="800" />
  <br/><i>概览页 — 同步健康状态、连接状态、待同步变更、冲突计数、设备列表</i>
</p>

<p align="center">
  <img src="../docs/screenshots/02-files.png" alt="配置文件" width="800" />
  <br/><i>配置文件浏览器 — 树状视图含状态标识、文件内容、Diff 查看器、冲突解决</i>
</p>

<p align="center">
  <img src="../docs/screenshots/03-binaries.png" alt="二进制管理" width="800" />
  <br/><i>Claude 二进制管理 — WebDAV 备份、GitHub Releases、官方安装器</i>
</p>

## Claude 二进制管理

除了配置文件，CC-Box 还能管理 Claude 二进制——安装、备份、切换和回滚——全部写入 Claude 官方 native install 布局。

### 安装目标

所有安装路径遵循 Claude 官方约定：

| 平台 | 二进制路径 |
|------|----------|
| Windows | `~/.local/bin/claude.exe` |
| macOS | `~/.local/bin/claude` |
| Linux | `~/.local/bin/claude` |

这意味着 `PATH` 上的 `claude` 就是 CC-Box 帮你管理的版本。没有私有目录，没有 shim。

### 三种安装来源

| 来源 | 适用场景 | 说明 |
|------|---------|------|
| **官方安装器** | 安装最新版 | 执行 Claude 官方安装命令。最快的获取最新稳定版方式。 |
| **GitHub Releases** | 固定指定版本 | 从 Claude 官方 GitHub Release 下载。适用于任意已发布版本。SHA256 校验。适合回退、测试、锁定版本。 |
| **WebDAV 备份** | 离线/无网恢复 | 恢复你之前上传过的版本。不依赖 GitHub 或官方安装源。 |

### GitHub Release 平台映射

| 当前平台 | GitHub 资源 |
|---------|-----------|
| Windows x64 | `claude-win32-x64.zip` |
| macOS Apple Silicon | `claude-darwin-arm64.tar.gz` |
| Linux x64 | `claude-linux-x64.tar.gz` |

当前不支持 Windows ARM64、macOS Intel、Linux ARM64 和 Linux musl。CC-Box 只安装当前运行平台的 binary。

### 安装流程与安全

安装指定版本时（GitHub 或 WebDAV）：

1. **验证** binary 的正确性（GitHub 来源校验 SHA256，WebDAV 来源校验哈希）。
2. **备份**当前版本（如果存在）。
3. **写入**目标 binary 到官方路径。
4. **执行** `claude install` 初始化 Claude 本地目录结构。
5. **再次验证**目标路径的 binary 仍匹配预期版本。
6. **失败自动回滚**——不留半成品状态。

对于 GitHub 来源的安装，CC-Box 对照 `SHASUMS256.txt` 验证 SHA256。关于签名：CC-Box 校验 SHA256 完整性；它**不会**验证 `SHASUMS256.txt.sig` 的 GPG 签名。界面明确说明这一点——不会假装下载签名文件就等于验签通过。

### PATH 配置

CC-Box 尝试确保 `~/.local/bin` 在你的 `PATH` 中。如果无法配置（如 shell 配置文件非标准），它会警告但不回滚已安装的 binary。Binary 仍在正确位置，你只需手动添加到 PATH。

### CC-Box 不管什么

- 只管 `claude` / `claude.exe`——不管 `uv`、`uvx`、Codex、Gemini 或其他工具。
- 二进制恢复不恢复 `~/.claude/` 或 `~/.claude.json`——这些由快照同步单独处理。
- 如果当前 `claude` 看起来是 npm shim、shell 脚本包装或其他非原生二进制，CC-Box 不会静默重定向到私有目录。它会安装到官方路径，替换 shim。

## 项目 `.claude.json` 同步

每个项目可以有独立的 `.claude.json`，包含项目级 MCP 服务器、允许工具和权限。CC-Box 通过 git remote 发现这些文件，并通过独立的 `projects/` WebDAV 命名空间同步。

### 发现机制

CC-Box 扫描 `~/.claude/projects/`（Claude Code 存储项目数据的位置），提取项目根路径，读取 git remote URL，并验证 `.claude.json` 存在。手动跟踪的项目（通过 GUI 添加）会被合并。

### 合并策略

拉取项目 `.claude.json` 时，CC-Box 采用合并而非覆盖：

| 字段 | 策略 |
|------|------|
| `mcpServers` | 远程优先，但仅存在于本地的服务器会保留 |
| `allowedTools` | 并集（两边合并） |
| `permissions` | 远程优先，但仅存在于本地的键会保留 |

### CLI 命令

```bash
cc-box project list           # 列出所有发现的项目配置
cc-box project push           # 推送所有项目 .claude.json
cc-box project pull           # 拉取并合并远程项目配置
cc-box project add <路径>     # 手动跟踪一个项目
```

## 功能对比：CLI vs GUI

| 功能 | CLI | GUI |
|------|-----|-----|
| 初始化同步组 | ✓ (`cc-box init`) | ✓（引导向导） |
| 推送 / 拉取 / 同步 | ✓ | ✓（工具栏 + 批量操作） |
| 快照历史 | ✓ (`cc-box log`) | ✓（历史页面，时间线） |
| 文件 diff（逐行） | ✓ (`cc-box diff`) | ✓（配置文件页，统一 diff 视图） |
| 冲突解决 | ✓ (`cc-box conflicts`) | ✓（并排 + Git 风格内联标记） |
| 文件树 + 状态标识 | — | ✓（配置文件页） |
| 管理加密密码 | ✓ | ✓（设置） |
| 排除规则 | ✓（配置文件） | ✓（设置 + 单文件排除） |
| Claude binary 安装/备份/切换 | ✓ | ✓（二进制页面） |
| GitHub Release 安装 | ✓ | ✓（二进制页面） |
| 项目 `.claude.json` 同步 | ✓ | ✓（项目页面） |
| 概览面板 | — | ✓（同步健康、冲突数、设备列表） |
| 系统托盘 | — | ✓（实时状态、单实例） |
| 文件变化监听 | — | ✓（自动同步触发） |
| CI / 脚本自动化 | ✓ | — |

CLI 和 GUI 共享同一 `core/` 模块——同步语义、加密、快照格式和对象存储完全一致。你可以在同一台机器上混用 CLI 和 GUI，不会出现兼容问题。

## 快速开始

### 下载 Release

预编译二进制可在 [Releases](https://github.com/maliaosaide/cc-box/releases) 下载：

| 文件 | 说明 |
|------|------|
| `cc-box.exe` | CLI 命令行工具 |
| `cc-box-gui.exe` | Windows 桌面应用（需要 WebView2） |

### CLI — 首次同步

```bash
# 1. 初始化（WebDAV 地址、用户名、密码、加密密码、设备名）
cc-box init

# 2. 推送当前配置
cc-box push

# 3. 在另一台机器上，加入同一个 WebDAV 根路径
cc-box init   # 使用相同的加密密码和 WebDAV 根路径
cc-box pull
```

### CLI — 日常命令

```bash
cc-box status                     # 上次同步后有什么变化？
cc-box push                       # 上传本地变更
cc-box pull                       # 下载远程变更
cc-box sync                       # 拉取后推送（一步到位）
cc-box log                        # 快照历史
cc-box diff settings.json         # 查看文件的具体变化
cc-box conflicts                  # 列出未解决的冲突
cc-box revert <快照ID>            # 回滚到指定快照

cc-box binary list                # 可用的 Claude 版本（WebDAV + GitHub）
cc-box binary push                # 备份当前 Claude binary 到 WebDAV
cc-box binary pull <版本>         # 下载并安装已备份版本
cc-box binary switch <版本>       # 切换到不同的本地版本
cc-box binary install --source official --latest
cc-box binary install --source github --version 1.2.3
cc-box binary uninstall <版本>    # 删除本地版本

cc-box project list               # 有 .claude.json 的项目
cc-box project push               # 上传所有项目配置
cc-box project pull               # 下载并合并
```

### GUI

```bash
# 构建
cd gui && wails build

# 运行
./gui/build/bin/cc-box-gui.exe
```

GUI 通过引导向导带你完成：WebDAV 连接 → 加密密码 → 设备名称 → 加入已有同步组或全新开始。之后你会看到概览面板、带 diff/冲突视图的文件浏览器、二进制管理器、项目同步、快照历史，以及带实时状态的系统托盘。

## 从源码构建

### 前置依赖

| 依赖 | 版本 | 说明 |
|------|------|------|
| Go | 1.25+ | 构建 CLI 和 GUI 后端 |
| Wails CLI | v2.x | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| Node.js + npm | LTS | 前端依赖和构建 |
| Git | 任意 | 项目配置发现 |

### 构建命令

```bash
# CLI（任意平台）
go -C cli build -o cc-box.exe ./cmd/cc-box/

# GUI（目标平台原生构建——需在目标平台上运行）
cd gui
npm --prefix frontend install
wails build

# 产物：
#   cli/cc-box.exe
#   gui/build/bin/cc-box-gui.exe
```

### 运行测试

```bash
go -C core test ./...
go -C cli test ./...
go -C gui test ./...
npm --prefix gui/frontend run build     # 前端类型检查 + 构建
```

## 技术栈

| 层级 | 技术 |
|------|------|
| 语言 | Go 1.25+ |
| CLI 框架 | [Cobra](https://github.com/spf13/cobra) + [Viper](https://github.com/spf13/viper) |
| GUI 框架 | [Wails v2](https://wails.io) |
| 前端 | [Svelte](https://svelte.dev) + [Vite](https://vitejs.dev) + [Tailwind CSS](https://tailwindcss.com) |
| 加密 | Argon2id（密钥派生）+ AES-256-GCM（加密） |
| 存储 | WebDAV（任意标准兼容服务商） |
| 文件监听 | [fsnotify](https://github.com/fsnotify/fsnotify) |
| 系统托盘 | [systray](https://github.com/getlantern/systray) |

## 项目结构

```
cc-box/
├── core/                        # 共享模块——不依赖 CLI 或 GUI
│   ├── binary/                  # Claude binary 安装、上传、下载、索引、平台检测
│   ├── config/                  # 本地配置、路径、keyring
│   ├── crypto/                  # Argon2id 密钥派生、AES-256-GCM 加解密
│   ├── normalize/               # 跨平台路径和换行规范化
│   ├── object/                  # 内容哈希和内容寻址对象存储
│   ├── pathutil/                # 安全路径拼接（防目录穿越）
│   ├── snapshot/                # 文件扫描、快照创建、差异计算
│   ├── sync/                    # 三路合并、冲突检测、历史处理
│   └── webdav/                  # WebDAV 客户端，支持 ETag、锁、重试、代理
├── cli/                         # CLI 应用模块
│   ├── cmd/cc-box/              # 入口
│   ├── internal/cli/            # Cobra 命令（init、push、pull、sync、binary、project...）
│   └── internal/project/        # 项目 .claude.json 发现和合并
├── gui/                         # Wails 桌面应用模块
│   ├── frontend/                # Svelte + Vite + Tailwind CSS
│   │   └── src/pages/           # 概览、配置文件、二进制、项目、历史、设置
│   ├── internal/desktop/        # 平台专属桌面适配
│   └── internal/project/        # GUI 项目配置管理
└── readme/                      # 多语言 README
```

### 架构说明

CLI 和 GUI 是**独立的应用**，共享 `core/` 模块。这意味着：

- 同步语义、加密和快照格式在 `core/` 中定义一次。
- CLI 是 `core/` 之上的薄 Cobra 命令层。
- GUI 是 Wails 应用，带 Svelte 前端，调用同样使用 `core/` 的 Go 后端方法。
- 两者产生和消费相同的 WebDAV 数据。你可以在同一台机器上用 CLI 推送，用 GUI 拉取。

修改核心行为（同步、加密、快照、WebDAV、binary）需要同时更新 CLI 和 GUI。仅 UI 层面的修改（布局、托盘、引导）只影响 `gui/`。

## WebDAV 兼容性

CC-Box 使用标准 WebDAV 方法和头部。任意兼容的服务商均可使用：

| 服务商 | 状态 | 说明 |
|--------|------|------|
| **Alist** | ✓ | 完全支持 |
| **坚果云** | ✓ | 完全支持 |
| **NextCloud** | ✓ | 完全支持 |
| **Synology** | ✓ | 完全支持 |
| **自建** | ✓ | Apache mod_dav、nginx-dav-ext 等 |

### 多设备同步要求

多设备并发安全依赖服务端正确实现：

- **ETag** — 条件写入头部
- **If-Match** / **If-None-Match** — 通过 CAS 实现原子 HEAD 更新
- **PROPFIND**（Depth 1）— 设备列表发现

没有 ETag 支持时，单设备同步仍可正常使用。多设备同步是尽力而为——如果服务器忽略条件头部，CC-Box 会检测到不一致但无法阻止。

### 超时与代理

- 默认请求超时：**8 秒**（概览页）/ 按请求（同步操作）。
- 代理：启动 CC-Box 前设置 `HTTP_PROXY` 和 `HTTPS_PROXY` 环境变量。
- GUI 和 CLI 必须从设置了这些变量的 shell 中启动，进程才能继承。

## GitHub Release 下载代理

从 GitHub Releases 安装 Claude 时，CC-Box 需要访问 GitHub API、Release 资源和下载跳转域名。如果在防火墙或代理后面，请添加这些域名：

```
github.com
githubusercontent.com
githubassets.com
amazonaws.com
```

Clash 规则示例：

```yaml
- DOMAIN-SUFFIX,github.com,PROXY
- DOMAIN-SUFFIX,githubusercontent.com,PROXY
- DOMAIN-SUFFIX,githubassets.com,PROXY
- DOMAIN-SUFFIX,amazonaws.com,PROXY
```

## 环境变量

| 变量 | 说明 |
|------|------|
| `CC_BOX_WEBDAV_PASSWORD` | WebDAV 密码。设置后可跳过 CLI 自动化的交互式密码输入。 |
| `HTTP_PROXY` / `HTTPS_PROXY` | HTTP/HTTPS 代理，用于 WebDAV 请求和 GitHub 下载。 |

## License

MIT

---

## Star History

<p align="center">
  <a href="https://star-history.com/#maliaosaide/cc-box&Date">
    <img src="https://api.star-history.com/svg?repos=maliaosaide/cc-box&type=Date" alt="Star History Chart" width="600" />
  </a>
</p>
