package api

import (
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/server/biz"
)

type APIKeyHandlersParams struct {
	fx.In

	APIKeyService *biz.APIKeyService
}

type APIKeyHandlers struct {
	APIKeyService *biz.APIKeyService
}

func NewAPIKeyHandlers(params APIKeyHandlersParams) *APIKeyHandlers {
	return &APIKeyHandlers{
		APIKeyService: params.APIKeyService,
	}
}
