package orchestrator

import (
	"encoding/json"
	"net/url"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func CloneRequestForOutboundAttempt(req *llm.Request) *llm.Request {
	if req == nil {
		return nil
	}

	cloned := *req
	cloned.Messages = cloneMessages(req.Messages)
	cloned.LogitBias = cloneMap(req.LogitBias)
	cloned.Metadata = cloneMap(req.Metadata)
	cloned.Modalities = cloneSlice(req.Modalities)
	cloned.Stop = cloneStop(req.Stop)
	cloned.StreamOptions = clonePtr(req.StreamOptions)
	cloned.Tools = cloneTools(req.Tools)
	cloned.ToolChoice = cloneToolChoice(req.ToolChoice)
	cloned.ResponseFormat = cloneResponseFormat(req.ResponseFormat)
	cloned.ExtraBody = cloneRawMessage(req.ExtraBody)
	cloned.Embedding = cloneEmbeddingRequest(req.Embedding)
	cloned.Rerank = cloneRerankRequest(req.Rerank)
	cloned.Image = cloneImageRequest(req.Image)
	cloned.Video = cloneVideoRequest(req.Video)
	cloned.Compact = cloneCompactRequest(req.Compact)
	cloned.Completion = cloneCompletionRequest(req.Completion)
	cloned.RawRequest = cloneHTTPClientRequest(req.RawRequest)
	cloned.TransformOptions = cloneTransformOptions(req.TransformOptions)
	cloned.TransformerMetadata = cloneAnyMap(req.TransformerMetadata)

	return &cloned
}

func cloneMessages(messages []llm.Message) []llm.Message {
	if len(messages) == 0 {
		return nil
	}

	cloned := make([]llm.Message, len(messages))
	for i, msg := range messages {
		cloned[i] = cloneMessage(msg)
	}

	return cloned
}

func cloneMessage(msg llm.Message) llm.Message {
	msg.Content = cloneMessageContent(msg.Content)
	msg.Name = clonePtr(msg.Name)
	msg.MessageIndex = clonePtr(msg.MessageIndex)
	msg.ToolCallID = clonePtr(msg.ToolCallID)
	msg.ToolCallName = clonePtr(msg.ToolCallName)
	msg.ToolCallIsError = clonePtr(msg.ToolCallIsError)
	msg.ToolCalls = cloneToolCalls(msg.ToolCalls)
	msg.ReasoningContent = clonePtr(msg.ReasoningContent)
	msg.Reasoning = clonePtr(msg.Reasoning)
	msg.ReasoningSignature = clonePtr(msg.ReasoningSignature)
	msg.RedactedReasoningContent = clonePtr(msg.RedactedReasoningContent)
	msg.CacheControl = clonePtr(msg.CacheControl)
	msg.Annotations = cloneAnnotations(msg.Annotations)
	msg.Audio = clonePtr(msg.Audio)

	return msg
}

func cloneMessageContent(content llm.MessageContent) llm.MessageContent {
	return llm.MessageContent{
		Content:         clonePtr(content.Content),
		MultipleContent: cloneMessageContentParts(content.MultipleContent),
	}
}

func cloneMessageContentParts(parts []llm.MessageContentPart) []llm.MessageContentPart {
	if len(parts) == 0 {
		return nil
	}

	cloned := make([]llm.MessageContentPart, len(parts))
	for i, part := range parts {
		cloned[i] = part
		cloned[i].Text = clonePtr(part.Text)
		cloned[i].ImageURL = clonePtr(part.ImageURL)
		cloned[i].VideoURL = clonePtr(part.VideoURL)
		cloned[i].Document = clonePtr(part.Document)
		cloned[i].InputAudio = clonePtr(part.InputAudio)
		cloned[i].Compact = clonePtr(part.Compact)
		cloned[i].CacheControl = clonePtr(part.CacheControl)
		cloned[i].TransformerMetadata = cloneAnyMap(part.TransformerMetadata)
	}

	return cloned
}

func cloneAnnotations(annotations []llm.Annotation) []llm.Annotation {
	if len(annotations) == 0 {
		return nil
	}

	cloned := make([]llm.Annotation, len(annotations))
	for i, annotation := range annotations {
		cloned[i] = annotation
		cloned[i].URLCitation = clonePtr(annotation.URLCitation)
	}

	return cloned
}

func cloneTools(tools []llm.Tool) []llm.Tool {
	if len(tools) == 0 {
		return nil
	}

	cloned := make([]llm.Tool, len(tools))
	for i, tool := range tools {
		cloned[i] = tool
		cloned[i].Function.Parameters = cloneRawMessage(tool.Function.Parameters)
		cloned[i].Function.ParametersJsonSchema = cloneRawMessage(tool.Function.ParametersJsonSchema)
		cloned[i].ImageGeneration = cloneImageGeneration(tool.ImageGeneration)
		cloned[i].WebSearch = clonePtr(tool.WebSearch)
		cloned[i].Google = cloneGoogleTools(tool.Google)
		cloned[i].ResponseCustomTool = cloneResponseCustomTool(tool.ResponseCustomTool)
		cloned[i].CacheControl = clonePtr(tool.CacheControl)
	}

	return cloned
}

func cloneImageGeneration(src *llm.ImageGeneration) *llm.ImageGeneration {
	if src == nil {
		return nil
	}

	cloned := *src
	cloned.InputImageMask = cloneAnyMap(src.InputImageMask)
	cloned.OutputCompression = clonePtr(src.OutputCompression)
	cloned.PartialImages = clonePtr(src.PartialImages)
	cloned.N = clonePtr(src.N)

	return &cloned
}

func cloneGoogleTools(src *llm.GoogleTools) *llm.GoogleTools {
	if src == nil {
		return nil
	}

	cloned := *src
	cloned.Search = clonePtr(src.Search)
	cloned.CodeExecution = clonePtr(src.CodeExecution)
	cloned.UrlContext = clonePtr(src.UrlContext)

	return &cloned
}

func cloneResponseCustomTool(src *llm.ResponseCustomTool) *llm.ResponseCustomTool {
	if src == nil {
		return nil
	}

	cloned := *src
	cloned.Format = clonePtr(src.Format)

	return &cloned
}

func cloneToolCalls(calls []llm.ToolCall) []llm.ToolCall {
	if len(calls) == 0 {
		return nil
	}

	cloned := make([]llm.ToolCall, len(calls))
	for i, call := range calls {
		cloned[i] = call
		cloned[i].ResponseCustomToolCall = clonePtr(call.ResponseCustomToolCall)
		cloned[i].CacheControl = clonePtr(call.CacheControl)
		cloned[i].TransformerMetadata = cloneAnyMap(call.TransformerMetadata)
	}

	return cloned
}

func cloneToolChoice(src *llm.ToolChoice) *llm.ToolChoice {
	if src == nil {
		return nil
	}

	cloned := *src
	cloned.ToolChoice = clonePtr(src.ToolChoice)
	cloned.NamedToolChoice = clonePtr(src.NamedToolChoice)

	return &cloned
}

func cloneResponseFormat(src *llm.ResponseFormat) *llm.ResponseFormat {
	if src == nil {
		return nil
	}

	cloned := *src
	cloned.JSONSchema = cloneRawMessage(src.JSONSchema)

	return &cloned
}

func cloneStop(src *llm.Stop) *llm.Stop {
	if src == nil {
		return nil
	}

	cloned := *src
	cloned.Stop = clonePtr(src.Stop)
	cloned.MultipleStop = cloneSlice(src.MultipleStop)

	return &cloned
}

func cloneEmbeddingRequest(src *llm.EmbeddingRequest) *llm.EmbeddingRequest {
	if src == nil {
		return nil
	}

	cloned := *src
	cloned.Dimensions = clonePtr(src.Dimensions)

	return &cloned
}

func cloneRerankRequest(src *llm.RerankRequest) *llm.RerankRequest {
	if src == nil {
		return nil
	}

	cloned := *src
	cloned.Documents = cloneSlice(src.Documents)
	cloned.TopN = clonePtr(src.TopN)
	cloned.ReturnDocuments = clonePtr(src.ReturnDocuments)

	return &cloned
}

func cloneImageRequest(src *llm.ImageRequest) *llm.ImageRequest {
	if src == nil {
		return nil
	}

	cloned := *src
	cloned.Images = cloneByteSlices(src.Images)
	cloned.Mask = cloneBytes(src.Mask)
	cloned.N = clonePtr(src.N)
	cloned.OutputCompression = clonePtr(src.OutputCompression)
	cloned.PartialImages = clonePtr(src.PartialImages)

	return &cloned
}

func cloneVideoRequest(src *llm.VideoRequest) *llm.VideoRequest {
	if src == nil {
		return nil
	}

	cloned := *src
	cloned.Content = cloneVideoContent(src.Content)
	cloned.Duration = clonePtr(src.Duration)
	cloned.Frames = clonePtr(src.Frames)
	cloned.Seed = clonePtr(src.Seed)
	cloned.GenerateAudio = clonePtr(src.GenerateAudio)
	cloned.CameraFixed = clonePtr(src.CameraFixed)
	cloned.Watermark = clonePtr(src.Watermark)
	cloned.Draft = clonePtr(src.Draft)
	cloned.ExecutionExpiresAfter = clonePtr(src.ExecutionExpiresAfter)

	return &cloned
}

func cloneVideoContent(src []llm.VideoContent) []llm.VideoContent {
	if len(src) == 0 {
		return nil
	}

	cloned := make([]llm.VideoContent, len(src))
	for i, content := range src {
		cloned[i] = content
		cloned[i].ImageURL = clonePtr(content.ImageURL)
	}

	return cloned
}

func cloneCompactRequest(src *llm.CompactRequest) *llm.CompactRequest {
	if src == nil {
		return nil
	}

	cloned := *src
	cloned.Input = cloneMessages(src.Input)

	return &cloned
}

func cloneCompletionRequest(src *llm.CompletionRequest) *llm.CompletionRequest {
	if src == nil {
		return nil
	}

	cloned := *src
	cloned.MaxTokens = clonePtr(src.MaxTokens)
	cloned.Temperature = clonePtr(src.Temperature)
	cloned.TopP = clonePtr(src.TopP)
	cloned.N = clonePtr(src.N)
	cloned.Logprobs = clonePtr(src.Logprobs)
	cloned.Echo = clonePtr(src.Echo)
	cloned.Stop = cloneStop(src.Stop)
	cloned.PresencePenalty = clonePtr(src.PresencePenalty)
	cloned.FrequencyPenalty = clonePtr(src.FrequencyPenalty)
	cloned.BestOf = clonePtr(src.BestOf)
	cloned.LogitBias = cloneMap(src.LogitBias)
	cloned.Seed = clonePtr(src.Seed)

	return &cloned
}

func cloneTransformOptions(src llm.TransformOptions) llm.TransformOptions {
	return llm.TransformOptions{
		ArrayInstructions: clonePtr(src.ArrayInstructions),
		ArrayInputs:       clonePtr(src.ArrayInputs),
	}
}

func cloneHTTPClientRequest(src *httpclient.Request) *httpclient.Request {
	if src == nil {
		return nil
	}

	cloned := *src
	cloned.Query = cloneURLValues(src.Query)
	cloned.Headers = src.Headers.Clone()
	cloned.Body = cloneBytes(src.Body)
	cloned.JSONBody = cloneBytes(src.JSONBody)
	cloned.Metadata = cloneMap(src.Metadata)
	cloned.TransformerMetadata = cloneAnyMap(src.TransformerMetadata)

	return &cloned
}

func cloneURLValues(src url.Values) url.Values {
	if len(src) == 0 {
		return nil
	}

	cloned := make(url.Values, len(src))
	for key, values := range src {
		cloned[key] = cloneSlice(values)
	}

	return cloned
}

func cloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}

	cloned := make(map[string]any, len(src))
	for key, value := range src {
		cloned[key] = cloneAny(value)
	}

	return cloned
}

func cloneAny(value any) any {
	switch typed := value.(type) {
	case json.RawMessage:
		return cloneRawMessage(typed)
	case []byte:
		return cloneBytes(typed)
	case []string:
		return cloneSlice(typed)
	case []any:
		cloned := make([]any, len(typed))
		for i, item := range typed {
			cloned[i] = cloneAny(item)
		}
		return cloned
	case map[string]any:
		return cloneAnyMap(typed)
	case map[string]string:
		return cloneMap(typed)
	case *string:
		return clonePtr(typed)
	case *bool:
		return clonePtr(typed)
	case *int:
		return clonePtr(typed)
	case *int64:
		return clonePtr(typed)
	case *float64:
		return clonePtr(typed)
	default:
		return value
	}
}

func cloneRawMessage(src json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), src...)
}

func cloneByteSlices(src [][]byte) [][]byte {
	if len(src) == 0 {
		return nil
	}

	cloned := make([][]byte, len(src))
	for i, item := range src {
		cloned[i] = cloneBytes(item)
	}

	return cloned
}

func cloneBytes(src []byte) []byte {
	return append([]byte(nil), src...)
}

func cloneSlice[T any](src []T) []T {
	return append([]T(nil), src...)
}

func cloneMap[T any](src map[string]T) map[string]T {
	if len(src) == 0 {
		return nil
	}

	cloned := make(map[string]T, len(src))
	for key, value := range src {
		cloned[key] = value
	}

	return cloned
}

func clonePtr[T any](src *T) *T {
	if src == nil {
		return nil
	}

	cloned := *src

	return &cloned
}
