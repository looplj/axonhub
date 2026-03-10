# AxonHub Agent API 示例

这个目录展示了如何使用 [genqlient](https://github.com/Khan/genqlient) 生成 Go 客户端代码，以便通过 GraphQL 调用 AxonHub 的 Agent 接口。

## 简介

AxonHub 提供了专用的 GraphQL 端点 `/agent/v1/graphql` 用于 Agent 相关的操作（例如获取 Agent 引导信息、注册实例、心跳、消息拉取/推送等）。这个示例演示了如何生成并使用 Go 代码来集成这些功能。

## 目录结构

- `agent.graphql`: AxonHub Agent API 的 GraphQL 操作定义（Query/Mutation）。
- `genqlient.yaml`: `genqlient` 的配置文件。
- `generated.go`: 自动生成的 Go 客户端代码。
- `client.go`: 封装了带有认证的 GraphQL 客户端创建函数。

## 快速开始

### 1. 生成代码

如果你修改了 `.graphql` 文件或需要重新生成代码，请运行：

```bash
# 安装工具（如果尚未安装）
go get -tool github.com/Khan/genqlient@5b0aabc933fa38078f8525e38a322d3baa78320e

# 运行生成命令
go run github.com/Khan/genqlient
```

这将会根据 `agent.graphql` 中的定义更新 `generated.go`。

### 2. 使用客户端

在代码中创建客户端并调用接口：

```go
package main

import (
    "fmt"
    "github.com/axon-hub/axonhub/cmd/axonclaw/api"
)

func main() {
    client := api.NewClient("http://localhost:8090", "your_agent_api_key")

    // 例如：获取 Agent 引导信息
    resp, err := AgentBootstrap(context.Background(), client)
    // ...
}
```

## API 概览

### Query

- **AgentBootstrap**: 获取 Agent 的引导配置信息，包括 Agent ID、名称、模型、系统提示词、工具、技能和内置工具等。

### Mutation

- **RegisterAgentInstance**: 注册一个新的 Agent 实例。
- **HeartbeatAgentInstance**: 发送 Agent 实例心跳，用于保活。
- **PullAgentMessages**: 拉取 Agent 消息。
- **AckAgentMessages**: 确认已处理的消息。
- **PushAgentMessage**: 向 Agent 推送消息。

## 使用注意点

### 认证

- 所有的 Agent API 请求都必须包含 `Authorization: Bearer <API_KEY>` 请求头。
- `client.go` 中的 `NewClient` 函数会自动添加认证头。

### 端点地址

- 默认端点为 `http://localhost:8090/agent/v1/graphql`。
- 实际使用时需确保 AxonHub 服务器正在运行。

### Schema 同步

- 如果 AxonHub 后端的 `agentapi.graphql` 发生了变化，你需要同步更新 `agent.graphql` 并重新生成代码。
- Schema 文件位置在 `genqlient.yaml` 中指定：`../../../internal/server/gql/agentapi/agentapi.graphql`
