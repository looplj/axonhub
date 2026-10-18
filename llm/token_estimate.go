package llm

import "unicode"

// EstimatePromptTokens estimates the prompt size of a request in tokens.
//
// This is a deliberately cheap, tokenizer-free heuristic: CJK runes count as
// 1/1.5 tokens and everything else as 1/4 token. It answers "roughly how big is
// this prompt", which is all callers need:
//
//   - channel association conditions compare it against model thresholds;
//   - the Anthropic inbound stream seeds `message_start.usage.input_tokens`
//     before upstream usage is known (see below).
//
// Why the inbound stream needs it: `message_start` is the ONLY event most
// Anthropic clients read `input_tokens` from. Claude Code, for example, keeps
// `message_start.usage.input_tokens` verbatim and only overwrites
// `output_tokens` when `message_delta` arrives. OpenAI-style upstreams report
// usage in a trailing chunk, so at `message_start` time the real count is not
// available yet. Reporting a placeholder (or zero) there leaves the client
// permanently showing the placeholder; reporting an estimate keeps the number
// in the right order of magnitude until the terminal event corrects it.
func EstimatePromptTokens(req *Request) int64 {
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

func countPromptMessageContent(content MessageContent) int {
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

func countPromptTools(tools []Tool) int {
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
