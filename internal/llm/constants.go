package llm

type APIFormat string

const (
	APIFormatOpenAIChatCompletion APIFormat = "openai/chat_completions"
	APIFormatOpenAIResponse       APIFormat = "openai/responses"
	APIFormatAnthropicMessage     APIFormat = "anthropic/messages"
	APIFormatAiSDKText            APIFormat = "aisdk/text"
	APIFormatAiSDKDataStream      APIFormat = "aisdk/datastream"
)

func (f APIFormat) String() string {
	return string(f)
}

const (
	ToolType                = "function"
	ToolTypeImageGeneration = "image_generation"
)
