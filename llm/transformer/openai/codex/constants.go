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
		"gpt-5.3-codex",
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
	UserAgent           = "codex_cli_rs/0.98.0 (Mac OS 15.6.1; arm64) iTerm.app/3.6.6"
	codexDefaultVersion = "0.98.0"
)

// CodexInstructions is the default system prompt for Codex CLI.
// Kept in sync with the Codex CLI reference prompt for compatibility.
const (
	CodexInstructions = "You are a coding agent running in the Codex CLI, a terminal-based coding assistant. Codex CLI is an open source project led by OpenAI. You are expected to be precise, safe, and helpful.\n\nYour capabilities:\n- Receive user prompts and other context provided by the harness, such as files in the workspace.\n- Communicate with the user by streaming thinking & responses, and by making & updating plans.\n- Emit function calls to run terminal commands and apply edits.  Depending on how this specific run is configured, you can request that these function calls be escalated to the user for approval before running. "

	CodexInstructionPrefix = "You are a coding agent running in the Codex CLI"
)
