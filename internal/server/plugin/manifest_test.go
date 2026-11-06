package plugin

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/build"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/events"
)

const validManifestJSON = `{
  "schemaVersion": 1,
  "id": "acme.failover-guard",
  "name": "Failover Guard",
  "version": "1.0.0",
  "description": "Notify and disable channels on failover storms",
  "axonhubVersion": "^1.0.0",
  "pluginApiVersion": 1,
  "permissions": ["read_channels", "webhook"],
  "hooks": {"request": false, "attempt": false},
  "configSchema": {
    "type": "object",
    "properties": {
      "threshold": {"type": "integer", "title": "Threshold", "minimum": 1, "default": 10},
      "target": {"type": "string", "title": "Webhook target"},
      "token": {"type": "string", "format": "secret"},
      "my.custom-prop": {"type": "boolean", "description": "custom flag"}
    }
  },
  "ui": {
    "pages": [
      {"key": "main", "entry": "ui/page.html", "title": {"en": "Failover Guard", "zh-CN": "故障转移守卫"}}
    ]
  }
}`

func TestManifest(t *testing.T) {
	t.Run("accepts opened fields", func(t *testing.T) {
		manifest, err := ParseManifest([]byte(validManifestJSON), "v1.0.0-beta9")
		require.NoError(t, err)
		require.Equal(t, "acme.failover-guard", manifest.ID)
		require.False(t, manifest.Hooks.Request)
		require.False(t, manifest.Hooks.Attempt)
		require.Contains(t, manifest.ConfigSchema.Properties, "my.custom-prop")
	})

	t.Run("rejects name, version, and description that exceed the column width", func(t *testing.T) {
		raw := mutateManifest(t, func(_ *testing.T, manifest map[string]any) {
			manifest["name"] = strings.Repeat("n", objects.MaxPluginNameLength+1)
		})
		_, err := ParseManifest(raw, "v1.0.0-beta9")
		require.ErrorContains(t, err, "name must be at most")

		raw = mutateManifest(t, func(_ *testing.T, manifest map[string]any) {
			manifest["version"] = "1.0.0-" + strings.Repeat("a", objects.MaxPluginVersionLength)
		})
		_, err = ParseManifest(raw, "v1.0.0-beta9")
		require.ErrorContains(t, err, "version must be at most")

		raw = mutateManifest(t, func(_ *testing.T, manifest map[string]any) {
			manifest["description"] = strings.Repeat("d", objects.MaxPluginDescriptionLength+1)
		})
		_, err = ParseManifest(raw, "v1.0.0-beta9")
		require.ErrorContains(t, err, "description must be at most")
	})

	t.Run("accepts the open event types", func(t *testing.T) {
		raw := mutateManifest(t, func(_ *testing.T, manifest map[string]any) {
			manifest["events"] = []any{}
		})
		manifest, err := ParseManifest(raw, "v1.0.0-beta9")
		require.NoError(t, err)
		require.True(t, manifest.EventsPresent)
		require.Empty(t, manifest.Events)

		names := make([]any, 0, len(events.OpenEventTypes()))
		for _, name := range events.OpenEventTypes() {
			names = append(names, name)
		}
		raw = mutateManifest(t, func(_ *testing.T, manifest map[string]any) {
			manifest["events"] = names
		})
		manifest, err = ParseManifest(raw, "v1.0.0-beta9")
		require.NoError(t, err)
		require.Equal(t, events.OpenEventTypes(), manifest.Events)
	})

	t.Run("accepts every opened permission", func(t *testing.T) {
		raw := mutateManifest(t, func(_ *testing.T, manifest map[string]any) {
			manifest["permissions"] = []any{
				"read_channels",
				"read_requests",
				"read_dashboard",
				"read_api_keys",
				"read_projects",
				"read_settings",
				"write_channels",
				"webhook",
			}
		})
		_, err := ParseManifest(raw, "v1.0.0-beta9")
		require.NoError(t, err)
	})

	t.Run("omitted optional fields use defaults", func(t *testing.T) {
		raw := mutateManifest(t, func(_ *testing.T, manifest map[string]any) {
			delete(manifest, "permissions")
			delete(manifest, "hooks")
			delete(manifest, "description")
			delete(manifest, "configSchema")
			delete(manifest, "ui")
		})
		manifest, err := ParseManifest(raw, "v1.0.0-beta9")
		require.NoError(t, err)
		require.Empty(t, manifest.Permissions)
		require.Empty(t, manifest.Network)
		require.Nil(t, manifest.Hooks)
		require.Empty(t, manifest.Description)
		require.Nil(t, manifest.ConfigSchema)
		require.Nil(t, manifest.UI)
	})

	t.Run("current build satisfies caret 1.0.0", func(t *testing.T) {
		_, err := ParseManifest([]byte(validManifestJSON), build.Version)
		require.NoError(t, err)
	})

	t.Run("title round trip", func(t *testing.T) {
		samples := []string{
			`"Overview"`,
			`{"en":"Overview","zh-CN":"概览"}`,
			`{"zh-CN":"概览","en":"Overview","fr":"Aperçu"}`,
		}
		for _, sample := range samples {
			var title objects.PluginTitle
			require.NoError(t, json.Unmarshal([]byte(sample), &title), sample)
			require.NoError(t, title.Validate(), sample)
			out, err := json.Marshal(title)
			require.NoError(t, err, sample)
			require.Equal(t, sample, string(out), sample)
		}

		raw := mutateManifest(t, func(t *testing.T, manifest map[string]any) {
			page0(t, manifest)["title"] = "Overview"
		})
		manifest, err := ParseManifest(raw, "v1.0.0-beta9")
		require.NoError(t, err)
		encoded, err := json.Marshal(manifest.UI.Pages[0].Title)
		require.NoError(t, err)
		require.Equal(t, `"Overview"`, string(encoded))
	})

	cases := []struct {
		name    string
		runtime string
		mutate  func(t *testing.T, manifest map[string]any)
		want    string
		notWant string
	}{
		{
			name: "schemaVersion 2",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["schemaVersion"] = 2
			},
			want: "schemaVersion must be 1",
		},
		{
			name: "schemaVersion missing",
			mutate: func(_ *testing.T, manifest map[string]any) {
				delete(manifest, "schemaVersion")
			},
			want: "schemaVersion is required",
		},
		{
			name: "schemaVersion string",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["schemaVersion"] = "1"
			},
			want: "schemaVersion must be an integer",
		},
		{
			name: "id without dot",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["id"] = "acme"
			},
			want: "id",
		},
		{
			name: "id uppercase",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["id"] = "Acme.tool"
			},
			want: "id",
		},
		{
			name: "id too long",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["id"] = "a." + strings.Repeat("b", 63)
			},
			want: "id",
		},
		{
			name: "id length 64",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["id"] = "a." + strings.Repeat("b", 62)
			},
		},
		{
			name: "reserved builtin id",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["id"] = "axonhub.webhook"
			},
			want: `plugin id "axonhub.webhook" is reserved`,
		},
		{
			name: "name empty",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["name"] = ""
			},
			want: "name is required",
		},
		{
			name: "name missing",
			mutate: func(_ *testing.T, manifest map[string]any) {
				delete(manifest, "name")
			},
			want: "name is required",
		},
		{
			name: "version not semver",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["version"] = "latest"
			},
			want: "version",
		},
		{
			name: "description must be a string",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["description"] = 1
			},
			want: "description must be a string",
		},
		{
			name: "axonhubVersion invalid range",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["axonhubVersion"] = ">>>"
			},
			want: "axonhubVersion",
		},
		{
			name:    "caret passes prerelease runtime",
			runtime: "v1.0.0-beta9",
		},
		{
			name:    "caret rejects 0.9.0",
			runtime: "0.9.0",
			want:    "0.9.0",
		},
		{
			name:    "caret rejects 2.0.0",
			runtime: "2.0.0",
			want:    "2.0.0",
		},
		{
			name: "plugin API version 2",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["pluginApiVersion"] = 2
			},
			want: "plugin API version 2 is not supported",
		},
		{
			name: "plugin API version 0",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["pluginApiVersion"] = 0
			},
			want: "plugin API version 0 is not supported",
		},
		{
			name: "plugin API version fractional",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["pluginApiVersion"] = 1.5
			},
			want: "pluginApiVersion must be an integer",
		},
		{
			name: "fetch is closed",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["permissions"] = []any{"fetch"}
			},
			want: "fetch is not enabled",
		},
		{
			name: "read_plugins is not a plugin permission",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["permissions"] = []any{"read_plugins"}
			},
			want:    `permission "read_plugins" is not allowed`,
			notWant: "fetch is not enabled",
		},
		{
			name: "write_plugins is not a plugin permission",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["permissions"] = []any{"write_plugins"}
			},
			want: `permission "write_plugins" is not allowed`,
		},
		{
			name: "configSchema type must be object",
			mutate: func(t *testing.T, manifest map[string]any) {
				schema := manifest["configSchema"].(map[string]any)
				schema["type"] = "array"
			},
			want: "configSchema.type must be object",
		},
		{
			name: "config property type is closed",
			mutate: func(t *testing.T, manifest map[string]any) {
				propertiesOf(t, manifest)["nested"] = map[string]any{"type": "object"}
			},
			want: "must be string, integer, number, or boolean",
		},
		{
			name: "config format must be secret",
			mutate: func(t *testing.T, manifest map[string]any) {
				token := propertiesOf(t, manifest)["token"].(map[string]any)
				token["format"] = "email"
			},
			want: "format must be secret",
		},
		{
			name: "unknown config schema keyword",
			mutate: func(t *testing.T, manifest map[string]any) {
				manifest["configSchema"].(map[string]any)["additionalProperties"] = false
			},
			want: "unknown field configSchema.additionalProperties",
		},
		{
			name: "unknown config property keyword",
			mutate: func(t *testing.T, manifest map[string]any) {
				threshold := propertiesOf(t, manifest)["threshold"].(map[string]any)
				threshold["pattern"] = "^x"
			},
			want: "unknown field configSchema.properties.threshold.pattern",
		},
		{
			name: "page key shape",
			mutate: func(t *testing.T, manifest map[string]any) {
				page0(t, manifest)["key"] = "Main"
			},
			want: "key",
		},
		{
			name: "page key too long",
			mutate: func(t *testing.T, manifest map[string]any) {
				page0(t, manifest)["key"] = strings.Repeat("a", 33)
			},
			want: "key",
		},
		{
			name: "duplicate page key",
			mutate: func(_ *testing.T, manifest map[string]any) {
				ui := manifest["ui"].(map[string]any)
				ui["pages"] = []any{
					map[string]any{"key": "main", "entry": "ui/page.html", "title": "Overview"},
					map[string]any{"key": "main", "entry": "ui/other.html", "title": "Other"},
				}
			},
			want: "duplicated",
		},
		{
			name: "entry absolute",
			mutate: func(t *testing.T, manifest map[string]any) {
				page0(t, manifest)["entry"] = "/ui/page.html"
			},
			want: "absolute",
		},
		{
			name: "entry parent segment",
			mutate: func(t *testing.T, manifest map[string]any) {
				page0(t, manifest)["entry"] = "ui/../page.html"
			},
			want: "..",
		},
		{
			name: "entry outside ui",
			mutate: func(t *testing.T, manifest map[string]any) {
				page0(t, manifest)["entry"] = "page.html"
			},
			want: "ui/",
		},
		{
			name: "entry not html",
			mutate: func(t *testing.T, manifest map[string]any) {
				page0(t, manifest)["entry"] = "ui/page.txt"
			},
			want: "html",
		},
		{
			name: "title blank",
			mutate: func(t *testing.T, manifest map[string]any) {
				page0(t, manifest)["title"] = "   "
			},
			want: "title must not be blank",
		},
		{
			name: "title null",
			mutate: func(t *testing.T, manifest map[string]any) {
				page0(t, manifest)["title"] = nil
			},
			want: "title must not be null",
		},
		{
			name: "title array",
			mutate: func(t *testing.T, manifest map[string]any) {
				page0(t, manifest)["title"] = []any{"Overview"}
			},
			want: "title must be a string or a language map",
		},
		{
			name: "title missing en",
			mutate: func(t *testing.T, manifest map[string]any) {
				page0(t, manifest)["title"] = map[string]any{"zh-CN": "概览"}
			},
			want: "title must include en",
		},
		{
			name: "title blank translation",
			mutate: func(t *testing.T, manifest map[string]any) {
				page0(t, manifest)["title"] = map[string]any{"en": "  ", "zh-CN": "概览"}
			},
			want: "non-empty string",
		},
		{
			name: "title value not a string",
			mutate: func(t *testing.T, manifest map[string]any) {
				page0(t, manifest)["title"] = map[string]any{"en": 1}
			},
			want: "must be a non-empty string",
		},
		{
			name: "events empty array",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["events"] = []any{}
			},
		},
		{
			name: "events populated",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["events"] = []any{"request.failover"}
			},
		},
		{
			name: "events channel.auto_disabled",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["events"] = []any{"channel.auto_disabled"}
			},
			want: `event "channel.auto_disabled" is not enabled`,
		},
		{
			name: "events provider.quota.warning",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["events"] = []any{"provider.quota.warning"}
			},
			want: `event "provider.quota.warning" is not enabled`,
		},
		{
			name: "events provider.quota.exhausted",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["events"] = []any{"provider.quota.exhausted"}
			},
			want: `event "provider.quota.exhausted" is not enabled`,
		},
		{
			name: "events duplicated",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["events"] = []any{"request.failover", "request.failover"}
			},
			want: `event "request.failover" is duplicated`,
		},
		{
			name: "events not an array",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["events"] = "request.failover"
			},
			want: "events must be an array of strings",
		},
		{
			name: "hooks.request true",
			mutate: func(t *testing.T, manifest map[string]any) {
				hooksOf(t, manifest)["request"] = true
			},
			want: "request hooks are not enabled",
		},
		{
			name: "hooks.attempt true",
			mutate: func(t *testing.T, manifest map[string]any) {
				hooksOf(t, manifest)["attempt"] = true
			},
			want: "request hooks are not enabled",
		},
		{
			name: "hooks.response present",
			mutate: func(t *testing.T, manifest map[string]any) {
				hooksOf(t, manifest)["response"] = map[string]any{}
			},
			want:    "response hooks are not enabled",
			notWant: "unknown field",
		},
		{
			name: "hooks.streamChunk present",
			mutate: func(t *testing.T, manifest map[string]any) {
				hooksOf(t, manifest)["streamChunk"] = true
			},
			want: "response hooks are not enabled",
		},
		{
			name: "ui.panels present",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["ui"].(map[string]any)["panels"] = []any{}
			},
			want:    "ui panels are not enabled",
			notWant: "unknown field",
		},
		{
			name: "providers empty array",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["providers"] = []any{}
			},
			want:    "custom channel providers are not enabled",
			notWant: "unknown field",
		},
		{
			name: "unknown hook typo",
			mutate: func(t *testing.T, manifest map[string]any) {
				hooksOf(t, manifest)["requset"] = true
			},
			want:    "unknown field hooks.requset",
			notWant: "response hooks are not enabled",
		},
		{
			name: "unknown root field",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["extra"] = true
			},
			want: "unknown field extra",
		},
		{
			name: "unknown ui field",
			mutate: func(_ *testing.T, manifest map[string]any) {
				manifest["ui"].(map[string]any)["slots"] = []any{}
			},
			want: "unknown field ui.slots",
		},
		{
			name: "unknown page field",
			mutate: func(t *testing.T, manifest map[string]any) {
				page0(t, manifest)["icon"] = "x"
			},
			want: "unknown field ui.pages[0].icon",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			runtime := tt.runtime
			if runtime == "" {
				runtime = "v1.0.0-beta9"
			}
			raw := []byte(validManifestJSON)
			if tt.mutate != nil {
				raw = mutateManifest(t, tt.mutate)
			}
			_, err := ParseManifest(raw, runtime)
			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tt.want)
			if tt.notWant != "" {
				require.NotContains(t, err.Error(), tt.notWant)
			}
		})
	}
}

func mutateManifest(t *testing.T, mutate func(t *testing.T, manifest map[string]any)) []byte {
	t.Helper()
	var manifest map[string]any
	require.NoError(t, json.Unmarshal([]byte(validManifestJSON), &manifest))
	mutate(t, manifest)
	payload, err := json.Marshal(manifest)
	require.NoError(t, err)
	return payload
}

func hooksOf(t *testing.T, manifest map[string]any) map[string]any {
	t.Helper()
	hooks, ok := manifest["hooks"].(map[string]any)
	require.True(t, ok)
	return hooks
}

func page0(t *testing.T, manifest map[string]any) map[string]any {
	t.Helper()
	ui, ok := manifest["ui"].(map[string]any)
	require.True(t, ok)
	pages, ok := ui["pages"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, pages)
	page, ok := pages[0].(map[string]any)
	require.True(t, ok)
	return page
}

func TestManifestNetwork(t *testing.T) {
	regex := `^https://api\.example\.com/v1/items/[0-9]+$`
	raw := mutateManifest(t, func(_ *testing.T, manifest map[string]any) {
		manifest["network"] = []any{
			"https://example.com/v1/check",
			map[string]any{"regex": regex},
		}
	})
	manifest, err := ParseManifest(raw, "v1.0.0-beta9")
	require.NoError(t, err)
	require.Equal(t, []string{"https://example.com/v1/check", regex}, manifest.NetworkGrants())
	encoded, err := json.Marshal(manifest.Network)
	require.NoError(t, err)
	var again []objects.PluginNetworkRule
	require.NoError(t, json.Unmarshal(encoded, &again))
	require.Equal(t, manifest.Network, again)

	rejected := []struct {
		name string
		rule any
		want string
	}{
		{name: "bare origin", rule: "https://example.com", want: "path"},
		{name: "userinfo", rule: "https://user:pw@example.com/v1", want: "userinfo"},
		{name: "query", rule: "https://example.com/v1?x=1", want: "query"},
		{name: "fragment", rule: "https://example.com/v1#x", want: "fragment"},
		{name: "http", rule: "http://example.com/v1", want: "https"},
		{name: "unanchored regex", rule: map[string]any{"regex": "https://example.com/v1"}, want: "^"},
		{name: "invalid regex", rule: map[string]any{"regex": "^($"}, want: "invalid"},
	}
	for _, item := range rejected {
		t.Run(item.name, func(t *testing.T) {
			raw := mutateManifest(t, func(_ *testing.T, manifest map[string]any) {
				manifest["network"] = []any{item.rule}
			})
			_, err := ParseManifest(raw, "v1.0.0-beta9")
			require.ErrorContains(t, err, item.want)
		})
	}

	raw = mutateManifest(t, func(_ *testing.T, manifest map[string]any) {
		manifest["network"] = []any{map[string]any{"regex": regex, "url": "https://example.com/v1"}}
	})
	_, err = ParseManifest(raw, "v1.0.0-beta9")
	require.ErrorContains(t, err, "unknown field network[0].url")

	raw = mutateManifest(t, func(_ *testing.T, manifest map[string]any) {
		manifest["network"] = []any{"https://example.com/v1/check", "https://example.com/v1/check"}
	})
	_, err = ParseManifest(raw, "v1.0.0-beta9")
	require.ErrorContains(t, err, "duplicated")

	long := "^" + strings.Repeat("a", objects.MaxPluginNetworkRegex) + "$"
	raw = mutateManifest(t, func(_ *testing.T, manifest map[string]any) {
		manifest["network"] = []any{map[string]any{"regex": long}}
	})
	_, err = ParseManifest(raw, "v1.0.0-beta9")
	require.ErrorContains(t, err, "256")

	rules := make([]any, objects.MaxPluginNetworkRules+1)
	for i := range rules {
		rules[i] = fmt.Sprintf("https://example.com/v1/%d", i)
	}
	raw = mutateManifest(t, func(_ *testing.T, manifest map[string]any) {
		manifest["network"] = rules
	})
	_, err = ParseManifest(raw, "v1.0.0-beta9")
	require.ErrorContains(t, err, "32")
}

func propertiesOf(t *testing.T, manifest map[string]any) map[string]any {
	t.Helper()
	schema, ok := manifest["configSchema"].(map[string]any)
	require.True(t, ok)
	properties, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	return properties
}
