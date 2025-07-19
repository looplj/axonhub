package dependencies

import (
	"context"

	"github.com/looplj/axonhub/log"
	"github.com/zhenzou/executors"
)

type ErrorHandler struct{}

func (h *ErrorHandler) CatchError(runnable executors.Runnable, err error) {
	log.Error(context.Background(), "executor error", log.Cause(err))
}

func NewExecutors(logger *log.Logger) executors.ScheduledExecutor {
	return executors.NewPoolScheduleExecutor(
		executors.WithErrorHandler(&ErrorHandler{}),
		executors.WithLogger(logger.AsSlog()),
	)
}
