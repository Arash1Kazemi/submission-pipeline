// Package queue defines the Postgres-backed job contract shared with the
// Spring side, and runs the worker loop that consumes it.
//
// The jobs table itself is owned by core's Flyway migrations. This package
// only issues DML against it.
package queue

import (
	"encoding/json"
	"time"
)

// JobType selects which handler processes a job.
type JobType string

const (
	JobTypeImage   JobType = "image"
	JobTypeTabular JobType = "tabular"
	JobTypeGeo     JobType = "geo"
)

// Status is the lifecycle state of a job row, and is distinct from the
// Outcome of the work itself. A submission that is rejected produces a done
// row carrying an OutcomeRejected result — the pipeline ran correctly and the
// answer was no. Only the worker malfunctioning produces StatusFailed.
type Status string

const (
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
)

// Outcome is the result of the work. The split between OutcomeRejected and
// OutcomeFailed is what drives retry behaviour: rejected is terminal, failed
// is retryable.
type Outcome string

const (
	// OutcomeOK — the job did its work successfully.
	OutcomeOK Outcome = "ok"

	// OutcomeRejected — the pipeline worked correctly and the answer is no:
	// the submitted content is unacceptable. Never retried; the contributor
	// should be shown why.
	OutcomeRejected Outcome = "rejected"

	// OutcomeFailed — the worker itself malfunctioned (storage unreachable,
	// disk full, timeout). Retryable; the contributor sees nothing.
	OutcomeFailed Outcome = "failed"
)

// Job is one row of the jobs table, as claimed by this worker.
type Job struct {
	ID          int64
	Type        JobType
	Payload     json.RawMessage
	Attempts    int
	MaxAttempts int
	RunAfter    time.Time
	CreatedAt   time.Time
}

// LogAttrs returns this job's correlation fields for slog.Logger.With, so
// every line written about the job carries them. Handlers add submission_id
// once they have parsed their own payload — the queue deliberately does not
// parse it twice just to log one field.
func (j Job) LogAttrs() []any {
	return []any{
		"job_id", j.ID,
		"type", string(j.Type),
		"attempt", j.Attempts,
	}
}

// Result is what the worker records for one job. At most one of the typed
// payloads is set, matching Type. It is stored as JSONB on the job row, which
// is where core reads it from.
type Result struct {
	Version    int       `json:"version"`
	JobID      int64     `json:"job_id"`
	Type       JobType   `json:"type"`
	Outcome    Outcome   `json:"outcome"`
	Attempt    int       `json:"attempt"`
	DurationMS int64     `json:"duration_ms"`
	FinishedAt time.Time `json:"finished_at"`

	// Reason is set when Outcome is OutcomeRejected.
	Reason *Reason `json:"reason,omitempty"`

	// Error is set when Outcome is OutcomeFailed. Operator-facing only — it may
	// contain internal detail and must not be shown to contributors.
	Error string `json:"error,omitempty"`

	Tabular *TabularResult `json:"tabular,omitempty"`
	Image   *ImageResult   `json:"image,omitempty"`
	Geo     *GeoResult     `json:"geo,omitempty"`
}

// Reason explains a rejection in a form the frontend can localise. Code is a
// stable identifier; Detail carries the values that vary per occurrence, for
// interpolation into a translated message.
type Reason struct {
	Code   string            `json:"code"`
	Detail map[string]string `json:"detail,omitempty"`
}

type TabularResult struct {
	Columns   []Column   `json:"columns"`
	RowCount  int        `json:"row_count"`
	ValidRows int        `json:"valid_rows"`
	RowErrors []RowError `json:"row_errors"`

	// ErrorsTruncated is how many row errors were dropped from RowErrors to
	// keep the stored result small.
	ErrorsTruncated int `json:"errors_truncated"`

	NormalizedKey string `json:"normalized_key,omitempty"`
}

type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type RowError struct {
	Row    int               `json:"row"`
	Column string            `json:"column,omitempty"`
	Code   string            `json:"code"`
	Detail map[string]string `json:"detail,omitempty"`
}

type ImageResult struct {
	Width    int       `json:"width"`
	Height   int       `json:"height"`
	Format   string    `json:"format"`
	Variants []Variant `json:"variants"`
}

type Variant struct {
	Label  string `json:"label"`
	Key    string `json:"key"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Bytes  int64  `json:"bytes"`
}

type GeoResult struct {
	FeatureCount int    `json:"feature_count"`
	SourceCRS    string `json:"source_crs"`
	OutputKey    string `json:"output_key"`
}
