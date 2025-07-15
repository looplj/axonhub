package middleware

import (
	"errors"
	"strings"
)

// ExtractAPIKeyFromHeader 从 Authorization header 中提取 API key
// 返回提取的 API key 和可能的错误
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