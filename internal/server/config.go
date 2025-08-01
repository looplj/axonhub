package server

import (
	"time"
)

type Config struct {
	Port           int
	Name           string
	BasePath       string
	RequestTimeout time.Duration
	Debug          bool
	CORS           CORS
}

type CORS struct {
	Debug              bool
	Enabled            bool
	AllowedOrigins     []string
	AllowedMethods     []string
	AllowedHeaders     []string
	ExposedHeaders     []string
	AllowCredentials   bool
	MaxAge             int
	OptionsPassthrough bool
}
