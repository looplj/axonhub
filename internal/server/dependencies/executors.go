package dependencies

import (
	"context"
	"reflect"

	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/log"
)

type ErrorHandler struct{}

func (h *ErrorHandler) CatchError(runnable executors.Runnable, err error) {
	log.Error(context.Background(), "run runnable error", log.Cause(err))
}

type RejectionHandler struct{}

func (h *RejectionHandler) RejectExecution(runnable executors.Runnable, e executors.Executor) error {
	log.Error(context.Background(), "runnable rejection by executor", log.String("runnable", reflect.ValueOf(runnable).String()))
	return nil
}

func NewExecutors(logger *log.Logger) executors.ScheduledExecutor {
	return executors.NewPoolScheduleExecutor(
		executors.WithMaxConcurrent(64),
		executors.WithMaxBlockingTasks(1024),
		executors.WithErrorHandler(&ErrorHandler{}),
		executors.WithRejectionHandler(&RejectionHandler{}),
		executors.WithLogger(logger.AsSlog()),
	)
}
