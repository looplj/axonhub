package codex

// DefaultModels returns a static list of Codex-capable model IDs.
//
// The ChatGPT Codex backend does not provide a stable public /models endpoint.
// CLIProxyAPI keeps a local registry; we mirror that approach to power AxonHub "Fetch Models".
func DefaultModels() []string {
	return []string{
		"gpt-5.6-sol",
		"gpt-5.6-terra",
		"gpt-5.6-luna",
		"gpt-6-astra",
		"gpt-6-sol",
		"gpt-6-luna",
		"gpt-6.1-sol",
		"codex-auto-review",
	}
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

	codexDefaultVersion = "0.159.0"

	// fabricatedBetaFeatures mirrors the X-Codex-Beta-Features value the current
	// Codex CLI sends, used when a non-Codex inbound client omits the header.
	fabricatedBetaFeatures = "remote_compaction_v2"
)
