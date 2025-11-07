# Unified API

## Overview
AxonHub exposes a unified API surface that mirrors the OpenAI and Anthropic HTTP contracts, allowing existing client SDKs to communicate with multiple providers through a single endpoint. The platform translates requests on the fly using a transformer pipeline so you can swap providers, remap models, or introduce custom policies without touching application code.

## Capabilities
- **Provider abstraction** – route traffic to OpenAI, Anthropic, DeepSeek, Zhipu, Moonshot, and more while retaining a consistent request schema.
- **Model remapping** – configure profile-based mappings to redirect models per environment or cost target.
- **Transport resilience** – automatic retries, timeouts, and logging are handled by the gateway so client logic stays simple.

## Integration Steps
1. Configure a channel in the AxonHub admin console with credentials for each upstream provider.
2. Point your application or SDK to the AxonHub base URL (default `http://localhost:8090`) and reuse OpenAI-compatible endpoints.
3. Use model profiles to define fallback rules or organization-specific defaults without modifying client code.

## API Surface

### Text Generation (Chat Completions)
AxonHub fronts the OpenAI-compatible chat completions contract. You can target the root (`/chat/completions`) or namespaced (`/v1/chat/completions`) endpoints and AxonHub will translate the request to the active provider defined by your channel profile.

| Provider               | Status     | Supported Models             |
| ---------------------- | ---------- | ---------------------------- |
| **OpenAI**             | ✅ Done    | GPT-4, GPT-4o, GPT-5, etc.   |
| **Anthropic**          | ✅ Done    | Claude 4.0, Claude 4.1, etc. |
| **Zhipu AI**           | ✅ Done    | GLM-4.5, GLM-4.5-air, etc.   |
| **Moonshot AI (Kimi)** | ✅ Done    | kimi-k2, etc.                |
| **DeepSeek**           | ✅ Done    | DeepSeek-V3.1, etc.          |
| **ByteDance Doubao**   | ✅ Done    | doubao-1.6, etc.             |
| **Gemini (OpenAI Compatible)** | ✅ Done | Gemini 2.5, etc.             |
| **AWS Bedrock**        | 🔄 Testing | Claude on AWS                |
| **Google Cloud**       | 🔄 Testing | Claude on GCP                |

**Endpoints**

- `POST /v1/chat/completions`
- `GET /models`
- `GET /v1/models`

Both request and streaming responses mirror OpenAI's schema. Set `stream: true` in the payload to receive server-sent events (SSE) from AxonHub; the gateway manages provider-specific streaming semantics.

### Anthropic Message API
When you need Anthropic-native semantics—such as tool use payloads or beta response fields—call the Messages API surface that AxonHub exposes under the Anthropic namespace.

- `POST /anthropic/v1/messages`
- `GET /anthropic/v1/models`

Requests follow Anthropic's official schema. AxonHub performs channel selection and policy enforcement, then relays the transformed request upstream.

```http
POST /anthropic/v1/messages HTTP/1.1
Host: axonhub.example.com
Content-Type: application/json
X-API-Key: <your api key>

{
  "model": "claude-3-5-sonnet",
  "max_tokens": 512,
  "messages": [
    {
      "role": "user",
      "content": [
        {"type": "text", "text": "Summarize the latest release notes."}
      ]
    }
  ],
  "stream": false
}
```

- Set `stream: true` and include `Accept: text/event-stream` to receive Anthropic-style SSE events (e.g., `event: message_start`, `event: content_block_delta`).
- AxonHub reuses the same credential (`X-API-Key`) and channel policies defined for chat completions, so you can swap between OpenAI- and Anthropic-style contracts without code changes.

## Related Resources
- [Claude Code & Codex Integration](claude-code-integration.md)
- [Image Generation Guide](image-generations.md)

---

# 统一 API

## 概述
AxonHub 提供统一的 API 接口，完全兼容 OpenAI 与 Anthropic 的调用协议，使现有客户端 SDK 可以通过单一入口访问多个模型服务。平台使用转换管线在请求进入时完成模型映射与策略控制，因此无需改动业务代码即可切换供应商或执行定制策略。

## 核心能力
- **供应商抽象**：通过统一的请求格式对接 OpenAI、Anthropic、DeepSeek、智谱、Moonshot 等服务。
- **模型映射**：结合模型配置文件（Profile）在不同环境或成本目标之间灵活切换模型。
- **传输弹性**：自动处理重试、超时与日志记录，让客户端保持精简。

## 接入步骤
1. 在 AxonHub 管理控制台中为每个上游服务配置渠道与凭证。
2. 将应用或 SDK 指向 AxonHub 的基础地址（默认 `http://localhost:8090`），沿用 OpenAI 兼容的接口。
3. 使用模型配置文件设定回退规则或组织默认策略，无需修改客户端代码。

## API 接口概览

### 文本生成（Chat Completions）
AxonHub 同时兼容根路径（`/chat/completions`）与 OpenAI 命名空间路径（`/v1/chat/completions`），并自动根据渠道配置选择上游模型提供商。

| 供应商                   | 状态       | 支持模型示例                 |
| ------------------------ | ---------- | ---------------------------- |
| **OpenAI**               | ✅ 已完成  | GPT-4、GPT-4o、GPT-5 等      |
| **Anthropic**            | ✅ 已完成  | Claude 4.0、Claude 4.1 等    |
| **智谱 AI（Zhipu）**     | ✅ 已完成  | GLM-4.5、GLM-4.5-air 等      |
| **月之暗面（Moonshot）** | ✅ 已完成  | kimi-k2 等                   |
| **DeepSeek**             | ✅ 已完成  | DeepSeek-V3.1 等             |
| **字节跳动豆包**         | ✅ 已完成  | doubao-1.6 等                |
| **Gemini（OpenAI 兼容）** | ✅ 已完成 | Gemini 2.5 等                |
| **AWS Bedrock**          | 🔄 测试中  | Claude on AWS                |
| **Google Cloud**         | 🔄 测试中  | Claude on GCP                |

**可用接口**

- `POST /chat/completions`
- `POST /v1/chat/completions`
- `GET /models`
- `GET /v1/models`

请求体与响应体保持 OpenAI 原生格式。若需要流式输出，可在请求中设置 `stream: true`，AxonHub 会自动处理 SSE 事件与各供应商的差异。

### Anthropic Message API
当业务需要 Anthropic 原生的 Messages 协议（例如工具调用、增量内容块等字段）时，可使用 AxonHub 暴露在 `/anthropic/v1` 命名空间下的接口。

- `POST /anthropic/v1/messages`
- `GET /anthropic/v1/models`

请求完全遵循官方 Anthropic Schema，AxonHub 会负责渠道选择与策略控制。

```http
POST /anthropic/v1/messages HTTP/1.1
Host: axonhub.example.com
Content-Type: application/json
X-API-Key: <你的 API Key>

{
  "model": "claude-3-5-sonnet",
  "max_tokens": 512,
  "messages": [
    {
      "role": "user",
      "content": [
        {"type": "text", "text": "请总结最新的发布说明。"}
      ]
    }
  ],
  "stream": false
}
```

- 若需流式返回，将 `stream` 设为 `true` 并添加 `Accept: text/event-stream` 头，AxonHub 将输出 Anthropic 样式的 SSE 事件（如 `event: message_start`、`event: content_block_delta`）。
- Messages 接口与 Chat Completions 共用 `X-API-Key` 认证与渠道策略，方便在两种调用协议之间切换而无需修改客户端代码。

## 相关资源
- [Claude Code / Codex 集成指南](claude-code-integration.md)
- [图像生成功能指南](image-generations.md)
