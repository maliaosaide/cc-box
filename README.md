# CC-Box

Claude Code 的配置箱——备份、同步、版本管理，一个工具搞定。

## 它做什么

你在多台电脑上用 Claude Code。每台机器都要配置 `~/.claude/`、安装二进制、调 MCP server。手动搞一遍还行，换台电脑就头大。

CC-Box 解决三个核心问题：

1. **配置备份与同步** — 把 `~/.claude/`（settings.json、CLAUDE.md、skills、plugins 等）通过 WebDAV 同步到所有设备
2. **二进制文件管理** — 备份 `~/.local/bin/claude.exe` 和历史版本，支持版本切换和回滚
3. **版本控制** — 每次同步产生快照，可以查看历史、回滚到任意版本

## 同步内容

```
~/.claude/                        # Claude 配置目录
├── settings.json                 # 全局设置（API 配置、权限）
├── CLAUDE.md                     # 全局指令
├── skills/                       # 自定义技能
├── commands/                     # 自定义命令
├── agents/                       # 自定义 Agent
├── plugins/                      # 插件配置
└── ...

~/.local/                         # 二进制与版本管理（核心）
├── bin/
│   ├── claude.exe                # Claude 主程序 (~242MB)
│   ├── uv.exe / uvx.exe         # Python 包管理工具
│   └── ...
└── share/claude/versions/
    ├── 2.1.126                   # 历史版本存档
    ├── 2.1.84
    └── 2.1.81

<project>/.claude.json            # 项目级 MCP 配置
```

### 排除（不同步）

```
.credentials.json     # OAuth token，安全风险
settings.local.json   # 机器相关权限
sessions/             # IDE 进程状态
cache/                # 可重建缓存
telemetry/            # 遥测数据
*.lock                # 锁文件
```

## 工作方式

类似 Git 的版本化同步，不是简单的文件上传下载：

```
┌─────────────┐      push       ┌─────────────┐
│   设备 A     │ ──────────────→ │   WebDAV     │
│ ~/.claude/  │                 │   云端存储    │
│ ~/.local/   │ ←────────────── │              │
└─────────────┘      pull       └─────────────┘
                                           ↕
┌─────────────┐      pull       ┌─────────────┐
│   设备 B     │ ←────────────── │   WebDAV     │
│ ~/.claude/  │                 │              │
│ ~/.local/   │ ──────────────→ │              │
└─────────────┘      push       └─────────────┘
```

每次 push 产生一个**快照**（类似 git commit），记录所有文件的哈希和状态。快照之间形成链式结构，可以沿链回溯任意历史版本。

### 快速上手

```bash
# 安装（计划支持）
# npm install -g cc-box
# 或直接下载二进制

# 首次使用：配置 WebDAV + 加密 + 创建初始快照
cc-box init

# 查看当前状态
cc-box status

# 推送配置和二进制到 WebDAV
cc-box push

# 在另一台设备上拉取
cc-box pull

# 查看历史
cc-box log

# 回滚到某个版本
cc-box revert snap_a1b2c3d4

# 管理二进制版本
cc-box binary list              # 列出已备份的版本
cc-box binary switch 2.1.84    # 切换到指定版本
cc-box binary backup           # 备份当前二进制到 WebDAV
```

## 核心特性

### 二进制版本管理

CC-Box 会备份 `~/.local/bin/` 下的 Claude 相关二进制文件，以及 `~/.local/share/claude/versions/` 中的历史版本。

```bash
cc-box binary list
#   VERSION     SIZE     DATE
#   2.1.126     243MB    2026-05-03
#   2.1.84      234MB    2026-04-28
#   2.1.81      232MB    2026-04-15

cc-box binary switch 2.1.84
#   当前版本 2.1.126 → 存入 ~/.local/share/claude/versions/2.1.126
#   从 WebDAV 下载 2.1.84 → ~/.local/bin/claude.exe
#   已切换到版本 2.1.84

cc-box binary push
#   正在上传 claude.exe (243MB) → WebDAV...
#   正在上传 uv.exe (65MB) → WebDAV...
#   历史版本 2.1.84 已存在云端，跳过

cc-box binary pull
#   云端当前版本: 2.1.126
#   正在下载 claude.exe (243MB) → ~/.local/bin/claude.exe
#   正在下载 uv.exe (65MB) → ~/.local/bin/uv.exe
```

**不做本地额外备份。** WebDAV 云端即是备份源，二进制文件通过流式传输直接从 WebDAV 写入目标位置，不在本地保留副本，避免双份 claude.exe 占用 ~500MB 空间。

### 端到端加密

所有上传到 WebDAV 的数据都经过加密：

```
用户密码 → Argon2id 派生 → AES-256-GCM 加密
```

同一密码在所有设备上生成相同的密钥。密码不存储，只存派生后的密钥。

### cc-switch 兼容

如果你同时使用 cc-cli（cc-switch）管理 API 配置，CC-Box 不会与之冲突：

- `settings.json` 使用 JSON 字段级合并，不整体替换
- `env` 字段按 key 合并，保留双方所有环境变量
- cc-switch 切换 API 后，pull 不会覆盖当前配置

### WebDAV 支持

| 服务 | 说明 |
|------|------|
| 坚果云 | 免费 1GB 足够（配置 < 10MB，二进止单独管理） |
| NextCloud | 自建或托管 |
| Alist | 支持多种后端存储 |
| Synology | NAS 自带 WebDAV |
| 自建 | 任何标准 WebDAV 服务器 |

## 命令一览

```
# 同步操作
cc-box init                     # 初始化（交互式向导）
cc-box push [-m MSG]            # 推送到 WebDAV
cc-box pull                     # 从 WebDAV 拉取
cc-box sync                     # pull + push
cc-box status                   # 查看本地/远程差异

# 版本历史
cc-box log [--oneline] [-n N]   # 查看快照历史
cc-box show <snapshot-id>       # 查看快照详情
cc-box diff [FILE]              # 查看文件差异
cc-box revert <snapshot-id>     # 回滚到指定快照

# 二进制管理
cc-box binary list              # 列出云端已有版本
cc-box binary push              # 上传本地二进制到 WebDAV
cc-box binary pull [VERSION]    # 从 WebDAV 下载版本（不指定则下载 current）
cc-box binary switch <VERSION>  # 切换 Claude 版本（从云端下载）
cc-box binary prune             # 清理云端旧版本

# 项目配置
cc-box project list             # 列出已追踪项目
cc-box project push [PATH]      # 推送 .claude.json
cc-box project pull             # 拉取项目配置

# 配置
cc-box config get <key>         # 查看配置
cc-box config set <key> <val>   # 修改配置

# 维护
cc-box gc                       # 清理云端过期数据
cc-box verify                   # 校验完整性
```

## 技术栈

- **语言**: Go — 单二进制，交叉编译 Win/Mac/Linux
- **同步**: WebDAV (RFC 4918)
- **加密**: AES-256-GCM + Argon2id
- **密钥存储**: 系统密钥环 (Keychain / Credential Manager / Secret Service)

## 项目状态

当前处于设计阶段，详细设计文档见 [DESIGN.md](./DESIGN.md)。

## License

MIT
