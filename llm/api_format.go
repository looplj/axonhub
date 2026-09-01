package llm

// IsOpenAIResponsesFormat reports whether format belongs to the OpenAI
// Responses protocol family, including its compact endpoint.
func IsOpenAIResponsesFormat(format APIFormat) bool {
	return format == APIFormatOpenAIResponse || format == APIFormatOpenAIResponseCompact
}
