# CC-Box - 跨平台 Claude Code 配置箱

> 配置同步 + 二进制备份 + 版本管理，一个工具搞定 Claude Code 的多设备管理。

## 项目定位

跨平台（Windows / macOS / Linux）的 Claude Code 管理工具。用类似 git 的版本化方式管理 `~/.claude/` 配置和 `~/.local/` 二进制文件，通过 WebDAV 协议在多设备间双向同步。

与现有方案的对比：

| 维度 | cc-cli | claude-sync | claudecode-local-sync | **CC-Box** |
|------|--------|-------------|----------------------|------------|
| 同步协议 | WebDAV | R2/S3/GCS | GitHub | **WebDAV** |
| 版本管理 | 无 | 无 | git 历史 | **类 git 快照链** |
| 跨平台 | 部分 | 是 | 是 | **是** |
| 端到端加密 | 无 | age | 无 | **AES-256-GCM** |
| cc-switch 兼容 | 自己就是 | 冲突 | 冲突 | **设计级兼容** |
| 二进制备份 | 无 | 无 | 无 | **版本备份与切换** |
| 项目会话同步 | 无 | 有 | 有 | **预留接口** |

核心差异：**git 式版本快照 + 二进制版本管理 + WebDAV + cc-switch 兼容**。不是简单的文件上传下载，而是每次同步产生一个快照（类似 commit），支持历史回溯和三方合并。

## 同步范围

### 二进制与版本管理（核心）

```
~/.local/
├── bin/
│   ├── claude[.exe]            # Claude 主程序 (~242MB)
│   ├── uv[.exe]                # Python 包管理器 (~65MB)
│   ├── uvx[.exe]               # Python 工具运行器 (~340KB)
│   └── uvw[.exe]               # Python wrapper (~340KB)
└── share/claude/versions/
    ├── 2.1.126                 # 历史版本存档 (~243MB)
    ├── 2.1.84                  # (~234MB)
    └── 2.1.81                  # (~232MB)
```

CC-Box 的二进制管理策略：
- **版本备份**：将 `~/.local/bin/claude` 和历史版本上传到 WebDAV（压缩），加密和分块策略可配置
- **按哈希去重**：相同版本（同平台）只存一份，多设备共享
- **版本切换**：`cc-box binary switch 2.1.84` 一键切换 Claude 版本
- **新设备恢复**：pull 时自动下载当前版本到 `~/.local/bin/`，无需重新安装
- **平台感知**：Windows 下载 `.exe`，macOS/Linux 下载对应二进制。不同平台的同名版本各自独立存储
- **版本清理**：`cc-box binary prune` 清理不再被任何设备引用的历史版本

### 配置文件同步

```
~/.claude/
├── settings.json              # 全局设置（API 配置、权限、环境变量）
├── CLAUDE.md                  # 全局指令
├── keybindings.json           # 快捷键绑定（如存在）
├── skills/                    # 自定义技能
│   └── <skill-name>/
│       ├── SKILL.md
│       └── ...
├── commands/                  # 自定义命令（如存在）
├── agents/                    # 自定义 Agent（如存在）
├── rules/                     # 自定义规则（如存在）
├── hooks/                     # 钩子脚本（如存在）
├── plugins/
│   ├── installed_plugins.json # 已安装插件列表
│   ├── known_marketplaces.json
│   └── blocklist.json
└── history.jsonl              # 命令历史（内容追加合并，详见合并策略）
```

### 项目级配置（按项目独立同步）

```
<project-root>/
└── .claude.json               # MCP server 配置、项目权限
```

### 预留但暂不同步

```
projects/                      # 会话数据 - 预留接口，Phase 4 实现
memory/                        # 持久化记忆 - 同上
```

### 永远排除

```
.credentials.json              # OAuth token
settings.local.json            # 机器相关权限（不同步）
sessions/                      # IDE 进程状态
cache/                         # 可重建
debug/                         # 调试日志
telemetry/                     # 遥测
downloads/                     # 下载缓存
paste-cache/                   # 粘贴缓存
shell-snapshots/               # Shell 快照
file-history/                  # 文件历史
session-env/                   # 会话环境
ide/                           # IDE 锁文件
backups/                       # Claude 自身备份
plans/                         # 计划文件（临时）
tasks/                         # 任务文件（临时）
teams/                         # 团队数据
plugins/data/                  # 插件运行数据（可能很大）
stats-cache.json               # 统计缓存
*.lock                         # 锁文件
```

## 技术选型

### 语言：Go

| 选项 | 优势 | 劣势 |
|------|------|------|
| Go | 单二进制、交叉编译成熟、标准库覆盖 HTTP/加密/文件、开发快 | 二进制比 Rust 稍大 |
| Rust | 最小二进制、性能最优 | 开发速度慢、学习曲线陡 |
| TypeScript | 生态丰富 | 需要 Node.js 运行时 |

选 Go 的理由：
- `GOOS=windows GOARCH=amd64 go build` 一行搞定交叉编译
- 标准库自带 `crypto/aes`、`crypto/sha256`、`net/http`，外部依赖少
- 交叉编译覆盖 Win/Mac/Linux（含 ARM），用户零配置
- 开发速度比 Rust 快 2-3 倍，对于 CLI 工具性能完全够用

### 核心依赖

```
cobra            # CLI 框架（kubectl/docker 同款）
viper            # 配置管理（TOML/JSON/环境变量）
reqwest          # HTTP 客户端（WebDAV 操作，支持 ETag/条件请求）
quick-xml        # WebDAV XML 解析
age              # 端到端加密（与 age 工具兼容）
argon2id         # 密码派生密钥
sha256           # 文件指纹
gosoft/gkeyring  # 系统密钥环（macOS Keychain / Linux Secret Service / Windows Credential Manager）
bubbletea        # TUI 界面（仅 init 交互式向导使用，非运行时依赖）
```

以下依赖仅在 GUI 模式使用（Phase 3 引入）：
```
Wails v2         # GUI 框架（Go 后端 + Web 前端）
fsnotify         # 文件变更监听（仅 GUI 自动同步模式使用）
```

## 核心设计：Git 式同步模型

### 概念映射

| Git 概念 | Claude Sync 对应 | 说明 |
|----------|-----------------|------|
| repository | `~/.cc-box/` | 本地同步仓库 |
| commit | snapshot | 文件状态的快照 |
| blob | object | 文件内容的加密存储单元 |
| tree | file_index | 快照中的文件目录树 |
| ref/HEAD | latest | 当前最新快照指针 |
| remote | WebDAV | 远程存储后端 |
| .gitignore | exclude | 排除规则 |

### 跨平台规范化

在计算文件哈希和执行差异比较前，先对文件内容和路径做规范化，避免因平台差异产生虚假变更：

| 维度 | 问题 | 规范化规则 |
|------|------|-----------|
| 换行符 | Windows CRLF vs Unix LF | 文本文件统一计算 LF 换行的哈希 |
| 路径分隔符 | `\` vs `/` | 快照中统一使用 `/` |
| 大小写 | Windows 不区分大小写 | 快照中文件路径统一小写存储 |
| 文件权限 | Unix 有执行位，Windows 无 | 快照中不记录权限位，只记录可执行标志 |

规范化仅影响哈希计算和差异比较——不会修改用户本地文件的实际内容。换行符统一通过扫描时做 `\r\n` → `\n` 转换后计算哈希。

### 快照（Snapshot）结构

每个快照记录一个时间点的所有已同步文件的状态：

```json
{
  "id": "snap_a1b2c3d4",
  "parent": "snap_e5f6g7h8",
  "timestamp": "2026-05-07T14:30:00Z",
  "device": "win-pc-abc123",
  "message": "auto sync",
  "files": {
    "settings.json": {
      "hash": "sha256:abc123...",
      "size": 2048,
      "modified": "2026-05-07T14:00:00Z"
    },
    "claude.md": {
      "hash": "sha256:def456...",
      "size": 4096,
      "modified": "2026-05-07T13:00:00Z"
    }
  },
  "binary": {
    "windows-amd64": {
      "claude": "2.1.126",
      "uv": "0.6.14"
    },
    "darwin-arm64": {
      "claude": "2.1.126"
    }
  }
}
```

**`binary` 字段说明**：记录该快照产生时各平台的当前二进制版本。此字段不强制上传二进制文件到云端，只是建立快照与二进制版本的关联，使得 log/history 中能展示"这次同步时使用了哪个版本"。

快照链：`snap_001 → snap_002 → snap_003 → ... → snap_latest`

每次 push 产生新快照，链接到上一个。支持沿链回溯任意历史版本。

### WebDAV 并发控制（乐观锁）

WebDAV 的 PUT 操作不提供原子 compare-and-swap。两台设备同时 push 时，后到的 PUT 会静默覆盖先到的 HEAD 指针，导致快照丢失。解决方案：**基于 ETag 的乐观锁**。

```
HEAD 指针格式：
  文件内容: snap_a1b2c3d4
  HTTP 响应头: ETag: "abc123..."

push 时更新 HEAD 的流程:
  1. GET /cc-box/HEAD → 获取当前 ID + ETag 值
  2. 创建新快照，上传 objects + snapshot
  3. PUT /cc-box/HEAD, 带上 If-Match: "abc123..."
     - 200 → 成功，更新本地 latest
     - 412 Precondition Failed → 远程已被其他设备更新
       4a. 重新下载远程最新快照
       4b. 将本地刚上传的快照 parent 指向远程最新（重写快照元数据）
       4c. 再次尝试 PUT HEAD（重复步骤 1-3），最多重试 3 次

并发安全保证：
  - WebDAV 服务端支持 ETag 和 If-Match 时：完全保证不丢失
  - 部分 WebDAV 服务端不支持 ETag（如 rclone serve 简化模式）：
    降级为无保护模式，记录警告日志。对于个人工具场景可接受。
  - 坚果云、NextCloud、Synology 均支持 ETag，覆盖主流场景
```

### 同步流程

#### push（推送）

```
cc-box push [-m "message"] [--dry-run]

1. 扫描本地文件（应用排除规则 + 跨平台规范化）
2. 计算每个文件的规范化 sha256 哈希
3. 与本地最新快照对比，找出变更文件:
   - 新增 (A)  - 本地有，快照无
   - 修改 (M)  - 哈希不同
   - 删除 (D)  - 快照有，本地无
4. 如无变更，提示 "nothing to push"
5. 加密变更文件内容 → 生成 object，上传到 WebDAV
6. 读取当前各平台二进制版本 → 写入快照 binary 字段
7. 创建新快照，parent 指向本地最新
8. 上传新快照到 WebDAV
9. 乐观锁更新远程 HEAD（GET HEAD → ETag → PUT with If-Match）
   - 冲突时合并重试（最多 3 次）
10. 更新本地 HEAD 指针
```

#### pull（拉取）

```
cc-box pull [--dry-run] [--force]

1. 从 WebDAV 下载远程 HEAD 指针
2. 与本地 HEAD 对比:
   - 相同 → "already up to date"
   - 不同 → 需要同步
3. 找出本地和远程的分叉点（共同祖先快照）
4. 如果找不到共同祖先（见降级策略），降级处理
5. 三方差异计算:
   ancestor = 分叉点快照
   local    = 本地当前文件状态
   remote   = 远程最新快照的文件状态
6. 对每个文件执行三方合并（按文件类型）:
   - 仅本地变 → 保留本地
   - 仅远程变 → 采纳远程
   - 双方都变 → 冲突，需解决
   - 双方都删 → 删除
7. 下载需要的 objects，解密后应用合并结果
8. 直接覆盖本地文件（云端已有备份，不做本地额外备份）
9. 创建合并快照，parent 指向远程最新
10. 上传合并快照 + 乐观锁更新 HEAD
```

#### 三方合并策略（按文件类型）

不同文件类型采用不同的合并策略：

**文本文件（CLAUDE.md、SKILL.md 等）**：
- 行级 diff 合并，类似于 `git merge-file`
- 冲突时插入 `<<<<<<< local` / `=======` / `>>>>>>> remote` 标记
- 用户通过 `cc-box resolve <file>` 交互式解决

**JSON 文件（settings.json、keybindings.json）**：
- 字段级结构化合并（详见 cc-switch 兼容设计）
- 普通 JSON 文件（非 settings.json）：同名 key 远程优先，新增 key 并集

**目录（skills/、commands/、agents/ 等）**：
- 递归到目录内每个文件做三方合并
- 仅一方新增的文件：直接采纳
- 仅一方删除的文件：标记删除
- 双方都修改的文件：进入文件级合并

**history.jsonl**（特殊处理）：
- 不做标准三方合并。按行内容去重追加：
  1. 取 ancestor → local 的新增行（本地独有的历史）
  2. 取 ancestor → remote 的新增行（远程独有的历史）
  3. 合并去重后追加到本地文件末尾
  4. 去重依据：`command + timestamp` 组合键。相同命令且同一分钟内执行的视为重复

#### 祖先不可达降级策略

当 GC 或其他原因导致本地和远程找不到共同祖先快照时：

```
降级判断:
  遍历本地快照链和远程快照链，查找第一个公共快照 ID
  如果遍历深度超过 50 个快照仍未找到 → 视为祖先不可达

降级行为:
  1. 提示用户: "无法找到共同祖先，将使用文件级双向合并"
  2. 不对单个文件做三方合并，改为：
     - 逐一比较本地和远程文件哈希
     - 哈希相同 → 跳过
     - 仅远程有 → 下载远程版本
     - 仅本地有 → 保留本地版本
     - 双方都有但不同 → 标记为冲突，同时打印双方修改时间
  3. 冲突文件：将远程版本另存为 <filename>.remote（不覆盖本地）
     提示用户手动选择
  4. 合并后的本地状态作为新快照的基线，parent 指向远程 HEAD
  5. warning 日志: "祖先不可达合并，建议尽快 push 以建立新的共同基线"
```

#### log（历史）

```
cc-box log [--oneline] [-n 10]

输出:
a1b2c3d4  2026-05-07 14:30  win-pc      auto sync
e5f6g7h8  2026-05-07 10:15  mac-book    added new skill
k9l0m1n2  2026-05-06 22:00  win-pc      binary updated to 2.1.126
```

`show <snapshot-id>` 展开显示文件变更列表和该快照记录的各平台二进制版本。

#### revert（回滚）

```
cc-box revert <snapshot-id>

1. 下载目标快照的文件索引
2. 对比当前状态，列出将要恢复的文件
3. 确认后从 WebDAV 下载对应 objects → 解密 → 写入本地
4. 创建新的 revert 快照（message 自动生成为 "revert to <snapshot-id>"）
```

### WebDAV 远程存储结构

```
/cc-box/
├── HEAD                          # 当前最新快照 ID（支持 ETag 乐观锁）
├── salt.bin                      # Argon2id salt（明文，用于多设备密钥派生）
├── snapshots/
│   ├── snap_a1b2c3d4.json.enc    # 快照元数据（加密）
│   ├── snap_e5f6g7h8.json.enc
│   └── ...
├── objects/
│   ├── ab/
│   │   └── c1234def...enc        # 配置文件内容对象（按哈希前缀 2 字符分目录）
│   └── ...
├── binaries/                     # 二进制文件存储（加密和分块策略可配置）
│   ├── index.json                # 版本索引（含引用计数）
│   ├── parts/                    # 分块存储（断点续传）
│   │   ├── sha256:abc123/
│   │   │   ├── part-000.zst      # 10MB 分块（仅压缩，encrypt=false 时）
│   │   │   ├── part-000.enc      # 10MB 分块（压缩+加密，encrypt=true 时）
│   │   │   ├── part-001.zst/.enc
│   │   │   └── manifest.json     # 分块清单（总数、每块哈希、总大小）
│   │   └── ...
│   ├── windows-amd64/
│   │   ├── claude-2.1.126.zst      # 完整文件（encrypt=false 时，仅压缩）
│   │   ├── claude-2.1.126.enc      # 完整文件（encrypt=true 时，压缩+加密）
│   │   └── ...
│   ├── darwin-arm64/
│   └── linux-amd64/
├── devices/
│   ├── win-pc-abc123.json        # 设备注册信息（含当前 HEAD、最后活跃时间）
│   └── mac-book-xyz789.json
└── projects/                     # 项目级配置
    └── <encoded-remote>/
        └── .claude.json.enc
```

### 本地同步仓库结构

```
~/.cc-box/
├── config.toml                   # 本地配置
├── HEAD                          # 本地最新快照 ID
├── cache/
│   ├── latest-remote             # 远程 HEAD 缓存（含最后 ETag）
│   └── objects/                  # 已下载的配置文件 objects 缓存（小文件）
├── snapshots/
│   ├── snap_a1b2c3d4.json       # 本地快照缓存
│   └── ...
└── key.bin                       # 派生加密密钥（0600 权限）
```

**不做本地文件备份**：WebDAV 云端即是备份。二进制文件（~242MB）和配置文件的状态都由快照链记录在云端，不需要在本地保留额外副本。pull 时直接覆盖本地文件，需要回滚时从 WebDAV 下载即可。

## cc-switch（cc-cli）兼容设计

cc-switch 的核心操作是修改 `settings.json` 的 `env` 字段（`ANTHROPIC_BASE_URL`、`ANTHROPIC_AUTH_TOKEN` 等 API 配置）。兼容策略：

### 1. JSON 字段级合并

对 `settings.json` 使用 JSON-aware 三方合并，不是文件整体替换：

```
合并规则:
- env 字段: 按 key 合并，不删除任何一方的环境变量
- permissions 字段: allow/deny 列表取并集
- 其他顶层字段: 值不同时保留远程版本（settings.json 的"权威版本"是最后修改的设备）
```

### 2. cc-switch 感知

```
settings.json 合并时的特殊处理:
┌───────────────────────────────────────────────────────┐
│ 字段               │ 合并策略                         │
├───────────────────────────────────────────────────────┤
│ env                │ 双向合并（保留双方所有 key）       │
│ permissions.allow  │ 并集                              │
│ permissions.deny   │ 并集                              │
│ includeCoAuthoredBy│ 远程优先                           │
│ 其他顶层字段       │ 远程优先                           │
└───────────────────────────────────────────────────────┘
```

### 3. 共存规则

- cc-switch 切换 API 配置后，下次 push 会将当前配置上传
- pull 时不会覆盖 cc-switch 刚切换的 API 配置（因为 env 合并保留所有 key）
- `~/.cc-cli/` 目录本身不参与同步（cc-cli 的本地管理数据）

## 端到端加密

### 加密方案

```
用户密码 → Argon2id(密码, salt) → 256-bit key → AES-256-GCM 加密
```

- 同一密码在所有设备上生成相同的密钥（salt 固定存储在 WebDAV `/cc-box/salt.bin`）
- salt 明文存储：威胁模型针对的是存储服务提供商，不是网络监听者
- 所有上传到 WebDAV 的 objects 和 snapshots 都经过加密
- 密码不存储在本地，只存储派生后的密钥到 `~/.cc-box/key.bin`（权限 0600）
- 加密格式与 [age](https://github.com/FiloSottile/age) 兼容，可用 age 工具手动解密验证

### 安全边界说明

**`key.bin` 被盗即数据泄露**：如果攻击者获取了设备文件系统访问权并读取 `key.bin`，则可以解密 WebDAV 上所有数据。这是设计上的取舍——便利性（不用每次都输密码）vs 安全性（本地密钥文件保护）。对于大多数用户的威胁模型（防止云存储提供商窥探数据），此方案足够。

### 加密范围

| 数据 | 是否加密 | 说明 |
|------|---------|------|
| objects（文件内容） | 是 | AES-256-GCM |
| snapshots | 是 | 含文件路径和哈希 |
| HEAD 指针 | 否 | 只是一个快照 ID |
| devices/*.json | 否 | 只含设备名和平台信息 |
| salt.bin | 否 | 需要新设备 init 时读取 |
| binaries 分块文件 | 可配置 | 由 `binary.encrypt` 控制。默认 false（仅压缩不加密） |
| binary index.json | 否 | 版本号/哈希/平台等元数据 |
| config.toml（本地） | 否 | 本地文件，含 WebDAV 地址和用户名 |

### 密钥管理

```
首次 init:
  1. 用户输入密码
  2. 生成 16 字节随机 salt
  3. Argon2id(password, salt) → 256-bit key
  4. key 写入 ~/.cc-box/key.bin (0600)
  5. salt 上传 WebDAV /cc-box/salt.bin
  6. 用 key 加密并上传首个快照 → 验证加解密链路正常

新设备 init:
  1. 用户输入密码
  2. 从 WebDAV 下载 salt
  3. 相同 Argon2id 参数派生密钥 → 写入 key.bin
  4. 尝试用 key 解密 HEAD 指向的 snapshot
     - 成功 → 密码正确，继续 normal pull
     - 失败 → 提示密码错误，重新输入（最多 3 次）
```

### 密钥轮转（Rekey）

用户修改密码时需要重新加密所有已存储数据：

```
cc-box config rekey

流程:
  1. 提示输入当前密码，验证（解密 HEAD snapshot 确认）
  2. 提示输入新密码
  3. 生成新 salt，Argon2id 派生新 key
  4. 从 WebDAV 下载所有 objects + snapshots（流式，逐个处理）
  5. 旧 key 解密 → 新 key 加密 → 上传回 WebDAV（覆盖写入）
  6. 上传新 salt.bin
  7. 本地更新 key.bin

安全保证：
  - 轮转过程中断：已上传的文件已完成加密切换，未上传的仍是旧加密
  - 中断后重试：先完成剩余文件，保证全部切换完成
  - 旧 key 不保留：轮转完成后旧 key 无法解密任何云端数据
```

## 项目级 .claude.json 同步

### 问题

`.claude.json` 存在于每个项目根目录，包含 MCP server 配置和 allowedTools。这些配置在不同设备上应该一致，但项目路径不同。

### 方案

```
push 时:
1. 扫描 ~/.claude/projects/ 下所有项目目录
2. 读取每个项目对应的 .claude.json（从实际项目目录读取）
3. 获取项目的所有 git remote URL，选择优先级：
   - 有 "origin" remote → 使用 origin URL
   - 无 origin → 使用第一个 remote URL
   - 无任何 remote → 使用项目目录名的 SHA256 作为后备 ID
4. 按 URL 编码后作为项目唯一标识
5. 上传到 /cc-box/projects/<encoded-remote>/.claude.json.enc

pull 时:
1. 下载 /cc-box/projects/ 下所有项目配置
2. 按 git remote 匹配本地项目
3. 合并 MCP server 配置到本地 .claude.json
4. 对于找不到匹配 remote 的配置 → 存储为 "orphan"
   - 记录在 ~/.cc-box/orphan_projects.json 中
   - 每次 pull 后提醒用户有 N 个 orphan 项目
   - orphan 项目手动确认后可关联到本地目录或删除
5. 本地已删除但云端仍有的项目配置 → 软删除（标记过期，30 天后 GC 清理）
```

### .claude.json 合并策略

```json
{
  "projects": {
    "github.com/user/repo": {
      "mcpServers": { ... },    // 合并：保留双方独有的 server，同名 server 远程优先
      "allowedTools": [ ... ],  // 并集
      "permissions": { ... }    // 合并
    }
  }
}
```

## 二进制版本管理

### 设计目标

Claude Code 的二进制文件（`~/.local/bin/claude`）约 242MB，加上历史版本（`~/.local/share/claude/versions/`）每个约 233MB。这些文件是跨设备同步的关键：

- 换新电脑不需要重新安装 Claude Code
- 可以在任意版本间切换（如新版本有 bug 回退到旧版本）
- 保留所有历史版本的备份

### 版本索引

```json
// 存储在 WebDAV /cc-box/binaries/index.json（不加密）
{
  "platforms": {
    "windows-amd64": {
      "claude": {
        "current": "2.1.126",
        "versions": {
          "2.1.126": {
            "hash": "sha256:abc123...",
            "size": 254053024,
            "refs": 2,
            "uploaded": "2026-05-03T21:34:00Z",
            "uploaded_by": "win-pc-abc123"
          },
          "2.1.84": {
            "hash": "sha256:def456...",
            "size": 245000000,
            "refs": 1,
            "uploaded": "2026-04-28T10:00:00Z",
            "uploaded_by": "win-pc-abc123"
          }
        }
      },
      "uv": {
        "current": "0.6.14",
        "versions": { ... }
      }
    },
    "darwin-arm64": {
      "claude": {
        "current": "2.1.126",
        "versions": { ... }
      }
    }
  }
}
```

**`refs` 字段**：引用计数，表示有多少个设备将该版本标记为 current。prune 时用于判断是否可以安全删除。

### 大文件分块上传与断点续传

二进制文件（~243MB）通过 WebDAV 上传，需要考虑网络中断和 WebDAV 服务端的限制。上传策略由两个配置项组合控制：

#### 四种上传模式

| 模式 | encrypt | chunk_mode | 行为 |
|------|---------|------------|------|
| 加密 + 按需分块 | true | auto | 超过阈值→分块加密上传；未超过→整体加密上传 |
| 不加密 + 按需分块 | false | auto | 超过阈值→分块压缩上传；未超过→整体压缩上传 |
| 加密 + 始终分块 | true | always | 所有文件分块加密上传，支持断点续传 |
| 不加密 + 始终分块 | false | always | 所有文件分块压缩上传，支持断点续传 |

默认配置：`encrypt = false`，`chunk_mode = "auto"`。适合大多数场景（二进制为公开可获取文件，不需要加密；小文件无需分块）。

所有模式都使用 zstd 压缩。加密在压缩之后执行（compress → encrypt），兼顾压缩比和安全性。

```
上传流程 (cc-box binary push):
1. 读取本地二进制文件
2. 计算整体 sha256 哈希（用于最终校验和去重）
3. 查询 WebDAV binaries/index.json，同哈希->跳过
4. 判断是否分块:
   - chunk_mode = "always" → 始终分块
   - chunk_mode = "auto"   → 文件 > chunk_threshold_mb 时分块
5. 分块上传路径:
   a. 分块:
      - 将文件按 chunk_size_mb 分块
      - 每块: compressed = zstd_compress(chunk)
      - 如 encrypt=true: encrypted = AES-256-GCM(compressed, key, nonce)
      - 上传 manifest.json 到 /cc-box/binaries/parts/<hash>/manifest.json
      - 逐个上传 part-NNN.zst（或 .enc）
        - 上传前检查该 part 是否已存在 → 跳过已上传的块（断点续传）
      - 所有 part 上传完成后，验证完整性
   b. 整体上传:
      - compressed = zstd_compress(file)
      - 如 encrypt=true: output = AES-256-GCM(compressed, key, nonce)
      - 上传到 /cc-box/binaries/<platform>/<name>-<version>.zst（或 .enc）
6. 更新 index.json

下载流程 (cc-box binary pull [VERSION]):
1. 下载 index.json → 找到目标版本
2. 根据版本记录判断分块或整体:
   a. 分块下载:
      - 下载 manifest.json
      - 逐个下载 part-NNN
      - 如 encrypt=true: 先解密再解压
      - 如 encrypt=false: 直接解压
      - 校验整体 sha256
   b. 整体下载:
      - 下载完整文件
      - 如 encrypt=true: 先解密再解压
      - 如 encrypt=false: 直接解压
3. 将临时文件 move 到最终位置

断点续传状态文件 (.cc-box-download/progress.json):
{
  "hash": "sha256:abc123...",
  "total_parts": 25,
  "completed_parts": [0,1,2,3,4,5],
  "started": "2026-05-07T14:30:00Z"
}
```

分块阈值由 `binary.chunk_threshold_mb` 控制（默认 50MB）。`chunk_mode = "auto"` 时，文件超过阈值才分块；`chunk_mode = "always"` 时忽略阈值，所有文件都分块。

### 版本切换流程

```
cc-box binary switch 2.1.84

1. 将当前 ~/.local/bin/claude.exe 移动到 ~/.local/share/claude/versions/{当前版本号}
2. 从 WebDAV 分块下载目标版本到 ~/.local/bin/claude.exe（支持断点续传）
3. 验证 sha256 哈希
4. 更新 index.json 中的 current 指针 + refs 引用计数
```

### 清理（Prune）—— 引用安全检查

```
cc-box binary prune

流程:
  1. 计算每个版本的最小 safe_refs：
     - 遍历所有设备的 device.json，获取其 HEAD 指向的快照
     - 检查每个快照的 binary 字段，统计各版本被哪些设备引用
  2. 同时检查本地 ~/.local/share/claude/versions/ 目录中的版本
  3. 标记 refs 为 0 且不在任何设备快照中的版本为 "可清理"
  4. 列出可清理的版本，确认后删除
  5. 同时清理对应的 parts/ 分块文件

安全规则：
  - 绝不删除任何设备 HEAD 关联的快照中引用的版本
  - 绝不删除 index.json 中 refs > 0 的版本
  - 绝不删除本地 versions/ 中存在的版本
```

## CLI 命令完整设计

```
# 初始化
cc-box init                     # 交互式向导（WebDAV 配置 + 加密设置 + 首次快照）
                                #   使用 bubbletea TUI 实现

# 日常同步
cc-box push [-m MSG] [--dry-run] [-q]
cc-box pull [--dry-run] [--force] [-q]
cc-box sync                     # pull + push 一步完成

# 状态查看
cc-box status                   # 本地 vs 远程差异概览
cc-box diff [FILE]              # 查看具体文件差异内容

# 版本历史
cc-box log [--oneline] [-n N]   # 查看快照历史
cc-box show <snapshot-id>       # 查看指定快照详情（含文件变更列表和二进制版本）
cc-box revert <snapshot-id>     # 回滚到指定快照

# 冲突处理
cc-box conflicts                # 列出未解决的冲突
cc-box resolve <file>           # 交互式解决文件冲突

# 项目配置
cc-box project list             # 列出已追踪的项目
cc-box project push [PATH]      # 推送指定项目的 .claude.json
cc-box project pull             # 拉取所有项目配置
cc-box project orphans          # 列出未匹配的 orphan 项目

# 配置管理
cc-box config get <key>         # 查看配置项
cc-box config set <key> <val>   # 修改配置项
cc-box config webdav            # 重新配置 WebDAV 连接
cc-box config encryption        # 重新配置加密（rekey）
cc-box config rekey             # 密钥轮转

# 二进制管理
cc-box binary list              # 列出所有已备份的版本
cc-box binary push              # 上传当前二进制到云端
cc-box binary pull [VERSION]    # 从云端下载二进制
cc-box binary switch <VERSION>  # 切换二进制版本
cc-box binary prune             # 清理不再被引用的版本

# 设备管理
cc-box device list              # 列出已注册设备
cc-box device rename <name>     # 重命名当前设备
cc-box device forget <id>       # 移除设备（清理其快照引用）

# 维护
cc-box backup                   # 创建本地完整备份
cc-box restore [snapshot-id]    # 从备份/快照恢复
cc-box gc                       # 清理过期 objects 和快照（引用安全）
cc-box verify                   # 验证本地文件完整性 + 远程可达性

# 通用
cc-box --help
cc-box --version
```

## WebDAV 兼容性矩阵

| 服务 | 基础 URL | 认证 | ETag 支持 | 备注 |
|------|----------|------|-----------|------|
| 坚果云 | `dav.jianguoyun.com/dav/` | Basic（应用密码） | 是 | 免费用户约 300 次/月 API，2GB 空间 |
| NextCloud | `/remote.php/dav/files/<user>/` | Basic | 是 | 完整 WebDAV + ETag |
| Synology | `:<port>/webdav/` | Basic/Digest | 是 | 需启用 WebDAV 服务 |
| Alist | `/dav/` | Basic | 是 | 支持多种后端存储 |
| OwnCloud | `/remote.php/dav/` | Basic | 是 | 与 NextCloud 类似 |
| IIS WebDAV | `/webdav/` | NTLM/Basic | 是 | Windows 原生支持 |
| rclone serve | `localhost:<port>/` | Basic | 视后端而定 | 本地测试用 |
| Box | `/dav/` | OAuth2 | 是 | 企业用户 |
| 海康威视 NAS | `/webdav/` | Basic | 部分 | 部分型号支持 |

### WebDAV 操作需求

| 操作 | 用途 | 必需 | ETag 相关 |
|------|------|------|-----------|
| `PROPFIND` (depth 0/1) | 检查文件/目录存在、列目录、获取 ETag | 是 | 获取 `getetag` 属性 |
| `GET` | 下载文件，附带 `ETag` 响应头 | 是 | 存储 ETag |
| `PUT` | 上传文件，支持 `If-Match` 条件请求 | 是 | 乐观锁 |
| `MKCOL` | 创建目录 | 是 | — |
| `DELETE` | 删除文件/目录 | 是 | — |
| `HEAD` | 检查文件元信息（大小/修改时间/ETag） | 否（优化用） | 快速获取 ETag |

### 坚果云优化策略

免费用户约 300 次 API 调用/月，需严格控制请求数：
- 本地缓存远程 HEAD ETag，未变化时跳过 pull
- 合并多个 PROPFIND 为单次 depth=1 请求
- 二进制文件采用分块上传避免重复上传整个文件
- 每次 push/pull 请求数预估：~10-30 次（含 objects 上传/下载）
- 建议用户使用付费版或自建 WebDAV 以获得更好的体验

## 安全设计

### 传输层
- HTTPS 强制（`--allow-http` 参数允许本地测试）
- Basic / Digest 认证
- WebDAV 密码存系统密钥环（macOS Keychain / Windows Credential Manager / Linux Secret Service）

### 数据层
- 端到端加密：Argon2id 派生 + AES-256-GCM
- 密码不存储，只存派生密钥
- 敏感文件排除（.credentials.json 永远不同步）
- settings.json 中 env.ANTHROPIC_AUTH_TOKEN 等敏感字段在快照中与整个文件一并加密

### 本地
- `~/.cc-box/key.bin` 权限 0600（仅当前用户可读）
- 备份文件 7 天自动清理
- gc 命令清理过期 objects（基于引用可达性，不是简单计数）

## 错误恢复与边界情况

### init 阶段

| 场景 | 处理 |
|------|------|
| WebDAV 已有数据（salt.bin 存在） | 提示用户选择：加入已有同步组 / 覆盖（清空后重新初始化） |
| salt.bin 存在但 HEAD 不存在 | 视为不完整状态，提示重新初始化或联系管理员 |
| HEAD 指向已删除的快照 | 提示数据损坏，建议从其他设备重新 push |
| 密码验证失败 | 最多重试 3 次，然后退出。不锁定密钥 |

### push 阶段

| 场景 | 处理 |
|------|------|
| WebDAV 不可达 | 报错退出，不做任何本地修改 |
| 上传 object 失败（网络中断） | 重试 3 次（指数退避），仍失败则报错。已上传的 objects 保留 |
| HEAD 乐观锁冲突（3 次都失败） | 提示用户手动 pull 后再 push |
| 磁盘空间不足（加密临时文件） | 清理临时文件后报错 |

### pull 阶段

| 场景 | 处理 |
|------|------|
| WebDAV 不可达 | 报错退出，不做任何本地修改 |
| 下载 object 失败 | 重试 3 次，仍失败则跳过该文件，记录到冲突列表 |
| 共同祖先不可达 | 降级为文件级双向合并（见降级策略） |
| 合并后文件与预期不符 | 用户可 revert 到合并前的快照 |

### 二进制下载

| 场景 | 处理 |
|------|------|
| 下载中断 | 保留已完成的分块，下次从断点继续 |
| 哈希校验失败 | 删除损坏的分块，重新下载 |
| 分块 manifest 与 index.json 不一致 | 以 index.json 为准，重新获取 manifest |
| 目标路径无写入权限 | 报错并提示手动修改权限 |

## 测试策略

### 单元测试（每个模块）

```
- config: TOML 解析、密钥环读写、默认值填充
- webdav: PROPFIND/GET/PUT/MKCOL/DELETE 请求构造与 XML 解析
- snapshot: 快照创建、序列化、链式遍历、祖先查找
- sync/merger: 三方合并（文本、JSON、目录）每种类型至少 5 个测试用例
- sync/json_merge: settings.json 字段级合并（覆盖 cc-switch 场景）
- crypto: 加解密往返测试、age 兼容性验证、keygen 确定性
- object: SHA256 计算、规范化哈希（CRLF/LF）
- binary: 分块/合并、断点续传状态文件读写
- normalizer: 路径、换行符、大小写规范化
```

### 集成测试

```
- WebDAV 往返测试: init → push → pull → status（使用 rclone serve 本地 WebDAV）
- 多设备模拟: 两个独立 ~/.cc-box/ 目录同时 push（验证乐观锁）
- GC 安全测试: 创建 30 个快照后 GC，验证不被引用的 objects 不被删除
- 加密测试: 验证加密后的 objects 确实不可读（无 key 时）
- 跨平台规范化: 同一文件 CRLF 和 LF 版本产生相同哈希
- 大文件分块: 模拟网络中断后断点续传
- 密钥轮转: rekey 后所有 objects 可用新密码解密
- 坚果云兼容: 针对坚果云特有行为测试（如有条件）
```

### 测试基础设施

- CI (GitHub Actions): 每个 PR 运行单元测试 + 集成测试
- WebDAV mock server: 内嵌测试用 WebDAV 服务，模拟各种异常场景
- 覆盖率目标: > 80%（核心 sync/merge/crypto 模块 > 90%）

## 项目结构

```
cc-box/
├── cmd/
│   └── cc-box/
│       └── main.go               # 入口（CLI 或 GUI 模式判断）
├── internal/
│   ├── cli/
│   │   ├── root.go               # 根命令
│   │   ├── init.go               # init 命令（bubbletea TUI 向导）
│   │   ├── push.go               # push 命令
│   │   ├── pull.go               # pull 命令
│   │   ├── sync.go               # sync 命令
│   │   ├── status.go             # status 命令
│   │   ├── diff.go               # diff 命令
│   │   ├── log.go                # log/show 命令
│   │   ├── revert.go             # revert 命令
│   │   ├── conflicts.go          # conflicts/resolve 命令
│   │   ├── project.go            # project 子命令
│   │   ├── config_cmd.go         # config + config rekey 子命令
│   │   ├── device.go             # device 子命令
│   │   ├── binary.go             # binary 子命令
│   │   └── maintenance.go        # gc/verify/backup/restore
│   ├── config/
│   │   ├── config.go             # 配置结构体 + TOML 读写
│   │   └── keyring.go            # 系统密钥环操作
│   ├── webdav/
│   │   ├── client.go             # WebDAV HTTP 客户端（支持 ETag/条件请求）
│   │   ├── operations.go         # PROPFIND/GET/PUT/MKCOL/DELETE
│   │   ├── lock.go               # 乐观锁（GET HEAD + ETag + PUT If-Match）
│   │   ├── auth.go               # Basic/Digest 认证
│   │   └── xml.go                # WebDAV XML 解析
│   ├── snapshot/
│   │   ├── snapshot.go           # 快照结构定义 + 序列化
│   │   ├── chain.go              # 快照链遍历、祖先查找
│   │   ├── scanner.go            # 本地文件扫描 + 排除规则
│   │   └── differ.go             # 快照差异计算
│   ├── normalize/
│   │   └── normalize.go          # 跨平台规范化（路径/换行/大小写）
│   ├── sync/
│   │   ├── engine.go             # 同步引擎（push/pull 核心逻辑）
│   │   ├── merger.go             # 三方合并主逻辑（按文件类型分发）
│   │   ├── text_merge.go         # 文本文件行级合并
│   │   ├── json_merge.go         # JSON 字段级合并（cc-switch 兼容）
│   │   ├── dir_merge.go          # 目录级合并（递归）
│   │   ├── history_merge.go      # history.jsonl 去重追加合并
│   │   └── conflict.go           # 冲突检测与解决
│   ├── crypto/
│   │   ├── encrypt.go            # AES-256-GCM 加解密
│   │   ├── keygen.go             # Argon2id 密钥派生
│   │   ├── rekey.go              # 密钥轮转
│   │   └── age_compat.go         # age 格式兼容
│   ├── object/
│   │   ├── store.go              # Object 存储管理（上传/下载/缓存）
│   │   └── hash.go               # SHA-256 文件指纹（含规范化）
│   ├── binary/
│   │   ├── index.go              # 版本索引管理（上传/下载/更新)
│   │   ├── chunker.go            # 大文件分块/合并
│   │   ├── upload.go             # 二进制上传（含断点续传）
│   │   ├── download.go           # 二进制下载（含断点续传）
│   │   └── prune.go              # 引用安全清理
│   ├── project/
│   │   ├── tracker.go            # 项目发现与 git remote 匹配
│   │   ├── config_merge.go       # .claude.json 合并
│   │   └── orphans.go            # orphan 项目管理
│   └── tui/
│       ├── wizard.go             # init 交互式向导（bubbletea）
│       └── widgets.go            # 通用 TUI 组件
├── go.mod
├── go.sum
├── Makefile
├── .goreleaser.yml               # 多平台构建发布
└── README.md
```

## 配置文件格式

`~/.cc-box/config.toml`：

```toml
[webdav]
url = "https://dav.jianguoyun.com/dav/"
username = "user@example.com"
# 密码存在系统密钥环，不在配置文件中
root = "/cc-box/"

[encryption]
enabled = true
# 密钥存在 ~/.cc-box/key.bin

[sync]
snapshot_limit = 50     # 本地保留的最近快照数（用于祖先查找）
conflict_strategy = "ask"  # ask / local / remote
merge_retry_max = 3     # HEAD 乐观锁冲突最大重试次数

[device]
id = "win-pc-abc123"    # 自动生成的设备 ID（hostname + 随机 6 字符）
name = "办公室电脑"      # 可自定义

[claude]
path = ""               # 留空自动检测 ~/.claude/

[binary]
encrypt = false              # 是否加密二进制文件（false=仅压缩，true=压缩+AES-256-GCM）
chunk_mode = "auto"          # 分块模式: "auto"=超过阈值时分块, "always"=始终分块
chunk_size_mb = 10           # 分块大小（MB）
chunk_threshold_mb = 50      # chunk_mode="auto" 时的分块阈值（MB）
auto_upload = false          # push 时是否自动上传二进制

[exclude]
patterns = [
    "sessions/",
    "cache/",
    "debug/",
    "telemetry/",
    "downloads/",
    "paste-cache/",
    "shell-snapshots/",
    "file-history/",
    "session-env/",
    "ide/",
    "backups/",
    "plans/",
    "tasks/",
    "teams/",
    "plugins/data/",
    "*.lock",
]
```

## 图形界面设计

### 技术方案：Wails v2 + Svelte + Tailwind

| 选项 | 优势 | 劣势 |
|------|------|------|
| **Wails** (Go + Web) | 单二进制、Go 后端复用、体积小 (~15MB)、跨平台 | 需要前端开发，Windows 需要 WebView2 |
| Fyne (纯 Go) | 纯 Go、单二进制 | UI 不够精致、自定义能力弱 |
| Electron | 生态成熟 | 体积大 (~100MB+)、内存占用高 |
| Qt (Go 绑定) | 原生体验 | CGO 依赖、编译复杂 |

选 Wails 的理由：
- Go 后端与 CLI 完全复用（同一个 `internal/` 包）
- Web 前端可以用 Svelte + Tailwind 做出现代 UI
- 编译出单个可执行文件，无运行时依赖（Windows 需系统自带 WebView2）
- 打包体积约 10-15MB（vs Electron 的 100MB+）
- 原生系统托盘、文件对话框支持

**与 CLI/TUI 的边界**：
- `bubbletea` 仅用于 CLI 的 `init` 交互式向导（非 GUI 模式下需要）
- `fsnotify` 仅用于 GUI 自动同步模式（监听 `~/.claude/` 变更）
- GUI 模式下所有长耗时操作（二进制下载/上传）通过 goroutine 执行，通过 Wails EventsEmit 推送进度

### GUI 模式下长耗时操作的非阻塞处理

Go 后端绑定方法在 Wails 中默认是同步的，会阻塞 UI。长操作需要异步模式：

```go
// 异步启动二进制下载，通过事件推送进度
func (a *App) PullBinaryAsync(version string) {
    go func() {
        // 进度回调 → runtime.EventsEmit(ctx, "binary:download-progress", progress)
        err := binary.Download(ctx, version, func(p Progress) {
            runtime.EventsEmit(a.ctx, "binary:download-progress", p)
        })
        if err != nil {
            runtime.EventsEmit(a.ctx, "binary:download-error", err.Error())
            return
        }
        runtime.EventsEmit(a.ctx, "binary:download-complete", nil)
    }()
}
```

### 界面结构

```
┌─────────────────────────────────────────────────────────────┐
│  CC-Box                                              — □ ×  │
├──────────┬──────────────────────────────────────────────────┤
│          │                                                  │
│  概览    │  [主内容区域]                                     │
│  配置    │                                                  │
│  二进制  │                                                  │
│  历史    │                                                  │
│  设置    │                                                  │
│          │                                                  │
│──────────│                                                  │
│  ● 已同步 │                                                  │
│  上次:    │                                                  │
│  10:30   │                                                  │
└──────────┴──────────────────────────────────────────────────┘
```

### 页面设计

#### 1. 概览（Dashboard）

主页面，一眼看到整体状态。

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│  ┌───────────────┐  ┌───────────────┐               │
│  │  同步状态      │  │  Claude 版本  │               │
│  │               │  │               │               │
│  │  ✅ 已同步     │  │  2.1.126      │               │
│  │  10 分钟前    │  │  云端最新      │               │
│  └───────────────┘  └───────────────┘               │
│                                                      │
│  ┌──────────────────────────────────────────────┐   │
│  │  快捷操作                                     │   │
│  │                                              │   │
│  │  [↑ 推送配置]    [↓ 拉取配置]    [⟷ 同步]     │   │
│  │                                              │   │
│  └──────────────────────────────────────────────┘   │
│                                                      │
│  ┌──────────────────────────────────────────────┐   │
│  │  已连接设备                                   │   │
│  │                                              │   │
│  │  💻 win-pc-abc123    2.1.126   刚刚           │   │
│  │  🍎 mac-book-xyz789  2.1.126   2小时前        │   │
│  └──────────────────────────────────────────────┘   │
│                                                      │
│  ┌──────────────────────────────────────────────┐   │
│  │  最近变更                                     │   │
│  │                                              │   │
│  │  M  settings.json           10 分钟前         │   │
│  │  A  skills/new-skill/       1 小时前          │   │
│  │  M  CLAUDE.md               3 小时前          │   │
│  └──────────────────────────────────────────────┘   │
│                                                      │
└──────────────────────────────────────────────────────┘
```

数据来源：Go 后端通过 Wails binding 暴露 `GetDashboard()` 方法。

#### 2. 配置文件

功能点：文件树展示带状态图标、点击文件显示 diff、批量选择同步/排除、冲突文件三选一。

#### 3. 二进制管理

功能点：版本列表区分"当前使用/仅云端/本地+云端"、切换版本、上传本地版本、删除云端版本、存储空间进度条、下载/上传进度条。

#### 4. 历史记录

快照时间线。每个快照显示设备、变更摘要（含该快照记录的各平台二进制版本）。支持展开查看文件变更列表、回滚按钮、按设备/时间筛选。

#### 5. 设置

WebDAV 连接配置（支持测试连接）、加密设置（含"更改密码"即 rekey）、同步偏好、设备管理。

### 系统托盘

托盘图标状态：🟢已同步 🟡待同步变更 🔴冲突或连接错误 🔵同步中

### 安装包

| 平台 | 格式 | 说明 |
|------|------|------|
| Windows | MSIX + portable zip | 开始菜单集成、开机自启 |
| macOS | .app + DMG | 拖入 Applications |
| Linux | AppImage + .deb | 免安装 |

## 开发计划

### Phase 1: 基础骨架（MVP）

目标：能跑通 init → push → pull → status 的基本流程。

- [ ] 项目初始化（Go module + cobra CLI 框架）
- [ ] config 模块（config.toml 读写 + 系统密钥环）
- [ ] WebDAV 客户端（PROPFIND/GET/PUT/MKCOL/DELETE，含 ETag 解析）
- [ ] 跨平台规范化器（路径/换行/大小写规范化）
- [ ] 文件扫描器（扫描 ~/.claude/，应用排除规则 + 规范化哈希）
- [ ] 快照管理（创建快照、链式存储、祖先查找）
- [ ] Object 存储（文件内容加密 → 上传/下载到 WebDAV）
- [ ] init 命令（bubbletea 交互式向导 + 首次快照创建）
- [ ] push 命令（扫描变更 → 加密 → 上传 → 乐观锁更新 HEAD）
- [ ] pull 命令（下载远程快照 → 差异对比 → 下载 objects → 应用）
- [ ] status 命令（本地 vs 远程差异概览）

### Phase 2: 合并、加密与二进制

目标：三方合并、端到端加密、二进制版本管理。

- [ ] 端到端加密（Argon2id + AES-256-GCM + age 兼容格式）
- [ ] 密钥轮转（rekey）
- [ ] 三方合并引擎（文本行级 + JSON 字段级 + 目录递归 + history.jsonl 特殊处理）
- [ ] cc-switch 兼容的 settings.json 合并
- [ ] 祖先不可达降级策略
- [ ] 冲突检测与交互式解决
- [ ] 二进制分块上传/下载（含断点续传）
- [ ] 二进制版本索引管理
- [ ] 二进制版本切换
- [ ] log / show / revert 命令
- [ ] backup / restore 命令
- [ ] 坚果云适配（请求频率控制 + 本地缓存优化）

### Phase 3: GUI + 项目同步

目标：图形界面、项目级配置同步、自动同步。

- [ ] Wails 项目集成（Go 后端 + Svelte 前端）
- [ ] Dashboard 页面（同步状态、快捷操作、设备列表）
- [ ] 配置文件页面（文件树、diff 查看、冲突解决）
- [ ] 二进制管理页面（版本列表、上传/下载/切换、进度条）
- [ ] 历史记录页面（快照时间线、回滚）
- [ ] 设置页面（WebDAV、加密 + rekey、同步偏好）
- [ ] 系统托盘（状态图标、快捷菜单、基于 fsnotify 的自动同步）
- [ ] 项目级 .claude.json 同步（git remote 匹配 + orphan 管理）
- [ ] diff 命令（文件内容对比）
- [ ] gc / verify / binary prune 命令
- [ ] macOS / Linux 适配测试

### Phase 4: 打磨与发布

目标：正式发布可用版本。

- [ ] 完整测试覆盖（单元 + 集成测试，覆盖率 > 80%）
- [ ] goreleaser 多平台发布（win/mac/linux, amd64/arm64）
- [ ] Windows MSIX 安装包 + portable zip
- [ ] macOS DMG 打包（含代码签名）
- [ ] Linux AppImage + .deb
- [ ] Homebrew formula（macOS）
- [ ] Scoop manifest（Windows）
- [ ] 完善文档和 README
- [ ] 预留 projects/ 会话同步接口（SyncTarget 接口设计完成，不实现）

## 预留：会话同步扩展点

Phase 4 不实现项目会话同步，但以下接口设计好：

```go
// internal/sync/engine.go 中的扩展接口
type SyncTarget interface {
    Scan() ([]FileEntry, error)
    Merge(local, remote, ancestor []FileEntry) ([]MergeResult, error)
    PathRewrite(path string, fromDevice DeviceInfo) (string, error)
}

// 当前实现：配置文件同步
type ConfigSync struct { /* ... */ }
func (c *ConfigSync) Scan() ([]FileEntry, error) { /* ... */ }

// 预留：会话数据同步（Phase 5+）
type SessionSync struct { /* ... */ }
func (s *SessionSync) Scan() ([]FileEntry, error) { /* ... */ }
func (s *SessionSync) PathRewrite(path string, fromDevice DeviceInfo) (string, error) {
    // 路径重写逻辑：C:\Users\a\... → /home/alice/...
}
```

在 config.toml 中预留：

```toml
[sync.targets]
config = true     # 当前实现
sessions = false  # 预留，默认关闭
memory = false    # 预留，默认关闭
```

## 设计权衡记录

记录几个有意的取舍，便于后续开发者理解：

1. **不用分布式锁**：WebDAV 不支持原生分布式锁，且本工具面向个人设备（通常 2-3 台），乐观锁足够。如果未来扩展到团队使用，可考虑在 WebDAV 上放一个 lock 文件做简单互斥。

2. **key.bin 本地存储 vs 每次输密**：选择了本地存储派生密钥的便利性。攻击者获取文件系统权限时无法防御，但对于云端存储提供商的数据泄露场景有效。

3. **快照链 vs DAG**：选择了线性快照链而不是 git 式的 DAG（有向无环图）。因为个人设备场景的分支情况很少，线性链简化了合并逻辑。如果未来出现多分支需求，可以通过 parent 字段扩展为 `parents[]` 数组。

4. **WebDAV 而非 S3/GCS**：牺牲了对象存储的原子语义（条件写入），换取了更广泛的存储后端支持（用户可以自建 NextCloud、用坚果云等）。

5. **二进制不参与快照 diff**：二进制文件的变化通过 binary index 独立管理，不纳入配置文件快照的 diff 流程。这避免了 243MB 文件的版本对比开销。
