package openai

import (
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
)

// ToLLMToolCall converts OpenAI ToolCall to unified llm.ToolCall.
func (tc ToolCall) ToLLMToolCall() llm.ToolCall {
	return llm.ToolCall{
		ID:   tc.ID,
		Type: tc.Type,
		Function: llm.FunctionCall{
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		},
		Index: tc.Index,
	}
}

// ToLLMRequest converts OpenAI Request to unified llm.Request.
func (r *Request) ToLLMRequest() *llm.Request {
	if r == nil {
		return nil
	}

	req := &llm.Request{
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
		ServiceTier:         r.ServiceTier,
		Stream:              r.Stream,
		ParallelToolCalls:   r.ParallelToolCalls,
		Verbosity:           r.Verbosity,
	}

	// Convert messages
	req.Messages = lo.Map(r.Messages, func(m Message, _ int) llm.Message {
		return m.ToLLMMessage()
	})

	// Convert Stop
	if r.Stop != nil {
		req.Stop = &llm.Stop{
			Stop:         r.Stop.Stop,
			MultipleStop: r.Stop.MultipleStop,
		}
	}

	// Convert StreamOptions
	if r.StreamOptions != nil {
		req.StreamOptions = &llm.StreamOptions{
			IncludeUsage: r.StreamOptions.IncludeUsage,
		}
	}

	// Convert Tools
	req.Tools = lo.Map(r.Tools, func(t Tool, _ int) llm.Tool {
		return t.ToLLMTool()
	})

	// Convert ToolChoice
	if r.ToolChoice != nil {
		req.ToolChoice = &llm.ToolChoice{
			ToolChoice: r.ToolChoice.ToolChoice,
		}
		if r.ToolChoice.NamedToolChoice != nil {
			req.ToolChoice.NamedToolChoice = &llm.NamedToolChoice{
				Type: r.ToolChoice.NamedToolChoice.Type,
				Function: llm.ToolFunction{
					Name: r.ToolChoice.NamedToolChoice.Function.Name,
				},
			}
		}
	}

	// Convert ResponseFormat
	if r.ResponseFormat != nil {
		req.ResponseFormat = &llm.ResponseFormat{
			Type: r.ResponseFormat.Type,
		}
	}

	return req
}

// ToLLMMessage converts OpenAI Message to unified llm.Message.
func (m Message) ToLLMMessage() llm.Message {
	msg := llm.Message{
		Role:             m.Role,
		Name:             m.Name,
		Refusal:          m.Refusal,
		ToolCallID:       m.ToolCallID,
		ReasoningContent: m.ReasoningContent,
	}

	// Convert Content
	msg.Content = m.Content.ToLLMContent()

	// Convert ToolCalls
	if m.ToolCalls != nil {
		msg.ToolCalls = lo.Map(m.ToolCalls, func(tc ToolCall, _ int) llm.ToolCall {
			return tc.ToLLMToolCall()
		})
	}

	return msg
}

// ToLLMContent converts OpenAI MessageContent to unified llm.MessageContent.
func (c MessageContent) ToLLMContent() llm.MessageContent {
	content := llm.MessageContent{
		Content: c.Content,
	}

	if c.MultipleContent != nil {
		content.MultipleContent = lo.Map(c.MultipleContent, func(p MessageContentPart, _ int) llm.MessageContentPart {
			return p.ToLLMPart()
		})
	}

	return content
}

// ToLLMPart converts OpenAI MessageContentPart to unified llm.MessageContentPart.
func (p MessageContentPart) ToLLMPart() llm.MessageContentPart {
	part := llm.MessageContentPart{
		Type: p.Type,
		Text: p.Text,
	}

	if p.ImageURL != nil {
		part.ImageURL = &llm.ImageURL{
			URL:    p.ImageURL.URL,
			Detail: p.ImageURL.Detail,
		}
	}

	if p.Audio != nil {
		part.Audio = &llm.Audio{
			Format: p.Audio.Format,
			Data:   p.Audio.Data,
		}
	}

	return part
}

// ResponseFromLLM creates OpenAI Response from unified llm.Response.
func ResponseFromLLM(r *llm.Response) *Response {
	if r == nil {
		return nil
	}

	resp := &Response{
		ID:                r.ID,
		Object:            r.Object,
		Created:           r.Created,
		Model:             r.Model,
		SystemFingerprint: r.SystemFingerprint,
		ServiceTier:       r.ServiceTier,
	}

	// Convert choices
	resp.Choices = lo.Map(r.Choices, func(c llm.Choice, _ int) Choice {
		return ChoiceFromLLM(c)
	})

	// Convert usage
	if r.Usage != nil {
		resp.Usage = UsageFromLLM(r.Usage)
	}

	// Convert error
	if r.Error != nil {
		resp.Error = &ResponseError{
			StatusCode: r.Error.StatusCode,
			Detail:     r.Error.Detail,
		}
	}

	return resp
}

// ChoiceFromLLM creates OpenAI Choice from unified llm.Choice.
func ChoiceFromLLM(c llm.Choice) Choice {
	choice := Choice{
		Index:        c.Index,
		FinishReason: c.FinishReason,
	}

	if c.Message != nil {
		msg := MessageFromLLM(*c.Message)
		choice.Message = &msg
	}

	if c.Delta != nil {
		delta := MessageFromLLM(*c.Delta)
		choice.Delta = &delta
	}

	if c.Logprobs != nil {
		choice.Logprobs = &Logprobs{
			Content: lo.Map(c.Logprobs.Content, func(t llm.TokenLogprob, _ int) TokenLogprob {
				return TokenLogprob{
					Token:   t.Token,
					Logprob: t.Logprob,
					Bytes:   t.Bytes,
					TopLogprobs: lo.Map(t.TopLogprobs, func(tl llm.TopLogprob, _ int) TopLogprob {
						return TopLogprob{
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
