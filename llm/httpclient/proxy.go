package httpclient

type ProxyType string

const (
	ProxyTypeDisabled    ProxyType = "disabled"
	ProxyTypeEnvironment ProxyType = "environment"
	ProxyTypeURL         ProxyType = "url"
)

type ProxyConfig struct {
	Type     ProxyType `json:"type"`
	URL      string    `json:"url,omitempty"`
	Username string    `json:"username,omitempty"`
	Password string    `json:"password,omitempty"`
}
