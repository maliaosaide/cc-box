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

CC-Box 把你的 Claude Code 完整环境——配置文件、skills、agents、MCP 配置、项目 `.claude.json`，甚至 Claude 二进制本身——装进一个**基于 WebDAV 的类 Git 快照系统**中，带**端到端加密**。

换电脑？开新虚拟机？CC-Box 一键恢复你的 Claude Code 环境。不用再拿 U 盘拷 `~/.claude/`，不用再担心云盘同步把配置搞乱。

## 为什么你会喜欢

- **多设备一键同步。** 台式机、笔记本、服务器——一台推送，另一台拉取即可。
- **快照，不是覆盖。** 每次同步生成一个版本快照。浏览历史、查看 diff、随时回滚。
- **端到端加密。** Argon2id + AES-256-GCM。你在 WebDAV 上存的是加密数据，不是明文配置。
- **冲突处理直观清晰。** 并排对比 + Git 风格内联标记（`<<<<<<<` / `=======` / `>>>>>>>`）。选本地、选远程、或手动合并。
- **Claude 二进制管理一体化。** 官方安装、GitHub Releases 安装、WebDAV 备份恢复、本地切换，一站搞定。
- **项目 `.claude.json` 同步。** 各项目的 MCP 配置多设备共享。
- **CLI 自动化 + GUI 日常用。** 两者共享同一套核心，选你喜欢的工作方式。

## 30 秒上手

### 下载 Release

从 [Releases](https://github.com/maliaosaide/cc-box/releases) 下载最新版本：

| 文件 | 说明 |
|------|------|
| `cc-box.exe` | CLI 命令行工具 |
| `cc-box-gui.exe` | Windows 桌面应用 |

### 从源码构建

**前置依赖：** Go 1.25+、Node.js、Wails CLI v2.x

```bash
# CLI
go -C cli build -o cc-box.exe ./cmd/cc-box/

# GUI（Windows/macOS/Linux）
cd gui && wails build
```

## 工作原理

```
你的电脑 A                                 你的 WebDAV 服务器                       你的电脑 B
───────────                              ──────────────────                      ───────────
~/.claude/          ── 加密 ──→         snapshots/*.enc                          ~/.claude/
  settings.json                         objects/{hash}            ── 解密 ──→     settings.json
  CLAUDE.md                             HEAD                                        CLAUDE.md
  skills/                               salt.bin                                    skills/
  agents/                                                                           agents/
~/.claude.json       ── 加密 ──→       （包含在快照中）              ── 解密 ──→   ~/.claude.json
```

1. **扫描** — 遍历 `~/.claude/` 和 `~/.claude.json`，对每个文件计算哈希。
2. **加密** — 从你的密码派生密钥，AES-256-GCM 加密。
3. **上传** — 推送对象和快照到你的 WebDAV。
4. **HEAD** — 指向最新快照的原子指针，受 ETag CAS 保护。

## 功能概览

### 配置文件同步和浏览

浏览你的同步配置文件树，带有状态标识（已修改、新增、冲突等）。文件内容支持完整滚动，可查看逐行 diff，并以 Git 风格内联标记解决冲突。

<p align="center"><i>同步状态、Diff、冲突解决——都在「配置文件」页面完成。</i></p>

### Claude 二进制版本管理

安装、备份、切换、回滚 Claude binary，全程不离开应用。

| 来源 | 用途 |
|------|------|
| **官方安装器** | 安装最新稳定版，一键完成。 |
| **GitHub Releases** | 从任意 Release 安装指定版本。SHA256 校验。 |
| **WebDAV 备份** | 恢复你之前上传过的版本。不依赖 GitHub。 |

所有安装都写入官方 Claude 路径（`~/.local/bin/claude` / `~/.local/bin/claude.exe`）。

### 概览页实时状态

一目了然的同步健康状态：连接状态、待同步变更、冲突数量、上次同步时间、Claude 版本、设备列表。

### 快照历史与回滚

每次同步的完整时间线。点击查看文件列表，回滚到任意历史状态。

### 项目 `.claude.json` 同步

通过 git remote 发现带有 `.claude.json` 的项目。在多设备间推送、拉取和合并 MCP 配置。

### 系统托盘（GUI）

最小化到托盘。实时同步状态和冲突指示。单实例——重复启动会唤起已有窗口。

## CLI 快速参考

```bash
cc-box init                # 首次配置：WebDAV、加密密码、设备
cc-box status              # 查看同步状态
cc-box push                # 上传本地变更
cc-box pull                # 下载远程变更
cc-box sync                # 拉取后推送
cc-box diff path/to/file   # 查看文件差异
cc-box conflicts           # 查看并解决冲突
cc-box log                 # 快照历史
cc-box revert <snap-id>    # 回滚到指定快照

cc-box binary list         # 列出可用的 Claude 版本
cc-box binary push         # 备份当前 Claude binary
cc-box binary pull <ver>   # 恢复已备份版本
cc-box binary install --source github --version 1.2.3

cc-box project list        # 列出发现的项目配置
cc-box project push        # 推送项目 .claude.json
```

## 技术栈

| 层级 | 技术 |
|------|------|
| 语言 | Go 1.25+ |
| CLI 框架 | Cobra + Viper |
| GUI 框架 | [Wails v2](https://wails.io) |
| 前端 | Svelte + Vite + Tailwind CSS |
| 加密 | Argon2id → AES-256-GCM |
| 存储 | WebDAV（任意服务商） |
| 文件监听 | fsnotify |
| 系统托盘 | systray |

## WebDAV 兼容

支持任意标准 WebDAV 服务：

- **Alist** · **NextCloud** · **坚果云** · **Synology** · **自建 WebDAV**

多设备并发同步依赖服务端正确支持 ETag / If-Match / If-None-Match。

## 构建

```bash
# CLI
go -C cli build -o cc-box.exe ./cmd/cc-box/

# GUI（需要 Wails CLI v2.x + Node.js）
cd gui && wails build

# 产物：
#   cli/cc-box.exe
#   gui/build/bin/cc-box-gui.exe
```

运行测试：
```bash
go -C core test ./...
go -C cli test ./...
go -C gui test ./...
npm --prefix gui/frontend run build
```

## 项目结构

```
cc-box/
├── core/           # 共享核心：config、crypto、snapshot、sync、WebDAV、binary
├── cli/            # CLI 应用（Cobra 命令层）
├── gui/            # Wails 桌面应用（Svelte 前端）
│   ├── frontend/   # Svelte + Vite + Tailwind
│   └── internal/   # 桌面适配、项目管理
└── readme/         # 多语言 README
```

## 环境变量

| 变量 | 说明 |
|------|------|
| `CC_BOX_WEBDAV_PASSWORD` | WebDAV 密码，设置后可减少交互输入 |

## License

MIT

---

## Star History

<p align="center">
  <a href="https://star-history.com/#maliaosaide/cc-box&Date">
    <img src="https://api.star-history.com/svg?repos=maliaosaide/cc-box&type=Date" alt="Star History Chart" width="600" />
  </a>
</p>
