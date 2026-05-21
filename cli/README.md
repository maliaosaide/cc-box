# CC-Box CLI

CC-Box CLI 是 CC-Box 的命令行版本，用来在终端中管理 Claude Code 配置同步、快照历史、冲突处理、项目配置和 Claude 二进制版本。

它适合这些场景：

- 在服务器、远程终端或无图形界面的环境中使用。
- 用脚本自动备份或同步 Claude Code 配置。
- 快速查看配置差异、历史快照和冲突状态。
- 管理 Claude 二进制文件的备份、下载和版本切换。

## 能做什么

### 初始化和同步

```bash
cc-box init
cc-box status
cc-box push
cc-box pull
cc-box sync
```

CLI 可以初始化 WebDAV、配置设备信息和加密密码，并在多台设备间同步 Claude Code 配置。

### 快照、历史和回滚

```bash
cc-box log
cc-box show <snapshot-id>
cc-box diff [FILE]
cc-box revert <snapshot-id>
```

每次同步都会产生快照。你可以查看历史、检查文件差异，也可以回滚到指定快照。

### 冲突处理

```bash
cc-box conflicts
cc-box resolve <file>
```

当本地和远程同时修改同一文件时，CLI 会保留冲突信息，并提供命令进行查看和解决。

### 配置管理

```bash
cc-box config get <key>
cc-box config set <key> <value>
cc-box config webdav
cc-box config rekey
```

支持查看和修改配置、重新配置 WebDAV，以及更改加密密码。

### Claude 二进制管理

```bash
cc-box binary list
cc-box binary push
cc-box binary pull [VERSION]
cc-box binary switch <VERSION>
cc-box binary prune
```

可以把 Claude 二进制文件备份到 WebDAV，也可以从历史版本中下载和切换。

### 项目级配置同步

```bash
cc-box project list
cc-box project push [PATH]
cc-box project pull
cc-box project orphans
```

用于同步项目内的 `.claude.json`，让多台设备保持项目级 MCP 和工具配置一致。

### 维护命令

```bash
cc-box backup
cc-box restore [snapshot-id]
cc-box verify
cc-box gc
```

用于本地备份、恢复、完整性检查和清理远程旧对象。

## 快速开始

构建 CLI：

```bash
go build -o build/bin/cc-box.exe ./cmd/cc-box/
```

查看帮助：

```bash
./build/bin/cc-box.exe --help
```

首次初始化：

```bash
./build/bin/cc-box.exe init
```

同步配置：

```bash
./build/bin/cc-box.exe status
./build/bin/cc-box.exe push
./build/bin/cc-box.exe pull
```

如果希望在脚本中避免输入 WebDAV 密码，可以设置环境变量：

```bash
CC_BOX_WEBDAV_PASSWORD=your-password ./build/bin/cc-box.exe status
```

## 常用命令速查

| 命令 | 作用 |
| --- | --- |
| `init` | 初始化 CC-Box。 |
| `status` | 查看本地和远程同步状态。 |
| `push` | 推送本地配置到远程。 |
| `pull` | 拉取远程配置到本地。 |
| `sync` | 先拉取再推送。 |
| `diff [FILE]` | 查看文件差异。 |
| `log` | 查看快照历史。 |
| `show <snapshot-id>` | 查看快照详情。 |
| `revert <snapshot-id>` | 回滚到指定快照。 |
| `conflicts` | 查看冲突文件。 |
| `resolve <file>` | 解决指定冲突。 |
| `config` | 管理本地配置和 WebDAV 配置。 |
| `device` | 管理已注册设备。 |
| `binary` | 管理 Claude 二进制版本。 |
| `project` | 管理项目级 `.claude.json`。 |
| `backup` / `restore` | 本地备份与恢复。 |
| `verify` | 校验本地和远程状态。 |
| `gc` | 清理远程旧对象。 |

全局选项：

| 选项 | 作用 |
| --- | --- |
| `--allow-http` | 允许使用 HTTP WebDAV 地址。 |
| `-q, --quiet` | 减少命令输出。 |

## 目录结构

```text
cli/
├── cmd/cc-box/                  # CLI 启动入口
├── internal/cli/                # 命令定义和命令执行逻辑
├── internal/config/             # 配置管理
├── internal/crypto/             # 加密密码、密钥派生、数据加密
├── internal/webdav/             # WebDAV 访问
├── internal/object/             # 对象存储和哈希
├── internal/snapshot/           # 快照、扫描、diff
├── internal/sync/               # 合并和冲突处理
├── internal/binary/             # Claude 二进制管理
├── internal/project/            # 项目级配置同步
├── internal/normalize/          # 跨平台路径和内容规范化
├── go.mod
└── go.sum
```

## 技术栈

| 依赖 | 用途 |
| --- | --- |
| Go 1.25+ | CLI 编译和测试。 |
| Cobra | 命令行命令和参数解析。 |
| Viper | 配置读取和保存。 |
| golang.org/x/crypto | Argon2id 和加密相关能力。 |
| WebDAV | 远程配置和对象存储。 |

## 构建

在 `cli/` 目录执行：

```bash
go build -o build/bin/cc-box.exe ./cmd/cc-box/
```

从仓库根目录也可以执行：

```bash
make build-cli
```

构建产物：

```text
cli/build/bin/cc-box.exe
```

## 测试

在 `cli/` 目录执行：

```bash
go test ./...
```

从仓库根目录也可以执行：

```bash
make test-cli
```

测试覆盖配置、加密、快照、合并、WebDAV、二进制管理、项目配置同步等主要能力。

## 与 GUI 的关系

CLI 和 GUI 是两个独立应用。CLI 可以单独开发、构建和发布，不需要依赖 GUI。

如果你只需要命令行能力，只关注当前 `cli/` 目录即可。
