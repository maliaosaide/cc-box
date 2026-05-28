# Claude binary 一键安装与版本控制流程设计

本文只讨论 CC-Box **二进制管理界面**里的 Claude binary 安装、切换、恢复和版本控制流程。

这里的“一键安装”只指安装或恢复当前受支持平台的 `claude` / `claude.exe` 本体，不包含 onboarding，不包含 WebDAV 同步组初始化，也不包含完整恢复 `~/.claude/`、`~/.claude.json` 等用户配置。

## 核心结论

CC-Box 的 Claude binary 管理不是另起一套私有安装体系，而是服务于 Claude 官方 native install 布局。

因此，GitHub Releases 和 WebDAV 备份版本最终都必须安装到官方命令入口：

```text
Windows: ~/.local/bin/claude.exe
macOS/Linux: ~/.local/bin/claude
```

这个路径就是用户终端里通过 `PATH` 执行 `claude` 命令时命中的入口。

官方安装入口只负责安装最新版；GitHub Releases 和 WebDAV 备份才是 CC-Box 做 Claude 版本控制的核心来源。

## 范围

二进制管理界面负责：

1. 安装当前受支持平台可用的 Claude binary。
2. 选择并切换指定 Claude 版本。
3. 从 WebDAV 备份版本恢复已备份 binary。
4. 从 GitHub Releases 安装指定历史版本。
5. 执行 Claude 官方文档安装命令安装最新版。
6. 在全新电脑上创建必要本地目录。
7. 默认执行所选 `claude(.exe) install`，让初始化行为与官方 native install 对齐。
8. 确保最终 `~/.local/bin/claude(.exe)` 是用户选择的版本。
9. 安装失败时不留下半成品，不错误标记为已安装。

二进制管理界面不负责：

- 初始化 CC-Box 同步组。
- 写入 `~/.cc-box/key.bin`、`salt.bin`、`HEAD`。
- 恢复 `~/.claude/` 用户配置目录。
- 恢复或创建 `~/.claude.json`。
- 安装 `uv`、`uvx`、`codex`、`gemini` 或其他工具。
- 跨平台安装 binary。
- 替代 Claude 官方安装器的完整逻辑。

## 当前支持平台

当前 binary 安装只支持三类主流平台：

| 平台 | GitHub asset | 说明 |
| --- | --- | --- |
| Windows x64 | `claude-win32-x64.zip` | 当前 Windows 主目标 |
| macOS Apple Silicon | `claude-darwin-arm64.tar.gz` | 面向 Mac M 系列芯片 |
| Linux x64 | `claude-linux-x64.tar.gz` | 面向主流 Linux x64 环境 |

当前不支持：

- Windows ARM64：`claude-win32-arm64.zip`
- macOS Intel：`claude-darwin-x64.tar.gz`
- Linux ARM64：`claude-linux-arm64.tar.gz`
- Linux musl：`claude-linux-x64-musl.tar.gz`、`claude-linux-arm64-musl.tar.gz`

`musl` 是 Alpine Linux 等发行版使用的 C 标准库变体。它需要与常见 glibc Linux 区分选择，因此不纳入当前主线支持范围。

未来如果扩展 binary 平台支持，应先补齐平台检测、asset 选择、安装测试和 UI 标识。binary 平台扩展理论上不应改变 `~/.claude/`、`~/.claude.json` 的配置备份语义，但配置跨平台迁移仍需单独验证后才能声明支持。

## 官方 native install 布局

Claude 官方 native install 使用的核心路径是：

```text
~/.local/bin/claude(.exe)
~/.local/share/claude/
~/.local/share/claude/versions/
~/.claude/
~/.claude.json
```

其中：

- `~/.local/bin/claude(.exe)` 是终端 `PATH` 命中的命令入口。
- `~/.local/share/claude/` 是官方 native install 使用的数据目录。
- `~/.local/share/claude/versions/` 是本地版本目录。
- `~/.claude/` 是 Claude 用户配置和状态目录。
- `~/.claude.json` 是 Claude 用户级配置文件。

CC-Box 的 binary 管理只直接管理 `claude(.exe)` 本体及其版本记录。

`~/.claude/` 和 `~/.claude.json` 属于配置备份/还原范围，不属于二进制管理页面单独安装时的写入目标。

## 官方安装命令

官方最新版安装入口必须直接执行 Claude 官方文档推荐命令。

Windows 优先使用 CMD 命令，因为 CMD 可用性更稳定，受 PowerShell 执行策略影响更小：

```text
curl -fsSL https://claude.ai/install.cmd -o install.cmd && install.cmd && del install.cmd
```

如果 CMD 官方安装失败，可以 fallback 到 PowerShell 官方命令：

```text
irm https://claude.ai/install.ps1 | iex
```

macOS / Linux 使用：

```text
curl -fsSL https://claude.ai/install.sh | bash
```

官方安装入口的语义是：

- 只安装官方最新版。
- 不提供历史版本选择。
- 不作为 CC-Box 版本控制主路径。
- 不由 CC-Box 复刻官方下载、校验、launcher、PATH 初始化逻辑。
- 安装完成后，CC-Box 只清理检测缓存并重新检测当前 `claude` 路径、版本和来源。

## 官方安装器负责什么

官方安装脚本会先创建临时下载目录：

```text
~/.claude/downloads/
```

然后下载官方最新版 manifest、checksum 和临时 `claude(.exe)`。

随后官方脚本会执行：

```text
claude(.exe) install
```

真正初始化官方 native install 布局的关键动作来自 `claude(.exe) install`，包括：

- 创建或更新 `~/.local/bin/claude(.exe)`。
- 创建或更新 `~/.local/share/claude/`。
- 创建或更新本地 versions 目录。
- 初始化 launcher / shell integration / PATH 相关行为。
- 初始化 Claude 运行所需的本地结构。

因此，官方脚本本身不是 CC-Box 版本控制的主路径。它的定位只是“安装最新版”。

## 三类安装来源

二进制管理界面应清晰区分三类来源：

```text
WebDAV 备份版本
GitHub Releases
官方最新版安装
```

### 1. WebDAV 备份版本

WebDAV 来源表示用户之前通过 CC-Box 上传过的 Claude binary。

语义：

- 用于精确恢复用户已备份版本。
- 用于快照恢复和跨设备复现。
- 不依赖 GitHub Releases 或官方安装源可用性。
- 只安装当前平台下的 `claude` / `claude.exe`。
- 是用户自己已经保存过的版本。

安装流程：

```text
选择 WebDAV 版本
→ 下载 WebDAV payload
→ 解密（如启用）
→ 校验 hash
→ 写入临时验证文件
→ 校验 binary 版本
→ 确保官方本地目录存在
→ 备份旧 ~/.local/bin/claude(.exe)
→ 原子写入所选版本到 ~/.local/bin/claude(.exe)
→ 默认执行 ~/.local/bin/claude(.exe) install
→ 再次原子写入所选版本到 ~/.local/bin/claude(.exe)
→ 再次校验最终命令入口版本
→ 记录来源为 webdav
→ 清理检测缓存
→ 刷新当前 Claude 状态
```

这里执行 `install` 的 binary 是刚刚写入 `~/.local/bin/` 的用户所选版本，而不是官方最新版安装脚本下载的 latest。

如果这是完整备份还原流程的一部分，应先用 `claude(.exe) install` 创建官方目录，再覆盖恢复 `~/.claude/` 和 `~/.claude.json` 等配置数据。

### 2. GitHub Releases

GitHub Releases 来源表示 Claude 官方在 GitHub 上发布的可选历史版本。

这是 CC-Box Claude 版本控制能力的核心外部来源。

语义：

- 用户可以选择具体版本安装。
- 可用于安装历史版本、回退版本、测试指定版本。
- 不应被官方最新版安装逻辑替代。
- 不应静默安装其他版本。
- 必须由用户显式点击安装指定版本。

当前主线支持的 GitHub asset 是：

```text
claude-win32-x64.zip
claude-darwin-arm64.tar.gz
claude-linux-x64.tar.gz
SHASUMS256.txt
SHASUMS256.txt.sig
```

安装流程：

```text
选择 GitHub Release 版本
→ 根据当前平台筛选 asset
→ 下载 zip / tar.gz
→ 下载 SHASUMS256.txt
→ 下载并校验 SHASUMS256.txt.sig（具备可信验签链时）
→ 校验压缩包 SHA256
→ 解压 claude / claude.exe
→ 写入临时验证文件
→ 校验 binary 版本
→ 确保官方本地目录存在
→ 备份旧 ~/.local/bin/claude(.exe)
→ 原子写入所选版本到 ~/.local/bin/claude(.exe)
→ 默认执行 ~/.local/bin/claude(.exe) install
→ 再次原子写入所选版本到 ~/.local/bin/claude(.exe)
→ 再次校验最终命令入口版本
→ 记录来源为 github
→ 清理检测缓存
→ 刷新当前 Claude 状态
```

GitHub 版本安装不应该执行官方安装脚本，也不应该调用官方最新版安装入口。

尤其不能用：

```text
irm https://claude.ai/install.ps1 | iex
curl -fsSL https://claude.ai/install.sh | bash
```

来替代 GitHub 指定版本安装，否则会破坏“安装用户选择版本”的语义。

### 3. 官方最新版安装

官方安装来源表示直接执行 Claude 官方文档推荐安装命令。

语义：

- 只作为“安装官方最新版”的快捷入口。
- 不参与历史版本选择。
- 不用于精确恢复。
- 不用于 WebDAV 快照复现。
- 不由 CC-Box 接管下载、校验和 launcher 初始化细节。
- 安装完成后只重新检测当前 `claude`。

安装流程：

```text
点击一键安装官方最新版
→ Windows 优先执行官方 CMD 安装命令
→ CMD 失败时 fallback 到官方 PowerShell 安装命令
→ macOS/Linux 执行官方 shell 安装命令
→ 官方脚本下载最新版临时 binary
→ 官方脚本执行 claude(.exe) install
→ 官方安装器初始化 native install 布局
→ CC-Box 清理检测缓存
→ CC-Box 重新检测当前 claude 路径和版本
→ 记录来源为 official
→ 刷新当前 Claude 状态
```

官方安装入口不提供版本选择。如需指定版本，必须使用 GitHub Releases 或 WebDAV 备份版本。

官方安装完成后不需要弹出“是否上传到 WebDAV”的额外提示。页面应直接刷新本地当前版本，让这个本地版本出现在二进制管理界面中，并提供明确的上传入口。

## 本地目录创建

GitHub/WebDAV 指定版本安装不能假设全新电脑上已经存在官方目录。

安装前应显式确保以下目录存在：

```text
~/.local/bin/
~/.local/share/claude/
~/.local/share/claude/versions/
```

Windows 对应：

```text
%USERPROFILE%\.local\bin\
%USERPROFILE%\.local\share\claude\
%USERPROFILE%\.local\share\claude\versions\
```

建议新增共享函数：

```go
func EnsureClaudeInstallDirs() error
```

职责：

```text
MkdirAll(config.LocalBinDir(), 0755)
MkdirAll(filepath.Dir(config.VersionsDir()), 0755)
MkdirAll(config.VersionsDir(), 0755)
```

这一步是 CC-Box 自己的显式目录保障，不应依赖 `WriteFileAtomic`、备份逻辑或官方安装器的副作用。

## 全新电脑上的初始化逻辑

全新电脑可能没有：

```text
~/.local/bin/
~/.local/share/claude/
~/.local/share/claude/versions/
~/.claude/
~/.claude.json
```

对于官方最新版安装，这些目录由官方安装脚本和 `claude(.exe) install` 自己处理。

对于 GitHub/WebDAV 指定版本安装，CC-Box 应默认执行所选版本的 `install` 子命令，让初始化行为与官方路径模型对齐。

关键点是：

```text
执行 install 的文件 = ~/.local/bin/claude(.exe)
```

也就是说，流程不是拿一个随意临时路径去初始化，而是：

```text
1. 下载或恢复用户选择的 claude(.exe)
2. 校验该 binary 版本
3. 创建 ~/.local/bin/ 等目录
4. 将该 binary 写入 ~/.local/bin/claude(.exe)
5. 执行 ~/.local/bin/claude(.exe) install
6. 再次把用户选择的 binary 覆盖回 ~/.local/bin/claude(.exe)
7. 校验 ~/.local/bin/claude(.exe) 最终版本
```

第 6 步必须保留，因为 `install` 子命令可能创建 launcher、写版本目录或调整入口文件。CC-Box 的最终不变量必须是：

```text
~/.local/bin/claude(.exe) 的版本 == 用户选择的 GitHub/WebDAV 版本
```

如果这个不变量无法满足，安装必须失败，不能标记为已安装。

## GitHub/WebDAV 受管安装事务

GitHub/WebDAV 应共用同一套受管安装逻辑。

输入：

```text
binary data
expected version
source: github | webdav
```

输出：

```text
安装结果
最终路径
最终版本
来源记录
命令状态
PATH 状态
warning
```

推荐事务顺序：

```text
1. 解析目标路径：~/.local/bin/claude(.exe)
2. 将所选 binary 写入临时验证文件
3. 设置可执行权限
4. 执行临时文件 --version
5. 校验版本等于 expected version
6. 确保官方本地目录存在
7. 读取旧目标文件用于失败恢复
8. 备份旧目标文件
9. 原子写入所选 binary 到 ~/.local/bin/claude(.exe)
10. 执行 ~/.local/bin/claude(.exe) install
11. 再次原子写入所选 binary 到 ~/.local/bin/claude(.exe)
12. 清理 Claude resolution cache
13. 检测最终 ~/.local/bin/claude(.exe) 版本
14. 校验最终版本仍等于 expected version
15. 记录来源 github/webdav
16. 检测命令状态
17. 如 auto_configure_path 开启，则尝试配置 PATH
18. 返回安装结果和 warning
```

失败处理：

- 临时文件版本校验失败：不能替换目标文件。
- 备份失败：不能替换目标文件。
- 首次原子写入失败：尽量恢复旧文件。
- `install` 子命令失败：尽量恢复旧文件，并返回失败。
- 最终写回失败：尽量恢复旧文件，并返回失败。
- 最终版本校验失败：尽量恢复旧文件，并返回失败。
- PATH 配置失败：不回滚安装，只返回 warning。
- Windows 文件占用：不强杀进程，只提示关闭相关进程后重试。

## 目标路径策略

默认目标路径应是官方命令入口：

```text
Windows: ~/.local/bin/claude.exe
macOS/Linux: ~/.local/bin/claude
```

也就是：

```go
binary.GetBinaryPath("claude")
```

在未显式配置 `binary.bin_dir` 或 `binary.claude_path` 时，`GetBinaryPath("claude")` 应解析到官方 `.local/bin` 路径。

如果用户显式配置了自定义路径，应遵守配置，但 UI 需要明确展示当前安装目标不是官方默认路径。

## shim 场景

正常情况下，`~/.local/bin/claude(.exe)` 应是官方 native binary，不应是 npm shim、脚本 shim 或其他包管理器包装文件。

如果检测到 shim，不应静默绕到私有路径并让用户误以为已经覆盖官方命令入口。

推荐策略：

```text
默认目标仍然是 ~/.local/bin/claude(.exe)
```

如果检测到 shim，应在二进制管理页面提示用户：

```text
当前 claude 命令入口看起来是脚本或 shim。
如果继续安装，CC-Box 将把它替换为官方 native binary。
```

可选策略：

1. 用户确认后替换 shim。
2. 用户取消安装。
3. 提示用户先卸载 npm/shim 版本后重试。

不推荐静默回退到：

```text
~/.cc-box/bin/claude(.exe)
```

因为这会偏离“CC-Box 服务官方 Claude native install 布局”的产品语义，也可能导致用户终端实际执行的 `claude` 仍然不是 CC-Box 管理的版本。

## PATH 状态

安装 binary 和激活 shell 命令是两件事。

状态应区分：

```text
activated
installed_not_activated
shadowed_by_other_binary
not_installed
```

含义：

- `activated`：终端 `claude` 命中的就是当前目标路径。
- `installed_not_activated`：目标路径已安装，但当前 shell 还不会命中它。
- `shadowed_by_other_binary`：目标路径已安装，但 PATH 中有更靠前的其他 `claude`。
- `not_installed`：目标路径不存在或不可用。

当：

```text
binary.auto_configure_path = false
```

应只提示用户，不修改 PATH 或 shell profile。

当：

```text
binary.auto_configure_path = true
```

可以尝试配置用户级 PATH / shell profile。

PATH 配置失败只作为 warning，不回滚 binary 安装。

官方安装脚本自己的 PATH 行为不受 `binary.auto_configure_path` 约束，因为那属于官方安装器行为，不是 CC-Box 的 PATH 写入逻辑。

## 配置备份/还原与 binary 安装的关系

`claude(.exe) install` 初始化后的用户配置范围主要包括：

```text
~/.claude/
~/.claude.json
```

这些路径属于 CC-Box 配置备份/还原机制的范围，而不是二进制管理页面单独安装 binary 时的写入目标。

正确关系是：

```text
官方 install 子命令负责初始化 Claude 可运行的本地布局
CC-Box binary 管理负责安装/切换 ~/.local/bin/claude(.exe)
CC-Box 配置恢复负责覆盖 ~/.claude/ 和 ~/.claude.json
```

因此，在完整恢复场景中，合理顺序是：

```text
1. 安装或恢复 claude(.exe)
2. 执行 ~/.local/bin/claude(.exe) install 初始化官方 native install 布局
3. 再次覆盖 ~/.local/bin/claude(.exe)，确保版本仍是用户选择版本
4. 恢复 ~/.claude/
5. 恢复 ~/.claude.json
6. 重新检测 claude 命令状态
```

但二进制管理页面的一键安装只执行第 1、2、3、6 步。

## 校验策略

WebDAV 备份版本应校验：

```text
payload hash
解密结果（如启用加密）
临时 binary --version
最终 ~/.local/bin/claude(.exe) --version
```

GitHub Releases 版本应校验：

```text
SHASUMS256.txt 中的 asset SHA256
SHASUMS256.txt.sig 签名（具备可信验签链时）
临时 binary --version
最终 ~/.local/bin/claude(.exe) --version
```

如果暂时没有可信公钥或验签链，不能把“下载了 `.sig` 文件”描述为已经完成签名校验。UI 或日志应明确区分：

```text
已完成 SHA256 校验
已完成签名校验
未执行签名校验
```

## UI 结构建议

二进制管理页面保留三个清晰来源区：

```text
WebDAV 备份
GitHub Releases
官方安装
```

### WebDAV 备份区

按钮文案：

```text
安装此版本
切换到此版本
上传当前本地版本
删除版本
```

展示信息：

```text
版本号
平台
备份时间
大小
hash
是否当前版本
来源状态
```

删除语义应是统一删除该版本，不拆成本地删除和云端删除两个用户概念。

### GitHub Releases 区

按钮文案：

```text
刷新版本列表
安装此版本
```

展示信息：

```text
版本号
发布时间
当前平台 asset
大小
是否来自缓存
错误信息 / 重试入口
```

GitHub Releases 只展示当前受支持平台可安装版本。

当前不展示 ARM、Mac Intel、Linux musl 等非当前支持范围的版本。

### 官方安装区

按钮文案：

```text
一键安装官方最新版
```

说明文案必须明确：

```text
官方安装只安装最新版；如需指定版本，请使用 GitHub Releases 或 WebDAV 备份版本。
```

官方安装区不应展示历史版本选择。

官方安装完成后，当前本地版本应直接刷新到页面里，并提供“上传当前本地版本”入口，而不是弹窗打断用户。

## 实现核对点

当前实现方向应满足：

- 官方最新版安装入口直接执行官方文档命令。
- Windows 官方安装优先 CMD，失败后 fallback PowerShell。
- GitHub Releases 来源是真实版本来源，不是占位。
- GitHub Releases 当前只支持 Windows x64、Mac M 系列、Linux x64。
- WebDAV 备份版本可以恢复用户已备份 binary。
- GitHub/WebDAV 安装前校验版本。
- GitHub asset 使用 `SHASUMS256.txt` 校验 SHA256。
- 具备可信验签链时校验 `SHASUMS256.txt.sig`。
- 安装前备份旧 binary。
- 写入失败尽量恢复旧文件。
- 默认执行 `~/.local/bin/claude(.exe) install` 初始化官方布局。
- 执行 install 后再次覆盖所选版本。
- 最终校验 `~/.local/bin/claude(.exe)` 仍是用户选择版本。
- PATH 配置为 best-effort warning。
- 安装后清理 Claude 检测缓存并重新检测状态。

仍需要重点确认或完善：

1. 新增 `EnsureClaudeInstallDirs()`，显式创建 `.local/bin` 和 `.local/share/claude/versions`。
2. GitHub/WebDAV 安装事务中加入默认执行 `~/.local/bin/claude(.exe) install`。
3. 执行 `install` 子命令后，必须再次覆盖并校验最终 `~/.local/bin/claude(.exe)`。
4. shim 场景不应静默回退到 `.cc-box/bin`。
5. GitHub 平台映射收窄为 Windows x64、Mac M 系列、Linux x64。
6. Linux musl 暂不支持；如未来支持，需要单独检测 glibc/musl。
7. 补充 `SHASUMS256.txt.sig` 签名校验能力或明确显示未执行签名校验。
8. Windows 官方安装改为 CMD 优先、PowerShell fallback。
9. 官方安装完成后直接刷新本地版本，并在 WebDAV/本地版本区提供上传入口。

## 一句话原则

官方安装负责最新版和官方布局初始化；GitHub Releases 和 WebDAV 负责版本控制；CC-Box 默认用 `~/.local/bin/claude(.exe) install` 对齐官方布局，并最终确保 `~/.local/bin/claude(.exe)` 仍然是用户选择的版本。