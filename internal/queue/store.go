package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NotifyChannel is the Postgres channel core signals after committing an
// enqueue. It only wakes workers sooner; polling is what guarantees delivery.
const NotifyChannel = "jobs"

// Store is the worker's access to the jobs table. Schema changes belong to
// core's Flyway migrations — nothing here creates or alters anything.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// claimSQL atomically takes the next runnable job.
//
// FOR UPDATE SKIP LOCKED is what lets several workers run this identical query
// concurrently with no coordination: the inner SELECT locks the row it picks,
// and any other worker skips straight past that locked row to the next one.
//
// The second OR clause recovers jobs whose worker died — the lock they held
// expires and the row becomes claimable again.
const claimSQL = `
UPDATE jobs
SET status       = 'running',
    attempts     = attempts + 1,
    locked_until = now() + make_interval(secs => $1),
    locked_by    = $2,
    updated_at   = now()
WHERE id = (
    SELECT id FROM jobs
    WHERE (status = 'pending' AND run_after <= now())
       OR (status = 'running' AND locked_until < now())
    ORDER BY run_after, id
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING id, type, payload, attempts, max_attempts, run_after, created_at`

// Claim takes the next runnable job, or reports false when the queue is empty.
func (s *Store) Claim(ctx context.Context, lock time.Duration, lockedBy string) (Job, bool, error) {
	var (
		job     Job
		payload []byte
	)

	err := s.pool.QueryRow(ctx, claimSQL, lock.Seconds(), lockedBy).Scan(
		&job.ID,
		&job.Type,
		&payload,
		&job.Attempts,
		&job.MaxAttempts,
		&job.RunAfter,
		&job.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, fmt.Errorf("claiming job: %w", err)
	}

	job.Payload = json.RawMessage(payload)
	return job, true, nil
}

const heartbeatSQL = `
UPDATE jobs
SET locked_until = now() + make_interval(secs => $1),
    updated_at   = now()
WHERE id = $2 AND locked_by = $3 AND status = 'running'`

// Heartbeat extends this worker's lock on a running job.
//
// The locked_by guard matters: if this worker stalled long enough for its lock
// to expire and another worker legitimately took the job over, the heartbeat
// must not steal it back.
func (s *Store) Heartbeat(ctx context.Context, id int64, lockedBy string, lock time.Duration) error {
	tag, err := s.pool.Exec(ctx, heartbeatSQL, lock.Seconds(), id, lockedBy)
	if err != nil {
		return fmt.Errorf("heartbeat for job %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("heartbeat for job %d: lock no longer held", id)
	}
	return nil
}

// Begin opens the transaction a handler's domain write shares with its job
// completion, so neither can land without the other.
func (s *Store) Begin(ctx context.Context) (pgx.Tx, error) {
	return s.pool.Begin(ctx)
}

const completeSQL = `
UPDATE jobs
SET status       = 'done',
    result       = $1,
    locked_until = NULL,
    locked_by    = NULL,
    last_error   = NULL,
    updated_at   = now()
WHERE id = $2`

// Complete marks a job done inside the caller's transaction. It must run in
// the same transaction as the handler's domain write.
func (s *Store) Complete(ctx context.Context, tx pgx.Tx, id int64, res Result) error {
	encoded, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("encoding result for job %d: %w", id, err)
	}
	if _, err := tx.Exec(ctx, completeSQL, encoded, id); err != nil {
		return fmt.Errorf("completing job %d: %w", id, err)
	}
	return nil
}

const retrySQL = `
UPDATE jobs
SET status       = 'pending',
    run_after    = now() + make_interval(secs => $1),
    last_error   = $2,
    locked_until = NULL,
    locked_by    = NULL,
    updated_at   = now()
WHERE id = $3`

// Retry returns a failed job to the queue after a delay.
func (s *Store) Retry(ctx context.Context, id int64, delay time.Duration, lastErr string) error {
	if _, err := s.pool.Exec(ctx, retrySQL, delay.Seconds(), lastErr, id); err != nil {
		return fmt.Errorf("scheduling retry for job %d: %w", id, err)
	}
	return nil
}

const deadLetterSQL = `
UPDATE jobs
SET status       = 'failed',
    last_error   = $1,
    locked_until = NULL,
    locked_by    = NULL,
    updated_at   = now()
WHERE id = $2`

// DeadLetter parks a job that has exhausted its attempts. Nothing retries it;
// it waits for a human.
func (s *Store) DeadLetter(ctx context.Context, id int64, lastErr string) error {
	if _, err := s.pool.Exec(ctx, deadLetterSQL, lastErr, id); err != nil {
		return fmt.Errorf("dead-lettering job %d: %w", id, err)
	}
	return nil
}

// Listen blocks until a NOTIFY arrives on NotifyChannel, ctx is cancelled, or
// the connection drops. It holds a dedicated connection for its lifetime,
// because a connection waiting on notifications cannot also serve queries.
func (s *Store) Listen(ctx context.Context, onNotify func()) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring listener connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN "+NotifyChannel); err != nil {
		return fmt.Errorf("listening on %s: %w", NotifyChannel, err)
	}

	for {
		if _, err := conn.Conn().WaitForNotification(ctx); err != nil {
			return fmt.Errorf("waiting for notification: %w", err)
		}
		onNotify()
	}
}
