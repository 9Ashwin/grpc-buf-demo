# grpc-buf-demo

一个不依赖公司内部基础设施的 Buf + gRPC API 生成与管理示例。仓库以 `.proto` 为契约源，通过公开 BSR 模块和远程插件生成 Go、gRPC、grpc-gateway 与 OpenAPI 产物。

## 包含内容

- Buf v2 workspace、依赖锁定、STANDARD lint 与 FILE breaking policy
- Managed Mode 统一管理 Go package prefix，proto 不写语言专属包路径
- Protovalidate 声明式请求校验
- gRPC unary RPC、服务端流式 RPC、HTTP/JSON 映射与 OpenAPI 文档
- Evans REPL + 服务端 reflection，直接观察和调用 gRPC API
- sqlc + SQLite 类型安全数据访问，goose 自动执行版本化迁移
- mise 固定工具版本、管理开发任务与依赖图，Air 支持热重载
- golangci-lint、govulncheck 与 Testcontainers 黑盒测试
- GitHub Actions 检查格式、lint、破坏性变更、生成代码漂移和 Go 测试
- 标准 gRPC Health、reflection 与 graceful shutdown

## 目录

```text
proto/          API 契约，唯一手写的接口定义
gen/go/         Buf 生成的 Go、gRPC 和 gateway 代码
gen/openapi/    Buf 生成的 OpenAPI v2 文档
db/migrations/  goose 管理的 SQLite 迁移
db/query.sql    sqlc 查询定义；生成 db.go、models.go、query.sql.go
db/open.go      SQLite 连接与嵌入式迁移入口
user/           UserService 实现与真实 SQLite 测试
cmd/server/     同时启动 gRPC（8080）和 HTTP（8081）
integration/    Testcontainers 容器黑盒测试
.mise.toml      唯一的工具版本与本地任务配置
```

## 本地使用

本地只需要预装 [mise](https://mise.jdx.dev/)。Go、Buf、sqlc、goose、Evans、Air、golangci-lint 和 govulncheck 均由 `.mise.toml` 固定版本，不需要单独用 Homebrew 或 `go install` 安装。项目为公开工具下载开启重试和两分钟超时；sqlc 与 govulncheck 使用 mise 的 Go backend，并固定公开的 `proxy.golang.org,direct` 和 Google checksum mirror，避免用户环境中的内部代理。Buf 代码生成插件由 BSR 远程执行，本机也不需要安装 `protoc-gen-go`。

```bash
mise trust
mise install -j 1
mise tasks ls
mise run verify
mise run run
```

`mise trust` 只需在首次进入仓库时执行。此后通过 `mise run <task>` 使用项目任务，不再维护 Make、just 或 Task 等并行入口。

服务默认使用仓库根目录的 `grpc-demo.db` 并在启动时自动执行 goose migration。可覆盖数据库地址：

```bash
DATABASE_URL="file:demo.db?_pragma=journal_mode(WAL)" mise run run
DATABASE_URL="demo.db" mise run db-status
```

修改查询或迁移后运行 `mise run generate`。不要手工编辑 sqlc 生成的 `db/db.go`、`db/models.go` 和 `db/query.sql.go`；`db/open.go` 是手写的连接和迁移装配代码。

服务启动后可通过 REST 调用：

```bash
curl -X POST "http://localhost:8081/v1/users" \
  -H "Content-Type: application/json" \
  -d '{"name":"Ada Lovelace","email":"ada@example.com"}'

curl "http://localhost:8081/v1/users?page_size=20"
```

也可利用服务端 reflection 直接调用 gRPC：

```bash
buf curl --protocol grpc \
  --http2-prior-knowledge \
  --data '{"id":"user-001"}' \
  "http://localhost:8080/demo.user.v1.UserService/GetUser"
```

## 使用 Evans 调试 gRPC

[Evans](https://github.com/ktr0731/evans) 是开源的 gRPC REPL。服务端已经启用 reflection，因此 Evans 能在运行时发现 package、service、message 和 RPC，无需编写临时 client 或手工指定 proto 文件。`mise install` 会安装项目固定的 Evans 版本。

完整的服务发现、请求校验、创建用户、分页、服务端流和 Health 检查示例见 [`docs/evans-debugging.md`](docs/evans-debugging.md)。

分别启动服务和 REPL：

```bash
# 终端 1
mise run run

# 终端 2
mise run debug-grpc
```

进入 REPL 后可以发现并调用接口：

```text
show package
package demo.user.v1
show service
service UserService
call CreateUser
call ListUsers
call WatchUsers
```

需要在脚本或健康检查中确认服务暴露情况时，使用 Evans CLI 模式：

```bash
mise run debug-grpc-list
```

默认连接 `localhost:8080`，可在调试其他环境时覆盖变量：

```bash
GRPC_HOST="127.0.0.1" GRPC_PORT="50051" mise run debug-grpc
```

## 开发检查

```bash
mise run dev      # Air 监听 Go、SQL、YAML 变化并重启服务
mise run lint     # Buf + golangci-lint
mise run vuln     # Go 官方漏洞数据库的可达性扫描
mise run verify   # 生成代码并执行格式、lint、测试、漏洞扫描和 go vet
```

`mise run test` 始终运行基于真实内存 SQLite 的服务测试。容器黑盒测试在 GitHub Actions 中自动开启；本地使用 `RUN_CONTAINER_TESTS=1 mise run test` 显式运行，它会通过 Testcontainers 构建 `Dockerfile` 并调用容器中的 gRPC 服务，Docker 不可用时明确跳过。

## API 变更流程

1. 修改 `proto/` 下的契约，并用 `reserved` 保留删除字段的编号和名称。
2. 运行 `mise run format` 和 `mise run verify`，提交 proto、SQL 与生成产物。
3. Pull Request 中的 Buf Action 自动对比目标分支，阻止 FILE 级破坏性变更。
4. CI 再次生成代码；若 `gen/` 出现 diff，说明生成产物或插件版本未同步。

示例将生成产物保存在同一仓库，便于直接运行和审阅。大型多语言项目可保持同样的 proto/CI 规则，再把各语言产物发布到独立 SDK 仓库或 BSR Generated SDK。

## License

本项目基于 [MIT License](LICENSE) 开源。
