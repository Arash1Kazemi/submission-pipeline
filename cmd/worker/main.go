package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"wikipg/internal/config"
	"wikipg/internal/handler"
	"wikipg/internal/queue"
	"wikipg/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run holds what main would otherwise do, so startup failures return an error
// instead of calling os.Exit from half a dozen places.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
		// file:line on every record - worth the cost while debugging,
		// noise in production.
		AddSource: cfg.LogLevel <= slog.LevelDebug,
	}))
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := newPool(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	store, err := storage.New(
		ctx,
		cfg.Storage.Endpoint,
		cfg.Storage.AccessKey,
		cfg.Storage.SecretKey,
		cfg.Storage.UseSSL,
		cfg.Worker.MaxFileBytes,
	)
	if err != nil {
		return err
	}

	health := &Health{}
	health.MarkPoll() // don't report stale before the first poll
	// hardcoded fix: from HEALTH_ADDR / HEALTH_MAX_AGE
	healthSrv := startHealthServer(log, health, ":8080", 30*time.Second)
	defer shutdownHealthServer(healthSrv, log)

	runner := queue.NewRunner(
		queue.NewStore(pool),
		log,
		queue.RunnerConfig{
			Concurrency:       cfg.Worker.Concurrency,
			JobTimeout:        cfg.Worker.JobTimeout,
			PollInterval:      cfg.Worker.PollInterval,
			HeartbeatInterval: cfg.Worker.HeartbeatInterval,
			LockDuration:      cfg.Worker.LockDuration,
			MaxAttempts:       cfg.Worker.MaxAttempts,
			RetryBaseDelay:    cfg.Worker.RetryBaseDelay,
		},
		identity(),
		health.MarkPoll,
	)

	runner.Register(queue.JobTypeImage, &handler.Image{
		Store:         store,
		RawBucket:     cfg.Storage.RawBucket,
		DerivedBucket: cfg.Storage.DerivedBucket,
	})

	log.Info("worker ready", "config", cfg, "identity", identity())

	if err := runner.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("worker stopped: %w", err)
	}
	log.Info("worker shut down cleanly")
	return nil
}

func newPool(ctx context.Context, dbCfg config.Database) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(dbCfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parsing database config: %w", err)
	}
	poolCfg.MaxConns = int32(dbCfg.MaxConns)

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	// Fail at startup rather than on the first claimed job.
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return pool, nil
}

func startHealthServer(log *slog.Logger, health *Health, addr string, maxAge time.Duration) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health.Handle(maxAge))

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("health server stopped", "err", err)
		}
	}()
	return srv
}

func shutdownHealthServer(srv *http.Server, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutting down health server", "err", err)
	}
}

// identity names this process in the jobs table's locked_by column, so a row
// says which worker is holding it.
func identity() string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	return fmt.Sprintf("%s/%d", host, os.Getpid())
}
