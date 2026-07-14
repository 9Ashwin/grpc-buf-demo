# grpc-buf-demo Agent 指南

本文件适用于仓库内所有目录。若子目录存在更具体的 `AGENTS.md`，以子目录规则为准。

## 项目目标

- 展示基于 Buf v2 的 gRPC API 定义、代码生成和兼容性管理。
- 所有依赖、Action、Buf 模块和生成插件必须公开可访问，不得引入公司内部域名、镜像、仓库或基础设施。
- 保持示例可独立运行；持久化只依赖进程内可启动的 SQLite，不要求外部数据库或消息队列。

## 目录职责

- `proto/`：手写 API 契约，是接口定义的唯一数据源。
- `gen/go/`：由 Buf 生成的 Go、gRPC 和 grpc-gateway 代码。
- `gen/openapi/`：由 Buf 生成的 OpenAPI 文档。
- `db/migrations/`：goose 管理的 SQLite 版本化迁移。
- `db/query.sql`：sqlc 查询定义；`db/db.go`、`db/models.go`、`db/query.sql.go` 是生成产物。
- `db/open.go`：手写的 SQLite 连接、配置和嵌入式迁移入口。
- `user/`：`UserService` 的领域实现和真实 SQLite 测试。
- `cmd/server/`：gRPC 与 HTTP gateway 的进程入口和依赖装配。
- `integration/`：Testcontainers 驱动的容器黑盒测试；Docker 不可用时允许明确跳过。
- `.mise.toml`：唯一的本地工具版本和任务入口。

## 工具与任务

- 本项目只使用 mise 管理开发工具和任务，不新增 Makefile、justfile、Taskfile 或未纳入 mise 的平行入口。
- 首次使用运行 `mise trust` 和 `mise install`；日常命令统一使用 `mise run <task>`。
- 修改任务或工具版本后运行 `mise tasks validate --errors-only`，确保配置可解析且任务依赖无环。
- 工具必须固定明确版本，并从公开 registry、GitHub Release、Go module proxy 或 BSR 获取。

## Proto 与 Buf

- 使用 `buf.yaml` 中的 `STANDARD` lint 和 `FILE` breaking 规则，不通过规则例外绕过正常问题。
- proto package 必须带版本后缀，并与文件目录一致，例如 `demo.user.v1` 对应 `demo/user/v1/`。
- RPC 使用独立且语义明确的请求、响应消息，遵循 Buf 的命名规则。
- 基础入参约束优先声明为 Protovalidate 规则；业务规则留在 Go 实现中。
- 删除字段时必须用 `reserved` 保留原字段编号和名称，不得复用已发布编号。
- 语言包路径由 `buf.gen.yaml` 的 Managed Mode 管理，不在本项目 proto 中添加 `go_package`。
- 外部模块必须排除在本项目 `go_package_prefix` 覆盖之外。
- 远程插件必须固定版本；新增依赖后运行 `buf dep update` 并提交更新后的 `buf.lock`。

## 生成代码

- 禁止手工编辑 `gen/` 下的文件；修改 proto 或生成配置后运行 `mise run generate`。
- 禁止手工编辑 sqlc 生成的 `db/db.go`、`db/models.go` 和 `db/query.sql.go`；修改 schema 或 query 后运行 `mise run generate`。
- proto、生成配置和生成产物必须在同一次变更中保持一致。
- 生成后确认再次执行 `mise run generate` 不会产生差异。

## Go 代码

- 遵循现有扁平、按领域划分的包结构，不引入 controller/service/repository 等空洞分层。
- 使用标准 gRPC status code 表达客户端可识别的错误。
- 所有请求入口执行契约校验；并发访问共享状态时保持数据一致性。
- goroutine 必须有明确退出路径；服务进程必须保留优雅关闭和超时设置。
- 错误返回时补充操作上下文，不在同一层既记录又返回同一个错误。
- 不扩大公共 API，不引入未使用依赖或仅为未来需求准备的抽象。
- 数据访问直接使用 sqlc 生成的具体类型，不为单一 SQLite 实现增加 repository 接口。
- 数据库结构变更必须新增 goose 迁移，禁止修改已经发布的迁移文件。
- SQLite 连接、事务和约束错误必须有明确处理；测试使用真实 SQLite，不 mock SQL。

## 修改与验证

优先执行：

```bash
mise run format
mise run verify
```

至少保证以下检查通过：

```bash
mise tasks validate --errors-only
mise run verify
```

涉及容器、Dockerfile 或进程装配时，额外运行 `RUN_CONTAINER_TESTS=1 mise run test`；Docker 或公共镜像仓库不可用时，记录明确的环境阻塞原因。

GitHub Actions 必须使用公开 Action、最小权限和固定主版本，并检查生成代码是否与仓库一致。不得在未获得用户明确确认时执行 `git commit`、`git push` 或修改远端仓库状态。
