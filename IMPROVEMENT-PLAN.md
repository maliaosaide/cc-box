# CC-Box GUI 完善计划

基于 2026-05-08 的测试反馈，10 项 GUI 问题逐项分析，按优先级和依赖关系排序。

---

## Sprint 1：立即修复（P0 — 功能缺陷）

### 10. 同步时间显示不正确 [已修复]

**现象**：History 页面显示的快照时间、Dashboard 最近变更时间、二进制上传时间全部比实际时间少 8 小时。

**根因分析**：

1. `snapshot.CreateSnapshot` (`snapshot.go:52`) 用 `time.Now().UTC()` 存储 UTC 时间
2. GUI 后端所有 `Format()` 调用（`pages.go:88/116/122/585`、`dashboard.go:183`）直接格式化 UTC 时间，未转为本地时区
3. `DashboardData.LastSync` 字段从未被赋值，概览页无法显示上次同步时间
4. 最近变更的 Time 硬编码为 `"刚刚"`，不反映真实时间

**修复内容**：

- `pages.go`：4 处 `snap.Timestamp.Format(...)` → `snap.Timestamp.Local().Format(...)`
- `dashboard.go`：添加 `data.LastSync` 赋值（取最新快照的本地时间）
- `dashboard.go`：最近变更 Time 从 `"刚刚"` 改为 `localSnap.Timestamp.Local().Format("15:04")`

**验证标准**：
- History 页面时间与本地时钟一致
- Dashboard 概览页显示"上次同步"时间
- 最近变更的时间不再是固定"刚刚"

### 1. 同步后历史记录不显示

**现象**：Dashboard 点击推送/拉取/同步，进度条走完后，History 页面显示"暂无快照记录"。

**根因分析**：

`GetSnapshotList()` (`pages.go:52`) 从远程 WebDAV 读取 HEAD，再沿 parent 链遍历。`QuickPush()` (`dashboard.go:279`) 创建新快照并上传到 `snapshots/{id}.json.enc`，然后更新远程 HEAD。流程本身是完整的。

问题出在 `QuickPush` 的本地缓存写入时机——第 310-311 行：

```go
os.WriteFile(config.CCBoxDir()+"/HEAD", []byte(newSnap.ID), 0600)
os.WriteFile(config.CCBoxDir()+"/snapshots/"+newSnap.ID+".json", snapData, 0600)
```

这两行写入了本地缓存，但 `GetSnapshotList` **完全不走本地缓存**，每次都从远程 HEAD 开始下载。如果 WebDAV PUT 的 HEAD 更新存在延迟（如坚果云的最终一致性），或者网络请求时序导致读到了旧 HEAD，就会看不到新快照。

此外，`loadSnapByID` (`pages.go:132`) 虽然先查本地缓存，但 `GetSnapshotList` 的入口是从远程 HEAD 开始的——如果远程 HEAD 还没更新，链就断了。

**方案**：

1. `GetSnapshotList` 改为**先读本地 HEAD**，从本地 HEAD 开始向 parent 遍历，每一步都先尝试本地缓存（已实现），本地缺失时才下载远程
2. `QuickPush` 完成后，在 `op:complete` 事件中通知 History 页面刷新（前端已有 `refresh()` 调用）
3. 增加一个 `GetLocalSnapshotList(limit)` 方法，纯本地读取快照链，不依赖网络

**改动范围**：
- `gui/pages.go`：新增 `GetLocalSnapshotList`，`GetSnapshotList` 改为本地优先 + 远程回退
- `gui/dashboard.go`：`QuickPush`/`QuickPull` 完成后确认本地 HEAD 已写入
- `gui/frontend/src/pages/History.svelte`：监听 `op:complete` 事件后自动刷新

**设计思路**：

CLI 的 push 流程（`cli/push.go`）也有同样模式——先写本地 HEAD 再更新远程。GUI 应该对齐：**本地状态是真相来源**，远程是持久化后端。History 展示应以本地快照缓存为主，远程同步是后台操作。

**验证标准**：
- QuickPush 后立即切到 History 页面，能看到新快照
- 断网状态下仍能看到已有的历史快照（从本地缓存）

---

### 2. 删除底部 StatusBar（重复的同步指示）

**现象**：Sidebar 底部和窗口底部各有一个同步状态显示，功能重复。

**现状**：
- `Sidebar.svelte:66-70`：底部固定区域显示 "已同步 · 刚刚"，但内容是硬编码的，不响应 `syncState`
- `StatusBar.svelte`：底部 28px 高度的条，显示 syncState 驱动的状态 + 版本号
- `App.svelte:69`：`<StatusBar {syncState} />` 引入底部条

**方案**：

1. **删除 `StatusBar.svelte` 组件**（或保留文件但不引用）
2. **将 Sidebar 的同步状态改为响应式**——接收 `syncState` prop，根据状态显示不同文案和颜色
3. 版本号 `v0.1.0` 移到 Sidebar 底部或直接去掉（Dashboard 已有版本信息）

**改动范围**：
- `gui/frontend/src/App.svelte`：移除 `<StatusBar>` 和 import
- `gui/frontend/src/lib/components/Sidebar.svelte`：增加 `export let syncState`，状态区域响应式
- 删除或空置 `StatusBar.svelte`

**设计思路**：

Sidebar 底部是同步状态的天然位置——用户视线自然落在左侧导航栏的底部。底部 StatusBar 占用垂直空间，在紧凑的桌面应用中是不必要的。保留 Sidebar 一个入口即可。

**验证标准**：
- Sidebar 底部根据推送/拉取状态实时变化（同步中→已同步→冲突）
- 窗口底部无多余条

---

## Sprint 2：功能增强（P1 — 体验改进）

### 3. 配置文件树目录状态颜色聚合

**现象**：文件树中，叶子文件有状态图标（✓/M/A/D/C），但目录始终显示📁，无法一眼看出哪些目录包含变更或冲突。

**现状**：`TreeNode.svelte` 中，目录行只有名称和箭头，没有状态标记。`hasMatchingChild()` 仅用于过滤，不用于显示。

**方案**：

1. 在 `TreeNode.svelte` 中新增 `aggregateStatus` 函数，递归计算目录的综合状态：
   - 子树包含 `conflict` → 目录标记为冲突（红色 C）
   - 子树包含 `modified` 或 `added` → 目录标记为变更（橙色 M/A）
   - 子树包含 `deleted` → 目录标记为删除（灰色 D）
   - 全部 `synced` → 目录标记为同步（绿色 ✓）
2. 目录行显示聚合状态图标，放在📁右侧
3. 后端 `GetFileTree` 如果能直接在 Node 结构中返回 `aggregatedStatus` 字段更好，但前端计算也可接受

**改动范围**：
- `gui/frontend/src/lib/components/TreeNode.svelte`：目录行增加状态标记
- 可选：`gui/files.go` 的 `FileNode` 结构增加 `AggregatedStatus` 字段

**设计思路**：

聚合优先级：`conflict > modified/added > deleted > synced`。这和 VS Code 的 SCM 视图、Git 客户端的文件树行为一致。实现上纯前端递归即可，不需要后端改动。

**验证标准**：
- 目录包含冲突文件时显示红色 C
- 目录包含修改文件时显示橙色 M
- 未展开的目录能通过颜色快速定位变更区域

---

### 4. 二进制管理页面合并与完善

**现象**：
- 本地版本和云端版本分两个列表显示，同一版本可能在两处出现
- 上传按钮在本地+云端都有该版本时应该灰显（已上传）
- 云端版本缺少删除按钮
- 缺少从二进制管理页面直接上传当前正在使用的二进制（而非仅从版本目录）

**方案**：

1. **合并列表**：按版本号去重，每个版本一行，用标签标注 `本地` / `云端`
2. **上传按钮状态**：版本已存在于云端时，上传按钮灰显 + 文案改为"已上传"
3. **删除按钮**：云端版本增加删除按钮（调用 `DeleteCloudBinaryVersion` 新接口）
4. **当前版本上传**：在"当前版本"区域增加"上传到云端"按钮（直接上传 `binPath` 对应的文件）

**改动范围**：
- `gui/frontend/src/pages/Binaries.svelte`：重构版本列表渲染逻辑
- `gui/pages.go`：新增 `DeleteCloudBinaryVersion` 方法，修改 `GetBinaryPage` 返回合并后的数据
- `gui/dashboard.go`：新增 `UploadCurrentBinary` 方法

**后端改动细节**：

`GetBinaryPage` 当前返回 `localVersions` 和 `versions`（云端）两个数组。改为返回统一的 `allVersions`：

```go
type UnifiedBinaryVersion struct {
    Version    string `json:"version"`
    Size       int64  `json:"size"`
    IsLocal    bool   `json:"isLocal"`
    IsRemote   bool   `json:"isRemote"`
    IsCurrent  bool   `json:"isCurrent"`
    UploadedBy string `json:"uploadedBy"`
    UploadedAt string `json:"uploadedAt"`
}
```

按版本号合并 local + remote 数据，`isLocal` / `isRemote` 分别标记。

**设计思路**：

用户不关心文件存在哪里，只关心"我有哪些版本"和"我能做什么操作"。合并列表消除了认知负担。标注 `本地`/`云端` 的小标签用颜色区分即可（本地=默认色，云端=accent 色）。

**验证标准**：
- 同版本在本地和云端都存在时只显示一行，双标签
- 已上传到云端的版本，上传按钮灰显
- 云端版本有删除按钮
- 当前版本可直接上传到云端

---

### 5. 还原（Revert）包含二进制文件

**现象**：History 页面的"回滚到此版本"只恢复配置文件（`~/.claude/`），不处理二进制文件。但快照的 `Binary` 字段记录了当时的二进制版本信息。

**现状**：`RevertToSnapshot` (`pages.go:753`) 只遍历 `snap.Files`，不处理 `snap.Binary`。

**方案**：

1. 在 `RevertToSnapshot` 中，恢复配置文件后，检查 `snap.Binary` 字段
2. 对每个 platform + name 组合，检查当前本地二进制版本是否匹配
3. 不匹配时，从云端下载对应版本（如果远程有）或从本地版本目录查找
4. 不需要做二进制的版本管理链——只需要在快照创建时记录当时的版本，还原时尝试恢复

**改动范围**：
- `gui/pages.go`：`RevertToSnapshot` 增加 Binary 恢复逻辑
- `gui/dashboard.go`：`QuickPush` / `QuickSync` 创建快照时填充 `Binary` 字段（当前已由 snapshot 自动记录？需确认）

**设计思路**：

二进制文件体积大（~242MB），还原时不应该无条件下载。应先比较版本号，相同则跳过。如果远程没有该版本的分块数据，则跳过并提示用户手动安装。这是一个 best-effort 操作，不应阻塞配置文件的还原。

**验证标准**：
- 回滚到包含二进制信息的快照时，自动检测并恢复对应的 claude 版本
- 远程缺少对应版本时，配置文件仍正常恢复，二进制部分显示警告
- 回滚不影响当前正在运行的 claude 进程

---

### 6. 跨设备加密密钥输入

**现象**：加入已有同步组时，需要输入密码来派生加密密钥。但 Onboarding 的"加入"流程（step 3）只要求输入密码，没有说明"密码必须与已有设备相同"。

**现状**：`InitJoinExisting` 接收密码，用 Argon2id 派生密钥。如果密码不对，解密远程数据时会失败。Onboarding 界面有提示文案（"密码将用于解密云端已有数据"），但缺乏明确的验证机制。

**方案**：

1. **Onboarding 加强**：在 `InitJoinExisting` 中，先下载远程 HEAD 和第一个快照，尝试解密。解密失败则说明密码错误，提示用户重新输入
2. **Settings 增加密钥验证**：加密设置页增加"验证密钥"按钮，尝试解密一个远程快照来验证当前密钥是否正确
3. **导入密钥文件**：Settings 加密页增加"导入密钥"选项，允许从已有设备导出的密钥文件直接导入（绕过密码记忆）

**改动范围**：
- `gui/init.go`（或 `gui/onboarding.go`）：`InitJoinExisting` 增加密钥验证步骤
- `gui/frontend/src/pages/Onboarding.svelte`：加入流程增加验证反馈
- `gui/frontend/src/pages/Settings.svelte`：加密 tab 增加验证和导入按钮
- `gui/pages.go`：新增 `VerifyEncryptionKey`、`ExportKey`、`ImportKey` 方法

**设计思路**：

密码错误的场景必须尽早发现——不在加入时验证，就会在第一次 pull 时崩溃。加入流程中的验证步骤应该是阻塞式的：连接成功 → 输入密码 → **验证解密** → 完成。验证通过才允许进入主界面。

**验证标准**：
- 输入错误密码时，Onboarding 显示明确的错误提示
- Settings 加密页可验证当前密钥是否与远程数据兼容
- 密钥导出/导入流程可在两台设备间迁移

---

## Sprint 3：体验打磨（P2 — 视觉与交互）

### 7. WebDAV 连接信息安全

**现象**：Settings 连接页面显示已保存密码（placeholder 显示"已保存"），需要确认构建产物不包含硬编码的测试凭据。

**现状**：Settings.svelte 的密码输入框用 placeholder 提示 "已保存"，不会泄露密码值。`GetConfig` 返回 `hasPassword: true`，不返回实际密码。这个部分**已经是安全的**。

需要确认的是：
- `gui/init.go` 或其他文件中是否有调试用的硬编码 URL/密码
- 构建产物中是否有残留的测试配置

**方案**：

1. 全局搜索 `jianguoyun`、`password`、`admin`、`test` 等关键词，确认无硬编码凭据
2. `.gitignore` 确保 `config.yaml`、`*.key` 不被提交
3. 构建前运行 `grep -r` 检查

**改动范围**：
- 检查并清理（如有）硬编码凭据
- 确认 `.gitignore` 覆盖

**验证标准**：
- `grep -rn "password\|passwd\|secret" gui/ --include="*.go"` 无敏感结果
- 构建的二进制中不含明文凭据

---

### 8. 版本信息显示所有存在版本

**现象**：Dashboard 概览页的"版本"区域只显示当前 claude 版本和硬编码的 uv 条目。

**现状**：`GetDashboard` (`dashboard.go:105-107`) 硬编码了 `uv` 的 BinaryInfo，且 `claudeLatest` 始终为 true（没有比较逻辑）。

**方案**：

1. 从 `GetBinaryPage` 的逻辑中提取本地 + 远程版本信息到 Dashboard
2. 显示所有已安装的二进制工具（claude、uv、uvx 等），每个标注版本号
3. 对每个工具，检查远程是否有更新版本（通过 binary index），标注"最新"或"可更新"
4. 点击"管理二进制"跳转到 Binaries 页面（已有此按钮）

**改动范围**：
- `gui/dashboard.go`：`GetDashboard` 增加动态二进制版本扫描
- `gui/frontend/src/pages/Dashboard.svelte`：版本区域动态渲染

**设计思路**：

Dashboard 应该是一个"一眼看到全局状态"的页面。版本信息的核心是：
- 当前安装的工具有哪些，版本是多少
- 是否有可用更新
- 快速跳转到详细管理

不需要在 Dashboard 展示历史版本列表——那是 Binaries 页面的职责。

**验证标准**：
- 概览页显示 claude 当前版本 + 安装状态
- 显示 uv/uvx（如安装）的版本
- 远程有更新版本时显示"可更新"标签

---

### 9. 概览页亮色/暗色主题切换

**现象**：当前只有暗色主题，需要增加亮色切换。

**现状**：`style.css` 中所有颜色通过 CSS 变量定义（`--surface-0`、`--accent` 等），全部是暗色值。没有 `[data-theme="light"]` 或 `.light` 覆盖层。

**方案**：

1. **定义亮色变量集**：在 `style.css` 的 `:root` 后增加 `[data-theme="light"]` 选择器，覆盖所有 CSS 变量
2. **主题切换按钮**：Dashboard 工具栏右侧增加太阳/月亮图标按钮
3. **持久化选择**：主题偏好保存到 localStorage，启动时读取
4. **App.svelte 根组件**监听主题变量，切换时设置 `document.documentElement.dataset.theme`

**亮色色板**（保持同一设计语言）：

```css
[data-theme="light"] {
  --surface-0: 249 247 244;
  --surface-1: 255 255 255;
  --surface-2: 242 240 236;
  --surface-3: 228 226 222;
  --accent: 180 95 55;
  --accent-dim: 155 80 45;
  --accent-bright: 200 120 75;
  --text-primary: 28 27 24;
  --text-secondary: 95 92 88;
  --text-muted: 145 141 136;
  --border: 218 216 212;
  --state-ok: 75 130 95;
  --state-warn: 175 140 45;
  --state-err: 175 65 65;
  --state-sync: 70 110 160;
}
```

**改动范围**：
- `gui/frontend/src/style.css`：增加亮色变量集
- `gui/frontend/src/App.svelte`：主题状态管理
- `gui/frontend/src/pages/Dashboard.svelte`：切换按钮

**设计思路**：

使用 `data-theme` 属性而非 class，语义更清晰。所有组件都用 CSS 变量，只需覆盖变量即可完成主题切换。噪声纹理的 opacity 在亮色模式下需要调低（0.015 → 0.008）。

**验证标准**：
- 点击切换按钮后所有页面颜色正确切换
- 刷新后保持用户选择的主题
- 状态颜色（ok/warn/err）在两个主题下都有足够对比度

---

## 实施时间线

| 阶段 | 内容 | 预计工时 | 状态 |
|------|------|---------|------|
| Sprint 1 | #10 时间显示 [已修复] + #1 同步后历史不显示 + #2 删除底部 StatusBar | 4h | #10 ✓ |
| Sprint 2 | #3 文件树颜色 + #4 二进制合并 + #5 还原含二进制 + #6 跨设备密钥 | 8h | |
| Sprint 3 | #7 安全检查 + #8 版本信息 + #9 主题切换 | 4h | |

**总计**：约 16 小时

---

## 依赖关系

```
#10 (时间) [已修复]
#1 (历史) ←── 无依赖，可独立修复
#2 (StatusBar) ←── 无依赖，可独立修复
#3 (颜色聚合) ←── 无依赖，纯前端
#4 (二进制合并) ←── 依赖后端数据结构改动
#5 (还原二进制) ←── 依赖 #4 的数据结构
#6 (跨设备密钥) ←── 无依赖，可独立实现
#7 (安全检查) ←── 无依赖，随时可做
#8 (版本信息) ←── 依赖 #4 的数据获取逻辑
#9 (主题切换) ←── 无依赖，纯前端
```

建议执行顺序：`#10 ✓ → #2 → #1 → #7 → #3 → #4 → #5 → #8 → #6 → #9`

- `#10` 已修复——所有 `Format()` 前加了 `.Local()`，`LastSync` 已赋值
- `#2` 和 `#1` 是最简单的立即修复，热身
- `#7` 安全检查应在任何新功能之前完成
- `#3` 纯前端改动，不需要等后端
- `#4` → `#5` → `#8` 是二进制相关的一条线
- `#6` 和 `#9` 独立，可穿插进行
