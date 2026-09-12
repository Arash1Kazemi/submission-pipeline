package queue

import (
	"context"
	"log/slog"
	"time"
)

// Handler processes one job. Implementations log domain ecents only;
// the dispatcher owns received/completed/failed and timing.
type Handler interface {
	Handle(ctx context.Context, log *slog.Logger, job Job) (Result, error)
}

// dispatch runs one job with correlation logging and timing around it.
func dispatch(ctx context.Context, log *slog.Logger, h Handler, job Job) (Result, error) {
	jobLog := log.With(job.LogAttrs()...)
	jobLog.Info("job received", "object_key", job.ObjectKey)

	start := time.Now()
	res, err := h.Handle(ctx, jobLog, job)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		jobLog.Error("job failed", "err", err, "duration_ms", elapsed)
		return res, err
	}

	jobLog.Info("job completed", "duration_ms", elapsed)
	return res, nil
}
