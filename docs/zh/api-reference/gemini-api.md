# Gemini API 参考

## 概述

AxonHub 原生支持 Gemini API，可访问 Gemini 强大的多模态功能。您可以使用 Gemini SDK 访问 Gemini 模型，也可以访问 OpenAI、Anthropic 和其他支持的模型。

GenerateContent 请求也接受 `system_instruction`、`inline_data` 和 `mime_type`，分别作为 `systemInstruction`、`inlineData` 和 `mimeType` 的兼容别名。两种写法都有值时，优先使用 camelCase 字段的值。Gemini 输出仍使用标准的 camelCase 字段名。

## 核心优势

- **API 互操作性**：使用 Gemini API 调用 OpenAI、Anthropic 和其他支持的模型
- **零代码变更**：继续使用现有的 Gemini 客户端 SDK，无需修改
- **自动转换**：AxonHub 在需要时自动在 API 格式之间进行转换
- **多模态支持**：通过 Gemini API 格式访问文本和图像功能

## Vertex AI 渠道凭证

一个 **Gemini Vertex AI** 渠道可以同时保存多条以下两种凭证：

- **API key（Express mode）**：填写 API key、所属 Google Cloud 项目 ID 或项目编号，以及 `global`、`us-central1` 等 location。
- **服务账号 JSON**：粘贴为 Google Cloud service account 创建的 JSON 密钥，或选择本地 JSON 文件。浏览器读取文件后将内容作为字符串提交。项目从 `project_id` 自动预填，也可改为该服务账号有权访问的其他项目；location 单独设置。

请在目标项目启用 Vertex AI API，并授予服务账号调用 Vertex AI 的权限，例如 `roles/aiplatform.user`。AxonHub 使用 `cloud-platform` scope，向 Google OAuth 端点交换访问令牌，并通过渠道配置的代理自动刷新。仅接受 service account JSON，不接受 Application Default Credentials 或其他 JSON 凭证类型。

渠道凭证管理支持逐条添加、编辑、删除、测试、禁用和恢复。API key 与服务账号共同参与渠道的凭证选择策略，每次请求使用选中条目对应的认证、项目与 location。仅调整 JSON 格式、项目或 location 时，服务账号条目的身份保持不变；更换账号或 signing key 会产生新的凭证身份。

## 支持的端点

**端点：**
- `POST /gemini/v1beta/models/{model}:generateContent` - 文本和多模态内容生成
- `POST /v1beta/models/{model}:generateContent` - 文本和多模态内容生成 (可选)
- `GET /gemini/v1beta/models` - 列出可用模型
- `GET /v1beta/models` - 列出可用模型 (可选)

**示例请求：**
```go
import (
    "context"
    "google.golang.org/genai"
)

// 使用 AxonHub 配置创建 Gemini 客户端
ctx := context.Background()
client, err := genai.NewClient(ctx, &genai.ClientConfig{
    APIKey:  "your-axonhub-api-key",
    Backend: genai.Backend(genai.APIBackendUnspecified), // 使用默认后端
    HTTPOptions: genai.HTTPOptions{
			BaseURL: "http://localhost:8090/gemini",
	},
})
if err != nil {
    // 适当处理错误
    panic(err)
}

// 使用 Gemini API 格式调用 OpenAI 模型
modelName := "gpt-4o"  // 通过 Gemini API 格式访问 OpenAI 模型
content := &genai.Content{
    Parts: []*genai.Part{
        {Text: genai.Ptr("Hello, GPT!")},
    },
}

// 可选：配置生成参数
config := &genai.GenerateContentConfig{
    Temperature: genai.Ptr(float32(0.7)),
    MaxOutputTokens: genai.Ptr(int32(1024)),
}

response, err := client.Models.GenerateContent(ctx, modelName, []*genai.Content{content}, config)
if err != nil {
    // 适当处理错误
    panic(err)
}

// 从响应中提取文本
if len(response.Candidates) > 0 &&
   len(response.Candidates[0].Content.Parts) > 0 {
    responseText := response.Candidates[0].Content.Parts[0].Text
    fmt.Println(*responseText)
}
```

**示例：多轮对话**
```go
// 创建带有对话历史的聊天会话
modelName := "claude-3-5-sonnet"
config := &genai.GenerateContentConfig{
    Temperature: genai.Ptr(float32(0.5)),
}

chat, err := client.Chats.Create(ctx, modelName, config, nil)
if err != nil {
    panic(err)
}

// 第一条消息
response1, err := chat.SendMessage(ctx, genai.Part{Text: genai.Ptr("My name is Alice")})
if err != nil {
    panic(err)
}

// 后续消息（模型记住上下文）
response2, err := chat.SendMessage(ctx, genai.Part{Text: genai.Ptr("What is my name?")})
if err != nil {
    panic(err)
}

// 提取响应
if len(response2.Candidates) > 0 {
    text := response2.Candidates[0].Content.Parts[0].Text
    fmt.Println(*text)  // 应该包含 "Alice"
}
```

## API 转换能力

AxonHub 自动在 API 格式之间进行转换，实现以下强大场景：

### 使用 Gemini SDK 调用 OpenAI 模型
```go
// Gemini SDK 调用 OpenAI 模型
content := &genai.Content{
    Parts: []*genai.Part{
        {Text: genai.Ptr("什么是人工智能？")},
    },
}

response, err := client.Models.GenerateContent(
    ctx,
    "gpt-4o",  // OpenAI 模型
    []*genai.Content{content},
    nil,
)

// 访问响应
if len(response.Candidates) > 0 &&
   len(response.Candidates[0].Content.Parts) > 0 {
    text := response.Candidates[0].Content.Parts[0].Text
    fmt.Println(*text)
}
// AxonHub 自动转换 Gemini 格式 → OpenAI 格式
```

### 使用 Gemini SDK 调用 Anthropic 模型
```go
// Gemini SDK 调用 Anthropic 模型
content := &genai.Content{
    Parts: []*genai.Part{
        {Text: genai.Ptr("解释神经网络")},
    },
}

response, err := client.Models.GenerateContent(
    ctx,
    "claude-3-5-sonnet",  // Anthropic 模型
    []*genai.Content{content},
    nil,
)

// 访问响应
if len(response.Candidates) > 0 &&
   len(response.Candidates[0].Content.Parts) > 0 {
    text := response.Candidates[0].Content.Parts[0].Text
    fmt.Println(*text)
}
// AxonHub 自动转换 Gemini 格式 → Anthropic 格式
```

## 认证

Gemini API 格式使用以下认证方式：

- **头部**：`X-Goog-API-Key: <your-api-key>`

API 密钥通过 AxonHub 的 API 密钥管理系统进行管理，无论使用哪种 API 格式，都提供相同的权限。

## 流式支持

Gemini API 格式支持流式响应以进行实时内容生成。

## 错误处理

Gemini 格式错误响应遵循标准 Gemini API 错误格式。

## 工具支持

AxonHub 通过 Gemini API 格式支持**函数工具**（自定义函数调用）。但是，**不支持**各提供商特有的工具：

| 工具类型 | 支持状态 | 说明 |
| -------- | -------- | ---- |
| **函数工具（Function Tools）** | ✅ 支持 | 自定义函数定义可跨所有提供商使用 |
| **网页搜索（Web Search）** | ❌ 不支持 | 提供商特有功能 |
| **代码解释器（Code Interpreter）** | ❌ 不支持 | 提供商特有功能 |
| **文件搜索（File Search）** | ❌ 不支持 | 提供商特有功能 |
| **计算机使用（Computer Use）** | ❌ 不支持 | Anthropic 特有功能 |

> **注意**：仅支持可跨提供商转换的通用函数工具。提供商特有工具需要直接访问提供商的基础设施，无法通过 AxonHub 代理。

## 最佳实践

1. **使用追踪头部**：包含 `AH-Trace-Id` 和 `AH-Thread-Id` 头部以获得更好的可观测性
2. **模型选择**：在请求中明确指定目标模型
3. **错误处理**：为 API 响应实现适当的错误处理
4. **流式处理**：对于长响应使用流式处理以获得更好的用户体验
5. **多模态内容**：在处理图像时利用 Gemini API 的多模态功能

## 迁移指南

### 从 Gemini 迁移到 AxonHub
```go
// 之前：直接 Gemini
ctx := context.Background()
client, err := genai.NewClient(ctx, &genai.ClientConfig{
    APIKey: "gemini-api-key",
})

// 之后：使用 Gemini API 的 AxonHub
ctx := context.Background()
client, err := genai.NewClient(ctx, &genai.ClientConfig{
    APIKey: "your-axonhub-api-key",
    HTTPOptions: genai.HTTPOptions{
        BaseURL: "http://localhost:8090/gemini",
    },
})
// 您的现有代码继续工作！
```
