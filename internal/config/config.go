// Package config loads and validates the worker's runtime configuration
// from the process environment.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"time"
)

// Config is the fully validated configuration for one worker process.
// It is loaded once at startup and passed explicitly to the components that
// need it — there is deliberately no package-level instance.
type Config struct {
	Database Database
	Storage  Storage
	Worker   Worker
	LogLevel slog.Level
}

// Database points at the same Postgres instance core uses. Every schema object
// is owned by core's Flyway migrations; the worker only issues DML against them.
type Database struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	MaxConns int
}

// DSN builds the connection string, escaping credentials so that a password
// containing reserved characters cannot corrupt the URL.
func (d Database) DSN() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(d.User, d.Password),
		Host:   net.JoinHostPort(d.Host, strconv.Itoa(d.Port)),
		Path:   d.Name,
	}
	return u.String()
}

type Storage struct {
	Endpoint      string
	AccessKey     string
	SecretKey     string
	UseSSL        bool
	RawBucket     string
	DerivedBucket string
}

type Worker struct {
	Concurrency  int
	MaxFileBytes int64
	JobTimeout   time.Duration

	// PollInterval is how often the queue is checked regardless of NOTIFY.
	// A worker that reconnects misses every notification sent while it was
	// away, so polling is what guarantees no job is stranded.
	PollInterval time.Duration

	// HeartbeatInterval is how often a running job extends its lock.
	HeartbeatInterval time.Duration

	// LockDuration is how far ahead locked_until is set when claiming. Once it
	// passes without a heartbeat, another worker may reclaim the job.
	LockDuration time.Duration

	// MaxAttempts is the default attempt ceiling for jobs that do not carry
	// their own. Exceeding it dead-letters the job.
	MaxAttempts int

	// RetryBaseDelay is the first retry delay; subsequent attempts back off
	// exponentially from it.
	RetryBaseDelay time.Duration
}

// Load reads configuration from the environment, reporting every problem
// it finds at once rather than failing on the first.
func Load() (Config, error) {
	var l loader

	cfg := Config{
		LogLevel: l.optLevel("LOG_LEVEL", slog.LevelInfo),
		Database: Database{
			Host:     l.optText("DB_HOST", "localhost"),
			Port:     l.optInt("DB_PORT", 5432),
			Name:     l.optText("DB_NAME", "core"),
			User:     l.require("DB_USERNAME"),
			Password: l.require("DB_PASSWORD"),
			MaxConns: l.optInt("DB_MAX_CONNS", 8),
		},
		Storage: Storage{
			Endpoint:      l.require("MINIO_ENDPOINT"),
			AccessKey:     l.require("MINIO_ACCESS_KEY"),
			SecretKey:     l.require("MINIO_SECRET_KEY"),
			UseSSL:        l.optBool("MINIO_USE_SSL", false),
			RawBucket:     l.require("MINIO_RAW_BUCKET"),
			DerivedBucket: l.require("MINIO_DERIVED_BUCKET"),
		},
		Worker: Worker{
			Concurrency:       l.optInt("WORKER_CONCURRENCY", 4),
			MaxFileBytes:      l.optInt64("WORKER_MAX_FILE_BYTES", 256<<20), // 256 MiB
			JobTimeout:        l.optDuration("WORKER_JOB_TIMEOUT", 5*time.Minute),
			PollInterval:      l.optDuration("WORKER_POLL_INTERVAL", 10*time.Second),
			HeartbeatInterval: l.optDuration("WORKER_HEARTBEAT_INTERVAL", time.Minute),
			LockDuration:      l.optDuration("WORKER_LOCK_DURATION", 10*time.Minute),
			MaxAttempts:       l.optInt("WORKER_MAX_ATTEMPTS", 5),
			RetryBaseDelay:    l.optDuration("WORKER_RETRY_BASE_DELAY", 30*time.Second),
		},
	}

	if cfg.Worker.Concurrency < 1 {
		l.errorf("WORKER_CONCURRENCY must be at least 1, got %d", cfg.Worker.Concurrency)
	}
	if cfg.Worker.MaxFileBytes < 1 {
		l.errorf("WORKER_MAX_FILE_BYTES must be positive, got %d", cfg.Worker.MaxFileBytes)
	}
	if cfg.Worker.MaxAttempts < 1 {
		l.errorf("WORKER_MAX_ATTEMPTS must be at least 1, got %d", cfg.Worker.MaxAttempts)
	}
	if cfg.Database.MaxConns < 1 {
		l.errorf("DB_MAX_CONNS must be at least 1, got %d", cfg.Database.MaxConns)
	}

	// A heartbeat that cannot fire at least twice inside the lock window lets a
	// healthy worker's lock expire mid-job, and another worker then reclaims
	// work that is still running.
	if cfg.Worker.HeartbeatInterval*2 > cfg.Worker.LockDuration {
		l.errorf("WORKER_HEARTBEAT_INTERVAL (%s) must be at most half of WORKER_LOCK_DURATION (%s)",
			cfg.Worker.HeartbeatInterval, cfg.Worker.LockDuration)
	}

	// The job timeout has to fit inside the lock window for the same reason.
	if cfg.Worker.JobTimeout > cfg.Worker.LockDuration {
		l.errorf("WORKER_JOB_TIMEOUT (%s) must not exceed WORKER_LOCK_DURATION (%s)",
			cfg.Worker.JobTimeout, cfg.Worker.LockDuration)
	}

	if err := errors.Join(l.errs...); err != nil {
		return Config{}, fmt.Errorf("invalid configuration:\n%w", err)
	}
	return cfg, nil
}
