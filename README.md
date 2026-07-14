# grpc-buf-demo

一个不依赖公司内部基础设施的 Buf + gRPC API 生成与管理示例。仓库以 `.proto` 为契约源，通过公开 BSR 模块和远程插件生成 Go、gRPC、grpc-gateway 与 OpenAPI 产物。

## 包含内容

- Buf v2 workspace、依赖锁定、STANDARD lint 与 FILE breaking policy
- Managed Mode 统一管理 Go package prefix，proto 不写语言专属包路径
- Protovalidate 声明式请求校验
- gRPC unary RPC、服务端流式 RPC、HTTP/JSON 映射与 OpenAPI 文档
- GitHub Actions 检查格式、lint、破坏性变更、生成代码漂移和 Go 测试
- 内存实现与 `bufconn` 集成测试，无数据库和外部服务依赖

## 目录

```text
proto/          API 契约，唯一手写的接口定义
gen/go/         Buf 生成的 Go、gRPC 和 gateway 代码
gen/openapi/    Buf 生成的 OpenAPI v2 文档
user/           UserService 的内存实现与测试
cmd/server/     同时启动 gRPC（8080）和 HTTP（8081）
```

## 本地使用

需要 Go 1.26+ 和 Buf 1.54+。代码生成插件由 BSR 远程执行，本机不需要安装 `protoc-gen-go` 等插件。

```bash
make generate
make verify
make run
```

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

## API 变更流程

1. 修改 `proto/` 下的契约，并用 `reserved` 保留删除字段的编号和名称。
2. 运行 `make format generate test`，提交 proto 与生成产物。
3. Pull Request 中的 Buf Action 自动对比目标分支，阻止 FILE 级破坏性变更。
4. CI 再次生成代码；若 `gen/` 出现 diff，说明生成产物或插件版本未同步。

示例将生成产物保存在同一仓库，便于直接运行和审阅。大型多语言项目可保持同样的 proto/CI 规则，再把各语言产物发布到独立 SDK 仓库或 BSR Generated SDK。
