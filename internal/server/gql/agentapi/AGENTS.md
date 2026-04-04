# Agent API GraphQL Schema

此目录包含 AxonHub Agent API 的 GraphQL Schema 定义和服务端 Resolvers。

## 目录结构

| 文件 | 说明 |
|------|------|
| `agentapi.graphql` | GraphQL Schema 定义，包含所有类型、查询和变更 |
| `agentapi.resolvers.go` | Resolver 实现，处理 GraphQL 操作的业务逻辑 |
| `resolver.go` | Resolver 结构体定义和 Schema 初始化 |
| `gqlgen.yml` | gqlgen 配置文件 |
| `generated.go` | gqlgen 自动生成的代码 |
| `models_gen.go` | gqlgen 自动生成的模型代码 |
| `helpers.go` | 辅助函数 |

## API 概览

### Query

| 操作 | 说明 |
|------|------|
| `agentBootstrap` | 获取 Agent 引导配置（ID、名称、模型、系统提示词、工具、技能等） |
| `peerAgents` | 获取同一项目中发现的其他 Agent 实例 |
| `pullAgentMessages` | 拉取发送给当前 Agent 的消息 |
| `pullAgentMessagesToUser` | 拉取发送给用户的消息 |

### Mutation

| 操作 | 说明 |
|------|------|
| `registerAgentInstance` | 注册新的 Agent 实例 |
| `heartbeatAgentInstance` | 发送心跳保持实例活跃 |
| `sendAgentMessage` | 向其他 Agent 发送消息（Agent 间通信） |
| `replyMessage` | Agent 回复消息给用户 |
| `ackAgentMessages` | 确认已处理的消息 |
| `deployAxonClaw` | 部署 AxonClaw 实例 |

## 代码生成

使用 gqlgen 生成代码：

```bash
go generate ./...
```

## 重要：同步更新客户端代码

**当修改此目录下的 Schema 后，必须同步更新客户端代码！**

客户端代码位于：`/axon/api/`

### 同步步骤

1. **更新 Schema 后**，检查 `axon/api/agent.graphql` 中的操作是否需要更新
2. **重新生成客户端代码**：

```bash
cd axon/api
go run github.com/Khan/genqlient
```

### 客户端配置

客户端通过 `genqlient.yaml` 配置指向此 Schema：

```yaml
schema: ../../../internal/server/gql/agentapi/agentapi.graphql
```

## 认证

所有 Agent API 请求需要通过 API Key 认证：
- 请求头：`Authorization: Bearer <API_KEY>`
- 端点：`/agent/v1/graphql`

## 相关服务

Resolver 依赖以下 Biz 服务：

- `biz.AgentBootstrapService` - Agent 引导、实例管理、消息处理
- `biz.AgentDeployService` - Agent 部署服务
