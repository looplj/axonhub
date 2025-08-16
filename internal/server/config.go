package server

import (
	"time"
)

type Config struct {
	Port        int
	Name        string
	BasePath    string
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration for writing the response.
	WriteTimeout time.Duration

	// RequestTimeout is the maximum duration for processing a request.
	RequestTimeout time.Duration

	// LLMRequestTimeout is the maximum duration for processing a request to LLM.
	LLMRequestTimeout time.Duration

	Debug bool
}
