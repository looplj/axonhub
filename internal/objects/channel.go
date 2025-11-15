package objects

type ProxyType string

const (
	ProxyTypeDisabled    ProxyType = "disabled"    // Do not use proxy
	ProxyTypeEnvironment ProxyType = "environment" // Use environment variables (HTTP_PROXY, etc.)
	ProxyTypeURL         ProxyType = "url"         // Use configured URL
)

type ProxyConfig struct {
	Type     ProxyType `json:"type"`          // disabled, environment, or url
	URL      string    `json:"url,omitempty"` // e.g., "http://proxy.example.com:8080"
	Username string    `json:"username,omitempty"`
	Password string    `json:"password,omitempty"`
}

type ModelMapping struct {
	// From is the model name in the request.
	From string `json:"from"`

	// To is the model name in the provider.
	To string `json:"to"`
}

type ChannelSettings struct {
	// ExtraModelPrefix sets the channel accept the model with the extra prefix.
	// e.g. a channel
	// supported_modles is ["deepseek-chat", "deepseek-reasoner"]
	// extraModelPrefix is "deepseek"
	// then the model "deepseek-chat", "deepseek-reasoner", "deepseek/deepseek-chat", "deepseek/deepseek-reasoner"  will be accepted.
	// And if other channel support "deepseek/deepseek-chat", "deepseek/deepseek-reasoner" modles, the two channels can accept the request both.
	ExtraModelPrefix string `json:"extraModelPrefix"`

	// ModelMappings add model alias for the model in the channels.
	// e.g. {"from": "deepseek-chat", "to": "deepseek/deepseek-chat"} will add a alias "deepseek-chat" for "deepseek/deepseek-chat".
	ModelMappings []ModelMapping `json:"modelMappings"`

	// OverrideParameters sets the channel override the request parameters.
	// e.g. {"max_tokens": 100}, {"temperature": 0.7}
	OverrideParameters string `json:"overrideParameters"`

	// Proxy configuration for the channel. If not set, defaults to environment proxy type.
	Proxy *ProxyConfig `json:"proxy,omitempty"`
}

type ChannelCredentials struct {
	// APIKey is the API key for the channel.
	APIKey string `json:"apiKey,omitempty"`

	// AWS is the AWS credentials for the channel.
	AWS *AWSCredential `json:"aws,omitempty"`

	// GCP is the GCP credentials for the channel.
	GCP *GCPCredential `json:"gcp,omitempty"`
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
	ProjectID               string `json:"project_id" validate:"required"`
	PrivateKeyID            string `json:"private_key_id" validate:"required"`
	PrivateKey              string `json:"private_key" validate:"required"`
	ClientEmail             string `json:"client_email" validate:"required"`
	ClientID                string `json:"client_id" validate:"required"`
	AuthURI                 string `json:"auth_uri" validate:"required"`
	TokenURI                string `json:"token_uri" validate:"required"`
	AuthProviderX509CertURL string `json:"auth_provider_x509_cert_url" validate:"required"`
	ClientX509CertURL       string `json:"client_x509_cert_url" validate:"required"`
	UniverseDomain          string `json:"universe_domain" validate:"required"`
}
