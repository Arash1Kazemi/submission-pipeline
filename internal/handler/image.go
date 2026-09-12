package handler

import (
	"context"
	"log/slog"
	"wikipg/internal/queue"
)

type Image struct {
	// storage client, config, ...
}

func (h *Image) Handle(ctx context.Context, log *slog.Logger, job queue.Job) (queue.Result, error) {
	// download via storage, then:
	//
	// rejected, reason, err := imagepipe.Validate(srcPath)
	// if err != nil {
	//     return queue.Result{}, fmt.Errorf("validating image: %w", err)
	// }
	// if rejected {
	//     log.Warn("image rejected", "reason", reason)
	//     return queue.Result{}, nil
	// }
	return queue.Result{}, nil
}
