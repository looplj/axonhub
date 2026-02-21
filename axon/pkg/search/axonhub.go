package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type AxonHubProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewAxonHubProvider(baseURL, apiKey string) *AxonHubProvider {
	baseURL = strings.TrimRight(baseURL, "/")

	return &AxonHubProvider{
		baseURL: baseURL,
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *AxonHubProvider) Search(ctx context.Context, req Request) (Response, error) {
	if p.baseURL == "" {
		return Response{}, fmt.Errorf("web search is not configured: base_url is empty")
	}

	if p.apiKey == "" {
		return Response{}, fmt.Errorf("web search is not configured: api_key is empty")
	}

	if strings.TrimSpace(req.Query) == "" {
		return Response{}, fmt.Errorf("query is required")
	}

	payload := map[string]any{
		"query": req.Query,
		"model": "__search",
	}
	if req.MaxResults != nil {
		payload["max_results"] = *req.MaxResults
	}

	if len(req.AllowedDomains) > 0 {
		payload["allowed_domains"] = req.AllowedDomains
	}

	if len(req.BlockedDomains) > 0 {
		payload["blocked_domains"] = req.BlockedDomains
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/search", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("failed to build request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("web search request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("failed to read web search response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = resp.Status
		}

		return Response{}, fmt.Errorf("web search failed (%s): %s", resp.Status, msg)
	}

	var out Response
	if err := json.Unmarshal(respBody, &out); err != nil {
		return Response{}, fmt.Errorf("failed to decode web search response: %w", err)
	}

	return out, nil
}
