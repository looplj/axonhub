package channel_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/channel"
)

func TestType_IsAnthropic(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		want bool
	}{
		{
			name: "anthropic",
			want: true,
		},
		{
			name: "zhipu_anthropic",
			want: false,
		},
		{
			name: "openai",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ty := channel.Type(tt.name)
			got := ty.IsAnthropic()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestType_IsAnthropicLike(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		want bool
	}{
		{
			name: "anthropic",
			want: false,
		},
		{
			name: "zhipu_anthropic",
			want: true,
		},
		{
			name: "openai",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ty := channel.Type(tt.name)
			got := ty.IsAnthropicLike()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestType_SupportsGoogleNativeTools(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		want bool
	}{
		{
			name: "gemini",
			want: true,
		},
		{
			name: "gemini_vertex",
			want: true,
		},
		{
			name: "gemini_openai",
			want: false,
		},
		{
			name: "openai",
			want: false,
		},
		{
			name: "xiaomi",
			want: false,
		},
		{
			name: "anthropic",
			want: false,
		},
		{
			name: "deepseek",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ty := channel.Type(tt.name)
			got := ty.SupportsGoogleNativeTools()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestType_SupportsAnthropicNativeTools(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		want bool
	}{
		{
			name: "anthropic",
			want: true,
		},
		{
			name: "anthropic_aws",
			want: false, // Bedrock does not support web search beta
		},
		{
			name: "anthropic_gcp",
			want: false, // Vertex does not support web search beta
		},
		{
			name: "deepseek_anthropic",
			want: false,
		},
		{
			name: "moonshot_anthropic",
			want: false,
		},
		{
			name: "openai",
			want: false,
		},
		{
			name: "gemini",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ty := channel.Type(tt.name)
			got := ty.SupportsAnthropicNativeTools()
			require.Equal(t, tt.want, got)
		})
	}
}
