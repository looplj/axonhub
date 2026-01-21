package biz

import (
	"errors"

	"github.com/looplj/axonhub/llm/transformer"
)

var (
	ErrInvalidJWT             = errors.New("invalid jwt token")
	ErrInvalidAPIKey          = errors.New("invalid api key")
	ErrInvalidPassword        = errors.New("invalid password")
	ErrInvalidModel           = transformer.ErrInvalidModel
	ErrInternal               = errors.New("server internal error, please try again later")
	ErrAPIKeyOwnerRequired    = errors.New("owner api key is required")
	ErrServiceAccountRequired = errors.New("service account api key required")
	ErrAPIKeyScopeRequired    = errors.New("api key missing required scope")
	ErrAPIKeyNameRequired     = errors.New("api key name is required")
)
