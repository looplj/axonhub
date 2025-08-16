package server

import (
	"time"
)

type Config struct {
	Port           int
	Name           string
	BasePath       string
	ReadTimeout    time.Duration
	RequestTimeout time.Duration
	Debug          bool
}
