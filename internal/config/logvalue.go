package config

import "log/slog"

// LogValue implements slog.LogValuer so that logging the config — at any
// level, from anywhere — cannot leak credentials.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("log_level", c.LogLevel.String()),
		slog.Any("database", c.Database),
		slog.Any("storage", c.Storage),
		slog.Any("worker", c.Worker),
	)
}

func (d Database) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("host", d.Host),
		slog.Int("port", d.Port),
		slog.String("name", d.Name),
		slog.String("user", d.User),
		slog.String("password", redacted(d.Password)),
		slog.Int("max_conns", d.MaxConns),
	)
}

func (s Storage) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("endpoint", s.Endpoint),
		slog.String("access_key", redacted(s.AccessKey)),
		slog.String("secret_key", redacted(s.SecretKey)),
		slog.Bool("use_ssl", s.UseSSL),
		slog.String("raw_bucket", s.RawBucket),
		slog.String("derived_bucket", s.DerivedBucket),
	)
}

func (w Worker) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("concurrency", w.Concurrency),
		slog.Int64("max_file_bytes", w.MaxFileBytes),
		slog.Duration("job_timeout", w.JobTimeout),
		slog.Duration("poll_interval", w.PollInterval),
		slog.Duration("heartbeat_interval", w.HeartbeatInterval),
		slog.Duration("lock_duration", w.LockDuration),
		slog.Int("max_attempts", w.MaxAttempts),
		slog.Duration("retry_base_delay", w.RetryBaseDelay),
	)
}

// redacted reports whether a secret is set without revealing it.
func redacted(s string) string {
	if s == "" {
		return "(unset)"
	}
	return "[REDACTED]"
}
