# CC-Box CLI

`cli/` 是 CC-Box 的命令行应用，已经与 GUI 完全拆离。它拥有自己的 Go module、入口、业务代码、依赖文件和构建产物，可以单独开发、测试和发布。

## 模块边界

- 模块路径：`github.com/user/cc-box/cli`
- 入口文件：`cmd/cc-box/main.go`
- 命令实现：`internal/cli/`
- CLI 只引用 `github.com/user/cc-box/cli/internal/...`
- CLI 不引用 `gui/`，也不依赖根目录 Go module

如果需要修改同步、加密、快照、WebDAV、二进制管理等底层逻辑，只改这里不会自动影响 GUI；GUI 有自己的 `internal/` 副本。

## 目录结构

```text
cli/
├── cmd/
│   └── cc-box/
│       └── main.go              # CLI 入口，调用 internal/cli.Execute
├── internal/
│   ├── cli/                     # Cobra 命令层
│   ├── config/                  # 配置读写、路径解析、密钥环集成
│   ├── crypto/                  # Argon2id 派生、AES-256-GCM 加密、密钥指纹
│   ├── webdav/                  # WebDAV 客户端、锁、XML 解析
│   ├── object/                  # 对象哈希、对象存储读写
│   ├── snapshot/                # 快照模型、扫描、差异计算
│   ├── sync/                    # 文本/JSON/history 三方合并与冲突处理
│   ├── binary/                  # Claude 二进制探测、分块上传、版本切换
│   ├── project/                 # 项目级 .claude.json 同步与合并
│   └── normalize/               # 路径、换行、内容哈希规范化
├── build/
│   └── bin/                     # CLI 构建产物目录，已被 .gitignore 忽略
├── go.mod
└── go.sum
```

## 命令能力

顶层命令为 `cc-box`，由 `internal/cli/root.go` 注册全局选项：

| 全局选项 | 说明 |
| --- | --- |
| `--allow-http` | 允许 HTTP WebDAV 连接 |
| `-q, --quiet` | 减少命令输出 |

当前已实现的命令：

| 命令 | 说明 |
| --- | --- |
| `init` | 初始化 CC-Box，配置 WebDAV、设备和加密密码 |
| `status` | 查看本地与远程同步状态 |
| `push` | 推送本地配置变更到远程 |
| `pull` | 拉取远程配置变更到本地 |
| `sync` | 先拉取再推送，相当于 pull + push |
| `diff [FILE]` | 查看文件内容差异 |
| `log` | 查看快照历史 |
| `show <snapshot-id>` | 查看指定快照详情 |
| `revert <snapshot-id>` | 回滚到指定快照 |
| `conflicts` | 列出未解决的冲突文件 |
| `resolve <file>` | 交互式解决指定冲突 |
| `config get/set/rekey/webdav` | 查看、修改配置，轮转加密密码，重配 WebDAV |
| `device list/rename/forget` | 查看、重命名、移除设备 |
| `binary list/push/pull/switch/prune` | 管理 Claude 二进制备份和版本切换 |
| `project list/push/pull/orphans` | 同步项目级 `.claude.json` |
| `backup` / `restore` | 本地备份与恢复 |
| `verify` | 校验本地文件完整性和远程可达性 |
| `gc` | 清理过期 objects 和快照 |

## 构建

在 `cli/` 目录内构建：

```bash
go build -o build/bin/cc-box.exe ./cmd/cc-box/
```

也可以从仓库根目录执行：

```bash
make build-cli
```

构建产物：

```text
cli/build/bin/cc-box.exe
```

## 测试

在 `cli/` 目录内运行全部测试：

```bash
go test ./...
```

也可以从仓库根目录执行：

```bash
make test-cli
```

测试覆盖的主要范围：

- 加密与密钥派生
- 路径和内容规范化
- 对象哈希与对象存储
- 快照序列化、扫描和 diff
- 文本、JSON、history 合并
- 项目配置合并
- 二进制分块上传、版本解析和切换安全性
- WebDAV 连接、乐观锁和完整同步流程

## 运行

构建后运行：

```bash
./build/bin/cc-box.exe --help
./build/bin/cc-box.exe status
./build/bin/cc-box.exe push
./build/bin/cc-box.exe pull
```

WebDAV 密码可以通过环境变量注入，避免交互输入：

```bash
CC_BOX_WEBDAV_PASSWORD=your-password ./build/bin/cc-box.exe status
```

## 开发约束

- CLI 的业务代码留在 `cli/internal/`。
- 不要从 CLI import `github.com/user/cc-box/gui/...`。
- 不要恢复对根目录 `github.com/user/cc-box/internal/...` 的引用。
- 如果需要 CLI 和 GUI 行为一致，分别修改 `cli/internal/` 和 `gui/internal/` 中对应代码。
