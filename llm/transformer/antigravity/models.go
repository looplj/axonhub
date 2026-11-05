package antigravity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
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
		// Prefer an exact upstream tier before applying compatibility aliases.
		if id := variants[effort]; id != "" {
			return id, nil
		}
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
			// Preserve legacy handling only when it produces an offered wire ID.
			// New catalog-only model families may have no bare upstream model.
			legacy := transformModelForAntigravity(model)
			if _, ok := c[legacy]; ok {
				return legacy, nil
			}
			return "", fmt.Errorf("Antigravity model %q does not offer reasoning effort %q", base, effort)
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
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if _, suffix := SplitReasoningModel(NormalizeModelName(model)); suffix != "" && effort == "" {
		return transformModelForAntigravity(model), nil
	}

	// A failed refresh is cached too: explicit efforts must not silently fall
	// back to a different effort while discovery is in its retry cooldown.
	load := func() (ModelCatalog, error, bool) {
		t.modelCatalogMu.Lock()
		defer t.modelCatalogMu.Unlock()
		if catalogVersion == t.modelCatalogVersion && time.Now().Before(t.modelCatalogExpiresAt) {
			return t.modelCatalog, t.modelCatalogError, true
		}
		return nil, nil, false
	}
	catalog, fetchErr, cached := load()
	if !cached {
		// Sharing is credential-specific. The bounded refresh outlives any one
		// caller, while every waiter can leave immediately on cancellation.
		result := t.modelCatalogRefresh.DoChan(strconv.FormatUint(catalogVersion, 10), func() (any, error) {
			if catalog, err, ok := load(); ok {
				return catalog, err
			}
			t.modelCatalogMu.Lock()
			client, project := t.httpClient, t.config.Project
			t.modelCatalogMu.Unlock()
			if client == nil {
				client = httpclient.NewHttpClientWithClient(&http.Client{Timeout: tokenExchangeTimeout})
			}
			fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			fetched, err := FetchModelCatalog(fetchCtx, client, accessToken, project)

			t.modelCatalogMu.Lock()
			defer t.modelCatalogMu.Unlock()
			if catalogVersion != t.modelCatalogVersion {
				// An old request may use its own result, but must neither read nor
				// overwrite the new credential's catalog.
				return fetched, err
			}
			t.modelCatalogError = err
			if err == nil {
				t.modelCatalog = fetched
				t.modelCatalogExpiresAt = time.Now().Add(5 * time.Minute)
			} else {
				t.modelCatalogExpiresAt = time.Now().Add(10 * time.Second)
			}
			return t.modelCatalog, err
		})
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case resolved := <-result:
			catalog, _ = resolved.Val.(ModelCatalog)
			fetchErr = resolved.Err
		}
	}

	if catalog == nil {
		if effort != "" && fetchErr != nil {
			return "", fmt.Errorf("resolve Antigravity reasoning effort: %w", fetchErr)
		}
		return transformModelForAntigravity(model), nil
	}
	return catalog.Resolve(model, effort)
}
