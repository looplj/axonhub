package server

import (
	"time"

	"github.com/looplj/axonhub/internal/tracing"
)

type Config struct {
	Port        int           `conf:"port" yaml:"port" json:"port"`
	Name        string        `conf:"name" yaml:"name" json:"name"`
	BasePath    string        `conf:"base_path" yaml:"base_path" json:"base_path"`
	ReadTimeout time.Duration `conf:"read_timeout" yaml:"read_timeout" json:"read_timeout"`

	// WriteTimeout is the maximum duration for writing the response.
	WriteTimeout time.Duration `conf:"write_timeout" yaml:"write_timeout" json:"write_timeout"`

	// RequestTimeout is the maximum duration for processing a request.
	RequestTimeout time.Duration `conf:"request_timeout" yaml:"request_timeout" json:"request_timeout"`

	// LLMRequestTimeout is the maximum duration for processing a request to LLM.
	LLMRequestTimeout time.Duration `conf:"llm_request_timeout" yaml:"llm_request_timeout" json:"llm_request_timeout"`

	Trace tracing.Config `conf:"trace" yaml:"trace" json:"trace"`

	Debug bool `conf:"debug" yaml:"debug" json:"debug"`
}
