package plugin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestValidateExports(t *testing.T) {
	events := &objects.PluginManifest{Events: []string{"request.failover"}}

	t.Run("missing named export", func(t *testing.T) {
		exports := evaluatedExports(t, map[string]string{
			"plugin.js": "export function onRequest() {}",
		})
		err := ValidateExports(events, exports)
		require.ErrorContains(t, err, "must export onEvent")
	})

	t.Run("default object only", func(t *testing.T) {
		exports := evaluatedExports(t, map[string]string{
			"plugin.js": "export default { onEvent() {} }",
		})
		_, ok := exports["onEvent"]
		require.False(t, ok)
		require.False(t, exports["default"].Function)
		err := ValidateExports(events, exports)
		require.ErrorContains(t, err, "must export onEvent")
	})

	t.Run("non-function export", func(t *testing.T) {
		exports := evaluatedExports(t, map[string]string{
			"plugin.js": "export const onEvent = 1",
		})
		require.Contains(t, exports, "onEvent")
		require.False(t, exports["onEvent"].Function)
		err := ValidateExports(events, exports)
		require.ErrorContains(t, err, "onEvent")
		require.ErrorContains(t, err, "must be a function")
	})

	t.Run("async export", func(t *testing.T) {
		exports := evaluatedExports(t, map[string]string{
			"plugin.js": "export async function onEvent() {}",
		})
		require.True(t, exports["onEvent"].Function)
		require.True(t, exports["onEvent"].Async)
		require.NoError(t, ValidateExports(events, exports))
	})

	t.Run("top-level await", func(t *testing.T) {
		_, err := evaluateSources(map[string]string{
			"plugin.js": "await Promise.resolve(1)\nexport function onEvent() {}\n",
		})
		require.ErrorContains(t, err, "plugin.js: top-level await is not allowed")

		_, err = evaluateSources(map[string]string{
			"plugin.js":   "import \"./lib/wait.js\"\nexport function onEvent() {}\n",
			"lib/wait.js": "await Promise.resolve(1)\nexport const ready = true\n",
		})
		require.ErrorContains(t, err, "lib/wait.js: top-level await is not allowed")
	})

	t.Run("valid named export", func(t *testing.T) {
		exports := evaluatedExports(t, map[string]string{
			"plugin.js": "export function onEvent() { return 1 }",
		})
		require.True(t, exports["onEvent"].Function)
		require.False(t, exports["onEvent"].Async)
		require.NoError(t, ValidateExports(events, exports))
	})

	t.Run("relative in-package import", func(t *testing.T) {
		exports := evaluatedExports(t, map[string]string{
			"plugin.js":      "import { inc } from \"./lib/counter.js\"\nexport function onEvent() { return inc(1) }\n",
			"lib/counter.js": "export function inc(n) { return n + 1 }\n",
		})
		require.True(t, exports["onEvent"].Function)
		require.False(t, exports["onEvent"].Async)
		require.NoError(t, ValidateExports(events, exports))
	})

	t.Run("bare import", func(t *testing.T) {
		_, err := evaluateSources(map[string]string{
			"plugin.js": "import lodash from \"lodash\"\nexport function onEvent() {}",
		})
		require.ErrorContains(t, err, "lodash")
		require.ErrorContains(t, err, "package-relative")
	})

	t.Run("missing plugin.js", func(t *testing.T) {
		err := ValidateExports(events, nil)
		require.ErrorContains(t, err, "plugin.js is required")
	})

	t.Run("request and attempt hooks", func(t *testing.T) {
		manifest := &objects.PluginManifest{Hooks: &objects.PluginHooksSpec{Request: &objects.PluginRequestHook{Enabled: true, RequestTypes: []llm.RequestType{llm.RequestTypeChat}}, Attempt: &objects.PluginAttemptHook{Enabled: true, APIFormats: []llm.APIFormat{llm.APIFormatOpenAIChatCompletion}}}}
		exports := evaluatedExports(t, map[string]string{
			"plugin.js": "export function onRequest() {}\nexport function onAttempt() {}\n",
		})
		require.NoError(t, ValidateExports(manifest, exports))

		err := ValidateExports(manifest, map[string]ExportFunc{
			"onRequest": {Function: true},
		})
		require.ErrorContains(t, err, "must export onAttempt")

		err = ValidateExports(&objects.PluginManifest{Hooks: &objects.PluginHooksSpec{Request: &objects.PluginRequestHook{Enabled: true, RequestTypes: []llm.RequestType{llm.RequestTypeChat}}}}, nil)
		require.ErrorContains(t, err, "plugin.js is required")
	})

	t.Run("unused async export is not required", func(t *testing.T) {
		exports := evaluatedExports(t, map[string]string{
			"plugin.js": "export async function helper() {}\nexport function onEvent() {}",
		})
		require.True(t, exports["helper"].Async)
		require.NoError(t, ValidateExports(events, exports))
		require.NoError(t, ValidateExports(&objects.PluginManifest{}, exports))
	})
}

func TestPluginTopLevelTimeout(t *testing.T) {
	raw := mustZip(t, map[string]string{
		"manifest.json": manifestWithoutUI(t),
		"plugin.js":     "for (;;) {}\nexport function onEvent() {}\n",
	})
	started := time.Now()
	_, err := ParsePackage(raw)
	elapsed := time.Since(started)
	require.ErrorContains(t, err, "exceeded 1s")
	require.Less(t, elapsed, 5*time.Second)
}

func TestPluginExportGetterTimeout(t *testing.T) {
	_, err := evaluateSources(map[string]string{"plugin.js": `
export function onEvent() {}
Object.defineProperty(onEvent, "constructor", {get() { for (;;) {} }});
`})
	require.ErrorIs(t, err, errPluginTopLevelTimeout)
}

func evaluatedExports(t *testing.T, sources map[string]string) map[string]ExportFunc {
	t.Helper()
	exports, err := evaluateSources(sources)
	require.NoError(t, err)
	return exports
}
