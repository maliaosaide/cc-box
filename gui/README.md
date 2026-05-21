# CC-Box GUI

`gui/` 是 CC-Box 的桌面图形应用，基于 Wails v2 + Svelte。它已经与 CLI 完全拆离，拥有自己的 Go module、Wails 配置、前端工程、后端绑定、业务代码和构建产物，可以单独开发、测试和发布。

## 模块边界

- 模块路径：`github.com/user/cc-box/gui`
- GUI 入口：`main.go`
- Wails 配置：`wails.json`
- 前端工程：`frontend/`
- 后端业务代码：`*.go` 与 `internal/`
- GUI 只引用 `github.com/user/cc-box/gui/internal/...`
- GUI 不引用 `cli/`，也不依赖根目录 Go module

如果需要修改同步、加密、快照、WebDAV、二进制管理等底层逻辑，只改这里不会自动影响 CLI；CLI 有自己的 `internal/` 副本。

## 目录结构

```text
gui/
├── main.go                      # Wails 桌面入口，配置窗口、菜单和资源嵌入
├── app.go                       # App 生命周期、初始化状态、系统对话框、资源管理器打开
├── dashboard.go                 # 概览页数据、QuickPush/QuickPull/QuickSync、远程修复
├── files.go                     # 文件树、文件内容、diff、冲突详情、冲突解决、批量同步
├── pages.go                     # 历史、设置、项目、二进制、加密密码等页面后端接口
├── onboarding.go                # 首次初始化、加入已有远程、WebDAV 检测
├── async.go                     # 后台操作、取消和进度状态管理
├── watcher.go                   # ~/.claude 文件监听、托盘待同步状态、自动同步触发
├── tray.go                      # 系统托盘、状态图标、托盘菜单、开机自启动
├── icon*.ico                    # 托盘状态图标
├── frontend/                    # Svelte 前端工程
├── internal/                    # GUI 独立业务代码副本
├── build/                       # Wails 构建输出，bin/ 已被 .gitignore 忽略
├── wails.json
├── go.mod
└── go.sum
```

`internal/` 的职责与 CLI 中同名包一致，但两边代码已经拆离：

| 包 | 说明 |
| --- | --- |
| `internal/config` | 配置读写、路径解析、密钥环集成 |
| `internal/crypto` | Argon2id 派生、AES-256-GCM 加密、密钥指纹 |
| `internal/webdav` | WebDAV 客户端、锁、XML 解析 |
| `internal/object` | 对象哈希、对象存储读写 |
| `internal/snapshot` | 快照模型、扫描、差异计算 |
| `internal/sync` | 文本/JSON/history 三方合并与冲突处理 |
| `internal/binary` | Claude 二进制探测、分块上传、版本切换 |
| `internal/project` | 项目级 `.claude.json` 同步与合并 |
| `internal/normalize` | 路径、换行、内容哈希规范化 |

## 前端结构

```text
frontend/
├── package.json                 # npm scripts 与前端依赖
├── vite.config.js               # Vite + Svelte 配置，开发端口 5173
├── tailwind.config.js           # Tailwind 配置
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
└── dist/                        # 前端构建产物，已被 .gitignore 忽略
```

页面导航包括：概览、配置、二进制、项目、历史、设置。`App.svelte` 会先通过 Wails 绑定调用 `IsInitialized()`，未初始化时显示 Onboarding，已初始化时显示主界面。

## Wails 后端绑定

`main.go` 将 `App` 绑定给前端：

```go
Bind: []interface{}{app}
```

前端通过 `frontend/wailsjs/go/main/App.js` 调用 Go 方法。当前主要绑定能力包括：

- 初始化：`TestWebDAVConnection`、`DetectExistingSetup`、`InitNewDevice`、`InitJoinExisting`
- 概览同步：`GetDashboard`、`QuickPush`、`QuickPull`、`QuickSync`、`RepairRemoteFromLocal`
- 文件配置：`GetFileTree`、`GetFileContent`、`GetFileDiff`、`GetConflictDetail`、`ResolveConflict`、`SaveMergedConflict`、`BulkSync`
- 历史快照：`GetSnapshotList`、`GetLocalSnapshotList`、`GetSnapshotDetail`、`RevertToSnapshot`
- 设置：`GetConfig`、`SetConfigField`、`SetWebDAVPassword`、`AddExcludePattern`、`RemoveExcludePattern`、`TestConnection`
- 项目：`GetProjectList`、`GetProjectDetail`、`AddProjectPath`、`DeleteOrphan`
- 二进制：`GetBinaryPage`、`GetBinaryStorage`、`SwitchBinaryVersion`、`UploadCurrentBinary`、`UploadBinaryVersion`、`DeleteLocalVersion`、`DeleteCloudBinaryVersion`
- 加密密码：`GetEncryptionStatus`、`VerifyEncryptionKey`、`PreviewEncryptionPassword`、`SaveEncryptionPassword`、`ChangeEncryptionPassword`
- 桌面能力：`BrowseFolder`、`BrowseFile`、`OpenInExplorer`、`CancelOperation`

## 桌面特性

- Wails 原生窗口，默认尺寸 `1120x720`，最小尺寸 `900x600`。
- 关闭窗口时默认隐藏到托盘，托盘退出才真正退出。
- 托盘菜单支持推送、拉取、同步、打开主窗口、开机自启动和退出。
- 托盘图标显示 `已同步`、`待同步`、`冲突或连接错误`、`同步中` 状态。
- 已初始化后会启动文件监听，监听 `~/.claude/` 变化并标记待同步。
- 前端支持亮色/暗色主题切换，主题保存在 localStorage。

## 构建

在 `gui/` 目录内构建：

```bash
wails build -clean -nopackage -m -nosyncgomod
```

也可以从仓库根目录执行：

```bash
make build-gui
```

构建产物：

```text
gui/build/bin/cc-box-gui.exe
```

`wails.json` 当前输出名为 `cc-box-gui`，避免和 CLI 的 `cc-box.exe` 混淆。

## 前端开发

安装依赖：

```bash
cd frontend
npm install
```

前端单独构建：

```bash
npm run build
```

前端开发服务：

```bash
npm run dev
```

Wails 开发模式通常从 `gui/` 目录启动：

```bash
wails dev
```

## 测试

在 `gui/` 目录内运行全部 Go 测试：

```bash
go test ./...
```

也可以从仓库根目录执行：

```bash
make test-gui
```

当前测试覆盖的主要范围：

- GUI 文件树、diff、冲突详情和冲突推荐选择
- Dashboard 同步状态、远程未初始化、远程数据不完整、加密密码不匹配等场景
- 虚拟 WebDAV 双设备工作流、二进制生命周期、密钥轮转
- 前端可点击元素和 Settings 标签关联的回归检查
- `internal/` 中的加密、规范化、对象、快照、合并、项目和二进制逻辑

## 手动验证建议

构建后运行：

```text
gui\build\bin\cc-box-gui.exe
```

建议重点验证：

1. 首次启动是否进入 Onboarding。
2. 已初始化环境是否进入主界面。
3. 概览页快捷推送、拉取、同步按钮是否可用。
4. 配置页文件树、diff、冲突解决是否正常。
5. 二进制、项目、历史、设置页面是否能正常加载。
6. 关闭窗口是否隐藏到托盘，托盘“打开主窗口”和“退出”是否正常。
7. 托盘同步状态图标是否随操作变化。
8. 亮色/暗色主题切换是否保存。

## 开发约束

- GUI 的业务代码留在 `gui/` 和 `gui/internal/`。
- 不要从 GUI import `github.com/user/cc-box/cli/...`。
- 不要恢复对根目录 `github.com/user/cc-box/internal/...` 的引用。
- `frontend/wailsjs/` 是 Wails 生成绑定，后端方法变化后由 Wails 重新生成。
- 如果需要 CLI 和 GUI 行为一致，分别修改 `gui/internal/` 和 `cli/internal/` 中对应代码。
