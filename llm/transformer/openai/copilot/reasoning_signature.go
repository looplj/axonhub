// 职责: 为 Copilot 出站筛选可回传的推理签名，排除已识别的其他来源。
// 关联文件:
//
//	llm/model.go
//	llm/transformer/shared/signature.go
//	llm/transformer/openai/copilot/outbound.go
//
// 注意事项:
//
//	未识别的非空签名允许用于 Copilot；来源判断沿用现有启发式规则。
package copilot

import (
	"strings"

	"github.com/looplj/axonhub/llm/transformer/shared"
)

// reasoningOpaqueForReplay uses a heuristic: unknown signatures are accepted,
// while signatures recognized as another provider are omitted.
func reasoningOpaqueForReplay(signature *string) *string {
	if signature == nil || strings.TrimSpace(*signature) == "" {
		return nil
	}
	if shared.GuessSignatureProvider(*signature).Provider != shared.ProviderUnknown {
		return nil
	}
	return signature
}
