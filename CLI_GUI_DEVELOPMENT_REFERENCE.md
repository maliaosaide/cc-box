# CLI/GUI 协同开发参考

本文件用于判断 CC-Box 后续开发中，哪些修改可以先专注 GUI，哪些修改必须同步考虑 CLI。

## 当前结构结论

当前项目拆成两个独立 Go module：

- `cli/`：命令行应用，主要面向脚本化、终端使用和自动化流程。
- `gui/`：Wails 桌面应用，主要面向可视化同步、冲突处理、托盘和页面交互。

两者不是“GUI 调用 CLI 核心库”的关系，而是各自拥有一套 `internal/` 业务代码。

两边重复存在的核心包包括：

- `internal/config`
- `internal/crypto`
- `internal/object`
- `internal/snapshot`
- `internal/sync`
- `internal/webdav`
- `internal/binary`

因此，修改 `gui/internal/...` 不会自动修复或更新 `cli/internal/...`，反之亦然。

## 可以先只改 GUI 的情况

如果修改只影响 GUI 体验层，可以先专注 GUI，之后再判断 CLI 是否需要补充一致能力。

典型场景：

- 界面布局、按钮、文案、状态展示
- Wails 绑定方法
- 托盘行为
- onboarding 流程
- Dashboard 展示
- 文件列表、diff、冲突页面交互
- 自动同步触发体验
- 异步任务进度展示

常见文件：

- `gui/app.go`
- `gui/onboarding.go`
- `gui/dashboard.go`
- `gui/files.go`
- `gui/pages.go`
- `gui/watcher.go`
- `gui/tray.go`
- `gui/async.go`

这些改动通常不会改变 CLI 能否读写现有数据，也不会改变 WebDAV 远程协议。

## 必须 CLI/GUI 一起考虑的情况

如果修改触及核心能力，就不要只改 GUI。因为 CLI 和 GUI 虽然代码独立，但共享同一个本地状态和 WebDAV 远程数据。

必须同步考虑 CLI 的场景：

- 同步算法
- 冲突判定
- `HEAD` 更新逻辑
- WebDAV 路径和远程目录结构
- 快照 JSON 结构
- object hash 规则
- 加密格式
- 加密密码、salt、rekey
- 二进制版本 index、上传、下载、删除
- 本地 `~/.cc-box` 状态文件
- CLI 和 GUI 混用同一个 WebDAV 远程时的数据兼容

这些修改如果只先改 GUI，可能导致：

- GUI 写出的数据 CLI 读不懂。
- CLI 后续 push/pull 时覆盖 GUI 状态。
- 两边对冲突、删除、快照父节点的判断不一致。
- 加密、对象、二进制文件无法互通。

## 需要成对检查的核心包

修改以下 GUI 包时，通常要检查 CLI 对应包：

| GUI | CLI |
| --- | --- |
| `gui/internal/config` | `cli/internal/config` |
| `gui/internal/crypto` | `cli/internal/crypto` |
| `gui/internal/object` | `cli/internal/object` |
| `gui/internal/snapshot` | `cli/internal/snapshot` |
| `gui/internal/sync` | `cli/internal/sync` |
| `gui/internal/webdav` | `cli/internal/webdav` |
| `gui/internal/binary` | `cli/internal/binary` |

## 推荐开发流程

推荐采用“GUI 优先，但核心能力同步审查”的流程：

1. 先按产品体验专注修改 GUI。
2. 修改完成后检查是否触及核心协议或共享数据。
3. 如果只是 GUI 体验层，只验证 GUI 即可。
4. 如果影响同步、加密、快照、WebDAV、二进制管理或本地状态，必须同步补 CLI。
5. 最后同时验证 CLI 和 GUI 的关键路径。

## 判断规则

每次修改后按以下问题判断是否需要同步 CLI：

1. 是否改了 `gui/internal/*`？
   - 是：检查 `cli/internal/*` 是否需要同等修改。
2. 是否改变 WebDAV 上的路径、文件名、JSON 字段、加密格式或 hash 规则？
   - 是：CLI/GUI 必须一起改。
3. 是否改变 `HEAD` 更新、冲突判定、pull/push 语义？
   - 是：CLI/GUI 必须一起改。
4. 是否只改 GUI main package 的展示、绑定、异步、托盘？
   - 是：通常可以先只改 GUI。
5. 用户是否可能混用 GUI 和 CLI 操作同一个 WebDAV 根路径？
   - 是：核心能力必须保持一致。
6. 是否涉及安全、加密、恢复或删除？
   - 是：CLI/GUI 必须一起审查。

## 长期架构建议

当前 CLI 和 GUI 复制了大量核心业务代码，后续容易出现行为漂移。长期更合理的结构是抽出共享核心 module：

```text
core/
  config/
  crypto/
  object/
  snapshot/
  sync/
  webdav/
  binary/

cli/
  只保留 Cobra 命令层

gui/
  只保留 Wails、前端、托盘和页面绑定
```

这样同步、加密、快照、WebDAV、二进制管理等能力只需要维护一份，CLI 和 GUI 都调用同一个 core，避免人工同步两份代码。

## 短期结论

短期可以继续以 GUI 为主推进开发，但要遵守一个边界：

- 纯 GUI 体验：可以先只改 GUI。
- 核心数据能力：CLI 和 GUI 必须一起改、一起测。

这能在保证开发效率的同时，降低 CLI/GUI 行为不一致和数据不兼容的风险。
