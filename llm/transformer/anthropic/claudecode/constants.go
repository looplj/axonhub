package claudecode

// DefaultModels returns a static list of Claude Code-capable model IDs.
func DefaultModels() []string {
	return []string{
		"claude-haiku-4-5-20251001",
		"claude-sonnet-4-5-20250929",
		"claude-opus-4-5-20251101",
		"claude-opus-4-6",
		"claude-sonnet-4-6",
	}
}

const (
	AuthorizeURL = "https://claude.ai/oauth/authorize"
	//nolint:gosec // false alert.
	TokenURL = "https://console.anthropic.com/v1/oauth/token"
	ClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"

	RedirectURI = "http://localhost:54545/callback"
	Scopes      = "org:create_api_key user:profile user:inference"
	// UserAgent keep consistent with Claude CLI.
	UserAgent = "claude-cli/2.1.110 (external, cli)"

	// ClaudeCodeBetaHeader contains the beta feature identifiers for Claude Code API.
	ClaudeCodeBetaHeader = "claude-code-20250219,interleaved-thinking-2025-05-14,redact-thinking-2026-02-12,context-management-2025-06-27,prompt-caching-scope-2026-01-05,effort-2025-11-24"

	// ClaudeCodeVersionHeader specifies the API version for Claude Code.
	ClaudeCodeVersionHeader = "2023-06-01"
	// ClaudeCodeBrowserAccessHeader enables browser access capabilities.
	ClaudeCodeBrowserAccessHeader = "true"
	// ClaudeCodeAppHeader identifies the client application type.
	ClaudeCodeAppHeader = "cli"
	// ClaudeCodeQuotaCheckModel is the model used for quota checking requests.
	ClaudeCodeQuotaCheckModel = "claude-haiku-4-5"
)
