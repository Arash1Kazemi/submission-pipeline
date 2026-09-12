package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// loader accumulates problems so Load can report all of them together.
type loader struct {
	errs []error
}

func (l *loader) errorf(format string, args ...any) {
	l.errs = append(l.errs, fmt.Errorf(format, args...))
}

// require reads a mandatory variable. It deliberately takes no default, so a
// secret cannot be given a fallback value.
func (l *loader) require(key string) string {
	v := os.Getenv(key)
	if v == "" {
		l.errorf("%s is required", key)
	}
	return v
}

func (l *loader) optText(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (l *loader) optInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		l.errorf("%s: %q is not a whole number", key, v)
		return def
	}
	return n
}

func (l *loader) optInt64(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		l.errorf("%s: %q is not a whole number", key, v)
		return def
	}
	return n
}

func (l *loader) optBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		l.errorf("%s: %q is not a boolean (true/false)", key, v)
		return def
	}
	return b
}

func (l *loader) optDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		l.errorf("%s: %q is not a duration (e.g. 30s, 5m)", key, v)
		return def
	}
	return d
}

func (l *loader) optLevel(key string, def slog.Level) slog.Level {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(strings.ToUpper(v))); err != nil {
		l.errorf("%s: %q is not a log level (debug, info, warn, error)", key, v)
		return def
	}
	return lvl
}
