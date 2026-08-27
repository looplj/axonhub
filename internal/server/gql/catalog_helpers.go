package gql

import "github.com/looplj/axonhub/internal/server/biz"

func providersCatalogFromSnapshot(snapshot biz.CatalogSnapshot) *ProvidersCatalog {
	return &ProvidersCatalog{
		Data:      snapshot.Data,
		FetchedAt: snapshot.FetchedAt,
		Source:    snapshot.Source,
		Filtered:  snapshot.Filtered,
	}
}
