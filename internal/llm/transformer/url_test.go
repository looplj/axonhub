package transformer

import (
	"testing"
)

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		version  string
		expected string
	}{
		{
			name:     "empty URL",
			url:      "",
			version:  "v1",
			expected: "",
		},
		{
			name:     "URL with trailing slash and version",
			url:      "https://api.example.com/",
			version:  "v1",
			expected: "https://api.example.com/v1",
		},
		{
			name:     "URL without trailing slash and version",
			url:      "https://api.example.com",
			version:  "v1",
			expected: "https://api.example.com/v1",
		},
		{
			name:     "URL already has version suffix",
			url:      "https://api.example.com/v1",
			version:  "v1",
			expected: "https://api.example.com/v1",
		},
		{
			name:     "URL already has version in path",
			url:      "https://api.example.com/v1/openai",
			version:  "v1",
			expected: "https://api.example.com/v1/openai",
		},
		{
			name:     "URL with trailing slash and version in path",
			url:      "https://api.example.com/v1/openai/",
			version:  "v1",
			expected: "https://api.example.com/v1/openai",
		},
		{
			name:     "URL with # suffix - no version",
			url:      "https://api.example.com/v1#",
			version:  "",
			expected: "https://api.example.com/v1",
		},
		{
			name:     "URL with # suffix - with version",
			url:      "https://api.example.com#",
			version:  "v1",
			expected: "https://api.example.com",
		},
		{
			name:     "URL with # and trailing slash",
			url:      "https://api.example.com/#",
			version:  "v1",
			expected: "https://api.example.com",
		},
		{
			name:     "URL without version parameter",
			url:      "https://api.example.com",
			version:  "",
			expected: "https://api.example.com",
		},
		{
			name:     "URL with trailing slash without version parameter",
			url:      "https://api.example.com/",
			version:  "",
			expected: "https://api.example.com",
		},
		{
			name:     "URL with # suffix without version parameter",
			url:      "https://api.example.com/v1#",
			version:  "",
			expected: "https://api.example.com/v1",
		},
		{
			name:     "OpenAI standard URL",
			url:      "https://api.openai.com",
			version:  "v1",
			expected: "https://api.openai.com/v1",
		},
		{
			name:     "OpenAI URL with v1 already",
			url:      "https://api.openai.com/v1",
			version:  "v1",
			expected: "https://api.openai.com/v1",
		},
		{
			name:     "Anthropic standard URL",
			url:      "https://api.anthropic.com",
			version:  "v1",
			expected: "https://api.anthropic.com/v1",
		},
		{
			name:     "Anthropic URL with v1 already",
			url:      "https://api.anthropic.com/v1",
			version:  "v1",
			expected: "https://api.anthropic.com/v1",
		},
		{
			name:     "Gemini standard URL",
			url:      "https://generativelanguage.googleapis.com",
			version:  "v1beta",
			expected: "https://generativelanguage.googleapis.com/v1beta",
		},
		{
			name:     "Gemini URL with v1beta already",
			url:      "https://generativelanguage.googleapis.com/v1beta",
			version:  "v1beta",
			expected: "https://generativelanguage.googleapis.com/v1beta",
		},
		{
			name:     "Azure OpenAI URL",
			url:      "https://my-resource.openai.azure.com",
			version:  "openai/v1",
			expected: "https://my-resource.openai.azure.com/openai/v1",
		},
		{
			name:     "Azure OpenAI URL with version already",
			url:      "https://my-resource.openai.azure.com/openai/v1",
			version:  "openai/v1",
			expected: "https://my-resource.openai.azure.com/openai/v1",
		},
		{
			name:     "DeepInfra URL with v1 in path",
			url:      "https://api.deepinfra.com/v1/openai",
			version:  "v1",
			expected: "https://api.deepinfra.com/v1/openai",
		},
		{
			name:     "SiliconFlow URL",
			url:      "https://api.siliconflow.cn",
			version:  "v1",
			expected: "https://api.siliconflow.cn/v1",
		},
		{
			name:     "SiliconFlow URL with v1 already",
			url:      "https://api.siliconflow.cn/v1",
			version:  "v1",
			expected: "https://api.siliconflow.cn/v1",
		},
		{
			name:     "Jina URL",
			url:      "https://api.jina.ai",
			version:  "v1",
			expected: "https://api.jina.ai/v1",
		},
		{
			name:     "Jina URL with v1 already",
			url:      "https://api.jina.ai/v1",
			version:  "v1",
			expected: "https://api.jina.ai/v1",
		},
		{
			name:     "Zai URL with v4",
			url:      "https://api.zai.com",
			version:  "v4",
			expected: "https://api.zai.com/v4",
		},
		{
			name:     "Zai URL with v4 already",
			url:      "https://api.zai.com/v4",
			version:  "v4",
			expected: "https://api.zai.com/v4",
		},
		{
			name:     "Zai URL with # suffix",
			url:      "https://api.zai.com/v4#",
			version:  "v4",
			expected: "https://api.zai.com/v4",
		},
		{
			name:     "Vertex AI URL with # suffix",
			url:      "https://us-central1-aiplatform.googleapis.com/v1#",
			version:  "v1",
			expected: "https://us-central1-aiplatform.googleapis.com/v1",
		},
		{
			name:     "Custom URL with multiple trailing slashes",
			url:      "https://api.example.com///",
			version:  "v1",
			expected: "https://api.example.com/v1",
		},
		{
			name:     "URL with port",
			url:      "https://api.example.com:8080",
			version:  "v1",
			expected: "https://api.example.com:8080/v1",
		},
		{
			name:     "URL with port and trailing slash",
			url:      "https://api.example.com:8080/",
			version:  "v1",
			expected: "https://api.example.com:8080/v1",
		},
		{
			name:     "URL with port and v1 already",
			url:      "https://api.example.com:8080/v1",
			version:  "v1",
			expected: "https://api.example.com:8080/v1",
		},
		{
			name:     "URL with port and # suffix",
			url:      "https://api.example.com:8080/v1#",
			version:  "v1",
			expected: "https://api.example.com:8080/v1",
		},
		{
			name:     "Different version - v2",
			url:      "https://api.example.com",
			version:  "v2",
			expected: "https://api.example.com/v2",
		},
		{
			name:     "Different version - v2 already in URL",
			url:      "https://api.example.com/v2",
			version:  "v2",
			expected: "https://api.example.com/v2",
		},
		{
			name:     "Version mismatch - URL has v1, request v2",
			url:      "https://api.example.com/v1",
			version:  "v2",
			expected: "https://api.example.com/v1/v2",
		},
		{
			name:     "Version in middle of path",
			url:      "https://api.example.com/v1/api",
			version:  "v1",
			expected: "https://api.example.com/v1/api",
		},
		{
			name:     "Version in middle of path with trailing slash",
			url:      "https://api.example.com/v1/api/",
			version:  "v1",
			expected: "https://api.example.com/v1/api",
		},
		{
			name:     "OpenRouter URL",
			url:      "https://openrouter.ai/api",
			version:  "v1",
			expected: "https://openrouter.ai/api/v1",
		},
		{
			name:     "OpenRouter URL with v1 already",
			url:      "https://openrouter.ai/api/v1",
			version:  "v1",
			expected: "https://openrouter.ai/api/v1",
		},
		{
			name:     "xAI default URL",
			url:      "https://api.x.ai",
			version:  "v1",
			expected: "https://api.x.ai/v1",
		},
		{
			name:     "xAI URL with v1 already",
			url:      "https://api.x.ai/v1",
			version:  "v1",
			expected: "https://api.x.ai/v1",
		},
		{
			name:     "Doubao URL",
			url:      "https://ark.cn-beijing.volces.com/api/v3",
			version:  "v3",
			expected: "https://ark.cn-beijing.volces.com/api/v3",
		},
		{
			name:     "Doubao URL with # suffix",
			url:      "https://ark.cn-beijing.volces.com/api/v3#",
			version:  "v3",
			expected: "https://ark.cn-beijing.volces.com/api/v3",
		},
		{
			name:     "ModelScope URL",
			url:      "https://api.modelscope.cn/v1",
			version:  "v1",
			expected: "https://api.modelscope.cn/v1",
		},
		{
			name:     "LongCat URL",
			url:      "https://api.longcat.com",
			version:  "v1",
			expected: "https://api.longcat.com/v1",
		},
		{
			name:     "LongCat URL with # suffix",
			url:      "https://api.longcat.com/v1#",
			version:  "v1",
			expected: "https://api.longcat.com/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeBaseURL(tt.url, tt.version)
			if result != tt.expected {
				t.Errorf("NormalizeBaseURL(%q, %q) = %q, want %q", tt.url, tt.version, result, tt.expected)
			}
		})
	}
}
