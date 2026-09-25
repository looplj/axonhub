package codex

type fastModelPair struct {
	model string
	alias string
}

var fastModelPairs = []fastModelPair{
	{model: "gpt-5.6-sol", alias: "gpt-5.6-sol-fast"},
	{model: "gpt-5.6-terra", alias: "gpt-5.6-terra-fast"},
	{model: "gpt-5.6-luna", alias: "gpt-5.6-luna-fast"},
	{model: "gpt-6-astra", alias: "gpt-6-astra-fast"},
	{model: "gpt-6-sol", alias: "gpt-6-sol-fast"},
	{model: "gpt-6-luna", alias: "gpt-6-luna-fast"},
}

// DefaultModels returns a static list of Codex-capable model IDs.
//
// The ChatGPT Codex backend does not provide a stable public /models endpoint.
// CLIProxyAPI keeps a local registry; we mirror that approach to power AxonHub "Fetch Models".
func DefaultModels() []string {
	models := make([]string, 0, len(fastModelPairs)*2+1)
	for _, pair := range fastModelPairs {
		models = append(models, pair.model, pair.alias)
	}
	return append(models, "codex-auto-review")
}

func fastModelBase(model string) (string, bool) {
	for _, pair := range fastModelPairs {
		if pair.alias == model {
			return pair.model, true
		}
	}

	return "", false
}

const (
	defaultImageMainModel = "gpt-5.4-mini"

	AxonHubOriginator = "axonhub"
	AuthorizeURL      = "https://auth.openai.com/oauth/authorize"
	//nolint:gosec // false alert.
	TokenURL    = "https://auth.openai.com/oauth/token"
	ClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	RedirectURI = "http://localhost:1455/auth/callback"
	Scopes      = "openid profile email offline_access"

	codexDefaultVersion = "0.156.0"

	// fabricatedBetaFeatures mirrors the X-Codex-Beta-Features value the current
	// Codex CLI sends, used when a non-Codex inbound client omits the header.
	fabricatedBetaFeatures = "remote_compaction_v2"
)
