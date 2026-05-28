# 恢复备份与 Claude 二进制恢复设计草案

本文是关于 CC-Box 在新机器或空系统上执行“恢复备份”时，如何处理 Claude 配置数据和 Claude 二进制文件的设计草案，供后续讨论。

## 背景问题

恢复备份本质上是从 WebDAV 拉取远端数据并落地到当前系统。

但在新系统上，可能存在以下情况：

- 没有 `~/.claude/`。
- 没有 `~/.claude.json`。
- 没有 `~/.local/bin/`。
- 没有 Claude binary。
- 有 Claude binary，但版本和备份快照不一致。
- 有 Claude binary，但来自系统全局安装、npm shim、只读路径或其他非当前配置/官方安装位置。
- 已下载 Claude binary，但当前 shell 的 `PATH` 不会解析到它。

因此恢复备份不能只理解为“恢复文件”，还需要单独处理 Claude binary 的恢复与激活状态。

## 当前实现状态标记（2026-05-24 核查）

> 本节用于区分“既有 WebDAV 二进制管理能力”“本设计已接入的同步/恢复语义”和“仍未完成的设计内容”。后续实现必须以本文设计为目标，不能把 UI 占位、旧能力或局部接入误判为完整实现。

### 已落地边界

- GUI / CLI 已有 WebDAV 已备份 Claude binary 的基础管理能力，包括上传、下载/切换、远端 index、加密/分块、hash 校验和本地缓存。这是既有能力，不等同于 GitHub Release 或官方安装来源已实现。
- `binary.sync_enabled` 已作为普通 push / pull / sync 是否纳入 Claude binary 状态的开关接入主要 CLI / GUI 同步流程。
- 快照恢复 / 回滚已能按 `snapshot.Binary[currentPlatform]["claude"]` 从 WebDAV 精确恢复当前平台 Claude binary。
- GUI 删除入口已改为统一删除语义，但后端旧的本地删除 / 云端删除接口仍残留，需要清理。

### 未完成实现

| 状态 | 内容 | 当前真实情况 | 实现约束 |
| --- | --- | --- | --- |
| 未完成 | 官方最新版安装入口 | GUI 只有“官方安装源尚未接入”占位；CLI 没有 `binary install --source official --latest`；后端没有执行官方安装脚本的方法。 | 必须由用户显式点击/执行；Windows 使用官方 PowerShell 安装命令，macOS / Linux 使用官方 shell 安装命令；官方安装器自己的 PATH 行为不受 `binary.auto_configure_path` 限制；安装完成后只重新检测当前 `claude` 路径、版本和来源，不迁移、不导入、不额外搬运官方安装结果。 |
| 未完成 | GitHub Release 版本源 | GUI 只有“GitHub 版本源尚未接入”占位；CLI 没有 `binary install --source github --version ...`；后端没有读取 release、筛选 asset、下载、校验、解压、替换的实现。 | 必须读取官方 `anthropics/claude-code` releases；只展示当前平台有 asset 的版本；必须通过平台映射筛选 asset，不能直接拼接 `config.Platform()`；下载后校验压缩包，解压定位 `claude` / `claude.exe`，备份现有目标文件并原子替换；安装后检测版本。 |
| 未完成 | GitHub / 官方来源的 GUI 真实数据和操作 | 二进制页面已有来源标签，但 GitHub / 官方标签仍是 pending UI，占位文案不能视为功能完成。 | 标签页必须展示真实可操作状态：当前平台是否支持、可安装版本、失败原因、重试入口和安装动作；GitHub 不可连接时只能影响 GitHub 来源，不阻塞本地/WebDAV 页面。 |
| 未完成 | CLI 来源模型 | `cc-box binary` 目前只有 WebDAV 语义的 `list` / `push` / `pull` / `switch` / `prune`。 | 需要补齐 `cc-box binary install --source github --version <version>` 和 `cc-box binary install --source official --latest`；CLI 输出必须包含当前版本、平台、来源、路径和命令状态；CLI 与 GUI 必须复用 core 中同一套安装/恢复能力。 |
| 未完成 | `binary.auto_configure_path` 真实行为 | 当前只有配置字段、保存逻辑和 UI 开关，没有 Windows 用户 PATH、PowerShell profile、`.bashrc`、`.zshrc` 等实际写入逻辑。 | 默认关闭；只有用户显式开启后，CC-Box 自己的 WebDAV 切换、同步恢复或 GitHub Release 安装才允许配置用户级 PATH / shell profile；必须幂等、只改用户级配置、不要求管理员权限；官方安装脚本自身 PATH 行为不受该开关限制。 |
| 未完成 | 外部来源安装后的统一状态刷新 | 当前没有“官方/GitHub 安装完成 → 清理检测缓存 → 重新检测版本/来源/路径/命令状态 → 刷新页面 → 可选上传 WebDAV”的完整链路。 | 安装完成后必须刷新二进制主页；外部来源版本只有在用户后续上传到 WebDAV 后，才可参与快照精确恢复。 |
| 部分完成 | 删除统一语义 | 前端已经统一调用 `DeleteBinaryVersion`；后端仍保留 `DeleteLocalVersion` / `DeleteCloudBinaryVersion`，Wails 绑定也会继续暴露。 | UI 不再提供本地/云端分裂删除；后端应移除旧接口并重新生成 Wails 绑定，避免 API 语义继续分叉。 |
| 部分完成 | `sync_enabled` 控制边界 | 普通 QuickPush / BulkPush / CLI push/pull 主流程已受开关控制；GUI onboarding 初始化远端和 RepairRemoteFromLocal 仍直接写入当前 binary 版本。 | 需要决定初始化/修复远端是否也必须遵守 `sync_enabled`；如果遵守，开关关闭时不写入 `Snapshot.Binary`；如果不遵守，需要在产品语义中明确它们不是普通同步。 |

### 实现硬约束

- 当前范围只管理 Claude binary，不恢复、不展示、不安装 `uv`、`uvx`、`uvw`、`codex`、`gemini` 或其他工具。
- 所有同步、恢复、切换、安装只作用于当前平台；不能展示或安装其他平台 asset。
- 快照精确恢复只能使用 WebDAV 已备份版本；不能在同步或快照恢复中静默调用官方安装或 GitHub Release 下载。
- GitHub Release 和官方安装都必须是用户显式安装动作，失败不能破坏配置文件同步结果。
- 写入、替换或执行外部安装前必须显式提示可能覆盖当前本地 Claude binary；能定位到现有真实二进制时必须先备份。替换失败不能留下半成品；Windows 下不能强杀占用 `claude.exe` 的进程，只能提示用户关闭后重试。
- Wails 绑定文件属于生成物；后端方法变更后通过构建/生成刷新，不手写维护绑定。

### 待确认问题

1. GUI 执行官方安装时，是否必须打开可见终端/PowerShell 窗口让用户看到官方脚本输出，还是允许在后台执行并把日志展示在 GUI？
2. 官方安装是否只提供“安装最新版”，还是也要支持官方脚本的指定版本参数？当前设计只要求官方最新版。
3. GUI onboarding 初始化远端和 RepairRemoteFromLocal 是否也必须遵守 `binary.sync_enabled`，即开关关闭时不写入 `Snapshot.Binary`？
4. GitHub Release 列表是否需要缓存上一次成功结果，还是每次进入 GitHub 标签页都实时请求并允许失败展示？
5. `auto_configure_path` 开启时，macOS / Linux 优先写入哪个 shell 配置文件：当前 `$SHELL` 对应文件，还是固定优先 `.zshrc`、`.bashrc`、`.profile`？

## 当前修订结论

本设计不再把配置文件和 Claude binary 绑定成一个不可拆分的恢复动作，而是拆成两个可选择的恢复对象：

```text
恢复对象：
- 全局配置文件
- Claude binary
- 全局配置文件 + Claude binary

恢复来源：
- 最新远端快照
- 指定历史快照
- 云端 binary 版本列表
```

关键结论：

1. **配置文件和 Claude binary 本质上可以分开恢复。**
   - 用户可以只恢复配置文件。
   - 用户可以只切换 Claude binary。
   - 用户也可以选择恢复某个快照对应的完整状态。

2. **快照代表“当时的状态”。**
   - 如果用户明确选择恢复某个快照，默认应恢复该快照中的全局配置文件。
   - 如果快照中记录了当前平台的 Claude binary 版本，默认也应恢复该版本。
   - 不新增单独恢复页面；在现有历史快照恢复动作中直接体现该语义。

3. **普通同步是否包含 binary，由设置开关控制。**
   - 关闭时：普通同步只处理配置文件，binary 页面仍可手动上传、下载、切换。
   - 开启时：同步流程自动检查当前平台 Claude binary，必要时上传或下载。

4. **跨平台时只恢复当前平台 binary。**
   - 同一个快照可以记录多个平台的 Claude 版本。
   - 当前设备只应用 `snapshot.Binary[currentPlatform]["claude"]`。
   - 配置文件可以跨平台恢复，binary 不能跨平台恢复。

## 设计原则

### 1. 恢复备份和安装环境分层处理

恢复流程分为三层：

```text
A. CC-Box 自身启动条件
   - WebDAV 配置
   - 加密密码
   - 本机设备信息
   - ~/.cc-box/ 本地状态

B. Claude 数据恢复
   - ~/.claude/
   - ~/.claude.json
   - 其他被快照追踪的 Claude 配置文件

C. Claude binary 恢复
   - 当前平台的 claude / claude.exe
   - 版本选择
   - 下载、校验、写入
   - PATH / CC_BOX_CLAUDE_PATH 激活检测
```

其中 B 是标准 pull 数据；C 是环境补全。

配置文件和二进制本质上可分开恢复；普通同步是否纳入 C 由设置开关决定；恢复快照是显式状态恢复，默认应包含 C（如果快照记录了当前平台版本），但允许用户取消。

### 2. 路径自动构建

新机器没有 `~/.claude/`、`~/.local/bin/` 是正常情况。

恢复时应该自动创建必要目录：

```text
~/.cc-box/
~/.claude/
~/.local/bin/
~/.local/share/claude/versions/
```

但不要无脑创建所有可能目录。原则是：

- 快照中存在对应文件时，创建该文件的父目录。
- 需要恢复 Claude binary 时，创建 binary 目标目录和版本归档目录。

### 3. PATH 自动配置默认关闭，可由设置显式开启

恢复流程需要检测 `PATH` 是否包含 Claude binary 所在目录，但不应默认静默修改：

- `.bashrc`
- `.zshrc`
- PowerShell profile
- Windows 用户 PATH

原因是修改环境变量属于全局副作用，影响范围超过“恢复备份”。

建议增加设置：

```toml
[binary]
auto_configure_path = false
```

建议行为：

```text
auto_configure_path = false
  → 检测 PATH 状态
  → 在现有状态展示或操作结果中提示
  → 优先保证 CC-Box 自己通过当前 Claude 目标路径或 binary.claude_path 找到 Claude
  → 不自动修改系统级 PATH 或 shell profile

auto_configure_path = true
  → 同步或恢复 Claude binary 后自动检测 PATH
  → 如果 claude 命令不可用，或没有命中当前 Claude 目标路径，则自动配置用户级 PATH
  → 配置后提示用户重新打开终端或刷新 shell
```

该设置必须默认关闭。用户显式开启后，才允许 CC-Box 在自己的同步、恢复、WebDAV 切换或 GitHub Release 安装流程中修改用户级 PATH 或 shell profile。

注意：官方最新版安装调用的是 Claude 官方安装脚本。该脚本本身可能会配置全局或用户级 PATH，这属于用户显式触发的官方安装行为，不受 `binary.auto_configure_path` 限制。

### 4. 官方安装和 GitHub 安装放到现有二进制页面

官方安装属于“安装 Claude Code”，不是“恢复用户备份”。

恢复流程不应默认联网执行官方安装命令。但可以在现有二进制页面中提供扩展安装能力，让用户显式选择安装来源：

```text
安装来源：
1. WebDAV 已备份版本
2. 官方最新版
3. GitHub Release 版本列表
```

三类来源的定位不同：

```text
WebDAV 已备份版本：
  用于恢复用户自己同步过的 Claude binary。
  这是快照恢复和同步恢复的首选来源。

官方最新版：
  使用官方文档推荐的安装流程。
  Windows：irm https://claude.ai/install.ps1 | iex
  macOS / Linux：curl -fsSL https://claude.ai/install.sh | bash
  安装路径、PATH 配置和更新行为遵循官方安装器。
  CC-Box 不迁移、不导入、不额外搬运官方安装结果。
  安装完成后重新检测当前 claude 路径、版本和来源，用于二进制页面展示。
  不参与快照精确恢复，除非用户后续把该版本上传到 WebDAV。

GitHub Release 版本列表：
  使用官方 `https://github.com/anthropics/claude-code/releases`。
  用于查看和选择可下载的历史版本。
  程序需要读取 release/tag 版本号，筛选当前平台 asset，下载压缩包，解压出 Claude binary，再替换当前平台的 Claude binary 目标文件。
  只能作为用户显式安装动作，不能在同步或快照恢复中静默触发。
```

如果远端没有当前平台可用的 Claude binary，应提示：

```text
当前备份无法恢复 Claude binary。
你可以：
1. 只恢复配置文件。
2. 先在另一台设备上传当前平台 Claude binary。
3. 在二进制页面安装官方最新版。
4. 在二进制页面从 GitHub Release 选择当前平台版本安装。
```

### 5. 外部来源安装必须显式触发

官方最新版和 GitHub Release 都属于外部来源安装，不能在恢复流程中静默执行。

共同要求：

- 用户显式点击。
- 展示来源。
- 只展示或默认选中当前平台可用版本。
- 安装失败不影响配置文件恢复结果。

官方最新版要求：

- 执行官方文档推荐的安装脚本。
- 安装路径、PATH 配置和更新行为遵循官方安装器。
- 官方安装器自身的 PATH 修改不受 `auto_configure_path` 限制。
- 安装完成后重新检测当前 `claude` 路径、版本和来源。

GitHub Release 要求：

- 校验平台。
- 校验版本。
- 优先使用 release asset 对应的 `.sha256` 校验文件。
- 解压后定位当前平台的 `claude` / `claude.exe`。
- 替换当前平台的 Claude binary 目标文件。
- 如果 `auto_configure_path` 开启，安装后由 CC-Box 检查并配置 PATH。

### 6. 只管理 Claude binary

当前项目范围只管理 Claude binary。

即使二进制页面提供官方最新版、GitHub Release、WebDAV 已备份版本三类安装来源，也只处理 Claude binary。

恢复流程不应恢复、展示或安装：

- `uv`
- `uvx`
- `uvw`
- `codex`
- `gemini`
- 其他无关工具

远端索引中即使历史上存在其他字段，新流程也只处理 `claude`。

### 7. 二进制同步由设置开关控制

建议在设置中提供一个明确开关：

```text
同步时包含 Claude binary：开启 / 关闭
```

候选配置名：

```toml
[binary]
sync_enabled = false
auto_configure_path = false
```

含义：

```text
sync_enabled
  普通 push / pull / sync 是否纳入 Claude binary。

auto_configure_path
  CC-Box 自己在同步、恢复、WebDAV 切换或 GitHub Release 安装 Claude binary 后，是否自动检查并配置系统 PATH。
  默认 false。
```

当前代码已有 `binary.auto_upload`，但它的语义更像“是否自动上传”。如果继续复用该字段，需要把用户可见文案和内部语义改清楚：它不只是上传开关，而是“普通同步是否纳入 Claude binary 状态”的开关。

开关语义：

```text
关闭：
  普通 push / pull / sync 只处理配置文件。
  二进制页面仍允许手动上传、下载、切换。
  快照详情仍可展示历史 binary 信息。

开启：
  push 前检查当前 Claude binary 是否已上传。
  如果云端没有当前平台当前版本，则自动上传。
  push 创建的快照记录当前平台 Claude 版本。
  pull 时检查远端快照要求的当前平台 Claude 版本。
  如果本地版本不同，则自动下载并切换到快照版本。
```

注意：恢复指定快照是显式状态恢复，不新增单独恢复页；现有历史快照恢复动作应默认按快照恢复 Claude binary。如果后续需要跳过 binary，可在现有历史快照操作中增加轻量选项，而不是新增完整向导。

## 当前项目基础

现有代码中已经有可复用能力。

### 快照中的 binary 记录

位置：`core/snapshot/snapshot.go`

```go
type Snapshot struct {
    ID        string                       `json:"id"`
    Parent    string                       `json:"parent,omitempty"`
    Timestamp time.Time                    `json:"timestamp"`
    Device    string                       `json:"device"`
    Message   string                       `json:"message"`
    Files     map[string]FileEntry         `json:"files"`
    Binary    map[string]map[string]string `json:"binary,omitempty"`
}
```

当前 GUI 记录的格式类似：

```json
{
  "windows-amd64": {
    "claude": "1.0.93"
  }
}
```

### 远端二进制索引

位置：`core/binary/index.go`

远端路径：

```text
binaries/index.json
```

核心结构：

```go
type Index struct {
    Platforms map[string]PlatformBins `json:"platforms"`
}

type PlatformBins struct {
    Claude *BinaryInfo `json:"claude,omitempty"`
}

type BinaryInfo struct {
    Current  string             `json:"current"`
    Versions map[string]Version `json:"versions"`
}

type Version struct {
    Hash       string    `json:"hash"`
    Size       int64     `json:"size"`
    Refs       int       `json:"refs"`
    Uploaded   time.Time `json:"uploaded"`
    UploadedBy string    `json:"uploaded_by"`
    Encrypted  bool      `json:"encrypted"`
    Chunked    bool      `json:"chunked"`
}
```

### 二进制下载能力

位置：`core/binary/download.go`

已有能力：

- 按当前平台读取 `binaries/index.json`。
- 支持整体下载。
- 支持分块下载。
- 支持加密 / 非加密。
- 支持 hash 校验。
- 支持原子写入。
- 自动创建目标目录。

### 默认路径遵循官方安装结果

位置：

- `core/binary/index.go`
- `core/binary/resolve.go`
- `core/config/config.go`

当前项目使用的 Claude binary 路径是围绕官方安装后产生的路径设计的，目的是避免 CC-Box 与官方 Claude 安装布局不一致。因此这里不引入额外的路径迁移或导入语义。

默认 binary 目录：

```text
~/.local/bin/
```

默认版本归档目录：

```text
~/.local/share/claude/versions/
```

## 普通同步行为设计

普通同步指 `push`、`pull`、`sync`、GUI 的 QuickPush / QuickPull / QuickSync，以及文件页批量同步。

### 开关关闭

```text
binary.sync_enabled = false
```

行为：

```text
push：
  只上传配置文件 object 和配置快照。
  不自动上传 Claude binary。
  不因为本地 Claude 版本变化而推进远端 HEAD。

pull：
  只拉取配置文件变更。
  不自动下载或切换 Claude binary。

sync：
  等同于配置文件 pull + push。
```

这适合用户只想同步 Claude 配置，不想让工具改动本地 Claude 可执行文件的情况。

### 开关开启

```text
binary.sync_enabled = true
```

push 行为：

```text
1. 检测当前 Claude binary。
2. 识别当前平台和版本。
3. 检查 binaries/index.json 是否已有该平台该版本。
4. 如果没有，自动上传当前 Claude binary。
5. 创建快照时写入 snapshot.Binary[currentPlatform]["claude"]。
6. 更新远端 HEAD。
```

pull 行为：

```text
1. 读取远端 HEAD 和远端快照。
2. 应用配置文件变更前，检查 remoteSnap.Binary[currentPlatform]["claude"]。
3. 如果远端快照要求的版本与本地不同，检查云端 payload 是否存在。
4. 下载并切换到该版本。
5. 配置文件和 binary 都成功后，再更新本地 HEAD。
```

如果快照记录了版本，但云端没有对应 payload，普通同步应明确失败或进入“配置已可拉取但 binary 缺失”的待处理状态，不能静默标记为完整同步成功。

### 快照恢复不等同于普通同步

恢复指定快照是用户显式选择“回到某个历史状态”。因此：

```text
默认恢复：配置文件 + 当前平台 Claude binary 精确版本
入口复用：沿用现有历史快照恢复能力，不新增恢复页面
失败处理：如果 binary 不可恢复，应在现有恢复动作中明确报错或提示
```

如果后续确实需要“只恢复配置、不恢复 binary”，应作为现有历史快照操作的轻量选项，而不是引入新的恢复向导。

## 恢复流程总览

推荐整体流程：

```text
1. Bootstrap
   → 创建 ~/.cc-box/
   → 保存 WebDAV 配置
   → 获取或生成加密材料

2. 远端发现
   → 读取 HEAD
   → 下载 HEAD 对应快照
   → 读取 binaries/index.json

3. 生成恢复计划
   → 计算需要恢复的配置文件
   → 判断当前平台
   → 判断快照期望 Claude 版本
   → 判断云端是否有该版本
   → 判断本机 Claude 状态
   → 判断目标路径和 PATH 状态

4. 按入口语义确定恢复范围
   → 普通同步：遵守 binary.sync_enabled
   → 快照恢复：默认恢复配置 + 快照记录的当前平台 Claude binary
   → binary 页面 / CLI binary 命令：按用户显式选择的来源和版本安装

5. 执行恢复
   → 下载并校验配置对象
   → 下载并校验 Claude binary
   → 备份本地已有文件
   → 写入配置文件
   → 写入 Claude binary
   → 清理 Claude 检测缓存
   → 检测 claude --version
   → 更新本地 HEAD

6. 展示结果
   → 配置文件恢复状态
   → Claude binary 恢复状态
   → PATH 激活状态
   → 后续操作建议
```

## 恢复计划设计

恢复前应生成计划，而不是直接写入文件。

示例展示：

```text
恢复目标：
- 快照：snap_xxxxxxxx
- 文件：23 个
- 当前平台：windows-amd64
- Claude binary：
  - 快照期望版本：1.0.93
  - 云端是否存在：是
  - 本地当前版本：未安装
  - 安装目标：C:\Users\a\.local\bin\claude.exe
  - PATH 状态：未激活
```

建议内部结构：

```go
type RestorePlan struct {
    SnapshotID string
    Files      FileRestorePlan
    Binary     ClaudeRestorePlan
    Issues     []RestoreIssue
}

type ClaudeRestorePlan struct {
    Platform       string
    PlatformLabel  string
    Policy         ClaudeRestorePolicy
    TargetVersion  string
    CurrentVersion string
    TargetPath     string
    Action         ClaudeRestoreAction
    PathActive     bool
    Issues         []RestoreIssue
}

type ClaudeRestorePolicy string

const (
    ClaudeRestoreExact  ClaudeRestorePolicy = "exact"
    ClaudeRestoreLatest ClaudeRestorePolicy = "latest"
    ClaudeRestoreSkip   ClaudeRestorePolicy = "skip"
)

type ClaudeRestoreAction string

const (
    ClaudeActionSkipAlreadyInstalled ClaudeRestoreAction = "skip_already_installed"
    ClaudeActionDownload             ClaudeRestoreAction = "download"
    ClaudeActionNeedUserChoice        ClaudeRestoreAction = "need_user_choice"
    ClaudeActionUnavailable           ClaudeRestoreAction = "unavailable"
)
```

恢复计划状态建议包含：

```text
installed_same_version        本地已是目标版本
download_required             需要从 WebDAV 下载
version_missing_in_index      快照记录了版本，但云端索引没有
payload_missing               索引有记录，但实际文件或分块缺失
unsupported_platform          当前平台不在支持范围
no_binary_for_platform        云端没有当前平台 binary
target_not_writable           目标路径不可写
target_locked                 目标文件被占用
path_not_active               已安装但 claude 命令不会解析到该路径
skip_by_user                  用户选择只恢复配置
```

## Claude binary 版本选择规则

### 默认策略：精确恢复快照记录版本

优先使用：

```text
snapshot.Binary[currentPlatform]["claude"]
```

原因：恢复备份的语义是恢复到快照当时的环境，而不是自动安装云端最新版。

示例：

```json
{
  "windows-amd64": {
    "claude": "1.0.93"
  }
}
```

当前系统是 Windows x64 时，目标版本就是 `1.0.93`。

### 快照没有 binary 记录

如果快照没有记录 Claude binary，默认只恢复配置文件。

可以额外提供用户选择：

```text
安装云端当前平台最高版本
```

但不能静默选择最新版。

### 快照记录版本但云端没有

不要自动降级或升级。

展示：

```text
快照需要 Claude 1.0.93，但云端没有当前平台的该版本。
```

用户可选：

```text
1. 只恢复配置。
2. 安装云端已有的其他版本。
3. 先在另一台设备上传该平台 binary。
4. 手动使用官方安装方式安装。
```

### 云端有多个版本

选择顺序：

```text
1. 快照精确版本。
2. 用户显式选择的 WebDAV 已备份版本。
3. 用户显式选择“最新版”时，使用当前平台最高版本或最近上传版本。
```

不要在没有用户确认时从 `Current` 或最高版本自动替代快照版本。

### 外部来源版本

官方最新版和 GitHub Release 版本不应参与快照精确恢复的自动匹配。

它们只用于二进制页面中的显式安装：

```text
用户选择官方最新版
  → 执行官方文档推荐的安装脚本
  → 安装路径、PATH 配置和更新行为由官方安装器负责
  → CC-Box 不迁移、不导入、不额外搬运官方安装结果
  → 安装完成后重新检测当前 claude 路径、版本和来源
  → 可选择上传到 WebDAV，供后续快照恢复使用

用户选择 GitHub Release 版本
  → 从官方 GitHub Releases 获取版本列表
  → 只展示当前平台有 asset 的版本
  → 下载用户选择版本的当前平台压缩包和校验文件
  → 校验压缩包后解压并定位 Claude binary
  → 备份当前平台的 Claude binary 目标文件
  → 原子替换当前平台的 Claude binary 目标文件
  → 执行版本校验
  → 可选择上传到 WebDAV，供后续快照恢复使用
```

如果用户希望某个外部来源版本能被快照稳定恢复，应先把该版本上传到 WebDAV binary index。

## 平台规则

当前第一阶段只处理以下支持平台：

```text
windows-amd64
  展示：Windows x64
  GitHub Release asset：claude-win32-x64.zip

darwin-arm64
  展示：Mac M 系列
  GitHub Release asset：claude-darwin-arm64.tar.gz

linux-amd64
  展示：Linux x64
  GitHub Release asset：claude-linux-x64.tar.gz
```

注意：`config.Platform()` 使用 Go 风格平台标识，例如 `windows-amd64`；GitHub Release asset 使用发布产物命名，例如 `win32-x64`。实现时必须通过映射筛选 release assets，不能直接把 `config.Platform()` 拼进文件名。

官方 Release 即使存在其他平台产物，当前阶段也不展示、不安装、不作为只读信息暴露。

恢复时只恢复当前平台的 Claude binary。

例如：

- Windows 机器只恢复 `windows-amd64` 下的 `claude.exe`。
- Mac M 系列只恢复 `darwin-arm64` 下的 `claude`。
- Linux x64 只恢复 `linux-amd64` 下的 `claude`。

不要跨平台恢复 binary。

跨平台同步时，快照应理解为：

```text
全局配置文件状态
+ 各平台各自的 Claude binary 版本指针
```

例如同一个快照可以同时记录：

```json
{
  "windows-amd64": {
    "claude": "1.0.93"
  },
  "darwin-arm64": {
    "claude": "1.0.93"
  },
  "linux-amd64": {
    "claude": "1.0.92"
  }
}
```

Windows 设备恢复该快照时，只要求恢复 `windows-amd64` 的版本；Mac M 系列只要求恢复 `darwin-arm64` 的版本。某个平台缺失时，只能说明“该快照没有当前平台的 Claude binary 状态”，不能拿其他平台替代。

## 本地 Claude 状态判断

恢复前需要调用现有解析逻辑：

```go
binary.ResolveClaudeBinary()
binary.ResolveClaudeManagedPath()
```

需要区分：

```text
1. 本地没有 Claude。
2. 本地有 Claude，版本和快照一致。
3. 本地有 Claude，版本和快照不同。
4. 本地 Claude 是 shim。
5. 本地 Claude 来自只读路径。
6. 本地 Claude 来自 PATH，但不是当前配置或官方安装路径。
7. 配置中指定了 binary.claude_path。
8. 环境变量 CC_BOX_CLAUDE_PATH 指定了路径。
```

处理建议：

- 本地版本相同：跳过下载。
- 本地版本不同：备份后写入当前平台的 Claude binary 目标路径。
- 检测到 shim：不要覆盖 shim，提示用户通过官方安装或明确的二进制安装动作修复。
- 检测到只读路径：不要覆盖系统只读位置，提示用户切换到可写的官方安装路径或配置路径。
- 检测到系统全局安装：不要覆盖系统目录，除非该路径就是当前配置明确指定的可写目标。

## 写入目标路径

目标路径来自：

```go
binary.GetBinaryPath("claude")
```

默认：

```text
~/.local/bin/claude
~/.local/bin/claude.exe
```

CC-Box 围绕官方 Claude 安装路径和当前配置路径工作，不额外引入“迁移到 CC-Box 私有目录”或“导入官方安装结果”的语义。写入前必须自动创建父目录。

建议权限：

```text
~/.claude/                    0700
~/.cc-box/                    0700
~/.local/bin/                 0755
~/.local/share/claude/        0755
配置文件                       0600
binary                         0755
```

## 二进制下载和校验

下载必须基于远端 index 元数据：

```text
version
hash
size
encrypted
chunked
```

整体模式：

```text
binaries/<platform>/claude-<version>.enc
binaries/<platform>/claude-<version>.bin
```

分块模式：

```text
binaries/parts/<hash>/manifest.json
binaries/parts/<hash>/part-000.enc
binaries/parts/<hash>/part-001.enc
...
```

下载后必须：

```text
1. 解密，如果 encrypted = true。
2. 校验每个分块 hash。
3. 校验整体 hash。
4. 原子写入目标路径。
5. 设置可执行权限。
6. 清理 Claude resolution cache。
7. 执行版本检测。
```

## 本地备份和回滚

写入 Claude binary 前，如果目标路径已有文件，应先备份。

建议备份到：

```text
~/.local/share/claude/versions/<version>-<short_hash>/claude
~/.local/share/claude/versions/<version>-<short_hash>/claude.exe
```

如果无法识别原版本，可使用：

```text
~/.local/share/claude/versions/unknown-<timestamp>/claude
```

恢复失败时：

```text
1. 尽量恢复原 binary。
2. 不更新本地 HEAD。
3. 展示失败原因。
4. 保留已下载临时文件或清理临时文件，二选一并保持一致。
```

## PATH 激活检测

二进制写入完成后，检测：

```text
claude --version
```

并判断当前命令解析路径是否为目标路径。

结果分为：

```text
activated
  已安装，且当前环境执行 claude 会命中当前 Claude 目标路径。

installed_not_activated
  已安装，但 PATH 未命中当前 Claude 目标路径。

shadowed_by_other_binary
  已安装，但 PATH 中更靠前的位置存在另一个 claude。

not_installed
  未安装或安装失败。
```

如果未激活，恢复结果应展示：

```text
配置已恢复，Claude binary 已安装，但当前终端还无法直接运行 claude。
请将以下目录加入 PATH：
~/.local/bin
```

如果 `binary.auto_configure_path = false`，只提示，不自动修改 PATH。

如果 `binary.auto_configure_path = true`，则自动配置用户级 PATH，并提示用户重新打开终端或刷新 shell。

## 事务边界

### 完整环境恢复模式

如果用户选择“恢复配置 + Claude binary”，建议要求二者都成功后才更新本地 HEAD。

流程：

```text
1. 预检查配置文件目标路径。
2. 预检查 binary 目标路径。
3. 下载配置对象到临时区。
4. 下载 Claude binary 到临时区。
5. 校验所有 hash。
6. 备份本地已有配置文件和 binary。
7. 写入配置文件。
8. 写入 Claude binary。
9. 检测 Claude 版本。
10. 缓存快照。
11. 更新本地 HEAD。
```

如果 binary 失败，不更新 HEAD。

否则会出现：

```text
本地 HEAD 显示已恢复到远端快照，但实际 Claude binary 没恢复。
```

这会造成状态漂移。

### 只恢复配置模式

如果用户明确选择只恢复配置，可以跳过 binary 并更新 HEAD。

但恢复结果必须记录并展示：

```text
Claude binary 未恢复。
当前系统可能无法直接运行 claude。
```

这应作为环境待处理问题，而不是同步冲突。

## GUI 设计建议

不新增独立恢复页面，基于现有功能入口完善：

```text
设置页：
  增加“同步时包含 Claude binary”开关。

现有同步入口：
  QuickPush / QuickPull / QuickSync 根据开关决定是否处理 binary。

现有文件页批量同步：
  BulkPush / BulkPull 根据同一开关决定是否处理 binary。

现有历史快照恢复：
  RevertToSnapshot 默认恢复配置文件 + 快照记录的当前平台 Claude binary。

现有 binary 页面：
  继续负责手动上传、下载、切换、删除 WebDAV 版本。
  扩展提供官方最新版安装。
  扩展提供 GitHub Release 版本检查和选择安装。
```

这样不引入新的恢复向导，产品语义也更一致：

```text
同步 = 让本地和远端状态一致。

开关关闭时：
  状态 = 全局配置文件。

开关开启时：
  状态 = 全局配置文件 + 当前平台 Claude binary 版本。

恢复快照时：
  状态 = 该快照当时的全局配置文件 + 当前平台 Claude binary 版本。
```

### 命令可用性处理

恢复或同步 binary 后，应保证 CC-Box 自己能找到当前 Claude 目标路径：

```text
1. 写入或使用当前 Claude 目标路径。
2. 清理 Claude resolution cache。
3. 检测恢复后的 Claude 版本。
4. 如有必要，在 CC-Box 配置中记录 `binary.claude_path`。
```

系统命令 `claude` 是否可用由 `binary.auto_configure_path` 控制：

```text
auto_configure_path = false
  不自动修改 Windows 用户 PATH。
  不自动修改 .bashrc / .zshrc / PowerShell profile。
  如果当前终端的 claude 命令未命中当前 Claude 目标路径，只提示。

auto_configure_path = true
  同步或恢复 Claude binary 后自动配置用户级 PATH。
  目标是让新终端中直接执行 claude 命中当前 Claude 目标路径。
```

自动配置必须是幂等的：

```text
Windows：
  只修改当前用户 PATH，不修改系统 PATH。
  不要求管理员权限。

macOS / Linux：
  只修改当前用户 shell 配置。
  使用 CC-Box 管理的标记块，避免重复追加。
```

如果自动配置失败，配置文件和 binary 恢复不应回滚，但结果中必须提示命令行可用性未完成。

### 二进制页面的跨平台规则

二进制页面可以展示多来源版本，但安装目标始终是当前平台，并且不同平台必须匹配不同 asset：

```text
Windows x64：
  只允许安装 windows-amd64 的 claude.exe。
  只匹配 Windows release asset。

Mac M 系列：
  只允许安装 darwin-arm64 的 claude。
  只匹配 macOS arm64 / Apple Silicon release asset。

Linux x64：
  只允许安装 linux-amd64 的 claude。
  只匹配 Linux x64 release asset。
```

WebDAV 和 GitHub Release 都只展示当前平台可用版本；官方最新版入口只在当前平台受支持时展示。其他平台版本不展示、不安装、不作为只读信息暴露。

平台匹配失败时：

```text
不展示安装按钮。
不尝试下载其他平台包。
不允许用户手动强装不匹配平台版本。
```

官方最新版入口和 GitHub Release 版本列表也必须按当前平台过滤：

```text
当前平台受支持：
  展示官方安装入口。
  GitHub Release 只展示当前平台有 asset 的版本。

当前平台不受支持：
  展示不可用原因，不展示安装按钮。
```

如果 GitHub 无法连接：

```text
GitHub 来源显示为不可用。
展示错误原因，例如网络不可达、API 限流、解析失败。
不影响 WebDAV 已备份版本的展示和安装。
不影响本地版本切换。
不影响配置同步。
提供重试入口，但不阻塞整个二进制页面。
```

如果 GitHub 可连接但某个 release 缺少当前平台 asset：

```text
该 release 不展示为可安装版本。
不把其他平台 asset 展示给当前平台用户。
```

安装完成后的统一动作按来源区分：

```text
官方最新版：
  1. 执行官方安装脚本。
  2. 安装路径、PATH 配置和更新行为由官方安装器负责。
  3. 清理 Claude resolution cache。
  4. 重新检测当前 claude 路径、版本和来源。
  5. 刷新二进制主页，展示当前版本、平台、来源、路径和命令状态。
  6. 提供“上传到 WebDAV”动作，让该版本可参与后续快照恢复。

GitHub Release / WebDAV：
  1. 写入或替换当前平台的 Claude binary 目标文件。
  2. 校验版本和 hash。
  3. 记录安装来源。
  4. 清理 Claude resolution cache。
  5. 如果 auto_configure_path 开启，由 CC-Box 检查并配置用户级 PATH。
  6. 刷新二进制主页，展示当前版本、平台、来源、路径和命令状态。
  7. 提供“上传到 WebDAV”动作，让该版本可参与后续快照恢复。
```

GitHub Release 安装的具体流程：

```text
1. 请求官方 anthropics/claude-code releases。
2. 解析 release tag，例如 v2.1.150。
3. 按当前平台映射筛选 asset，不能直接拼接 config.Platform()。
4. 下载对应压缩包和 .sha256 校验文件到临时目录。
5. 校验压缩包。
6. 解压压缩包。
7. 在解压结果中定位 claude / claude.exe。
8. 使用 claude --version 校验版本与所选 release 匹配。
9. 备份当前平台的 Claude binary 目标文件。
10. 原子替换到 binary.GetBinaryPath("claude")。
11. 记录安装来源和版本信息。
12. 清理临时目录和 Claude resolution cache。
```

安装或替换完成后，二进制主页必须能看到当前版本信息：

```text
当前 Claude 版本：2.1.150
当前平台：Windows x64
安装来源：GitHub Release / WebDAV / 官方安装 / 本地已存在
安装路径：C:\Users\...\.local\bin\claude.exe
命令状态：已激活 / 未激活 / 被其他路径遮蔽
```

替换失败时：

```text
Windows 下如果 claude.exe 正在运行，替换可能失败。
此时不强杀进程，只提示用户关闭相关进程后重试。
失败时保留原 binary，不写入半成品。
```

## CLI 设计建议

CLI 也必须遵守同一套产品语义，不能只在 GUI 中实现。

共享配置：

```toml
[binary]
sync_enabled = false
auto_configure_path = false
```

### 普通同步命令

CLI 的普通同步命令包括：

```bash
cc-box push
cc-box pull
cc-box sync
```

它们遵守 `binary.sync_enabled`：

```text
binary.sync_enabled = false
  push / pull / sync 只处理配置文件。
  不自动上传、下载或切换 Claude binary。

binary.sync_enabled = true
  push / pull / sync 处理配置文件 + 当前平台 Claude binary 状态。
```

`cc-box push` 在开关开启时：

```text
1. 扫描配置文件变更。
2. 检测当前平台 Claude binary。
3. 识别当前 Claude 版本。
4. 检查 WebDAV binary index 是否已有当前平台该版本。
5. 如果没有，先上传当前 Claude binary。
6. 创建快照时写入 snapshot.Binary[currentPlatform]["claude"]。
7. 配置 object、binary payload 和快照都成功后，才更新远端 HEAD。
```

`cc-box pull` 在开关开启时：

```text
1. 读取远端 HEAD 和远端快照。
2. 检查 remoteSnap.Binary[currentPlatform]["claude"]。
3. 如果远端快照记录了当前平台版本，确认 WebDAV binary index 和 payload 可用。
4. 应用配置文件变更。
5. 下载并切换 Claude binary 到快照记录版本。
6. 如果 auto_configure_path 开启，配置用户级 PATH。
7. 配置文件和 binary 都成功后，才更新本地 HEAD。
```

`cc-box sync`：

```text
等同于遵守同一开关的 pull + push。
```

### 快照恢复命令

CLI 的快照恢复也应默认追求“当时状态一致”。

候选命令形式：

```bash
cc-box restore --snapshot latest
cc-box restore --snapshot snap_xxxxxxxx
cc-box restore --snapshot snap_xxxxxxxx --binary exact
cc-box restore --snapshot snap_xxxxxxxx --binary skip
```

默认语义：

```text
--binary exact
  默认值。恢复快照记录的当前平台 Claude binary 精确版本。

--binary skip
  只恢复配置文件，不恢复 Claude binary。
```

不建议让 CLI 快照恢复默认安装官方最新版或 GitHub Release 版本；外部来源安装应走显式 binary 子命令。

### CLI 二进制安装命令

现有 `cc-box binary` 命令应扩展为和 GUI binary 页面相同的来源模型：

```bash
cc-box binary list
cc-box binary push
cc-box binary pull <version>
cc-box binary switch <version>
cc-box binary install --source github --version 2.1.150
cc-box binary install --source official --latest
```

来源语义：

```text
WebDAV：
  用于同步和快照恢复，可复现。

GitHub Release：
  使用官方 https://github.com/anthropics/claude-code/releases。
  只筛选当前平台 asset。
  下载压缩包和校验文件、解压、校验、替换当前平台 Claude binary 目标文件。

官方最新版：
  显式执行官方文档推荐安装流程。
  Windows：irm https://claude.ai/install.ps1 | iex
  macOS / Linux：curl -fsSL https://claude.ai/install.sh | bash
  安装路径、PATH 配置和更新行为遵循官方安装器。
  CC-Box 不迁移、不导入、不额外搬运官方安装结果。
  不参与快照精确恢复，除非安装后上传到 WebDAV。
```

CLI 安装或切换完成后，也要输出：

```text
当前 Claude 版本
当前平台
安装来源
安装路径
命令状态
```

如果 `binary.auto_configure_path = true`，CLI 在恢复、WebDAV 切换或 GitHub Release 安装 binary 后也应自动配置用户级 PATH。官方安装脚本自身的 PATH 行为不受该开关限制。

## CLI 现状问题与本设计的不对应关系

以下是 CLI 当前实现与目标设计的差距，实施时需要逐项补齐：

1. **CLI 创建的快照没有记录 Claude binary 版本。**
   - `cli/internal/cli/push.go` 创建 `snapshot.CreateSnapshot(...)` 后没有设置 `newSnap.Binary`。
   - `cli/internal/cli/init.go` 创建初始快照时也没有写入 binary 信息。
   - 结果是 CLI 产生的历史快照无法表达“当时使用的 Claude 版本”。

2. **CLI `pull` 只拉取配置文件，不应用 `Snapshot.Binary`。**
   - `cli/internal/cli/pull.go` 的首次拉取、三方合并、降级拉取都围绕 `Files`。
   - `binary.sync_enabled` 开启后，CLI pull 需要补上 binary 计划和应用逻辑。

3. **CLI `restoreFromSnapshot` 只恢复配置文件。**
   - `cli/internal/cli/maintenance.go` 只计算 `toRestore` / `toDelete`。
   - 它不会根据目标快照的 `Binary` 恢复 Claude 版本。
   - 它创建的新恢复快照也没有继承或重新记录 binary 状态。

4. **CLI `revert` 只按文件快照回滚。**
   - `cli/internal/cli/revert.go` 创建的新快照没有设置 `Binary`。
   - 与“快照代表当时完整状态”的设计不一致。

5. **CLI binary 命令和配置同步命令是两套独立流程。**
   - `cc-box binary push/pull/switch` 可以手动管理版本。
   - 但 `cc-box push/pull/sync/restore` 不会自动调用这些能力。
   - 后续需要把二进制计划逻辑下沉到 core，供 CLI/GUI 共用。

6. **`binary.auto_upload` 字段目前没有形成完整同步语义。**
   - 配置中存在该字段，但现有搜索结果显示主要上传入口仍是手动 `UploadCurrentBinary` / `binary push`。
   - 如果保留该字段，需要明确它是否升级为 `binary.sync_enabled`，还是新增单独开关。

7. **上传 binary 不等于快照可完整恢复。**
   - `core/binary.Upload` 会把版本加入 `binaries/index.json`。
   - 但快照是否记录该版本，取决于创建快照时是否写入 `Snapshot.Binary`。
   - CLI 当前缺少这个连接点。

8. **`BinaryInfo.Current` 不能作为快照恢复依据。**
   - 快照恢复应使用 `snapshot.Binary[currentPlatform]["claude"]`。
   - `binaries/index.json` 的 `Current` 只能作为 binary 页面或“安装最新版/当前云端版本”的参考。

9. **CLI 与 GUI 的历史快照语义会出现分叉。**
   - GUI 新快照通常会记录当前 Claude 版本。
   - CLI 新快照当前不会记录。
   - 后续需要统一，否则同一个历史列表里有些快照可恢复完整状态，有些只能恢复配置。

## 建议新增核心能力

为避免 CLI 和 GUI 行为分叉，建议把二进制恢复计划和执行放到 `core/binary` 或新的共享 core 包中。

候选文件：

```text
core/binary/restore.go
```

建议接口：

```go
type ClaudeRestorePolicy string

const (
    ClaudeRestoreExact  ClaudeRestorePolicy = "exact"
    ClaudeRestoreLatest ClaudeRestorePolicy = "latest"
    ClaudeRestoreSkip   ClaudeRestorePolicy = "skip"
)

type ClaudeRestorePlan struct {
    Platform        string
    PlatformLabel   string
    TargetVersion   string
    CurrentVersion  string
    TargetPath      string
    Action          string
    PathActive      bool
    Issues          []RestoreIssue
}

func PlanClaudeRestore(
    client *webdav.Client,
    key []byte,
    snap *snapshot.Snapshot,
    policy ClaudeRestorePolicy,
) (*ClaudeRestorePlan, error)

func ApplyClaudeRestore(
    client *webdav.Client,
    key []byte,
    plan *ClaudeRestorePlan,
    progress DownloadProgress,
) error
```

需要注意：如果放在 `core/binary` 中直接依赖 `core/snapshot`，要检查是否会造成包依赖方向不合理。若不合适，可以新建更上层共享包，例如：

```text
core/restore/
```

由 `core/restore` 组合 `snapshot`、`binary`、`object`、`webdav`。

## 与现有流程的接入点

### GUI

可能接入位置：

```text
gui/files.go
  BulkSync / doBulkPull / applyRemoteSnapshot

gui/pages.go
  RevertToSnapshot
  revertBinary
  binary 页面安装来源扩展

gui/dashboard.go
  QuickPush / QuickPull / QuickSync
```

不新增独立恢复向导，直接完善现有入口。

### CLI

可能接入位置：

```text
cli/internal/cli/push.go
  sync_enabled 开启时上传当前平台 Claude binary，并写入 Snapshot.Binary。

cli/internal/cli/pull.go
  sync_enabled 开启时应用远端快照记录的当前平台 Claude binary。

cli/internal/cli/sync.go
  复用 pull / push 的同一开关语义。

cli/internal/cli/maintenance.go
  restoreFromSnapshot 默认恢复快照记录的当前平台 Claude binary。

cli/internal/cli/revert.go
  revert 后创建的新快照应记录恢复后的当前平台 Claude binary 状态。

cli/internal/cli/binary.go
  扩展 WebDAV / GitHub Release / 官方最新版三类安装来源。
```

CLI 不应另起一套语义，必须复用 core 中的 binary 规划、下载、替换、PATH 配置能力。

## 风险和边界

### 1. 快照 HEAD 与 binary 恢复状态不一致

如果配置恢复成功但 binary 失败，却更新 HEAD，会导致状态漂移。

建议：完整恢复模式下 binary 失败则不更新 HEAD。

### 2. Windows 目标文件被占用

Windows 下如果 `claude.exe` 正在运行，可能无法替换。

建议：

- 写入前检测是否可重命名。
- 失败时提示关闭相关进程后重试。
- 不要强杀进程。

### 3. PATH 被其他 Claude shadow

用户 PATH 中可能存在另一个更靠前的 `claude`。

恢复结果要区分：

```text
已安装到当前 Claude 目标路径
但当前命令解析到另一个 claude
```

### 4. 远端 index 有记录但 payload 缺失

例如 `binaries/index.json` 有版本，但实际文件或分块不存在。

这应视为远端备份不完整。

处理：

- 不更新 HEAD。
- 展示缺失路径或简化错误。
- 建议重新上传该版本。

### 5. 跨平台恢复

不要在 Windows 上尝试恢复 Mac 或 Linux binary。

如果快照只有其他平台 binary：

```text
配置可恢复，但当前平台没有可用 Claude binary 备份。
```

### 6. 历史索引字段

`PlatformBins` 里当前还有 `UV`、`UVX`、`UVW`、`Custom` 字段。

恢复流程可以兼容读取，但新 UI、新命令、新恢复逻辑只处理：

```text
claude
```

## 推荐默认产品行为

### 普通同步

普通同步由设置开关决定是否纳入 Claude binary：

```text
同步时包含 Claude binary = 关闭
  → push / pull / sync 只处理配置文件。
  → binary 页面仍可手动上传、下载、切换。

同步时包含 Claude binary = 开启
  → push 自动检查并上传当前平台当前 Claude 版本。
  → push 创建快照时记录当前平台 Claude 版本。
  → pull 自动检查远端快照记录的当前平台 Claude 版本。
  → pull 自动下载并切换到快照要求的版本。
  → 如果 auto_configure_path 开启，恢复或切换后自动配置用户级 PATH。
```

### 快照恢复

快照恢复默认追求“当时状态一致”：

```text
恢复某个快照默认做：
1. 恢复该快照中的全局配置文件。
2. 如果快照记录了当前平台 Claude 版本，默认恢复该版本。
3. 自动创建缺失的 .claude / .local / 版本目录。
4. 校验配置 object 和 binary hash。
5. 检测恢复后的 Claude 版本。
6. 检测 PATH 是否命中当前 Claude 目标路径。
7. 如果 PATH 未激活，只提示，不默认修改。
```

不新增恢复确认页；如果后续需要取消 binary 恢复，应作为现有历史快照操作的轻量选项处理。

### 新设备加入 / 空系统恢复

新设备加入已有同步组时：

```text
1. 先恢复最新快照中的配置文件。
2. 如果二进制同步开关开启，继续恢复该快照记录的当前平台 Claude binary。
3. 如果二进制同步开关关闭，展示该快照记录的 Claude 版本，并提示可稍后在 binary 页面恢复。
```

### 明确不做

```text
不在同步或快照恢复中默认执行官方安装。
不在同步或快照恢复中默认从 GitHub 下载 binary。
官方安装和 GitHub 下载只能在现有二进制页面由用户显式触发。
不恢复或安装其他平台 binary。
不恢复 uv/uvx/codex/gemini。
不在 auto_configure_path 关闭时由 CC-Box 自己静默修改 PATH；官方安装脚本的 PATH 行为按官方安装器执行。
```

## 后续讨论问题

已经倾向确定的方向：

1. 配置文件和 Claude binary 可以选择性恢复。
2. 普通同步是否包含 Claude binary，由设置开关控制。
3. 恢复指定快照默认恢复该快照记录的当前平台 Claude binary。
4. 快照语义应尽量代表“当时的全局配置 + 当前平台 binary 状态”。
5. 跨平台只恢复当前平台 binary，不用其他平台替代。
6. CLI 必须补齐同一套同步、恢复、安装来源和 PATH 配置语义。
7. 项目使用的 Claude 路径应与官方安装后产生的路径保持一致，避免 CC-Box 与官方布局分叉。
8. 官方最新版安装调用官方安装流程，安装路径、PATH 配置和更新行为遵循官方安装器。
9. 安装来源用于二进制页面展示当前本机 Claude 来自哪里，不作为快照恢复依据。

仍需进一步确认的问题：

1. 新设置是新增 `binary.sync_enabled`，还是复用并重定义现有 `binary.auto_upload`？
2. 普通 pull 在二进制缺失时，是完全失败并不更新 HEAD，还是允许“只恢复配置但标记 binary 缺失”？
3. 新设备加入时，如果二进制同步开关关闭，是否只提示可在现有 binary 页面恢复？
4. 恢复 binary 后是否写入 `binary.claude_path`，以保证 CC-Box 自己优先使用当前配置路径？
5. Windows 自动配置 PATH 时，采用用户级 PATH 写入还是仅依赖官方安装路径？
6. macOS / Linux 自动配置 PATH 时，优先写 `.zshrc`、`.bashrc` 还是 `.profile`？
7. GitHub Release 来源是项目自维护版本索引，还是直接读取外部 Release 列表？
8. GitHub API 限流或无法连接时，是否需要缓存上一次成功获取的版本列表？
9. 快照没有当前平台 binary 记录时，是否允许用户在现有 binary 页面选择云端当前平台最新版？
10. 是否需要新增 `core/restore` 包，而不是把恢复编排放进 `core/binary`？

## 一句话结论

配置和 Claude binary 可分开恢复；普通同步由开关决定是否同步 binary；恢复快照默认按快照记录恢复当前平台 Claude 版本，以保证“当时状态”一致。
