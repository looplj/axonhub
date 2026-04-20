package cacheidentity

// Config holds cache identity resolution settings. This mirrors
// server.OpenAICacheIdentity but lives in this package to avoid an
// import cycle between internal/server and internal/server/cacheidentity.
type Config struct {
	Enabled                      bool     `conf:"enabled" yaml:"enabled" json:"enabled"`
	ExtraSessionHeaders          []string `conf:"extra_session_headers" yaml:"extra_session_headers" json:"extra_session_headers"`
	ExtraSessionBodyFields       []string `conf:"extra_session_body_fields" yaml:"extra_session_body_fields" json:"extra_session_body_fields"`
	DeriveFromConversationAnchor bool     `conf:"derive_from_conversation_anchor" yaml:"derive_from_conversation_anchor" json:"derive_from_conversation_anchor"`
	AnchorMaxBytes               int      `conf:"anchor_max_bytes" yaml:"anchor_max_bytes" json:"anchor_max_bytes"`
}
