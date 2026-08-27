package biz

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/samber/lo"
)

// DefaultDeveloperIDs mirrors frontend/src/features/models/data/constants.ts.
var DefaultDeveloperIDs = []string{
	"deepseek",
	"alibaba",
	"tencent",
	"zai",
	"openai",
	"moonshot",
	"anthropic",
	"google",
	"minimax",
	"kwaipilot",
	"xiaomi",
	"longcat",
	"mistral",
	"nvidia",
	"xai",
	"bytedance",
	"stepfun",
	"meta",
	"ibm",
	"poolside",
	"inclusionai",
	"thinkingmachines",
}

type catalogFile struct {
	Providers map[string]catalogProvider `json:"providers"`
}

type catalogProvider struct {
	ID          string           `json:"id,omitempty"`
	API         string           `json:"api,omitempty"`
	Name        string           `json:"name,omitempty"`
	Doc         string           `json:"doc,omitempty"`
	DisplayName string           `json:"display_name,omitempty"`
	Vision      *bool            `json:"vision,omitempty"`
	Models      []map[string]any `json:"models,omitempty"`
	Metadata    map[string]any   `json:"metadata,omitempty"`
}

func cloneProvider(in catalogProvider) catalogProvider {
	raw, err := json.Marshal(in)
	if err != nil {
		return catalogProvider{ID: in.ID, Name: in.Name, DisplayName: in.DisplayName}
	}

	var out catalogProvider
	if err := json.Unmarshal(raw, &out); err != nil {
		return catalogProvider{ID: in.ID, Name: in.Name, DisplayName: in.DisplayName}
	}

	return out
}

func cloneModel(in map[string]any) map[string]any {
	raw, err := json.Marshal(in)
	if err != nil {
		return map[string]any{}
	}

	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}

	return out
}

func modelString(model map[string]any, key string) string {
	value, _ := model[key].(string)
	return value
}

func allowedSet(ids []string) map[string]struct{} {
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}

	return out
}

func sortedStringKeys[V any](in map[string]V) []string {
	keys := lo.Keys(in)
	sort.Strings(keys)

	return keys
}

func modelsFromIDMap(merged map[string]map[string]any) []map[string]any {
	keys := sortedStringKeys(merged)
	models := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		models = append(models, merged[key])
	}

	return models
}

func filterCatalogProviders(data catalogFile, allowedIDs []string) catalogFile {
	allowed := allowedSet(allowedIDs)
	filtered := catalogFile{Providers: map[string]catalogProvider{}}

	for key, provider := range data.Providers {
		if _, ok := allowed[provider.ID]; ok {
			filtered.Providers[key] = cloneProvider(provider)
		}
	}

	if _, ok := allowed["meta"]; ok {
		mapLlamaToMeta(data, &filtered)
	}

	if _, ok := allowed["ibm"]; ok {
		aggregateIBMGranite(data, &filtered)
	}

	if _, ok := allowed["bytedance"]; ok {
		mapDoubaoToByteDance(data, &filtered)
	}

	if _, ok := allowed["tencent"]; ok {
		mergeTencentPlans(data, &filtered)
	}

	if _, ok := allowed["xiaomi"]; ok {
		mergeXiaomiTokenPlans(data, &filtered)
	}

	if _, ok := allowed["kwaipilot"]; ok {
		if provider := buildKWAIPilotProvider(data); provider != nil {
			filtered.Providers["kwaipilot"] = *provider
		}
	}

	if _, ok := allowed["nvidia"]; ok {
		filterNVIDIACreatedModels(&filtered)
	}

	if _, ok := allowed["thinkingmachines"]; ok {
		delete(filtered.Providers, "thinkingmachines")
	}

	return filtered
}

func mapLlamaToMeta(data catalogFile, filtered *catalogFile) {
	upstreamMeta, hasMeta := data.Providers["meta"]
	llama, hasLlama := data.Providers["llama"]
	if !hasMeta && !hasLlama {
		return
	}

	modelsByID := make(map[string]map[string]any)
	for _, model := range upstreamMeta.Models {
		id := modelString(model, "id")
		if id == "" {
			continue
		}
		modelsByID[id] = cloneModel(model)
	}

	for _, model := range llama.Models {
		id := strings.ToLower(modelString(model, "id"))
		if strings.HasPrefix(id, "llama") {
			originalID := modelString(model, "id")
			if _, exists := modelsByID[originalID]; !exists {
				modelsByID[originalID] = cloneModel(model)
			}
		}
	}

	if len(modelsByID) == 0 {
		return
	}

	provider := cloneProvider(upstreamMeta)
	if !hasMeta {
		provider = cloneProvider(llama)
	}
	provider.ID = "meta"
	provider.Name = "Meta"
	provider.DisplayName = "Meta"
	provider.Models = modelsFromIDMap(modelsByID)
	filtered.Providers["meta"] = provider
}

func aggregateIBMGranite(data catalogFile, filtered *catalogFile) {
	seen := map[string]struct{}{}
	models := make([]map[string]any, 0)

	for _, key := range sortedStringKeys(data.Providers) {
		provider := data.Providers[key]
		for _, model := range provider.Models {
			id := strings.ToLower(modelString(model, "id"))
			if id == "" {
				continue
			}
			if _, exists := seen[id]; exists {
				continue
			}
			if !strings.Contains(id, "granite") && !strings.Contains(id, "ibm") {
				continue
			}
			if strings.Contains(id, "embedding") || strings.Contains(id, "guardian") {
				continue
			}
			if strings.HasPrefix(id, "@cf/") || strings.Contains(id, "workers-ai/") {
				continue
			}

			seen[id] = struct{}{}
			models = append(models, cloneModel(model))
		}
	}

	if len(models) == 0 {
		return
	}

	filtered.Providers["ibm"] = catalogProvider{
		ID:          "ibm",
		Name:        "IBM",
		DisplayName: "IBM",
		Models:      models,
	}
}

func mapDoubaoToByteDance(data catalogFile, filtered *catalogFile) {
	doubao, ok := data.Providers["doubao"]
	if !ok {
		return
	}

	models := make([]map[string]any, 0)
	for _, model := range doubao.Models {
		id := strings.ToLower(modelString(model, "id"))
		if strings.HasPrefix(id, "doubao") {
			models = append(models, cloneModel(model))
		}
	}

	if len(models) == 0 {
		return
	}

	provider := cloneProvider(doubao)
	provider.ID = "bytedance"
	provider.Name = "ByteDance"
	provider.DisplayName = "ByteDance"
	provider.Models = models
	filtered.Providers["bytedance"] = provider
}

func mergeTencentPlans(data catalogFile, filtered *catalogFile) {
	merged := map[string]map[string]any{}
	base := filtered.Providers["tencent"]
	if base.ID == "" {
		base = data.Providers["tencent"]
	}

	addTencentModel := func(model map[string]any, requireFamily bool) {
		id := strings.ToLower(modelString(model, "id"))
		normalized := strings.TrimPrefix(id, "tencent/")
		if normalized == "" {
			return
		}
		if requireFamily {
			if normalized != "hy3" && !strings.HasPrefix(normalized, "hy3-") && !strings.HasPrefix(normalized, "hunyuan-") {
				return
			}
		}
		if _, exists := merged[normalized]; exists {
			return
		}
		merged[normalized] = cloneModel(model)
	}

	for _, model := range base.Models {
		addTencentModel(model, false)
	}

	for _, key := range []string{"tencent-token-plan", "tencent-tokenhub", "tencent-coding-plan"} {
		provider, ok := data.Providers[key]
		if !ok {
			continue
		}
		for _, model := range provider.Models {
			addTencentModel(model, true)
		}
	}

	if len(merged) == 0 {
		return
	}

	models := modelsFromIDMap(merged)

	provider := cloneProvider(base)
	provider.ID = "tencent"
	provider.Name = "Tencent"
	provider.DisplayName = "Tencent"
	provider.Models = models
	filtered.Providers["tencent"] = provider
}

func mergeXiaomiTokenPlans(data catalogFile, filtered *catalogFile) {
	merged := map[string]map[string]any{}
	base := filtered.Providers["xiaomi"]
	if base.ID == "" {
		base = data.Providers["xiaomi"]
	}

	for _, model := range base.Models {
		id := modelString(model, "id")
		if id == "" {
			continue
		}
		merged[id] = cloneModel(model)
	}

	for _, key := range []string{"xiaomi-token-plan-cn", "xiaomi-token-plan-sgp", "xiaomi-token-plan-ams"} {
		provider, ok := data.Providers[key]
		if !ok {
			continue
		}
		for _, model := range provider.Models {
			id := modelString(model, "id")
			if id == "" {
				continue
			}
			if _, exists := merged[id]; exists {
				continue
			}
			merged[id] = cloneModel(model)
		}
	}

	if len(merged) == 0 {
		return
	}

	models := modelsFromIDMap(merged)

	provider := cloneProvider(base)
	provider.ID = "xiaomi"
	provider.Name = "Xiaomi"
	provider.DisplayName = "Xiaomi"
	provider.Models = models
	filtered.Providers["xiaomi"] = provider
}

func normalizeKATSuffix(modelID string) string {
	trimmed := strings.TrimSpace(modelID)
	if trimmed == "" {
		return ""
	}

	if slash := strings.Index(trimmed, "/"); slash >= 0 {
		trimmed = trimmed[slash+1:]
	}

	return strings.ToLower(strings.TrimSpace(trimmed))
}

func isKATFamilyModel(model map[string]any) bool {
	modelID := modelString(model, "id")
	family := strings.ToLower(modelString(model, "family"))
	if family == "kat-coder" {
		return true
	}

	lowerID := strings.ToLower(modelID)
	if strings.HasPrefix(lowerID, "kwaipilot/") || strings.HasPrefix(lowerID, "kuaishou/") {
		return true
	}

	return strings.Contains(lowerID, "kat-coder") || strings.Contains(lowerID, "kat-dev")
}

func buildKWAIPilotProvider(data catalogFile) *catalogProvider {
	byID := map[string]map[string]any{}

	for _, key := range sortedStringKeys(data.Providers) {
		provider := data.Providers[key]
		for _, model := range provider.Models {
			if !isKATFamilyModel(model) {
				continue
			}

			normalizedID := normalizeKATSuffix(modelString(model, "id"))
			if normalizedID == "" {
				continue
			}

			cloned := cloneModel(model)
			cloned["id"] = normalizedID
			if modelString(cloned, "family") == "" {
				cloned["family"] = "kat-coder"
			}

			if existing, ok := byID[normalizedID]; ok {
				mergeDefined(existing, cloned)
				continue
			}

			byID[normalizedID] = cloned
		}
	}

	if len(byID) == 0 {
		return nil
	}

	models := modelsFromIDMap(byID)

	return &catalogProvider{
		ID:          "kwaipilot",
		Name:        "KwaiPilot",
		DisplayName: "KwaiPilot",
		Models:      models,
	}
}

func filterNVIDIACreatedModels(filtered *catalogFile) {
	provider, ok := filtered.Providers["nvidia"]
	if !ok {
		return
	}

	models := make([]map[string]any, 0, len(provider.Models))
	for _, model := range provider.Models {
		if strings.HasPrefix(strings.ToLower(modelString(model, "id")), "nvidia/") {
			models = append(models, model)
		}
	}

	if len(models) == 0 {
		return
	}

	provider.Models = models
	filtered.Providers["nvidia"] = provider
}

func mergeDefined(target, source map[string]any) {
	for key, value := range source {
		if value == nil {
			continue
		}

		existing, ok := target[key]
		if !ok || existing == nil || existing == "" {
			target[key] = value
			continue
		}

		srcMap, srcIsMap := value.(map[string]any)
		dstMap, dstIsMap := existing.(map[string]any)
		if srcIsMap && dstIsMap {
			mergeDefined(dstMap, srcMap)
		}
	}
}

func mergeExtraModels(data *catalogFile, extra []byte) error {
	if len(extra) == 0 {
		return nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(extra, &raw); err != nil {
		return err
	}

	for key, value := range raw {
		var models []map[string]any
		if err := json.Unmarshal(value, &models); err == nil {
			appendExtraModels(data, key, models)
			continue
		}

		var provider catalogProvider
		if err := json.Unmarshal(value, &provider); err != nil {
			continue
		}

		if provider.ID == "" {
			provider.ID = key
		}

		existing, ok := data.Providers[key]
		if !ok {
			data.Providers[key] = provider
			continue
		}

		existingIDs := map[string]struct{}{}
		for _, model := range existing.Models {
			existingIDs[modelString(model, "id")] = struct{}{}
		}

		for _, model := range provider.Models {
			id := modelString(model, "id")
			if id == "" {
				continue
			}
			if _, exists := existingIDs[id]; exists {
				continue
			}
			existing.Models = append(existing.Models, model)
			existingIDs[id] = struct{}{}
		}

		data.Providers[key] = existing
	}

	return nil
}

func appendExtraModels(data *catalogFile, key string, models []map[string]any) {
	existing, ok := data.Providers[key]
	if !ok {
		data.Providers[key] = catalogProvider{ID: key, Models: models}
		return
	}

	existingIDs := map[string]struct{}{}
	for _, model := range existing.Models {
		existingIDs[modelString(model, "id")] = struct{}{}
	}

	for _, model := range models {
		id := modelString(model, "id")
		if id == "" {
			continue
		}
		if _, exists := existingIDs[id]; exists {
			continue
		}
		existing.Models = append(existing.Models, model)
		existingIDs[id] = struct{}{}
	}

	data.Providers[key] = existing
}

func sortCatalogModels(data *catalogFile) {
	for key, provider := range data.Providers {
		sortModelsByReleaseDate(provider.Models)
		data.Providers[key] = provider
	}
}

func sortModelsByReleaseDate(models []map[string]any) {
	sort.SliceStable(models, func(i, j int) bool {
		left, leftErr := time.Parse("2006-01-02", modelString(models[i], "release_date"))
		right, rightErr := time.Parse("2006-01-02", modelString(models[j], "release_date"))
		if leftErr != nil {
			left = time.Time{}
		}
		if rightErr != nil {
			right = time.Time{}
		}

		if !left.Equal(right) {
			return left.After(right)
		}

		return modelString(models[i], "id") < modelString(models[j], "id")
	})
}
