# CC-Box GUI

CC-Box GUI 是 CC-Box 的桌面图形版本，用来可视化管理 Claude Code 配置同步、冲突处理、快照历史、项目配置和 Claude 二进制版本。

它适合这些场景：

- 想用界面完成 WebDAV 初始化和同步配置。
- 想直观看到本地和远程配置差异。
- 想在图形界面里解决冲突，而不是只看命令行输出。
- 想通过托盘常驻提醒配置是否需要同步。
- 想管理 Claude 二进制版本，但不想记命令。

## 能做什么

### 首次配置向导

首次启动时，GUI 会进入 Onboarding 流程，用来配置：

- WebDAV 地址、用户名、密码和远程目录
- 加密密码
- 当前设备名称
- 创建新远程或加入已有远程

### 同步状态概览

概览页用于快速了解当前状态：

- 本地是否已初始化
- 远程是否已初始化
- 是否存在本地或远程变更
- 是否存在冲突
- 加密密码是否匹配
- 是否需要修复远程数据

也可以直接执行快捷操作：

- 推送本地配置
- 拉取远程配置
- 一键同步
- 从本地修复未初始化远程

### 配置文件管理

配置页提供图形化文件视图：

- 浏览 `~/.claude/` 配置文件树
- 查看文件内容
- 查看本地和远程 diff
- 查看冲突详情
- 选择本地或远程版本
- 编辑并保存合并后的冲突内容
- 批量推送或拉取配置变更

### Claude 二进制管理

二进制页用于管理 Claude 可执行文件：

- 查看当前 Claude 二进制解析结果
- 查看本地历史版本
- 查看远程已备份版本
- 上传当前二进制
- 上传指定版本
- 切换到指定版本
- 删除本地或远程版本
- 查看二进制存储占用

### 项目配置管理

项目页用于管理项目级 `.claude.json`：

- 添加项目路径
- 查看已追踪项目
- 查看项目配置详情
- 处理远程 orphan 项目配置

### 快照历史和回滚

历史页用于查看同步历史：

- 查看本地快照列表
- 查看远程快照列表
- 查看快照详情
- 回滚到指定快照

### 设置和加密密码管理

设置页用于维护运行配置：

- 查看和修改 WebDAV 配置
- 保存 WebDAV 密码
- 测试连接
- 配置排除规则
- 查看 Claude 二进制路径解析结果
- 验证加密密码
- 预览加密密码是否匹配
- 保存或更改加密密码

### 系统托盘和自动提醒

GUI 支持系统托盘常驻：

- 托盘菜单执行推送、拉取、同步
- 打开主窗口
- 退出应用
- 根据状态切换托盘图标
- 监听 `~/.claude/` 文件变化并标记待同步

## 快速开始

构建 GUI：

```bash
wails build -clean -nopackage -m -nosyncgomod
```

运行构建产物：

```text
gui/build/bin/cc-box-gui.exe
```

如果从 `gui/` 目录内运行，则产物路径是：

```text
build/bin/cc-box-gui.exe
```

首次启动后按照界面提示完成 WebDAV 和加密密码配置即可。

## 页面说明

| 页面 | 作用 |
| --- | --- |
| Onboarding | 首次初始化或加入已有远程。 |
| 概览 | 查看同步状态，执行快捷同步。 |
| 配置 | 查看配置文件树、diff 和冲突。 |
| 二进制 | 管理 Claude 二进制备份和版本切换。 |
| 项目 | 管理项目级 `.claude.json`。 |
| 历史 | 查看快照历史和执行回滚。 |
| 设置 | 管理 WebDAV、排除规则、加密密码和二进制路径。 |

## 目录结构

```text
gui/
├── main.go                      # Wails 应用入口
├── app.go                       # 应用生命周期和桌面能力
├── dashboard.go                 # 概览和同步状态
├── files.go                     # 配置文件树、diff、冲突处理
├── pages.go                     # 历史、设置、项目、二进制等页面数据
├── onboarding.go                # 首次配置流程
├── async.go                     # 后台任务和取消操作
├── watcher.go                   # 文件监听和待同步提醒
├── tray.go                      # 系统托盘
├── frontend/                    # Svelte 前端工程
├── internal/                    # GUI 业务实现
├── wails.json                   # Wails 构建配置
├── go.mod
└── go.sum
```

前端目录：

```text
frontend/
├── src/
│   ├── App.svelte               # 页面容器、初始化判断、主题切换
│   ├── pages/                   # 页面组件
│   ├── lib/components/          # 复用组件
│   ├── main.js                  # 前端入口
│   └── style.css                # 全局样式
├── package.json
├── vite.config.js
├── tailwind.config.js
└── postcss.config.js
```

## 技术栈

| 依赖 | 用途 |
| --- | --- |
| Go 1.25+ | GUI 后端编译和测试。 |
| Wails v2 | 桌面应用窗口、菜单、资源嵌入和前后端绑定。 |
| Svelte | 前端页面和组件。 |
| Vite | 前端开发服务和构建。 |
| Tailwind CSS | 前端样式。 |
| fsnotify | 监听 `~/.claude/` 文件变化。 |
| systray | 系统托盘和托盘菜单。 |
| WebDAV | 远程配置和对象存储。 |

## 构建

在 `gui/` 目录执行：

```bash
wails build -clean -nopackage -m -nosyncgomod
```

从仓库根目录也可以执行：

```bash
cd gui && wails build -clean -nopackage -m -nosyncgomod
```

构建产物：

```text
gui/build/bin/cc-box-gui.exe
```

## 前端开发

安装前端依赖：

```bash
cd frontend
npm install
```

单独构建前端：

```bash
npm run build
```

启动前端开发服务：

```bash
npm run dev
```

启动 Wails 开发模式：

```bash
wails dev
```

## 测试

在 `gui/` 目录执行：

```bash
go test ./...
```

从仓库根目录也可以执行：

```bash
go -C gui test ./...
```

当前测试覆盖 GUI 数据处理、文件树、diff、冲突处理、Dashboard 状态、虚拟 WebDAV 流程、二进制生命周期、密钥轮转，以及 GUI 自带 `internal/` 业务逻辑。共享核心能力的单元测试在 `core/` module 中运行。

## 手动验证建议

GUI 是桌面应用，构建通过不等于界面交互一定正常。发布前建议手动验证：

1. 首次启动是否进入 Onboarding。
2. 已初始化环境是否进入主界面。
3. 概览页推送、拉取、同步是否可用。
4. 配置页文件树、diff、冲突处理是否可用。
5. 二进制、项目、历史、设置页面是否能正常加载。
6. 关闭窗口是否隐藏到托盘。
7. 托盘菜单里的打开主窗口、同步和退出是否正常。
8. 亮色/暗色主题切换是否保存。

## 与 CLI 的关系

GUI 和 CLI 是两个独立应用，但都依赖根目录的 `core/` module。GUI 可以单独开发、构建和发布，不需要依赖 CLI。

如果你只需要桌面图形界面，只关注当前 `gui/` 目录即可；如果修改同步、加密、快照、WebDAV 或 Claude 二进制管理，需要同时关注 `../core/`。
