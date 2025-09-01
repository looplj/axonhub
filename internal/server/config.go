package server

import (
	"time"

	"github.com/looplj/axonhub/internal/tracing"
)

type Config struct {
	Port        int           `conf:"port"`
	Name        string        `conf:"name"`
	BasePath    string        `conf:"base_path"`
	ReadTimeout time.Duration `conf:"read_timeout"`

	// WriteTimeout is the maximum duration for writing the response.
	WriteTimeout time.Duration `conf:"write_timeout"`

	// RequestTimeout is the maximum duration for processing a request.
	RequestTimeout time.Duration `conf:"request_timeout"`

	// LLMRequestTimeout is the maximum duration for processing a request to LLM.
	LLMRequestTimeout time.Duration `conf:"llm_request_timeout"`

	Trace tracing.Config `conf:"trace"`

	Debug bool `conf:"debug"`
}
