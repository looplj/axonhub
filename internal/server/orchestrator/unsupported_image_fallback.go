package orchestrator

import (
	"context"
	"errors"
	"net/http"
	"slices"
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

	normalizeProviderCode := func(value string) string {
		return strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(strings.ToLower(strings.TrimSpace(value)))
	}
	explicitProviderCodes := []string{
		"unsupported_image",
		"unsupported_image_input",
		"image_not_supported",
		"image_input_unsupported",
		"image_input_not_supported",
		"images_not_supported",
		"unsupported_vision",
		"unsupported_vision_input",
		"vision_not_supported",
		"vision_input_unsupported",
		"unsupported_multimodal",
		"unsupported_multimodal_input",
		"multimodal_not_supported",
		"multimodal_input_unsupported",
		"unsupported_modality_image",
		"image_modality_unsupported",
		"image_modality_not_supported",
	}
	if slices.Contains(explicitProviderCodes, normalizeProviderCode(responseErr.Detail.Code)) ||
		slices.Contains(explicitProviderCodes, normalizeProviderCode(responseErr.Detail.Type)) {
		return true
	}

	message := strings.ToLower(responseErr.Detail.Message)
	return lo.SomeBy([]string{
		"image input is unsupported",
		"image input is not supported",
		"image input not supported",
		"image inputs are unsupported",
		"images are not supported",
		"does not support image input",
		"doesn't support image input",
		"does not support images",
		"doesn't support images",
		"image modality is unsupported",
		"unsupported image modality",
		"vision input is unsupported",
		"vision input is not supported",
		"does not support vision",
		"doesn't support vision",
		"multimodal input is unsupported",
		"multi-modal input is unsupported",
		"does not support multimodal",
		"does not support multi-modal",
		"only supports text",
		"supports text only",
		"text-only model",
		"text only model",
		"不支持图片输入",
		"不支持图像输入",
		"不支持视觉输入",
		"不支持多模态输入",
		"模型不支持图片",
		"模型不支持图像",
		"仅支持文本",
		"只支持文本",
		"纯文本模型",
	}, func(cue string) bool { return strings.Contains(message, cue) })
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
