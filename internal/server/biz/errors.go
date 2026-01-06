package biz

import (
	"errors"

	"github.com/looplj/axonhub/llm/transformer"
)

var (
	ErrInvalidJWT      = errors.New("invalid jwt token")
	ErrInvalidAPIKey   = errors.New("invalid api key")
	ErrInvalidPassword = errors.New("invalid password")
	ErrInvalidModel    = transformer.ErrInvalidModel
	ErrInternal        = errors.New("server internal error, please try again later")
)
