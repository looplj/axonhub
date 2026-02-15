package xjson

import (
	"encoding/json"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/samber/lo"
)

func Transform(rawSchema json.RawMessage, transform func(*jsonschema.Schema)) (json.RawMessage, error) {
	var schema jsonschema.Schema
	if err := json.Unmarshal(rawSchema, &schema); err != nil {
		return nil, err
	}

	transformSchemaRecursive(&schema, transform)

	return json.Marshal(&schema)
}

func transformSchemaRecursive(schema *jsonschema.Schema, transform func(*jsonschema.Schema)) {
	if schema == nil {
		return
	}

	transform(schema)

	// Recursively transform sub-schema fields
	lo.ForEach([]*jsonschema.Schema{
		schema.Items,
		schema.AdditionalItems,
		schema.Contains,
		schema.Not,
		schema.If,
		schema.Then,
		schema.Else,
		schema.PropertyNames,
		schema.UnevaluatedProperties,
		schema.UnevaluatedItems,
		schema.ContentSchema,
	}, func(subSchema *jsonschema.Schema, _ int) {
		transformSchemaRecursive(subSchema, transform)
	})

	// Transform schema slices
	schemaSlices := [][]*jsonschema.Schema{
		schema.PrefixItems,
		schema.ItemsArray,
		schema.AllOf,
		schema.AnyOf,
		schema.OneOf,
	}
	lo.ForEach(schemaSlices, func(schemas []*jsonschema.Schema, _ int) {
		lo.ForEach(schemas, func(subSchema *jsonschema.Schema, _ int) {
			transformSchemaRecursive(subSchema, transform)
		})
	})

	// Transform schema maps
	schemaMaps := []map[string]*jsonschema.Schema{
		schema.Defs,
		schema.Definitions,
		schema.DependentSchemas,
		schema.Properties,
		schema.PatternProperties,
		schema.DependencySchemas,
	}
	lo.ForEach(schemaMaps, func(schemaMap map[string]*jsonschema.Schema, _ int) {
		lo.ForEach(lo.Values(schemaMap), func(subSchema *jsonschema.Schema, _ int) {
			transformSchemaRecursive(subSchema, transform)
		})
	})
}

func CleanSchema(rawSchema json.RawMessage, fields ...string) (json.RawMessage, error) {
	return Transform(rawSchema, func(schema *jsonschema.Schema) {
		clearFieldsFromSchema(schema, fields...)
	})
}

// clearFieldsFromSchema clears specified fields from a single schema.
func clearFieldsFromSchema(schema *jsonschema.Schema, fields ...string) {
	schemaValue := reflect.ValueOf(schema).Elem()
	schemaType := reflect.TypeFor[jsonschema.Schema]()

	for _, fieldName := range fields {
		// Handle special cases for JSON field names that don't match Go struct field names
		switch fieldName {
		case "$schema":
			schema.Schema = ""
		case "$id":
			schema.ID = ""
		case "$ref":
			schema.Ref = ""
		case "$comment":
			schema.Comment = ""
		case "$defs":
			schema.Defs = nil
		case "$anchor":
			schema.Anchor = ""
		case "$dynamicAnchor":
			schema.DynamicAnchor = ""
		case "$dynamicRef":
			schema.DynamicRef = ""
		case "$vocabulary":
			schema.Vocabulary = nil
		case "additionalProperties":
			schema.AdditionalProperties = nil
		case "definitions":
			schema.Definitions = nil
		case "dependentSchemas":
			schema.DependentSchemas = nil
		case "dependentRequired":
			schema.DependentRequired = nil
		case "patternProperties":
			schema.PatternProperties = nil
		case "propertyNames":
			schema.PropertyNames = nil
		case "unevaluatedProperties":
			schema.UnevaluatedProperties = nil
		case "unevaluatedItems":
			schema.UnevaluatedItems = nil
		case "contentSchema":
			schema.ContentSchema = nil
		case "extra":
			schema.Extra = nil
		default:
			// Try to find and clear the field by name
			if _, found := schemaType.FieldByName(fieldName); found {
				fieldValue := schemaValue.FieldByName(fieldName)
				if fieldValue.CanSet() {
					// Set to zero value based on field type
					fieldValue.Set(reflect.Zero(fieldValue.Type()))
				}
			}
		}
	}
}
