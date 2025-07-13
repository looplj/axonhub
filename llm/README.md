# LLM Gateway Components

本目录包含了重构后的LLM网关组件，提供了模块化的请求处理、转换和装饰功能。

## 目录结构

```
llm/
├── decorator/                 # 装饰器组件
│   ├── interfaces.go         # 装饰器接口定义
│   ├── chat_completion.go    # 聊天完成装饰器实现
│   └── chain.go             # 装饰器链实现
├── transformer/              # 转换器组件
│   ├── interfaces.go        # 转换器接口定义
│   ├── registry.go          # 转换器注册表
│   ├── chat_completion.go   # 通用聊天完成转换器
│   ├── anthropic/           # Anthropic API 转换器
│   │   ├── types.go        # Anthropic 类型定义
│   │   ├── inbound.go      # 入站转换器
│   │   └── outbound.go     # 出站转换器
│   ├── doubao/             # 豆包 API 转换器
│   │   ├── types.go        # 豆包类型定义
│   │   ├── inbound.go      # 入站转换器
│   │   └── outbound.go     # 出站转换器
│   └── deepseek/           # DeepSeek API 转换器
│       ├── types.go        # DeepSeek 类型定义
│       ├── inbound.go      # 入站转换器
│       └── outbound.go     # 出站转换器
├── types/                   # 通用类型定义
│   ├── openai.go           # OpenAI 格式类型
│   └── http_request.go     # HTTP 请求类型
└── example/                # 使用示例
    └── usage.go            # 使用示例代码
```

## 核心概念

### 装饰器 (Decorator)

装饰器用于在请求发送到模型提供商之前修改 `ChatCompletionRequest`。

- **ChatCompletionDecorator**: 装饰器接口
- **DecoratorChain**: 装饰器链，支持多个装饰器的顺序执行

### 转换器 (Transformer)

转换器分为两种类型：

- **InboundTransformer**: 将 HTTP 请求转换为标准的 `ChatCompletionRequest`
- **OutboundTransformer**: 将 `ChatCompletionRequest` 转换为供应商特定的格式

### 支持的供应商

1. **Anthropic**: Claude API
2. **豆包**: ByteDance 豆包 API
3. **DeepSeek**: DeepSeek API

## 使用方法

### 基本使用

```go
package main

import (
    "context"
    "github.com/looplj/axonhub/llm/decorator"
    "github.com/looplj/axonhub/llm/transformer"
    "github.com/looplj/axonhub/llm/transformer/anthropic"
)

func main() {
    ctx := context.Background()
    
    // 创建装饰器
    decorator := decorator.NewChatCompletionDecoratorImpl("default")
    decorator.SetDefaultTemperature(0.7)
    
    // 创建装饰器链
    chain := decorator.NewChain()
    chain.Add(decorator)
    
    // 创建转换器注册表
    registry := transformer.NewRegistry()
    registry.RegisterOutboundTransformer("anthropic", anthropic.NewOutboundTransformer())
    
    // 处理请求
    request := &types.ChatCompletionRequest{
        Model: "claude-3-sonnet-20240229",
        Messages: []types.ChatCompletionMessage{
            {
                Role:    "user",
                Content: "Hello!",
            },
        },
    }
    
    // 应用装饰器
    decoratedRequest, err := chain.Execute(ctx, request)
    if err != nil {
        // 处理错误
    }
    
    // 获取转换器并转换请求
    transformer, err := registry.GetOutboundTransformer("anthropic")
    if err != nil {
        // 处理错误
    }
    
    httpRequest, err := transformer.Transform(ctx, decoratedRequest)
    if err != nil {
        // 处理错误
    }
    
    // 发送 HTTP 请求...
}
```

### 添加新的供应商

要添加新的供应商支持，需要：

1. 在 `transformer/` 下创建新的供应商目录
2. 定义供应商特定的类型 (`types.go`)
3. 实现 `InboundTransformer` (`inbound.go`)
4. 实现 `OutboundTransformer` (`outbound.go`)
5. 在注册表中注册新的转换器

### 自定义装饰器

```go
type CustomDecorator struct {
    name string
}

func (d *CustomDecorator) Decorate(ctx context.Context, request *types.ChatCompletionRequest) (*types.ChatCompletionRequest, error) {
    // 自定义装饰逻辑
    request.User = "custom-user"
    return request, nil
}

func (d *CustomDecorator) Name() string {
    return d.name
}
```

## 特性

- **模块化设计**: 装饰器和转换器分离，易于扩展
- **供应商无关**: 统一的接口支持多个 LLM 供应商
- **类型安全**: 完整的类型定义和接口约束
- **并发安全**: 注册表和装饰器链支持并发访问
- **可扩展**: 易于添加新的供应商和装饰器

## 接口设计

### 装饰器接口

```go
type ChatCompletionDecorator interface {
    Decorate(ctx context.Context, request *types.ChatCompletionRequest) (*types.ChatCompletionRequest, error)
    Name() string
}

type DecoratorChain interface {
    Add(decorator ChatCompletionDecorator)
    Remove(name string)
    Execute(ctx context.Context, request *types.ChatCompletionRequest) (*types.ChatCompletionRequest, error)
    List() []ChatCompletionDecorator
    Clear()
    Size() int
}
```

### 转换器接口

```go
type InboundTransformer interface {
    Transform(ctx context.Context, httpReq *http.Request) (*types.ChatCompletionRequest, error)
    SupportsContentType(contentType string) bool
    Name() string
}

type OutboundTransformer interface {
    Transform(ctx context.Context, request *types.ChatCompletionRequest) (*types.GenericHttpRequest, error)
    TransformResponse(ctx context.Context, response *types.GenericHttpResponse, originalRequest *types.ChatCompletionRequest) (*types.ChatCompletionResponse, error)
    TransformStreamResponse(ctx context.Context, response *types.GenericHttpResponse, originalRequest *types.ChatCompletionRequest) (<-chan *types.ChatCompletionStreamResponse, error)
    SupportsProvider(provider string) bool
    Name() string
}
```

## 注意事项

1. 所有转换器都应该正确处理错误情况
2. 流式响应需要适当的资源清理
3. 装饰器应该保持请求的完整性
4. 供应商特定的认证信息应该通过 `GatewayMetadata` 传递