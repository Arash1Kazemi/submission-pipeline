package queue

import "time"

type JobType string

// Status is the outcome of a job.
// The sp;it between Rejexted and Failed is what drives retry behaviour:
// rejexted is terminal, failed is retryable
type Status string

// Package queue defines the Redis job contract shared with the Spring
// side, and runs the worker loop that consumes it
const (
	// StatusOK - the job did its work successfully
	StatusOK Status = "ok"

	// StatusRejected = the pipeline worked correctly and the answer is no:
	// the suvmitted content is unacceptable
	// Never retried, the contraivutor should be shown why
	StatusRejected Status = "rejected"

	// StatusFailed - the worker itself malfunctioned (storage unreachable, disk full, timeout)
	// Retryable; the contributor sees noting.
	StatusFailed Status = "failed"

	// TODO: one constant per job type the contract defines —
	// likely JobTypeNormalize, JobTypeParse, JobTypeImage, JobTypeGeo
	JobTypeRandom JobType = ""
)

// Job is one queue payload, matching the agreed contract exactly
type Job struct {
	ID           string
	Type         JobType
	SubmissionID string
	ObjectKey    string
	// TODO: remaining fields from waht amir set later
}

// Result is what the worker reports back for one job.
//
//	At most one of the typed payloads is set, matching Type.
type Result struct {
	Version      int       `json:"version"`
	JobID        string    `json:"job_id"`
	SubmissionID string    `json:"submission_id"`
	Type         JobType   `json:"type"`
	Status       Status    `json:"status"`
	Attempt      int       `json:"attempt"`
	DurationMS   int64     `json:"duration_ms"`
	FinishedAt   time.Time `json:"finished_at"`

	// Reason is set when Status is StatusRejected.
	Reason *Reason `json:"reason,omitempty"`
	// Error is set when Status is StatusFailed. Operator-facing only —
	// it may contain internal detail and must not be shown to contributors.
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
	// keep the message small.
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

// LogAttes returns this job's correlation fields for slog.Logger.With, so
// every line written about the job carries them.
func (j Job) LogAttrs() []any {
	return []any{
		"job_id", j.ID,
		"submission_id", j.SubmissionID,
		"type", string(j.Type),
	}
}
