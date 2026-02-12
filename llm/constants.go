package llm

type RequestType string

const (
	RequestTypeChat      RequestType = "chat"
	RequestTypeEmbedding RequestType = "embedding"
	RequestTypeRerank    RequestType = "rerank"
	RequestTypeImage     RequestType = "image"
)

func (r RequestType) String() string {
	return string(r)
}

type APIFormat string

const (
	APIFormatOpenAIChatCompletion  APIFormat = "openai/chat_completions"
	APIFormatOpenAIResponse        APIFormat = "openai/responses"
	APIFormatOpenAIImageGeneration APIFormat = "openai/image_generation"
	APIFormatOpenAIImageEdit       APIFormat = "openai/image_edit"
	APIFormatOpenAIImageVariation  APIFormat = "openai/image_variation"
	APIFormatOpenAIEmbedding       APIFormat = "openai/embeddings"
	APIFormatGeminiContents        APIFormat = "gemini/contents"
	APIFormatAnthropicMessage      APIFormat = "anthropic/messages"
	APIFormatAiSDKText             APIFormat = "aisdk/text"
	APIFormatAiSDKDataStream       APIFormat = "aisdk/datastream"

	APIFormatJinaRerank    APIFormat = "jina/rerank"
	APIFormatJinaEmbedding APIFormat = "jina/embeddings"
)

func (f APIFormat) String() string {
	return string(f)
}

const (
	// ToolTypeFunction is the function grounding tool type for OpenAI.
	ToolTypeFunction = "function"

	// ToolTypeImageGeneration is the image generation grounding tool type for OpenAI.
	ToolTypeImageGeneration = "image_generation"

	// ToolTypeWebSearch is the web search grounding tool type.
	ToolTypeWebSearch = "web_search"

	// ToolTypeGoogleSearch is the Google Search grounding tool type for Gemini.
	ToolTypeGoogleSearch = "google_search"

	// ToolTypeGoogleCodeExecution is the code execution tool type for Gemini.
	ToolTypeGoogleCodeExecution = "google_code_execution"

	// ToolTypeGoogleUrlContext is the URL context grounding tool type for Gemini 2.0+.
	ToolTypeGoogleUrlContext = "google_url_context"
)
