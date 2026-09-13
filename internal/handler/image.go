package handler

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"wikipg/internal/queue"
)

// ImagePayload is the job payload core enqueues for an image submission.
type ImagePayload struct {
	SubmissionID string `json:"submission_id"`
	ObjectKey    string `json:"object_key"`
}

type Image struct {
	// storage client, config, ...
}

func (h *Image) Handle(ctx context.Context, log *slog.Logger, tx pgx.Tx, job queue.Job) (queue.Result, error) {
	var payload ImagePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		// Malformed payloads never become valid on a retry, so this is
		// reported as a rejection rather than an error.
		return queue.Result{
			Outcome: queue.OutcomeRejected,
			Reason:  &queue.Reason{Code: "malformed_payload"},
		}, nil
	}

	log = log.With("submission_id", payload.SubmissionID)
	log.Debug("image job claimed", "object_key", payload.ObjectKey)

	// TODO: download payload.ObjectKey via storage, then
	//   rejected, reason, err := imagepipe.Validate(srcPath)
	//   on err  -> return queue.Result{}, fmt.Errorf("validating image: %w", err)
	//   rejected -> OutcomeRejected with a Reason code
	//   else     -> imagepipe.Process, upload variants, write domain rows on tx

	return queue.Result{Outcome: queue.OutcomeOK}, nil
}
