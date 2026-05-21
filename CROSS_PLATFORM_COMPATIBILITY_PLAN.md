# CC-Box GUI 跨平台兼容计划

本文档记录 CC-Box GUI 面向 Windows、macOS、Linux 三个平台的兼容改造计划。目标是在不破坏 CLI/GUI 共享数据兼容性的前提下，逐步统一桌面体验、平台路径、二进制管理和安全边界。

## 当前决策

- 不做开机自启动功能。
- 删除现有 GUI 中的自启动入口和相关实现。
- 后续计划不再包含 `AutoStartManager`。
- 桌面集成能力只放在 `gui/`。
- CLI/GUI 共享数据能力只放在 `core/`。
- 不改变 WebDAV 远程路径、snapshot JSON、object hash、加密格式、`HEAD` 语义、二进制 index JSON。

## 总体边界

| 能力 | 放置位置 | 原因 |
| --- | --- | --- |
| FileOpener | `gui/` | 依赖 Explorer、Finder、xdg-open/gio 等桌面环境。 |
| TrayAdapter | `gui/` | 依赖 systray、菜单、窗口显示隐藏和 GUI 生命周期。 |
| Watcher | `gui/` | 当前 watcher 是 GUI 自动同步体验，依赖托盘状态和异步任务。 |
| SecretStore | `core/` | CLI 和 GUI 都需要读取和保存 WebDAV 密码，必须共享。 |
| BinaryPlatform | `core/` | 二进制平台 key、候选路径、上传下载和远程 index 是共享协议。 |
| path safety | `core/` | CLI/GUI 都需要一致的路径越界防护。 |
| config | `core/` | `~/.cc-box/config.toml` 是 CLI/GUI 共享状态。 |
| project tracker | 后续评估放入 `core/` | 当前 CLI/GUI 仍有重复，但不放在第一阶段处理。 |

## 不做的范围

以下内容明确不纳入本轮跨平台兼容计划：

- 不做开机自启动。
- 不实现 Windows Startup、注册表 Run key、macOS LaunchAgent、Linux XDG autostart。
- 不保留 GUI 中的“开机自启动”托盘菜单项。
- 不把 Wails、systray、fsnotify 引入 `core/`。
- 不迁移 `~/.cc-box` 到 `%APPDATA%`、`Application Support` 或 XDG config 目录。
- 不改变 `config.Platform()` 返回格式。
- 不改变 WebDAV 远程目录结构。
- 不改变 snapshot JSON 结构。
- 不改变 object hash 规则。
- 不改变加密格式。
- 不改变二进制远程 index JSON。
- 不在同一阶段同时重写 watcher、二进制平台、SecretStore 和 project tracker。

## 当前主要问题

### 1. 文件打开逻辑不是完整的平台适配

当前 `gui/app.go` 中 `OpenInExplorer` 使用 `runtime.BrowserOpenURL` 打开 `file:///` URL。这在 Windows、macOS、Linux 上的行为不够一致，也不能保证实现“在文件管理器中定位文件”。

后续应抽出 `FileOpener`：

```go
type FileOpener interface {
    Open(path string) error
    Reveal(path string) error
}
```

目标行为：

| 平台 | 目录 | 文件 |
| --- | --- | --- |
| Windows | Explorer 打开目录 | Explorer 选中文件 |
| macOS | Finder/open 打开目录 | Finder reveal 文件 |
| Linux | `xdg-open` / `gio open` 打开目录 | 优先打开父目录，必要时 fallback |

### 2. 托盘和业务逻辑耦合较重

当前 `gui/tray.go` 同时负责：

- systray 初始化。
- 菜单构造。
- 托盘图标状态。
- QuickPush、QuickPull、QuickSync 调用。
- 打开主窗口。
- 退出应用。
- 自启动状态检测和设置。

后续应抽出 `TrayAdapter`，并删除自启动相关菜单和逻辑。

建议接口：

```go
type TrayAdapter interface {
    Start(actions TrayActions) error
    Stop()
    SetState(state TrayState)
    IsReady() bool
}

type TrayActions struct {
    OnPush func()
    OnPull func()
    OnSync func()
    OnOpen func()
    OnQuit func()
}
```

托盘菜单保留：

```text
+----------------------+
|  ↑ 推送配置           |
|  ↓ 拉取配置           |
|  ⟷ 同步               |
|  ------------------- |
|  打开主窗口           |
|  ------------------- |
|  退出                 |
+----------------------+
```

### 3. SecretStore 当前只是文件 fallback

当前 WebDAV 密码读取入口在 `core/config` 中，实际实现使用 `~/.cc-box/secrets.json`。这可以作为 fallback，但不应该继续被当作真正的系统密钥环。

后续应在 `core` 中抽 `SecretStore`，保持现有公开 API 不变：

- `config.LoadWebDAVPassword()`
- `config.SaveWebDAVPassword(password)`

推荐读取顺序：

1. 环境变量 `CC_BOX_WEBDAV_PASSWORD`。
2. OS SecretStore。
3. 旧的 `~/.cc-box/secrets.json` fallback。
4. 如果从旧文件读到密码，尝试写入 OS SecretStore。
5. 不自动删除旧文件，方便回滚。

注意：第一阶段不迁移 `~/.cc-box/key.bin`，避免扩大加密恢复风险。

### 4. BinaryPlatform 分散在 `core/binary` 和 `core/config`

当前二进制平台逻辑涉及：

- `config.Platform()`。
- `LocalBinDir()`。
- `VersionsDir()`。
- Claude 候选文件名。
- Claude 常见安装目录。
- managed binary path。
- Windows 路径大小写比较。
- Windows 原子替换逻辑。
- WebDAV 上的二进制远程路径。

后续应集中为 `BinaryPlatform`，但必须保持远程协议不变。

必须保持稳定：

- 平台 key：`GOOS-GOARCH`，例如 `windows-amd64`、`darwin-arm64`、`linux-amd64`。
- index JSON：`binaries/index.json`。
- 远程路径：`binaries/<platform>/<name>-<version>.<ext>`。
- 分块路径：`binaries/parts/<hash>/...`。
- `.enc` / `.bin` 扩展语义。

### 5. Watcher 需要硬化，但不进入 `core`

当前 `gui/watcher.go` 直接依赖：

- `fsnotify`。
- 托盘状态。
- `appRef.QuickSync()`。
- 全局异步任务状态。

它不是纯文件监听器，而是 GUI 自动同步控制器。后续应留在 `gui/`，拆成：

- `FileWatchService`：负责文件系统事件监听和归一化。
- `AutoSyncController`：负责 debounce、定时同步、状态回调和 QuickSync 调用。

平台注意点：

| 平台 | 风险 |
| --- | --- |
| Windows | 重复事件、文件锁、编辑器保存时短暂不可读。 |
| macOS | 原子保存通常表现为 rename。 |
| Linux | inotify watch 数量限制、桌面环境差异。 |

### 6. path safety 应进入 `core`

当前 CLI 和 GUI 都有自己的 `safeJoin` 实现，逻辑重复。路径越界防护是安全边界，应移动到共享 core。

目标：

- CLI/GUI 使用同一份路径安全判断。
- 处理空路径、空字节、`..`、绝对路径、Windows volume name、反斜杠路径。
- 不改变 snapshot path key 的规范化规则。

## 分阶段实施计划

### 阶段 0：建立当前行为基线

目标：确认现有 Windows 行为和测试基线，不做功能重构。

验证项：

```bash
go -C core test ./...
go -C cli test ./...
go -C gui test ./...
npm --prefix gui/frontend run build
cd gui && wails build
```

Windows 手动验证：

- GUI 能启动。
- 托盘能显示。
- 关闭窗口时能最小化到托盘。
- 托盘推送、拉取、同步能触发。
- 设置页路径能保存。
- 二进制页面能检测 Claude。
- 文件变更后 watcher 能标记待同步。

成功标准：

- 明确当前行为基线。
- 后续阶段可以判断问题是否由重构引入。

### 阶段 1：GUI 桌面适配层

目标：只处理 GUI 桌面体验，不碰 core 协议。

改动范围：

```text
gui/app.go
gui/tray.go
gui/exec_windows.go
gui/exec_other.go
gui/internal/desktop/
```

实施内容：

1. 新增 `FileOpener`。
2. `App.OpenInExplorer` 保持 Wails 绑定名不变，内部委托 `FileOpener`。
3. 新增 `TrayAdapter`。
4. 托盘菜单通过 callback 调用 GUI 方法。
5. 删除现有自启动菜单项。
6. 删除现有自启动检测和设置逻辑。
7. Linux 托盘不可用时提供 no-op fallback，不阻塞主窗口启动。

建议文件结构：

```text
gui/internal/desktop/
  file_opener.go
  file_opener_windows.go
  file_opener_darwin.go
  file_opener_linux.go
  tray.go
  tray_systray.go
  tray_noop.go
```

验证：

- Windows：Explorer 打开目录和定位文件。
- macOS：Finder 打开目录和 reveal 文件。
- Linux：`xdg-open` / `gio open` 可用时打开目录；不可用时返回明确错误。
- 托盘菜单只包含推送、拉取、同步、打开主窗口、退出。
- 不再出现“开机自启动”。
- 托盘不可用不影响主窗口启动。

### 阶段 2：core SecretStore 和 path safety

目标：统一 CLI/GUI 的共享安全能力。

改动范围：

```text
core/config/
core/pathutil/ 或 core/internal/pathutil/
cli/internal/cli/path_safety.go
gui/files.go
```

实施内容：

1. 抽 `SecretStore`。
2. 保持 `LoadWebDAVPassword` / `SaveWebDAVPassword` API 不变。
3. 保留 `secrets.json` fallback。
4. 抽共享 `SafeJoin`。
5. CLI/GUI 都调用 core 的 path safety。

验证：

- 环境变量密码优先。
- 旧 `secrets.json` 仍可读取。
- 保存后 CLI/GUI 都能读取 WebDAV 密码。
- path traversal 测试覆盖：
  - `../x`
  - `..\\x`
  - `/abs/path`
  - `C:\\abs\\path`
  - 空字节
  - 正常相对路径

### 阶段 3：集中 BinaryPlatform

目标：集中平台判断，不改变远程协议。

改动范围：

```text
core/binary/resolve.go
core/binary/index.go
core/binary/download.go
core/binary/upload.go
core/config/config.go
```

实施内容：

1. 新增 `core/binary/platform.go`。
2. 集中 managed binary name。
3. 集中 Claude candidate names。
4. 集中 common install dirs。
5. 集中 executable extension。
6. 集中 path comparison case sensitivity。
7. 保留 Windows executable replace 逻辑。

验证：

- Windows 候选名包含 `.exe`、`.cmd`、`.bat`、`.ps1`。
- macOS/Linux 候选名为 `claude`。
- Windows managed binary path 仍加 `.exe`。
- `config.Platform()` 仍返回 `GOOS-GOARCH`。
- 旧 `binaries/index.json` 可解析。
- 上传、下载、删除远程路径不变。
- CLI/GUI 二进制页面和命令行为一致。

### 阶段 4：Watcher 跨平台硬化

目标：增强 GUI 自动同步稳定性，不改变同步算法。

改动范围：

```text
gui/watcher.go
gui/async.go
TrayAdapter 相关文件
```

实施内容：

1. 拆出 `FileWatchService`。
2. 拆出 `AutoSyncController`。
3. 监听新增目录时记录错误，不再静默忽略。
4. 事件归一化后再触发 pending 状态。
5. QuickSync 结果通过回调更新托盘状态。

验证：

- 新建文件。
- 修改文件。
- 删除文件。
- rename/atomic save。
- 新建子目录后继续监听。
- 快速连续变更只触发一次 pending。
- 同步成功回到 synced。
- 同步失败进入 conflict/error。

### 阶段 5：project tracker 后置整理

目标：解决 CLI/GUI 项目发现逻辑重复和路径解码风险。

当前问题：

- CLI/GUI 都有 `internal/project`。
- 当前扫描路径硬编码为 `~/.claude/projects`。
- `decodeProjectDir` 使用 `strings.Split(dirName, "-")`，路径段包含 `-` 时有误解析风险。

建议：

- 不在第一阶段处理。
- 等桌面适配和 core 安全边界稳定后，再评估抽到 `core/project`。
- 抽离前先补测试，避免改变项目发现行为。

## 跨平台验证矩阵

| 能力 | Windows | macOS | Linux |
| --- | --- | --- | --- |
| GUI 启动 | 必测 | 必测 | 必测 |
| Wails build | 必测 | 必测 | 必测 |
| FileOpener 打开目录 | Explorer | Finder/open | xdg-open/gio |
| FileOpener 定位文件 | explorer /select | open -R | 打开父目录或 fallback |
| 托盘显示 | 必测 | 必测 | 尽力支持，允许 no-op |
| 托盘同步菜单 | 必测 | 必测 | 托盘可用时必测 |
| 自启动 | 不做 | 不做 | 不做 |
| WebDAV 密码读取 | env + SecretStore + fallback | env + SecretStore + fallback | env + SecretStore + fallback |
| 二进制检测 | exe/cmd/bat/ps1 | claude | claude |
| 二进制下载替换 | 保留 Windows replace 逻辑 | os.Rename | os.Rename |
| watcher | 文件锁/重复事件 | rename/atomic save | inotify 限制 |

## 风险与处理策略

| 风险 | 影响 | 策略 |
| --- | --- | --- |
| 改动 `config.Platform()` | 旧远程二进制索引失效 | 保持 `GOOS-GOARCH` 不变。 |
| 改动 snapshot path key | 多平台同步出现新增/删除误判 | 不改 `normalize.PathLower` 语义。 |
| SecretStore 迁移失败 | 用户 WebDAV 密码不可用 | 保留 `secrets.json` fallback。 |
| Linux tray 不可用 | 主窗口无法启动或托盘缺失 | no-op fallback，不阻塞主流程。 |
| Windows 二进制替换失败 | 下载/切换版本失败 | 保留现有 Windows replace 逻辑并明确报错。 |
| watcher 漏事件 | 自动同步不可靠 | watcher 硬化阶段单独处理，不改同步算法。 |
| project path 解码误判 | 项目发现不准确 | 后置为单独阶段，先补测试。 |

## 推荐下一步

优先执行阶段 1：GUI 桌面适配层。

最小可交付内容：

1. 新增 `FileOpener` 并接入 `App.OpenInExplorer`。
2. 新增 `TrayAdapter`，将托盘菜单和业务回调解耦。
3. 删除 GUI 自启动菜单和相关代码。
4. 确保 Windows 现有托盘行为不退化。
5. 确保 macOS/Linux 至少可构建、可启动，托盘不可用时可降级。

阶段 1 不触碰：

- WebDAV 协议。
- snapshot JSON。
- object hash。
- 加密格式。
- 二进制远程 index。
- `config.Platform()`。
- CLI 命令行为。
