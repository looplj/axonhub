package biz

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// detectableEndpointAPIFormats is the ordered set of protocol surfaces the
// endpoint detector probes. Only these three formats are auto-detected: if a
// channel speaks all of them it can relay virtually every client protocol
// without lossy transformations.
var detectableEndpointAPIFormats = []string{
	llm.APIFormatOpenAIChatCompletion.String(),
	llm.APIFormatOpenAIResponse.String(),
	llm.APIFormatAnthropicMessage.String(),
}

// endpointDetectTimeout bounds each upstream probe so a slow or hanging
// provider cannot stall the whole detection request.
const endpointDetectTimeout = 20 * time.Second

// Detection outcome reasons. They are stable values consumed by the frontend
// to explain why a protocol was or was not marked as supported.
const (
	endpointDetectReasonSupported   = "supported"
	endpointDetectReasonNotFound    = "not_found"
	endpointDetectReasonAuthError   = "auth_error"
	endpointDetectReasonRateLimited = "rate_limited"
	endpointDetectReasonServerError = "server_error"
	endpointDetectReasonUnreachable = "unreachable"
	endpointDetectReasonInvalid     = "invalid_request"
)

// DetectChannelEndpointsInput is the input for automatic endpoint detection.
type DetectChannelEndpointsInput struct {
	ChannelID objects.GUID `json:"channelID"`
	// Model optionally pins the model used for the probe. When empty the
	// channel default test model (or first supported model) is used.
	Model *string `json:"model,omitempty"`
}

// DetectedChannelEndpoint reports the detection outcome for a single protocol.
type DetectedChannelEndpoint struct {
	APIFormat  string `json:"apiFormat"`
	Supported  bool   `json:"supported"`
	StatusCode int    `json:"statusCode"`
	Reason     string `json:"reason"`
}

// DetectChannelEndpointsPayload is the result of automatic endpoint detection.
type DetectChannelEndpointsPayload struct {
	Endpoints []DetectedChannelEndpoint `json:"endpoints"`
}

// DetectChannelEndpoints probes the channel upstream for the three supported
// relay protocols (chat completions, responses and messages) and reports which
// ones the upstream actually exposes. It never mutates the channel; callers
// decide whether to persist the detected endpoints.
func (svc *ChannelService) DetectChannelEndpoints(
	ctx context.Context,
	input DetectChannelEndpointsInput,
) (*DetectChannelEndpointsPayload, error) {
	entity, err := svc.entFromContext(ctx).Channel.Get(ctx, input.ChannelID.ID)
	if err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}

	ch, err := svc.buildChannelWithTransformer(entity)
	if err != nil {
		return nil, fmt.Errorf("failed to build channel %s: %w", entity.Name, err)
	}

	model := resolveEndpointDetectModel(entity, input.Model)

	results := make([]DetectedChannelEndpoint, len(detectableEndpointAPIFormats))
	var wg sync.WaitGroup

	for i, apiFormat := range detectableEndpointAPIFormats {
		wg.Add(1)

		go func(index int, format string) {
			defer wg.Done()

			results[index] = svc.detectChannelEndpoint(ctx, entity, ch, format, model)
		}(i, apiFormat)
	}

	wg.Wait()

	return &DetectChannelEndpointsPayload{Endpoints: results}, nil
}

// detectChannelEndpoint probes a single api_format and classifies the result.
// A route is considered supported unless the upstream clearly reports that the
// path does not exist (404/405) or cannot be reached at all.
func (svc *ChannelService) detectChannelEndpoint(
	ctx context.Context,
	entity *ent.Channel,
	ch *Channel,
	apiFormat string,
	model string,
) DetectedChannelEndpoint {
	result := DetectedChannelEndpoint{
		APIFormat: apiFormat,
		Reason:    endpointDetectReasonUnreachable,
	}

	outbound, err := svc.buildNonDefaultEndpointOutbound(entity, ch, objects.ChannelEndpoint{APIFormat: apiFormat})
	if err != nil {
		log.Debug(ctx, "endpoint detection: cannot build outbound",
			log.String("channel", entity.Name),
			log.String("api_format", apiFormat),
			log.Cause(err))
		result.Reason = endpointDetectReasonInvalid

		return result
	}

	probeCtx, cancel := context.WithTimeout(ctx, endpointDetectTimeout)
	defer cancel()

	httpReq, err := outbound.TransformRequest(probeCtx, buildEndpointDetectRequest(model))
	if err != nil {
		log.Debug(ctx, "endpoint detection: cannot build request",
			log.String("channel", entity.Name),
			log.String("api_format", apiFormat),
			log.Cause(err))
		result.Reason = endpointDetectReasonInvalid

		return result
	}

	resp, err := ch.HTTPClient.Do(probeCtx, httpReq)
	if err == nil {
		result.Supported = true
		result.StatusCode = resp.StatusCode
		result.Reason = endpointDetectReasonSupported

		return result
	}

	var httpErr *httpclient.Error
	if !errors.As(err, &httpErr) {
		log.Debug(ctx, "endpoint detection: transport error",
			log.String("channel", entity.Name),
			log.String("api_format", apiFormat),
			log.Cause(err))

		return result
	}

	result.StatusCode = httpErr.StatusCode

	switch {
	case httpErr.StatusCode == 404 || httpErr.StatusCode == 405:
		result.Reason = endpointDetectReasonNotFound
	case httpErr.StatusCode == 401 || httpErr.StatusCode == 403:
		result.Reason = endpointDetectReasonAuthError
	case httpErr.StatusCode == 429:
		result.Supported = true
		result.Reason = endpointDetectReasonRateLimited
	case httpErr.StatusCode >= 500:
		result.Reason = endpointDetectReasonServerError
	default:
		// Any other client error (400/422/...) means the route exists and the
		// upstream rejected the probe payload, so the protocol is available.
		result.Supported = true
		result.Reason = endpointDetectReasonSupported
	}

	return result
}

// buildEndpointDetectRequest builds a minimal, cheap probe request. The model is
// always sent even when empty so upstreams that validate the field still reply
// with a route-level 4xx instead of a routing 404.
func buildEndpointDetectRequest(model string) *llm.Request {
	return &llm.Request{
		Model:               model,
		Messages:            []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("ping")}}},
		MaxCompletionTokens: lo.ToPtr(int64(1)),
		Stream:              lo.ToPtr(false),
	}
}

// resolveEndpointDetectModel picks the model used for probing, preferring an
// explicit request, then the channel default test model, then the first
// supported model.
func resolveEndpointDetectModel(entity *ent.Channel, requested *string) string {
	if requested != nil {
		if model := strings.TrimSpace(*requested); model != "" {
			return model
		}
	}

	if model := strings.TrimSpace(entity.DefaultTestModel); model != "" {
		return model
	}

	for _, model := range entity.SupportedModels {
		if trimmed := strings.TrimSpace(model); trimmed != "" {
			return trimmed
		}
	}

	return ""
}
