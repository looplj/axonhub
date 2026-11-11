package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/looplj/axonhub/internal/objects"
)

// resolveConfigSchema compiles the manifest config schema into a JSON Schema.
// Undeclared fields are rejected through additionalProperties false. required
// is kept only when withRequired is true, so a disabled plugin can save partial config.
func resolveConfigSchema(schema *objects.PluginConfigSchema, withRequired bool) (*jsonschema.Resolved, error) {
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("configSchema cannot be encoded: %w", err)
	}
	var compiled jsonschema.Schema
	if err := json.Unmarshal(encoded, &compiled); err != nil {
		return nil, fmt.Errorf("configSchema is not a valid JSON Schema: %w", err)
	}
	compiled.AdditionalProperties = &jsonschema.Schema{Not: &jsonschema.Schema{}}
	if !withRequired {
		compiled.Required = nil
	}
	resolved, err := compiled.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("configSchema is not a valid JSON Schema: %w", err)
	}
	return resolved, nil
}

// validateConfigValues checks config against schema with a JSON Schema validator.
// The returned error names fields and keywords but never a config value.
func validateConfigValues(schema *objects.PluginConfigSchema, config map[string]any, withRequired bool) error {
	resolved, err := resolveConfigSchema(schema, withRequired)
	if err != nil {
		return err
	}
	// Round-trip through JSON so json.Number and Go integer types reach the
	// validator as float64, the form it uses to tell integer from number.
	encoded, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("config cannot be encoded: %w", err)
	}
	var instance map[string]any
	if err := json.Unmarshal(encoded, &instance); err != nil {
		return fmt.Errorf("config cannot be decoded: %w", err)
	}
	if instance == nil {
		instance = map[string]any{}
	}
	if err := resolved.Validate(instance); err != nil {
		return describeConfigError(schema, err)
	}
	return nil
}

// checkConfigForEnable validates config with required fields. A field the owner
// has not saved counts as set when it has a schema default, matching what the
// settings form shows. An empty string, stored or as the default, counts as
// missing, since it is what a cleared text input holds. An empty array counts
// as missing for the same reason.
func checkConfigForEnable(schema *objects.PluginConfigSchema, config map[string]any) error {
	if schema == nil || len(schema.Properties) == 0 {
		return nil
	}
	effective := make(map[string]any, len(schema.Properties))
	for name, value := range config {
		if text, ok := value.(string); ok && text == "" {
			continue
		}
		if items, ok := value.([]any); ok && len(items) == 0 {
			continue
		}
		effective[name] = value
	}
	for name, prop := range schema.Properties {
		if _, ok := effective[name]; !ok && prop.Default != nil && prop.Default != "" {
			effective[name] = prop.Default
		}
	}
	return validateConfigValues(schema, effective, true)
}

var quotedName = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)

// describeConfigError rewrites a jsonschema-go error, whose text can embed the
// rejected value, into a message that keeps only field names and the keyword.
func describeConfigError(schema *objects.PluginConfigSchema, err error) error {
	rest := err.Error()
	field := ""
	for strings.HasPrefix(rest, "validating ") {
		location, after, ok := strings.Cut(rest[len("validating "):], ": ")
		if !ok {
			break
		}
		if name, isProperty := strings.CutPrefix(location, "/properties/"); isProperty {
			// Array item errors report /properties/<name>/items; keep only the property name.
			name, _, _ = strings.Cut(name, "/")
			field = strings.NewReplacer("~1", "/", "~0", "~").Replace(name)
		}
		rest = after
	}
	switch {
	case strings.HasPrefix(rest, "required:"):
		return fmt.Errorf("required config fields are missing: %s", strings.Join(quotedNames(rest), ", "))
	case strings.HasPrefix(rest, "unexpected additional properties"):
		names := quotedNames(rest)
		if len(names) > 0 {
			return fmt.Errorf("config field %q is not declared", names[0])
		}
		return errors.New("config contains undeclared fields")
	}
	keyword, _, _ := strings.Cut(rest, ":")
	if field == "" {
		return fmt.Errorf("config does not satisfy %s", keyword)
	}
	switch keyword {
	case "type":
		prop := schema.Properties[field]
		if prop.Type == objects.ConfigTypeArray && prop.Items != nil {
			return fmt.Errorf("config field %q: expected array of %s", field, prop.Items.Type)
		}
		return fmt.Errorf("config field %q: expected %s", field, prop.Type)
	case "enum":
		return fmt.Errorf("config field %q: value is not in enum", field)
	case "minimum":
		return fmt.Errorf("config field %q: value is below minimum", field)
	case "maximum":
		return fmt.Errorf("config field %q: value is above maximum", field)
	default:
		return fmt.Errorf("config field %q: value does not satisfy %s", field, keyword)
	}
}

func quotedNames(text string) []string {
	matches := quotedName.FindAllString(text, -1)
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		if name, err := strconv.Unquote(match); err == nil {
			names = append(names, name)
		}
	}
	return names
}
