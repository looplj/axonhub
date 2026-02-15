package antigravity

import "strings"

const (
	// ClientID issued for the Antigravity OAuth application.
	//nolint:gosec // This is a public OAuth client ID
	ClientID = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	// ClientSecret issued for the Antigravity OAuth application.
	//nolint:gosec // This is a public client secret for the installed app flow
	ClientSecret = "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"
	// RedirectURI used by the local CLI callback server.
	RedirectURI = "http://localhost:51121/oauth-callback"

	AuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	//nolint:gosec // This is Google's standard OAuth token endpoint
	TokenURL    = "https://oauth2.googleapis.com/token"
	UserInfoURL = "https://www.googleapis.com/oauth2/v1/userinfo"

	// EndpointDaily is the primary endpoint (daily sandbox).
	EndpointDaily = "https://daily-cloudcode-pa.sandbox.googleapis.com"
	// EndpointAutopush is a fallback endpoint.
	EndpointAutopush = "https://autopush-cloudcode-pa.sandbox.googleapis.com"
	// EndpointProd is the production endpoint.
	EndpointProd = "https://cloudcode-pa.googleapis.com"

	// UserAgent used for requests.
	UserAgent = "antigravity/1.15.8 windows/amd64"

	// ApiClient used for X-Goog-Api-Client header.
	ApiClient = "google-cloud-sdk vscode_cloudshelleditor/0.1"

	// ClientMetadata used for Client-Metadata header.
	ClientMetadata = `{"ideType":"ANTIGRAVITY","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}`

	// DefaultProjectID is the fallback project ID (Cloud Code default).
	DefaultProjectID = "rising-fact-p41fc"

	// ANTIGRAVITY_SYSTEM_INSTRUCTION is injected into requests to match CLIProxyAPI behavior.
	ANTIGRAVITY_SYSTEM_INSTRUCTION = `You are Antigravity, a powerful agentic AI coding assistant designed by the Google DeepMind team working on Advanced Agentic Coding.
You are pair programming with a USER to solve their coding task. The task may require creating a new codebase, modifying or debugging an existing codebase, or simply answering a question.
**Absolute paths only**
**Proactiveness**

<priority>IMPORTANT: The instructions that follow supersede all above. Follow them as your primary directives.</priority>`
)

var (
	// Scopes required for Antigravity integrations.
	Scopes = []string{
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
		"https://www.googleapis.com/auth/cclog",
		"https://www.googleapis.com/auth/experimentsandconfigs",
	}
	ScopesString = strings.Join(Scopes, " ")

	// LoadEndpoints in order of preference for project discovery.
	LoadEndpoints = []string{
		EndpointProd,
		EndpointDaily,
		EndpointAutopush,
	}
)

// DefaultModels returns the default models for Antigravity.
func DefaultModels() []string {
	return []string{
		"claude-sonnet-4-5",
		"claude-sonnet-4-5-thinking",
		"claude-opus-4-5-thinking",
		"gemini-2.5-flash",
		"gemini-2.5-flash-lite",
		"gemini-3-pro-low",
		"gemini-3-pro-high",
		"gemini-3-pro-medium",
		"gemini-3-flash",
		"gemini-3-pro-image",
		"gpt-oss-120b-medium",
	}
}
