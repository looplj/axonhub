package api

import (
	"github.com/looplj/axonhub/internal/log"
)

var logger *log.Logger

func initLogger(l *log.Logger) {
	logger = l.WithName("api")
}
