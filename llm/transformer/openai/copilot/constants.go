package copilot

// ProviderConfURL is the URL to fetch the provider configuration.
// This contains model listings for GitHub Copilot.
// Using a specific immutable reference (tag or commit SHA) for stability and security.
// IMPORTANT: Update ProviderConfSHA256 when changing this URL.
const ProviderConfURL = "https://raw.githubusercontent.com/ThinkInAIXYZ/PublicProviderConf/refs/heads/main/dist/all.json"

// ProviderConfSHA256 is the SHA256 hash of the expected provider configuration file.
// This should be updated whenever ProviderConfURL is changed to a new version.
// You can obtain this by running: sha256sum dist/all.json
const ProviderConfSHA256 = "" // TODO: Update with actual SHA256 hash after pinning URL version

// ProviderID is the provider identifier in the PublicProviderConf.
const ProviderID = "github-copilot"
