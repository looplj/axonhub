package log

import (
	"log/slog"

	"go.uber.org/zap/exp/zapslog"
)

func (l *Logger) AsSlog() *slog.Logger {
	return slog.New(zapslog.NewHandler(l.logger.Core()))
}
