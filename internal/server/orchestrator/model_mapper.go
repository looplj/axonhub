package orchestrator

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xregexp"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
)

func applyModelMapping(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return &apiKeyModelMappingMiddleware{
		inbound: inbound,
	}
}

type apiKeyModelMappingMiddleware struct {
	pipeline.DummyMiddleware
	// RequestModel is the original model from client request, before any API key profile mapping
	RequestModel string
	inbound      *PersistentInboundTransformer
}

func (m *apiKeyModelMappingMiddleware) Name() string {
	return "apply-model-mapping"
}

func (m *apiKeyModelMappingMiddleware) OnInboundLlmRequest(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
	if llmRequest.Model == "" {
		return nil, fmt.Errorf("%w: request model is empty", biz.ErrInvalidModel)
	}

	// Save the original client request model before any mapping
	if m.RequestModel == "" {
		m.RequestModel = llmRequest.Model
	}

	// Apply model mapping from API key profiles if active profile exists
	if m.inbound.state.APIKey == nil {
		return llmRequest, nil
	}

	originalModel := llmRequest.Model
	mappedModel := m.inbound.state.ModelMapper.MapModel(ctx, m.inbound.state.APIKey, originalModel)

	if mappedModel != originalModel {
		llmRequest.Model = mappedModel
		log.Debug(ctx, "applied model mapping from API key profile",
			log.String("api_key_name", m.inbound.state.APIKey.Name),
			log.String("original_model", originalModel),
			log.String("mapped_model", mappedModel))
	}

	// Save the model for later use, e.g. retry from next channels, should use the original model to choose channel model.
	// This should be done after the api key level model mapping.
	// This should be done before the request is created.
	// The outbound transformer will restore the original model if it was mapped.
	if m.inbound.state.OriginalModel == "" {
		m.inbound.state.OriginalModel = llmRequest.Model
	} else {
		// Restore original model if it was mapped
		// This should not happen, the inbound should not be called twice.
		// Just in case, restore the original model.
		llmRequest.Model = m.inbound.state.OriginalModel
	}

	return llmRequest, nil
}

func (m *apiKeyModelMappingMiddleware) OnOutboundLlmResponse(ctx context.Context, response *llm.Response) (*llm.Response, error) {
	m.inbound.state.ModelMapper.ReplaceResponseModel(response, m.RequestModel)
	return response, nil
}

func (m *apiKeyModelMappingMiddleware) OnOutboundRawStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*httpclient.StreamEvent], error) {
	return stream, nil
}

func (m *apiKeyModelMappingMiddleware) OnOutboundLlmStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*llm.Response], error) {
	if m.RequestModel == "" {
		return stream, nil
	}

	// Wrap the stream to replace model in each response
	return streams.Map(stream, func(response *llm.Response) *llm.Response {
		m.inbound.state.ModelMapper.ReplaceResponseModel(response, m.RequestModel)
		return response
	}), nil
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

// ReplaceResponseModel replaces the model field in llm.Response with the original client request model.
func (m *ModelMapper) ReplaceResponseModel(response *llm.Response, requestModel string) {
	// If the response model is empty, it means the model field was not sent from the LLM service.
	// In this case, we should not replace the model.
	if response != nil && response.Model != "" && response.Model != requestModel {
		response.Model = requestModel
	}
}
