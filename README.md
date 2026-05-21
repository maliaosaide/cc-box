# CC-Box

把 Claude Code 配置、项目设置和 Claude 二进制版本装进一个可同步、可回滚、可加密的工具箱。

CC-Box 面向经常在多台电脑上使用 Claude Code 的用户：你不再需要手动复制 `~/.claude/`，不需要反复配置 MCP、skills、commands，也不需要担心某次同步把本地配置覆盖坏。它用类似 Git 的快照方式管理 Claude Code 配置，并通过 WebDAV 在多设备之间同步。

## 为什么需要 CC-Box

Claude Code 越用越顺手，配置也会越来越多：

- 全局指令写在 `CLAUDE.md`。
- API、权限、环境变量写在 `settings.json`。
- 自定义能力散落在 `skills/`、`commands/`、`agents/`、`plugins/`。
- 不同项目还有各自的 `.claude.json`。
- Claude 二进制版本也可能需要备份、切换或回滚。

换一台电脑时，这些东西往往很难完整迁移；手动复制容易漏文件，云盘同步又缺少冲突处理、历史版本和加密控制。

CC-Box 要解决的就是这个问题：**让 Claude Code 的配置像代码一样可同步、可追踪、可回滚。**

## 核心优势

| 优势 | 说明 |
| --- | --- |
| 多设备同步 | 通过 WebDAV 在多台设备之间同步 Claude Code 配置。 |
| 版本化快照 | 每次同步都有历史记录，可查看、对比和回滚。 |
| 端到端加密 | 上传前加密，远程只保存加密后的内容。 |
| 冲突可处理 | 本地和远程同时修改时，不是简单覆盖，而是保留冲突并支持选择。 |
| 二进制版本管理 | Claude 可执行文件也能备份、下载、切换和清理。 |
| 项目配置同步 | 支持项目级 `.claude.json`，多设备共享项目 MCP 配置。 |
| CLI + GUI 双入口 | 自动化用 CLI，日常桌面管理用 GUI。 |

## 你可以用它做什么

### 同步 Claude Code 全局配置

把 Claude Code 的关键配置同步到 WebDAV：

```text
~/.claude/
├── settings.json
├── CLAUDE.md
├── skills/
├── commands/
├── agents/
├── plugins/
└── ...
```

适合这些情况：

- 新电脑想快速恢复 Claude Code 使用环境。
- 台式机、笔记本、远程机器之间保持配置一致。
- 想备份自己的 prompt、skills、commands 和 MCP 设置。

### 像 Git 一样管理配置历史

CC-Box 不是简单地把文件上传覆盖。每次 push 都会生成快照，记录文件状态和父快照。

你可以：

- 查看历史快照。
- 对比本地和远程差异。
- 查看具体文件 diff。
- 回滚到任意快照。
- 在冲突时选择本地版本、远程版本或合并内容。

### 加密后再上传

同步到 WebDAV 前会先加密：

```text
加密密码 → Argon2id 派生 → AES-256-GCM 加密
```

这意味着远程 WebDAV 存储里保存的是加密数据，而不是直接暴露的 Claude Code 配置内容。

### 管理 Claude 二进制版本

除了配置文件，CC-Box 也能管理 Claude 二进制文件：

- 备份当前 Claude 可执行文件。
- 上传到 WebDAV。
- 下载指定历史版本。
- 在本地切换版本。
- 清理旧版本。

这适合需要保留可用版本、回滚版本或在多台设备间统一二进制版本的用户。

### 同步项目级 `.claude.json`

很多项目会有自己的 `.claude.json`，里面包含项目级 MCP、工具权限等配置。CC-Box 支持把这些项目配置也纳入同步范围，避免每台设备都重新配置一遍。

## CLI 和 GUI 怎么选

| 版本 | 适合谁 | 典型用途 |
| --- | --- | --- |
| CLI | 喜欢终端、脚本和自动化的用户 | 初始化、push/pull、查看状态、回滚、CI/脚本调用 |
| GUI | 喜欢可视化操作的桌面用户 | 查看状态、处理冲突、浏览 diff、管理二进制、托盘常驻 |

两个版本可以独立使用。你只想要命令行，就用 `cli/`；你只想用桌面界面，就用 `gui/`。

## 快速开始

### CLI 版

```bash
cd cli
go build -o build/bin/cc-box.exe ./cmd/cc-box/

./build/bin/cc-box.exe init
./build/bin/cc-box.exe status
./build/bin/cc-box.exe push
./build/bin/cc-box.exe pull
```

常用命令：

```bash
cc-box init                 # 初始化 WebDAV、设备和加密密码
cc-box status               # 查看同步状态
cc-box push                 # 推送本地配置到远程
cc-box pull                 # 拉取远程配置到本地
cc-box sync                 # 拉取后推送
cc-box log                  # 查看快照历史
cc-box diff [FILE]          # 查看文件差异
cc-box conflicts            # 查看冲突
cc-box binary list          # 查看二进制版本
cc-box project list         # 查看项目配置
```

CLI 详细说明见：[cli/README.md](./cli/README.md)

### GUI 版

```bash
cd gui
wails build -clean -nopackage -m -nosyncgomod
```

构建后运行：

```text
gui/build/bin/cc-box-gui.exe
```

GUI 版提供初始化向导、同步概览、文件 diff、冲突处理、二进制版本管理、项目配置管理、历史快照、设置页和系统托盘。

GUI 详细说明见：[gui/README.md](./gui/README.md)

## 功能一览

| 功能 | CLI | GUI |
| --- | --- | --- |
| 首次初始化 | 支持 | 支持 |
| WebDAV 同步 | 支持 | 支持 |
| 快照历史 | 支持 | 支持 |
| 文件 diff | 支持 | 支持 |
| 冲突处理 | 支持 | 支持 |
| 加密密码管理 | 支持 | 支持 |
| Claude 二进制管理 | 支持 | 支持 |
| 项目级 `.claude.json` 同步 | 支持 | 支持 |
| 系统托盘 | 不适用 | 支持 |
| 文件变化监听 | 不适用 | 支持 |
| 脚本自动化 | 支持 | 不适用 |

## 项目结构

```text
cc-box/
├── cli/                         # 命令行版
│   ├── README.md                # CLI 使用说明
│   ├── cmd/                     # CLI 入口
│   ├── internal/                # CLI 业务实现
│   ├── go.mod
│   └── go.sum
├── gui/                         # 桌面 GUI 版
│   ├── README.md                # GUI 使用说明
│   ├── frontend/                # Svelte 前端
│   ├── internal/                # GUI 业务实现
│   ├── main.go                  # Wails 入口
│   ├── wails.json
│   ├── go.mod
│   └── go.sum
├── Makefile                     # 统一构建和测试入口
└── README.md                    # 项目总览
```

## 技术栈

| 分类 | 技术 |
| --- | --- |
| 语言 | Go 1.25+ |
| CLI | Cobra、Viper |
| GUI | Wails v2、Svelte、Vite、Tailwind CSS |
| 文件监听 | fsnotify |
| 系统托盘 | systray |
| 加密 | Argon2id、AES-256-GCM |
| 远程存储 | WebDAV |

## 构建

从根目录使用 Makefile：

```bash
make build       # 构建 CLI 和 GUI
make build-cli   # 只构建 CLI
make build-gui   # 只构建 GUI
```

没有 `make` 时可以分别构建：

```bash
cd cli && go build -o build/bin/cc-box.exe ./cmd/cc-box/
cd gui && wails build -clean -nopackage -m -nosyncgomod
```

构建产物：

```text
cli/build/bin/cc-box.exe
gui/build/bin/cc-box-gui.exe
```

## 测试

```bash
make test       # 测试 CLI 和 GUI
make test-cli   # 只测试 CLI
make test-gui   # 只测试 GUI
```

或直接执行：

```bash
cd cli && go test ./...
cd gui && go test ./...
```

## 环境要求

| 依赖 | 说明 |
| --- | --- |
| Go 1.25+ | 构建和测试 CLI / GUI 后端。 |
| Wails CLI v2.x | 构建和开发 GUI。 |
| Node.js / npm | 安装和构建 GUI 前端依赖。 |
| WebDAV 服务 | 存储同步数据。 |
| Git | 项目级配置同步时用于识别项目 remote。 |

## 环境变量

| 变量 | 说明 |
| --- | --- |
| `CC_BOX_WEBDAV_PASSWORD` | WebDAV 密码。设置后可减少交互输入，适合 CLI 自动化场景。 |

## WebDAV 兼容性

理论上支持标准 WebDAV 服务，例如：

- Alist
- 坚果云
- NextCloud
- Synology WebDAV
- 自建 WebDAV 服务

实际兼容性取决于服务端对 WebDAV 方法、鉴权、ETag 和大文件上传的支持情况。

## 开发说明

当前仓库中 CLI 和 GUI 是两个独立应用：

- 只开发命令行能力，进入 `cli/`。
- 只开发桌面界面，进入 `gui/`。
- 根目录没有 Go module。

如果要保持两个版本行为一致，需要分别同步修改两边对应实现。

## License

MIT
