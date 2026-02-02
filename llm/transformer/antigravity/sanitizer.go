package antigravity

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/samber/lo"
)

// UppercaseSchemaTypes converts all "type" field values to UPPERCASE to match Gemini API requirements.
// The Gemini/Antigravity API expects type values in uppercase (OBJECT, STRING, etc.) per protobuf spec.
// Reference: opencode-antigravity-auth/src/plugin/transform/gemini.ts toGeminiSchema() line 76
func UppercaseSchemaTypes(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	result := make(map[string]any)

	for key, value := range schema {
		if key == "type" {
			if typeStr, ok := value.(string); ok {
				result[key] = strings.ToUpper(typeStr)
			} else {
				result[key] = value
			}
		} else if subSchema, ok := value.(map[string]any); ok {
			// Recursively process nested schemas
			result[key] = UppercaseSchemaTypes(subSchema)
		} else if subArray, ok := value.([]any); ok {
			// Process arrays (for anyOf, oneOf, allOf, items, etc.)
			newArray := make([]any, len(subArray))
			for i, item := range subArray {
				if itemMap, ok := item.(map[string]any); ok {
					newArray[i] = UppercaseSchemaTypes(itemMap)
				} else {
					newArray[i] = item
				}
			}
			result[key] = newArray
		} else {
			result[key] = value
		}
	}

	return result
}

// UNSUPPORTED_CONSTRAINTS that should be moved to description hints.
var unsupportedConstraints = []string{
	"minLength", "maxLength", "exclusiveMinimum", "exclusiveMaximum",
	"pattern", "minItems", "maxItems", "format",
	"default", "examples",
}

// UNSUPPORTED_KEYWORDS that should be removed after hint extraction.
var unsupportedKeywords = []string{
	// Include all constraints that were moved to description
	"minLength", "maxLength", "exclusiveMinimum", "exclusiveMaximum",
	"pattern", "minItems", "maxItems", "format",
	"default", "examples",
	// Other unsupported keywords
	"$schema", "$defs", "definitions", "const", "$ref", "additionalProperties",
	"propertyNames", "title", "$id", "$comment",
}

// emptySchemaPlaceholderName is the name of the placeholder field.
const emptySchemaPlaceholderName = "_placeholder"

// emptySchemaPlaceholderDescription is the description of the placeholder field.
const emptySchemaPlaceholderDescription = "Placeholder. Always pass true."

// SanitizeJSONSchema cleans a JSON schema for Antigravity API compatibility.
// It transforms unsupported features into description hints while preserving semantic information.
func SanitizeJSONSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	result := deepCopy(schema)

	// Phase 1: Convert and add hints
	result = convertRefsToHints(result)
	result = convertConstToEnum(result)
	result = addEnumHints(result)
	result = addAdditionalPropertiesHints(result)
	result = moveConstraintsToDescription(result)

	// Phase 2: Flatten complex structures
	result = mergeAllOf(result)
	result = flattenAnyOfOneOf(result)
	result = flattenTypeArrays(result, nil, "")

	// Phase 3: Cleanup
	result = removeUnsupportedKeywords(result, false)
	result = cleanupRequiredFields(result)

	// Phase 4: Add placeholder for empty object schemas
	result = addEmptySchemaPlaceholder(result)

	return result
}

func deepCopy(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	var dst map[string]any
	b, _ := json.Marshal(src)
	_ = json.Unmarshal(b, &dst)
	return dst
}

// Phase 1 Helpers

func appendDescriptionHint(schema map[string]any, hint string) map[string]any {
	if schema == nil {
		return nil
	}
	existing, _ := schema["description"].(string)
	if existing != "" {
		schema["description"] = fmt.Sprintf("%s (%s)", existing, hint)
	} else {
		schema["description"] = hint
	}
	return schema
}

func convertRefsToHints(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	// Check if this object has $ref
	if ref, ok := schema["$ref"].(string); ok {
		parts := strings.Split(ref, "/")
		defName := parts[len(parts)-1]
		hint := fmt.Sprintf("See: %s", defName)

		newSchema := make(map[string]any)
		newSchema["type"] = "object"
		if desc, ok := schema["description"].(string); ok {
			newSchema["description"] = fmt.Sprintf("%s (%s)", desc, hint)
		} else {
			newSchema["description"] = hint
		}
		return newSchema
	}

	// Recursively process all properties
	for key, value := range schema {
		if subSchema, ok := value.(map[string]any); ok {
			schema[key] = convertRefsToHints(subSchema)
		} else if subArray, ok := value.([]any); ok {
			schema[key] = processArray(subArray, convertRefsToHints)
		}
	}
	return schema
}

func convertConstToEnum(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	if val, ok := schema["const"]; ok {
		if _, hasEnum := schema["enum"]; !hasEnum {
			schema["enum"] = []any{val}
		}
	}

	for key, value := range schema {
		if key == "const" {
			continue
		}
		if subSchema, ok := value.(map[string]any); ok {
			schema[key] = convertConstToEnum(subSchema)
		} else if subArray, ok := value.([]any); ok {
			schema[key] = processArray(subArray, convertConstToEnum)
		}
	}
	return schema
}

func addEnumHints(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	if enumVals, ok := schema["enum"].([]any); ok && len(enumVals) > 1 && len(enumVals) <= 10 {
		var strVals []string
		for _, v := range enumVals {
			strVals = append(strVals, fmt.Sprintf("%v", v))
		}
		hint := fmt.Sprintf("Allowed: %s", strings.Join(strVals, ", "))
		schema = appendDescriptionHint(schema, hint)
	}

	for key, value := range schema {
		if key != "enum" {
			if subSchema, ok := value.(map[string]any); ok {
				schema[key] = addEnumHints(subSchema)
			} else if subArray, ok := value.([]any); ok {
				schema[key] = processArray(subArray, addEnumHints)
			}
		}
	}
	return schema
}

func addAdditionalPropertiesHints(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	if ap, ok := schema["additionalProperties"].(bool); ok && !ap {
		schema = appendDescriptionHint(schema, "No extra properties allowed")
	}

	for key, value := range schema {
		if key != "additionalProperties" {
			if subSchema, ok := value.(map[string]any); ok {
				schema[key] = addAdditionalPropertiesHints(subSchema)
			} else if subArray, ok := value.([]any); ok {
				schema[key] = processArray(subArray, addAdditionalPropertiesHints)
			}
		}
	}
	return schema
}

func moveConstraintsToDescription(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	for _, constraint := range unsupportedConstraints {
		if val, ok := schema[constraint]; ok {
			// Skip if it's an object/array (complex structure), assuming constraints are primitives
			if _, isMap := val.(map[string]any); !isMap {
				if _, isSlice := val.([]any); !isSlice {
					hint := fmt.Sprintf("%s: %v", constraint, val)
					schema = appendDescriptionHint(schema, hint)
				}
			}
		}
	}

	for key, value := range schema {
		if subSchema, ok := value.(map[string]any); ok {
			schema[key] = moveConstraintsToDescription(subSchema)
		} else if subArray, ok := value.([]any); ok {
			schema[key] = processArray(subArray, moveConstraintsToDescription)
		}
	}
	return schema
}

// Phase 2 Helpers

func mergeAllOf(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	if allOf, ok := schema["allOf"].([]any); ok {
		merged := make(map[string]any)
		mergedProps := make(map[string]any)
		var mergedRequired []string

		for _, item := range allOf {
			if sub, ok := item.(map[string]any); ok {
				// Merge properties
				if props, ok := sub["properties"].(map[string]any); ok {
					for k, v := range props {
						mergedProps[k] = v
					}
				}
				// Merge required
				if req, ok := sub["required"].([]any); ok {
					for _, r := range req {
						if rStr, ok := r.(string); ok {
							if !lo.Contains(mergedRequired, rStr) {
								mergedRequired = append(mergedRequired, rStr)
							}
						}
					}
				}
				// Copy other fields
				for k, v := range sub {
					if k != "properties" && k != "required" {
						if _, exists := merged[k]; !exists {
							merged[k] = v
						}
					}
				}
			}
		}

		// Apply merged content
		if len(mergedProps) > 0 {
			if existingProps, ok := schema["properties"].(map[string]any); ok {
				for k, v := range mergedProps {
					existingProps[k] = v
				}
			} else {
				schema["properties"] = mergedProps
			}
		}

		if len(mergedRequired) > 0 {
			var existingRequired []string
			if req, ok := schema["required"].([]any); ok {
				for _, r := range req {
					if rStr, ok := r.(string); ok {
						existingRequired = append(existingRequired, rStr)
					}
				}
			}
			for _, r := range mergedRequired {
				if !lo.Contains(existingRequired, r) {
					existingRequired = append(existingRequired, r)
				}
			}

			// Convert back to []any for storage
			reqAny := make([]any, len(existingRequired))
			for i, v := range existingRequired {
				reqAny[i] = v
			}
			schema["required"] = reqAny
		}

		for k, v := range merged {
			if k != "properties" && k != "required" {
				if _, exists := schema[k]; !exists {
					schema[k] = v
				}
			}
		}

		delete(schema, "allOf")
	}

	// Recursion
	for key, value := range schema {
		if subSchema, ok := value.(map[string]any); ok {
			schema[key] = mergeAllOf(subSchema)
		} else if subArray, ok := value.([]any); ok {
			schema[key] = processArray(subArray, mergeAllOf)
		}
	}
	return schema
}

func flattenAnyOfOneOf(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	for _, unionKey := range []string{"anyOf", "oneOf"} {
		if options, ok := schema[unionKey].([]any); ok && len(options) > 0 {
			parentDesc, _ := schema["description"].(string)

			// Check for enum pattern
			mergedEnum := tryMergeEnumFromUnion(options)
			if mergedEnum != nil {
				delete(schema, unionKey)
				schema["type"] = "string"
				schema["enum"] = mergedEnum
				if parentDesc != "" {
					schema["description"] = parentDesc
				}
				continue
			}

			// Flatten logic
			bestIdx := 0
			bestScore := -1
			var allTypes []string

			for i, opt := range options {
				if optMap, ok := opt.(map[string]any); ok {
					score, typeName := scoreSchemaOption(optMap)
					if typeName != "" {
						allTypes = append(allTypes, typeName)
					}
					if score > bestScore {
						bestScore = score
						bestIdx = i
					}
				}
			}

			if bestIdx < len(options) {
				if selected, ok := options[bestIdx].(map[string]any); ok {
					// Recursively flatten selected
					selected = flattenAnyOfOneOf(selected)

					// Merge selected into schema
					delete(schema, unionKey)
					// delete(schema, "description") // Handle description merge below

					// Description merge
					childDesc, _ := selected["description"].(string)
					if parentDesc != "" {
						if childDesc != "" && childDesc != parentDesc {
							selected["description"] = fmt.Sprintf("%s (%s)", parentDesc, childDesc)
						} else {
							selected["description"] = parentDesc
						}
					}

					// Type hints
					uniqueTypes := lo.Uniq[string](allTypes)
					if len(uniqueTypes) > 1 {
						hint := fmt.Sprintf("Accepts: %s", strings.Join(uniqueTypes, " | "))
						selected = appendDescriptionHint(selected, hint)
					}

					// Copy selected fields to schema
					for k, v := range selected {
						schema[k] = v
					}
				}
			}
		}
	}

	// Recursion
	for key, value := range schema {
		if subSchema, ok := value.(map[string]any); ok {
			schema[key] = flattenAnyOfOneOf(subSchema)
		} else if subArray, ok := value.([]any); ok {
			schema[key] = processArray(subArray, flattenAnyOfOneOf)
		}
	}
	return schema
}

func flattenTypeArrays(schema map[string]any, nullableFields map[string][]string, currentPath string) map[string]any {
	if schema == nil {
		return nil
	}

	if nullableFields == nil {
		nullableFields = make(map[string][]string)
	}

	if types, ok := schema["type"].([]any); ok {
		var nonNullTypes []string
		hasNull := false

		for _, t := range types {
			if tStr, ok := t.(string); ok {
				if tStr == "null" {
					hasNull = true
				} else {
					nonNullTypes = append(nonNullTypes, tStr)
				}
			}
		}

		firstType := "string"
		if len(nonNullTypes) > 0 {
			firstType = nonNullTypes[0]
		} else if hasNull {
			firstType = "null"
		}
		schema["type"] = firstType

		if len(nonNullTypes) > 1 {
			hint := fmt.Sprintf("Accepts: %s", strings.Join(nonNullTypes, " | "))
			schema = appendDescriptionHint(schema, hint)
		}

		if hasNull {
			schema = appendDescriptionHint(schema, "nullable")
		}
	}

	// Recursively process properties
	if props, ok := schema["properties"].(map[string]any); ok {
		for key, val := range props {
			if propSchema, ok := val.(map[string]any); ok {
				propPath := fmt.Sprintf("%s.properties.%s", currentPath, key)
				if currentPath == "" {
					propPath = fmt.Sprintf("properties.%s", key)
				}

				processed := flattenTypeArrays(propSchema, nullableFields, propPath)
				props[key] = processed

				if desc, ok := processed["description"].(string); ok && strings.Contains(desc, "nullable") {
					path := currentPath
					nullableFields[path] = append(nullableFields[path], key)
				}
			}
		}
	}

	// Remove nullable fields from required array at root
	if currentPath == "" {
		if req, ok := schema["required"].([]any); ok {
			nullableAtRoot := nullableFields[""]
			var newReq []any
			for _, r := range req {
				if rStr, ok := r.(string); ok {
					if !lo.Contains(nullableAtRoot, rStr) {
						newReq = append(newReq, rStr)
					}
				}
			}
			if len(newReq) == 0 {
				delete(schema, "required")
			} else {
				schema["required"] = newReq
			}
		}
	}

	// Recursion for other fields
	for key, value := range schema {
		if key != "properties" {
			if subSchema, ok := value.(map[string]any); ok {
				schema[key] = flattenTypeArrays(subSchema, nullableFields, fmt.Sprintf("%s.%s", currentPath, key))
			} else if subArray, ok := value.([]any); ok {
				// Arrays handling can be complex for paths, simplifying for now as mostly properties matter for required
				schema[key] = processArray(subArray, func(m map[string]any) map[string]any {
					return flattenTypeArrays(m, nullableFields, currentPath) // pass same path or approximate
				})
			}
		}
	}

	return schema
}

// Phase 3 Helpers

func removeUnsupportedKeywords(schema map[string]any, insideProperties bool) map[string]any {
	if schema == nil {
		return nil
	}

	for key, value := range schema {
		if !insideProperties && lo.Contains(unsupportedKeywords, key) {
			delete(schema, key)
			continue
		}

		if subSchema, ok := value.(map[string]any); ok {
			if key == "properties" {
				// Process properties values, marking as NOT inside properties (recurse logic)
				// Actually, the original TS code says:
				// if (key === "properties") { ... removeUnsupportedKeywords(propSchema, false); }
				// So we iterate over properties values and treat them as schemas
				newProps := make(map[string]any)
				for k, v := range subSchema {
					if vMap, ok := v.(map[string]any); ok {
						newProps[k] = removeUnsupportedKeywords(vMap, false)
					} else {
						newProps[k] = v
					}
				}
				schema[key] = newProps
			} else {
				schema[key] = removeUnsupportedKeywords(subSchema, false)
			}
		} else if subArray, ok := value.([]any); ok {
			schema[key] = processArray(subArray, func(m map[string]any) map[string]any {
				return removeUnsupportedKeywords(m, false)
			})
		}
	}
	return schema
}

func cleanupRequiredFields(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	if req, ok := schema["required"].([]any); ok {
		if props, ok := schema["properties"].(map[string]any); ok {
			var validReq []any
			for _, r := range req {
				if rStr, ok := r.(string); ok {
					if _, exists := props[rStr]; exists {
						validReq = append(validReq, rStr)
					}
				}
			}
			if len(validReq) == 0 {
				delete(schema, "required")
			} else {
				schema["required"] = validReq
			}
		}
	}

	for key, value := range schema {
		if subSchema, ok := value.(map[string]any); ok {
			schema[key] = cleanupRequiredFields(subSchema)
		} else if subArray, ok := value.([]any); ok {
			schema[key] = processArray(subArray, cleanupRequiredFields)
		}
	}
	return schema
}

// Phase 4 Helpers

func addEmptySchemaPlaceholder(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	if typeVal, ok := schema["type"].(string); ok && typeVal == "object" {
		props, hasProps := schema["properties"].(map[string]any)
		if !hasProps || len(props) == 0 {
			schema["properties"] = map[string]any{
				emptySchemaPlaceholderName: map[string]any{
					"type":        "boolean",
					"description": emptySchemaPlaceholderDescription,
				},
			}
			schema["required"] = []any{emptySchemaPlaceholderName}
		}
	}

	for key, value := range schema {
		if subSchema, ok := value.(map[string]any); ok {
			schema[key] = addEmptySchemaPlaceholder(subSchema)
		} else if subArray, ok := value.([]any); ok {
			schema[key] = processArray(subArray, addEmptySchemaPlaceholder)
		}
	}
	return schema
}

// Utility functions

func processArray(arr []any, fn func(map[string]any) map[string]any) []any {
	newArr := make([]any, len(arr))
	for i, item := range arr {
		if itemMap, ok := item.(map[string]any); ok {
			newArr[i] = fn(itemMap)
		} else {
			newArr[i] = item
		}
	}
	return newArr
}

func scoreSchemaOption(schema map[string]any) (int, string) {
	if schema == nil {
		return 0, "unknown"
	}

	typeName, _ := schema["type"].(string)

	if typeName == "object" || schema["properties"] != nil {
		return 3, "object"
	}
	if typeName == "array" || schema["items"] != nil {
		return 2, "array"
	}
	if typeName != "" && typeName != "null" {
		return 1, typeName
	}
	if typeName == "" {
		return 0, "null"
	}
	return 0, typeName
}

func tryMergeEnumFromUnion(options []any) []any {
	var enumValues []any

	for _, opt := range options {
		optMap, ok := opt.(map[string]any)
		if !ok {
			return nil
		}

		if val, ok := optMap["const"]; ok {
			enumValues = append(enumValues, val)
			continue
		}

		if enums, ok := optMap["enum"].([]any); ok {
			if len(enums) == 1 {
				enumValues = append(enumValues, enums[0])
				continue
			}
			if len(enums) > 0 {
				enumValues = append(enumValues, enums...)
				continue
			}
		}

		// If complex structure, not simple enum
		if optMap["properties"] != nil || optMap["items"] != nil || optMap["anyOf"] != nil || optMap["oneOf"] != nil || optMap["allOf"] != nil {
			return nil
		}

		// If type but no const/enum
		if _, hasType := optMap["type"]; hasType && optMap["const"] == nil && optMap["enum"] == nil {
			return nil
		}
	}

	if len(enumValues) > 0 {
		return enumValues
	}
	return nil
}
