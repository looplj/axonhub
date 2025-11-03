package transformer

import (
	"errors"
)

var (
	ErrInvalidRequest = errors.New("invalid request")
	ErrInvalidModel   = errors.New("model not found")
)
