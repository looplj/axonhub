# Middleware Package

这个包提供了用于 HTTP 请求处理的中间件功能，特别是 API key 验证。

## API Key 验证中间件

### 基本用法

```go
import (
    "github.com/looplj/axonhub/server/middleware"
    "github.com/gin-gonic/gin"
)

// 使用默认配置
router.Use(middleware.WithAPIKey(entClient))
```

### 支持的 Header 格式

默认配置支持以下 headers（按优先级排序）：

1. `Authorization`
2. `X-API-Key`
3. `X-Api-Key`
4. `API-Key`
5. `Api-Key`

### 支持的前缀格式

- `Bearer sk-1234567890abcdef`
- `Token sk-1234567890abcdef`
- `Api-Key sk-1234567890abcdef`
- `API-Key sk-1234567890abcdef`
- `sk-1234567890abcdef` (无前缀)

### 示例请求

```bash
# 使用 Authorization header with Bearer
curl -H "Authorization: Bearer sk-1234567890abcdef" http://localhost:8080/api/chat

# 使用 X-API-Key header
curl -H "X-API-Key: sk-1234567890abcdef" http://localhost:8080/api/chat

# 使用 Token 前缀
curl -H "Authorization: Token sk-1234567890abcdef" http://localhost:8080/api/chat

# 使用无前缀格式
curl -H "X-API-Key: sk-1234567890abcdef" http://localhost:8080/api/chat
```

### 自定义配置

```go
// 创建自定义配置
config := &middleware.APIKeyConfig{
    Headers:         []string{"Custom-API-Key", "Authorization"},
    RequireBearer:   true,  // 对 Authorization header 强制要求 Bearer 前缀
    AllowedPrefixes: []string{"Bearer ", "Custom "},
}

// 使用自定义配置
router.Use(middleware.WithAPIKeyConfig(entClient, config))
```

### 仅支持特定 headers

```go
// 只支持 X-API-Key header
config := &middleware.APIKeyConfig{
    Headers:         []string{"X-API-Key"},
    RequireBearer:   false,
    AllowedPrefixes: []string{}, // 不允许任何前缀
}

router.Use(middleware.WithAPIKeyConfig(entClient, config))
```

### 向后兼容

原有的 `ExtractAPIKeyFromHeader` 函数仍然可用，保持完全向后兼容：

```go
apiKey, err := middleware.ExtractAPIKeyFromHeader("Bearer sk-1234567890abcdef")
```

### 新的提取函数

```go
// 使用默认配置从请求中提取 API key
apiKey, err := middleware.ExtractAPIKeyFromRequestSimple(request)

// 使用自定义配置
apiKey, err := middleware.ExtractAPIKeyFromRequest(request, config)
```

## 功能特性

- ✅ 支持多个 header 名称
- ✅ 支持多种前缀格式
- ✅ 支持无前缀的 API key
- ✅ 可配置的优先级
- ✅ 自动去除前后空格
- ✅ 完全向后兼容
- ✅ 详细的错误信息
- ✅ 高性能实现

## 错误处理

中间件会返回适当的 HTTP 状态码：

- `401 Unauthorized`: API key 缺失、无效或格式错误
- `500 Internal Server Error`: 数据库查询失败

错误响应格式：

```json
{
  "error": "API key not found in any of the supported headers"
}
```