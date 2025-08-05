package middleware

import (
	"errors"
	"net/http"
	"strings"
)

// APIKeyConfig 配置 API key 提取的选项.
type APIKeyConfig struct {
	// Headers 定义要检查的 header 名称列表，按优先级排序
	Headers []string
	// RequireBearer 是否要求 Bearer 前缀（仅对 Authorization header 有效）
	RequireBearer bool
	// AllowedPrefixes 允许的前缀列表（如 "Bearer ", "Token ", 等）
	AllowedPrefixes []string
}

// DefaultAPIKeyConfig 返回默认的 API key 配置.
func DefaultAPIKeyConfig() *APIKeyConfig {
	return &APIKeyConfig{
		Headers:         []string{"Authorization", "X-API-Key", "X-Api-Key", "API-Key", "Api-Key"},
		RequireBearer:   false, // 改为不强制要求 Bearer
		AllowedPrefixes: []string{"Bearer ", "Token ", "Api-Key ", "API-Key "},
	}
}

// ExtractAPIKeyFromHeader 从 Authorization header 中提取 API key（保持向后兼容）
// 返回提取的 API key 和可能的错误.
func ExtractAPIKeyFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("Authorization header is required")
	}

	// 检查是否以 "Bearer " 开头
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", errors.New("Authorization header must start with 'Bearer '")
	}

	// 提取 API key
	apiKeyValue := strings.TrimPrefix(authHeader, "Bearer ")
	if apiKeyValue == "" {
		return "", errors.New("API key is required")
	}

	return apiKeyValue, nil
}

// ExtractAPIKeyFromRequest 从 HTTP 请求中提取 API key，支持多个 headers 和前缀.
func ExtractAPIKeyFromRequest(r *http.Request, config *APIKeyConfig) (string, error) {
	if config == nil {
		config = DefaultAPIKeyConfig()
	}

	var lastError error

	// 按优先级检查每个 header
	for _, headerName := range config.Headers {
		headerValue := r.Header.Get(headerName)
		if headerValue == "" {
			continue
		}

		// 对于 Authorization header，如果配置要求 Bearer 前缀
		if strings.ToLower(headerName) == "authorization" && config.RequireBearer {
			if !strings.HasPrefix(headerValue, "Bearer ") {
				lastError = errors.New("Authorization header must start with 'Bearer '")
				continue
			}

			apiKey := strings.TrimPrefix(headerValue, "Bearer ")
			if apiKey == "" {
				lastError = errors.New("API key is required")
				continue
			}

			return apiKey, nil
		}

		// 尝试匹配允许的前缀
		var (
			apiKey      string
			foundPrefix bool
		)

		for _, prefix := range config.AllowedPrefixes {
			if strings.HasPrefix(headerValue, prefix) {
				apiKey = strings.TrimPrefix(headerValue, prefix)
				foundPrefix = true

				break
			}
		}

		// 如果没有找到匹配的前缀，直接使用原值（支持无前缀的 API key）
		if !foundPrefix {
			apiKey = headerValue
		}

		// 验证 API key 不为空
		if strings.TrimSpace(apiKey) == "" {
			lastError = errors.New("API key is required")
			continue
		}

		return strings.TrimSpace(apiKey), nil
	}

	// 如果所有 headers 都没有找到有效的 API key
	if lastError != nil {
		return "", lastError
	}

	return "", errors.New("API key not found in any of the supported headers")
}

// ExtractAPIKeyFromRequestSimple 简化版本，使用默认配置.
func ExtractAPIKeyFromRequestSimple(r *http.Request) (string, error) {
	return ExtractAPIKeyFromRequest(r, nil)
}
