package copilot

// DefaultModels returns a static list of GitHub Copilot model IDs.
//
// GitHub Copilot does not provide a public /models endpoint.
// These are the models available through GitHub Copilot's API.
func DefaultModels() []string {
	return []string{
		"gpt-4o",
		"gpt-4o-mini",
		"claude-3.5-sonnet",
		"o1",
		"o3-mini",
	}
}
