# CC-Box 全面核查问题与修复计划

> 生成日期：2026-05-23  
> 范围：跨平台、前端、后端三类问题。  
> 目标：把已发现的需修复问题、代码位置、边界条件、已有测试/已补测试、后续测试计划和修复顺序集中记录，后续按阶段逐项修复。

## 当前状态

### 已执行的验证

- `core`: `go test ./...` 通过。
- `cli`: `go test ./...` 通过。
- `gui`: `go test ./...` 通过。
- `gui/frontend`: `npm run build` 通过。

这些验证只能说明当前代码可编译、现有测试通过；本文件中的问题多数属于现有测试未覆盖的逻辑边界、数据一致性、跨平台语义或前端异步竞态。

### 执行结果（2026-05-23）

- P0 已完成：CLI revert/restore/pull/conflict resolve 的路径安全、HEAD 发布语义、CAS 和错误处理已修复；scanner 不再跟随 symlink。
- P1 已完成：路径大小写规范、Windows object cache 文件名、项目目录反解、当前平台 binary prune、Linux opener、watcher error channel 已修复。
- P2 已完成：Projects/Files/History/Binaries/Dashboard/Onboarding/Settings 的高风险前端状态、过期响应和错误可见性已修复。
- P3 已完成：binary index CAS 与远程锁、GUI opResults 有界清理、新设备初始化条件创建与初始化锁、跨页面 dirty/refresh、项目缓存后台回传已修复。
- 最新验证：`core/go test ./...`、`cli/go test ./...`、`gui/go test ./...`、`gui/frontend/npm run build` 均通过。
- 复核后追加修复：保留原始路径大小写，object 上传/下载不再把文本 CRLF 改写为 LF；CLI 初始化改用配置后的 WebDAV 根路径、初始化锁与 `PUTIfAbsent`；GUI 远程修复改用初始化锁与条件创建。
- Windows 真实 Wails 开发环境验证已完成：Dashboard、配置文件、项目、二进制、历史、设置页面可加载；Dashboard 重新检查会进入检查中并恢复到已同步，顶部状态同步更新。
- 未执行破坏性真实操作：未点击真实推送、拉取、同步、删除本地、删除云端、切换二进制或回滚确认；这些路径由自动化测试和非破坏性页面验证覆盖到边界。

### 已经修改并补测试的内容

以下是本轮对话中已经完成、但尚未提交的修复和测试，后续修复其他问题时不要误删。

| 已完成项 | 修改位置 | 已补测试 | 验证状态 |
| --- | --- | --- | --- |
| 托盘图标与同步异步状态绑定 | `gui/async.go` | `gui/async_test.go`：同步成功、同步失败、非同步操作、嵌套同步操作 | `gui/go test ./...` 通过 |
| GUI 单实例锁 | `gui/main.go` | `gui/main_test.go`：单实例锁非空、固定 UUID v4、二次启动回调已注册 | `gui/go test ./...` 通过 |
| CLI revert/restore/pull/conflict 数据安全 | `cli/internal/cli/*.go` | `cli/internal/cli/conflicts_test.go`、`phase3_test.go`，覆盖嵌套冲突、安全路径、短 ID | `cli/go test ./...` 通过 |
| scanner symlink 和大小写碰撞 | `core/snapshot/scanner.go`、`core/normalize/normalize.go` | `core/snapshot/scanner_test.go`、`core/normalize/normalize_test.go` | `core/go test ./...` 通过 |
| Windows object cache 文件名 | `core/object/store.go` | `core/object/hash_test.go`：缓存文件名不含冒号 | `core/go test ./...` 通过 |
| 项目目录 metadata cwd 识别 | `cli/internal/project/tracker.go`、`gui/internal/project/tracker.go` | CLI/GUI tracker 测试覆盖连字符路径 | `cli/go test ./...`、`gui/go test ./...` 通过 |
| binary prune 当前平台限制 | `cli/internal/cli/binary.go` | `cli/internal/cli/binary_test.go`：只收集当前平台 Claude 版本 | `cli/go test ./...` 通过 |
| Linux opener 和 watcher 错误处理 | `gui/internal/desktop/file_opener_linux.go`、`gui/watcher.go` | Linux opener fallback 测试、watcher error 测试 | `gui/go test ./...` 通过 |
| 前端异步竞态和错误可见性 | `gui/frontend/src/pages/*.svelte` | 当前以 `npm run build` 和后端事件测试覆盖编译与绑定边界；运行时竞态需手动验证 | `gui/frontend/npm run build` 通过 |
| binary index CAS 和远程锁 | `core/binary/*.go`、`core/webdav/client.go` | `core/binary/upload_test.go` 覆盖 stale create 冲突与 CAS retry merge | `core/go test ./...` 通过 |
| GUI opResults 清理 | `gui/async.go` | `gui/async_test.go`：结果上限裁剪、消费后删除 | `gui/go test ./...` 通过 |
| 新设备初始化 TOCTOU | `gui/onboarding.go`、`core/webdav/client.go` | `gui/onboarding_test.go`：初始化锁互斥、半成品清理 | `gui/go test ./...` 通过 |
| 跨页面 dirty/refresh 与项目缓存回传 | `gui/pages.go`、`gui/files.go`、`gui/frontend/src/App.svelte`、`Projects.svelte` | 后端事件和前端构建验证；真实页面切换需手动验证 | `gui/go test ./...`、`gui/frontend/npm run build` 通过 |

### 不应纳入本次修复的内容

- 不修改生成物：`gui/frontend/wailsjs/`。
- 不修改依赖目录：`node_modules/`。
- 不把构建输出混入修复：`dist/`、`build/`。
- 不重新引入开机自启动相关逻辑。
- 涉及同步协议、快照、加密、WebDAV、二进制索引、HEAD 语义的问题，必须同时考虑 CLI/GUI 和 `core` 的一致性。

---

## P0：数据安全与数据一致性，优先修复

### P0-1 CLI 回滚/恢复路径越界风险

- 严重程度：Critical
- 类型：后端 / 安全 / 跨平台路径
- 位置：
  - `cli/internal/cli/revert.go:84`
  - `cli/internal/cli/revert.go:103`
  - `cli/internal/cli/maintenance.go:220`
  - `cli/internal/cli/maintenance.go:231`
- 问题：
  - CLI 回滚/恢复直接使用 `filepath.Join(config.ClaudeDir(), filepath.FromSlash(path))` 拼接远程 snapshot 中的路径。
  - 没有使用 `pathutil.SafeJoin` 或 CLI/GUI 等价安全路径函数。
  - 远程 snapshot 中若存在 `../outside`、绝对路径、Windows volume path 等非法路径，可能写入或删除 `.claude` 根目录外部文件。
- 影响：
  - 恶意或损坏的远程 snapshot 可诱导 CLI 覆盖/删除本机任意可写文件。
  - GUI 对应逻辑更安全，CLI/GUI 安全边界不一致。
- 修复边界：
  - 所有从 snapshot 中读取的文件路径，在读、写、删前都必须经过同一套安全路径校验。
  - 非法路径应直接中止当前 revert/restore，而不是跳过后继续执行半套操作。
  - 不改变远程 snapshot JSON 结构。
- 计划修改：
  - 将 CLI revert/restore 中的路径拼接统一替换为安全路径函数。
  - 若 CLI 当前没有共享 helper，则下沉或复用 `core/pathutil.SafeJoin`。
  - 对 snapshot ID 本身也做路径字符校验，避免加载远程 snapshot 时路径注入。
- 当前测试：
  - 现有测试未覆盖恶意 snapshot 路径。
- 需要新增测试：
  - CLI revert：snapshot 文件路径包含 `../outside` 时必须返回错误，且 `.claude` 外文件不被写入。
  - CLI restore：snapshot 文件路径包含绝对路径或 Windows volume path 时必须返回错误。
  - 合法嵌套路径仍可正常恢复。

### P0-2 CLI restore 只更新本地 HEAD，可能产生未发布快照

- 严重程度：Critical
- 类型：后端 / HEAD 一致性 / 错误处理
- 位置：
  - `cli/internal/cli/maintenance.go:236`
  - `cli/internal/cli/maintenance.go:237`
  - `cli/internal/cli/maintenance.go:238`
  - `cli/internal/cli/maintenance.go:239`
  - `cli/internal/cli/maintenance.go:240`
  - `cli/internal/cli/maintenance.go:243`
  - `cli/internal/cli/maintenance.go:244`
- 问题：
  - `restoreFromSnapshot` 创建 restore snapshot 后，只更新本地 `HEAD` 和本地 snapshot 缓存。
  - 没有更新远程 `HEAD`。
  - `Serialize`、`Encrypt`、`EnsureDir`、`PUT`、`WriteFile` 等错误被忽略。
- 影响：
  - 本地认为已恢复到新快照，但远程其他设备仍看到旧 `HEAD`。
  - 上传失败时，本地 `HEAD` 可能指向远程不存在或不可解密的 snapshot。
  - CLI/GUI 混用时会产生本地 HEAD、远程 HEAD、工作区状态分叉。
- 修复边界：
  - restore 是发布型操作，必须遵循与 push/revert 相同的 HEAD 发布语义。
  - 远程 HEAD 成功更新前，不允许更新本地 HEAD。
  - 所有错误必须显式返回。
- 计划修改：
  - restore 前读取远程 `HEAD` 和 ETag。
  - 上传 restore snapshot。
  - 使用 CAS / If-Match 更新远程 `HEAD`。
  - 远程 HEAD 更新成功后，再写本地 `HEAD` 和本地 snapshot 缓存。
- 当前测试：
  - 现有测试未覆盖 restore 远程发布语义和错误回滚。
- 需要新增测试：
  - snapshot 上传失败时，本地 HEAD 不变化。
  - 远程 HEAD CAS 失败时，本地 HEAD 不变化。
  - restore 成功时，远程 HEAD 和本地 HEAD 都指向新快照。

### P0-3 CLI revert 无 CAS 更新远程 HEAD，且忽略错误

- 严重程度：Critical
- 类型：后端 / 并发一致性 / HEAD 更新
- 位置：
  - `cli/internal/cli/revert.go:123`
  - `cli/internal/cli/revert.go:124`
  - `cli/internal/cli/revert.go:125`
- 问题：
  - `updateRemoteHEAD(client, newSnap.ID, "")` 无条件写远程 HEAD，没有 ETag / CAS。
  - `updateRemoteHEAD`、`updateLocalHEAD`、`saveLocalSnapshot` 的错误没有检查。
- 影响：
  - 多设备并发时，CLI revert 可能覆盖其他设备已经发布的新 HEAD。
  - 远程更新失败后，本地仍可能被写成新 HEAD。
- 修复边界：
  - revert 必须是乐观锁发布。
  - 失败时不得产生本地状态前进。
  - 与 GUI `RevertToSnapshot` 的语义保持一致。
- 计划修改：
  - revert 前读取远程 HEAD 和 ETag。
  - 校验远程 HEAD 仍处于预期状态。
  - 使用 CAS 更新远程 HEAD。
  - 所有错误返回。
- 当前测试：
  - 现有测试未覆盖 revert 并发冲突。
- 需要新增测试：
  - 远程 HEAD 已变化时，revert 返回错误且不更新本地 HEAD。
  - 远程 HEAD PUT/CAS 失败时，不写本地 HEAD。
  - 成功路径验证远程、本地 HEAD 一致。

### P0-4 scanner 跟随 symlink，可能同步 `.claude` 外部敏感文件

- 严重程度：Critical
- 类型：后端 / 安全 / 数据泄漏
- 位置：
  - `core/snapshot/scanner.go:131`
  - `core/snapshot/scanner.go:166`
  - `core/snapshot/scanner.go:167`
  - `core/snapshot/scanner.go:171`
- 问题：
  - `readableRegularFileInfo` 对 symlink 使用 `os.Stat(path)`，会跟随符号链接。
  - 随后 `os.ReadFile(path)` 会读取 symlink 目标内容。
- 影响：
  - `.claude` 内 symlink 可指向外部敏感文件，并被上传到 WebDAV。
  - pull/restore 侧不会还原 symlink 语义，跨设备后可能变成普通文件。
- 修复边界：
  - 默认不跟随 symlink 是最安全方案。
  - 如果未来要支持 symlink，应作为明确功能设计，保存 symlink 元数据，而不是读取目标内容。
- 计划修改：
  - scanner 默认跳过 symlink 或将 symlink 记录为扫描失败。
  - 建议选择“跳过并记录 failure”，让 GUI 能提示用户。
- 当前测试：
  - 现有测试未覆盖 symlink 指向外部文件。
- 需要新增测试：
  - `.claude/link -> outside-secret` 不应出现在 `ScanResult.Files`。
  - 不应读取 outside-secret 的内容。
  - Windows 上 symlink 测试需考虑权限，可跳过或用可用性检测。

### P0-5 CLI pull 遇到冲突仍更新本地 HEAD，与 GUI 语义不一致

- 严重程度：Important
- 类型：后端 / CLI-GUI 一致性 / 冲突语义
- 位置：
  - `cli/internal/cli/pull.go:304`
  - `cli/internal/cli/pull.go:305`
  - `cli/internal/cli/pull.go:308`
  - `cli/internal/cli/pull.go:396`
  - `cli/internal/cli/pull.go:399`
  - `gui/files.go:1100`
- 问题：
  - CLI pull 保存冲突文件后仍会更新本地 HEAD 为远程 HEAD。
  - GUI pull 在 `Conflicts > 0` 时不更新本地 HEAD。
- 影响：
  - CLI/GUI 混用同一个本地状态时，对“是否已位于远程 HEAD”的理解不同。
  - 后续 push/pull 的 merge base 可能变化，冲突处理结果不一致。
- 修复边界：
  - 必须先确定统一语义。
  - 建议选择 GUI 当前更保守语义：存在未解决冲突时不推进本地 HEAD。
  - 若需要记录远程 pending HEAD，应新增独立状态文件，不复用 HEAD。
- 计划修改：
  - CLI pull 在存在冲突时保存冲突文件，但不更新本地 HEAD、不缓存远程 ETag。
  - 冲突解决后由后续 push/pull 明确推进状态。
- 当前测试：
  - 现有测试未覆盖 CLI 与 GUI 冲突 HEAD 语义一致性。
- 需要新增测试：
  - CLI pull 产生冲突后，本地 HEAD 保持原值。
  - GUI pull 产生冲突后，本地 HEAD 保持原值。
  - 冲突解决后再同步能正常推进。

---

## P1：跨平台一致性和核心兼容性

### P1-1 路径大小写规范跨平台不一致

- 严重程度：Critical / Important
- 类型：跨平台 / snapshot key 语义
- 位置：
  - `core/normalize/normalize.go:18`
  - `core/normalize/normalize.go:19`
  - `core/normalize/normalize_test.go:25`
- 问题：
  - `PathLower` 只在 Windows 下转小写。
  - 注释和测试表达的是“跨平台一致”，测试期望 `FOO/BAR.JSON -> foo/bar.json`。
- 影响：
  - Windows 生成小写 key，macOS/Linux 保留大小写 key。
  - 同一 WebDAV 远程在不同平台间可能出现新增/删除误判、重复文件、大小写冲突。
- 修复边界：
  - 需要先确定产品策略：推荐采用“远程 snapshot key 全平台小写”。
  - 若采用全平台小写，必须处理大小写碰撞，例如 Linux 下同时存在 `Foo` 和 `foo`。
  - 不改变对象 hash 规则，只改变文件路径 key 的规范化一致性。
- 计划修改：
  - `PathLower` 改为无条件 `strings.ToLower`。
  - scanner 在写入 `result.Files[relPath]` 前检测 key 是否已存在。
  - 发现大小写碰撞时记录扫描失败并让完整 `Scan()` 返回错误，避免静默覆盖。
  - 排除规则匹配也要确认在规范化小写路径下符合预期。
- 当前测试：
  - `core/normalize/normalize_test.go:25` 已经表达了跨平台小写预期，但在 Windows 以外才会暴露失败。
- 需要新增测试：
  - `PathLower("FOO") == "foo"` 在所有平台成立。
  - scanner 遇到 `Foo.json` 和 `foo.json` 时不得静默覆盖，应返回失败。
  - exclude pattern 对大小写路径的行为明确。

### P1-2 Windows 本地 object 缓存文件名包含冒号

- 严重程度：Important
- 类型：跨平台 / Windows 文件系统
- 位置：
  - `core/object/store.go:295`
  - `core/object/store.go:297`
- 问题：
  - `cachePath` 使用完整 hash 作为文件名：`sha256:<digest>.enc`。
  - `:` 在 Windows 文件名中非法，或被解释为 NTFS alternate data stream。
- 影响：
  - Windows 本地 object 缓存写入失败或不可见。
  - 缓存失效导致性能下降，且错误可能被忽略。
- 修复边界：
  - 只改变本地缓存文件名，不改变远程 object path。
  - 需要兼容已有本地缓存：旧缓存可自然失效，不必迁移。
- 计划修改：
  - `cachePath` 使用 digest 部分作为文件名，例如 `<digest>.enc`。
  - hash 解析失败时返回安全 fallback 或错误。
- 当前测试：
  - 现有测试未覆盖 Windows 文件名合法性。
- 需要新增测试：
  - `cachePath("sha256:abc...")` 不包含 `:`。
  - 上传/下载仍可命中新缓存。

### P1-3 项目目录反解无法处理路径中的连字符

- 严重程度：Important
- 类型：跨平台 / Claude project 路径解析
- 位置：
  - `gui/internal/project/tracker.go:169`
  - `cli/internal/project/tracker.go:173`
- 问题：
  - `decodeProjectDir` 使用 `strings.Split(dirName, "-")` 反解路径。
  - 真实路径中常见连字符，例如 `cc-box`、`my-project`，会被错误拆分。
- 影响：
  - 当前项目路径就包含 `cc-box`，项目发现可能不准确。
  - 项目列表、remote 匹配、`.claude.json` 管理可能失效。
- 修复边界：
  - 不应靠不可逆 split 推断真实路径。
  - 优先读取 Claude project 元数据中保存的真实 cwd/path。
  - 若必须解码目录名，应实现与 Claude Code 完全一致的可逆规则，并补测试。
- 计划修改：
  - 调研 `.claude/projects/*` 中可用元数据字段。
  - CLI/GUI 两份 tracker 必须同步修改，或下沉到 core 共享实现。
- 当前测试：
  - 现有测试未覆盖路径包含连字符。
- 需要新增测试：
  - Windows 路径 `C:\Users\a\Desktop\cc-box` 能被正确识别。
  - Unix 路径 `/Users/a/my-project` 能被正确识别。

### P1-4 CLI `binary prune` 会清理所有平台版本

- 严重程度：Important
- 类型：跨平台 / 二进制管理范围
- 位置：
  - `cli/internal/cli/binary.go:253`
- 问题：
  - CLI prune 遍历 `idx.Platforms` 的所有平台。
  - 项目约定是 Claude 二进制管理只处理当前平台。
- 影响：
  - Windows 上 prune 可能删除 macOS/Linux 平台版本；其他平台同理。
- 修复边界：
  - 默认只 prune `config.Platform()` 当前平台。
  - 物理删除对象前仍需全局引用计数，避免删除其他平台仍引用的 hash。
- 计划修改：
  - `runBinaryPrune` 只收集当前平台 targets。
  - 若未来支持全平台 prune，必须显式 flag 和确认提示。
- 当前测试：
  - 现有测试未覆盖多平台 prune。
- 需要新增测试：
  - index 含 Windows/macOS/Linux 时，当前平台 prune 只移除当前平台未引用版本。
  - 其他平台版本记录保持不变。

### P1-5 Linux 文件管理器打开失败不可见

- 严重程度：Important
- 类型：跨平台 / Linux 桌面环境
- 位置：
  - `gui/internal/desktop/file_opener_linux.go:38`
  - `gui/internal/desktop/file_opener_linux.go:39`
- 问题：
  - 只调用 `cmd.Start()`，不等待 `xdg-open` / `gio` 的退出结果。
- 影响：
  - 无桌面、无 DBus、无默认文件管理器时，GUI 认为成功但实际没打开。
- 修复边界：
  - GUI 桌面场景下返回可见错误。
  - 无桌面用户使用 CLI，不需要为 GUI 做复杂兼容。
- 计划修改：
  - 使用带超时的 `cmd.Run()` 或 `CombinedOutput()`。
  - `xdg-open` 失败后继续尝试 `gio open`。
  - 返回 stderr 中的明确错误。
- 当前测试：
  - 现有测试未覆盖 Linux opener 失败。
- 需要新增测试：
  - 可通过注入 command runner 测试 `xdg-open` 失败后 fallback 到 `gio`。
  - 两者都失败时返回错误。

### P1-6 fsnotify watcher 未消费 Errors channel

- 严重程度：Important
- 类型：跨平台 / watcher 稳定性
- 位置：
  - `gui/watcher.go:119`
- 问题：
  - `watchLoop` 只读取 `w.fsw.Events`，不读取 `w.fsw.Errors`。
- 影响：
  - Linux inotify overflow、目录删除、底层 watcher 错误可能静默丢失。
  - 自动同步状态不可靠。
- 修复边界：
  - 读取 Errors channel 后至少要记录并将托盘置为异常或触发重新扫描。
  - 不应因为单个 watcher error 直接退出整个 GUI。
- 计划修改：
  - select 增加 `case err, ok := <-w.fsw.Errors`。
  - 保存最近 watcher error，必要时暴露给 Dashboard/Files。
- 当前测试：
  - 现有测试未覆盖 watcher errors。
- 需要新增测试：
  - 建议把 watcher loop 的事件处理抽出可测试函数。
  - 模拟 error 进入后，状态被记录且 watcher 不崩溃。

---

## P2：前端高风险状态和误操作问题

### P2-1 项目页空数组为 null 时崩溃

- 严重程度：Important
- 类型：前端 / 空数据边界
- 位置：
  - `gui/frontend/src/pages/Projects.svelte:80`
  - `gui/pages.go:629`
- 问题：
  - 后端 `ProjectListResult{}` 中 nil slice 可能序列化为 `null`。
  - 前端直接访问 `data.projects.length` 和 `data.orphans.length`。
- 影响：
  - 首次使用或无项目时项目页可能空白/崩溃。
- 修复边界：
  - 前端应对后端 null 做归一化。
  - 后端也可初始化空 slice，双保险。
- 计划修改：
  - 前端增加 `projects = data?.projects || []`、`orphans = data?.orphans || []`。
  - 后端 `discoverProjectList` 初始化空 slice。
- 当前测试：
  - 前端 build 不覆盖运行时空数据。
- 需要新增测试：
  - 若项目暂未引入前端单测，可新增轻量逻辑测试或在后端保证 JSON 为 `[]`。
  - 手动验证空项目页显示空态。

### P2-2 文件详情、冲突详情、Diff 存在过期响应

- 严重程度：Important
- 类型：前端 / 异步竞态 / 误操作
- 位置：
  - `gui/frontend/src/pages/Files.svelte:69`
  - `gui/frontend/src/pages/Files.svelte:85`
  - `gui/frontend/src/pages/Files.svelte:89`
  - `gui/frontend/src/pages/Files.svelte:99`
  - `gui/frontend/src/pages/Files.svelte:102`
- 问题：
  - 快速选择 A 再选择 B，A 的慢响应可能覆盖 B 的详情状态。
- 影响：
  - 显示“路径 B + 内容 A”。
  - 冲突处理时可能基于错误内容做选择。
- 修复边界：
  - 所有和当前选中文件相关的异步请求都必须校验请求仍然有效。
  - 不能只修 content，不修 conflict/detail/diff。
- 计划修改：
  - 增加 `detailRequestId` 或捕获 `path`。
  - 响应返回后确认 `selectedPath === requestPath` 且 requestId 仍匹配。
- 当前测试：
  - 现有前端构建不覆盖该竞态。
- 需要新增测试：
  - 若引入前端测试，模拟 A 慢、B 快，最终只能显示 B。
  - 手动验证快速点击文件时详情不串行污染。

### P2-3 历史详情过期响应可能导致错误回滚

- 严重程度：Important
- 类型：前端 / 异步竞态 / 高风险操作
- 位置：
  - `gui/frontend/src/pages/History.svelte:63`
  - `gui/frontend/src/pages/History.svelte:69`
  - `gui/frontend/src/pages/History.svelte:79`
- 问题：
  - 展开 A 后立即展开 B，A 的慢响应可能显示在 B 行下。
- 影响：
  - 用户看着 A 的详情，却点击 B 行回滚按钮。
- 修复边界：
  - detail 绑定必须和 expandedId 一致。
  - 回滚这类高风险操作建议增加二次确认。
- 计划修改：
  - `GetSnapshotDetail(id)` 返回后确认 `expandedId === id`。
  - detailLoading 时禁用回滚按钮。
  - 回滚前弹出确认，显示目标 snapshot id 和时间。
- 当前测试：
  - 无现有前端竞态测试。
- 需要新增测试：
  - 模拟过期 detail 响应不应覆盖当前展开行。
  - 手动验证回滚确认文案。

### P2-4 页面永久缓存导致跨页面状态不同步

- 严重程度：Important
- 类型：前端 / 状态失效机制
- 位置：
  - `gui/frontend/src/App.svelte:36`
  - `gui/frontend/src/pages/Files.svelte:111`
  - `gui/frontend/src/pages/Files.svelte:119`
  - `gui/frontend/src/pages/History.svelte:79`
- 问题：
  - `mountedPages` 永久缓存页面。
  - 不是所有修改共享状态的操作都会通知其他页面刷新。
- 影响：
  - 解决冲突后 Dashboard 冲突数可能旧。
  - 回滚后 Files/Dashboard 仍显示旧状态。
  - 设置/二进制修改后其他页面缓存不刷新。
- 修复边界：
  - 不建议简单取消所有页面缓存，否则可能带来性能和交互回退。
  - 应建立统一 data changed 事件或 App 层 dirty map。
- 计划修改：
  - 后端对 `ResolveConflict`、`ExcludeFile`、`RevertToSnapshot`、`SetConfigField` 等成功后发事件。
  - App 层维护 dirty version，各页面 active 时刷新。
- 当前测试：
  - 现有 build 不覆盖跨页面状态。
- 需要新增测试：
  - 手动验证：解决冲突后切回 Dashboard，冲突数刷新。
  - 回滚后切 Files，文件树刷新。

### P2-5 二进制上传早期失败不提示，且可重复点击

- 严重程度：Important
- 类型：前端 / 异步状态 / 错误可见性
- 位置：
  - `gui/frontend/src/pages/Binaries.svelte:41`
  - `gui/frontend/src/pages/Binaries.svelte:42`
  - `gui/frontend/src/pages/Binaries.svelte:49`
  - `gui/pages.go:1095`
  - `gui/pages.go:1120`
- 问题：
  - `op:complete` 在 `!uploadProgress` 时直接忽略。
  - 后端早期失败可能发生在首次 progress 前。
  - 按钮主要依赖 progress 禁用，首次 progress 前可重复启动。
- 影响：
  - 失败无提示。
  - 可启动多个上传任务。
- 修复边界：
  - 点击后立即进入 in-flight 状态。
  - 必须按 opId 关联完成事件。
- 计划修改：
  - `UploadBinaryVersion` / `UploadCurrentBinary` 返回 opId 后保存为 `uploadOpId`。
  - 即使没有 progress，complete 也要清理状态并显示错误。
- 当前测试：
  - 无前端运行时测试。
- 需要新增测试：
  - 后端早期返回错误时前端显示错误。
  - 重复点击被禁用。

### P2-6 Dashboard QuickSync 子操作可能覆盖顶层同步状态

- 严重程度：Important
- 类型：前端 / 异步状态
- 位置：
  - `gui/frontend/src/pages/Dashboard.svelte:31`
  - `gui/frontend/src/pages/Dashboard.svelte:39`
  - `gui/frontend/src/pages/Dashboard.svelte:40`
  - `gui/frontend/src/pages/Dashboard.svelte:87`
  - `gui/dashboard.go:533`
  - `gui/dashboard.go:556`
- 问题：
  - `QuickSync` 会启动 `QuickPull` / `QuickPush` 子操作。
  - 子操作完成事件可能触发 dashboard refresh，覆盖顶层 sync 状态。
- 影响：
  - 顶层同步未完成时 UI 提前显示“已同步/待同步”。
- 修复边界：
  - 当前 actionLoading 存在时，非当前顶层 op 不应覆盖主状态。
  - 进度可接收子操作，但状态最终归属顶层 op。
- 计划修改：
  - 区分顶层 opId 和子操作 opId。
  - `actionLoading` 存在时，忽略非当前 op 的状态刷新或延后刷新。
- 当前测试：
  - `gui/async_test.go` 已覆盖托盘层嵌套同步不提前切状态，但未覆盖前端 Dashboard。
- 需要新增测试：
  - 手动验证 QuickSync 过程中状态不会提前跳回已同步。
  - 若引入前端测试，模拟子 op complete 不覆盖顶层状态。

### P2-7 文件页远程刷新失败被静默吞掉

- 严重程度：Important
- 类型：前端 / 错误可见性
- 位置：
  - `gui/frontend/src/pages/Files.svelte:57`
  - `gui/frontend/src/pages/Files.svelte:59`
  - `gui/frontend/src/pages/Files.svelte:60`
- 问题：
  - `refreshTree()` 先加载本地树，再加载远程树。
  - 远程失败时，只在没有本地树时才显示错误。
- 影响：
  - 用户可能以为远程状态正常，实际只是本地预览或旧状态。
- 修复边界：
  - 保留本地树可用性，但必须显示远程刷新失败。
  - 同步状态应反映 connection/error。
- 计划修改：
  - 增加 `remoteError` 或直接设置 error banner。
  - 设置 `syncState = 'connection_error'` 或更具体状态。
- 当前测试：
  - 无前端运行时测试。
- 需要新增测试：
  - 模拟 `GetFileTreeLocal` 成功、`GetFileTree` 失败，页面显示远程错误。

### P2-8 项目列表缓存后台刷新结果不会回到前端

- 严重程度：Important
- 类型：前端 / 缓存刷新
- 位置：
  - `gui/frontend/src/pages/Projects.svelte:17`
  - `gui/pages.go:613`
  - `gui/pages.go:614`
- 问题：
  - 后端缓存命中时返回旧数据，并后台刷新。
  - 前端没有接收后台刷新完成的结果。
- 影响：
  - 项目/orphan 列表可能长期停留在旧缓存。
- 修复边界：
  - 不一定要取消后端缓存。
  - 前端 active 时应有刷新机制或后端事件通知。
- 计划修改：
  - 项目页 active 时调用 `RefreshProjectList()`。
  - 或后端后台刷新完成后 emit 事件。
- 当前测试：
  - 无缓存刷新测试。
- 需要新增测试：
  - 手动验证新增/删除项目后项目页能刷新。

### P2-9 加密密码指纹预览存在过期响应

- 严重程度：Important
- 类型：前端 / 异步竞态 / 安全 UX
- 位置：
  - `gui/frontend/src/pages/Onboarding.svelte:70`
  - `gui/frontend/src/pages/Settings.svelte:327`
  - `gui/frontend/src/pages/Settings.svelte:347`
- 问题：
  - debounce 异步请求没有校验当前输入是否仍是请求发起时的密码/WebDAV 参数。
- 影响：
  - 旧密码的指纹或 mismatch/success 状态可能显示在新输入下。
- 修复边界：
  - 不改变后端密码校验逻辑。
  - 只保证前端展示与当前输入一致。
- 计划修改：
  - 为每类预览维护 request id。
  - 响应后确认输入仍匹配再写状态。
- 当前测试：
  - 无前端运行时测试。
- 需要新增测试：
  - 手动验证快速修改密码时不会显示旧请求结果。

### P2-10 项目详情展开存在过期响应

- 严重程度：Important
- 类型：前端 / 异步竞态
- 位置：
  - `gui/frontend/src/pages/Projects.svelte:24`
  - `gui/frontend/src/pages/Projects.svelte:30`
- 问题：
  - 快速展开项目 A 再展开 B，A 的慢响应可能显示在 B 下。
- 影响：
  - 用户看到错误项目的 `.claude.json` 内容。
- 修复边界：
  - detail 与 expandedIdx/path 必须一致。
- 计划修改：
  - 捕获请求 path/index，响应后确认仍为当前展开项。
- 当前测试：
  - 无前端运行时测试。
- 需要新增测试：
  - 模拟过期响应不覆盖当前 detail。

---

## P3：后端并发、长期运行和初始化边界

### P3-1 二进制版本 index 无 CAS，多设备并发会丢更新

- 严重程度：Important
- 类型：后端 / WebDAV 并发一致性
- 位置：
  - `core/binary/index.go:74`
  - `core/binary/index.go:80`
  - `core/binary/index.go:81`
- 问题：
  - `SaveIndex` 直接 `PUT("binaries/index.json", data, "")`。
  - 没有 ETag / If-Match。
- 影响：
  - 多设备同时上传、切换 current、prune/delete 时，后写者覆盖先写者。
  - 可能丢版本、指向不存在的二进制、删除仍被引用版本。
- 修复边界：
  - 会改变 index 写入流程，但不改变 index JSON 结构。
  - CLI/GUI 都调用 core binary，需要一起验证。
- 计划修改：
  - `LoadIndex` 返回 index + ETag。
  - `SaveIndex` 接收 expected ETag 并 If-Match。
  - 冲突时重新加载、合并、重试。
- 当前测试：
  - 现有测试未覆盖并发 index 更新。
- 需要新增测试：
  - 两个客户端基于同一旧 ETag 写入，第二个必须失败或合并重试。
  - 上传、切换 current、prune 都走 CAS。

### P3-2 GUI `opResults` 永久增长

- 严重程度：Important
- 类型：后端 / GUI 长期运行内存
- 位置：
  - `gui/async.go:20`
  - `gui/async.go:63`
  - `gui/async.go:67`
- 问题：
  - `opResults` 全局 map 操作完成后只写入，不清理。
- 影响：
  - GUI 长期运行、自动同步/手动同步多次后 map 持续增长。
- 修复边界：
  - 不能立刻删除结果，因为 watcher / QuickSync 当前会等待子 op 后读取结果。
  - 需要保留短时间或显式消费后删除。
- 计划修改：
  - 引入 async manager 或 TTL 清理。
  - QuickSync / watcher 读取结果后删除对应 op result。
  - 保留最近 N 条结果用于调试也可以，但必须有上限。
- 当前测试：
  - `gui/async_test.go` 已覆盖异步状态，但未覆盖 result 清理。
- 需要新增测试：
  - 操作完成并被消费后，`opResults` 不保留无限增长。
  - 嵌套 QuickSync 读取子结果后清理子结果。

### P3-3 新设备初始化存在 TOCTOU

- 严重程度：Important
- 类型：后端 / 初始化并发 / WebDAV 一致性
- 位置：
  - `gui/onboarding.go:71`
  - `gui/onboarding.go:83`
  - `gui/onboarding.go:142`
- 问题：
  - 初始化先检查远程 `salt.bin` / `HEAD` 是否存在，再分别写入。
  - 检查和写入之间没有远程锁或条件创建。
- 影响：
  - 两个客户端同时初始化同一 WebDAV root，可能产生 salt、snapshot、HEAD 混合状态。
- 修复边界：
  - 不改变远程数据格式的前提下，优先用 WebDAV 条件写。
  - 如果 WebDAV 服务不支持条件写，需要返回明确错误或使用初始化锁文件。
- 计划修改：
  - 对 `salt.bin` / `HEAD` 首次创建使用 `If-None-Match: *` 或等价条件。
  - 初始化失败时尽量清理半成品或标记失败。
- 当前测试：
  - 现有测试未覆盖双客户端并发初始化。
- 需要新增测试：
  - 两个初始化并发时，只允许一个成功。
  - 失败者不能覆盖 salt 或 HEAD。

### P3-4 CLI conflict resolve 不递归且目录创建错误

- 严重程度：Important
- 类型：后端 / CLI 冲突处理
- 位置：
  - `cli/internal/cli/conflicts.go:40`
  - `cli/internal/cli/conflicts.go:50`
  - `cli/internal/cli/conflicts.go:116`
  - `cli/internal/cli/conflicts.go:117`
- 问题：
  - `runConflicts` 只读取冲突目录顶层，跳过子目录。
  - `runResolve` 对完整文件路径调用 `os.MkdirAll(targetPath, 0755)`，会把目标文件创建成目录。
  - 路径拼接也未使用 SafeJoin。
- 影响：
  - 嵌套路径冲突无法列出。
  - resolve 写入可能失败。
  - 存在路径越界风险。
- 修复边界：
  - 与 P0-1 的安全路径修复可以一起做。
  - conflict 文件路径只能来自已存在 conflict 记录或经过严格校验。
- 计划修改：
  - `runConflicts` 使用 `filepath.WalkDir` 递归收集 `.local` / `.remote`。
  - `runResolve` 使用安全路径函数定位目标文件。
  - 创建父目录用 `filepath.Dir(targetPath)`。
- 当前测试：
  - 现有测试未覆盖嵌套冲突。
- 需要新增测试：
  - 嵌套冲突能列出。
  - resolve 嵌套冲突创建父目录而不是目标目录。
  - `../` 参数无法写出 `.claude`。

---

## 分阶段修复计划

### 阶段 1：安全和数据一致性

优先修复会导致越界写入、数据泄漏或 HEAD 分叉的问题。

1. P0-1：CLI revert/restore 安全路径。
2. P0-2：CLI restore 远程 HEAD 发布和错误处理。
3. P0-3：CLI revert CAS 更新远程 HEAD。
4. P0-4：scanner symlink 泄漏。
5. P0-5：CLI pull 冲突 HEAD 语义与 GUI 对齐。
6. P3-4：CLI conflict resolve 递归、安全路径和父目录创建。

阶段 1 验证：

- `core/go test ./...`
- `cli/go test ./...`
- `gui/go test ./...`
- 手动或集成测试：构造恶意 snapshot 路径、冲突 pull、restore/revert 并发失败。

### 阶段 2：跨平台一致性

修复 Windows/macOS/Linux 语义差异和平台边界。

1. P1-1：路径大小写跨平台一致性和碰撞检测。
2. P1-2：Windows object cache 文件名。
3. P1-3：项目目录反解连字符。
4. P1-4：CLI binary prune 当前平台限制。
5. P1-5：Linux opener 错误可见。
6. P1-6：watcher errors channel。

阶段 2 验证：

- `core/go test ./...`
- `cli/go test ./...`
- `gui/go test ./...`
- Windows 本机验证 object cache。
- Linux 桌面/无桌面边界验证 opener。
- macOS/Linux 上验证 normalize 测试。

### 阶段 3：前端高风险误操作

修复页面崩溃、过期响应和同步状态误导。

1. P2-1：Projects 空数组/null。
2. P2-2：Files 详情/冲突/Diff 过期响应。
3. P2-3：History 详情过期响应和回滚确认。
4. P2-5：Binaries 上传早期失败和重复点击。
5. P2-6：Dashboard QuickSync 子操作状态覆盖。
6. P2-7：Files 远程刷新错误可见。
7. P2-9：加密密码指纹预览过期响应。
8. P2-10：Projects 详情过期响应。

阶段 3 验证：

- `gui/frontend/npm run build`
- `gui/go test ./...`
- 手动 UI 验证：快速点击、快速切换、远程失败、上传失败、回滚确认。

### 阶段 4：长期运行和并发边界

修复长期运行内存和多设备并发写问题。

1. P3-1：binary index CAS。
2. P3-2：opResults 清理。
3. P3-3：新设备初始化 TOCTOU。
4. P2-4：跨页面 dirty/refresh 统一机制。
5. P2-8：项目列表缓存刷新回传。

阶段 4 验证：

- `core/go test ./...`
- `cli/go test ./...`
- `gui/go test ./...`
- `gui/frontend/npm run build`
- 多设备/双客户端并发测试。
- 长时间 GUI 运行和多次自动同步 smoke test。

---

## 每次修复必须遵守的验证边界

1. 修改 `core` 同步、快照、对象、WebDAV、二进制、加密相关代码时，必须同时跑 `core`、`cli`、`gui` 的 Go 测试。
2. 修改 `gui/frontend/src` 时，必须跑 `npm run build`，并尽量启动 GUI 做交互验证。
3. 修改 CLI/GUI 共享语义时，必须说明两端是否一致，以及是否影响已有远程数据。
4. 修改远程写入逻辑时，必须考虑：
   - 远程写失败；
   - 本地写失败；
   - CAS 失败；
   - 多设备并发；
   - 操作中断后是否留下半成品。
5. 修改路径逻辑时，必须覆盖：
   - `../`；
   - 绝对路径；
   - Windows volume path；
   - 大小写碰撞；
   - symlink；
   - 嵌套目录。
6. 不修改 `gui/frontend/wailsjs/`、`node_modules/`、`dist/`、`build/`。
