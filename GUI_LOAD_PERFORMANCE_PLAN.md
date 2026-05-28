# GUI 页面加载性能全量统一优化方案

## 结论

本方案不是只优化 Settings，也不是只做一轮试点后把其它已知卡顿点留下。目标是对 GUI 已确认的页面加载路径做一次统一治理：所有会阻塞首屏、重复触发慢操作、隐藏页面后台重刷的问题都纳入本次优化范围。

“分批推进”只表示实现顺序和验证节奏，不表示缩小最终范围。最终交付前，应完成 Dashboard、Settings、Binaries、Files、History、Projects、Onboarding 相关加载路径的统一优化，并通过完整 GUI 验证。

## 背景

概览页已经按 `DASHBOARD_FAST_LOAD_DESIGN.md` 拆成“本地首屏 + 后台远程检查”，启动后不再被 WebDAV 远程检查整体阻塞。继续审查其它页面后，可以确认 GUI 卡顿不是某一个页面的孤立问题，而是多个页面存在同类结构：

- 首屏 `loading` 同时等待本地轻量数据、本地重扫描、远程 I/O、外部命令。
- 多个页面重复执行同一慢操作，例如 `claude --version`、远程 `HEAD`、远程 `binaries/index.json`。
- 页面首次访问后虽然被保留，但隐藏页面的事件监听仍可能触发重刷新。
- 部分用户主动操作同步执行远程请求或下载，缺少后台进度和局部刷新。

因此，本次优化应按“统一加载模型 + 页面逐一接入 + 横向缓存/事件收敛”的方式解决。

## 统一目标

1. 所有页面首屏优先显示本地可用信息。
2. 本地重扫描、远程 WebDAV、外部命令检测不得阻塞整页首屏。
3. 同一轮 GUI 会话中避免重复执行同一慢操作。
4. 隐藏页面不得因无关事件执行重刷新。
5. 用户主动触发的慢操作必须有明确状态、进度或局部加载反馈。
6. 不改变同步协议、远程数据格式、加密格式、WebDAV 路径或 CLI 可读写兼容性。

## 支持平台范围

本轮 GUI 加载性能优化按当前产品支持矩阵验证，不扩大到未计划支持的架构。

| 平台 | 内部平台 key | 支持范围 |
| --- | --- | --- |
| Windows | `windows-amd64` | 支持 |
| macOS Apple Silicon | `darwin-arm64` | 支持，用户文案使用“Mac M 系列” |
| Linux x86_64 | `linux-amd64` | 支持，用户文案使用“Linux” |

明确不纳入本轮实现、测试和发布范围：

- `darwin-amd64`，即 Intel Mac。
- `linux-arm64` 或其它 Linux ARM 架构。
- `windows-arm64` 或其它 Windows ARM 架构。

如果程序运行在未支持平台，可以保留内部 `GOOS-GOARCH` 字符串作为降级展示，但不为这些平台新增 binary 管理、发布验证或桌面行为承诺。

## 统一加载模型

### 数据分层

| 层级 | 类型 | 典型来源 | UI 策略 |
| --- | --- | --- | --- |
| L1 本地轻量 | 读配置、读本地 HEAD、读小状态文件 | `config.Load()`、本地 `HEAD`、本地快照索引 | 可进入首屏关键路径 |
| L2 本地重扫描 | 遍历 `.claude`、读取文件 hash、扫描项目目录 | `snapshot.Scanner.Scan()`、项目发现、目录枚举 | 首屏后后台执行，局部加载 |
| L3 远程 I/O | WebDAV `HEAD`、远程快照、二进制 index、devices | `webdav.Client.Get()`、`binary.LoadIndex()`、`PROPFIND` | 后台刷新，不阻塞首屏 |
| L4 外部命令 | `claude --version`、`git remote -v` | `binary.DetectVersion()`、项目 Git remote 检测 | 默认读缓存；手动重新检测或缓存过期才执行 |

### 页面状态要求

每个页面至少区分以下状态，而不是只有一个整页 `loading`：

- `initialLoading`：只用于首屏必须数据。
- `localScanning`：本地重扫描进行中。
- `remoteChecking`：远程状态刷新中。
- `actionLoading`：用户主动操作进行中。
- `dirty`：页面隐藏时相关数据已变更，等待激活后刷新。

### 刷新要求

- 页面首次进入时只加载首屏必要数据。
- 页面被隐藏时，不做无关重刷新。
- 操作完成后只刷新受影响区域。
- 页面重新激活时，如果有 `dirty` 标记，再执行对应刷新。

## 修改范围

### 一、前端范围

必须纳入修改或审查的手写前端文件：

- `gui/frontend/src/App.svelte`
- `gui/frontend/src/pages/Dashboard.svelte`
- `gui/frontend/src/pages/Settings.svelte`
- `gui/frontend/src/pages/Binaries.svelte`
- `gui/frontend/src/pages/Files.svelte`
- `gui/frontend/src/pages/History.svelte`
- `gui/frontend/src/pages/Projects.svelte`
- `gui/frontend/src/pages/Onboarding.svelte`
- `gui/frontend/src/lib/components/Sidebar.svelte`，仅在新增或调整全局同步状态文案时修改

前端修改内容：

1. App 级页面激活状态和 dirty 标记。
   - 当前 `App.svelte` 已实现首次导航懒挂载和挂载后保留。
   - 需要补充当前页面激活状态传递，或由 App 统一管理操作完成后的刷新影响范围。
2. 页面级首屏拆分。
   - 每个页面首屏只调用 fast/local 接口。
   - 远程检查、扫描、外部命令改为后台任务或局部刷新。
3. 事件监听收敛。
   - 每个页面只响应与自身数据相关的 `op:complete`。
   - 隐藏页面不立即执行重刷新。
4. 局部 loading。
   - 页面不能因为某个二级区域在扫描而整体空白。
   - 远程失败不得清空本地已加载数据。

不应主动手改的前端文件：

- `gui/frontend/wailsjs/**`

这些是 Wails 自动生成绑定。只有在新增、删除或改名 Go 绑定方法后，通过 Wails 生成流程更新。

### 二、GUI 后端范围

必须纳入修改或审查的 GUI Go 文件：

- `gui/dashboard.go`
- `gui/files.go`
- `gui/pages.go`
- `gui/onboarding.go`
- `gui/app.go`，仅在 App 级缓存、状态或事件分发需要时修改
- `gui/async.go`，仅在慢操作需要接入现有异步进度模型时修改
- `gui/internal/project/tracker.go`

GUI 后端修改内容：

1. 拆分 fast/local 与 remote/heavy 接口。
2. 增加 GUI App 层短 TTL 缓存。
3. 避免一次页面接口内部同时执行本地重扫描、远程 I/O 和外部命令。
4. 用户主动慢操作接入现有进度事件，而不是同步阻塞页面。
5. 保持旧接口兼容，直到前端迁移完成；迁移完成后再删除无用旧路径。

### 三、Core 范围

允许修改的 core 范围：

- `core/binary/resolve.go`
- `core/binary/platform.go`
- `core/binary/index.go`
- 必要时补充 `core/binary` 测试

Core 修改边界：

- 可以增加 cached/fast 版本检测能力。
- 可以调整 GUI 使用的二进制检测入口，避免页面首屏执行 `claude --version`。
- 不应改变 CLI 现有命令的用户可见行为，除非同步更新 CLI 并补充测试。
- 不改变二进制版本 index 的远程格式。

不纳入本次 core 修改的范围：

- `core/sync` 同步算法。
- `core/snapshot` 快照 JSON 结构和 hash 规则。
- `core/crypto` 加密格式。
- `core/webdav` 远程路径语义。

### 四、不纳入本次修改的范围

- 不做 UI 视觉重设计。
- 不改图标、安装包资源或品牌素材。
- 不重新引入开机自启动。
- 不改变 CLI 命令行为。
- 不调整同步协议、远程目录结构、快照格式、加密格式。
- 不做大规模架构重构，例如重新拆 module 或迁移所有页面状态管理框架。

## 页面详细修改方案

### 1. Dashboard

相关文件：

- `gui/frontend/src/pages/Dashboard.svelte`
- `gui/dashboard.go`
- `core/binary/resolve.go`

当前状态：

- 已拆成 `GetDashboardLocal()` 和 `RefreshDashboardRemote()`。
- 首屏已能先显示本地概览。
- 同步、推送、拉取、修复远程后会刷新远程状态。

剩余问题：

- `buildDashboardBase()` 仍可能执行 Claude binary 检测。
- `RefreshDashboardRemote()` 重新构建完整 base，可能重复读取本地快照和检测 binary。
- Dashboard 与 Settings、Binaries 共享 binary 检测慢点。

修改要求：

1. `GetDashboardLocal()` 保持首屏快路径。
   - 只保留配置状态、本地 HEAD、最近本地快照摘要、冲突计数等本地轻量数据。
   - 不主动执行 `claude --version`。
2. 新增或调整远程 delta 刷新。
   - 远程刷新只返回远程健康状态、remote HEAD、devices、remote binary、远程备份摘要。
   - 前端把 delta merge 到已有 dashboard，避免重建本地 base。
3. Dashboard 使用共享 binary cache。
   - 首屏可展示缓存版本或“待检测”。
   - 后台检测完成后局部更新 Claude binary 区域。

验收标准：

- 打开 Dashboard 时本地概览立即出现。
- 远程不可达时 Dashboard 不回到整页 loading。
- 远程刷新不重复执行本地 binary resolve。
- sync/push/pull/repair 后远程状态仍正确更新。

### 2. Settings

相关文件：

- `gui/frontend/src/pages/Settings.svelte`
- `gui/pages.go`
- `core/binary/resolve.go`

当前问题：

- `loadConfig()` 串行等待 `GetConfig()`、`GetClaudeDirectories()`、`GetClaudeExcludeFiles()`、`GetEncryptionStatus()`。
- 默认连接 tab 也会触发 Claude 目录扫描、排除项扫描、二进制检测和加密状态读取。
- 保存任意配置后重新跑完整 `loadConfig()`。

修改要求：

1. 拆分 Settings 后端接口。
   - `GetConfigFast()`：返回连接配置、路径配置、排除 patterns 等可编辑字段。
   - `GetSettingsEncryptionStatus()`：返回加密状态。
   - `GetSettingsClaudeDirectories()`：返回 Claude 目录列表。
   - `GetSettingsClaudeExcludeFiles()`：返回 Claude 排除文件状态。
   - `GetSettingsClaudeRuntime()`：返回缓存的 Claude binary 状态。
2. 前端按 tab 懒加载。
   - 连接 tab 首屏只依赖 `GetConfigFast()`。
   - 加密 tab 激活后再读加密状态。
   - 排除项 tab 激活后再扫描目录和文件。
   - 二进制相关区域只读缓存，重新检测按钮才触发外部命令。
3. 保存后局部刷新。
   - 保存 WebDAV URL/root 后刷新连接配置显示。
   - 保存路径后刷新路径和排除项相关缓存。
   - 保存排除项后刷新排除列表。
   - 不再无条件调用完整 `loadConfig()`。

验收标准：

- 打开 Settings 连接 tab 时不执行 `claude --version`。
- 打开 Settings 连接 tab 时不扫描 Claude 目录和排除文件。
- 切换到排除项 tab 后才出现目录/排除文件局部加载。
- 保存配置后不触发无关区域重扫。
- 加密文案继续使用“加密密码”，不把用户操作描述为“密钥”。

### 3. Binaries

相关文件：

- `gui/frontend/src/pages/Binaries.svelte`
- `gui/pages.go`
- `core/binary/index.go`
- `core/binary/resolve.go`

当前问题：

- 首屏串行调用 `GetBinaryPage()` 和 `GetBinaryStorage()`。
- 两个接口可能重复读取远程 `binaries/index.json`。
- `GetBinaryPage()` 会触发本地 binary resolve/version 检测。
- `RedetectClaudeBinary()` 后又完整 `loadBinary()`，可能重复检测。

修改要求：

1. 合并或拆分首屏接口，但不能重复取同一数据。
   - 推荐：`GetBinaryPageLocal()` 返回本地/缓存数据。
   - `RefreshBinaryRemote()` 后台补远程版本、storage、index。
   - 或让 `GetBinaryPage()` 一次返回页面需要的所有数据，删除首屏第二次 `GetBinaryStorage()`。
2. binary index 使用短 TTL cache。
   - Dashboard、Binaries 共享同一远程 index 缓存。
3. `RedetectClaudeBinary()` 直接返回检测后的 runtime 数据。
   - 前端只更新相关区域，不立即再次完整加载页面。
4. 远程下载/切换版本接入异步进度。
   - 大文件下载不应让页面没有反馈地同步等待。

验收标准：

- 打开 Binaries 时不重复请求 `binaries/index.json`。
- 本地二进制状态先显示，远程版本后台补齐。
- 点击重新检测后只执行一次检测，并局部更新。
- 远程版本操作有进度或明确 loading 状态。

### 4. Files

相关文件：

- `gui/frontend/src/pages/Files.svelte`
- `gui/files.go`
- `core/snapshot/scanner.go`
- `core/webdav/client.go`

当前问题：

- 首屏直接调用 `GetFileTree()`。
- 后端一次性执行本地扫描、本地快照读取、远程 HEAD、远程快照读取和状态合成。
- 远程不可达时，本地树也可能被远程请求拖慢。
- `op:complete` 刷新范围过宽。

修改要求：

1. 拆分文件树加载。
   - `GetFileTreeLocal()`：扫描本地树，并基于本地 HEAD/本地快照计算本地状态。
   - `RefreshFileRemoteStatus()`：后台获取远程 HEAD/快照，返回状态 overlay。
2. 前端先显示本地树。
   - 远程检查中显示局部状态。
   - 远程失败只显示远程异常，不清空本地树。
3. 减少重复远程请求。
   - 一次刷新中只读取一次 remote HEAD。
   - remote snapshot 按 id 缓存。
4. 事件刷新过滤。
   - 只响应会影响文件树的操作。
   - 隐藏时标记 dirty，切回 Files 后刷新。

验收标准：

- WebDAV 不可达时 Files 仍能先显示本地文件树。
- 远程状态补齐不会造成整页闪烁或清空。
- 无关操作完成不会触发隐藏 Files 页面重扫。
- push/pull/sync/恢复后文件状态正确更新。

### 5. History

相关文件：

- `gui/frontend/src/pages/History.svelte`
- `gui/pages.go`

当前问题：

- 前端先 `GetLocalSnapshotList()`，为空后 `GetSnapshotList()`。
- `GetSnapshotList()` 后端内部又先尝试本地列表，存在重复判断。
- 远程 fallback 会沿 snapshot parent 链读取多个快照。
- 详情接口应本地优先，但当前路径可能先准备远程 client。
- `op:complete` 刷新范围过宽。

修改要求：

1. 提供单一列表接口。
   - `GetSnapshotListPreferLocal(limit)`：后端内部本地优先，必要时远程 fallback。
2. 详情接口本地优先。
   - 先读本地缓存，本地没有再加载 WebDAV client。
3. 分页改为 append/cursor 思路。
   - 加载更多不应每次重拉前面全部数据。
4. 事件刷新过滤。
   - 只响应会创建、删除或恢复快照的操作。

验收标准：

- 有本地快照时 History 不依赖远程即可显示。
- 详情本地存在时不要求远程可用。
- 加载更多不会造成整页重置。
- 隐藏 History 不因无关操作重拉列表。

### 6. Projects

相关文件：

- `gui/frontend/src/pages/Projects.svelte`
- `gui/pages.go`
- `gui/internal/project/tracker.go`

当前问题：

- 首次进入调用 `GetProjectList()`。
- 项目发现会扫描 `~/.claude/projects`。
- 每个项目可能执行 `git remote -v`，缺少明确 timeout。
- 还会逐个读取 `.claude.json` 统计 MCP。

修改要求：

1. 首屏优先读取缓存/已追踪项目索引。
2. 项目发现后台执行。
3. Git remote 检测加 timeout。
4. 如果并发扫描，必须限流。
5. MCP 统计作为后台补充字段，不阻塞项目列表首屏。

验收标准：

- 项目目录较多时 Projects 不长时间整页 loading。
- Git remote 卡住时不会拖死整个页面。
- 后台扫描完成后项目列表能局部更新。

### 7. Onboarding

相关文件：

- `gui/frontend/src/pages/Onboarding.svelte`
- `gui/onboarding.go`
- `gui/pages.go`

当前状态：

- 首屏只调用 `GetAppInfo()`，不是主要卡顿来源。
- 慢点主要在用户主动测试 WebDAV、初始化、加入已有同步组时出现。

修改要求：

1. 首屏保持轻量，不新增远程检查。
2. 测试 WebDAV、初始化、加入同步组保留明确 loading/progress。
3. 同一次加入流程中可缓存 salt/head/snapshot 预览结果，避免重复远程读取。
4. 不改变初始化和加入同步组的远程数据格式。

验收标准：

- Onboarding 首屏不变慢。
- 测试连接和加入同步组有明确状态反馈。
- 重复预览同一远程配置时不重复拉取相同远程数据。

## 横向基础改造

### 1. Claude binary 检测缓存

相关文件：

- `core/binary/resolve.go`
- `core/binary/platform.go`
- `gui/pages.go`
- `gui/dashboard.go`

修改要求：

1. 增加 cached/fast 查询能力。
   - 返回 path、version、source、isShim、updatedAt、stale 等字段。
   - 常规页面读取缓存或轻量路径判断。
   - 真实 `claude --version` 只在缓存过期、首次必要检测或用户点击重新检测时执行。
2. 避免重复检测。
   - 同一时间只有一个检测任务执行，其它调用复用结果或等待同一任务。
3. shim 判断前置。
   - Windows `.cmd/.bat/.ps1` 候选先识别 shim，再决定是否执行版本命令。
   - macOS/Linux 的 symlink、shebang 脚本、文本 shim 也必须在执行版本命令前识别。
4. 支持平台限定。
   - binary 管理只需要覆盖 `windows-amd64`、`darwin-arm64`、`linux-amd64`。
   - 不为 `darwin-amd64`、`linux-arm64`、`windows-arm64` 扩展候选路径、远程版本展示或发布验证。
5. CLI 兼容。
   - 如果新增 GUI 专用 fast 方法，CLI 行为不变。
   - 如果调整 core 默认方法，必须同步检查 CLI 调用并补测试。

### 2. WebDAV / binary index / snapshot 短 TTL 缓存

相关文件：

- `gui/dashboard.go`
- `gui/files.go`
- `gui/pages.go`
- `core/binary/index.go`

修改要求：

1. 在 GUI App 层建立短 TTL cache。
   - remote HEAD：5-10 秒。
   - remote snapshot：按 snapshot id 缓存。
   - binary index：10-30 秒。
   - devices list：10-30 秒。
2. 缓存必须可失效。
   - push/pull/sync/repair 后失效 HEAD、snapshot、devices。
   - binary upload/delete/switch 后失效 binary index/storage。
   - Settings 改 WebDAV 配置后清空所有远程缓存。
3. 缓存不改变真实同步语义。
   - 用户主动同步、推送、拉取、修复远程必须使用真实最新状态。
   - 缓存只用于页面展示和后台刷新去重。

### 3. 操作影响范围和页面 dirty 机制

相关文件：

- `gui/async.go`
- `gui/frontend/src/App.svelte`
- 各页面 Svelte 文件

修改要求：

1. 补齐完成事件信息。
   - `op:complete` 的 payload 需要包含 `operation`，或由 App 维护 `opId -> operation` 映射。
   - 否则隐藏页面无法判断操作影响范围，只能继续无差别刷新。
2. 定义操作影响范围。
   - sync/push/pull/repair：Dashboard、Files、History。
   - bulk-push/bulk-pull：Dashboard、Files、History。
   - ResolveConflict/SaveMergedConflict：Dashboard、Files、History。
   - ExcludeFile：Settings exclude 区域、Files、Dashboard config/status。
   - RevertToSnapshot：Dashboard、Files、History。
   - binary upload/delete/switch/redetect：Dashboard、Binaries、Settings runtime 区域。
   - settings save：Settings、Dashboard config status，必要时清空远程缓存。
   - project scan/update：Projects。
3. 页面激活时才执行重刷新。
4. 隐藏页面收到相关影响时只设置 dirty。
5. 用户切回页面后再按 dirty 类型刷新。

## 跨平台准确性修正

这些修正是实施前必须纳入的准确性约束，用于避免方案在 Windows、Mac M 系列和 Linux x86_64 上承诺过头。

1. Claude binary cache 当前不是 fast cache。
   - 当前缓存命中后仍可能执行 `claude --version`。
   - 实施前必须扩展缓存结构，保存 `version`、`updatedAt`、`stale` 等信息，或新增 GUI 专用 cached/fast 查询接口。
   - “重新检测只执行一次”必须依赖 singleflight/互斥复用，否则不能作为验收承诺。
2. shim 判断必须前置且跨平台。
   - Windows `.cmd/.bat/.ps1` 不应在首屏路径中直接执行。
   - macOS/Linux 的 symlink、shebang、文本 shim 也应先识别，再决定是否执行版本命令。
3. Projects 路径解码需要跨平台审查。
   - 当前 Claude projects 目录名还原逻辑如果要修改，必须同步检查 CLI 对应实现，避免 GUI/CLI 行为漂移。
   - 路径组件包含 `-`、Windows 盘符、Unix 路径都要纳入测试。
4. Projects 首屏缓存不能只依赖 tracked index。
   - tracked index 只代表用户手动添加项目。
   - 如果要首屏展示自动发现项目，需要新增 discovered projects cache。
5. Files 本地首屏只能承诺不被远程阻塞。
   - 本地扫描仍可能遍历文件并计算 hash。
   - 如果要求大 `.claude` 目录也快速显示，需要另做目录骨架、上次扫描缓存或增量扫描；否则验收标准只承诺“远程不可达不阻塞本地扫描结果”。
6. WebDAV 和 binary index cache 只允许用于 GUI 展示去重。
   - 真实同步、推送、拉取、修复、冲突判断和二进制写操作必须绕过或刷新缓存。
7. 平台验证按支持矩阵执行。
   - 必测：`windows-amd64`、`darwin-arm64`、`linux-amd64`。
   - 不承诺 Intel Mac、Linux ARM、Windows ARM 的桌面验证或 binary 管理行为。

## 执行顺序

下面是实现顺序，不是最终范围裁剪。所有批次都完成后，才算本方案完成。

### 批次 1：横向基础能力

1. Claude binary cached/fast 检测。
2. GUI App 层远程短 TTL cache。
3. 操作影响范围和页面 dirty 机制。

完成条件：后续页面可以复用缓存和 dirty 机制，不需要各自重复造轮子。

### 批次 2：用户已感知且共享根因的页面

1. Settings 首屏拆分和 tab 懒加载。
2. Binaries 本地/远程拆分或数据合并去重。
3. Dashboard 使用 binary cache 和远程 delta 刷新。

完成条件：Settings、Binaries、Dashboard 不再重复执行 binary 检测和远程 index 读取。

### 批次 3：重扫描和远程 fallback 页面

1. Files 本地树与远程 overlay 拆分。
2. History 本地优先接口和详情本地优先。
3. Files/History 事件刷新过滤。

完成条件：远程不可达时，Files 和 History 仍能显示本地可用数据。

### 批次 4：项目和 Onboarding 主动操作优化

1. Projects 缓存首屏、后台项目发现、Git timeout、并发限流。
2. Onboarding 远程预览缓存和进度反馈确认。

完成条件：Projects 不因项目发现或 git remote 长时间整页 loading；Onboarding 首屏保持轻量。

### 批次 5：全量回归和清理

1. 删除前端不再使用的旧调用路径。
2. 确认 Wails 绑定重新生成且只包含必要变化。
3. 检查所有页面的 loading/error/dirty 状态一致性。
4. 完整执行自动化和桌面验证。

## 约束

### 协议和数据约束

- 不改变 WebDAV 路径。
- 不改变远程目录结构。
- 不改变 `HEAD` 语义。
- 不改变 snapshot JSON 字段和 parent 链语义。
- 不改变 object hash 规则。
- 不改变加密格式、salt、key 派生或 rekey 语义。
- 不改变 binary index 远程 JSON 格式。
- binary index 只需要保证 `windows-amd64`、`darwin-arm64`、`linux-amd64` 三类平台展示和管理正确。

### CLI/GUI 兼容约束

- GUI 页面展示缓存不得影响 CLI 读写同一 WebDAV 远程。
- 如果修改 `core/`，必须判断 CLI 是否受影响。
- 如果 core 行为变化会影响 CLI，必须同步改 CLI 并补 CLI/core 测试。
- 仅 GUI 页面展示缓存不得被同步算法当成权威状态。

### UI 行为约束

- 首屏不能为了等待远程状态而空白。
- 远程失败不能清空本地已加载数据。
- 局部区域 loading 不能阻塞整页交互，除非该操作本身必须阻塞保存/提交。
- 用户主动慢操作必须有明确反馈。
- 加密相关文案统一使用“加密密码”。

### 工程约束

- 优先修改手写源码，Wails 绑定只通过生成流程更新。
- 不搜索或修改 `node_modules/`、`dist/`、无关 `build/` 产物。
- 不引入不必要的大型状态管理库。
- 不为单次调用过度抽象。
- 不把页面优化顺手扩展成 UI 重设计。
- 不处理与加载性能无关的图标、发布、README、安装包资源变更。

### 缓存约束

- 缓存必须有明确 TTL 或失效点。
- 用户主动刷新、同步、推送、拉取、修复必须绕过或刷新相关缓存。
- 切换 WebDAV 配置、加密密码或本地路径后，相关缓存必须失效。
- 缓存只能用于展示加速，不能改变真实写入和冲突判断的权威来源。

## 测试计划

### 自动化测试

#### Go 测试

在仓库根目录执行：

```bash
go test -C "gui" ./...
```

如果修改 `core/`，补充执行对应 core 测试：

```bash
go test -C "core" ./...
```

需要新增或调整的测试：

1. Dashboard
   - 本地首屏不依赖远程。
   - 远程刷新返回 synced/pending/conflict/error。
   - 远程刷新不覆盖本地已加载数据。
2. Settings
   - `GetConfigFast()` 不触发 binary version 检测。
   - tab 详情接口只返回各自区域数据。
   - 保存配置后只刷新相关数据。
3. Binaries
   - binary index cache 命中时不重复远程读取。
   - `RedetectClaudeBinary()` 只执行一次检测并返回最新 runtime。
4. Files
   - `GetFileTreeLocal()` 在远程不可用时仍成功返回本地树。
   - `RefreshFileRemoteStatus()` 能正确合成远程 overlay。
5. History
   - 本地有快照时不访问远程。
   - 本地详情存在时不要求远程 client。
6. Projects
   - Git remote 检测 timeout 后不阻塞整个项目列表。
7. Cache
   - push/pull/sync/repair 后相关远程 cache 失效。
   - WebDAV 配置变化后远程 cache 清空。

#### 前端构建

在仓库根目录执行：

```bash
npm --prefix "gui/frontend" run build
```

#### Wails 构建

在 `gui/` 目录执行：

```bash
wails build -clean -nopackage -m -nosyncgomod
```

#### Diff 检查

在仓库根目录执行：

```bash
git diff --check
```

### 桌面手工验证

#### 平台矩阵验证

本轮跨平台验证只覆盖当前支持平台：

| 平台 | 必要验证 |
| --- | --- |
| `windows-amd64` | Windows Wails 构建、WebView2 桌面启动、Claude binary 查找、`.cmd/.bat/.ps1` shim 判断、Git remote timeout、托盘/窗口行为 |
| `darwin-arm64` | Mac M 系列 Wails 构建、`.app` 启动、Claude binary 查找、symlink/shebang shim 判断、Git remote timeout、菜单栏/窗口行为 |
| `linux-amd64` | Linux x86_64 Wails 构建、Claude binary 查找、symlink/shebang shim 判断、Git remote timeout、托盘可用性和窗口行为 |

Intel Mac、Linux ARM、Windows ARM 不纳入本轮测试矩阵。当前平台本机验证只能证明当前平台行为；其它支持平台需要目标平台本机或 CI matrix 验证。

#### 全局验证

1. 启动应用后进入 Dashboard。
   - 首屏快速显示本地摘要。
   - 后台远程状态从 checking 更新为最终状态。
2. 快速切换所有页面。
   - 不应出现长时间整页空白。
   - 已加载本地数据不应因远程刷新失败消失。
3. 在远程不可达情况下启动 GUI。
   - Dashboard、Files、History、Settings 仍显示本地可用内容。
   - 远程错误只显示在远程状态区域。
4. 执行一次 QuickSync/QuickPush/QuickPull。
   - Dashboard、Files、History 正确刷新。
   - Binaries、Projects 不因无关同步执行重刷新。

#### Settings 验证

1. 打开 Settings 连接 tab。
   - 首屏快速出现。
   - 不扫描排除目录。
   - 不执行 `claude --version`。
2. 切换到排除项 tab。
   - 目录和排除文件局部加载。
   - 加载失败不影响连接配置区域。
3. 保存 WebDAV 配置。
   - 只刷新连接配置和必要远程 cache。
   - 不重新检测 Claude binary。
4. 保存排除项。
   - 只刷新排除项列表。
5. 检查加密相关文案。
   - 用户动作统一描述为“加密密码”。

#### Binaries 验证

1. 打开 Binaries 页面。
   - 本地版本/安装状态先显示。
   - 远程版本和 storage 后台补齐。
2. 点击重新检测 Claude binary。
   - 有明确 loading。
   - 只执行一次检测。
   - 检测结果局部更新。
3. 上传、删除或切换版本。
   - 有明确进度或 loading。
   - 操作完成后 Binaries 和 Dashboard 状态更新。
   - binary index cache 被正确失效。

#### Files 验证

1. 远程可用时打开 Files。
   - 本地扫描不等待远程请求。
   - 远程状态随后补齐。
2. 远程不可用时打开 Files。
   - 本地扫描结果不被远程超时阻塞。
   - 远程错误局部展示。
3. 执行 push/pull/sync 后切回 Files。
   - dirty 刷新触发。
   - 文件状态正确。
4. 在其它无关操作完成后保持 Files 隐藏。
   - 不应立即重扫本地树。

#### History 验证

1. 有本地快照时打开 History。
   - 列表快速显示。
   - 不等待远程。
2. 点击本地快照详情。
   - 本地详情可直接显示。
3. 无本地快照时打开 History。
   - 进入远程 fallback，并显示局部加载/错误。
4. 加载更多。
   - 追加数据，不重置整页。

#### Dashboard 验证

1. 启动后 Dashboard 首屏显示本地信息。
2. 后台远程状态正确更新为 synced/pending/conflict/remote_uninitialized/connection_error 等。
3. 点击重新检查。
   - 只刷新远程 delta。
   - 不重新执行无关本地重扫描。
4. QuickSync/QuickPush/QuickPull/Repair 后 Dashboard 状态正确。

#### Projects 验证

1. 打开 Projects。
   - 先显示缓存或已追踪项目。
   - 后台发现项目并局部更新。
2. 模拟 git remote 慢或失败。
   - 单个项目失败不阻塞整个列表。
3. 项目很多时。
   - 不同时启动过多 git 进程。

#### Onboarding 验证

1. 未初始化状态下打开应用。
   - Onboarding 首屏保持快速。
2. 测试 WebDAV 连接。
   - 显示明确 loading 和结果。
3. 加入已有同步组。
   - 远程预览有状态反馈。
   - 同一输入重复预览不重复拉取相同远程数据。

## 完成定义

本方案完成必须同时满足：

1. 所有纳入范围页面已接入统一加载模型。
2. 已确认慢点没有被故意保留。
3. Dashboard、Settings、Binaries、Files、History、Projects、Onboarding 均完成手工验证。
4. Go 测试、前端构建、Wails 构建、diff 检查通过。
5. 支持平台矩阵 `windows-amd64`、`darwin-arm64`、`linux-amd64` 已完成对应构建和桌面验证，或明确标注未验证平台。
6. 若改动 `core/`，CLI 影响已审查并通过相应测试。
7. 自动生成的 `wailsjs` 变更来自生成流程，不含手工业务逻辑改动。
