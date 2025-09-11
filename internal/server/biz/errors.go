package biz

import (
	"errors"
)

var (
	ErrInvalidJWT      = errors.New("invalid jwt token")
	ErrInvalidAPIKey   = errors.New("invalid api key")
	ErrInvalidPassword = errors.New("invalid password")
	ErrInvalidModel    = errors.New("invalid model")
	ErrInternal        = errors.New("server internal error, please try again later")
)
