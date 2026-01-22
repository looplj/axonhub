package claudecode

// DefaultModels returns a static list of Claude Code-capable model IDs.
func DefaultModels() []string {
	return []string{
		"claude-haiku-4-5-20251001",
		"claude-sonnet-4-5-20250929",
		"claude-opus-4-5-20251101",
	}
}

const (
	AuthorizeURL = "https://claude.ai/oauth/authorize"
	//nolint:gosec // false alert.
	TokenURL    = "https://console.anthropic.com/v1/oauth/token"
	ClientID    = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	RedirectURI = "http://localhost:54545/callback"
	Scopes      = "org:create_api_key user:profile user:inference"
	// UserAgent keep consistent with Claude CLI.
	UserAgent = "claude-cli/1.0.83 (external, cli)"
)
