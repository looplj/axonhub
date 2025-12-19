package llm

type APIFormat string

const (
	APIFormatOpenAIChatCompletion  APIFormat = "openai/chat_completions"
	APIFormatOpenAIResponse        APIFormat = "openai/responses"
	APIFormatOpenAIImageGeneration APIFormat = "openai/image_generation"
	APIFormatOpenAIEmbedding       APIFormat = "openai/embeddings"
	APIFormatOpenAIRerank          APIFormat = "openai/rerank"
	APIFormatGeminiContents        APIFormat = "gemini/contents"
	APIFormatAnthropicMessage      APIFormat = "anthropic/messages"
	APIFormatAiSDKText             APIFormat = "aisdk/text"
	APIFormatAiSDKDataStream       APIFormat = "aisdk/datastream"
)

func (f APIFormat) String() string {
	return string(f)
}

const (
	ToolType                = "function"
	ToolTypeImageGeneration = "image_generation"
	// ToolTypeGoogleSearch is the Google Search grounding tool type for Gemini.
	ToolTypeGoogleSearch = "google_search"
	// ToolTypeGoogleCodeExecution is the code execution tool type for Gemini.
	ToolTypeGoogleCodeExecution = "google_code_execution"
	// ToolTypeGoogleUrlContext is the URL context grounding tool type for Gemini 2.0+.
	ToolTypeGoogleUrlContext = "google_url_context"

	// ToolTypeAnthropicWebSearch is the native web search tool type for Anthropic (Beta).
	// This tool is only supported by native Anthropic API format channels.
	ToolTypeAnthropicWebSearch = "web_search_20250305"
	// AnthropicWebSearchFunctionName is the standard function name that triggers
	// native Anthropic web search tool transformation.
	AnthropicWebSearchFunctionName = "web_search"
)
