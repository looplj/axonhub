package copilot

// ProviderConfURL is the URL to fetch the provider configuration.
// This contains model listings for GitHub Copilot.
// Using dev branch for latest updates, fetching standalone config file.
const ProviderConfURL = "https://raw.githubusercontent.com/ThinkInAIXYZ/PublicProviderConf/dev/dist/github-copilot.json"

// ProviderConfSHA256 is the SHA256 hash of the expected provider configuration file.
// This should be updated whenever ProviderConfURL is changed to a new version.
// If empty, integrity checking is skipped (useful during development or when hash is not yet known).
const ProviderConfSHA256 = ""

// ProviderID is the provider identifier in the PublicProviderConf.
const ProviderID = "github-copilot"
