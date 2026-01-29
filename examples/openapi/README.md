# AxonHub OpenAPI 示例

这个目录展示了如何使用 [genqlient](https://github.com/Khan/genqlient) 生成 Go 客户端代码，以便通过 GraphQL 调用 AxonHub 的管理接口。

## 简介

AxonHub 提供了一个专用的 GraphQL 端点 `/openapi/v1/graphql` 用于管理资源（例如通过 API 动态创建 API Key）。这个示例演示了如何生成并使用 Go 代码来集成这些功能。

## 目录结构

- `graphql/openapi.graphql`: AxonHub OpenAPI 的 GraphQL Schema 定义。
- `graphql/api_key.graphql`: 定义了具体的操作（Mutation/Query）。
- `graphql/genqlient.yaml`: `genqlient` 的配置文件。
- `graphql/generated.go`: 自动生成的 Go 客户端代码。
- `main.go`: 使用生成代码的示例程序。

## 快速开始

### 1. 生成代码

如果你修改了 `.graphql` 文件或需要重新生成代码，请运行：

```bash
# 安装工具（如果尚未安装）
go get -tool github.com/Khan/genqlient@5b0aabc933fa38078f8525e38a322d3baa78320e

# 运行生成命令
cd graphql
go run github.com/Khan/genqlient
```

这将会根据 `graphql/*.graphql` 中的定义更新 `graphql/generated.go`。

### 2. 运行示例

1. 确保 AxonHub 服务器正在运行（默认端口 8090）。
2. 获取一个具有 `service_account` 类型且拥有 `write_api_keys` 权限的 API Key。
3. 运行示例程序：

```bash
export AXONHUB_API_KEY="your_service_account_api_key"
go run main.go
```

## 使用注意点

### 认证与权限

- **认证**: 所有的 OpenAPI 请求都必须包含 `Authorization: Bearer <API_KEY>` 请求头。
- **Key 类型**: 只有 **Service Account** 类型的 API Key 才能访问 OpenAPI 接口。普通的 User 类型 Key 将被拒绝。
- **Scope 权限**: 调用 `CreateLLMAPIKey` 接口需要调用方 Key 具有 `write_api_keys` 权限。

### 接口行为

- **默认权限**: 通过 `CreateLLMAPIKey` 创建的新 Key 将默认拥有 `read_channels` 和 `write_requests` 权限，适用于常规的 LLM 调用。
- **Schema 同步**: 如果 AxonHub 后端的 `openapi.graphql` 发生了变化，你需要同步更新 `graphql/openapi.graphql` 并重新生成代码。
- **端点地址**: 默认端点为 `http://localhost:8090/openapi/v1/graphql`。

## 常见问题

- **401 Unauthorized**: 请检查你的 API Key 是否为 `service_account` 类型，且请求头格式是否正确（`Bearer ` 前缀）。
- **权限拒绝 (Deny)**: 请检查该 Key 是否关联了 `write_api_keys` 权限。
