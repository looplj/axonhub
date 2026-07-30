package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	responsesapi "github.com/looplj/axonhub/llm/transformer/openai/responses"
)

const (
	maxPreviousResponseChainDepth         = 1024
	maxPreviousResponseHistoryBytes int64 = 32 * 1024 * 1024
)

type previousResponseExchangeLoader interface {
	LoadCompletedResponseExchange(context.Context, string, int, *int) (*biz.StoredResponseExchange, error)
}

type storedResponseRequestState struct {
	Instructions       string  `json:"instructions"`
	PreviousResponseID *string `json:"previous_response_id"`
}

// nextPreviousResponseHistoryBytes safely adds one retained exchange to the
// cumulative history size without allowing the configured limit to overflow.
func nextPreviousResponseHistoryBytes(total int64, requestBytes, responseBytes int) (int64, bool) {
	remaining := maxPreviousResponseHistoryBytes - total
	if int64(requestBytes) > remaining {
		return total, false
	}
	remaining -= int64(requestBytes)
	if int64(responseBytes) > remaining {
		return total, false
	}
	return total + int64(requestBytes) + int64(responseBytes), true
}

// hydratePreviousResponsesForChat expands server-side Responses history into
// explicit messages because Chat Completions cannot consume previous_response_id.
func hydratePreviousResponsesForChat(
	ctx context.Context,
	request *llm.Request,
	state *PersistenceState,
) (*llm.Request, error) {
	if request == nil || request.PreviousResponseID == nil || strings.TrimSpace(*request.PreviousResponseID) == "" {
		return request, nil
	}
	if state == nil || state.Request == nil || state.RequestService == nil {
		return nil, fmt.Errorf("%w: previous_response_id history storage is unavailable", transformer.ErrInvalidRequest)
	}

	var apiKeyID *int
	if state.APIKey != nil {
		apiKeyID = &state.APIKey.ID
	}

	history, err := loadPreviousResponsesHistory(
		ctx,
		state.RequestService,
		*request.PreviousResponseID,
		state.Request.ProjectID,
		apiKeyID,
	)
	if err != nil {
		return nil, err
	}

	hydrated := *request
	hydrated.Messages = append(history, request.Messages...)
	hydrated.PreviousResponseID = nil

	return &hydrated, nil
}

func loadPreviousResponsesHistory(
	ctx context.Context,
	loader previousResponseExchangeLoader,
	responseID string,
	projectID int,
	apiKeyID *int,
) ([]llm.Message, error) {
	type historySegment struct {
		messages []llm.Message
	}

	inbound := responsesapi.NewInboundTransformer()
	segments := make([]historySegment, 0, 4)
	visited := make(map[string]struct{})
	currentID := responseID
	var historyBytes int64

	for depth := 0; currentID != ""; depth++ {
		if depth >= maxPreviousResponseChainDepth {
			return nil, fmt.Errorf("%w: previous_response_id chain exceeds %d responses", transformer.ErrInvalidRequest, maxPreviousResponseChainDepth)
		}
		if _, exists := visited[currentID]; exists {
			return nil, fmt.Errorf("%w: previous_response_id chain contains a cycle at %q", transformer.ErrInvalidRequest, currentID)
		}
		visited[currentID] = struct{}{}

		exchange, err := loader.LoadCompletedResponseExchange(ctx, currentID, projectID, apiKeyID)
		if err != nil {
			return nil, fmt.Errorf("failed to load previous response %q: %w", currentID, err)
		}
		if exchange == nil {
			return nil, fmt.Errorf("%w: previous response %q was not found in the current project and API-key scope", transformer.ErrInvalidRequest, currentID)
		}
		nextHistoryBytes, ok := nextPreviousResponseHistoryBytes(historyBytes, len(exchange.RequestBody), len(exchange.ResponseBody))
		if !ok {
			return nil, fmt.Errorf("%w: previous_response_id history exceeds %d bytes", transformer.ErrInvalidRequest, maxPreviousResponseHistoryBytes)
		}
		historyBytes = nextHistoryBytes

		var requestState storedResponseRequestState
		if err := json.Unmarshal(exchange.RequestBody, &requestState); err != nil {
			return nil, fmt.Errorf("%w: stored request for previous response %q is invalid: %v", transformer.ErrInvalidRequest, currentID, err)
		}

		previousRequest, err := inbound.TransformRequest(ctx, &httpclient.Request{Body: exchange.RequestBody})
		if err != nil {
			return nil, fmt.Errorf("%w: stored request for previous response %q is unavailable or invalid: %v", transformer.ErrInvalidRequest, currentID, err)
		}
		// Responses top-level instructions apply only to their own generation and
		// are not inherited through previous_response_id. The inbound transformer
		// represents them as the first system message, so omit that synthetic entry
		// while retaining explicit system/developer items from input.
		if requestState.Instructions != "" {
			if len(previousRequest.Messages) == 0 || previousRequest.Messages[0].Role != "system" {
				return nil, fmt.Errorf("%w: stored request for previous response %q has inconsistent instructions", transformer.ErrInvalidRequest, currentID)
			}
			previousRequest.Messages = previousRequest.Messages[1:]
		}

		var previousResponse responsesapi.Response
		if err := json.Unmarshal(exchange.ResponseBody, &previousResponse); err != nil {
			return nil, fmt.Errorf("%w: stored previous response %q is invalid: %v", transformer.ErrInvalidRequest, currentID, err)
		}
		if previousResponse.ID != currentID {
			return nil, fmt.Errorf("%w: stored previous response ID %q does not match requested ID %q", transformer.ErrInvalidRequest, previousResponse.ID, currentID)
		}

		responseModel := previousResponse.Model
		if responseModel == "" {
			responseModel = previousRequest.Model
		}
		syntheticBody, err := json.Marshal(struct {
			Model string              `json:"model"`
			Input []responsesapi.Item `json:"input"`
		}{Model: responseModel, Input: previousResponse.Output})
		if err != nil {
			return nil, fmt.Errorf("failed to encode previous response %q history: %w", currentID, err)
		}
		previousOutput, err := inbound.TransformRequest(ctx, &httpclient.Request{Body: syntheticBody})
		if err != nil {
			return nil, fmt.Errorf("%w: stored previous response %q output is invalid: %v", transformer.ErrInvalidRequest, currentID, err)
		}

		segmentMessages := make([]llm.Message, 0, len(previousRequest.Messages)+len(previousOutput.Messages))
		segmentMessages = append(segmentMessages, previousRequest.Messages...)
		segmentMessages = append(segmentMessages, previousOutput.Messages...)
		segments = append(segments, historySegment{messages: segmentMessages})

		currentID = ""
		if requestState.PreviousResponseID != nil {
			currentID = strings.TrimSpace(*requestState.PreviousResponseID)
		}
	}

	history := make([]llm.Message, 0)
	for i := len(segments) - 1; i >= 0; i-- {
		history = append(history, segments[i].messages...)
	}

	return history, nil
}
