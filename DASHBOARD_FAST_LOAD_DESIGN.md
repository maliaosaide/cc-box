# 概览页快速加载设计方案

## 背景

当前 GUI 启动后默认进入概览页。概览页需要展示本地 Claude 版本、备份快照、配置状态、同步状态、远程二进制版本和设备信息。

这些数据里有两类来源：

- 本地数据：本地配置、本地 HEAD、本地快照缓存、本地 Claude 二进制版本、本地冲突文件。
- 远程数据：WebDAV HEAD、远程快照、远程二进制索引、远程设备列表。

之前的问题是首屏会等待远程检查完成后才显示概览内容。即使已经做了页面 lazy mount 和部分后端去重，WebDAV 检查、远程快照读取、远程二进制索引读取仍然可能让概览页右侧出现短暂空白或加载较慢。

## 目标

1. 启动后概览页尽快显示可用信息。
2. 启动时自动检查一次远程状态。
3. 页面切换回概览时不重复检查远程。
4. 同步、推送、拉取、修复远程后刷新远程状态。
5. 用户能明确看到当前状态是“本地已加载，正在检查远程”，而不是误以为页面卡住。
6. 不改变同步协议、快照格式、加密格式、WebDAV 路径或 CLI 行为。

## 非目标

- 不改同步算法。
- 不改冲突判定规则。
- 不改 WebDAV 数据结构。
- 不引入自动定时远程检查。
- 不在用户只是切换页面时重复请求 WebDAV。

## 总体方案

概览页采用两段式加载：

1. **本地快速加载**
   - 只读取本地配置、本地 HEAD、本地快照缓存、本地 Claude 版本、本地冲突状态。
   - 立即返回给前端渲染。
   - 同步状态显示为 `checking` 或“正在检查远程...”。

2. **后台远程检查**
   - 启动后后台请求 WebDAV。
   - 检查远程 HEAD、远程快照、二进制索引、设备列表。
   - 检查完成后更新概览页状态。
   - 后续只有同步相关动作完成或用户手动刷新时才重新检查。

## 状态模型

新增一个 GUI 展示层状态，不改变核心同步语义：

| 状态 | 含义 | 展示文案 |
| --- | --- | --- |
| `checking` | 本地数据已显示，正在检查远程 | 正在检查远程... |
| `synced` | 本地 HEAD 与远程 HEAD 一致 | 已同步 |
| `pending` | 本地 HEAD 与远程 HEAD 不一致 | 待同步 |
| `conflict` | 本地存在冲突文件 | 存在冲突 |
| `remote_uninitialized` | 远程没有 HEAD | 远程未初始化 |
| `remote_incomplete` | 远程 HEAD 或快照不完整 | 远程数据不完整 |
| `key_mismatch` | 加密密码无法解密远程快照 | 加密密码不匹配 |
| `connection_error` | WebDAV 访问失败 | 连接异常 |
| `local_error` | 本地 HEAD 或配置异常 | 本地配置异常 |

`checking` 只用于 GUI 体验层，不写入本地 HEAD，也不代表新的同步结果。

## 后端接口设计

### `GetDashboardLocal()`

用于首屏快速返回。

职责：

- 检查是否已初始化。
- 读取配置。
- 读取本地 HEAD。
- 优先读取本地快照缓存。
- 构造备份列表。
- 检测本地 Claude 二进制版本。
- 读取本地冲突数量。
- 构造配置状态。
- 返回 `syncStatus: "checking"`。

不做：

- 不访问 WebDAV。
- 不下载远程快照。
- 不读取远程二进制索引。
- 不读取远程设备列表。

建议返回结构继续使用 `DashboardData`，避免前端新增一套模型。

### `RefreshDashboardRemote()`

用于后台远程检查。

职责：

- 读取 WebDAV HEAD。
- 校验远程 HEAD。
- 加载远程快照。
- 对比本地 HEAD 和远程 HEAD。
- 读取远程二进制索引。
- 读取远程设备列表。
- 合并本地基础信息后返回完整 `DashboardData`。

可以复用现有 `GetDashboard()` 中的远程逻辑，但应避免再次执行不必要的本地慢操作。

### 兼容保留 `GetDashboard()`

短期可以保留现有 `GetDashboard()`，供测试或旧调用使用。前端概览页切到新接口：

- 首屏调用 `GetDashboardLocal()`。
- 后台调用 `RefreshDashboardRemote()`。

后续如果确认没有其他调用，再考虑将 `GetDashboard()` 简化为组合方法或删除。

## 前端流程设计

文件：`gui/frontend/src/pages/Dashboard.svelte`

### 首次挂载

1. `onMount` 调用 `loadLocal()`。
2. `loadLocal()` 调用 `GetDashboardLocal()`。
3. 本地数据返回后立即渲染概览。
4. 如果本地状态不是明显本地错误，再调用 `refreshRemote()`。
5. `refreshRemote()` 调用 `RefreshDashboardRemote()`。
6. 远程返回后替换 `dashboard` 并更新 `syncState`。

### 页面切换

- 切到其他页面再切回概览，不重新调用远程检查。
- 因为 `App.svelte` 已经采用 mountedPages 保留页面实例，概览页状态会保留。

### 同步动作后

以下动作完成后调用 `refreshRemote()`：

- `QuickPush()` 完成。
- `QuickPull()` 完成。
- `QuickSync()` 完成。
- `RepairRemoteFromLocal()` 完成。

现有 `op:complete` 事件中可以把 `await refresh()` 改成 `await refreshRemote()` 或先本地后远程刷新。

### 手动刷新

可以在概览页状态区域增加一个小按钮：

- 文案：`重新检查`
- 行为：调用 `refreshRemote()`
- 禁用条件：远程检查进行中

这个按钮不是首版必需，但建议加入，方便用户主动确认远程状态。

## 数据刷新策略

| 场景 | 本地加载 | 远程检查 |
| --- | --- | --- |
| 应用启动进入概览 | 是 | 是 |
| 从其他页面回到概览 | 否 | 否 |
| 推送完成 | 可选 | 是 |
| 拉取完成 | 可选 | 是 |
| 同步完成 | 可选 | 是 |
| 修复远程完成 | 可选 | 是 |
| 用户点击重新检查 | 否 | 是 |
| 设置页保存 WebDAV 配置 | 不自动 | 用户回概览后可手动检查，或后续再扩展事件 |

## 实现步骤

1. 后端拆分本地数据构建函数。
   - 新增 `GetDashboardLocal()`。
   - 复用 `buildConfigStatus()`、`collectClaudeBinaryInfo(nil)`、`GetLocalSnapshotList()`、`listConflicts()`。

2. 后端拆分远程检查函数。
   - 新增 `RefreshDashboardRemote()`。
   - 复用 `loadClientKey()`、`fillDashboardFromSnapshots()`。
   - 避免重复本地 Claude 版本检测。

3. 前端调整概览页加载流程。
   - 首屏调用 `GetDashboardLocal()`。
   - 设置 `syncState = 'checking'`。
   - 后台调用 `RefreshDashboardRemote()`。
   - 远程失败时显示对应错误状态，不清空本地数据。

4. 增加状态文案。
   - `Dashboard.svelte` 的 `statusLabel()` 增加 `checking`。
   - `Sidebar.svelte` 的状态展示增加 `checking`。

5. 同步完成后刷新远程。
   - `op:complete` 成功或失败后调用远程刷新。
   - 失败时保留错误提示，同时刷新最新可见状态。

6. 运行验证。
   - Go 后端测试。
   - 前端构建。
   - Wails 构建。
   - 启动 exe 验证 1.5 秒内概览页显示本地数据。

## 测试计划

### Go 测试

新增或调整：

- `TestGetDashboardLocalDoesNotRequireRemote`
  - 无 WebDAV 可用时仍能返回本地数据。
  - `syncStatus` 为 `checking` 或本地错误状态。

- `TestRefreshDashboardRemoteReportsSynced`
  - 本地 HEAD 与远程 HEAD 一致时返回 `synced`。

- `TestRefreshDashboardRemoteReportsPending`
  - 本地 HEAD 与远程 HEAD 不一致时返回 `pending`。

- `TestRefreshDashboardRemoteKeepsLocalBinaryVersion`
  - 远程二进制索引加载时不重复执行本地版本检测。

### 前端验证

- `npm --prefix gui/frontend run build`
- 打开 exe 后观察：
  - 左侧菜单出现。
  - 概览右侧快速显示本地数据。
  - 状态先显示“正在检查远程...”。
  - 远程检查完成后状态更新。

### 桌面验证

- `wails build`
- 启动 `gui/build/bin/cc-box-gui.exe`
- 截图或人工确认：
  - 1.5 秒内概览页不是空白。
  - 同步按钮操作完成后状态刷新。
  - 页面切换回概览不重复长时间加载。

## 风险和处理

### 风险：本地数据和远程状态短暂不一致

启动时用户可能先看到本地快照，随后远程检查显示待同步。

处理：用“正在检查远程...”明确表达状态尚未最终确认。

### 风险：远程检查失败时误以为本地也不可用

处理：远程失败只更新状态为连接异常，不清空已经显示的本地数据。

### 风险：同步完成后状态不刷新

处理：所有 `op:complete` 分支统一触发一次 `RefreshDashboardRemote()`。

### 风险：接口拆分导致重复代码

处理：后端抽出内部构建函数，例如：

- `buildDashboardBase(cfg)`
- `fillDashboardFromLocal(data, cfg)`
- `fillDashboardFromRemote(data, cfg, client, key)`

只抽共享逻辑，不做额外架构重构。

## 建议结论

建议采用两段式加载方案：

- 启动时本地数据立即显示。
- 后台只检查一次远程。
- 后续只在同步相关动作或用户手动刷新时检查远程。

这样能同时解决首屏空白、WebDAV 慢导致的体验问题，以及频繁远程请求的问题，而且不会改变现有同步数据格式和核心语义。
