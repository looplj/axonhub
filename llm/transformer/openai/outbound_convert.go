package openai

import (
	"encoding/json"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
)

// RequestFromLLM creates OpenAI Request from unified llm.Request with reasoning field configuration.
func RequestFromLLM(r *llm.Request, reasoningField ReasoningField) *Request {
	if r == nil {
		return nil
	}

	req := &Request{
		Model:               r.Model,
		FrequencyPenalty:    r.FrequencyPenalty,
		Logprobs:            r.Logprobs,
		MaxCompletionTokens: r.MaxCompletionTokens,
		MaxTokens:           r.MaxTokens,
		PresencePenalty:     r.PresencePenalty,
		Seed:                r.Seed,
		Store:               r.Store,
		Temperature:         r.Temperature,
		TopLogprobs:         r.TopLogprobs,
		TopP:                r.TopP,
		PromptCacheKey:      r.PromptCacheKey,
		SafetyIdentifier:    r.SafetyIdentifier,
		User:                r.User,
		LogitBias:           r.LogitBias,
		Metadata:            r.Metadata,
		Modalities:          r.Modalities,
		ReasoningEffort:     r.ReasoningEffort,
		ReasoningBudget:     r.ReasoningBudget,
		ReasoningSummary:    r.ReasoningSummary,
		ServiceTier:         r.ServiceTier,
		Stream:              r.Stream,
		ParallelToolCalls:   r.ParallelToolCalls,
		Verbosity:           r.Verbosity,
	}

	// Restore top_k carried through TransformerMetadata (canonical llm.Request has
	// no TopK field). Mirrors the Anthropic top_k restoration, shared neutral key.
	if r.TransformerMetadata != nil {
		if topK, ok := r.TransformerMetadata[TransformerMetadataKeyTopK].(*int64); ok && topK != nil {
			req.TopK = topK
		}
	}

	// Restore OpenRouter sampling knobs (repetition_penalty/min_p/top_a) carried
	// through TransformerMetadata (mirrors top_k restoration).
	if r.TransformerMetadata != nil {
		if rp, ok := r.TransformerMetadata[TransformerMetadataKeyRepetitionPenalty].(*float64); ok && rp != nil {
			req.RepetitionPenalty = rp
		}
		if minP, ok := r.TransformerMetadata[TransformerMetadataKeyMinP].(*float64); ok && minP != nil {
			req.MinP = minP
		}
		if topA, ok := r.TransformerMetadata[TransformerMetadataKeyTopA].(*float64); ok && topA != nil {
			req.TopA = topA
		}
	}

	// Restore top-level cache_control carried through TransformerMetadata as
	// opaque json.RawMessage (mirrors top_k restoration).
	if r.TransformerMetadata != nil {
		if raw, ok := r.TransformerMetadata[TransformerMetadataKeyCacheControl].(json.RawMessage); ok && len(raw) > 0 {
			var cc CacheControl
			if err := json.Unmarshal(raw, &cc); err == nil && cc.Type != "" {
				req.CacheControl = &cc
			}
		}
	}

	// Convert messages
	req.Messages = lo.Map(r.Messages, func(m llm.Message, _ int) Message {
		return MessageFromLLMWithConfig(m, reasoningField)
	})

	// Convert Stop
	if r.Stop != nil {
		req.Stop = &Stop{
			Stop:         r.Stop.Stop,
			MultipleStop: r.Stop.MultipleStop,
		}
	}

	// Convert StreamOptions
	if r.StreamOptions != nil {
		req.StreamOptions = &StreamOptions{
			IncludeUsage: r.StreamOptions.IncludeUsage,
		}
	}

	// Convert Chat-supported tools. Keep function/custom only.
	// image_generation/web_search/google_* are omitted here and diagnosed by
	// recordOpenAIChatUnsupportedNativeToolLossyDowngrades. Responses custom tools
	// bridge into Chat custom via ResponseCustomTool/OpenAIChatCustomTool.
	req.Tools = lo.FilterMap(r.Tools, func(t llm.Tool, _ int) (Tool, bool) {
		if t.Type == llm.ToolTypeFunction {
			return ToolFromLLM(t), true
		}
		if t.Type == llm.ToolTypeResponsesCustomTool && t.ResponseCustomTool != nil {
			return ToolFromLLM(responsesCustomToolToOpenAIChatTool(t)), true
		}
		if t.Type == "custom" && t.OpenAIChatCustomTool != nil {
			return ToolFromLLM(t), true
		}
		return Tool{}, false
	})

	// Convert ToolChoice
	if r.ToolChoice != nil {
		req.ToolChoice = &ToolChoice{
			ToolChoice: r.ToolChoice.ToolChoice,
		}
		if r.ToolChoice.NamedToolChoice != nil {
			req.ToolChoice.NamedToolChoice = &NamedToolChoice{
				Type: r.ToolChoice.NamedToolChoice.Type,
				Function: ToolFunction{
					Name: r.ToolChoice.NamedToolChoice.Function.Name,
				},
			}
		}
		if r.ToolChoice.OpenAIChatCustomToolChoice != nil {
			req.ToolChoice.Custom = &CustomToolChoice{
				Name: r.ToolChoice.OpenAIChatCustomToolChoice.Name,
			}
		}
		if r.ToolChoice.OpenAIChatAllowedTools != nil {
			req.ToolChoice.AllowedTools = &AllowedToolsToolChoice{
				Mode:  r.ToolChoice.OpenAIChatAllowedTools.Mode,
				Tools: append([]json.RawMessage(nil), r.ToolChoice.OpenAIChatAllowedTools.Tools...),
			}
		}
	}

	// Convert ResponseFormat
	if r.ResponseFormat != nil {
		req.ResponseFormat = &ResponseFormat{
			Type:       r.ResponseFormat.Type,
			JSONSchema: r.ResponseFormat.JSONSchema,
		}
	}

	if len(req.Tools) == 0 {
		req.ParallelToolCalls = nil
	}

	return req
}

// applyReasoningEffortMapping replaces reasoning_effort according to a per-channel mapping.
// The first entry whose From matches the effort value wins; values not in the list (or an
// empty/nil list) pass through unchanged. This lets non-standard OpenAI-compatible providers
// (ollama, opencode, evolink, self-hosted gateways) opt in to conversions like xhigh→max
// without affecting standard OpenAI channels. Applied in OutboundTransformer.TransformRequest.
func applyReasoningEffortMapping(effort string, mappings []llm.ReasoningEffortMapping) string {
	if len(mappings) == 0 || effort == "" {
		return effort
	}
	for _, m := range mappings {
		if m.From == effort {
			return m.To
		}
	}
	return effort
}

// MessageFromLLM creates OpenAI Message from unified llm.Message.
// Defaults to ReasoningFieldAll to preserve both reasoning fields.
func MessageFromLLM(m llm.Message) Message {
	return MessageFromLLMWithConfig(m, ReasoningFieldAll)
}

// MessageFromLLMWithConfig creates OpenAI Message from unified llm.Message with reasoning field configuration.
func MessageFromLLMWithConfig(m llm.Message, reasoningField ReasoningField) Message {
	var reasoningContent, reasoning *string
	reasoningDetails := m.ReasoningDetails

	// Apply reasoning field configuration
	switch reasoningField {
	case ReasoningFieldContent:
		// Only use reasoning_content field
		// Prefer ReasoningContent, fallback to Reasoning if ReasoningContent is nil
		reasoningContent = m.ReasoningContent
		if reasoningContent == nil && m.Reasoning != nil {
			reasoningContent = m.Reasoning
		}
		reasoning = nil
	case ReasoningFieldReasoning:
		// Only use reasoning field
		// Prefer Reasoning, fallback to ReasoningContent if Reasoning is nil
		reasoning = m.Reasoning
		if reasoning == nil && m.ReasoningContent != nil {
			reasoning = m.ReasoningContent
		}
		reasoningContent = nil
	case ReasoningFieldNone:
		// Strip all reasoning fields
		reasoningContent = nil
		reasoning = nil
		reasoningDetails = nil
	default: // ReasoningFieldAll
		// Preserve both reasoning fields with sync logic
		reasoningContent = m.ReasoningContent
		reasoning = m.Reasoning

		// Sync: if one field has value and the other is nil/empty, copy the value
		if reasoningContent == nil && reasoning != nil && *reasoning != "" {
			reasoningContent = reasoning
		}
		if reasoning == nil && reasoningContent != nil && *reasoningContent != "" {
			reasoning = reasoningContent
		}
	}

	// Build the Message with determined fields
	msg := Message{
		Role:             m.Role,
		Name:             m.Name,
		Refusal:          m.Refusal,
		ToolCallID:       m.ToolCallID,
		ReasoningContent: reasoningContent,
		Reasoning:        reasoning,
		ReasoningDetails: reasoningDetails,
		Images:           m.Images,
	}

	if m.Audio != nil {
		msg.Audio = &OutputAudio{
			ID:         m.Audio.ID,
			Data:       m.Audio.Data,
			ExpiresAt:  m.Audio.ExpiresAt,
			Transcript: m.Audio.Transcript,
		}
	}

	// Convert Content
	msg.Content = messageContentFromLLMForRole(m.Content, m.Role)

	// Convert ToolCalls. Deprecated function_call origins must round-trip as
	// legacy function_call, not modern tool_calls, for multi-turn Chat history.
	if shouldEmitDeprecatedFunctionCall(m.ToolCalls, nil) && len(m.ToolCalls) > 0 {
		first := m.ToolCalls[0]
		msg.FunctionCall = &FunctionCall{
			Name:      first.Function.Name,
			Arguments: first.Function.Arguments,
		}
	} else if m.ToolCalls != nil {
		msg.ToolCalls = lo.Map(m.ToolCalls, func(tc llm.ToolCall, _ int) ToolCall {
			return ToolCallFromLLM(tc)
		})
	}

	// Convert Annotations
	if len(m.Annotations) > 0 {
		msg.Annotations = lo.Map(m.Annotations, func(a llm.Annotation, _ int) Annotation {
			return AnnotationFromLLM(a)
		})
	}

	return msg
}

// AnnotationFromLLM creates OpenAI Annotation from unified llm.Annotation.
func AnnotationFromLLM(a llm.Annotation) Annotation {
	annotation := Annotation{
		Type:       a.Type,
		StartIndex: a.StartIndex,
		EndIndex:   a.EndIndex,
	}

	if a.URLCitation != nil {
		annotation.URLCitation = &URLCitation{
			URL:   a.URLCitation.URL,
			Title: a.URLCitation.Title,
		}
	}

	return annotation
}

// MessageContentFromLLM creates OpenAI MessageContent from unified llm.MessageContent.
func MessageContentFromLLM(c llm.MessageContent) MessageContent {
	content := MessageContent{
		Content: c.Content,
	}

	if c.MultipleContent != nil {
		content.MultipleContent = lo.FilterMap(c.MultipleContent, func(p llm.MessageContentPart, _ int) (MessageContentPart, bool) {
			switch p.Type {
			case "compaction", "compaction_summary", "document", "anthropic_raw_block":
				// anthropic_raw_block is an Anthropic-native placeholder. Its raw
				// bytes live in ProviderExtensions and are only hydrated by the
				// Anthropic outbound adapter, so it must never become a Chat part.
				return MessageContentPart{}, false
			case "file":
				// Chat's file payload can represent file_data, file_id, or filename.
				// A Responses-only file_url with none of those fields has no Chat
				// equivalent and must not become an empty file object.
				if p.OpenAIChatFile == nil || (p.OpenAIChatFile.FileData == nil && p.OpenAIChatFile.FileID == nil && p.OpenAIChatFile.Filename == nil) {
					return MessageContentPart{}, false
				}
				return MessageContentPartFromLLM(p), true
			default:
				return MessageContentPartFromLLM(p), true
			}
		})
		if len(content.MultipleContent) == 0 {
			return MessageContent{Content: lo.ToPtr("")}
		}
	}

	return content
}

func messageContentFromLLMForRole(content llm.MessageContent, role string) MessageContent {
	if role != "system" && role != "developer" && role != "assistant" && role != "tool" {
		return MessageContentFromLLM(content)
	}
	if content.Content != nil {
		return MessageContent{Content: content.Content}
	}

	allowedParts := lo.FilterMap(content.MultipleContent, func(part llm.MessageContentPart, _ int) (MessageContentPart, bool) {
		switch part.Type {
		case "text", "input_text":
			if part.Text != nil {
				return MessageContentPart{Type: "text", Text: part.Text}, true
			}
		case "refusal":
			if role == "assistant" && part.OpenAIChatRefusal != nil {
				return MessageContentPart{Type: "refusal", Refusal: part.OpenAIChatRefusal}, true
			}
		}

		return MessageContentPart{}, false
	})
	if len(allowedParts) == 0 {
		return MessageContent{Content: lo.ToPtr("")}
	}

	return MessageContent{MultipleContent: allowedParts}
}

// MessageContentPartFromLLM creates OpenAI MessageContentPart from unified llm.MessageContentPart.
func MessageContentPartFromLLM(p llm.MessageContentPart) MessageContentPart {
	part := MessageContentPart{
		Type: p.Type,
		Text: p.Text,
	}

	if p.ImageURL != nil {
		part.ImageURL = &ImageURL{
			URL:    p.ImageURL.URL,
			Detail: p.ImageURL.Detail,
		}
	}

	if p.VideoURL != nil {
		part.VideoURL = &VideoURL{
			URL: p.VideoURL.URL,
		}
	}

	if p.InputAudio != nil {
		part.InputAudio = &InputAudio{
			Format: p.InputAudio.Format,
			Data:   p.InputAudio.Data,
		}
	}

	if p.OpenAIChatFile != nil {
		part.File = &FileContent{
			FileData: p.OpenAIChatFile.FileData,
			FileID:   p.OpenAIChatFile.FileID,
			Filename: p.OpenAIChatFile.Filename,
		}
	}
	if p.OpenAIChatRefusal != nil {
		part.Refusal = p.OpenAIChatRefusal
	}

	return part
}

// ToolFromLLM creates OpenAI Tool from unified llm.Tool.
func ToolFromLLM(t llm.Tool) Tool {
	result := Tool{
		Type: t.Type,
		Function: Function{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
			Strict:      t.Function.Strict,
		},
	}
	if t.Type == "custom" && t.OpenAIChatCustomTool != nil {
		result.Custom = &CustomTool{
			Name:        t.OpenAIChatCustomTool.Name,
			Description: t.OpenAIChatCustomTool.Description,
			Format:      append(json.RawMessage(nil), t.OpenAIChatCustomTool.Format...),
		}
	}
	return result
}

// ToolCallFromLLM creates OpenAI ToolCall from unified llm.ToolCall.
func ToolCallFromLLM(tc llm.ToolCall) ToolCall {
	toolCall := ToolCall{
		ID:   tc.ID,
		Type: tc.Type,
		Function: FunctionCall{
			Name:      tc.Function.CompositeName(),
			Arguments: tc.Function.Arguments,
		},
		Index: tc.Index,
	}

	if raw, ok := tc.TransformerMetadata[TransformerMetadataKeyGoogleThoughtSignature].(string); ok && raw != "" {
		toolCall.ExtraContent = &ToolCallExtraContent{
			Google: &ToolCallGoogleExtraContent{
				ThoughtSignature: raw,
			},
		}
	}

	if tc.Type == llm.ToolTypeResponsesCustomTool && tc.ResponseCustomToolCall != nil {
		toolCall.Type = "custom"
		toolCall.Function = FunctionCall{}
		toolCall.Custom = &CustomToolCall{
			Name:  tc.ResponseCustomToolCall.Name,
			Input: tc.ResponseCustomToolCall.Input,
		}
		return toolCall
	} else if tc.Type == "custom" && tc.OpenAIChatCustomToolCall != nil {
		toolCall.Custom = &CustomToolCall{
			Name:  tc.OpenAIChatCustomToolCall.Name,
			Input: tc.OpenAIChatCustomToolCall.Input,
			Index: tc.OpenAIChatCustomToolCall.Index,
		}
	}

	return toolCall
}

func responsesCustomToolToOpenAIChatTool(src llm.Tool) llm.Tool {
	converted := src
	converted.Type = "custom"
	converted.OpenAIChatCustomTool = &llm.OpenAIChatCustomTool{
		Name:        src.ResponseCustomTool.Name,
		Description: lo.ToPtr(src.ResponseCustomTool.Description),
		Format:      openAIChatCustomToolFormat(src.ResponseCustomTool.Format),
	}
	return converted
}

func openAIChatCustomToolFormat(src *llm.ResponseCustomToolFormat) json.RawMessage {
	if src == nil {
		return nil
	}

	var raw any
	if src.Type == "grammar" {
		raw = map[string]any{
			"type": "grammar",
			"grammar": map[string]string{
				"syntax":     src.Syntax,
				"definition": src.Definition,
			},
		}
	} else {
		raw = struct {
			Type string `json:"type"`
		}{Type: src.Type}
	}

	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	return encoded
}

// ToLLMResponse converts OpenAI Response to unified llm.Response.
func (r *Response) ToLLMResponse() *llm.Response {
	if r == nil {
		return nil
	}

	resp := &llm.Response{
		ID:                r.ID,
		Object:            r.Object,
		Created:           r.Created,
		Model:             r.Model,
		SystemFingerprint: r.SystemFingerprint,
		ServiceTier:       r.ServiceTier,
	}

	// Convert choices
	resp.Choices = lo.Map(r.Choices, func(c Choice, _ int) llm.Choice {
		return c.ToLLMChoice()
	})

	// Convert usage
	if r.Usage != nil {
		resp.Usage = r.Usage.ToLLMUsage()
	}

	// Convert error
	if r.Error != nil {
		resp.Error = &llm.ResponseError{
			StatusCode: r.Error.StatusCode,
			Detail:     r.Error.Detail,
		}
	}

	// Store citations in TransformerMetadata if present
	if len(r.Citations) > 0 {
		if resp.TransformerMetadata == nil {
			resp.TransformerMetadata = make(map[string]any)
		}

		resp.TransformerMetadata[TransformerMetadataKeyCitations] = r.Citations
	}

	return resp
}

// ToLLMChoice converts OpenAI Choice to unified llm.Choice.
func (c Choice) ToLLMChoice() llm.Choice {
	choice := llm.Choice{
		Index:        c.Index,
		FinishReason: c.FinishReason,
	}

	if c.Message != nil {
		msg := c.Message.ToLLMMessage()
		choice.Message = &msg
	}

	if c.Delta != nil {
		delta := c.Delta.ToLLMMessage()
		choice.Delta = &delta
	}

	if c.Logprobs != nil {
		choice.Logprobs = &llm.LogprobsContent{
			Content: lo.Map(c.Logprobs.Content, func(t TokenLogprob, _ int) llm.TokenLogprob {
				return llm.TokenLogprob{
					Token:   t.Token,
					Logprob: t.Logprob,
					Bytes:   t.Bytes,
					TopLogprobs: lo.Map(t.TopLogprobs, func(tl TopLogprob, _ int) llm.TopLogprob {
						return llm.TopLogprob{
							Token:   tl.Token,
							Logprob: tl.Logprob,
							Bytes:   tl.Bytes,
						}
					}),
				}
			}),
		}
	}

	return choice
}
