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

## 快速上手

### 编译安装

需要 Go 1.24+：

```bash
git clone https://github.com/maliaosaide/cc-box.git
cd cc-box
go build -o cc-box ./cmd/cc-box/
```

### 使用

```bash
# 首次使用：配置 WebDAV + 加密 + 创建初始快照
cc-box init

# 查看当前状态
cc-box status

# 推送配置到 WebDAV
cc-box push

# push 并附带提交信息
cc-box push -m "添加新的 skill"

# 仅查看将要推送的变更
cc-box push --dry-run

# 在另一台设备上拉取
cc-box pull

# 仅查看将要拉取的变更
cc-box pull --dry-run
```

## 核心特性

### 端到端加密

所有上传到 WebDAV 的数据都经过加密：

```
用户密码 → Argon2id 派生 → AES-256-GCM 加密
```

同一密码在所有设备上生成相同的密钥。密码不存储，只存派生后的密钥。

### 跨平台规范化

自动处理不同操作系统的差异，确保哈希计算一致：

| 维度 | 规范化规则 |
|------|-----------|
| 换行符 | CRLF → LF（仅影响哈希计算，不修改本地文件） |
| 路径分隔符 | `\` → `/` |
| 大小写 | Windows 上统一小写 |

### WebDAV 支持

已测试兼容：

| 服务 | 状态 | 说明 |
|------|------|------|
| Alist | 已测试 | Basic 认证、大文件上传、断点续传 |
| 坚果云 | 待测试 | 免费 1GB 足够（配置 < 10MB） |
| NextCloud | 待测试 | 完整 WebDAV + ETag 支持 |
| Synology | 待测试 | NAS 自带 WebDAV |
| 自建 | 待测试 | 任何标准 WebDAV 服务器 |

### cc-switch 兼容

如果你同时使用 cc-cli（cc-switch）管理 API 配置，CC-Box 不会与之冲突：

- `settings.json` 使用 JSON 字段级合并，不整体替换
- `env` 字段按 key 合并，保留双方所有环境变量
- cc-switch 切换 API 后，pull 不会覆盖当前配置

## 命令一览

```
# 同步操作（已实现）
cc-box init                     # 初始化（交互式向导）
cc-box push [-m MSG] [--dry-run] # 推送到 WebDAV
cc-box pull [--dry-run]         # 从 WebDAV 拉取
cc-box sync                     # pull + push 一步完成
cc-box status                   # 查看本地/远程差异

# 版本历史（已实现）
cc-box log [--oneline] [-n N]   # 查看快照历史
cc-box show <snapshot-id>       # 查看快照详情
cc-box revert <snapshot-id>     # 回滚到指定快照

# 冲突处理（已实现）
cc-box conflicts                # 列出未解决的冲突文件
cc-box resolve <file>           # 交互式解决文件冲突

# 二进制管理（已实现）
cc-box binary list              # 列出云端已有版本
cc-box binary push              # 上传本地二进制到 WebDAV
cc-box binary pull [VERSION]    # 从 WebDAV 下载版本
cc-box binary switch <VERSION>  # 切换 Claude 版本
cc-box binary prune             # 清理云端旧版本

# 配置（已实现）
cc-box config get <key>         # 查看配置
cc-box config set <key> <val>   # 修改配置
cc-box config webdav            # 重新配置 WebDAV 连接
cc-box config rekey             # 更改加密密码（密钥轮转）

# 维护（已实现）
cc-box backup                   # 创建本地完整备份
cc-box restore                  # 从备份恢复
cc-box verify                   # 校验本地/远程完整性
cc-box gc                       # 清理云端过期数据

# 项目配置（Phase 3）
cc-box project list             # 列出已追踪项目
cc-box project push [PATH]      # 推送 .claude.json
cc-box project pull             # 拉取项目配置
```

## 技术栈

- **语言**: Go 1.24+ — 单二进制，交叉编译 Win/Mac/Linux
- **CLI**: Cobra
- **同步**: WebDAV (RFC 4918)
- **加密**: AES-256-GCM + Argon2id
- **密钥存储**: 系统密钥环 (Keychain / Credential Manager / Secret Service)

## 项目结构

```
cc-box/
├── cmd/cc-box/main.go           # 入口
├── internal/
│   ├── cli/                     # CLI 命令（init/push/pull/status/log/revert/binary/config...）
│   ├── config/                  # 配置管理 + 密钥环
│   ├── webdav/                  # WebDAV 客户端（ETag 乐观锁）
│   ├── snapshot/                # 快照管理 + 文件扫描器 + Diff
│   ├── normalize/               # 跨平台规范化
│   ├── crypto/                  # 端到端加密（Argon2id + AES-256-GCM）
│   ├── object/                  # Object 存储管理（哈希去重）
│   ├── sync/                    # 三方合并引擎（文本/JSON/history.jsonl）
│   └── binary/                  # 二进制分块上传/下载 + 版本索引
├── go.mod
├── Makefile
└── DESIGN.md                    # 详细设计文档
```

## 测试

```bash
# 单元测试
go test ./internal/...

# 集成测试（需要 WebDAV 服务）
# 修改 internal/integration_test.go 中的连接信息后运行
go test ./internal/ -run TestWebDAV -v
go test ./internal/ -run TestFullSyncFlow -v
go test ./internal/ -run TestPhase2 -v
```

当前 37 个测试通过：8 crypto + 6 normalize + 6 object + 7 snapshot + 9 sync + 7 集成。

## 项目状态

- [x] **Phase 1** — MVP：init → push → pull → status 完整流程
- [x] **Phase 2** — 三方合并引擎、二进制版本管理、密钥轮转、log/revert/conflicts 等命令
- [ ] **Phase 3** — GUI (Wails + Svelte)、项目配置同步
- [ ] **Phase 4** — 测试覆盖、多平台发布、打磨

详细设计文档见 [DESIGN.md](./DESIGN.md)。

## License

MIT
