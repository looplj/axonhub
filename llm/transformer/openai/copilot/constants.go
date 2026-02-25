package copilot

// ProviderConfURL is the URL to fetch the latest provider configuration.
// This contains up-to-date model listings for GitHub Copilot.
// Using branch HEAD for automatic updates instead of fixed commit SHA.
const ProviderConfURL = "https://raw.githubusercontent.com/ThinkInAIXYZ/PublicProviderConf/refs/heads/main/dist/all.json"

// ProviderID is the provider identifier in the PublicProviderConf.
const ProviderID = "github-copilot"
