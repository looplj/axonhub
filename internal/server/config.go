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
}
