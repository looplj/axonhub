package llm

type APIFormat string

const (
	APIFormatOpenAIChatCompletion APIFormat = "openai/chat_completions"
	APIFormatOpenAIResponse       APIFormat = "openai/response"
	APIFormatAnthropicMessage     APIFormat = "anthropic/messages"
	APIFormatAiSDKText            APIFormat = "aisdk/text"
)
