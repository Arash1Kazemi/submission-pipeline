package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jackc/pgx/v5"

	"wikipg/internal/queue"
	"wikipg/internal/storage"
)

// ImagePayload is the job payload core enqueues for an image submission.
type ImagePayload struct {
	SubmissionID string `json:"submission_id"`
	ObjectKey    string `json:"object_key"`
}

type Image struct {
	Store         *storage.Client
	RawBucket     string
	DerivedBucket string
}

func (h *Image) Handle(ctx context.Context, log *slog.Logger, tx pgx.Tx, job queue.Job) (queue.Result, error) {
	var payload ImagePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		// Malformed payloads never become valid on a retry, so this is
		// reported as a rejection rather than an error.
		return reject("malformed_payload", nil), nil
	}

	log = log.With("submission_id", payload.SubmissionID)

	dir, err := os.MkdirTemp("", "imgjob-")
	if err != nil {
		return queue.Result{}, fmt.Errorf("creating work dir: %w", err)
	}
	defer os.RemoveAll(dir)

	// filepath.Base strips any directory part: the object key arrives in the
	// job payload, and a key like "../../etc/passwd" must not escape the
	// temp dir.
	src := filepath.Join(dir, filepath.Base(payload.ObjectKey))

	size, err := h.Store.Download(ctx, h.RawBucket, payload.ObjectKey, src)
	if err != nil {
		var tooLarge *storage.TooLargeError
		switch {
		case errors.As(err, &tooLarge):
			return reject("file_too_large", map[string]string{
				"size":  strconv.FormatInt(tooLarge.Size, 10),
				"limit": strconv.FormatInt(tooLarge.Limit, 10),
			}), nil
		case errors.Is(err, storage.ErrNotFound):
			return reject("object_missing", nil), nil
		}
		// Anything else is infrastructure: retry it.
		return queue.Result{}, fmt.Errorf("downloading %s: %w", payload.ObjectKey, err)
	}

	log.Debug("source downloaded", "bytes", size)

	// TODO:
	//   rejected, reason, err := imagepipe.Validate(src)
	//     err      -> return queue.Result{}, fmt.Errorf("validating image: %w", err)
	//     rejected -> return reject(reason, nil), nil
	//   imagepipe.Process(src, dir) -> variants
	//   h.Store.Upload(ctx, h.DerivedBucket, <key>, variant.Path, "image/jpeg")
	//   write image_variants rows on tx

	return queue.Result{Outcome: queue.OutcomeOK}, nil
}

// reject builds a terminal result: the pipeline worked, the content is not
// acceptable. Never retried.
func reject(code string, detail map[string]string) queue.Result {
	return queue.Result{
		Outcome: queue.OutcomeRejected,
		Reason:  &queue.Reason{Code: code, Detail: detail},
	}
}
