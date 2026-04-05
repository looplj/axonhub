# MessageGateway 架构

## 概述

MessageGateway 是 AxonHub 的消息网关服务，负责在即时通讯平台（如飞书、Slack、钉钉等）和 Agent 实例之间进行双向消息路由。采用 `ChannelHandler` 接口抽象，支持多渠道类型扩展。

## 架构图

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              MessageGateway                                      │
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                        Channel Watcher (30s interval)                    │    │
│  │                                                                          │    │
│  │   ┌──────────────┐     ┌──────────────┐     ┌──────────────┐           │    │
│  │   │   Query      │     │    Sync      │     │   Manage     │           │    │
│  │   │  Channels    │────▶│  Channels    │────▶│  Runners     │           │    │
│  │   └──────────────┘     └──────────────┘     └──────────────┘           │    │
│  │                                                                        │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                       │                                          │
│                         ┌─────────────┼─────────────┐                            │
│                         ▼             ▼             ▼                            │
│  ┌──────────────────────────────────────────────────────────────────────────┐   │
│  │                    channelRunner (per channel)                            │   │
│  │                                                                          │   │
│  │   ┌──────────────────────────────────────────────────────────────────┐   │   │
│  │   │                 ChannelHandler (interface)                        │   │   │
│  │   │                                                                  │   │   │
│  │   │   Implementations:                                               │   │   │
│  │   │   ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │   │   │
│  │   │   │  feishuHandler│  │ (slack)      │  │ (dingtalk)   │  ...     │   │   │
│  │   │   │  Start()     │  │ Start()      │  │ Start()      │          │   │   │
│  │   │   │  Stop()      │  │ Stop()       │  │ Stop()       │          │   │   │
│  │   │   │  SendMessage()│ │ SendMessage()│  │ SendMessage()│          │   │   │
│  │   │   └──────────────┘  └──────────────┘  └──────────────┘          │   │   │
│  │   └──────────────────────────────────────────────────────────────────┘   │   │
│  │                                                                          │   │
│  │   ┌──────────────────────────────────────────────────────────────────┐   │   │
│  │   │              AgentMessageClient (shared logic)                   │   │   │
│  │   │                                                                  │   │   │
│  │   │   ┌──────────────┐     ┌──────────────┐     ┌──────────────┐   │   │   │
│  │   │   │   Route      │     │    Create     │     │   Pair Code  │   │   │   │
│  │   │   │   to Agent   │     │    Message    │     │   Matching   │   │   │   │
│  │   │   └──────────────┘     └──────────────┘     └──────────────┘   │   │   │
│  │   └──────────────────────────────────────────────────────────────────┘   │   │
│  │                                                                          │   │
│  │   ┌──────────────────────────────────────────────────────────────────┐   │   │
│  │   │              Agent Message Watcher (1s poll)                     │   │   │
│  │   │                                                                  │   │   │
│  │   │   ┌──────────────┐     ┌──────────────┐     ┌──────────────┐   │   │   │
│  │   │   │   Query      │     │ SendMessage  │     │    Update    │   │   │   │
│  │   │   │   Pending    │────▶│ via Handler  │────▶│   Status     │   │   │   │
│  │   │   └──────────────┘     └──────────────┘     └──────────────┘   │   │   │
│  │   └──────────────────────────────────────────────────────────────────┘   │   │
│  │                                                                          │   │
│  └──────────────────────────────────────────────────────────────────────────┘   │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
                    │                                           ▲
                    │                                           │
                    ▼                                           │
┌───────────────────────────────┐       ┌───────────────────────────────────────┐
│   IM Platforms (via Handler)  │       │            Database                    │
│                               │       │                                       │
│  ┌─────────────────────────┐  │       │  ┌─────────────────────────────────┐  │
│  │  Feishu / Slack / ...   │  │       │  │       MessageChannel            │  │
│  │                         │  │       │  │  - id, name, type, status       │  │
│  │  - Push events          │◀─┼───────┼──│  - settings (type-specific)     │  │
│  │  - Receive messages     │  │       │  └─────────────────────────────────┘  │
│  │  - Send messages        │  │       │                                       │
│  └─────────────────────────┘  │       │  ┌─────────────────────────────────┐  │
│                               │       │  │  MessageChannelAgentInstance    │  │
│                               │       │  │  - message_channel_id           │  │
│                               │       │  │  - agent_instance_id            │  │
│                               │       │  │  - enabled, order               │  │
│                               │       │  │  - config (allow_from, etc.)    │  │
│                               │       │  └─────────────────────────────────┘  │
│                               │       │                                       │
│                               │       │  ┌─────────────────────────────────┐  │
│                               │       │  │        AgentMessage             │  │
│                               │       │  │  - direction (to_agent/to_user) │  │
│                               │       │  │  - sender_type, sender_id       │  │
│                               │       │  │  - content, status              │  │
│                               │       │  └─────────────────────────────────┘  │
│                               │       │                                       │
└───────────────────────────────┘       └───────────────────────────────────────┘
```

## 消息流程

### 1. IM 平台消息 → Agent (Inbound)

```
┌─────────┐     ┌─────────────┐     ┌──────────────┐     ┌─────────────┐     ┌──────────┐
│ IM User │     │  Channel    │     │   Agent      │     │   Message   │     │  Agent   │
│         │────▶│  Handler    │────▶│ MessageClient│────▶│   Router    │────▶│ Instance │
└─────────┘     └─────────────┘     └──────────────┘     └─────────────┘     └──────────┘
                      │                    │                    │
                      │                    │                    │
                      ▼                    ▼                    ▼
              Platform-specific     Validate Sender      Create AgentMessage
              event handling        Filter Keywords      direction=to_agent
                                    Route by Binding     status=pending
```

**处理步骤：**

1. **平台事件接收** - ChannelHandler 接收平台推送的消息事件
2. **发送者验证** - 检查 `AllowFrom` 白名单（通道级 + 绑定级）
3. **关键词过滤** - 排除包含 `ExcludeKeywords` 的消息
4. **群聊检测** - 非私聊消息需要 @机器人 触发（平台相关）
5. **路由分发** - AgentMessageClient 按绑定顺序路由到多个 AgentInstance
6. **创建消息** - 写入 `AgentMessage` 表

### 2. Agent → IM 平台消息 (Outbound)

```
┌──────────┐     ┌─────────────┐     ┌──────────────┐     ┌─────────────┐     ┌─────────┐
│  Agent   │     │   Agent     │     │   Message    │     │  Channel    │     │ IM User │
│ Instance │────▶│   Message   │────▶│   Watcher    │────▶│  Handler    │────▶│         │
└──────────┘     └─────────────┘     └──────────────┘     └─────────────┘     └─────────┘
                      │                    │                    │
                      │                    │                    │
                      ▼                    ▼                    ▼
               Create Message       Query Pending       handler.SendMessage()
               direction=to_user    status=pending      Platform-specific send
               status=pending       Extract chat_id     Update status=acked
```

**处理步骤：**

1. **Agent 创建消息** - Agent 处理完成后创建 `AgentMessage`
2. **轮询检测** - 每秒查询 `pending` 状态的 `to_user` 消息
3. **提取路由信息** - 从 `content` 中提取 `chat_id`
4. **发送到平台** - 调用 `handler.SendMessage()` 发送消息
5. **更新状态** - 将消息状态更新为 `acked`

## 数据模型关系

```
┌─────────────────┐       ┌─────────────────────────────┐       ┌─────────────────┐
│  MessageChannel │       │ MessageChannelAgentInstance │       │  AgentInstance  │
│─────────────────│       │─────────────────────────────│       │─────────────────│
│ id              │◀──────│ message_channel_id          │──────▶│ id              │
│ project_id      │       │ agent_instance_id           │       │ agent_id        │
│ name            │       │ enabled                     │       │ status          │
│ type (feishu)   │       │ order                       │       │                 │
│ status          │       │ config (JSON)               │       │                 │
│ settings (JSON) │       │   - allow_from []           │       │                 │
│   - feishu      │       │   - exclude_keywords []     │       │                 │
│     - app_id    │       │                             │       │                 │
│     - app_secret│       └─────────────────────────────┘       └─────────────────┘
│     - allow_from│                                                 │
│     - exclude_  │                                                 │
│       keywords  │                                                 │
└─────────────────┘                                                 │
        │                                                           │
        │                                                           │
        ▼                                                           ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                   AgentMessage                                   │
│─────────────────────────────────────────────────────────────────────────────────│
│ id                                                                              │
│ agent_id                                                                        │
│ agent_instance_id                                                               │
│ direction (to_agent / to_user)                                                  │
│ sender_type (user / agent / system / message_channel)                           │
│ sender_id (message_channel.id when sender_type=message_channel)                 │
│ type (chat / approval_request / approval_result / system_event)                 │
│ content (JSON)                                                                  │
│   - text                                                                        │
│   - chat_id                                                                     │
│   - chat_type                                                                   │
│   - feishu_message_id                                                           │
│ status (pending / acked / expired)                                              │
│ sequence                                                                        │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## 配置说明

### MessageChannel Settings (Feishu)

```json
{
  "feishu": {
    "appId": "cli_xxx",
    "appSecret": "xxx",
    "encryptKey": "xxx",
    "verificationToken": "xxx",
    "allowFrom": ["ou_xxx", "@username"],
    "excludeKeywords": ["忽略", "skip"]
  }
}
```

### MessageChannelAgentInstance Config

```json
{
  "allowFrom": ["ou_xxx"],
  "excludeKeywords": ["测试"]
}
```

## 技术栈

| 组件 | 技术 |
|------|------|
| 飞书 WebSocket 客户端 | `github.com/larksuite/oapi-sdk-go/v3/ws` |
| 飞书 API | `github.com/larksuite/oapi-sdk-go/v3` |
| 依赖注入 | `go.uber.org/fx` |
| 日志 | `go.uber.org/zap` |
| 数据库 | Ent ORM |

## 关键特性

1. **多渠道类型支持** - 通过 `ChannelHandler` 接口抽象，支持飞书、Slack、钉钉等多种 IM 平台
2. **多 Agent 路由** - 一个消息通道可绑定多个 Agent 实例
3. **分层过滤** - 支持通道级和绑定级的发送者白名单和关键词过滤
4. **平台无关的消息处理** - `AgentMessageClient` 提供统一的消息路由和配对码匹配逻辑
5. **优雅启停** - 通过 fx 生命周期管理，支持平滑关闭

## 扩展新渠道类型

要添加新的渠道类型（如 Slack），需要：

1. 在 `internal/ent/schema/message_channel.go` 的 `type` 枚举中添加新值
2. 在 `internal/objects/message_channel.go` 中添加对应的 Settings 结构
3. 创建 `message_gateway_<type>.go` 文件，实现 `ChannelHandler` 接口
4. 在 `init()` 中调用 `RegisterChannelHandler()` 注册工厂函数
