package copilot

// ProviderConfURL is the URL to fetch the provider configuration.
// This contains model listings for GitHub Copilot.
// Using dev branch for latest updates, fetching standalone config file.
// IMPORTANT: Update ProviderConfSHA256 when changing this URL.
const ProviderConfURL = "https://raw.githubusercontent.com/ThinkInAIXYZ/PublicProviderConf/dev/dist/github-copilot.json"

// ProviderConfSHA256 is the SHA256 hash of the expected provider configuration file.
// This should be updated whenever ProviderConfURL is changed to a new version.
// You can obtain this by running: sha256sum dist/github-copilot.json
const ProviderConfSHA256 = "a1590da9b1fadb0156797dbc52fc309a600adb338cd6bab75a7eba318852b901"

// ProviderID is the provider identifier in the PublicProviderConf.
const ProviderID = "github-copilot"
