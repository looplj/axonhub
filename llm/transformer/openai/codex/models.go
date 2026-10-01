package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
)

// SupportsModelCatalog reports whether the base URL uses the official Codex catalog.
func SupportsModelCatalog(baseURL string) bool {
	if baseURL == "" || baseURL == "https://api.openai.com/v1" {
		return true
	}
	parsed, err := url.Parse(strings.TrimRight(baseURL, "#"))
	return err == nil && parsed.Scheme == "https" && parsed.Host == "chatgpt.com" &&
		strings.TrimRight(parsed.Path, "/") == "/backend-api/codex"
}

// ModelsRequest uses the same OAuth provider and client version as inference.
func ModelsRequest(ctx context.Context, tokens oauth.TokenGetter, baseURL string) (*httpclient.Request, error) {
	creds, err := tokens.Get(ctx)
	if err != nil {
		return nil, err
	}
	if baseURL == "" || baseURL == "https://api.openai.com/v1" {
		baseURL = codexBaseURL
	}
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer "+creds.AccessToken)
	headers.Set("Accept", "application/json")
	headers.Set("Version", codexDefaultVersion)
	headers.Set("Originator", AxonHubOriginator)
	headers.Set(BetaFeaturesHeader, fabricatedBetaFeatures)
	if accountID := ExtractChatGPTAccountIDFromJWT(creds.AccessToken); accountID != "" {
		headers.Set("Chatgpt-Account-Id", accountID)
	}
	return &httpclient.Request{
		Method:  http.MethodGet,
		URL:     strings.TrimRight(baseURL, "/#") + "/models?client_version=" + codexDefaultVersion,
		Headers: headers,
	}, nil
}

// ParseModelCatalog returns only picker-visible models in upstream priority order.
func ParseModelCatalog(body []byte) ([]string, error) {
	var response struct {
		Models []struct {
			Slug       string `json:"slug"`
			Visibility string `json:"visibility"`
			Priority   int    `json:"priority"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	sort.SliceStable(response.Models, func(i, j int) bool {
		return response.Models[i].Priority < response.Models[j].Priority
	})
	models := make([]string, 0, len(response.Models))
	for _, model := range response.Models {
		if model.Visibility == "list" && strings.TrimSpace(model.Slug) != "" {
			models = append(models, model.Slug)
		}
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("Codex catalog has no visible models")
	}
	return models, nil
}
