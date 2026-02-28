package vertex

// ProviderConfURL is the URL to fetch the provider configuration.
// This contains model listings for Google Vertex AI.
const ProviderConfURL = "https://raw.githubusercontent.com/ThinkInAIXYZ/PublicProviderConf/dev/dist/google-vertex.json"

// ProviderConfSHA256 is the SHA256 hash of the expected provider configuration file.
// This should be updated whenever ProviderConfURL is changed to a new version.
// If empty, integrity checking is skipped (useful during development or when hash is not yet known).
const ProviderConfSHA256 = ""

// ProviderID is the provider identifier in the PublicProviderConf.
const ProviderID = "google-vertex"
