// Package config loads and validates the worker's runtime configuration
// from the process environment.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// Config is the fully validated configuration for one worker process.
// It is loaded once at startup and passed explicitly to the components that
// need it — there is deliberately no package-level instance.
type Config struct {
	Redis    Redis
	Storage  Storage
	Worker   Worker
	LogLevel slog.Level
}

type Redis struct {
	Addr     string
	Password string
	Stream   string
	Group    string
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
}

// Load reads configuration from the environment, reporting every problem
// it finds at once rather than failing on the first.
func Load() (Config, error) {
	var l loader

	cfg := Config{
		LogLevel: l.optLevel("LOG_LEVEL", slog.LevelInfo),
		Redis: Redis{
			Addr: l.require("REDIS_ADDR"),
			// Optional: the local compose Redis runs without auth.
			Password: os.Getenv("REDIS_PASSWORD"),
			Stream:   l.optText("REDIS_JOB_STREAM", "submissions"),
			Group:    l.optText("REDIS_CONSUMER_GROUP", "pipeline"),
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
			Concurrency:  l.optInt("WORKER_CONCURRENCY", 4),
			MaxFileBytes: l.optInt64("WORKER_MAX_FILE_BYTES", 256<<20), // 256 MiB
			JobTimeout:   l.optDuration("WORKER_JOB_TIMEOUT", 5*time.Minute),
		},
	}

	if cfg.Worker.Concurrency < 1 {
		l.errorf("WORKER_CONCURRENCY must be at least 1, got %d", cfg.Worker.Concurrency)
	}
	if cfg.Worker.MaxFileBytes < 1 {
		l.errorf("WORKER_MAX_FILE_BYTES must be positive, got %d", cfg.Worker.MaxFileBytes)
	}

	if err := errors.Join(l.errs...); err != nil {
		return Config{}, fmt.Errorf("invalid configuration:\n%w", err)
	}
	return cfg, nil
}
