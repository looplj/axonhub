package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
)

type AbstractService struct {
	db *ent.Client
}

func (a *AbstractService) entFromContext(ctx context.Context) *ent.Client {
	db := ent.FromContext(ctx)
	if db != nil {
		return db
	}

	return a.db
}
