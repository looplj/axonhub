package objects

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
)

type (
	ProxyType   = httpclient.ProxyType
	ProxyConfig = httpclient.ProxyConfig
)

type ModelMapping struct {
	// From is the model name in the request.
	From string `json:"from"`

	// To is the model name in the provider.
	To string `json:"to"`
}

type HeaderEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Override operation types.
const (
	OverrideOpSet    = "set"
	OverrideOpDelete = "delete"
	OverrideOpRename = "rename"
	OverrideOpCopy   = "copy"
)

// OverrideOperation defines a structured override operation for request body/header manipulation.
type OverrideOperation struct {
	Op        string `json:"op"`
	Path      string `json:"path,omitempty"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Value     string `json:"value,omitempty"`
	Condition string `json:"condition,omitempty"`
}

func HeaderEntriesToOverrideOperations(headers []HeaderEntry) []OverrideOperation {
	if len(headers) == 0 {
		return nil
	}

	ops := make([]OverrideOperation, 0, len(headers))
	for _, header := range headers {
		if header.Value == "__AXONHUB_CLEAR__" {
			ops = append(ops, OverrideOperation{Op: OverrideOpDelete, Path: header.Key})
			continue
		}

		ops = append(ops, OverrideOperation{Op: OverrideOpSet, Path: header.Key, Value: header.Value})
	}

	return ops
}

type TransformOptions struct {
	// ForceArrayInstructions forces the channel to accept array format for instructions.
	ForceArrayInstructions bool `json:"forceArrayInstructions"`

	// ForceArrayInputs forces the channel to accept array format for inputs.
	ForceArrayInputs bool `json:"forceArrayInputs"`

	// ReplaceDeveloperRoleWithSystem replaces developer role with system in messages for Bailian compatibility.
	ReplaceDeveloperRoleWithSystem bool `json:"replaceDeveloperRoleWithSystem"`
}

type ChannelSettings struct {
	// ExtraModelPrefix sets the channel accept the model with the extra prefix.
	// e.g. a channel
	// supported_modles is ["deepseek-chat", "deepseek-reasoner"]
	// extraModelPrefix is "deepseek"
	// then the model "deepseek-chat", "deepseek-reasoner", "deepseek/deepseek-chat", "deepseek/deepseek-reasoner"  will be accepted.
	// And if other channel support "deepseek/deepseek-chat", "deepseek/deepseek-reasoner" modles, the two channels can accept the request both.
	ExtraModelPrefix string `json:"extraModelPrefix"`

	// AutoTrimedModelPrefixes configures prefixes to automatically trim the model name when added to supported models.
	// e.g. a channel
	// supported_modles is ["deepseek-ai/deepseek-chat", "openai/gpt-4"]
	// autoTrimedModelPrefixes is ["openai", "deepseek"]
	// then the model "openai/gpt-4", "deepseek/deepseek-chat", "deepseek-chat", "gpt-4" will be accepted.
	AutoTrimedModelPrefixes []string `json:"autoTrimedModelPrefixes"`

	// ModelMappings add model alias for the model in the channels.
	// e.g. {"from": "deepseek-chat", "to": "deepseek/deepseek-chat"} will add a alias "deepseek-chat" for "deepseek/deepseek-chat".
	ModelMappings []ModelMapping `json:"modelMappings"`

	// HideOriginalModels hides the original models from the model list when model mappings are configured.
	// When enabled, only the mapped model names (from field) will be exposed, not the actual model names (to field).
	HideOriginalModels bool `json:"hideOriginalModels"`

	// HideMappedModels hides the mapped models from the model list when model mappings are configured.
	// When enabled, only the original model names (from field) will be exposed, not the mapped model names (to field).
	HideMappedModels bool `json:"hideMappedModels"`

	// OverrideParameters sets the channel override the request body.
	// A json string.
	// e.g. {"max_tokens": 100}, {"temperature": 0.7}
	// Deprecated Use bodyOverrideOperations instead.
	OverrideParameters string `json:"overrideParameters"`

	// BodyOverrideOperations sets the channel override operations for the request body.
	// When present (including an empty array), it takes precedence over OverrideParameters.
	BodyOverrideOperations []OverrideOperation `json:"bodyOverrideOperations,omitempty"`

	// OverrideHeaders sets the channel override the request headers.
	// e.g. [{"key": "User-Agent", "value": "AxonHub"}]
	// Supported ops: set (default), delete, rename, copy.
	// Deprecated Use headerOverrideOperations instead.
	OverrideHeaders []HeaderEntry `json:"overrideHeaders"`

	// HeaderOverrideOperations sets the channel override operations for request headers.
	// When present (including an empty array), it takes precedence over OverrideHeaders.
	HeaderOverrideOperations []OverrideOperation `json:"headerOverrideOperations,omitempty"`

	// Proxy configuration for the channel. If not set, defaults to environment proxy type.
	Proxy *httpclient.ProxyConfig `json:"proxy,omitempty"`

	// TransformOptions configures the transform options for the channel.
	TransformOptions TransformOptions `json:"transformOptions"`
}

// DisabledAPIKey 记录被禁用的 API key 信息（敏感，按 credentials 同级保护）
// 注意：禁用判断以 Key 明文为主键。
type DisabledAPIKey struct {
	Key        string    `json:"key"`
	DisabledAt time.Time `json:"disabledAt"`
	ErrorCode  int       `json:"errorCode"`
	Reason     string    `json:"reason,omitempty"`
}

type ChannelCredentials struct {
	// APIKey is the API key for the channel, for the single key channel, e.g. Codex, Claude code, Antigravity.
	// It is kept for backward compatibility with existing data, recommend to use OAuth instead.
	APIKey string `json:"apiKey,omitempty"`

	// OAuth is the OAuth credentials for the channel, for the OAuth channel, e.g. Codex, Claude code, Antigravity.
	OAuth *OAuthCredentials `json:"oauth,omitempty"`

	// APIKeys is a list of API keys for the channel.
	// When multiple keys are provided, they will be used in a round-robin fashion.
	APIKeys []string `json:"apiKeys,omitempty"`

	// Azure configuration for the channel.
	Azure *AzureCredential `json:"azure,omitempty"`

	// GCP is the GCP credentials for the channel.
	GCP *GCPCredential `json:"gcp,omitempty"`
}

// GetAllAPIKeys returns all API keys for the channel, combining APIKey and APIKeys fields.
// This ensures backward compatibility with old data that only has APIKey set.
func (c *ChannelCredentials) GetAllAPIKeys() []string {
	if c == nil {
		return nil
	}

	var keys []string

	// Add legacy APIKey if present (only if not OAuth credential)
	if c.APIKey != "" && !c.IsOAuth() {
		keys = append(keys, c.APIKey)
	}

	// Add new APIKeys
	keys = append(keys, c.APIKeys...)

	return keys
}

// GetEnabledAPIKeys returns API keys that are not disabled.
func (c *ChannelCredentials) GetEnabledAPIKeys(disabledKeys []DisabledAPIKey) []string {
	allKeys := c.GetAllAPIKeys()
	if len(disabledKeys) == 0 {
		return allKeys
	}

	disabledSet := make(map[string]struct{}, len(disabledKeys))
	for _, dk := range disabledKeys {
		if dk.Key == "" {
			continue
		}

		disabledSet[dk.Key] = struct{}{}
	}

	enabled := make([]string, 0, len(allKeys))
	for _, key := range allKeys {
		if _, ok := disabledSet[key]; ok {
			continue
		}

		enabled = append(enabled, key)
	}

	return enabled
}

// IsOAuth returns true if OAuth credentials are configured and valid.
// It checks both the new OAuth field and legacy APIKey field for backward compatibility.
func (c *ChannelCredentials) IsOAuth() bool {
	if c == nil {
		return false
	}

	// Check new OAuth field first
	if c.OAuth != nil && c.OAuth.AccessToken != "" {
		return true
	}

	// Backward compatibility: check if APIKey contains OAuth JSON
	return isOAuthJSON(c.APIKey)
}

// isOAuthJSON checks if a string is an OAuth JSON credential.
func isOAuthJSON(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") && strings.Contains(s, "access_token")
}

type OAuthCredentials = oauth.OAuthCredentials

type AzureCredential struct {
	// APIVersion is a optional version for the channel.
	APIVersion string `json:"apiVersion"`
}

type GCPCredential struct {
	Region    string `json:"region"`
	ProjectID string `json:"projectID"`
	JSONData  string `json:"jsonData"`
}

type GCPCredentialsJSON struct {
	Type                    string `json:"type" validate:"required"`
	ProjectID               string `json:"projectID" validate:"required"`
	PrivateKeyID            string `json:"privateKeyID" validate:"required"`
	PrivateKey              string `json:"privateKey" validate:"required"`
	ClientEmail             string `json:"clientEmail" validate:"required"`
	ClientID                string `json:"clientID" validate:"required"`
	AuthURI                 string `json:"authURI" validate:"required"`
	TokenURI                string `json:"tokenURI" validate:"required"`
	AuthProviderX509CertURL string `json:"authProviderX509CertURL" validate:"required"`
	ClientX509CertURL       string `json:"clientX509CertURL" validate:"required"`
	UniverseDomain          string `json:"universeDomain" validate:"required"`
}

type CapabilityPolicy string

const (
	CapabilityPolicyUnlimited CapabilityPolicy = "unlimited"
	CapabilityPolicyRequire   CapabilityPolicy = "require"
	CapabilityPolicyForbid    CapabilityPolicy = "forbid"
)

type ChannelPolicies struct {
	Stream CapabilityPolicy `json:"stream,omitempty"`
}

// ParseOverrideOperations parses the override parameters string.
// Supports both legacy map format (JSON object) and new operation array format (JSON array).
// Legacy format is automatically converted to OverrideOperation slice.
func ParseOverrideOperations(raw string) ([]OverrideOperation, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" || raw == "[]" {
		return nil, nil
	}

	if raw[0] == '[' {
		var ops []OverrideOperation
		if err := json.Unmarshal([]byte(raw), &ops); err != nil {
			return nil, fmt.Errorf("invalid override operations: %w", err)
		}

		return ops, nil
	}

	var legacy map[string]any
	if err := json.Unmarshal([]byte(raw), &legacy); err != nil {
		return nil, fmt.Errorf("invalid override parameters: %w", err)
	}

	ops := make([]OverrideOperation, 0, len(legacy))
	for key, value := range legacy {
		if strVal, ok := value.(string); ok && strVal == "__AXONHUB_CLEAR__" {
			ops = append(ops, OverrideOperation{Op: OverrideOpDelete, Path: key})
		} else {
			// Convert value to string
			var strValue string

			switch v := value.(type) {
			case string:
				strValue = v
			default:
				strValue = fmt.Sprintf("%v", value)
			}

			ops = append(ops, OverrideOperation{Op: OverrideOpSet, Path: key, Value: strValue})
		}
	}

	return ops, nil
}

// SerializeOverrideOperations converts override operations to a JSON string for storage.
func SerializeOverrideOperations(ops []OverrideOperation) (string, error) {
	if len(ops) == 0 {
		return "[]", nil
	}

	data, err := json.Marshal(ops)
	if err != nil {
		return "", fmt.Errorf("failed to serialize override operations: %w", err)
	}

	return string(data), nil
}
