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
| `createLLMAPIKey(name)` | 程序化签发一把 user 类型的 LLM API Key（默认 scopes：`read_channels`、`write_requests`）。名称在项目内必须唯一，重名会被拒绝 |
| `updateAPIKeyProfiles(id, name, input)` | 整体替换某把 API Key 的 profiles 列表（包含 `activeProfile`）。`id` 与 `name` 二选一定位目标 Key |
| `loadApiKeyProfileTemplate(input)` | 把项目下的某个 `APIKeyProfileTemplate` 追加到目标 API Key 的 profiles（自动重命名避冲突，不动 `activeProfile`）。模板用 `templateID`/`templateName` 二选一、目标 Key 用 `apiKeyID`/`apiKeyName` 二选一定位，可混用（如 `templateName` + `apiKeyID`） |

> "应用模板并立即生效"的语义需要两步：先 `loadApiKeyProfileTemplate`，再 `updateAPIKeyProfiles` 把 `activeProfile` 切到新 profile。

## 当前可用的 Queries

| Query | 作用 |
|---|---|
| `apiKey(id, key, name)` | 查询某把 API Key 的详情（`id` / 明文 `key` / `name` / `scopes` / `profiles`）。三个参数三选一；手里只有名称时用它先解析出 id/key |
| `apiKeyQuotaUsages(apiKeyId, key, name)` | 查询某把 API Key 中启用了额度（quota）的各 profile 的实时用量（`requestCount` / `totalTokens` / `totalCost` 及统计窗口）。`apiKeyId` / `key` / `name` 三选一 |

> ⚠️ `apiKey` 与 `apiKeyQuotaUsages` 都需要 `read_api_keys` 权限，且只能查询调用方 service account **所属项目内**的 Key；跨项目或不存在的 Key 统一表现为查不到（不泄漏 Key 是否存在）。按 `name` 查询同理——名称只在本项目内解析。
>
> ⚠️ 服务端只接受 **POST**：`/openapi/v1/graphql` 已不再注册 GET transport（GET 请求返回 `400 transport not supported`）。这是为了确保明文 `key` 等参数不会经 URL 查询串（`?variables={"key":...}`）泄漏到反向代理 / 访问日志 / 浏览器历史。genqlient 客户端默认即用 POST，无需额外配置。

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

# 可选: 设置其一即可让示例额外演示 apiKeyQuotaUsages 查询
# export AXONHUB_QUERY_KEY_ID="gid://axonhub/APIKey/123"   # 按 GUID 查
# export AXONHUB_QUERY_KEY="ah-xxxxxxxx"                    # 按明文 Key 查
# export AXONHUB_QUERY_KEY_NAME="my-llm-key"                # 按名称查（项目内唯一）

go run main.go
```

示例程序会依次演示：创建一把 LLM Key → 按名称反查该 Key 的详情（`apiKey` 查询）→（可选）查询额度用量。

## 使用注意点

### 认证与权限

- **认证**: 所有的 OpenAPI 请求都必须包含 `Authorization: Bearer <API_KEY>` 请求头。
- **Key 类型**: 只有 **Service Account** 类型的 API Key 才能访问 OpenAPI 接口。普通的 User 类型 Key 将被拒绝。
- **Scope 权限**:
  - `createLLMAPIKey` — 需要 `write_api_keys`
  - `updateAPIKeyProfiles` — 需要 `read_api_keys` + `write_api_keys`
  - `loadApiKeyProfileTemplate` — 需要 `read_api_keys` + `write_api_keys`
  - `apiKey` — 需要 `read_api_keys`（只读）
  - `apiKeyQuotaUsages` — 需要 `read_api_keys`（只读）

### 接口行为

- **默认 LLM Key 权限**: 通过 `createLLMAPIKey` 创建的新 Key 将默认拥有 `read_channels` 和 `write_requests` 权限，适用于常规的 LLM 调用。
- **名称即标识符**: API Key 的 `name` 在项目内唯一（`createLLMAPIKey` 重名会被拒绝），模板的 `name` 同样在项目内唯一。因此凡是接受标识参数的接口，都支持 id 与 name 并存、按其一定位（`updateAPIKeyProfiles` 二选一、`apiKey` / `apiKeyQuotaUsages` 三选一、`loadApiKeyProfileTemplate` 的模板与目标 Key 各二选一）。同时提供多个或一个都不提供都会报错。
- **同项目约束**: 所有 mutation 与 query 仅能作用于调用方 service account 所属的项目；跨项目的 `apiKeyID` / `key` / `name` / `templateID` / `templateName` 会被拒绝（查不到）。
- **GUID 类型校验**: 所有 `ID` 参数必须是对应类型的 GUID（如 `gid://axonhub/APIKey/123`、`gid://axonhub/APIKeyProfileTemplate/45`）；类型不匹配会被直接拒绝。
- **Profile 命名冲突**: `loadApiKeyProfileTemplate` 在追加时若发现同名 profile，会自动加 `(1)` / `(2)` 后缀，不会覆盖。
- **整体替换语义**: `updateAPIKeyProfiles` 是**整体替换**——传入的 profiles 列表会完全覆盖原有的，且 `activeProfile` 必须存在于列表中。
- **Schema 同步**: 如果 AxonHub 后端的 `openapi.graphql` 发生了变化，你需要同步更新 `graphql/openapi.graphql` 并重新生成代码。
- **端点地址**: 默认端点为 `http://localhost:8090/openapi/v1/graphql`。

## 常见问题

- **401 Unauthorized**: 请检查你的 API Key 是否为 `service_account` 类型，且请求头格式是否正确（`Bearer ` 前缀）。
- **权限拒绝 (Deny)**: 请检查该 Key 是否关联了对应 mutation 所需的 scope（详见上文）。
- **跨项目错误**: 检查 `apiKeyID` / `templateID`（或对应的 name）是否与当前 service account key 同属一个项目。
- **按 name 查询报 "not singular"**: 历史版本的 `createLLMAPIKey` 不校验重名，项目里可能残留同名 Key。此时请改用 id 或明文 key 定位，并在管理后台把重名 Key 改名。
