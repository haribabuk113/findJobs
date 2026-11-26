package telemetry

import (
	"context"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// IAMTracer provides helpers for IAM client instrumentation.
type IAMTracer struct {
	metrics *Metrics
	logger  zerolog.Logger
}

// NewIAMTracer creates a tracer for IAM outbound calls.
func NewIAMTracer(metrics *Metrics, logger zerolog.Logger) *IAMTracer {
	return &IAMTracer{metrics: metrics, logger: logger}
}

// StartCall begins timing an IAM call and returns a context with the timer stored.
func (t *IAMTracer) StartCall(ctx context.Context, endpoint string) (context.Context, time.Time) {
	start := time.Now()
	ctx = context.WithValue(ctx, timerKey{}, start)
	return ctx, start
}

// EndCall records the outcome and latency of an IAM call.
// endpoint is the IAM endpoint path (e.g., "/token", "/refresh").
func (t *IAMTracer) EndCall(ctx context.Context, endpoint string, start time.Time, err error) {
	duration := time.Since(start)
	success := err == nil

	t.metrics.RecordIAMCall(endpoint, success, duration)

	if err != nil {
		t.logger.Error().
			Err(err).
			Str("endpoint", endpoint).
			Dur("duration", duration).
			Msg("iam call failed")
	} else if duration > 500*time.Millisecond {
		t.logger.Warn().
			Str("endpoint", endpoint).
			Dur("duration", duration).
			Msg("slow iam call")
	}
}

// PropagateRequestID adds X-Request-ID header from context to the outbound request.
func PropagateRequestID(ctx context.Context, req *http.Request) {
	if id := RequestIDFromContext(ctx); id != "" {
		req.Header.Set(requestIDHeader, id)
	}
}
