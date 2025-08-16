package objects

type ModelMapping struct {
	// From is the model name in the request.
	From string `json:"from"`

	// To is the model name in the provider.
	To string `json:"to"`
}

type ChannelSettings struct {
	ModelMappings []ModelMapping
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
