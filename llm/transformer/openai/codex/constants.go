package codex

// DefaultModels returns a static list of Codex-capable model IDs.
//
// The ChatGPT Codex backend does not provide a stable public /models endpoint.
// CLIProxyAPI keeps a local registry; we mirror that approach to power AxonHub "Fetch Models".
func DefaultModels() []string {
	return []string{
		"gpt-5",
		"gpt-5-codex",
		"gpt-5-codex-mini",
		"gpt-5.1",
		"gpt-5.1-codex",
		"gpt-5.1-codex-mini",
		"gpt-5.1-codex-max",
		"gpt-5.2",
		"gpt-5.2-codex",
	}
}

const (
	AuthorizeURL = "https://auth.openai.com/oauth/authorize"
	//nolint:gosec // false alert.
	TokenURL    = "https://auth.openai.com/oauth/token"
	ClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	RedirectURI = "http://localhost:1455/auth/callback"
	Scopes      = "openid profile email offline_access"
	// UserAgent keep consistent with Codex CLI.
	UserAgent = "codex_cli_rs/0.38.0 (Ubuntu 22.04.0; x86_64) WindowsTerminal"
)
