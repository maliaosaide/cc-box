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
- **版本备份**：将 `~/.local/bin/claude` 和历史版本上传到 WebDAV（压缩加密）
- **按哈希去重**：相同版本只存一份，不同设备共享
- **版本切换**：`cc-box binary switch 2.1.84` 一键切换 Claude 版本
- **新设备恢复**：pull 时自动下载当前版本到 `~/.local/bin/`，无需重新安装
- **平台感知**：Windows 下载 `.exe`，macOS/Linux 下载对应二进制
- **版本清理**：`cc-box binary prune` 清理不需要的历史版本

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
└── history.jsonl              # 命令历史（追加合并）
```

### 项目级配置（按项目独立同步）

```
<project-root>/
└── .claude.json               # MCP server 配置、项目权限
```

### 预留但暂不同步

```
projects/                      # 会话数据 - 预留接口，Phase 3 实现
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
cobra          # CLI 框架（kubectl/docker 同款）
viper          # 配置管理（TOML/JSON/环境变量）
reqwest        # HTTP 客户端（WebDAV 操作）
quick-xml      # WebDAV XML 解析
age            # 端到端加密（与 age 工具兼容）
argon2id       # 密码派生密钥
sha256         # 文件指纹
gosoft/gkeyring  # 系统密钥环（macOS Keychain / Linux Secret Service / Windows Credential Manager）
fsnotify       # 文件变更监听
bubbletea      # TUI 界面（交互式向导、状态面板）
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
    "CLAUDE.md": {
      "hash": "sha256:def456...",
      "size": 4096,
      "modified": "2026-05-07T13:00:00Z"
    }
  }
}
```

快照链：`snap_001 → snap_002 → snap_003 → ... → snap_latest`

每次 push 产生新快照，链接到上一个。支持沿链回溯任意历史版本。

### 同步流程

#### push（推送）

```
cc-box push [-m "message"] [--dry-run]

1. 扫描本地文件（应用排除规则）
2. 计算每个文件的 sha256 哈希
3. 与本地最新快照对比，找出变更文件:
   - 新增 (A)  - 本地有，快照无
   - 修改 (M)  - 哈希不同
   - 删除 (D)  - 快照有，本地无
4. 如无变更，提示 "nothing to push"
5. 加密变更文件内容 → 生成 object
6. 创建新快照，parent 指向当前最新
7. 上传 objects + 新快照到 WebDAV
8. 更新远程 latest 指针
9. 更新本地 latest 指针
```

#### pull（拉取）

```
cc-box pull [--dry-run] [--force]

1. 从 WebDAV 下载远程 latest 指针
2. 与本地 latest 对比:
   - 相同 → "already up to date"
   - 不同 → 需要同步
3. 找出本地和远程的分叉点（共同祖先快照）
4. 三方差异计算:
   ancestor = 分叉点快照
   local    = 本地当前文件状态
   remote   = 远程最新快照的文件状态
5. 对每个文件执行三方合并:
   - 仅本地变 → 保留本地
   - 仅远程变 → 采纳远程
   - 双方都变 → 冲突，需解决
   - 双方都删 → 删除
6. 下载需要的 objects，解密
7. 备份被覆盖的文件 → ~/.cc-box/backups/snap_xxx/
8. 应用合并结果
9. 创建合并快照，parent 指向远程最新
10. 上传合并快照 + 更新 latest
```

#### log（历史）

```
cc-box log [--oneline] [-n 10]

输出:
a1b2c3d4  2026-05-07 14:30  win-pc      auto sync
e5f6g7h8  2026-05-07 10:15  mac-book    added new skill
k9l0m1n2  2026-05-06 22:00  win-pc      updated CLAUDE.md
```

#### revert（回滚）

```
cc-box revert <snapshot-id>

1. 下载目标快照的文件索引
2. 对比当前状态，列出将要恢复的文件
3. 确认后恢复文件（备份当前版本）
4. 创建新的 revert 快照
```

### WebDAV 远程存储结构

```
/cc-box/
├── HEAD                          # 当前最新快照 ID
├── snapshots/
│   ├── snap_a1b2c3d4.json.enc    # 快照元数据（加密）
│   ├── snap_e5f6g7h8.json.enc
│   └── ...
├── objects/
│   ├── ab/
│   │   └── c1234def...enc        # 配置文件内容对象（按哈希前缀分目录）
│   └── ...
├── binaries/                     # 二进制文件存储（核心）
│   ├── index.json                # 版本索引
│   ├── windows-amd64/
│   │   ├── claude-2.1.126.exe.enc
│   │   ├── claude-2.1.84.exe.enc
│   │   ├── uv.exe.enc
│   │   └── ...
│   ├── darwin-arm64/
│   │   ├── claude-2.1.126.enc
│   │   └── ...
│   └── linux-amd64/
│       ├── claude-2.1.126.enc
│       └── ...
├── devices/
│   ├── win-pc-abc123.json        # 设备注册信息
│   └── mac-book-xyz789.json
└── projects/                     # 项目级配置（预留）
    └── ...
```

### 本地同步仓库结构

```
~/.cc-box/
├── config.toml                   # 本地配置
├── HEAD                          # 本地最新快照 ID
├── cache/
│   ├── latest-remote             # 远程 latest 缓存
│   └── objects/                  # 已下载的 objects 缓存
├── backups/
│   ├── snap_a1b2c3d4/           # 回滚/覆盖前的文件备份
│   └── ...
├── snapshots/
│   ├── snap_a1b2c3d4.json       # 本地快照缓存
│   └── ...
├── binary-cache/                 # 二进制下载缓存
│   ├── claude-2.1.126.exe       # 已下载的版本
│   └── ...
└── key.bin                       # 派生加密密钥（0600 权限）
```

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

- 同一密码在所有设备上生成相同的密钥（salt 固定存储在 WebDAV）
- 所有上传到 WebDAV 的 objects 和 snapshots 都经过加密
- 密码不存储在本地，只存储派生后的密钥到 `~/.cc-box/key.bin`
- 加密格式与 [age](https://github.com/FiloSottile/age) 兼容，可用 age 工具手动解密验证

### 加密范围

| 数据 | 是否加密 | 说明 |
|------|---------|------|
| objects（文件内容） | 是 | AES-256-GCM |
| snapshots | 是 | 含文件路径和哈希 |
| HEAD 指针 | 否 | 只是一个快照 ID |
| devices/*.json | 否 | 只含设备名和平台信息 |
| config.toml（本地） | 否 | 本地文件，含 WebDAV 地址和用户名 |

### 密钥管理

```
init 阶段:
  输入密码 → Argon2id 派生 → key.bin 写入本地
  同时将 salt 上传到 WebDAV /cc-box/salt.bin

新设备 init:
  输入相同密码 → 下载 salt → 派生相同密钥
  尝试解密 HEAD 指向的 snapshot → 验证密码正确性
```

## 项目级 .claude.json 同步

### 问题

`.claude.json` 存在于每个项目根目录，包含 MCP server 配置和 allowedTools。这些配置在不同设备上应该一致，但项目路径不同。

### 方案

```
push 时:
1. 扫描 ~/.claude/projects/ 下所有项目目录
2. 读取每个项目对应的 .claude.json（从实际项目目录读取）
3. 按 git remote URL 作为项目唯一标识（而非路径）
4. 上传到 /cc-box/projects/<encoded-remote>/.claude.json.enc

pull 时:
1. 下载 /cc-box/projects/ 下所有项目配置
2. 按 git remote 匹配本地项目
3. 合并 MCP server 配置到本地 .claude.json
4. 对于找不到匹配 remote 的配置 → 存储为 "orphan"，提示用户
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

## 二进制版本管理（核心）

### 设计目标

Claude Code 的二进制文件（`~/.local/bin/claude`）约 242MB，加上历史版本（`~/.local/share/claude/versions/`）每个约 233MB。这些文件是跨设备同步的关键：

- 换新电脑不需要重新安装 Claude Code
- 可以在任意版本间切换（如新版本有 bug 回退到旧版本）
- 保留所有历史版本的备份

### 版本索引

```json
// 存储在 WebDAV /cc-box/binaries/index.json
{
  "platforms": {
    "windows-amd64": {
      "claude": {
        "current": "2.1.126",
        "versions": {
          "2.1.126": {
            "hash": "sha256:abc123...",
            "size": 254053024,
            "uploaded": "2026-05-03T21:34:00Z",
            "uploaded_by": "win-pc-abc123"
          },
          "2.1.84": {
            "hash": "sha256:def456...",
            "size": 245000000,
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

### 版本切换流程

```
cc-box binary switch 2.1.84

1. 检查本地 binary-cache/ 是否已有该版本
   - 有 → 直接复制到 ~/.local/bin/claude.exe
   - 无 → 从 WebDAV 下载到 binary-cache/，再复制
2. 将当前版本移动到 ~/.local/share/claude/versions/{version}
3. 更新 index.json 中的 current 指针
4. 验证新版本的 sha256 哈希
```

### 备份流程

```
cc-box binary backup

1. 读取当前 ~/.local/bin/claude 的版本号
2. 计算 sha256 哈希
3. 对比 WebDAV index.json，检查是否已存在相同哈希
   - 已存在 → 跳过（去重）
   - 不存在 → 压缩 + 加密 → 上传
4. 扫描 ~/.local/share/claude/versions/ 下的历史版本
   - 同样按哈希去重上传
5. 更新 index.json
```

### 恢复流程（新设备）

```
cc-box binary restore

1. 从 WebDAV 下载 index.json
2. 读取当前平台（windows-amd64 / darwin-arm64 / linux-amd64）
3. 下载 current 指向的版本
4. 解密 + 解压 + 验证哈希
5. 写入 ~/.local/bin/claude[.exe]
6. 同时恢复 uv/uvx/uvw 等配套工具
7. 创建 ~/.local/share/claude/versions/ 目录结构
```

### 存储优化

| 策略 | 说明 |
|------|------|
| 哈希去重 | 相同版本文件只存一份，多设备共享 |
| gzip 压缩 | PE/Mach-O 二进制压缩率约 40-50% |
| 按需下载 | pull 时只下载当前版本，历史版本按需 |
| 增量备份 | 只上传本地有而远程没有的版本 |
| 平台隔离 | Windows/Mac/Linux 版本分开存储 |

### 存储空间估算

```
单个平台、3 个版本：
  claude × 3 = ~700MB → 压缩后 ~400MB
  uv × 1 = ~65MB → 压缩后 ~40MB
  总计 ~440MB（加密后略有膨胀）

坚果云免费 1GB 够用，建议用付费版或自建 WebDAV
```

## CLI 命令完整设计

```
# 初始化
cc-box init                     # 交互式向导（WebDAV 配置 + 加密设置 + 首次快照）

# 日常同步
cc-box push [-m MSG] [--dry-run] [-q]
cc-box pull [--dry-run] [--force] [-q]
cc-box sync                     # pull + push 一步完成

# 状态查看
cc-box status                   # 本地 vs 远程差异概览
cc-box diff [FILE]              # 查看具体文件差异内容

# 版本历史
cc-box log [--oneline] [-n N]   # 查看快照历史
cc-box show <snapshot-id>       # 查看指定快照详情
cc-box revert <snapshot-id>     # 回滚到指定快照

# 冲突处理
cc-box conflicts                # 列出未解决的冲突
cc-box resolve <file>           # 交互式解决文件冲突

# 项目配置
cc-box project list             # 列出已追踪的项目
cc-box project push [PATH]      # 推送指定项目的 .claude.json
cc-box project pull             # 拉取所有项目配置

# 配置管理
cc-box config get <key>         # 查看配置项
cc-box config set <key> <val>   # 修改配置项
cc-box config webdav            # 重新配置 WebDAV 连接
cc-box config encryption        # 重新配置加密

# 设备管理
cc-box device list              # 列出已注册设备
cc-box device rename <name>     # 重命名当前设备
cc-box device forget <id>       # 移除设备（清理其快照）

# 维护
cc-box backup                   # 创建本地完整备份
cc-box restore [snapshot-id]    # 从备份/快照恢复
cc-box gc                       # 清理过期的 objects 和备份
cc-box verify                   # 验证本地文件完整性

# 通用
cc-box --help
cc-box --version
```

## WebDAV 兼容性矩阵

| 服务 | 基础 URL | 认证 | 备注 |
|------|----------|------|------|
| 坚果云 | `dav.jianguoyun.com/dav/` | Basic（应用密码） | 免费用户有 API 频率限制 |
| NextCloud | `/remote.php/dav/files/<user>/` | Basic | 完整 WebDAV 支持 |
| Synology | `:<port>/webdav/` | Basic/Digest | 需启用 WebDAV 服务 |
| Alist | `/dav/` | Basic | 支持多种后端存储 |
| OwnCloud | `/remote.php/dav/` | Basic | 与 NextCloud 类似 |
| IIS WebDAV | `/webdav/` | NTLM/Basic | Windows 原生支持 |
| rclone serve | `localhost:<port>/` | Basic | 本地测试用 |
| Box | `/dav/` | OAuth2 | 企业用户 |
| 海康威视 NAS | `/webdav/` | Basic | 部分型号支持 |

### WebDAV 操作需求

| 操作 | 用途 | 必需 |
|------|------|------|
| `PROPFIND` (depth 0/1) | 检查文件/目录存在、列目录 | 是 |
| `GET` | 下载文件 | 是 |
| `PUT` | 上传文件 | 是 |
| `MKCOL` | 创建目录 | 是 |
| `DELETE` | 删除文件/目录 | 是 |
| `HEAD` | 检查文件元信息（大小/修改时间） | 否（优化用） |

坚果云适配注意：免费用户每月有请求次数限制（约 300 次/月），需要做本地缓存减少请求。

## 安全设计

### 传输层
- HTTPS 强制（`--allow-http` 参数允许本地测试）
- Basic / Digest 认证
- WebDAV 密码存系统密钥环（macOS Keychain / Windows Credential Manager / Linux Secret Service）

### 数据层
- 端到端加密：Argon2id 派生 + AES-256-GCM
- 密码不存储，只存派生密钥
- 敏感文件排除（.credentials.json 永远不同步）
- settings.json 中 env.ANTHROPIC_AUTH_TOKEN 等敏感字段在快照中加密存储

### 本地
- `~/.cc-box/key.bin` 权限 0600（仅当前用户可读）
- 备份文件 7 天自动清理
- gc 命令清理过期 objects（默认保留最近 20 个快照）

## 项目结构

```
cc-box/
├── cmd/
│   └── cc-box/
│       └── main.go               # 入口
├── internal/
│   ├── cli/
│   │   ├── root.go               # 根命令
│   │   ├── init.go               # init 命令
│   │   ├── push.go               # push 命令
│   │   ├── pull.go               # pull 命令
│   │   ├── sync.go               # sync 命令
│   │   ├── status.go             # status 命令
│   │   ├── diff.go               # diff 命令
│   │   ├── log.go                # log 命令
│   │   ├── revert.go             # revert 命令
│   │   ├── conflicts.go          # conflicts/resolve 命令
│   │   ├── project.go            # project 子命令
│   │   ├── config_cmd.go         # config 子命令
│   │   └── device.go             # device 子命令
│   ├── config/
│   │   ├── config.go             # 配置结构体 + 读写
│   │   └── keyring.go            # 系统密钥环操作
│   ├── webdav/
│   │   ├── client.go             # WebDAV HTTP 客户端
│   │   ├── operations.go         # PROPFIND/GET/PUT/MKCOL/DELETE
│   │   ├── auth.go               # Basic/Digest 认证
│   │   └── xml.go                # WebDAV XML 解析
│   ├── snapshot/
│   │   ├── snapshot.go           # 快照结构与管理
│   │   ├── scanner.go            # 本地文件扫描
│   │   └── differ.go             # 快照差异计算
│   ├── sync/
│   │   ├── engine.go             # 同步引擎（push/pull 核心逻辑）
│   │   ├── merger.go             # 三方合并
│   │   ├── json_merge.go         # JSON 字段级合并（cc-switch 兼容）
│   │   └── conflict.go           # 冲突检测与解决
│   ├── crypto/
│   │   ├── encrypt.go            # AES-256-GCM 加解密
│   │   ├── keygen.go             # Argon2id 密钥派生
│   │   └── age_compat.go         # age 格式兼容
│   ├── object/
│   │   ├── store.go              # Object 存储管理（上传/下载/缓存）
│   │   └── hash.go               # SHA-256 文件指纹
│   ├── project/
│   │   ├── tracker.go            # 项目发现与追踪
│   │   └── config_merge.go       # .claude.json 合并
│   ├── version/
│   │   └── tracker.go            # 二进制版本追踪
│   └── tui/
│       ├── wizard.go             # init 交互式向导
│       └── status_panel.go       # 状态面板渲染
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
snapshot_limit = 20     # 保留最近 N 个快照
backup_days = 7         # 备份保留天数
auto_backup = true      # pull 前自动备份
conflict_strategy = "ask"  # ask / local / remote

[device]
id = "win-pc-abc123"    # 自动生成的设备 ID
name = "办公室电脑"      # 可自定义

[claude]
path = ""               # 留空自动检测 ~/.claude/

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

## 开发计划

### Phase 1: 基础骨架（MVP）

目标：能跑通 init → push → pull → status 的基本流程。

- [ ] 项目初始化（Go module + cobra CLI 框架）
- [ ] config 模块（config.toml 读写 + 系统密钥环）
- [ ] WebDAV 客户端（PROPFIND/GET/PUT/MKCOL/DELETE）
- [ ] 文件扫描器（扫描 ~/.claude/，应用排除规则）
- [ ] 快照管理（创建快照、计算哈希、链式存储）
- [ ] Object 存储（文件内容上传/下载到 WebDAV）
- [ ] init 命令（交互式向导 + 首次快照创建）
- [ ] push 命令（扫描变更 → 加密 → 上传 → 更新快照链）
- [ ] pull 命令（下载远程快照 → 对比 → 下载 → 应用）
- [ ] status 命令（本地 vs 远程差异）

### Phase 2: 合并与加密

目标：处理冲突场景，加入端到端加密。

- [ ] 端到端加密（Argon2id + AES-256-GCM + age 兼容格式）
- [ ] 三方合并引擎（基于快照链的共同祖先）
- [ ] JSON 字段级合并（cc-switch 兼容的 settings.json 合并）
- [ ] 冲突检测与交互式解决（diff 展示 + 选择）
- [ ] log / show / revert 命令
- [ ] 备份与恢复
- [ ] 坚果云适配（请求频率控制 + 本地缓存优化）

### Phase 3: 高级功能

目标：项目级配置同步、自动同步、多平台测试。

- [ ] 项目级 .claude.json 同步（按 git remote 匹配）
- [ ] 二进制版本追踪
- [ ] diff 命令（文件内容对比）
- [ ] gc 命令（清理过期 objects 和备份）
- [ ] verify 命令（完整性校验）
- [ ] sync 命令（pull + push 一步完成）
- [ ] macOS / Linux 适配测试
- [ ] CI/CD（GitHub Actions 多平台构建）

### Phase 4: 打磨与发布

目标：正式发布可用版本。

- [ ] 完整测试覆盖（单元 + 集成测试）
- [ ] goreleaser 多平台发布（win/mac/linux, amd64/arm64）
- [ ] Homebrew formula（macOS）
- [ ] Scoop manifest（Windows）
- [ ] 完善文档和 README
- [ ] 预留 projects/ 会话同步接口（不实现，但留好扩展点）

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
func (s *SessionSync) PathRewrite(path string, from DeviceInfo) (string, error) {
    // 路径重写逻辑：C:\Users\a\... → /home/alice/...
}
```

在 config.toml 中预留：

```toml
[sync.targets]
config = true     # 当前阶段
sessions = false  # 预留，默认关闭
memory = false    # 预留，默认关闭
```
