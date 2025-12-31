package orchestrator

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xregexp"
	"github.com/looplj/axonhub/internal/server/biz"
)

func applyApiKeyModelMapping(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return pipeline.OnLlmRequest("apply-model-mapping", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
		if llmRequest.Model == "" {
			return nil, fmt.Errorf("%w: request model is empty", biz.ErrInvalidModel)
		}

		// Apply model mapping from API key profiles if active profile exists
		if inbound.state.APIKey == nil {
			return llmRequest, nil
		}

		originalModel := llmRequest.Model
		mappedModel := inbound.state.ModelMapper.MapModel(ctx, inbound.state.APIKey, originalModel)

		if mappedModel != originalModel {
			llmRequest.Model = mappedModel
			log.Debug(ctx, "applied model mapping from API key profile",
				log.String("api_key_name", inbound.state.APIKey.Name),
				log.String("original_model", originalModel),
				log.String("mapped_model", mappedModel))
		}

		// Save the model for later use, e.g. retry from next channels, should use the original model to choose channel model.
		// This should be done after the api key level model mapping.
		// This should be done before the request is created.
		// The outbound transformer will restore the original model if it was mapped.
		if inbound.state.OriginalModel == "" {
			inbound.state.OriginalModel = llmRequest.Model
		} else {
			// Restore original model if it was mapped
			// This should not happen, the inbound should not be called twice.
			// Just in case, restore the original model.
			llmRequest.Model = inbound.state.OriginalModel
		}

		return llmRequest, nil
	})
}

// ModelMapper handles model mapping based on API key profiles.
type ModelMapper struct{}

// NewModelMapper creates a new ModelMapper instance.
func NewModelMapper() *ModelMapper {
	return &ModelMapper{}
}

// MapModel applies model mapping from API key profiles if an active profile exists
// Returns the mapped model name or the original model if no mapping is found.
func (m *ModelMapper) MapModel(ctx context.Context, apiKey *ent.APIKey, originalModel string) string {
	if apiKey == nil || apiKey.Profiles == nil {
		return originalModel
	}

	profiles := apiKey.Profiles
	if profiles.ActiveProfile == "" {
		log.Debug(ctx, "No active profile found for API key", log.String("api_key_name", apiKey.Name))
		return originalModel
	}

	activeProfile := apiKey.GetActiveProfile()
	if activeProfile == nil {
		log.Warn(ctx, "Active profile not found in profiles list",
			log.String("active_profile", profiles.ActiveProfile),
			log.String("api_key_name", apiKey.Name))

		return originalModel
	}

	// Apply model mapping
	mappedModel := m.applyModelMapping(activeProfile.ModelMappings, originalModel)

	if mappedModel != originalModel {
		log.Debug(ctx, "Model mapped using API key profile",
			log.String("api_key_name", apiKey.Name),
			log.String("active_profile", profiles.ActiveProfile),
			log.String("original_model", originalModel),
			log.String("mapped_model", mappedModel))
	} else {
		log.Debug(ctx, "Model not mapped using API key profile",
			log.String("api_key_name", apiKey.Name),
			log.String("active_profile", profiles.ActiveProfile),
			log.String("original_model", originalModel))
	}

	return mappedModel
}

// applyModelMapping applies model mappings from the given list
// Returns the mapped model or the original if no mapping is found.
func (m *ModelMapper) applyModelMapping(mappings []objects.ModelMapping, model string) string {
	for _, mapping := range mappings {
		if m.matchesMapping(mapping.From, model) {
			return mapping.To
		}
	}

	return model
}

// matchesMapping checks if a model matches a mapping pattern using cached regex
// Supports exact match and regex patterns (including wildcard conversion).
func (m *ModelMapper) matchesMapping(pattern, model string) bool {
	return xregexp.MatchString(pattern, model)
}
