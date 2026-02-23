package copilot

// DefaultModels returns a static list of GitHub Copilot model IDs.
//
// GitHub Copilot does not provide a public /models endpoint.
// These are the models available through GitHub Copilot's API.
func DefaultModels() []string {
	return []string{
		// GPT-4o models
		"gpt-4o",
		"gpt-4o-mini",
		// Claude models
		"claude-3.5-sonnet",
		"claude-3.7-sonnet",
		"claude-3.7-sonnet-thought",
		// o1 models
		"o1",
		"o1-mini",
		"o3",
		"o3-mini",
		"o4-mini",
		// Gemini models
		"gemini-2.0-flash-001",
		"gemini-2.5-pro-preview",
	}
}
