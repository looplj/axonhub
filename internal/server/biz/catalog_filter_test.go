package biz

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilterCatalogProviders(t *testing.T) {
	t.Parallel()

	input := catalogFile{Providers: map[string]catalogProvider{
		"openai": {
			ID: "openai",
			Models: []map[string]any{
				{"id": "gpt-5.6-luna", "release_date": "2026-03-01"},
			},
		},
		"unknown-lab": {
			ID: "unknown-lab",
			Models: []map[string]any{
				{"id": "secret-model"},
			},
		},
		"llama": {
			ID: "llama",
			Models: []map[string]any{
				{"id": "llama-4"},
				{"id": "other-7b"},
			},
		},
		"nvidia": {
			ID: "nvidia",
			Models: []map[string]any{
				{"id": "nvidia/nemotron"},
				{"id": "meta/llama-hosted"},
			},
		},
		"some-host": {
			ID: "some-host",
			Models: []map[string]any{
				{"id": "kwaipilot/kat-coder-pro", "family": "kat-coder"},
			},
		},
		"thinkingmachines": {
			ID: "thinkingmachines",
			Models: []map[string]any{
				{"id": "tinker-only"},
			},
		},
	}}

	filtered := filterCatalogProviders(input, DefaultDeveloperIDs)
	require.Contains(t, filtered.Providers, "openai")
	require.NotContains(t, filtered.Providers, "unknown-lab")
	require.Equal(t, "meta", filtered.Providers["meta"].ID)
	require.Equal(t, "llama-4", filtered.Providers["meta"].Models[0]["id"])
	require.Len(t, filtered.Providers["nvidia"].Models, 1)
	require.Equal(t, "nvidia/nemotron", filtered.Providers["nvidia"].Models[0]["id"])
	require.Equal(t, "kat-coder-pro", filtered.Providers["kwaipilot"].Models[0]["id"])
	require.NotContains(t, filtered.Providers, "thinkingmachines")

	extra := []byte(`{"thinkingmachines":[{"id":"tm-canonical","release_date":"2026-01-01"}]}`)
	require.NoError(t, mergeExtraModels(&filtered, extra))
	require.Equal(t, "tm-canonical", filtered.Providers["thinkingmachines"].Models[0]["id"])
}

func TestFilterCatalogProviders_MergesUpstreamMetaAndLlama(t *testing.T) {
	t.Parallel()

	input := catalogFile{Providers: map[string]catalogProvider{
		"meta": {
			ID: "meta",
			Models: []map[string]any{
				{"id": "muse-1", "source": "meta"},
				{"id": "llama-4", "source": "meta"},
			},
		},
		"llama": {
			ID: "llama",
			Models: []map[string]any{
				{"id": "llama-4", "source": "llama"},
				{"id": "llama-5", "source": "llama"},
				{"id": "other-7b"},
			},
		},
	}}

	filtered := filterCatalogProviders(input, DefaultDeveloperIDs)
	meta := filtered.Providers["meta"]
	require.Equal(t, []string{"llama-4", "llama-5", "muse-1"}, modelIDs(meta.Models))
	require.Equal(t, "meta", meta.Models[0]["source"])
	require.Equal(t, "llama", meta.Models[1]["source"])
}

func TestValidateCatalogSettings(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateCatalogSettings(CatalogSettings{}))
	require.Error(t, validateCatalogSettings(CatalogSettings{UpstreamURL: "ftp://x"}))
	require.Error(t, validateCatalogSettings(CatalogSettings{UpstreamURL: "not-a-url"}))
	require.NoError(t, validateCatalogSettings(CatalogSettings{UpstreamURL: DefaultCatalogUpstreamURL}))
}

func TestCatalogFallbackJSONParses(t *testing.T) {
	t.Parallel()

	var data catalogFile
	require.NoError(t, json.Unmarshal(catalogFallbackJSON, &data))
	require.NotEmpty(t, data.Providers)
}

func TestFilterCatalogProviders_DeterministicOrder(t *testing.T) {
	t.Parallel()

	input := catalogFile{Providers: map[string]catalogProvider{
		"tencent": {
			ID: "tencent",
			Models: []map[string]any{
				{"id": "hunyuan-b", "release_date": "2026-01-01"},
				{"id": "hunyuan-a", "release_date": "2026-01-01"},
			},
		},
		"tencent-token-plan": {
			ID: "tencent-token-plan",
			Models: []map[string]any{
				{"id": "hunyuan-c", "release_date": "2026-01-01"},
			},
		},
		"xiaomi": {
			ID: "xiaomi",
			Models: []map[string]any{
				{"id": "mimo-b", "release_date": "2026-02-01"},
				{"id": "mimo-a", "release_date": "2026-02-01"},
			},
		},
		"host-b": {
			ID: "host-b",
			Models: []map[string]any{
				{"id": "kwaipilot/kat-coder-pro", "family": "kat-coder", "release_date": "2026-01-01"},
			},
		},
		"host-a": {
			ID: "host-a",
			Models: []map[string]any{
				{"id": "kuaishou/kat-dev", "family": "kat-coder", "release_date": "2026-01-01"},
			},
		},
		"ibm-b": {
			ID: "ibm-b",
			Models: []map[string]any{
				{"id": "granite-y", "release_date": "2025-01-01"},
			},
		},
		"ibm-a": {
			ID: "ibm-a",
			Models: []map[string]any{
				{"id": "granite-x", "release_date": "2025-01-01"},
			},
		},
	}}

	var encoded string
	var filtered catalogFile
	for i := range 20 {
		filtered = filterCatalogProviders(input, DefaultDeveloperIDs)
		sortCatalogModels(&filtered)
		raw, err := json.Marshal(filtered)
		require.NoError(t, err)
		if i == 0 {
			encoded = string(raw)
			continue
		}
		require.Equal(t, encoded, string(raw))
	}

	require.Equal(t, []string{"hunyuan-a", "hunyuan-b", "hunyuan-c"}, modelIDs(filtered.Providers["tencent"].Models))
	require.Equal(t, []string{"mimo-a", "mimo-b"}, modelIDs(filtered.Providers["xiaomi"].Models))
	require.Equal(t, []string{"kat-coder-pro", "kat-dev"}, modelIDs(filtered.Providers["kwaipilot"].Models))
	require.Equal(t, []string{"granite-x", "granite-y"}, modelIDs(filtered.Providers["ibm"].Models))
}

func TestSortModelsByReleaseDate_UsesIDTieBreaker(t *testing.T) {
	t.Parallel()

	models := []map[string]any{
		{"id": "b", "release_date": "2026-01-01"},
		{"id": "a", "release_date": "2026-01-01"},
		{"id": "c", "release_date": "2026-02-01"},
	}
	sortModelsByReleaseDate(models)
	require.Equal(t, []string{"c", "a", "b"}, modelIDs(models))
}

func TestDefaultDeveloperIDsMatchFrontendConstants(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join(repoRoot(t), "frontend", "src", "features", "models", "data", "constants.ts"))
	require.NoError(t, err)

	match := regexp.MustCompile(`export const DEVELOPER_IDS = \[([\s\S]*?)\]`).FindSubmatch(content)
	require.NotNil(t, match)

	var frontend []string
	for _, idMatch := range regexp.MustCompile(`['"]([^'"]+)['"]`).FindAllSubmatch(match[1], -1) {
		frontend = append(frontend, string(idMatch[1]))
	}

	require.Equal(t, frontend, DefaultDeveloperIDs)
}

func modelIDs(models []map[string]any) []string {
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, modelString(model, "id"))
	}

	return ids
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "frontend")); err == nil {
				return dir
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}
