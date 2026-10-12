package biz

import "github.com/looplj/axonhub/llm"

func isNativeImageFormat(format string) bool {
	return format == llm.APIFormatOpenAIImageGeneration.String() || format == llm.APIFormatOpenAIImageEdit.String()
}
