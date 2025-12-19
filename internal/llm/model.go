package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

var (
	DoneStreamEvent = httpclient.StreamEvent{
		Data: []byte("[DONE]"),
	}

	DoneResponse = &Response{
		Object: "[DONE]",
	}
)

// Request is the unified llm request model for AxonHub, to keep compatibility with major app and framework.
// It choose to base on the OpenAI chat completion request, but add some extra fields to support more features.
type Request struct {
	// Messages is a list of messages to send to the llm model.
	Messages []Message `json:"messages" validator:"required,min=1"`

	// Model is the model ID used to generate the response.
	Model string `json:"model" validator:"required"`

	// Number between -2.0 and 2.0. Positive values penalize new tokens based on
	// their existing frequency in the text so far, decreasing the model's likelihood
	// to repeat the same line verbatim.
	//
	// See [OpenAI's
	// documentation](https://platform.openai.com/docs/api-reference/parameter-details)
	// for more information.
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`

	// Whether to return log probabilities of the output tokens or not. If true,
	// returns the log probabilities of each output token returned in the `content` of
	// `message`.
	Logprobs *bool `json:"logprobs,omitempty"`

	// An upper bound for the number of tokens that can be generated for a completion,
	// including visible output tokens and
	// [reasoning tokens](https://platform.openai.com/docs/guides/reasoning).
	MaxCompletionTokens *int64 `json:"max_completion_tokens,omitempty"`

	// The maximum number of [tokens](/tokenizer) that can be generated in the chat
	// completion. This value can be used to control
	// [costs](https://openai.com/api/pricing/) for text generated via API.
	//
	// This value is now deprecated in favor of `max_completion_tokens`, and is not
	// compatible with
	// [o-series models](https://platform.openai.com/docs/guides/reasoning).
	MaxTokens *int64 `json:"max_tokens,omitempty"`

	// How many chat completion choices to generate for each input message. Note that
	// you will be charged based on the number of generated tokens across all of the
	// choices. Keep `n` as `1` to minimize costs.
	// NOTE: Not supported, always 1.
	// N *int64 `json:"n,omitempty"`

	// Number between -2.0 and 2.0. Positive values penalize new tokens based on
	// whether they appear in the text so far, increasing the model's likelihood to
	// talk about new topics.
	PresencePenalty *float64 `json:"presence_penalty,omitempty"`

	// This feature is in Beta. If specified, our system will make a best effort to
	// sample deterministically, such that repeated requests with the same `seed` and
	// parameters should return the same result. Determinism is not guaranteed, and you
	// should refer to the `system_fingerprint` response parameter to monitor changes
	// in the backend.
	Seed *int64 `json:"seed,omitempty"`

	// Whether or not to store the output of this chat completion request for use in
	// our [model distillation](https://platform.openai.com/docs/guides/distillation)
	// or [evals](https://platform.openai.com/docs/guides/evals) products.
	//
	// Supports text and image inputs. Note: image inputs over 10MB will be dropped.
	Store *bool `json:"store,omitzero"`

	// What sampling temperature to use, between 0 and 2. Higher values like 0.8 will
	// make the output more random, while lower values like 0.2 will make it more
	// focused and deterministic. We generally recommend altering this or `top_p` but
	// not both.
	Temperature *float64 `json:"temperature,omitempty"`

	// An integer between 0 and 20 specifying the number of most likely tokens to
	// return at each token position, each with an associated log probability.
	// `logprobs` must be set to `true` if this parameter is used.
	TopLogprobs *int64 `json:"top_logprobs,omitzero"`

	// An alternative to sampling with temperature, called nucleus sampling, where the
	// model considers the results of the tokens with top_p probability mass. So 0.1
	// means only the tokens comprising the top 10% probability mass are considered.
	//
	// We generally recommend altering this or `temperature` but not both.
	TopP *float64 `json:"top_p,omitempty"`

	// Used by OpenAI to cache responses for similar requests to optimize your cache
	// hit rates. Replaces the `user` field.
	// [Learn more](https://platform.openai.com/docs/guides/prompt-caching).
	PromptCacheKey *bool `json:"prompt_cache_key,omitzero"`

	// A stable identifier used to help detect users of your application that may be
	// violating OpenAI's usage policies. The IDs should be a string that uniquely
	// identifies each user. We recommend hashing their username or email address, in
	// order to avoid sending us any identifying information.
	// [Learn more](https://platform.openai.com/docs/guides/safety-best-practices#safety-identifiers).
	SafetyIdentifier *string `json:"safety_identifier,omitzero"`

	// This field is being replaced by `safety_identifier` and `prompt_cache_key`. Use
	// `prompt_cache_key` instead to maintain caching optimizations. A stable
	// identifier for your end-users. Used to boost cache hit rates by better bucketing
	// similar requests and to help OpenAI detect and prevent abuse.
	// [Learn more](https://platform.openai.com/docs/guides/safety-best-practices#safety-identifiers).
	User *string `json:"user,omitempty"`

	// Parameters for audio output. Required when audio output is requested with
	// `modalities: ["audio"]`.
	// [Learn more](https://platform.openai.com/docs/guides/audio).
	// TODO
	// Audio ChatCompletionAudioParam `json:"audio,omitzero"`

	// Modify the likelihood of specified tokens appearing in the completion.
	//
	// Accepts a JSON object that maps tokens (specified by their token ID in the
	// tokenizer) to an associated bias value from -100 to 100. Mathematically, the
	// bias is added to the logits generated by the model prior to sampling. The exact
	// effect will vary per model, but values between -1 and 1 should decrease or
	// increase likelihood of selection; values like -100 or 100 should result in a ban
	// or exclusive selection of the relevant token.
	LogitBias map[string]int64 `json:"logit_bias,omitempty"`

	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata map[string]string `json:"metadata,omitempty"`

	// Output types that you would like the model to generate. Most models are capable
	// of generating text, which is the default:
	//
	// `["text"]`
	// To generate audio, you can use:
	// `["text", "audio"]`
	// To generate image, you can use:
	// `["text", "image"]`
	// Please note that not all models support audio and image generation.
	// Any of "text", "audio", "image".
	Modalities []string `json:"modalities,omitempty"`

	// Controls effort on reasoning for reasoning models. It can be set to "low", "medium", or "high".
	ReasoningEffort string `json:"reasoning_effort,omitempty"`

	// Reasoning budget for reasoning models.
	// Help fields， will not be sent to the llm service.
	ReasoningBudget *int64 `json:"-"`

	// Specifies the processing type used for serving the request.
	ServiceTier *string `json:"service_tier,omitempty"`

	// Not supported with latest reasoning models `o3` and `o4-mini`.
	//
	// Up to 4 sequences where the API will stop generating further tokens. The
	// returned text will not contain the stop sequence.
	Stop *Stop `json:"stop,omitempty"` // string or []string

	Stream        *bool          `json:"stream,omitempty"`
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`

	// Static predicted output content, such as the content of a text file that is
	// being regenerated.
	// TODO
	// Prediction ChatCompletionPredictionContentParam `json:"prediction,omitempty"`

	// Whether to enable
	// [parallel function calling](https://platform.openai.com/docs/guides/function-calling#configuring-parallel-function-calling)
	// during tool use.
	ParallelToolCalls *bool       `json:"parallel_tool_calls,omitempty"`
	Tools             []Tool      `json:"tools,omitempty"`
	ToolChoice        *ToolChoice `json:"tool_choice,omitempty"`

	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`

	// Constrains the verbosity of the model's response. Lower values will result in
	// more concise responses, while higher values will result in more verbose
	// responses. Currently supported values are `low`, `medium`, and `high`.
	//
	// Any of "low", "medium", "high".
	Verbosity *string `json:"verbosity,omitempty"`

	// Help fields， will not be sent to the llm service.

	// ExtraBody is helpful to extend the request for different providers.
	// It will not be sent to the OpenAI server.
	ExtraBody json.RawMessage `json:"extra_body,omitempty"`

	// RawRequest is the raw request from the client.
	RawRequest *httpclient.Request `json:"-"`

	// RawAPIFormat is the original format of the request.
	// e.g. the request from the chat/completions endpoint is in the openai/chat_completion format.
	RawAPIFormat APIFormat `json:"-"`

	// TransformerMetadata stores transformer-specific metadata for preserving format during transformations.
	// This is a help field and will not be sent to the llm service.
	// Keys used:
	// - "include": []string - additional output data to include in the model response
	// - "max_tool_calls": *int64 - maximum number of total calls to built-in tools
	// - "prompt_cache_key": *string - string key used by OpenAI to cache responses
	// - "prompt_cache_retention": *string - retention policy for the prompt cache ("in-memory", "24h")
	// - "truncation": *string - truncation strategy ("auto", "disabled")
	// - "include_obfuscation": *bool - whether to enable stream obfuscation (Responses API specific)
	TransformerMetadata map[string]any `json:"-"`
}

func (r *Request) ClearHelpFields() {
	for i, msg := range r.Messages {
		msg.ClearHelpFields()
		r.Messages[i] = msg
	}

	r.ExtraBody = nil

	// If tools are present, keep only function tools
	tools := lo.Filter(r.Tools, func(tool Tool, _ int) bool {
		return tool.Type == "function"
	})
	r.Tools = tools
}

func (r *Request) IsImageGenerationRequest() bool {
	return len(r.Modalities) > 0 && slices.Contains(r.Modalities, "image")
}

type StreamOptions struct {
	// If set, an additional chunk will be streamed before the data: [DONE] message.
	// The usage field on this chunk shows the token usage statistics for the entire request,
	// and the choices field will always be an empty array.
	// All other chunks will also include a usage field, but with a null value.
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type Stop struct {
	Stop         *string
	MultipleStop []string
}

func (s Stop) MarshalJSON() ([]byte, error) {
	if s.Stop != nil {
		return json.Marshal(s.Stop)
	}

	if len(s.MultipleStop) > 0 {
		return json.Marshal(s.MultipleStop)
	}

	return []byte("[]"), nil
}

func (s *Stop) UnmarshalJSON(data []byte) error {
	var str string

	err := json.Unmarshal(data, &str)
	if err == nil {
		s.Stop = &str
		return nil
	}

	var strs []string

	err = json.Unmarshal(data, &strs)
	if err == nil {
		s.MultipleStop = strs
		return nil
	}

	return errors.New("invalid stop type")
}

// Message represents a message in the conversation.
type Message struct {
	// user, assistant, system, tool, developer
	Role string `json:"role,omitempty"`
	// Content of the message.
	// string or []ContentPart, be careful about the omitzero tag, it required.
	// Some framework may depended on the behavior, we should not response the field if not present.
	Content MessageContent `json:"content,omitzero"`
	Name    *string        `json:"name,omitempty"`

	// The refusal message generated by the model.
	Refusal string `json:"refusal,omitempty"`

	// For tool call response.

	// The index of the message that the tool call is associated with.
	// Is is a help field, will not be sent to the llm service.
	MessageIndex *int    `json:"-"`
	ToolCallID   *string `json:"tool_call_id,omitempty"`
	// The name of the tool call.
	// Is is a help field, will not be sent to the llm service.
	ToolCallName *string `json:"-"`
	// This field is a help field, will not be sent to the llm service.
	ToolCallIsError *bool      `json:"-"`
	ToolCalls       []ToolCall `json:"tool_calls,omitempty"`

	// This property is used for the "reasoning" feature supported by deepseek-reasoner
	// the doc from deepseek:
	// - https://api-docs.deepseek.com/api/create-chat-completion#responses
	ReasoningContent *string `json:"reasoning_content,omitempty"`

	// Help field, will not be sent to the llm service, to adapt the anthropic think signature.
	// https://platform.claude.com/docs/en/build-with-claude/extended-thinking
	ReasoningSignature *string `json:"reasoning_signature,omitempty"`

	// Help field, will not be sent to the llm service, to adapt the anthropic think signature.
	// https://platform.claude.com/docs/en/build-with-claude/extended-thinking
	RedactedReasoningContent *string `json:"redacted_reasoning_content,omitempty"`

	// CacheControl is used for provider-specific cache control (e.g., Anthropic).
	// This field is not serialized in JSON.
	CacheControl *CacheControl `json:"-"`
}

func (m *Message) ClearHelpFields() {
	m.ReasoningContent = nil
	m.ReasoningSignature = nil
	m.RedactedReasoningContent = nil
}

type MessageContent struct {
	Content         *string              `json:"content,omitempty"`
	MultipleContent []MessageContentPart `json:"multiple_content,omitempty"`
}

func (c MessageContent) MarshalJSON() ([]byte, error) {
	if len(c.MultipleContent) > 0 {
		if len(c.MultipleContent) == 1 && c.MultipleContent[0].Type == "text" {
			return json.Marshal(c.MultipleContent[0].Text)
		}

		return json.Marshal(c.MultipleContent)
	}

	return json.Marshal(c.Content)
}

func (c *MessageContent) UnmarshalJSON(data []byte) error {
	var str string

	err := json.Unmarshal(data, &str)
	if err == nil {
		c.Content = &str
		return nil
	}

	var parts []MessageContentPart

	err = json.Unmarshal(data, &parts)
	if err == nil {
		c.MultipleContent = parts
		return nil
	}

	return errors.New("invalid content type")
}

// MessageContentPart represents different types of content (text, image, etc.)
type MessageContentPart struct {
	// Type is the type of the content part.
	// e.g. "text", "image_url"
	Type string `json:"type"`
	// Text is the text content, required when type is "text"
	Text *string `json:"text,omitempty"`

	// ImageURL is the image URL content, required when type is "image_url"
	ImageURL *ImageURL `json:"image_url,omitempty"`

	// Audio is the audio content, required when type is "input_audio"
	Audio *Audio `json:"audio,omitempty"`

	// CacheControl is used for provider-specific cache control (e.g., Anthropic).
	// This field is not serialized in JSON.
	CacheControl *CacheControl `json:"-"`

	// TransformerMetadata stores transformer-specific metadata for preserving format during transformations.
	// This is a help field and will not be sent to the llm service.
	TransformerMetadata map[string]any `json:"-"`
}

// ImageURL represents an image URL with optional detail level.
type ImageURL struct {
	// URL is the URL of the image.
	URL string `json:"url"`

	// Specifies the detail level of the image. Learn more in the
	// [Vision guide](https://platform.openai.com/docs/guides/vision#low-or-high-fidelity-image-understanding).
	//
	// Any of "auto", "low", "high".
	Detail *string `json:"detail,omitempty"`
}

type Audio struct {
	// The format of the encoded audio data. Currently supports "wav" and "mp3".
	//
	// Any of "wav", "mp3".
	Format string `json:"format"`

	// Base64 encoded audio data.
	Data string `json:"data"`
}

// ResponseFormat specifies the format of the response.
type ResponseFormat struct {
	// Any of "json_schema", "json_object", "text".
	Type string `json:"type"`
	// TODO: Schema
	// json.RawMessage `json:"schema,omitempty"`
}

// Response is the unified response model.
// To reduce the work of converting the response, we use the OpenAI response format.
// And other llm provider should convert the response to this format.
// NOTE: the OpenAI stream and non-stream response reuse same struct.
type Response struct {
	ID string `json:"id"`

	// A list of chat completion choices. Can be more than one if `n` is greater
	// than 1.
	Choices []Choice `json:"choices"`

	// Object is the type of the response.
	// e.g. "chat.completion", "chat.completion.chunk"
	Object string `json:"object"`

	// Created is the timestamp of when the response was created.
	Created int64 `json:"created"`

	// Model is the model used to generate the response.
	Model string `json:"model"`

	// An optional field that will only be present when you set stream_options: {"include_usage": true} in your request.
	// When present, it contains a null value except for the last chunk which contains the token usage statistics
	// for the entire request.
	Usage *Usage `json:"usage,omitempty"`

	// This fingerprint represents the backend configuration that the model runs with.
	//
	// Can be used in conjunction with the `seed` request parameter to understand when
	// backend changes have been made that might impact determinism.
	SystemFingerprint string `json:"system_fingerprint,omitempty"`

	// ServiceTier is the service tier of the response.
	// e.g. "free", "standard", "premium"
	ServiceTier string `json:"service_tier,omitempty"`

	// Error is the error information, will present if request to llm service failed with status >= 400.
	Error *ResponseError `json:"error,omitempty"`

	// ProviderData stores provider-specific response payloads that do not map to the chat completion schema.
	// This field is ignored when serializing to JSON and is only used internally by transformers (e.g., embeddings).
	ProviderData any `json:"-"`

	// TransformerMetadata stores metadata from transformers that process the response.
	// This field is ignored when serializing to JSON and is only used internally by transformers.
	TransformerMetadata map[string]any `json:"-"`
}

func (r *Response) ClearHelpFields() {
	for i, choice := range r.Choices {
		if choice.Message != nil {
			choice.Message.ClearHelpFields()
		}

		if choice.Delta != nil {
			choice.Delta.ClearHelpFields()
		}

		r.Choices[i] = choice
	}
}

// Choice represents a choice in the response.
// Choice represents a choice in the response.
type Choice struct {
	// Index is the index of the choice in the list of choices.
	Index int `json:"index"`

	// Message is the message content, will present if stream is false
	Message *Message `json:"message,omitempty"`

	// Delta is the stream event content, will present if stream is true
	Delta *Message `json:"delta,omitempty"`

	// FinishReason is the reason the model stopped generating tokens.
	// e.g. "stop", "length", "content_filter", "function_call", "tool_calls"
	FinishReason *string `json:"finish_reason,omitempty"`

	Logprobs *LogprobsContent `json:"logprobs,omitempty"`

	// TransformerMetadata stores metadata from transformers that process the response.
	// This field is ignored when serializing to JSON and is only used internally by transformers.
	TransformerMetadata map[string]any `json:"-"`
}

// LogprobsContent represents logprobs information.
type LogprobsContent struct {
	Content []TokenLogprob `json:"content"`
}

// TokenLogprob represents logprob for a token.
type TokenLogprob struct {
	Token       string       `json:"token"`
	Logprob     float64      `json:"logprob"`
	Bytes       []int        `json:"bytes,omitempty"`
	TopLogprobs []TopLogprob `json:"top_logprobs,omitempty"`
}

// TopLogprob represents top alternative tokens.
type TopLogprob struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
	Bytes   []int   `json:"bytes,omitempty"`
}

type ResponseMeta struct {
	ID    string `json:"id"`
	Usage *Usage `json:"usage"`
}

// Usage Represents the total token usage per request to OpenAI.
type Usage struct {
	PromptTokens            int64                    `json:"prompt_tokens"`
	CompletionTokens        int64                    `json:"completion_tokens"`
	TotalTokens             int64                    `json:"total_tokens"`
	PromptTokensDetails     *PromptTokensDetails     `json:"prompt_tokens_details"`
	CompletionTokensDetails *CompletionTokensDetails `json:"completion_tokens_details"`

	// Output only. A detailed breakdown of the token count for each modality in the prompt.
	PromptModalityTokenDetails []ModalityTokenCount `json:"-"`
	// Output only. A detailed breakdown of the token count for each modality in the candidates.
	CompletionModalityTokenDetails []ModalityTokenCount `json:"-"`
}

func (u *Usage) GetCompletionTokens() *int64 {
	if u == nil {
		return nil
	}

	return &u.CompletionTokens
}

func (u *Usage) GetPromptTokens() *int64 {
	if u == nil {
		return nil
	}

	return &u.PromptTokens
}

// CompletionTokensDetails Breakdown of tokens used in a completion.
type CompletionTokensDetails struct {
	AudioTokens              int64 `json:"audio_tokens"`
	ReasoningTokens          int64 `json:"reasoning_tokens"`
	AcceptedPredictionTokens int64 `json:"accepted_prediction_tokens"`
	RejectedPredictionTokens int64 `json:"rejected_prediction_tokens"`
}

// PromptTokensDetails Breakdown of tokens used in the prompt.
type PromptTokensDetails struct {
	AudioTokens  int64 `json:"audio_tokens"`
	CachedTokens int64 `json:"cached_tokens"`
	// hidden field, used for internal calculation.
	WriteCachedTokens int64 `json:"-"`
}

// ResponseError represents an error response.
type ResponseError struct {
	StatusCode int         `json:"-"`
	Detail     ErrorDetail `json:"error"`
}

func (e ResponseError) Error() string {
	sb := strings.Builder{}
	if e.StatusCode != 0 {
		sb.WriteString(fmt.Sprintf("Request failed: %s, ", http.StatusText(e.StatusCode)))
	}

	if e.Detail.Message != "" {
		sb.WriteString("error: ")
		sb.WriteString(e.Detail.Message)
	}

	if e.Detail.Code != "" {
		sb.WriteString(", code: ")
		sb.WriteString(e.Detail.Code)
	}

	if e.Detail.Type != "" {
		sb.WriteString(", type: ")
		sb.WriteString(e.Detail.Type)
	}

	if e.Detail.RequestID != "" {
		sb.WriteString(", request_id: ")
		sb.WriteString(e.Detail.RequestID)
	}

	return sb.String()
}

// ErrorDetail represents error details.
type ErrorDetail struct {
	Code      string `json:"code,omitempty"`
	Message   string `json:"message"`
	Type      string `json:"type"`
	Param     string `json:"param,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// ModalityTokenCount Represents token counting info for a single modality.
type ModalityTokenCount struct {
	Modality string `json:"modality,omitempty"`
	// Number of tokens.
	TokenCount int64 `json:"token_count,omitempty"`
}
