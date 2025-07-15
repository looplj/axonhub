package middleware

import (
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

// BenchmarkExtractAPIKeyFromHeader 性能测试
func BenchmarkExtractAPIKeyFromHeader(b *testing.B) {
	authHeader := "Bearer sk-1234567890abcdef"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExtractAPIKeyFromHeader(authHeader)
	}
}