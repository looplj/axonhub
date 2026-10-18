package orchestrator

import (
	"context"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

// filterResolvedCandidatesForRequest evaluates `When` only after association
// resolution. This keeps the matching step focused on static association
// structure and makes request-dependent filtering easier to follow.
func filterResolvedCandidatesForRequest(
	ctx context.Context,
	req *llm.Request,
	resolvedCandidates []*resolvedAssociationCandidate,
) []*ChannelModelsCandidate {
	if len(resolvedCandidates) == 0 {
		return []*ChannelModelsCandidate{}
	}

	hasConditionalCandidates := lo.ContainsBy(resolvedCandidates, func(candidate *resolvedAssociationCandidate) bool {
		return candidate != nil && candidate.when != nil
	})
	if !hasConditionalCandidates {
		candidates := aggregateChannelModelCandidates(resolvedCandidates)
		candidates = populateAPIFormat(ctx, candidates, req)

		return candidates
	}

	promptTokens := estimatePromptTokens(req)
	stream := reqStream(req)
	requestFormat := reqAPIFormat(req)
	reasoningEffort := reqReasoningEffort(req)
	contentFeatures := detectRequestContentFeatures(req)
	requestHeaders := buildRequestHeaderMap(req)
	now := time.Now()
	filtered := make([]*resolvedAssociationCandidate, 0, len(resolvedCandidates))

	for _, candidate := range resolvedCandidates {
		if candidate == nil {
			continue
		}

		if !matchesAssociationWhen(promptTokens, stream, requestFormat, reasoningEffort, contentFeatures, requestHeaders, now, candidate.when) {
			continue
		}

		filtered = append(filtered, candidate)
	}

	candidates := aggregateChannelModelCandidates(filtered)

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "evaluated conditional associations",
			log.Int64("estimated_prompt_tokens", promptTokens),
			log.Int("matched_candidate_count", len(filtered)),
		)
	}

	candidates = populateAPIFormat(ctx, candidates, req)

	return candidates
}

func populateAPIFormat(ctx context.Context, candidates []*ChannelModelsCandidate, req *llm.Request) []*ChannelModelsCandidate {
	filtered := make([]*ChannelModelsCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c == nil || c.Channel == nil {
			continue
		}

		// A candidate may contain several models that are retried in order. Apply
		// protocol overrides to one model at a time; applying them to the whole
		// slice would merge unrelated model overrides and select the wrong
		// protocol for the first attempt.
		baseEndpoints := c.Channel.ResolveEndpoints()
		if len(c.Models) > 0 {
			selectedModels := make([]biz.ChannelModelEntry, 0, len(c.Models))
			selectedFormats := make([]string, 0, len(c.Models))
			for _, entry := range c.Models {
				endpoints := applyForcedAPIFormatsForRequest(ctx, c.Channel, []biz.ChannelModelEntry{entry}, req.Model, req.RequestType, baseEndpoints)
				format := SelectAPIFormat(endpoints, req)
				// Explicit protocols have no generic fallback. A model whose forced protocol
				// list cannot serve the requested protocol must not remain as the first retry
				// entry, otherwise an empty candidate format falls back to the channel's
				// primary (usually chat) outbound.
				if requiresExplicitEndpoint(req.RequestType) && format == "" {
					continue
				}

				selectedModels = append(selectedModels, entry)
				selectedFormats = append(selectedFormats, format)
			}

			if len(selectedModels) == 0 {
				continue
			}

			if requiresExplicitEndpoint(req.RequestType) {
				c.Models = selectedModels
			}
			c.modelAPIFormats = selectedFormats
			c.APIFormat = selectedFormats[0]
		} else {
			c.modelAPIFormats = nil
			endpoints := applyForcedAPIFormatsForRequest(ctx, c.Channel, c.Models, req.Model, req.RequestType, baseEndpoints)
			c.APIFormat = SelectAPIFormat(endpoints, req)
		}

		if requiresExplicitEndpoint(req.RequestType) && c.APIFormat == "" {
			continue
		}

		filtered = append(filtered, c)
	}

	return filtered
}

func reqStream(req *llm.Request) bool {
	if req == nil || req.Stream == nil {
		return false
	}

	return *req.Stream
}

func reqAPIFormat(req *llm.Request) string {
	if req == nil {
		return ""
	}

	return string(req.APIFormat)
}

func reqReasoningEffort(req *llm.Request) string {
	if req == nil {
		return ""
	}

	return req.ReasoningEffort
}

type requestContentFeatures struct {
	hasImage    bool
	hasVideo    bool
	hasDocument bool
	hasAudio    bool
}

func matchesAssociationWhen(
	promptTokens int64,
	stream bool,
	requestFormat string,
	reasoningEffort string,
	contentFeatures requestContentFeatures,
	requestHeaders map[string]string,
	now time.Time,
	when *objects.ModelAssociationWhen,
) bool {
	if when == nil {
		return true
	}

	if !when.Enabled {
		return true
	}

	if when.Condition != nil && !objects.Evaluate(*when.Condition, map[string]any{
		objects.ModelAssociationConditionFieldPromptTokens:    promptTokens,
		objects.ModelAssociationConditionFieldStream:          stream,
		objects.ModelAssociationConditionFieldRequestFormat:   requestFormat,
		objects.ModelAssociationConditionFieldReasoningEffort: reasoningEffort,
		objects.ModelAssociationConditionFieldHasImage:        contentFeatures.hasImage,
		objects.ModelAssociationConditionFieldHasVideo:        contentFeatures.hasVideo,
		objects.ModelAssociationConditionFieldHasDocument:     contentFeatures.hasDocument,
		objects.ModelAssociationConditionFieldHasAudio:        contentFeatures.hasAudio,
		objects.ModelAssociationConditionFieldRequestHeader:   requestHeaders,
		"now": now,
	}) {
		return false
	}

	return true
}

func detectRequestContentFeatures(req *llm.Request) requestContentFeatures {
	var features requestContentFeatures
	if req == nil {
		return features
	}

	for _, message := range req.Messages {
		for _, part := range message.Content.MultipleContent {
			switch {
			case part.ImageURL != nil:
				features.hasImage = true
			case part.VideoURL != nil:
				features.hasVideo = true
			case part.Document != nil:
				features.hasDocument = true
			case part.InputAudio != nil:
				features.hasAudio = true
			}

			if features.hasAll() {
				return features
			}
		}
	}

	return features
}

func (features requestContentFeatures) hasAll() bool {
	return features.hasImage && features.hasVideo && features.hasDocument && features.hasAudio
}

// estimatePromptTokens is kept as a thin wrapper so the association-condition
// path and the Anthropic inbound stream share one implementation
// (`llm.EstimatePromptTokens`). Duplicating the heuristic in two packages is how
// the two would drift apart.
func estimatePromptTokens(req *llm.Request) int64 {
	return llm.EstimatePromptTokens(req)
}
