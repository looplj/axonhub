# AI Gateway System

这是一个基于 Golang 的 AI Gateway 系统，采用 Transformer Chain 设计模式，可以封装各个渠道的 AI 请求，对外提供兼容 OpenAI 的统一 API。

## 系统架构

### 核心组件

1. **Types** (`pkg/types/`)
   - `openai.go`: OpenAI Chat Completion API 的完整结构定义
   - `http_request.go`: 通用 HTTP 请求和响应结构定义

2. **Interfaces** (`pkg/interfaces/`)
   - 定义了所有核心接口：Decorator、Transformer、RequestProcessor 等

3. **Decorator** (`pkg/decorator/`)
   - 装饰器模式，用于在请求发送前修改或增强请求
   - 支持优先级排序和条件应用

4. **Transformer** (`pkg/transformer/`)
   - 转换器，将 OpenAI 格式转换为各个服务商的特定格式
   - 支持请求和响应的双向转换

5. **Processor** (`pkg/processor/`)
   - 请求处理器，协调整个处理流程
   - 管理 Decorator Chain 和 Transformer Registry

6. **Client** (`pkg/client/`)
   - HTTP 客户端实现，支持重试、流式响应等功能

### 处理流程

```
接收请求 → Decorator Chain → Transformer → HTTP Client → 服务商 API
                                                              ↓
返回响应 ← Response Transformer ← HTTP Response ← 服务商响应
```

## 主要特性

### 1. OpenAI API 兼容
- 完整支持 OpenAI Chat Completion API 规范
- 支持流式和非流式响应
- 支持工具调用、函数调用等高级功能
- 允许额外参数扩展

### 2. 多服务商支持
- 统一的 Transformer 接口
- 可插拔的服务商适配器
- 支持服务商特定配置

### 3. 请求装饰
- 支持多个装饰器链式处理
- 优先级控制
- 条件应用
- 可扩展的装饰逻辑

### 4. 高级功能
- 请求重试机制
- 流式响应支持
- 请求追踪和元数据
- 错误处理和恢复

## 使用示例

### 基本使用

```go
package main

import (
    "context"
    "time"
    
    "github.com/looplj/axonhub/pkg/client"
    "github.com/looplj/axonhub/pkg/decorator"
    "github.com/looplj/axonhub/pkg/processor"
    "github.com/looplj/axonhub/pkg/transformer"
    "github.com/looplj/axonhub/pkg/types"
)

func main() {
    // 1. 创建 HTTP 客户端
    httpClient := client.NewHttpClient(30 * time.Second)
    
    // 2. 创建装饰器链
    decoratorChain := processor.NewDecoratorChain()
    defaultDecorator := decorator.NewChatCompletionDecorator("default", 100)
    decoratorChain.AddDecorator(defaultDecorator)
    
    // 3. 创建转换器注册表
    transformerRegistry := processor.NewTransformerRegistry()
    openaiTransformer := transformer.NewChatCompletionTransformer("openai")
    transformerRegistry.RegisterTransformer(openaiTransformer, "openai")
    
    // 4. 创建请求处理器
    requestProcessor := processor.NewRequestProcessor()
    requestProcessor.SetDecoratorChain(decoratorChain)
    requestProcessor.SetTransformerRegistry(transformerRegistry)
    requestProcessor.SetHttpClient(httpClient)
    
    // 5. 创建请求
    request := &types.ChatCompletionRequest{
        Model: "gpt-3.5-turbo",
        Messages: []types.ChatCompletionMessage{
            {
                Role:    "user",
                Content: "Hello, how are you?",
            },
        },
    }
    
    // 6. 处理请求
    response, err := requestProcessor.ProcessRequest(context.Background(), request, "openai")
    if err != nil {
        panic(err)
    }
    
    // 7. 处理响应
    fmt.Printf("Response: %v\n", response.Choices[0].Message.Content)
}
```

### 自定义装饰器

```go
type CustomDecorator struct {
    name     string
    priority int
}

func (d *CustomDecorator) Decorate(ctx context.Context, request *types.ChatCompletionRequest) error {
    // 添加自定义逻辑
    if request.Temperature == nil {
        temp := 0.7
        request.Temperature = &temp
    }
    return nil
}

func (d *CustomDecorator) Name() string {
    return d.name
}

func (d *CustomDecorator) Priority() int {
    return d.priority
}

func (d *CustomDecorator) ShouldApply(ctx context.Context, request *types.ChatCompletionRequest) bool {
    return true
}
```

### 自定义转换器

```go
type CustomTransformer struct {
    name string
    providerConfigs map[string]*types.ProviderConfig
}

func (t *CustomTransformer) Transform(ctx context.Context, request *types.ChatCompletionRequest) (*types.GenericHttpRequest, error) {
    // 实现自定义转换逻辑
    // 将 OpenAI 格式转换为目标服务商格式
    return genericRequest, nil
}

func (t *CustomTransformer) TransformResponse(ctx context.Context, response *types.GenericHttpResponse, originalRequest *types.ChatCompletionRequest) (*types.ChatCompletionResponse, error) {
    // 实现响应转换逻辑
    // 将服务商响应转换回 OpenAI 格式
    return chatResponse, nil
}
```

## 配置示例

### 服务商配置

```go
// OpenAI 配置
openaiConfig := &types.ProviderConfig{
    Name:    "openai",
    BaseURL: "https://api.openai.com",
    APIKey:  "your-openai-api-key",
    Settings: map[string]interface{}{
        "timeout": 30,
        "retries": 3,
    },
}

// Azure OpenAI 配置
azureConfig := &types.ProviderConfig{
    Name:    "azure",
    BaseURL: "https://your-resource.openai.azure.com",
    APIKey:  "your-azure-api-key",
    Settings: map[string]interface{}{
        "api_version": "2023-12-01-preview",
        "deployment_id": "your-deployment-id",
    },
}
```

### 重试策略配置

```go
retryPolicy := &types.RetryPolicy{
    MaxRetries:    3,
    InitialDelay:  time.Second,
    MaxDelay:      10 * time.Second,
    BackoffFactor: 2.0,
    RetryableErrors: []string{"timeout", "rate_limit"},
}
```

## 扩展点

### 1. 添加新的服务商支持
1. 实现 `ChatCompletionTransformer` 接口
2. 注册到 `TransformerRegistry`
3. 配置服务商特定参数

### 2. 添加中间件功能
1. 实现 `Middleware` 接口
2. 添加到 `MiddlewareChain`
3. 支持认证、限流、缓存等功能

### 3. 添加监控和指标
1. 实现 `MetricsCollector` 接口
2. 集成到请求处理流程
3. 支持 Prometheus、Grafana 等监控系统

## 最佳实践

1. **错误处理**: 使用结构化错误，包含错误类型和详细信息
2. **日志记录**: 使用结构化日志，包含请求 ID 和上下文信息
3. **性能优化**: 使用连接池、请求缓存等优化技术
4. **安全性**: 不在日志中记录敏感信息，使用安全的认证方式
5. **可观测性**: 添加指标、追踪和日志，便于监控和调试

## 目录结构

```
pkg/
├── types/              # 类型定义
│   ├── openai.go       # OpenAI API 结构
│   └── http_request.go # HTTP 请求结构
├── interfaces/         # 接口定义
│   └── interfaces.go   # 所有核心接口
├── decorator/          # 装饰器实现
│   └── chat_completion.go
├── transformer/        # 转换器实现
│   └── chat_completion.go
├── processor/          # 处理器实现
│   └── processor.go
├── client/            # HTTP 客户端
│   └── http_client.go
└── examples/          # 使用示例
    └── gateway_example.go
```

这个设计提供了高度的灵活性和可扩展性，可以轻松添加新的服务商支持、中间件功能和自定义逻辑。