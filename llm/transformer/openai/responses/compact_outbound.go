package responses

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// transformCompactRequest transforms a compact llm.Request to an HTTP request for the upstream provider.
func (t *OutboundTransformer) transformCompactRequest(
	ctx context.Context,
	llmReq *llm.Request,
	scope shared.TransportScope,
) (*httpclient.Request, error) {
	if llmReq.Compact == nil {
		return nil, fmt.Errorf("compact request is nil in llm.Request")
	}

	// Build the compact API request payload
	// Compact request input is always an ordered message array and must not be collapsed to plain text.
	input := convertInputFromMessages(llmReq.Compact.Input, llm.TransformOptions{ArrayInputs: lo.ToPtr(true)}, scope)

	payload := CompactAPIRequest{
		Model:          llmReq.Model,
		Input:          input,
		Instructions:   llmReq.Compact.Instructions,
		PromptCacheKey: llmReq.Compact.PromptCacheKey,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal compact request: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	fullURL := t.buildCompactURL()

	apiKey := t.config.APIKeyProvider.Get(ctx)

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     fullURL,
		Headers: headers,
		Body:    body,
		Auth: &httpclient.AuthConfig{
			Type:   "bearer",
			APIKey: apiKey,
		},
		RequestType:           string(llm.RequestTypeCompact),
		APIFormat:             string(llm.APIFormatOpenAIResponseCompact),
		SkipInboundQueryMerge: true,
		Metadata:              scope.Metadata(),
	}, nil
}

// buildCompactURL constructs the compact API URL.
func (t *OutboundTransformer) buildCompactURL() string {
	if t.config.RawURL {
		return t.config.BaseURL
	}
	return t.config.BaseURL + "/responses/compact"
}

// transformCompactResponse transforms an HTTP compact response to unified llm.Response.
func (t *OutboundTransformer) transformCompactResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp.StatusCode >= 400 {
		return nil, t.TransformError(ctx, &httpclient.Error{
			StatusCode: httpResp.StatusCode,
			Body:       httpResp.Body,
		})
	}

	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	scope, _ := shared.GetTransportScope(ctx)

	var compactResp CompactAPIResponse
	if err := json.Unmarshal(httpResp.Body, &compactResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal compact response: %w", err)
	}

	outputMessages, err := convertInputToMessages(&Input{Items: compactResp.Output})
	if err != nil {
		return nil, fmt.Errorf("failed to convert compact response output: %w", err)
	}

	for i := range outputMessages {
		outputMessages[i].ReasoningSignature = shared.EncodeOpenAIEncryptedContentInScope(outputMessages[i].ReasoningSignature, scope)
	}

	llmResp := &llm.Response{
		RequestType: llm.RequestTypeCompact,
		APIFormat:   llm.APIFormatOpenAIResponseCompact,
		ID:          compactResp.ID,
		Created:     compactResp.CreatedAt,
		Object:      "response.compaction",
		Model:       compactResp.Model,
		Compact: &llm.CompactResponse{
			ID:           compactResp.ID,
			CreatedAt:    compactResp.CreatedAt,
			Object:       "response.compaction",
			Instructions: compactResp.Instructions,
			Output:       outputMessages,
		},
	}

	if compactResp.Usage != nil {
		llmResp.Usage = compactResp.Usage.ToUsage()
	}

	return llmResp, nil
}
