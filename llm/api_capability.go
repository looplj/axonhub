package llm

// CapableAPIFormats returns the API formats that can serve the given request type.
// The returned map is keyed by the string form of APIFormat.
// A nil map means the request type has no dedicated capability set.
func CapableAPIFormats(requestType RequestType) map[string]struct{} {
	//nolint:exhaustive // Unknown request types have no dedicated capability set.
	switch requestType {
	case RequestTypeChat:
		return map[string]struct{}{
			APIFormatOpenAIChatCompletion.String(): {},
			APIFormatOpenAIResponse.String():       {},
			APIFormatAnthropicMessage.String():     {},
			APIFormatGeminiContents.String():       {},
			APIFormatOllamaChat.String():           {},
		}
	case RequestTypeCompact:
		return map[string]struct{}{
			APIFormatOpenAIResponseCompact.String(): {},
		}
	case RequestTypeCompletion:
		return map[string]struct{}{
			APIFormatOpenAICompletion.String(): {},
		}
	case RequestTypeEmbedding:
		return map[string]struct{}{
			APIFormatOpenAIEmbedding.String(): {},
			APIFormatJinaEmbedding.String():   {},
			APIFormatGeminiEmbedding.String(): {},
		}
	case RequestTypeModeration:
		return map[string]struct{}{
			APIFormatOpenAIModeration.String(): {},
		}
	case RequestTypeImage:
		return map[string]struct{}{
			APIFormatOpenAIImageGeneration.String(): {},
			APIFormatOpenAIImageEdit.String():       {},
			APIFormatOpenAIImageVariation.String():  {},
		}
	case RequestTypeRerank:
		return map[string]struct{}{
			APIFormatJinaRerank.String(): {},
		}
	case RequestTypeVideo:
		return map[string]struct{}{
			APIFormatOpenAIVideo.String():   {},
			APIFormatSeedanceVideo.String(): {},
		}
	case RequestTypeSpeech:
		return map[string]struct{}{
			APIFormatOpenAISpeech.String(): {},
		}
	case RequestTypeTranscription:
		return map[string]struct{}{
			APIFormatOpenAITranscription.String(): {},
		}
	case RequestTypeTranslation:
		return map[string]struct{}{
			APIFormatOpenAITranslation.String(): {},
		}
	default:
		return nil
	}
}

// RequestTypeForModelType maps a configured Model entity type to a request type
// used for structural endpoint capability checks.
func RequestTypeForModelType(modelType string) RequestType {
	switch modelType {
	case "chat":
		return RequestTypeChat
	case "embedding":
		return RequestTypeEmbedding
	case "rerank":
		return RequestTypeRerank
	case "image_generation":
		return RequestTypeImage
	case "video_generation":
		return RequestTypeVideo
	default:
		return ""
	}
}
