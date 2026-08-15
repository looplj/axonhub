package orchestrator

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
)

const unsupportedImageMarker = "[Unsupported Image]"

func unsupportedImageFallback(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return pipeline.OnLlmRequest("unsupported-image-fallback", func(ctx context.Context, request *llm.Request) (*llm.Request, error) {
		state := inbound.state
		if request == nil || state == nil || state.DelegationDepth > 0 ||
			!unsupportedImageFallbackEnabled(state.SourceModel) ||
			!detectRequestContentFeatures(request).hasImage {
			return request, nil
		}

		settings := state.SourceModel.Settings
		if settings.VisionDelegation.Enabled || !modelDeclaresTextOnly(state.SourceModel) {
			return request, nil
		}

		if applyUnsupportedImageFallback(state, request) {
			log.Debug(ctx, "replaced image input for explicitly text-only model",
				log.String("model", state.SourceModel.ModelID),
			)
		}

		return request, nil
	})
}

func unsupportedImageFallbackEnabled(source *ent.Model) bool {
	return source != nil && source.Settings != nil && source.Settings.UnsupportedImageFallback.IsEnabled()
}

func modelDeclaresTextOnly(source *ent.Model) bool {
	if source == nil || source.ModelCard == nil || source.ModelCard.SupportsVision() {
		return false
	}

	return lo.SomeBy(source.ModelCard.Modalities.Input, func(modality string) bool {
		return strings.EqualFold(strings.TrimSpace(modality), "text")
	})
}

func isUnsupportedImageInputError(err error) bool {
	if err == nil || !pipeline.IsUpstreamError(err) {
		return false
	}

	var responseErr *llm.ResponseError
	if !errors.As(err, &responseErr) || responseErr == nil {
		return false
	}
	if responseErr.StatusCode != http.StatusBadRequest &&
		responseErr.StatusCode != http.StatusUnsupportedMediaType &&
		responseErr.StatusCode != http.StatusUnprocessableEntity {
		return false
	}

	detail := strings.ToLower(strings.Join([]string{
		responseErr.Detail.Code,
		responseErr.Detail.Type,
		responseErr.Detail.Param,
		responseErr.Detail.Message,
	}, " "))
	imageCue := lo.SomeBy([]string{
		"image", "vision", "multimodal", "multi-modal", "图片", "图像", "视觉", "多模态",
	}, func(cue string) bool { return strings.Contains(detail, cue) })
	unsupportedCue := lo.SomeBy([]string{
		"unsupported", "not supported", "does not support", "doesn't support",
		"not accept", "doesn't accept", "not allowed", "text-only", "text only",
		"only supports text", "cannot process", "can't process",
		"不支持", "不接受", "仅支持文本", "纯文本", "无法处理", "不能处理",
	}, func(cue string) bool { return strings.Contains(detail, cue) })

	return imageCue && unsupportedCue
}

func isVisionDelegationTargetImageUnsupportedError(err error) bool {
	return errors.Is(err, biz.ErrVisionDelegationTargetImageUnsupported)
}

func applyUnsupportedImageFallback(state *PersistenceState, request *llm.Request) bool {
	if !replaceImagesWithUnsupportedMarker(request) {
		return false
	}

	if state != nil {
		state.DisableRequestBodyPassThrough = true
	}

	return true
}

func replaceImagesWithUnsupportedMarker(request *llm.Request) bool {
	if request == nil {
		return false
	}

	replaced := false
	for messageIndex := range request.Messages {
		message := &request.Messages[messageIndex]
		if message.Content.Content != nil {
			cleaned := stripImageSourceMarker(*message.Content.Content)
			if cleaned != *message.Content.Content {
				replaced = true
				message.Content.Content = lo.ToPtr(cleaned)
			}
		}

		parts := make([]llm.MessageContentPart, 0, len(message.Content.MultipleContent))
		for _, part := range message.Content.MultipleContent {
			if part.Type == "image_url" || part.ImageURL != nil {
				replaced = true
				parts = append(parts, llm.MessageContentPart{
					Type: "text",
					Text: lo.ToPtr(unsupportedImageMarker),
				})
				continue
			}

			if part.Type == "text" && part.Text != nil {
				cleaned := stripImageSourceMarker(*part.Text)
				if strings.TrimSpace(*part.Text) != "" && cleaned == "" {
					replaced = true
					continue
				}
				if cleaned != *part.Text {
					replaced = true
					part.Text = lo.ToPtr(cleaned)
				}
			}

			parts = append(parts, part)
		}
		message.Content.MultipleContent = parts
	}

	return replaced
}
