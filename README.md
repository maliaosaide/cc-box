# CC-Box

CC-Box 是面向 Claude Code 的配置箱，用来备份、同步、版本管理 Claude Code 配置和 Claude 二进制文件。

当前项目已经拆成两个完全独立的应用：

- `cli/`：命令行应用，适合脚本化、终端使用和自动化流程。
- `gui/`：桌面图形应用，适合可视化同步、冲突处理、二进制管理和托盘常驻。

两个应用各自拥有独立的 Go module、源码、`internal/` 业务代码、依赖文件和构建产物。根目录不再是 Go module，只负责项目总览、统一 Makefile 和文档入口。

## 项目目标

CC-Box 解决 Claude Code 多设备使用中的三个问题：

1. **配置备份与同步**：同步 `~/.claude/` 下的全局指令、settings、skills、commands、agents、plugins 等配置。
2. **二进制版本管理**：备份和切换本地 Claude 二进制文件，支持历史版本回滚。
3. **版本化快照**：每次同步形成快照链，可以查看历史、diff、冲突和回滚。

## 当前仓库结构

```text
cc-box/
├── cli/                         # 命令行应用，独立 Go module
│   ├── README.md                # CLI 专用说明
│   ├── cmd/cc-box/main.go       # CLI 入口
│   ├── internal/                # CLI 独立业务代码
│   ├── build/bin/               # CLI 构建产物目录，已忽略
│   ├── go.mod
│   └── go.sum
├── gui/                         # Wails 桌面应用，独立 Go module
│   ├── README.md                # GUI 专用说明
│   ├── main.go                  # GUI 入口
│   ├── frontend/                # Svelte 前端工程
│   ├── internal/                # GUI 独立业务代码
│   ├── build/bin/               # GUI 构建产物目录，已忽略
│   ├── wails.json
│   ├── go.mod
│   └── go.sum
├── Makefile                     # 根目录统一构建/测试入口
└── README.md                    # 当前总项目说明
```

### 顶层文件说明

| 路径 | 说明 |
| --- | --- |
| `cli/` | CLI 独立应用目录，包含命令入口、Cobra 命令层和 CLI 自己的业务实现。 |
| `gui/` | GUI 独立应用目录，包含 Wails 后端、Svelte 前端、托盘、监听和 GUI 自己的业务实现。 |
| `Makefile` | 从根目录统一执行 `build`、`test`、`clean`、`build-cli`、`build-gui` 等任务。 |

根目录没有 `go.mod`。如果要直接执行 Go 命令，请进入 `cli/` 或 `gui/`。

## 拆分后的模块边界

```text
cli/  -> module github.com/user/cc-box/cli
        只引用 github.com/user/cc-box/cli/internal/...

gui/  -> module github.com/user/cc-box/gui
        只引用 github.com/user/cc-box/gui/internal/...
```

约束：

- CLI 不 import GUI。
- GUI 不 import CLI。
- 两边都不 import 根路径 `github.com/user/cc-box/internal/...`。
- 根目录不放共享 Go 代码。
- CLI 和 GUI 的 `internal/` 是两份独立代码。

这个拆法的特点是开发隔离明确：可以只开发 CLI，也可以只开发 GUI。代价是底层同步、加密、WebDAV、快照、二进制管理等逻辑如果要保持一致，需要分别同步修改两边对应代码。

## CLI 应用

详见 [`cli/README.md`](./cli/README.md)。

### 技术框架

- Go 1.25+
- Cobra：命令注册和参数解析
- Viper：配置读取与写入
- `golang.org/x/crypto`：加密密码派生与加密能力

### 结构特点

```text
cli/
├── cmd/cc-box/main.go           # main 入口，只调用 internal/cli.Execute
├── internal/cli/                # Cobra 命令层
├── internal/config/             # 配置、路径、密钥环
├── internal/crypto/             # Argon2id + AES-256-GCM
├── internal/webdav/             # WebDAV 客户端和锁
├── internal/object/             # 对象哈希与对象存储
├── internal/snapshot/           # 快照、扫描、diff
├── internal/sync/               # 三方合并与冲突处理
├── internal/binary/             # Claude 二进制版本管理
├── internal/project/            # 项目级 .claude.json 同步
└── internal/normalize/          # 跨平台路径和内容规范化
```

### 当前能力

CLI 面向终端和自动化场景，当前包含：

- `init`：初始化 WebDAV、设备和加密密码。
- `status` / `push` / `pull` / `sync`：同步主流程。
- `diff` / `log` / `show` / `revert`：快照和版本历史。
- `conflicts` / `resolve`：冲突查看与解决。
- `config`：配置查看、修改、WebDAV 重配、加密密码轮转。
- `device`：设备列表、重命名、移除。
- `binary`：Claude 二进制上传、下载、切换、清理。
- `project`：项目级 `.claude.json` 同步。
- `backup` / `restore` / `verify` / `gc`：维护命令。

### CLI 构建与测试

```bash
# 从根目录构建 CLI
make build-cli

# 从根目录测试 CLI
make test-cli

# 直接在 cli/ 内构建
cd cli && go build -o build/bin/cc-box.exe ./cmd/cc-box/

# 直接在 cli/ 内测试
cd cli && go test ./...
```

CLI 构建产物：

```text
cli/build/bin/cc-box.exe
```

## GUI 应用

详见 [`gui/README.md`](./gui/README.md)。

### 技术框架

- Go 1.25+
- Wails v2：桌面窗口、菜单、前后端绑定、资源嵌入
- Svelte：前端组件和页面
- Vite：前端开发和构建
- Tailwind CSS：界面样式
- fsnotify：监听 `~/.claude/` 变化
- systray：系统托盘和托盘菜单

### 后端结构特点

```text
gui/
├── main.go                      # Wails 入口，配置窗口、菜单、资源嵌入和 App 绑定
├── app.go                       # 生命周期、初始化状态、系统对话框、打开文件管理器
├── dashboard.go                 # 概览数据、快捷同步、远程修复
├── files.go                     # 文件树、文件内容、diff、冲突详情、冲突解决、批量同步
├── pages.go                     # 历史、设置、项目、二进制、加密密码等页面接口
├── onboarding.go                # 首次初始化、加入已有远程、WebDAV 检测
├── async.go                     # 后台操作、取消和进度状态
├── watcher.go                   # 文件监听、待同步状态、自动同步触发
├── tray.go                      # 系统托盘、状态图标、托盘菜单、开机自启动
├── icon*.ico                    # 托盘状态图标
└── internal/                    # GUI 独立业务代码副本
```

GUI 后端通过 Wails 将 `App` 绑定给前端，前端从 `frontend/wailsjs/go/main/App.js` 调用 Go 方法。

### 前端结构特点

```text
gui/frontend/
├── package.json                 # npm scripts 与前端依赖
├── vite.config.js               # Vite + Svelte 配置，开发端口 5173
├── tailwind.config.js
├── postcss.config.js
├── src/
│   ├── main.js                  # Svelte 挂载入口
│   ├── App.svelte               # 初始化判断、主题切换、页面容器
│   ├── style.css                # 全局样式与主题变量
│   ├── pages/
│   │   ├── Onboarding.svelte    # 首次设置/加入已有远程
│   │   ├── Dashboard.svelte     # 概览、同步状态、快捷同步
│   │   ├── Files.svelte         # 配置文件树、diff、冲突处理
│   │   ├── Binaries.svelte      # Claude 二进制版本管理
│   │   ├── Projects.svelte      # 项目级配置同步
│   │   ├── History.svelte       # 快照历史和回滚
│   │   └── Settings.svelte      # WebDAV、加密密码、排除规则等设置
│   └── lib/components/
│       ├── Sidebar.svelte       # 左侧导航和同步状态
│       └── TreeNode.svelte      # 文件树节点
├── wailsjs/                     # Wails 生成的前后端绑定代码
└── dist/                        # 前端构建产物，已忽略
```

### 当前能力

GUI 面向桌面可视化场景，当前包含：

- Onboarding：首次配置 WebDAV、加密密码、设备名，也可加入已有远程。
- 概览页：显示同步状态、远程健康状态、快捷推送/拉取/同步。
- 配置页：展示配置文件树、查看内容和 diff、处理冲突、批量同步。
- 二进制页：查看本地/远程 Claude 版本、上传当前版本、切换版本、删除版本。
- 项目页：管理项目级 `.claude.json` 同步。
- 历史页：查看快照历史、快照详情、回滚。
- 设置页：WebDAV、加密密码、排除规则、二进制路径等配置。
- 系统托盘：推送、拉取、同步、打开主窗口、开机自启动、退出。
- 文件监听：已初始化后监听 `~/.claude/`，变化后标记待同步。
- 主题切换：支持亮色/暗色主题，保存在 localStorage。

### GUI 构建与测试

```bash
# 从根目录构建 GUI
make build-gui

# 从根目录测试 GUI
make test-gui

# 直接在 gui/ 内构建
cd gui && wails build -clean -nopackage -m -nosyncgomod

# 直接在 gui/ 内测试
cd gui && go test ./...
```

GUI 构建产物：

```text
gui/build/bin/cc-box-gui.exe
```

## 根目录 Makefile

根目录提供统一入口：

| 命令 | 说明 |
| --- | --- |
| `make build` | 同时构建 CLI 和 GUI。 |
| `make build-cli` | 只构建 CLI。 |
| `make build-gui` | 只构建 GUI。 |
| `make test` | 同时测试 CLI 和 GUI。 |
| `make test-cli` | 只测试 CLI。 |
| `make test-gui` | 只测试 GUI。 |
| `make clean` | 删除 CLI 和 GUI 构建产物。 |

如果当前环境没有 `make`，可以直接进入对应目录执行上面的 Go/Wails 命令。

## 环境要求

| 依赖 | 版本 | 用途 |
| --- | --- | --- |
| Go | 1.25+ | CLI 和 GUI 后端编译、测试 |
| Wails CLI | v2.x | GUI 构建和开发模式 |
| Node.js / npm | 与当前前端依赖兼容即可 | GUI 前端依赖安装和构建 |
| WebDAV 服务 | 标准 WebDAV | 配置和对象存储远程同步 |
| Git | 任意 | 项目级配置同步时用于识别项目 remote |

## 同步内容

```text
~/.claude/                        # Claude Code 配置目录
├── settings.json                 # 全局设置
├── CLAUDE.md                     # 全局指令
├── skills/                       # 自定义技能
├── commands/                     # 自定义命令
├── agents/                       # 自定义 Agent
├── plugins/                      # 插件配置
└── ...

~/.local/                         # 二进制与版本管理
├── bin/
│   └── claude.exe                # Claude 主程序或配置指定的二进制
└── share/claude/versions/        # 历史版本存档

<project>/.claude.json            # 项目级 MCP 配置
```

默认不应同步 OAuth token、本地权限、会话、缓存、遥测和锁文件等机器相关或可重建内容。

## 核心机制

### 版本化同步

CC-Box 的同步不是简单覆盖文件。每次 push 会生成快照，记录文件路径、哈希、状态和父快照，形成类似 Git 的快照链。pull 时基于本地、远程和共同祖先做合并。

### 端到端加密

```text
用户加密密码 → Argon2id 派生 → AES-256-GCM 加密
```

加密密码用于派生数据加密密钥。界面和命令中应统一称为“加密密码”。

### 跨平台规范化

- 路径分隔符统一为 `/`。
- 文本换行按 LF 参与哈希计算。
- Windows 下路径大小写按规则规范化。

### 冲突处理

当本地和远程相对共同祖先同时变更时，系统会生成冲突信息。CLI 通过 `conflicts` / `resolve` 处理，GUI 通过配置页展示本地/远程内容、差异和推荐选择。

## 开发约束

- 只开发 CLI 时，主要进入 `cli/`，不要改 `gui/`。
- 只开发 GUI 时，主要进入 `gui/`，不要改 `cli/`。
- 需要两端行为一致时，分别修改 `cli/internal/` 和 `gui/internal/`。
- 不要在根目录重新添加共享 `internal/` 或根 `go.mod`，除非明确要重新设计为共享模块。
- 不要让 CLI import GUI，也不要让 GUI import CLI。
- 构建产物留在 `cli/build/bin/` 和 `gui/build/bin/`。

## 文档入口

- CLI 详细说明：[cli/README.md](./cli/README.md)
- GUI 详细说明：[gui/README.md](./gui/README.md)

## License

MIT
