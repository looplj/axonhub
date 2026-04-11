package orchestrator

import (
	"context"
	"unicode"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

// filterResolvedCandidatesForRequest evaluates `When` only after association
// resolution. This keeps the matching step focused on static association
// structure and makes request-dependent filtering easier to follow.
func filterResolvedCandidatesForRequest(
	ctx context.Context,
	req *llm.Request,
	resolvedCandidates []*resolvedAssociationCandidate,
) []*ChannelModelsCandidate {
	if len(resolvedCandidates) == 0 {
		return []*ChannelModelsCandidate{}
	}

	hasConditionalCandidates := lo.ContainsBy(resolvedCandidates, func(candidate *resolvedAssociationCandidate) bool {
		return candidate != nil && candidate.when != nil
	})
	if !hasConditionalCandidates {
		return aggregateChannelModelCandidates(resolvedCandidates)
	}

	promptTokens := estimatePromptTokens(req)
	filtered := make([]*resolvedAssociationCandidate, 0, len(resolvedCandidates))

	for _, candidate := range resolvedCandidates {
		if candidate == nil {
			continue
		}

		if !matchesAssociationWhen(promptTokens, candidate.when) {
			continue
		}

		filtered = append(filtered, candidate)
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "evaluated conditional associations",
			log.Int64("estimated_prompt_tokens", promptTokens),
			log.Int("matched_candidate_count", len(filtered)),
		)
	}

	return aggregateChannelModelCandidates(filtered)
}

func matchesAssociationWhen(promptTokens int64, when *objects.ModelAssociationWhen) bool {
	if when == nil {
		return true
	}

	if !when.Enabled {
		return true
	}

	if when.Condition != nil && !objects.Evaluate(*when.Condition, map[string]any{
		"prompt_tokens": promptTokens,
	}) {
		return false
	}

	return true
}

func estimatePromptTokens(req *llm.Request) int64 {
	if req == nil {
		return 0
	}

	total := 0
	for _, message := range req.Messages {
		total += estimateTokens(message.Role)
		total += estimateTokensPtr(message.Name)
		total += countPromptMessageContent(message.Content)
		total += estimateTokensPtr(message.ToolCallID)
		total += estimateTokensPtr(message.ToolCallName)
		total += estimateTokensPtr(message.ReasoningContent)

		for _, toolCall := range message.ToolCalls {
			total += estimateTokens(toolCall.Type)
			total += estimateTokens(toolCall.Function.Name)
			total += estimateTokens(toolCall.Function.Arguments)
		}
	}

	total += countPromptTools(req.Tools)

	return int64(total)
}

func countPromptMessageContent(content llm.MessageContent) int {
	tokens := estimateTokensPtr(content.Content)
	for _, part := range content.MultipleContent {
		tokens += estimateTokens(part.Type)
		tokens += estimateTokensPtr(part.Text)

		if part.ImageURL != nil {
			tokens += 128
			tokens += estimateTokensPtr(part.ImageURL.Detail)
		}

		if part.VideoURL != nil {
			tokens += 128
		}

		if part.Document != nil {
			tokens += min(estimateTokens(part.Document.URL), 512)
			tokens += estimateTokens(part.Document.MIMEType)
		}

		if part.InputAudio != nil {
			tokens += 128
			tokens += estimateTokens(part.InputAudio.Format)
		}

		if part.Compact != nil {
			tokens += min(estimateTokens(part.Compact.EncryptedContent), 256)
		}
	}

	return tokens
}

func countPromptTools(tools []llm.Tool) int {
	if len(tools) == 0 {
		return 0
	}

	total := 0
	for _, tool := range tools {
		total += estimateTokens(tool.Type)

		total += estimateTokens(tool.Function.Name)
		total += estimateTokens(tool.Function.Description)
		total += estimateTokens(string(tool.Function.Parameters))

		if tool.WebSearch != nil {
			if tool.WebSearch.MaxUses != nil {
				total += 2
			}

			if tool.WebSearch.Strict != nil {
				total++
			}

			for _, domain := range tool.WebSearch.AllowedDomains {
				total += estimateTokens(domain)
			}

			for _, domain := range tool.WebSearch.BlockedDomains {
				total += estimateTokens(domain)
			}

			total += estimateTokens(tool.WebSearch.UserLocation.City)
			total += estimateTokens(tool.WebSearch.UserLocation.Country)
			total += estimateTokens(tool.WebSearch.UserLocation.Region)
			total += estimateTokens(tool.WebSearch.UserLocation.Timezone)
			total += estimateTokens(tool.WebSearch.UserLocation.Type)
		}

		if tool.ResponseCustomTool != nil {
			total += estimateTokens(tool.ResponseCustomTool.Name)

			total += estimateTokens(tool.ResponseCustomTool.Description)
			if tool.ResponseCustomTool.Format != nil {
				total += estimateTokens(tool.ResponseCustomTool.Format.Type)
				total += estimateTokens(tool.ResponseCustomTool.Format.Syntax)
				total += estimateTokens(tool.ResponseCustomTool.Format.Definition)
			}
		}
	}

	return total
}

func estimateTokens(value string) int {
	if len(value) == 0 {
		return 0
	}

	var (
		cjkChars   int
		otherChars int
	)

	for _, r := range value {
		if isCJK(r) {
			cjkChars++
			continue
		}

		if unicode.IsSpace(r) {
			continue
		}

		otherChars++
	}

	cjkTokens := float64(cjkChars) / 1.5
	otherTokens := float64(otherChars) / 4.0

	total := int(cjkTokens + otherTokens)
	if total == 0 {
		return 1
	}

	return total
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r)
}

func estimateTokensPtr(value *string) int {
	if value == nil {
		return 0
	}

	return estimateTokens(*value)
}
