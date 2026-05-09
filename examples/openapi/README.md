# AxonHub OpenAPI 示例

这个目录展示了如何使用 [genqlient](https://github.com/Khan/genqlient) 生成 Go 客户端代码，以便通过 GraphQL 调用 AxonHub 的管理接口。

## 简介

AxonHub 提供了一个专用的 GraphQL 端点 `/openapi/v1/graphql` 用于程序化管理 LLM API Key。这个示例演示了如何生成并使用 Go 代码来集成这些功能。

## 目录结构

- `graphql/openapi.graphql`: AxonHub OpenAPI 的 GraphQL Schema 定义。
- `graphql/api_key.graphql`: 定义了具体的操作（Mutation/Query）。
- `graphql/genqlient.yaml`: `genqlient` 的配置文件。
- `graphql/generated.go`: 自动生成的 Go 客户端代码。
- `main.go`: 使用生成代码的示例程序。

## 当前可用的 Mutations

| Mutation | 作用 |
|---|---|
| `createLLMAPIKey(name)` | 程序化签发一把 user 类型的 LLM API Key（默认 scopes：`read_channels`、`write_requests`） |
| `updateAPIKeyProfiles(id, input)` | 整体替换某把 API Key 的 profiles 列表（包含 `activeProfile`） |
| `loadApiKeyProfileTemplate(input)` | 把项目下的某个 `APIKeyProfileTemplate` 追加到目标 API Key 的 profiles（自动重命名避冲突，不动 `activeProfile`） |

> "应用模板并立即生效"的语义需要两步：先 `loadApiKeyProfileTemplate`，再 `updateAPIKeyProfiles` 把 `activeProfile` 切到新 profile。

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
2. 获取一个具有 `service_account` 类型且拥有 `read_api_keys` + `write_api_keys` 权限的 API Key。
3. 运行示例程序：

```bash
export AXONHUB_API_KEY="your_service_account_api_key"
go run main.go
```

## 使用注意点

### 认证与权限

- **认证**: 所有的 OpenAPI 请求都必须包含 `Authorization: Bearer <API_KEY>` 请求头。
- **Key 类型**: 只有 **Service Account** 类型的 API Key 才能访问 OpenAPI 接口。普通的 User 类型 Key 将被拒绝。
- **Scope 权限**:
  - `createLLMAPIKey` — 需要 `write_api_keys`
  - `updateAPIKeyProfiles` — 需要 `read_api_keys` + `write_api_keys`
  - `loadApiKeyProfileTemplate` — 需要 `read_api_keys` + `write_api_keys`

### 接口行为

- **默认 LLM Key 权限**: 通过 `createLLMAPIKey` 创建的新 Key 将默认拥有 `read_channels` 和 `write_requests` 权限，适用于常规的 LLM 调用。
- **同项目约束**: 所有 mutation 仅能作用于调用方 service account 所属的项目；跨项目的 `apiKeyID` / `templateID` 会被拒绝。
- **Profile 命名冲突**: `loadApiKeyProfileTemplate` 在追加时若发现同名 profile，会自动加 `(1)` / `(2)` 后缀，不会覆盖。
- **整体替换语义**: `updateAPIKeyProfiles` 是**整体替换**——传入的 profiles 列表会完全覆盖原有的，且 `activeProfile` 必须存在于列表中。
- **Schema 同步**: 如果 AxonHub 后端的 `openapi.graphql` 发生了变化，你需要同步更新 `graphql/openapi.graphql` 并重新生成代码。
- **端点地址**: 默认端点为 `http://localhost:8090/openapi/v1/graphql`。

## 常见问题

- **401 Unauthorized**: 请检查你的 API Key 是否为 `service_account` 类型，且请求头格式是否正确（`Bearer ` 前缀）。
- **权限拒绝 (Deny)**: 请检查该 Key 是否关联了对应 mutation 所需的 scope（详见上文）。
- **跨项目错误**: 检查 `apiKeyID` / `templateID` 是否与当前 service account key 同属一个项目。
