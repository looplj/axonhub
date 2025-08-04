package middleware

import (
	"net/http"
	"testing"
)

func TestExtractAPIKeyFromHeader(t *testing.T) {
	tests := []struct {
		name        string
		authHeader  string
		expectedKey string
		expectedErr string
	}{
		{
			name:        "valid bearer token",
			authHeader:  "Bearer sk-1234567890abcdef",
			expectedKey: "sk-1234567890abcdef",
			expectedErr: "",
		},
		{
			name:        "empty header",
			authHeader:  "",
			expectedKey: "",
			expectedErr: "Authorization header is required",
		},
		{
			name:        "missing Bearer prefix",
			authHeader:  "sk-1234567890abcdef",
			expectedKey: "",
			expectedErr: "Authorization header must start with 'Bearer '",
		},
		{
			name:        "Bearer with lowercase",
			authHeader:  "bearer sk-1234567890abcdef",
			expectedKey: "",
			expectedErr: "Authorization header must start with 'Bearer '",
		},
		{
			name:        "Bearer without space",
			authHeader:  "Bearersk-1234567890abcdef",
			expectedKey: "",
			expectedErr: "Authorization header must start with 'Bearer '",
		},
		{
			name:        "Bearer with empty key",
			authHeader:  "Bearer ",
			expectedKey: "",
			expectedErr: "API key is required",
		},
		{
			name:        "Bearer with only spaces",
			authHeader:  "Bearer    ",
			expectedKey: "   ",
			expectedErr: "",
		},
		{
			name:        "valid key with special characters",
			authHeader:  "Bearer sk-proj-1234567890abcdef_ghijklmnop",
			expectedKey: "sk-proj-1234567890abcdef_ghijklmnop",
			expectedErr: "",
		},
		{
			name:        "multiple Bearer prefixes",
			authHeader:  "Bearer Bearer sk-1234567890abcdef",
			expectedKey: "Bearer sk-1234567890abcdef",
			expectedErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := ExtractAPIKeyFromHeader(tt.authHeader)

			if tt.expectedErr != "" {
				if err == nil {
					t.Errorf("expected error '%s', got nil", tt.expectedErr)
					return
				}
				if err.Error() != tt.expectedErr {
					t.Errorf("expected error '%s', got '%s'", tt.expectedErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if key != tt.expectedKey {
				t.Errorf("expected key '%s', got '%s'", tt.expectedKey, key)
			}
		})
	}
}

func TestExtractAPIKeyFromRequest(t *testing.T) {
	tests := []struct {
		name        string
		headers     map[string]string
		config      *APIKeyConfig
		expectedKey string
		expectedErr string
	}{
		{
			name: "Authorization header with Bearer",
			headers: map[string]string{
				"Authorization": "Bearer sk-1234567890abcdef",
			},
			config:      nil, // 使用默认配置
			expectedKey: "sk-1234567890abcdef",
			expectedErr: "",
		},
		{
			name: "X-API-Key header",
			headers: map[string]string{
				"X-API-Key": "sk-1234567890abcdef",
			},
			config:      nil,
			expectedKey: "sk-1234567890abcdef",
			expectedErr: "",
		},
		{
			name: "X-Api-Key header",
			headers: map[string]string{
				"X-Api-Key": "sk-1234567890abcdef",
			},
			config:      nil,
			expectedKey: "sk-1234567890abcdef",
			expectedErr: "",
		},
		{
			name: "API-Key header",
			headers: map[string]string{
				"API-Key": "sk-1234567890abcdef",
			},
			config:      nil,
			expectedKey: "sk-1234567890abcdef",
			expectedErr: "",
		},
		{
			name: "Authorization without Bearer prefix",
			headers: map[string]string{
				"Authorization": "sk-1234567890abcdef",
			},
			config:      nil,
			expectedKey: "sk-1234567890abcdef",
			expectedErr: "",
		},
		{
			name: "Token prefix",
			headers: map[string]string{
				"Authorization": "Token sk-1234567890abcdef",
			},
			config:      nil,
			expectedKey: "sk-1234567890abcdef",
			expectedErr: "",
		},
		{
			name: "Priority test - Authorization first",
			headers: map[string]string{
				"Authorization": "Bearer auth-key",
				"X-API-Key":     "x-api-key",
			},
			config:      nil,
			expectedKey: "auth-key",
			expectedErr: "",
		},
		{
			name: "Priority test - X-API-Key when Authorization missing",
			headers: map[string]string{
				"X-API-Key": "x-api-key",
				"API-Key":   "api-key",
			},
			config:      nil,
			expectedKey: "x-api-key",
			expectedErr: "",
		},
		{
			name: "Custom config with RequireBearer",
			headers: map[string]string{
				"Authorization": "sk-1234567890abcdef",
			},
			config: &APIKeyConfig{
				Headers:         []string{"Authorization"},
				RequireBearer:   true,
				AllowedPrefixes: []string{"Bearer "},
			},
			expectedKey: "",
			expectedErr: "Authorization header must start with 'Bearer '",
		},
		{
			name: "Custom config with custom headers",
			headers: map[string]string{
				"Custom-API-Key": "custom-key",
			},
			config: &APIKeyConfig{
				Headers:         []string{"Custom-API-Key"},
				RequireBearer:   false,
				AllowedPrefixes: []string{},
			},
			expectedKey: "custom-key",
			expectedErr: "",
		},
		{
			name: "Empty API key",
			headers: map[string]string{
				"X-API-Key": "",
			},
			config:      nil,
			expectedKey: "",
			expectedErr: "API key not found in any of the supported headers",
		},
		{
			name: "Whitespace only API key",
			headers: map[string]string{
				"X-API-Key": "   ",
			},
			config:      nil,
			expectedKey: "",
			expectedErr: "API key is required",
		},
		{
			name: "API key with leading/trailing spaces",
			headers: map[string]string{
				"X-API-Key": "  sk-1234567890abcdef  ",
			},
			config:      nil,
			expectedKey: "sk-1234567890abcdef",
			expectedErr: "",
		},
		{
			name:        "No headers provided",
			headers:     map[string]string{},
			config:      nil,
			expectedKey: "",
			expectedErr: "API key not found in any of the supported headers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建 HTTP 请求
			req, err := http.NewRequest(http.MethodGet, "/test", nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			// 设置 headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// 提取 API key
			key, err := ExtractAPIKeyFromRequest(req, tt.config)

			if tt.expectedErr != "" {
				if err == nil {
					t.Errorf("expected error '%s', got nil", tt.expectedErr)
					return
				}
				if err.Error() != tt.expectedErr {
					t.Errorf("expected error '%s', got '%s'", tt.expectedErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if key != tt.expectedKey {
				t.Errorf("expected key '%s', got '%s'", tt.expectedKey, key)
			}
		})
	}
}

func TestExtractAPIKeyFromRequestSimple(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("X-Api-Key", "simple-test-key")

	key, err := ExtractAPIKeyFromRequestSimple(req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if key != "simple-test-key" {
		t.Errorf("expected key 'simple-test-key', got '%s'", key)
	}
}

func TestDefaultAPIKeyConfig(t *testing.T) {
	config := DefaultAPIKeyConfig()

	expectedHeaders := []string{"Authorization", "X-API-Key", "X-Api-Key", "API-Key", "Api-Key"}
	if len(config.Headers) != len(expectedHeaders) {
		t.Errorf("expected %d headers, got %d", len(expectedHeaders), len(config.Headers))
	}

	for i, expected := range expectedHeaders {
		if i >= len(config.Headers) || config.Headers[i] != expected {
			t.Errorf("expected header[%d] to be '%s', got '%s'", i, expected, config.Headers[i])
		}
	}

	if config.RequireBearer {
		t.Error("expected RequireBearer to be false")
	}

	expectedPrefixes := []string{"Bearer ", "Token ", "Api-Key ", "API-Key "}
	if len(config.AllowedPrefixes) != len(expectedPrefixes) {
		t.Errorf("expected %d prefixes, got %d", len(expectedPrefixes), len(config.AllowedPrefixes))
	}
}

// BenchmarkExtractAPIKeyFromHeader 性能测试.
func BenchmarkExtractAPIKeyFromHeader(b *testing.B) {
	authHeader := "Bearer sk-1234567890abcdef"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExtractAPIKeyFromHeader(authHeader)
	}
}

// BenchmarkExtractAPIKeyFromRequest 性能测试.
func BenchmarkExtractAPIKeyFromRequest(b *testing.B) {
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer sk-1234567890abcdef")
	config := DefaultAPIKeyConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExtractAPIKeyFromRequest(req, config)
	}
}
