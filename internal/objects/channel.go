package objects

import "github.com/looplj/axonhub/llm/httpclient"

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

type TransformOptions struct {
	// ForceArrayInstructions forces the channel to accept array format for instructions.
	ForceArrayInstructions bool `json:"forceArrayInstructions"`

	// ForceArrayInputs forces the channel to accept array format for inputs.
	ForceArrayInputs bool `json:"forceArrayInputs"`
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
	OverrideParameters string `json:"overrideParameters"`

	// OverrideHeaders sets the channel override the request headers.
	// e.g. [{"key": "User-Agent", "value": "AxonHub"}]
	OverrideHeaders []HeaderEntry `json:"overrideHeaders"`

	// Proxy configuration for the channel. If not set, defaults to environment proxy type.
	Proxy *httpclient.ProxyConfig `json:"proxy,omitempty"`

	// TransformOptions configures the transform options for the channel.
	TransformOptions TransformOptions `json:"transformOptions"`
}

type ChannelCredentials struct {
	// APIKey is the API key for the channel.
	APIKey string `json:"apiKey,omitempty"`

	// PlatformType distinguishes different platform configurations (e.g., "openai", "azure").
	// e.g., "openai", "azure", "anthropic", "vertex" etc.
	PlatformType string `json:"platformType,omitempty"`

	// Azure configuration for the channel.
	Azure *AzureCredential `json:"azure,omitempty"`

	// AWS is the AWS credentials for the channel.
	AWS *AWSCredential `json:"aws,omitempty"`

	// GCP is the GCP credentials for the channel.
	GCP *GCPCredential `json:"gcp,omitempty"`
}

type AzureCredential struct {
	// APIVersion is a optional version for the channel.
	APIVersion string `json:"apiVersion"`
}

type AWSCredential struct {
	Region          string `json:"region"`
	AccessKeyID     string `json:"accessKeyID"`
	SecretAccessKey string `json:"secretAccessKey"`
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
