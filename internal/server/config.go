package server

import (
	"time"

	"github.com/looplj/axonhub/internal/tracing"
)

type Config struct {
	Host        string        `conf:"host" yaml:"host" json:"host"`
	Port        int           `conf:"port" yaml:"port" json:"port"`
	Name        string        `conf:"name" yaml:"name" json:"name"`
	BasePath    string        `conf:"base_path" yaml:"base_path" json:"base_path"`
	ReadTimeout time.Duration `conf:"read_timeout" yaml:"read_timeout" json:"read_timeout"`

	// RequestTimeout is the maximum duration for processing a request.
	RequestTimeout time.Duration `conf:"request_timeout" yaml:"request_timeout" json:"request_timeout"`

	// LLMRequestTimeout is the maximum duration for processing a request to LLM.
	LLMRequestTimeout time.Duration `conf:"llm_request_timeout" yaml:"llm_request_timeout" json:"llm_request_timeout"`

	Trace     tracing.Config `conf:"trace" yaml:"trace" json:"trace"`
	Dashboard Dashboard      `conf:"dashboard" yaml:"dashboard" json:"dashboard"`

	Debug            bool `conf:"debug" yaml:"debug" json:"debug"`
	DisableSSLVerify bool `conf:"disable_ssl_verify" yaml:"disable_ssl_verify" json:"disable_ssl_verify"`
	CORS             CORS `conf:"cors" yaml:"cors" json:"cors"`
	API              API  `conf:"api" yaml:"api" json:"api"`

	OpenAICacheIdentity OpenAICacheIdentity `conf:"openai_cache_identity" yaml:"openai_cache_identity" json:"openai_cache_identity"`
}

// OpenAICacheIdentity configures upstream-native cache identity resolution
// for OpenAI-compatible traffic. When enabled, AxonHub derives stable
// session IDs and prompt cache keys for clients that do not send explicit
// session identifiers.
type OpenAICacheIdentity struct {
	// Enabled controls whether cache identity resolution runs.
	// Default: true.
	Enabled bool `conf:"enabled" yaml:"enabled" json:"enabled"`

	// ExtraSessionHeaders lists additional HTTP headers to inspect for
	// stable session identifiers beyond the built-in candidates:
	//   - Session_id (Codex direct header)
	//   - X-Codex-Turn-Metadata (session_id extracted via codex.GetSessionIDFromHeaders)
	//   - AH-Thread-Id (configurable thread header)
	//   - Conversation_id
	ExtraSessionHeaders []string `conf:"extra_session_headers" yaml:"extra_session_headers" json:"extra_session_headers"`

	// ExtraSessionBodyFields lists additional JSON body fields (gjson paths)
	// to inspect for stable session identifiers beyond the built-in candidates
	// (metadata.session_id, metadata.conversation_id).
	ExtraSessionBodyFields []string `conf:"extra_session_body_fields" yaml:"extra_session_body_fields" json:"extra_session_body_fields"`

	// DeriveFromConversationAnchor controls whether a deterministic
	// conversation-anchor fingerprint is derived when no explicit session
	// identity is available.
	// Default: true.
	DeriveFromConversationAnchor bool `conf:"derive_from_conversation_anchor" yaml:"derive_from_conversation_anchor" json:"derive_from_conversation_anchor"`

	// AnchorMaxBytes is the maximum number of bytes of the canonicalized
	// anchor content fed to the hash function. Larger values improve
	// uniqueness for long system prompts at a negligible CPU cost.
	// Default: 32768.
	AnchorMaxBytes int `conf:"anchor_max_bytes" yaml:"anchor_max_bytes" json:"anchor_max_bytes"`
}

// Dashboard holds configuration for the dashboard cache settings.
type Dashboard struct {
	// AllTimeTokenStatsSoftTTL is the duration after which cached all-time token stats
	// are considered stale and will be refreshed asynchronously (stale-while-revalidate).
	// Default: 1 hour
	AllTimeTokenStatsSoftTTL time.Duration `conf:"all_time_token_stats_soft_ttl" yaml:"all_time_token_stats_soft_ttl" json:"all_time_token_stats_soft_ttl"`

	// AllTimeTokenStatsHardTTL is the maximum duration for which cached all-time token stats
	// are considered valid. After this, synchronous refresh is required.
	// Default: 24 hours
	AllTimeTokenStatsHardTTL time.Duration `conf:"all_time_token_stats_hard_ttl" yaml:"all_time_token_stats_hard_ttl" json:"all_time_token_stats_hard_ttl"`
}

type CORS struct {
	Enabled          bool          `conf:"enabled" yaml:"enabled" json:"enabled"`
	AllowedOrigins   []string      `conf:"allowed_origins" yaml:"allowed_origins" json:"allowed_origins"`
	AllowedMethods   []string      `conf:"allowed_methods" yaml:"allowed_methods" json:"allowed_methods"`
	AllowedHeaders   []string      `conf:"allowed_headers" yaml:"allowed_headers" json:"allowed_headers"`
	ExposedHeaders   []string      `conf:"exposed_headers" yaml:"exposed_headers" json:"exposed_headers"`
	AllowCredentials bool          `conf:"allow_credentials" yaml:"allow_credentials" json:"allow_credentials"`
	MaxAge           time.Duration `conf:"max_age" yaml:"max_age" json:"max_age"`
}

type API struct {
	Auth APIAuth `conf:"auth" yaml:"auth" json:"auth"`
}

type APIAuth struct {
	AllowNoAuth bool `conf:"allow_no_auth" yaml:"allow_no_auth" json:"allow_no_auth"`
}
