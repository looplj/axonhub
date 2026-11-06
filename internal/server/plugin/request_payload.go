package plugin

import "github.com/looplj/axonhub/llm"

// preserveRequestPayload retains private data when onRequest replaces its public
// projection. JS never receives these buffers, so retain them without cloning.
// This does not copy or isolate requests between upstream attempts.
func preserveRequestPayload(dst, src *llm.Request) {
	if src.Image != nil {
		if dst.Image == nil {
			dst.Image = &llm.ImageRequest{}
		}
		dst.Image.ModelSpecified = src.Image.ModelSpecified
		dst.Image.Mask = src.Image.Mask
		dst.Image.Images = src.Image.Images
	}
	if src.Transcription != nil {
		if dst.Transcription == nil {
			dst.Transcription = &llm.TranscriptionRequest{}
		}
		dst.Transcription.File = src.Transcription.File
		dst.Transcription.FileName = src.Transcription.FileName
	}
	if src.Translation != nil {
		if dst.Translation == nil {
			dst.Translation = &llm.TranslationRequest{}
		}
		dst.Translation.File = src.Translation.File
		dst.Translation.FileName = src.Translation.FileName
	}
	if src.AlphaSearch != nil {
		if dst.AlphaSearch == nil {
			dst.AlphaSearch = &llm.AlphaSearchRequest{}
		}
		dst.AlphaSearch.Body = src.AlphaSearch.Body
	}
	if src.Decisions != nil {
		if dst.Decisions == nil {
			dst.Decisions = &llm.DecisionsRequest{}
		}
		dst.Decisions.Body = src.Decisions.Body
	}
}
