package plugin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
)

func requiredSchema(t *testing.T) *objects.PluginConfigSchema {
	t.Helper()
	var schema objects.PluginConfigSchema
	require.NoError(t, json.Unmarshal([]byte(`{
		"type": "object",
		"required": ["token", "threshold", "mode"],
		"properties": {
			"threshold": {"type": "integer", "minimum": 1, "maximum": 100, "default": 10},
			"mode": {"type": "string", "enum": ["fast", "safe"]},
			"token": {"type": "string", "format": "secret"}
		}
	}`), &schema))
	require.NoError(t, schema.Validate())
	return &schema
}

func TestValidateConfigValuesErrorsOmitValues(t *testing.T) {
	schema := requiredSchema(t)
	const secret = "sk-very-secret-value"

	cases := []struct {
		name   string
		config map[string]any
		want   string
	}{
		{"type", map[string]any{"threshold": secret}, `config field "threshold": expected integer`},
		{"fraction is not integer", map[string]any{"threshold": 1.5}, `config field "threshold": expected integer`},
		{"json number", map[string]any{"threshold": json.Number("0")}, `config field "threshold": value is below minimum`},
		{"maximum", map[string]any{"threshold": 101}, `config field "threshold": value is above maximum`},
		{"enum", map[string]any{"mode": secret}, `config field "mode": value is not in enum`},
		{"undeclared", map[string]any{"extra": secret}, `config field "extra" is not declared`},
		{"secret type", map[string]any{"token": 42}, `config field "token": expected string`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfigValues(schema, tc.config, false)
			require.EqualError(t, err, tc.want)
			require.NotContains(t, err.Error(), secret)
		})
	}

	require.NoError(t, validateConfigValues(schema, map[string]any{"threshold": json.Number("5"), "mode": "safe"}, false))
	require.NoError(t, validateConfigValues(schema, map[string]any{}, false), "required is skipped for partial saves")
}

func TestCheckConfigForEnable(t *testing.T) {
	schema := requiredSchema(t)

	err := checkConfigForEnable(schema, map[string]any{})
	require.EqualError(t, err, "required config fields are missing: token, mode")

	err = checkConfigForEnable(schema, map[string]any{"token": "", "mode": "fast"})
	require.EqualError(t, err, "required config fields are missing: token")

	require.NoError(t, checkConfigForEnable(schema, map[string]any{"token": "kept", "mode": "fast"}), "threshold falls back to its default")
	require.Error(t, checkConfigForEnable(schema, map[string]any{"token": "kept", "mode": "fast", "threshold": 0}))

	require.NoError(t, checkConfigForEnable(nil, map[string]any{}))
}

func arraySchema(t *testing.T) *objects.PluginConfigSchema {
	t.Helper()
	var schema objects.PluginConfigSchema
	require.NoError(t, json.Unmarshal([]byte(`{
		"type": "object",
		"required": ["models"],
		"properties": {
			"models": {"type": "array", "items": {"type": "string"}},
			"tags": {"type": "array", "items": {"type": "string"}, "default": ["a"]}
		}
	}`), &schema))
	require.NoError(t, schema.Validate())
	return &schema
}

func TestValidateConfigValuesStringArray(t *testing.T) {
	schema := arraySchema(t)
	const secret = "sk-very-secret-value"

	require.NoError(t, validateConfigValues(schema, map[string]any{"models": []any{"gpt-4o", "claude"}}, true))
	require.NoError(t, validateConfigValues(schema, map[string]any{"models": []string{"gpt-4o"}}, true))

	err := validateConfigValues(schema, map[string]any{"models": secret}, false)
	require.EqualError(t, err, `config field "models": expected array of string`)
	require.NotContains(t, err.Error(), secret)

	err = validateConfigValues(schema, map[string]any{"models": []any{"ok", 42, secret}}, false)
	require.EqualError(t, err, `config field "models": expected array of string`)
	require.NotContains(t, err.Error(), secret)
}

func TestCheckConfigForEnableEmptyArrayIsMissing(t *testing.T) {
	schema := arraySchema(t)

	require.EqualError(t, checkConfigForEnable(schema, map[string]any{"models": []any{}}), "required config fields are missing: models")
	require.NoError(t, checkConfigForEnable(schema, map[string]any{"models": []any{"gpt-4o"}}))
}
