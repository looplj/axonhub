package openapi

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

// resolveAPIKey loads an API key through the privacy-gated read path from the
// caller-facing identifier set: typed GUID, plaintext key, or project-scoped
// name. guidID rejects wrong-typed GUIDs; GetForRead enforces exactly-one-of
// and the read_api_keys / own-project privacy policy, so foreign or missing
// keys surface as NotFound.
func (r *Resolver) resolveAPIKey(ctx context.Context, id *objects.GUID, key *string, name *string) (*ent.APIKey, error) {
	keyID, err := guidID(id, ent.TypeAPIKey)
	if err != nil {
		return nil, err
	}

	return r.apiKeyService.GetForRead(ctx, keyID, key, name)
}

// resolveTemplate is the template counterpart of resolveAPIKey (id or name).
func (r *Resolver) resolveTemplate(ctx context.Context, id *objects.GUID, name *string) (*ent.APIKeyProfileTemplate, error) {
	templateID, err := guidID(id, ent.TypeAPIKeyProfileTemplate)
	if err != nil {
		return nil, err
	}

	return r.apiKeyProfileTemplateService.GetForRead(ctx, templateID, name)
}

// guidID validates that the GUID, when present, carries the expected ent type
// and extracts its numeric id. UnmarshalGQL accepts any gid://axonhub/<type>/<id>,
// so resolvers must reject GUIDs of the wrong type before any DB lookup. A nil
// GUID passes through as a nil id so callers can feed the result straight into
// the exactly-one-of validation in the biz GetForRead helpers.
func guidID(g *objects.GUID, expectedType string) (*int, error) {
	if g == nil {
		return nil, nil
	}

	if g.Type != expectedType {
		return nil, fmt.Errorf("invalid id: expected a %s GUID, got %s", expectedType, g.Type)
	}

	return &g.ID, nil
}

// toOpenAPIAPIKey projects the rich ent.APIKey down to the minimal OpenAPI
// surface — only what programmatic callers need (id/key/name/scopes/profiles).
//
// Lives in its own file (not openapi.resolvers.go) so gqlgen's regeneration
// pass doesn't sweep it into a warning block as "unknown code".
func toOpenAPIAPIKey(ctx context.Context, k *ent.APIKey, created bool) *APIKey {
	if k == nil {
		return nil
	}

	key := ""
	caller, ok := contexts.GetAPIKey(ctx)
	if created || (ok && caller != nil && caller.ID == k.ID && caller.Key == k.Key) {
		key = k.Key
	}
	return &APIKey{
		ID:       objects.GUID{Type: "APIKey", ID: k.ID},
		Key:      key,
		Name:     k.Name,
		Scopes:   k.Scopes,
		Profiles: k.Profiles,
	}
}
