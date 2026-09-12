package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
	"wikipg/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
		// file:line on every record - worth the cost while debugging,
		// noise in production.
		AddSource: cfg.LogLevel <= slog.LevelDebug,
	}))
	slog.SetDefault(log)

	health := &Health{}
	health.MarkPoll() // don't report stale before the first poll

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health.Handle(30*time.Second)) // hardcoded fix: form HEALTH_MAX_AGE

	srv := &http.Server{
		Addr:              ":8080", // hardcoded fix: form HEALTH_ADDR
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("health server stopped", "err", err)
		}
	}()

	// storage.New(cfg.Storage), queue.New(cfg.Redis), ...
}
