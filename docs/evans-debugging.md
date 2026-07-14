# 使用 Evans 调试 gRPC

本示例通过 gRPC Server Reflection 暴露服务描述，因此 Evans 不需要本地 proto 文件就能发现并调用 RPC。

## 1. 安装项目工具

仓库只使用 mise 管理工具。首次进入项目时执行：

```bash
mise trust
mise install -j 1
```

使用单并发安装可以降低公开下载服务连接不稳定时出现 broken pipe 的概率；项目已经配置下载重试和超时。

## 2. 启动服务

在第一个终端启动使用内存 SQLite 的服务：

```bash
DATABASE_URL="file:evans-test?mode=memory&cache=shared" mise run run
```

预期日志：

```text
INFO server started grpc=:8080 http=:8081
```

该数据库只在进程生命周期内存在，重启服务后数据会清空。

## 3. 检查 reflection

在第二个终端执行一次性 CLI 检查：

```bash
mise run debug-grpc-list
```

预期至少包含：

```text
demo.user.v1.UserService
grpc.health.v1.Health
grpc.reflection.v1.ServerReflection
```

`debug-grpc-list` 输出后会立即退出。`show`、`package`、`service` 和 `call` 是 Evans REPL 命令，不能在 zsh 中直接运行。

## 4. 进入 REPL

```bash
mise run debug-grpc
```

看到 `localhost:8080>` 提示符后，选择业务服务：

```text
show package
package demo.user.v1
show service
service UserService
show rpc
```

提示符变为 `demo.user.v1.UserService@localhost:8080>`，表示 package 和 service 已选中。

## 5. 验证请求校验

调用创建接口：

```text
call CreateUser
```

先输入一个非法邮箱：

```text
name (TYPE_STRING) => mike
email (TYPE_STRING) => invalid-email
```

Protovalidate 会返回 `InvalidArgument`：

```text
rpc error: code = InvalidArgument desc = validation error: email: must be a valid email address
```

再次调用并输入合法数据：

```text
call CreateUser
name (TYPE_STRING) => mike
email (TYPE_STRING) => 123@qq.com
```

预期返回包含数据库生成的 ID：

```json
{
  "user": {
    "id": "user-001",
    "name": "mike",
    "email": "123@qq.com"
  }
}
```

## 6. 查询、分页和服务端流

查询刚创建的用户：

```text
call GetUser
id (TYPE_STRING) => user-001
```

列出用户时可将 `page_size` 设置为 `20`，`page_token` 留空：

```text
call ListUsers
```

读取服务端流：

```text
call WatchUsers
```

当前 Demo 的 `WatchUsers` 会按 ID 顺序发送调用时已有用户的快照，然后结束流；它不是持续监听数据库变化的订阅。

## 7. 检查标准 Health 服务

切换到 Health 服务：

```text
package grpc.health.v1
service Health
call Check
```

`service` 字段留空表示检查整个进程，预期状态为 `SERVING`。

## 8. 退出与常见问题

输入 `exit` 或按 `Ctrl-D` 退出 Evans。

- `zsh: command not found: show`：仍在 shell；先执行 `mise run debug-grpc`。
- `connection refused`：确认服务进程仍在运行，并检查 `lsof -nP -iTCP:8080 -sTCP:LISTEN`。
- `AlreadyExists`：当前数据库已存在相同邮箱；更换邮箱或重启内存数据库服务。
- 调试其他地址：`GRPC_HOST="127.0.0.1" GRPC_PORT="50051" mise run debug-grpc`。
