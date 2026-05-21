# CC-Box 开发约定

本文件是 Claude Code 在本项目中的开发指令。执行任何代码修改、重构、修复或功能开发时，必须遵守以下约定。

本文件已内联 `CLI_GUI_DEVELOPMENT_REFERENCE.md` 的核心内容。后续开发时优先遵守本文件；如果需要更详细背景，可以再阅读 `CLI_GUI_DEVELOPMENT_REFERENCE.md`。

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

## Claude 执行提示

- 先判断修改属于手写源码、自动生成绑定、构建产物还是第三方依赖；不要把依赖或生成物当作业务代码处理。
- 默认只搜索必要范围：`core/**/*.go`、`cli/**/*.go`、`gui/*.go`、`gui/internal/**/*.go`、`gui/frontend/src/**` 和明确相关的文档。
- 不要对整个仓库或整个 `gui/` 做无约束全文搜索；除非用户明确要求，不搜索 `node_modules/`、`dist/`、`build/`。
- `gui/frontend/wailsjs/` 是 Wails 自动生成绑定；可以检查 diff，但不要当作手写业务源码主动修改。
- 构建或测试后先区分变更来源，再决定是否保留；不要把依赖目录和构建输出混入代码修改。

## GitHub 同步版本号提示词

当用户要求“同步到 GitHub”“推送到 GitHub”“发布”或类似操作时，按以下提示词执行：

- 先检查 `git status`、`git diff` 和最近提交，确认本次同步包含哪些改动，不要盲目提交。
- 每次同步必须带版本号。若用户没有指定版本号，先根据改动范围建议一个语义化版本号，并让用户可以改：
  - 仅修复 bug 或小体验问题：建议递增 patch，例如 `v0.1.1`。
  - 增加小功能或明显体验改进：建议递增 minor，例如 `v0.2.0`。
  - 破坏性变更或重大重构：建议递增 major，例如 `v1.0.0`。
- 提交信息和最终回复都要明确写出本次版本号。
- 对需要作为版本留痕的同步，提交后创建对应 Git tag，例如 `v0.1.1`，并同时推送代码和 tag。
- 不要静默编造版本号；如果版本范围不明确，先给出推荐版本和理由，再等待用户确认。
- 如果用户明确说只是普通备份同步、不发布版本，则可以只提交和推送，但仍要在最终回复中说明“未创建版本 tag”。

## 核心结构速记

- `core/` 是共享核心 module，只放 CLI/GUI 都需要的数据能力；不能依赖 `cli/`、`gui/`、Wails 或前端代码。
- `cli/` 依赖 `core/`，只放命令行入口、Cobra 命令层和 CLI 专属适配。
- `gui/` 依赖 `core/`，只放 Wails 桌面应用、前端绑定、托盘、窗口生命周期和 GUI 专属桌面适配。
- `cli/` 和 `gui/` 不互相依赖；需要共享的能力下沉到 `core/`。
- GUI 桌面平台差异集中在 `gui/internal/desktop`，例如文件打开、托盘、平台命令窗口隐藏。
- 平台专属代码优先用 `*_windows.go`、`*_darwin.go`、`*_linux.go`、`*_other.go` 和 build tags 隔离，不要在普通文件里直接引用 Windows-only 字段或桌面 API。
- Wails 绑定方法定义在 `gui` 主包的 `App` 方法中，前端通过 `gui/frontend/wailsjs/` 调用生成绑定。
- 当前明确不做开机自启动；不要重新引入 `AutoStartManager`、Startup 快捷方式、注册表 Run key、LaunchAgent 或 XDG autostart。

## 平台验证原则

- 当前在哪个平台运行，就优先验证该平台的真实构建、打包和桌面行为。
- Windows 环境优先验证 Windows Wails 构建、窗口/托盘、文件管理器打开和同步关键路径。
- macOS 环境优先验证 macOS Wails 构建、`.app` 启动、Finder reveal、托盘/菜单栏和窗口行为。
- Linux 环境优先验证 Linux Wails 构建、`xdg-open`/`gio open`、托盘可用性和窗口行为。
- 从非目标平台做 Linux/macOS/Windows 交叉编译只作为 build tags 和编译边界 smoke test；不能视为对应平台安装包或桌面交互已验证。
- 正式发布安装包应在目标平台本机或 CI matrix 中分别构建和验证。

## 总体策略

开发时采用“GUI 优先，但核心能力同步审查”的策略：

- 纯 GUI 体验：可以先只改 GUI。
- 核心数据能力：CLI 和 GUI 必须一起改、一起测。

核心原则是：CLI 和 GUI 虽然代码独立，但共享同一个本地状态和 WebDAV 远程数据。任何会影响共享数据读写、远程协议、加密、快照、同步语义或二进制版本管理的改动，都不能只改一端。

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

如果修改触及核心能力，就不要只改 GUI。必须同步考虑 CLI 的场景包括：

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

每次开发按以下流程执行：

1. 先判断修改范围。
   - 纯 GUI 展示、交互、托盘、Wails 绑定：可以优先只改 GUI。
   - 涉及核心数据能力：必须同时检查 CLI 和 GUI。

2. 修改 GUI 时检查是否触及核心能力。
   - 如果只改 `gui/app.go`、`gui/onboarding.go`、`gui/dashboard.go`、`gui/files.go`、`gui/pages.go`、`gui/watcher.go`、`gui/tray.go`、`gui/async.go` 中的展示或交互逻辑，通常可以只做 GUI。
   - 如果改到 `gui/internal/*`，必须检查 `cli/internal/*` 是否需要同等修改。

3. 修改完成后检查是否触及核心协议或共享数据。
   - 如果只是 GUI 体验层，只验证 GUI 即可。
   - 如果影响同步、加密、快照、WebDAV、二进制管理或本地状态，必须同步补 CLI。

4. 最后按修改范围验证关键路径。
   - GUI 体验层修改：验证 GUI 关键路径。
   - CLI/GUI 核心能力修改：同时验证 CLI 和 GUI 关键路径。
   - 涉及共享远程数据格式、加密、删除、恢复的修改，要优先保证兼容和可回滚。

## 判断规则

每次修改前后按以下问题判断是否需要同步 CLI：

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

## 简化原则

- 不要为了 GUI 修复引入不必要的 CLI 重构。
- 不要只改 GUI 的核心逻辑后忽略 CLI 对应实现。
- 不要改变 WebDAV、快照、对象、加密或二进制格式，除非同时更新 CLI/GUI 并明确验证兼容性。
- 优先做最小、直接、可验证的修改。
- 不要做超出当前任务需要的抽象、功能或兼容层。

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
