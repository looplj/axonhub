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

	AWS *struct {
		AccessKeyID     string `json:"accessKeyID"`
		SecretAccessKey string `json:"secretAccessKey"`
		Region          string `json:"region"`
	} `json:"aws,omitempty"`
}
