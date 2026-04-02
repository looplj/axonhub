package shared

import "context"

const (
	MetadataKeyBaseURL         = "transformer_base_url"
	MetadataKeyAccountIdentity = "transformer_account_identity"
)

type TransportScope struct {
	BaseURL         string
	AccountIdentity string
}

func (s TransportScope) Footprint() string {
	return ComputeFootprint(s.BaseURL, s.AccountIdentity)
}

func (s TransportScope) Metadata() map[string]string {
	if s.AccountIdentity == "" {
		return nil
	}

	metadata := map[string]string{}
	if s.BaseURL != "" {
		metadata[MetadataKeyBaseURL] = s.BaseURL
	}
	if s.AccountIdentity != "" {
		metadata[MetadataKeyAccountIdentity] = s.AccountIdentity
	}

	return metadata
}

func ScopeFromMetadata(metadata map[string]string) TransportScope {
	if metadata == nil {
		return TransportScope{}
	}

	return TransportScope{
		BaseURL:         metadata[MetadataKeyBaseURL],
		AccountIdentity: metadata[MetadataKeyAccountIdentity],
	}
}

type transportScopeContextKey struct{}

func ContextWithTransportScope(ctx context.Context, scope TransportScope) context.Context {
	if scope.BaseURL == "" && scope.AccountIdentity == "" {
		return ctx
	}

	return context.WithValue(ctx, transportScopeContextKey{}, scope)
}

func GetTransportScope(ctx context.Context) (TransportScope, bool) {
	if ctx == nil {
		return TransportScope{}, false
	}

	scope, ok := ctx.Value(transportScopeContextKey{}).(TransportScope)
	return scope, ok
}
