package transformer

import (
	"errors"
)

var (
	ErrInvalidRequest        = errors.New("invalid request")
	ErrUnsupportedConversion = errors.New("unsupported conversion")
	ErrInvalidModel          = errors.New("model not found")
	ErrInvalidResponse       = errors.New("invalid response")
)
