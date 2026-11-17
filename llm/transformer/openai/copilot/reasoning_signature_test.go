// 职责: 验证 Copilot 推理状态原样传递及已知其他提供商签名的过滤。
// 关联文件:
//
//	llm/transformer/openai/copilot/reasoning_signature.go
//	llm/transformer/shared/signature.go
package copilot

import (
	"encoding/base64"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestCopilotReasoningOpaqueProviderFiltering(t *testing.T) {
	opaque := "V+l5RhjeGPTC"
	require.Equal(t, opaque, *reasoningOpaqueForReplay(&opaque))
	require.Nil(t, shared.DecodeGeminiThoughtSignature(&opaque))
	require.Nil(t, shared.DecodeAnthropicSignature(&opaque))
	require.Nil(t, shared.DecodeOpenAIEncryptedContent(&opaque))
	for _, signature := range []string{
		"CgR0ZXN0",
		base64.StdEncoding.EncodeToString([]byte("claude-sonnet-5")),
		"gAAAA_openai",
		"", "  ",
	} {
		require.Nil(t, reasoningOpaqueForReplay(lo.ToPtr(signature)), signature)
	}
	require.Nil(t, reasoningOpaqueForReplay(nil))
	raw := "  V+l5RhjeGPTC  "
	require.Equal(t, raw, *reasoningOpaqueForReplay(&raw))
}
