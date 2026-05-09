package openapi

import (
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

// toOpenAPIAPIKey projects the rich ent.APIKey down to the minimal OpenAPI
// surface — only what programmatic callers need (id/key/name/scopes/profiles).
//
// Lives in its own file (not openapi.resolvers.go) so gqlgen's regeneration
// pass doesn't sweep it into a warning block as "unknown code".
func toOpenAPIAPIKey(k *ent.APIKey) *APIKey {
	if k == nil {
		return nil
	}

	return &APIKey{
		ID:       objects.GUID{Type: "APIKey", ID: k.ID},
		Key:      k.Key,
		Name:     k.Name,
		Scopes:   k.Scopes,
		Profiles: k.Profiles,
	}
}
