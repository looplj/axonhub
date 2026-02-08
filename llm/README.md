# LLM Pipeline 架构概览

## 核心设计理念

AxonHub 的 LLM Pipeline 采用**转换器链（Transformer Chain）**模式，将不同 AI 提供商的 API 统一为标准的 OpenAI 兼容接口。

**核心思想**：所有请求经过"标准化 → 处理 → 反标准化"的流程，实现多提供商的无缝切换和智能路由。

---

## 数据模型

### 统一请求（LLMRequest）
兼容 OpenAI 格式，扩展支持：
- 多模态输入（文本、图像、文档、音频）
- 工具调用（函数调用）
- 推理努力控制（如 o1 系列）
- 缓存控制
- 图像生成
- 嵌入和重排序

### 统一响应（LLMResponse）
标准化输出，包含：
- 流式/非流式内容
- 使用统计（Token 消耗）
- 错误信息
- 完成原因
- 嵌入向量
- 重排序结果

---

## 核心流程

### 阶段一：入站处理（标准化）

```
┌──────────────┐     ┌──────────────────┐     ┌─────────────────┐
│              │     │                  │     │                 │
│ 客户端请求    │────▶│ InboundTransformer│────▶│ 统一 LLM 请求    │
│ (多种格式)    │     │  提取&标准化     │     │  (内部格式)     │
│              │     │                  │     │                 │
└──────────────┘     └──────────────────┘     └─────────────────┘
```

**关键步骤**：
1. **请求转换** - 解析 HTTP 请求并转换为内部 LLM 格式
2. **模型映射** - 根据渠道配置映射模型名称（如 `gpt-4o` → `custom-model`）
3. **渠道选择** - 基于健康状态和可用性选择合适的 AI 提供商渠道
4. **持久化** - 创建请求记录，生成唯一 Request ID

---

### 阶段二：出站处理（反标准化）

```
┌─────────────────┐     ┌────────────────────┐     ┌──────────────────┐
│                 │     │                    │     │                  │
│ 统一 LLM 请求    │────▶│ OutboundTransformer│────▶│ 提供商特定请求    │
│  (内部格式)     │     │  转换&适配         │     │  (OpenAI/Claude等)│
│                 │     │                    │     │                  │
└─────────────────┘     └────────────────────┘     └──────────────────┘
```

**关键步骤**：
1. **格式转换** - 将统一格式转换为提供商特定格式（OpenAI、Anthropic、Gemini、AI SDK 等）
2. **参数覆盖** - 应用渠道配置中的覆盖参数（MaxTokens、温度等）
3. **HTTP 执行** - 发送请求到 AI 提供商
4. **响应转换** - 将提供商响应转换回统一格式

---

### 完整请求生命周期

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                             请求生命周期                                      │
│                                                                             │
│  1. 入站转换 (标准化)                                                        │
│      └─▶ 请求验证 → 模型映射 → 渠道选择 → 创建记录                            │
│                                                                             │
│  2. 中间件处理 (可选)                                                        │
│      └─▶ MaxToken 限制 → 使用统计 → 请求日志                                │
│                                                                             │
│  3. 出站转换 (反标准化)                                                      │
│      └─▶ 格式转换 → 参数覆盖 → 准备 HTTP 请求                               │
│                                                                             │
│  4. Pipeline 执行（核心）                                                    │
│      ┌─────────────────────────────────────────────────────────────┐         │
│      │ 渠道循环 (Failover)                                         │         │
│      │   ├─▶ 执行请求 (processRequest)                             │         │
│      │   │     └─▶ 成功？ → 直接返回结果                           │         │
│      │   │                                                         │         │
│      │   ├─▶ 失败？ → 调用错误中间件 → 检查重试策略                │         │
│      │   │                                                         │         │
│      │   ├─▶ 策略 A: 同渠道重试 (ChannelRetryable)                 │         │
│      │   │     └─▶ 额度未满 & 可重试？ → Prepare & 继续本渠道循环  │         │
│      │   │                                                         │         │
│      │   └─▶ 策略 B: 跨渠道重试 (Retryable)                        │         │
│      │         └─▶ 策略 A 不满足 & 切换额度未满？ → NextChannel    │         │
│      │                                                             │         │
│      └─────────────────────────────────────────────────────────────┘         │
│                                                                             │
│  5. 响应处理                                                                 │
│      └─▶ 流式聚合 → 统计提取 → 记录完成状态 → 返回客户端                     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 重试机制

### 智能错误处理

**两大重试策略（计数器独立）**：

| 类型 | 触发条件 | 行为 | 优先级 |
|------|---------|------|-------|
| **同渠道重试** | 限流错误（429）、临时网络错误 | 调用 `PrepareForRetry` 后在同一渠道重试 | **高** (优先尝试) |
| **跨渠道重试** | 同渠道重试耗尽、渠道不可用、认证失败 | 调用 `NextChannel` 切换到下一个备选渠道 | **低** (故障转移) |

**设计细节**：
- **计数器解耦**：同渠道重试不消耗跨渠道切换的额度。
- **状态重置**：当发生跨渠道切换时，同渠道重试计数器会重置，确保新渠道拥有完整的重试机会。
- **中间件联动**：每次请求失败都会触发 `OnOutboundRawError` 中间件，无论是否重试。
- **退避策略**：如果配置了 `retryDelay`，每次重试之间会进行等待。

---

## 流式响应

### 实时数据传输

```
提供商 SSE 流 → 转换器处理 → 统一格式块 → 转发到客户端
     ↓                                              ↑
     └──────→ 持久化存储 ←────── 块聚合 ←───────┘
```

**工作流程**：
1. **创建流** - 初始化流式响应记录
2. **逐块处理** - 每收到一块立即转换并转发
3. **实时聚合** - 在内存中累积内容，提取统计信息
4. **完成保存** - 流结束后保存聚合结果

**支持的流式协议**：
- OpenAI Server-Sent Events (SSE)
- Anthropic Event Stream
- AI SDK TextStream/DataStream
- Gemini Event Stream

---

## Transformer 实现

### 支持的 AI 提供商

| 提供商 | InboundTransformer | OutboundTransformer | 特色功能 |
|--------|-------------------|--------------------|---------|
| **OpenAI** | OpenAI → 统一格式 | 统一格式 → OpenAI | 工具调用聚合、推理内容支持、图像生成、嵌入 |
| **Anthropic** | Claude → 统一格式 | 统一格式 → Claude | 系统消息合并、思考内容支持、缓存控制 |
| **Gemini** | Gemini → 统一格式 | 统一格式 → Gemini | Google 原生工具、思考内容支持 |
| **AI SDK** | AI SDK → 统一格式 | 统一格式 → AI SDK | 兼容 Vercel AI SDK（TextStream/DataStream） |
| **Jina** | Jina → 统一格式 | 统一格式 → Jina | 嵌入和重排序支持 |

### 渠道类型

系统支持 30+ 种渠道类型，包括：

**OpenAI 兼容**：
- `openai` - OpenAI 官方 API
- `openai_responses` - OpenAI Responses API
- `openai_fake` - 测试用模拟渠道
- `vercel` - Vercel AI SDK
- `deepseek` - DeepSeek
- `deepinfra` - DeepInfra
- `moonshot` - Moonshot AI
- `zhipu` - 智谱 AI
- `ppio` - PPIO
- `siliconflow` - SiliconFlow
- `volcengine` - 火山引擎
- `minimax` - MiniMax
- `aihubmix` - AIHubMix
- `burncloud` - BurnCloud
- `github` - GitHub Models
- `claudecode` - Claude Code

**Anthropic 兼容**：
- `anthropic` - Anthropic 官方 API
- `anthropic_aws` - AWS Bedrock (Claude)
- `anthropic_gcp` - Google Vertex AI (Claude)
- `anthropic_fake` - 测试用模拟渠道
- `deepseek_anthropic` - DeepSeek (Anthropic 格式)
- `doubao_anthropic` - 豆包 (Anthropic 格式)
- `moonshot_anthropic` - Moonshot (Anthropic 格式)
- `zhipu_anthropic` - 智谱 (Anthropic 格式)
- `zai_anthropic` - Zai (Anthropic 格式)
- `longcat_anthropic` - Longcat (Anthropic 格式)
- `minimax_anthropic` - MiniMax (Anthropic 格式)

**Gemini 兼容**：
- `gemini` - Gemini 官方 API
- `gemini_openai` - Gemini (OpenAI 兼容格式)
- `gemini_vertex` - Google Vertex AI (Gemini)

**其他**：
- `doubao` - 豆包
- `zai` - Zai
- `xai` - xAI (Grok)
- `openrouter` - OpenRouter
- `xiaomi` - Xiaomi MIMO (OpenAI 兼容格式)
- `longcat` - Longcat
- `modelscope` - ModelScope
- `bailian` - 阿里百炼
- `jina` - Jina AI

### 多平台支持

- **Azure OpenAI**：资源名称 + 部署 ID 特殊端点
- **AWS Bedrock**：AWS SigV4 认证，特殊事件格式
- **Google Vertex AI**：区域端点 + GCP 认证

---

## 中间件系统

### 可插拔的功能扩展

中间件在 Pipeline 执行前后介入，提供额外功能：

**中间件接口**：
```go
type Middleware interface {
    Name() string
    OnInboundLlmRequest(ctx context.Context, request *llm.Request) (*llm.Request, error)
    OnInboundRawResponse(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error)
    OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error)
    OnOutboundRawError(ctx context.Context, err error)
    OnOutboundRawResponse(ctx context.Context, response *httpclient.Response) (*httpclient.Response, error)
    OnOutboundLlmResponse(ctx context.Context, response *llm.Response) (*llm.Response, error)
    OnOutboundRawStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error)
    OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error)
}
```

**内置中间件**：
1. **MaxToken 限制** - 限制单次请求的最大 Token 数
2. **使用统计** - 实时跟踪 Token 消耗
3. **请求日志** - 记录详细的请求响应信息
4. **渠道切换** - 重试时自动切换渠道

**中间件执行点**：
- 入站请求转换后（`OnInboundLlmRequest`）
- 入站响应转换后（`OnInboundRawResponse`）
- 出站请求发送前（`OnOutboundRawRequest`）
- 出站请求发送失败（`OnOutboundRawError`）
- 出站响应接收后（`OnOutboundRawResponse`）
- 出站 LLM 响应转换后（`OnOutboundLlmResponse`）
- 出站流式响应处理中（`OnOutboundRawStream`）
- 出站 LLM 流式响应处理中（`OnOutboundLlmStream`）

---

## 持久化机制

### 数据记录

**Request 记录**：
- 请求元数据（ID、时间、用户）
- 原始请求体（JSON）
- 使用的模型和渠道

**执行记录（RequestExecution）**：
- 每次尝试的详细信息
- 渠道配置快照
- 是否成功、错误信息

**流式块（StreamChunk）**：
- 每个 SSE 块实时保存
- 用于审计和调试

---

## 架构优势

### 1. 提供商无关性
- 统一接口屏蔽提供商差异
- 新增提供商只需实现 Transformer
- 客户端代码无需修改

### 2. 高可用性
- 自动故障转移（渠道切换）
- 智能重试策略
- 健康检查和负载均衡

### 3. 可观测性
- 完整的请求链路追踪
- 详细的执行日志
- 实时使用统计

### 4. 可扩展性
- 中间件机制灵活扩展
- 配置驱动无需代码修改
- 支持自定义渠道策略

---

## 快速入门

### 新手指南：Transformer 实现

1. **实现接口**（2 个文件）
   - `inbound.go` - 提供商格式 → 统一格式
   - `outbound.go` - 统一格式 → 提供商格式

2. **在业务逻辑中注册**
   - 在 `internal/server/biz/channel_llm.go` 中添加新渠道类型的处理逻辑
   - 根据渠道类型创建对应的 Transformer

3. **配置渠道**
   - 在管理界面添加新渠道
   - 选择提供商类型
   - 填写认证信息

### 示例：创建新的 Outbound Transformer

```go
package myprovider

import (
    "github.com/looplj/axonhub/llm"
    "github.com/looplj/axonhub/llm/transformer"
)

type OutboundTransformer struct {
    baseURL string
    apiKey  string
}

func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
    return &OutboundTransformer{
        baseURL: baseURL,
        apiKey:  apiKey,
    }, nil
}

func (t *OutboundTransformer) APIFormat() llm.APIFormat {
    return "myprovider/api"
}

func (t *OutboundTransformer) TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error) {
    // 将统一请求转换为提供商特定格式
    // ...
}

func (t *OutboundTransformer) TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
    // 将提供商响应转换为统一格式
    // ...
}

// 实现其他必要方法...
```

---

## 架构图

```
┌─────────────────────────────────────────────────────────────────────┐
│                           客户端请求                                  │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      HTTP Handler (/chat/completion)                  │
│                      文件: internal/server/api/chat.go               │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                       ChatCompletionProcessor                         │
│                      文件: internal/server/chat/completion.go        │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
│  InboundTransformer                                                    │
│  文件: llm/transformer/*/inbound.go                                  │
│  功能: HTTP Request → LLM Request                                    │
└ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┬────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         中间件处理                                     │
│  - Model Mapping (模型名称映射)                                       │
│  - Channel Selection (渠道选择)                                       │
│  - MaxToken Enforcement (Token 限制)                                 │
│  - Request Persistence (请求持久化)                                   │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
│  OutboundTransformer                                                   │
│  文件: llm/transformer/*/outbound.go                                 │
│  功能: LLM Request → HTTP Request                                    │
└ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┬────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        LLM Pipeline                                   │
│  文件: llm/pipeline/pipeline.go                                      │
│  功能: 执行请求 + 重试逻辑                                              │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
│                Transformer (根据渠道类型选择)                           │
│                                                                        │
│   ┌─────────────────┐        ┌─────────────────┐        ┌──────────┐│
│   │ OpenAI          │        │ Anthropic       │        │ Gemini   ││
│   │ Transformers    │        │ Transformers    │        │Transform ││
│   └─────────────────┘        └─────────────────┘        └──────────┘│
│                                                                        │
│   ┌─────────────────┐        ┌─────────────────┐        ┌──────────┐│
│   │ AI SDK          │        │ Jina            │        │ Custom   ││
│   │ Transformers    │        │ Transformers    │        │Transform ││
│   └─────────────────┘        └─────────────────┘        └──────────┘│
└ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┬────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      AI 提供商 (OpenAI/Anthropic/Gemini/...)            │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐
│                Transformer (响应转换)                                  │
│                                                                        │
│  LLM Response ←─────────────────────┐                                 │
│                                      │                                │
│   ┌─────────────────┐        ┌──────▼──────┐        ┌──────────┐    │
│   │ OpenAI          │        │ Anthropic   │        │ Gemini   │    │
│   │ Inbound         │        │ Inbound     │        │ Inbound  │    │
│   └─────────────────┘        └─────────────┘        └──────────┘    │
│                                                                        │
│   ┌─────────────────┐        ┌─────────────────┐        ┌──────────┐    │
│   │ AI SDK          │        │ Jina            │        │ Custom   │    │
│   │ Inbound         │        │ Inbound         │        │ Inbound  │    │
│   └─────────────────┘        └─────────────────┘        └──────────┘    │
└ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┬ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         中间件处理                                     │
│  - Response Persistence (响应持久化)                                  │
│  - Usage Tracking (使用统计)                                          │
│  - Channel Switching (渠道切换，重试时)                               │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      客户端响应 (SSE/JSON)                            │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 相关文件

### 核心接口
- `llm/model.go` - 统一数据模型
- `llm/constants.go` - 常量定义（RequestType、APIFormat、ToolType）
- `llm/transformer/interfaces.go` - Transformer 接口定义
- `llm/pipeline/pipeline.go` - Pipeline 实现
- `llm/pipeline/executor.go` - Executor 接口
- `llm/pipeline/middleware.go` - 中间件接口和实现
- `llm/pipeline/maxtoken/max_token.go` - MaxToken 中间件

### Transformer 实现
- `llm/transformer/openai/` - OpenAI Transformer（包括 Responses API）
- `llm/transformer/anthropic/` - Anthropic Transformer
- `llm/transformer/gemini/` - Gemini Transformer
- `llm/transformer/aisdk/` - AI SDK Transformer（TextStream/DataStream）
- `llm/transformer/jina/` - Jina Transformer（嵌入和重排序）
- `llm/transformer/openrouter/` - OpenRouter Transformer
- `llm/transformer/doubao/` - 豆包 Transformer
- `llm/transformer/zai/` - Zai Transformer
- `llm/transformer/xai/` - xAI Transformer
- `llm/transformer/longcat/` - Longcat Transformer
- `llm/transformer/modelscope/` - ModelScope Transformer
- `llm/transformer/bailian/` - 阿里百炼 Transformer

### Pipeline
- `llm/pipeline/pipeline.go` - 主 Pipeline 实现
- `llm/pipeline/stream.go` - 流式处理
- `llm/pipeline/non_streaming.go` - 非流式处理
- `llm/pipeline/middleware.go` - 中间件系统

### HTTP 客户端
- `llm/httpclient/client.go` - HTTP 客户端
- `llm/httpclient/decoder.go` - 流式解码器
- `llm/httpclient/builder.go` - 请求构建器
- `llm/httpclient/proxy.go` - 代理支持

### 流式处理
- `llm/streams/stream.go` - 流式接口定义
- `llm/streams/slice.go` - 切片流实现
- `llm/streams/map.go` - 映射流
- `llm/streams/filter.go` - 过滤流
- `llm/streams/append.go` - 追加流

### 业务逻辑
- `internal/server/biz/channel_llm.go` - 渠道管理和 Transformer 创建
- `internal/server/biz/trace.go` - 追踪和 Transformer 管理
- `internal/ent/channel/` - 渠道数据模型和类型定义

### 工具
- `llm/transformer/url.go` - URL 处理工具
- `llm/transformer/errors.go` - 错误处理
- `llm/tools.go` - 工具调用相关

---

## 高级特性

### 1. 自定义执行器

某些渠道（如 AWS Bedrock）需要自定义 HTTP 执行器来处理特殊的认证或请求格式：

```go
type ChannelCustomizedExecutor interface {
    CustomizeExecutor(Executor) Executor
}
```

### 2. 渠道重试接口

支持两种重试策略：

**同渠道重试**：
```go
type ChannelRetryable interface {
    CanRetry(err error) bool
    PrepareForRetry(ctx context.Context) error
}
```

**跨渠道重试**：
```go
type Retryable interface {
    HasMoreChannels() bool
    NextChannel(ctx context.Context) error
}
```

### 3. 请求类型支持

系统支持多种请求类型：
- `RequestTypeChat` - 聊天完成
- `RequestTypeEmbedding` - 嵌入
- `RequestTypeRerank` - 重排序

### 4. API 格式支持

支持多种 API 格式：
- `APIFormatOpenAIChatCompletion` - OpenAI 聊天完成
- `APIFormatOpenAIResponse` - OpenAI Responses API
- `APIFormatOpenAIImageGeneration` - OpenAI 图像生成
- `APIFormatOpenAIEmbedding` - OpenAI 嵌入
- `APIFormatGeminiContents` - Gemini 内容 API
- `APIFormatAnthropicMessage` - Anthropic 消息 API
- `APIFormatAiSDKText` - AI SDK 文本流
- `APIFormatAiSDKDataStream` - AI SDK 数据流
- `APIFormatJinaRerank` - Jina 重排序
- `APIFormatJinaEmbedding` - Jina 嵌入

### 5. 工具类型支持

支持多种工具类型：
- `ToolTypeFunction` - 函数调用（OpenAI）
- `ToolTypeImageGeneration` - 图像生成（OpenAI）
- `ToolTypeGoogleSearch` - Google 搜索（Gemini）
- `ToolTypeGoogleCodeExecution` - Google 代码执行（Gemini）
- `ToolTypeGoogleUrlContext` - Google URL 上下文（Gemini）
- `ToolTypeAnthropicWebSearch` - Anthropic 网络搜索（Beta）

---

## 测试

每个 Transformer 都有完整的测试套件：
- 单元测试（`*_test.go`）
- 集成测试（`*_integration_test.go`）
- 假数据测试（`fake.go`, `fake_test.go`）

运行测试：
```bash
# 运行所有测试
go test ./llm/...

# 运行特定 Transformer 的测试
go test ./llm/transformer/openai/...

# 运行集成测试
go test -tags=integration ./llm/transformer/anthropic/...
```

---

## 贡献指南

添加新的 Transformer：

1. 在 `llm/transformer/` 下创建新目录
2. 实现 `Inbound` 和 `Outbound` 接口
3. 添加完整的测试套件
4. 在 `internal/server/biz/channel_llm.go` 中注册新渠道类型
5. 在 `internal/ent/channel/channel.go` 中添加渠道类型常量
6. 更新本文档

---

## 许可证

本项目采用 MIT 许可证。详见 LICENSE 文件。
