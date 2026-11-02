/**
 * Upper bound for the retry policy response timeouts, in seconds.
 *
 * The backend is the authority: it rejects a per-channel override above
 * `maxRetryResponseTimeoutSeconds` in `internal/server/biz/system.go` and clamps
 * the global values to the same limit. A literal cannot be shared across Go and
 * TypeScript, so this copy only drives the `max` hint on the number inputs.
 *
 * Keep the channel response schemas permissive about this upper bound. A schema
 * that rejected it would fail to parse a channel the backend accepted, which
 * takes down the whole channel list instead of surfacing one bad field.
 */
export const RESPONSE_TIMEOUT_MAX_SECONDS = 600;
