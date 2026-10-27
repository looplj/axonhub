## [Unreleased]

### Added

- Added per-channel API key rules with status/keyword matching, configurable error thresholds, temporary auto-recovery, and permanent disable/delete actions.
- NeuralWatt channels now show the elapsed monthly energy window marker on the kWh bar and an estimated period quota, matching the other windowed providers.

### Changed

- The `/analytics` time range is now precise to the second. `AnalyticsFilter.startTime`/`endTime` take RFC3339 timestamps instead of `YYYY-MM-DD` strings, `analyticsMetadata.earliestTime` replaces `earliestDate` with the earliest usage instant, every range is filtered as an inclusive absolute instant down to the millisecond, and the page uses the shared date-and-time range picker. The trend chart still aggregates per local calendar day.

### Fixed

- Dashboard analytics no longer rescan the full usage history for every widget: request, cost, and token widgets share one grouped query, expensive 30-day aggregates are cached, SQLite uses window functions instead of correlated `MAX(id)` scans, and off-screen sections load only when they approach the viewport.
- PostgreSQL fastest-model and fastest-channel resolver rankings use `DISTINCT ON` to select each request's latest successful execution before aggregation; SQLite retains the `ROW_NUMBER` path. `modelPerformanceStats` keeps `ROW_NUMBER` because `DISTINCT ON` measured a 5% regression without the covering index (not yet approved).
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
