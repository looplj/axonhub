package openai

import (
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
)

// RequestFromLLM creates OpenAI Request from unified llm.Request.
func RequestFromLLM(r *llm.Request) *Request {
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
		ServiceTier:         r.ServiceTier,
		Stream:              r.Stream,
		ParallelToolCalls:   r.ParallelToolCalls,
		Verbosity:           r.Verbosity,
	}

	// Convert messages
	req.Messages = lo.Map(r.Messages, func(m llm.Message, _ int) Message {
		return MessageFromLLM(m)
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

	// Convert Tools
	req.Tools = lo.Map(r.Tools, func(t llm.Tool, _ int) Tool {
		return ToolFromLLM(t)
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
	}

	// Convert ResponseFormat
	if r.ResponseFormat != nil {
		req.ResponseFormat = &ResponseFormat{
			Type: r.ResponseFormat.Type,
		}
	}

	return req
}

// MessageFromLLM creates OpenAI Message from unified llm.Message.
func MessageFromLLM(m llm.Message) Message {
	msg := Message{
		Role:             m.Role,
		Name:             m.Name,
		Refusal:          m.Refusal,
		ToolCallID:       m.ToolCallID,
		ReasoningContent: m.ReasoningContent,
	}

	// Convert Content
	msg.Content = MessageContentFromLLM(m.Content)

	// Convert ToolCalls
	if m.ToolCalls != nil {
		msg.ToolCalls = lo.Map(m.ToolCalls, func(tc llm.ToolCall, _ int) ToolCall {
			return ToolCallFromLLM(tc)
		})
	}

	return msg
}

// MessageContentFromLLM creates OpenAI MessageContent from unified llm.MessageContent.
func MessageContentFromLLM(c llm.MessageContent) MessageContent {
	content := MessageContent{
		Content: c.Content,
	}

	if c.MultipleContent != nil {
		content.MultipleContent = lo.Map(c.MultipleContent, func(p llm.MessageContentPart, _ int) MessageContentPart {
			return MessageContentPartFromLLM(p)
		})
	}

	return content
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

	if p.Audio != nil {
		part.Audio = &Audio{
			Format: p.Audio.Format,
			Data:   p.Audio.Data,
		}
	}

	return part
}

// ToolFromLLM creates OpenAI Tool from unified llm.Tool.
func ToolFromLLM(t llm.Tool) Tool {
	return Tool{
		Type: t.Type,
		Function: Function{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
			Strict:      t.Function.Strict,
		},
	}
}

// ToolCallFromLLM creates OpenAI ToolCall from unified llm.ToolCall.
func ToolCallFromLLM(tc llm.ToolCall) ToolCall {
	return ToolCall{
		ID:   tc.ID,
		Type: tc.Type,
		Function: FunctionCall{
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		},
		Index: tc.Index,
	}
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
