package codex

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexModelsIncludeImageModels(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want []string
	}{
		{"missing", `{"models":[{"slug":"second","visibility":"list","priority":2},{"slug":"first","visibility":"list","priority":1}]}`, []string{"first", "second", "gpt-image-2", "gpt-image-2.5-sunburst", "gpt-image-2.5-flare", "gpt-image-2.6-sunburst", "gpt-image-2.6-flare"}},
		{"existing", `{"models":[{"slug":"first","visibility":"list","priority":1},{"slug":"gpt-image-2.5-flare","visibility":"list","priority":2},{"slug":"last","visibility":"list","priority":3}]}`, []string{"first", "gpt-image-2.5-flare", "last", "gpt-image-2", "gpt-image-2.5-sunburst", "gpt-image-2.6-sunburst", "gpt-image-2.6-flare"}},
		{"duplicate image", `{"models":[{"slug":"gpt-image-2.6-flare","visibility":"list","priority":1},{"slug":"first","visibility":"list","priority":2},{"slug":"gpt-image-2.6-flare","visibility":"list","priority":3}]}`, []string{"gpt-image-2.6-flare", "first", "gpt-image-2", "gpt-image-2.5-sunburst", "gpt-image-2.5-flare", "gpt-image-2.6-sunburst"}},
		{"stable order", `{"models":[{"slug":"second","visibility":"list","priority":1},{"slug":"first","visibility":"list","priority":1},{"slug":"second","visibility":"list","priority":2}]}`, []string{"second", "first", "second", "gpt-image-2", "gpt-image-2.5-sunburst", "gpt-image-2.5-flare", "gpt-image-2.6-sunburst", "gpt-image-2.6-flare"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Given a nonempty visible upstream catalog.
			body := []byte(tt.body)
			// When the catalog is parsed.
			models, err := ParseModelCatalog(body)
			// Then one image model is included without reordering other models.
			require.NoError(t, err)
			require.Equal(t, tt.want, models)
		})
	}

	t.Run("fallback", func(t *testing.T) {
		// Given the static fallback catalog, when it is requested.
		models := DefaultModels()
		// Then its original order is retained with all supported image models appended.
		require.Equal(t, []string{"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-6-astra", "gpt-6-sol", "gpt-6-luna", "gpt-6.1-sol", "codex-auto-review", "gpt-image-2", "gpt-image-2.5-sunburst", "gpt-image-2.5-flare", "gpt-image-2.6-sunburst", "gpt-image-2.6-flare"}, models)
	})

	for _, body := range []string{
		`{"models":[]}`,
		`{"models":[{"slug":"gpt-image-2","visibility":"hide"}]}`,
		`{"models":[{"slug":" ","visibility":"list"}]}`,
	} {
		t.Run("empty visible catalog "+body, func(t *testing.T) {
			// Given a catalog with no valid visible models, when it is parsed.
			models, err := ParseModelCatalog([]byte(body))
			// Then the original validation still rejects it before supplementation.
			require.EqualError(t, err, "Codex catalog has no visible models")
			require.Nil(t, models)
		})
	}
}

func TestThirdPartyModelCatalogKeepsListedSlugsRegardlessOfVisibility(t *testing.T) {
	models, err := ParseThirdPartyModelCatalog([]byte(`{"models":[{"slug":"visible","visibility":"list","priority":1},{"slug":"hidden","visibility":"hide","priority":2}]}`))

	require.NoError(t, err)
	require.Equal(t, []string{"visible", "hidden"}, models)
}
