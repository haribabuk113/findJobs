// Package telemetry provides request-level and dependency-level observability:
// request ID propagation, structured logging for outbound calls (IAM, Postgres,
// Redis), and Prometheus metrics for HTTP, IAM, Postgres and Redis.
package telemetry

import (
	"context"
	"time"
)

// contextKey is an unexported type for context keys defined in this package.
type contextKey int

const requestIDKey contextKey = iota

// WithRequestID returns a context carrying the given request ID, so outbound
// calls (IAM, Postgres, Redis) can propagate it in their logs and headers.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext returns the request ID stored in ctx, or "".
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// Timer is a minimal stopwatch used by tracers/hooks to record latency.
type Timer struct {
	start time.Time
}

// StartTimer returns a running Timer.
func StartTimer() Timer { return Timer{start: time.Now()} }

// Since returns the elapsed duration since the timer was started.
func (t Timer) Since() time.Duration { return time.Since(t.start) }
