## [Unreleased]

### Added

- Added per-channel API key rules with status/keyword matching, configurable error thresholds, temporary auto-recovery, and permanent disable/delete actions.
- NeuralWatt channels now show the elapsed monthly energy window marker on the kWh bar and an estimated period quota, matching the other windowed providers.

### Fixed

- Dashboard analytics no longer rescan the full usage history for every widget: request, cost, and token widgets share one grouped query, expensive 30-day aggregates are cached, SQLite uses window functions instead of correlated `MAX(id)` scans, and off-screen sections load only when they approach the viewport.
- Personal API Key names can be reused by different creators in a project; creating or renaming a key rejects collisions with non-personal keys visible to its creator. Duplicate checks run after create authorization.
- Synthetic weekly credit amounts that parse to NaN or infinity are now ignored instead of leaking into the period money fields and breaking quota persistence.

### Removed

- Wafer quota monitor removed — Wafer is PAYG-only. Legacy wafer quota rows are purged at startup before schema migration when automatic migration is enabled.

v0.4.0

- Introduced thread-aware tracing with zero-SDK integration and configurable trace headers
- Added trace visualization interface for following end-to-end conversations
- Added configurable data storage policies to keep or trim trace payloads based on compliance needs

v0.3.0

- Launched project workspace management with per-project API keys and dashboards
- Improved project-scoped permissions and usage insights

v0.2.0

- Added multimodal image generation via chat completions and image_generation tools
- Documented provider support and sample usage for image workflows

v0.1.1

- Add OpenRouter outbound transformer
- Support Google Gemini OpenAI compatible API
