package config

import "log/slog"

// LogValue implements slog.LogValuer so that logging the config — at any
// level, from anywhere — cannot leak credentials.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("log_level", c.LogLevel.String()),
		slog.Any("redis", c.Redis),
		slog.Any("storage", c.Storage),
		slog.Any("worker", c.Worker),
	)
}

func (r Redis) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("addr", r.Addr),
		slog.String("password", redacted(r.Password)),
		slog.String("stream", r.Stream),
		slog.String("group", r.Group),
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
	)
}

// redacted reports whether a secret is set without revealing it.
func redacted(s string) string {
	if s == "" {
		return "(unset)"
	}
	return "[REDACTED]"
}
