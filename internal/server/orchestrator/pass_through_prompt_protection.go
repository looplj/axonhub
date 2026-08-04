package orchestrator

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

// rawPromptTextField identifies a scalar string or structured JSON value in an
// API-native request body and its unified message role.
type rawPromptTextField struct {
	path           string
	role           string
	structuredJSON bool
}

// mergePassThroughRequestBodyWithPromptProtection preserves the original JSON
// body while applying model mapping and matched prompt-protection mask rules.
func mergePassThroughRequestBodyWithPromptProtection(
	rawBody []byte,
	apiFormat llm.APIFormat,
	model string,
	rules []*ent.PromptProtectionRule,
) ([]byte, error) {
	body, err := mergePassThroughRequestBody(rawBody, apiFormat, model)
	if err != nil || len(rules) == 0 {
		return body, err
	}

	return patchPassThroughPromptProtection(body, apiFormat, rules)
}

// patchPassThroughPromptProtection replaces only text fields represented by the
// unified request. Unsupported or unmatched JSON layouts return an error so the
// caller retains the transformed, already-protected outbound request body.
func patchPassThroughPromptProtection(
	body []byte,
	apiFormat llm.APIFormat,
	rules []*ent.PromptProtectionRule,
) ([]byte, error) {
	if !json.Valid(body) {
		return nil, fmt.Errorf("invalid pass-through JSON body")
	}

	fields, err := rawPromptTextFields(body, apiFormat)
	if err != nil {
		return nil, err
	}

	patched := append([]byte(nil), body...)
	matchCount := 0

	for _, field := range fields {
		nextBody, changed, err := patchPassThroughPromptField(patched, field, rules)
		if err != nil {
			return nil, err
		}
		if !changed {
			continue
		}

		patched = nextBody
		matchCount++
	}

	if matchCount == 0 {
		return nil, fmt.Errorf("matched prompt protection rules did not match a raw request text field")
	}

	return patched, nil
}

// patchPassThroughPromptField applies mask rules to one raw string or the JSON
// object used as Gemini's structured function-response content.
func patchPassThroughPromptField(
	body []byte,
	field rawPromptTextField,
	rules []*ent.PromptProtectionRule,
) ([]byte, bool, error) {
	value := gjson.GetBytes(body, field.path)
	if !value.Exists() {
		return nil, false, fmt.Errorf("prompt text field %q is missing", field.path)
	}

	if field.structuredJSON {
		return patchPassThroughPromptJSONField(body, field, value, rules)
	}
	if value.Type != gjson.String {
		return nil, false, fmt.Errorf("prompt text field %q is not a string", field.path)
	}

	replacement, changed := replacePassThroughPromptText(value.String(), field.role, rules)
	if !changed {
		return body, false, nil
	}

	nextBody, err := sjson.SetBytes(body, field.path, replacement)
	if err != nil {
		return nil, false, fmt.Errorf("replace protected prompt at %q: %w", field.path, err)
	}

	return nextBody, true, nil
}

// patchPassThroughPromptJSONField mirrors Gemini's map unmarshal/marshal path
// before replacement, so tool-scope rules cover functionResponse.response too.
func patchPassThroughPromptJSONField(
	body []byte,
	field rawPromptTextField,
	value gjson.Result,
	rules []*ent.PromptProtectionRule,
) ([]byte, bool, error) {
	canonical, err := canonicalPromptProtectionJSON(value.Raw)
	if err != nil {
		return nil, false, fmt.Errorf("normalize protected JSON at %q: %w", field.path, err)
	}

	replacement, changed := replacePassThroughPromptText(string(canonical), field.role, rules)
	if !changed {
		return body, false, nil
	}
	if !json.Valid([]byte(replacement)) {
		return nil, false, fmt.Errorf("protected JSON at %q is no longer valid", field.path)
	}

	nextBody, err := sjson.SetRawBytes(body, field.path, []byte(replacement))
	if err != nil {
		return nil, false, fmt.Errorf("replace protected JSON at %q: %w", field.path, err)
	}

	return nextBody, true, nil
}

// canonicalPromptProtectionJSON produces the same compact map JSON created by
// Gemini inbound conversion for a structured function response.
func canonicalPromptProtectionJSON(raw string) ([]byte, error) {
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, err
	}

	return json.Marshal(value)
}

// replacePassThroughPromptText applies the same ordered mask rules used by the
// unified prompt-protection service to one API-native text field.
func replacePassThroughPromptText(text, role string, rules []*ent.PromptProtectionRule) (string, bool) {
	changed := false

	for _, rule := range rules {
		if rule == nil || rule.Settings == nil || rule.Settings.Action != objects.PromptProtectionActionMask {
			continue
		}

		if !promptProtectionRuleAppliesToRawRole(rule.Settings.Scopes, role) || !biz.MatchPromptProtectionRule(rule.Pattern, text) {
			continue
		}

		text = biz.ReplacePromptProtectionRule(rule.Pattern, text, rule.Settings.Replacement)
		changed = true
	}

	return text, changed
}

// promptProtectionRuleAppliesToRawRole applies configured scopes to a role read
// from an API-native request body.
func promptProtectionRuleAppliesToRawRole(scopes []objects.PromptProtectionScope, role string) bool {
	if len(scopes) == 0 {
		return true
	}

	return slices.Contains(scopes, objects.PromptProtectionScope(strings.ToLower(role)))
}

// rawPromptTextFields returns maskable textual fields for direct provider APIs.
func rawPromptTextFields(body []byte, apiFormat llm.APIFormat) ([]rawPromptTextField, error) {
	switch apiFormat {
	case llm.APIFormatOpenAIChatCompletion, llm.APIFormatOllamaChat:
		return openAIChatPromptTextFields(body), nil
	case llm.APIFormatOpenAIResponse:
		return openAIResponsesPromptTextFields(body), nil
	case llm.APIFormatAnthropicMessage:
		return anthropicPromptTextFields(body), nil
	case llm.APIFormatGeminiContents:
		return geminiPromptTextFields(body), nil
	default:
		return nil, fmt.Errorf("API format %q does not support raw prompt protection patches", apiFormat)
	}
}

// openAIChatPromptTextFields returns string and text-part content from chat-style bodies.
func openAIChatPromptTextFields(body []byte) []rawPromptTextField {
	fields := make([]rawPromptTextField, 0)

	for i := range jsonArrayLength(body, "messages") {
		messagePath := fmt.Sprintf("messages.%d", i)
		role := gjson.GetBytes(body, messagePath+".role").String()
		fields = append(fields, rawContentTextFields(body, messagePath+".content", role, "text")...)
	}

	return fields
}

// openAIResponsesPromptTextFields returns instruction, input, and tool-output text fields.
func openAIResponsesPromptTextFields(body []byte) []rawPromptTextField {
	fields := rawStringField(body, "instructions", "system")
	input := gjson.GetBytes(body, "input")
	if input.Type == gjson.String {
		return append(fields, rawPromptTextField{path: "input", role: "user"})
	}

	for i := range jsonArrayLength(body, "input") {
		itemPath := fmt.Sprintf("input.%d", i)
		itemType := strings.ToLower(gjson.GetBytes(body, itemPath+".type").String())
		role := gjson.GetBytes(body, itemPath+".role").String()

		if itemType == "input_text" {
			fields = append(fields, rawStringField(body, itemPath+".text", role)...)
		}

		fields = append(fields, rawContentTextFields(body, itemPath+".content", role, "input_text", "output_text", "text")...)

		if itemType == "function_call_output" || itemType == "custom_tool_call_output" {
			fields = append(fields, rawContentTextFields(body, itemPath+".output", "tool", "input_text", "output_text", "text")...)
		}
	}

	return fields
}

// anthropicPromptTextFields returns top-level system text, message text, and tool-result text.
func anthropicPromptTextFields(body []byte) []rawPromptTextField {
	fields := rawContentTextFields(body, "system", "system", "text")

	for i := range jsonArrayLength(body, "messages") {
		messagePath := fmt.Sprintf("messages.%d", i)
		role := gjson.GetBytes(body, messagePath+".role").String()
		contentPath := messagePath + ".content"
		fields = append(fields, rawContentTextFields(body, contentPath, role, "text")...)

		for j := range jsonArrayLength(body, contentPath) {
			blockPath := fmt.Sprintf("%s.%d", contentPath, j)
			if gjson.GetBytes(body, blockPath+".type").String() != "tool_result" {
				continue
			}

			fields = append(fields, rawContentTextFields(body, blockPath+".content", "tool", "text")...)
		}
	}

	return fields
}

// geminiPromptTextFields returns system instruction and content-part text fields.
func geminiPromptTextFields(body []byte) []rawPromptTextField {
	fields := geminiContentPromptTextFields(body, "systemInstruction", "system")

	for i := range jsonArrayLength(body, "contents") {
		contentPath := fmt.Sprintf("contents.%d", i)
		role := geminiRawRoleToPromptRole(gjson.GetBytes(body, contentPath+".role").String())
		fields = append(fields, geminiContentPromptTextFields(body, contentPath, role)...)
	}

	return fields
}

// geminiContentPromptTextFields returns text fields from one Gemini Content object.
func geminiContentPromptTextFields(body []byte, contentPath, role string) []rawPromptTextField {
	fields := make([]rawPromptTextField, 0)

	for i := range jsonArrayLength(body, contentPath+".parts") {
		partPath := fmt.Sprintf("%s.parts.%d", contentPath, i)
		fields = append(fields, rawStringField(body, partPath+".text", role)...)
		fields = append(fields, rawJSONField(body, partPath+".functionResponse.response", "tool")...)
	}

	return fields
}

// geminiRawRoleToPromptRole mirrors Gemini inbound role normalization.
func geminiRawRoleToPromptRole(role string) string {
	switch role {
	case "model":
		return "assistant"
	case "", "user":
		return "user"
	default:
		return role
	}
}

// rawContentTextFields returns either a scalar content string or typed text parts.
func rawContentTextFields(body []byte, contentPath, role string, allowedTypes ...string) []rawPromptTextField {
	content := gjson.GetBytes(body, contentPath)
	if content.Type == gjson.String {
		return []rawPromptTextField{{path: contentPath, role: role}}
	}

	fields := make([]rawPromptTextField, 0)
	for i := range jsonArrayLength(body, contentPath) {
		partPath := fmt.Sprintf("%s.%d", contentPath, i)
		partType := strings.ToLower(gjson.GetBytes(body, partPath+".type").String())
		if !slices.Contains(allowedTypes, partType) {
			continue
		}

		fields = append(fields, rawStringField(body, partPath+".text", role)...)
	}

	return fields
}

// rawStringField returns a field only when the supplied JSON path is a string.
func rawStringField(body []byte, path, role string) []rawPromptTextField {
	if gjson.GetBytes(body, path).Type != gjson.String {
		return nil
	}

	return []rawPromptTextField{{path: path, role: role}}
}

// rawJSONField returns one structured field when a provider stores protected
// tool content as an object instead of a JSON string.
func rawJSONField(body []byte, path, role string) []rawPromptTextField {
	if gjson.GetBytes(body, path).Type != gjson.JSON {
		return nil
	}

	return []rawPromptTextField{{path: path, role: role, structuredJSON: true}}
}

// jsonArrayLength returns the number of elements at a JSON array path.
func jsonArrayLength(body []byte, path string) int {
	return int(gjson.GetBytes(body, path+".#").Int())
}
