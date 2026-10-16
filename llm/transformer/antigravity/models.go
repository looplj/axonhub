package antigravity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/looplj/axonhub/llm/httpclient"
)

// ModelCatalog contains the public wire IDs offered to this OAuth account.
type ModelCatalog map[string]struct{}

// SplitReasoningModel separates an effort suffix without changing other model
// qualifiers such as -thinking, -image or -tiered.
func SplitReasoningModel(model string) (string, string) {
	for _, effort := range []string{"extra-low", "minimal", "xhigh", "medium", "high", "low"} {
		if base, ok := strings.CutSuffix(model, "-"+effort); ok && base != "" {
			return base, effort
		}
	}
	return model, ""
}

func (c ModelCatalog) BaseModels() []string {
	unique := make(map[string]struct{}, len(c))
	for id := range c {
		base, _ := SplitReasoningModel(id)
		unique[base] = struct{}{}
	}
	models := make([]string, 0, len(unique))
	for id := range unique {
		models = append(models, id)
	}
	sort.Strings(models)
	return models
}

// Resolve applies the explicit request effort first, then a legacy model suffix,
// then a stable default. It never invents a variant absent from this account.
func (c ModelCatalog) Resolve(model, effort string) (string, error) {
	model = NormalizeModelName(model)
	base, suffix := SplitReasoningModel(model)
	effort = strings.ToLower(strings.TrimSpace(effort))
	if effort == "" {
		if _, ok := c[model]; ok {
			return model, nil
		}
		if suffix != "" {
			return model, nil
		}
	}
	variants := make(map[string]string)
	for id := range c {
		candidate, level := SplitReasoningModel(id)
		if candidate == base && level != "" {
			variants[level] = id
		}
	}
	if len(variants) == 0 {
		return transformModelForAntigravity(model), nil
	}
	if effort != "" {
		switch effort {
		case "xhigh", "max":
			effort = "high"
		case "minimal":
			if variants["extra-low"] != "" {
				effort = "extra-low"
			} else {
				effort = "low"
			}
		}
		if id := variants[effort]; id != "" {
			return id, nil
		}
		switch effort {
		case "extra-low", "low", "medium", "high":
			return "", fmt.Errorf("Antigravity model %q does not offer reasoning effort %q", base, effort)
		default:
			return transformModelForAntigravity(model), nil
		}
	}
	defaults := []string{"medium", "low", "high", "extra-low", "minimal", "xhigh"}
	if strings.HasPrefix(base, "gemini-3") {
		defaults[0], defaults[1] = defaults[1], defaults[0]
	}
	for _, level := range defaults {
		if id := variants[level]; id != "" {
			return id, nil
		}
	}
	return model, nil
}

func FetchModelCatalog(ctx context.Context, client *httpclient.HttpClient, accessToken, project string) (ModelCatalog, error) {
	body := make(map[string]string)
	if project != "" {
		body["project"] = project
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode Antigravity model request: %w", err)
	}
	headers := http.Header{"Content-Type": {"application/json"}, "Accept": {"application/json"}}
	SetClientHeaders(headers)
	headers.Set("Authorization", "Bearer "+accessToken)
	response, err := client.Do(ctx, &httpclient.Request{Method: http.MethodPost, URL: EndpointDaily + "/v1internal:fetchAvailableModels", Headers: headers, Body: encoded})
	if err != nil {
		return nil, fmt.Errorf("fetch Antigravity model catalog: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch Antigravity model catalog: status %d", response.StatusCode)
	}
	var payload struct {
		Models map[string]struct {
			IsInternal bool `json:"isInternal"`
		} `json:"models"`
	}
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		return nil, fmt.Errorf("parse Antigravity model catalog: %w", err)
	}
	models := make(ModelCatalog)
	for id, model := range payload.Models {
		if strings.TrimSpace(id) != "" && !model.IsInternal {
			models[id] = struct{}{}
		}
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("Antigravity model catalog contains no available public models")
	}
	return models, nil
}

// The cache belongs to the transformer/credential, never to all accounts. No
// serialized cache format changes are involved, and SetAPIKey invalidates it.
func (t *Transformer) resolveReasoningModel(ctx context.Context, model, effort, accessToken string, catalogVersion uint64) (string, error) {
	if _, suffix := SplitReasoningModel(NormalizeModelName(model)); suffix != "" && effort == "" {
		return transformModelForAntigravity(model), nil
	}

	t.modelCatalogMu.Lock()
	catalog := t.modelCatalog
	expiresAt := t.modelCatalogExpiresAt
	currentVersion := t.modelCatalogVersion
	needsFetch := catalogVersion != currentVersion || time.Now().After(expiresAt)
	client := t.httpClient
	project := t.config.Project
	t.modelCatalogMu.Unlock()

	if needsFetch {
		if client == nil {
			client = httpclient.NewHttpClientWithClient(&http.Client{Timeout: tokenExchangeTimeout})
		}
		fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		fetched, err := FetchModelCatalog(fetchCtx, client, accessToken, project)
		cancel()

		t.modelCatalogMu.Lock()
		if err == nil {
			if catalogVersion == t.modelCatalogVersion {
				t.modelCatalog = fetched
				t.modelCatalogExpiresAt = time.Now().Add(5 * time.Minute)
				catalog = fetched
			}
		} else {
			if ctx.Err() != nil {
				t.modelCatalogMu.Unlock()
				return "", ctx.Err()
			}
			if catalogVersion == t.modelCatalogVersion {
				catalog = t.modelCatalog
				t.modelCatalogExpiresAt = time.Now().Add(10 * time.Second)
			} else {
				catalog = nil
			}
			if catalog == nil && effort != "" {
				t.modelCatalogMu.Unlock()
				return "", fmt.Errorf("resolve Antigravity reasoning effort: %w", err)
			}
		}
		if catalog == nil && catalogVersion == t.modelCatalogVersion {
			catalog = t.modelCatalog
		}
		t.modelCatalogMu.Unlock()
	}

	if catalog == nil {
		return transformModelForAntigravity(model), nil
	}
	return catalog.Resolve(model, effort)
}
