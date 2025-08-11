package biz

import (
	"errors"
)

var (
	ErrInvalidJWT    = errors.New("invalid jwt token")
	ErrInvalidAPIKey = errors.New("invalid api key")
)
