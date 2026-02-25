package copilot

// ProviderConfURL is the URL to fetch the provider configuration.
// This contains model listings for GitHub Copilot.
// Using a specific immutable reference (tag or commit SHA) for stability and security.
// IMPORTANT: Update ProviderConfSHA256 when changing this URL.
const ProviderConfURL = "https://raw.githubusercontent.com/ThinkInAIXYZ/PublicProviderConf/refs/heads/main/dist/all.json"

// ProviderConfSHA256 is the SHA256 hash of the expected provider configuration file.
// This should be updated whenever ProviderConfURL is changed to a new version.
// You can obtain this by running: sha256sum dist/all.json
const ProviderConfSHA256 = "128b6c01d21fd145f3aa511355319c2ab6a5eaa560c86bce561ffc7fd82fde43"

// ProviderID is the provider identifier in the PublicProviderConf.
const ProviderID = "github-copilot"
