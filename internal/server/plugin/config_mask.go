package plugin

import (
	"maps"

	"github.com/looplj/axonhub/internal/objects"
)

// SecretMask is the exact GraphQL value returned for a secret config field.
// Posting this value back leaves the stored secret unchanged.
const SecretMask = "******"

// maskConfig returns the config shown by GraphQL.
// Secret values become SecretMask. A missing non-secret field receives its
// schema default. The stored map is not modified. A nil or empty schema
// declares no fields, so the result is an empty object.
func maskConfig(schema *objects.PluginConfigSchema, stored map[string]any) map[string]any {
	out := map[string]any{}
	if schema == nil || len(schema.Properties) == 0 {
		return out
	}
	for name, prop := range schema.Properties {
		value, ok := storedValue(stored, name)
		if prop.IsSecret() {
			if ok {
				out[name] = SecretMask
			}
			continue
		}
		if ok {
			out[name] = value
			continue
		}
		if prop.Default != nil {
			out[name] = prop.Default
		}
	}
	return out
}

// keepSecretValues copies incoming and restores stored secrets when the
// incoming value is exactly SecretMask. A mask for a secret that was never
// stored is dropped, so the mask itself is not written. A nil incoming map
// stays nil and fails config validation.
func keepSecretValues(schema *objects.PluginConfigSchema, stored, incoming map[string]any) map[string]any {
	if incoming == nil {
		return nil
	}
	out := make(map[string]any, len(incoming))
	maps.Copy(out, incoming)
	if schema == nil {
		return out
	}
	for name, prop := range schema.Properties {
		if !prop.IsSecret() {
			continue
		}
		value, ok := out[name]
		if !ok {
			continue
		}
		text, isString := value.(string)
		if !isString || text != SecretMask {
			continue
		}
		if previous, has := storedValue(stored, name); has {
			out[name] = previous
			continue
		}
		delete(out, name)
	}
	return out
}

func storedValue(config map[string]any, name string) (any, bool) {
	if config == nil {
		return nil, false
	}
	value, ok := config[name]
	if !ok || value == nil {
		return nil, false
	}
	return value, true
}
