package telemetry

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// RedisHook implements go-redis Hook to record operation latency and errors.
type RedisHook struct {
	metrics *Metrics
	logger  zerolog.Logger
}

// NewRedisHook creates a hook that records Redis operation metrics and logs errors.
func NewRedisHook(metrics *Metrics, logger zerolog.Logger) *RedisHook {
	return &RedisHook{metrics: metrics, logger: logger}
}

// DialHook is called on new dial. Not used for per-command timing.
func (h *RedisHook) DialHook(next goredis.DialHook) goredis.DialHook {
	return next
}

// ProcessHook records latency and errors for each Redis command.
func (h *RedisHook) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		duration := time.Since(start)

		op := cmd.Name()
		success := err == nil

		h.metrics.RecordRedisOp(op, success, duration)

		if err != nil {
			h.logger.Error().
				Err(err).
				Str("command", op).
				Dur("duration", duration).
				Msg("redis operation failed")
		} else {
			// TEST LOGGING - log all successful Redis operations (remove after verification)
			h.logger.Info().
				Str("command", op).
				Dur("duration", duration).
				Msg("redis operation succeeded")
		}

		return err
	}
}

// ProcessPipelineHook records latency for Redis pipeline operations.
func (h *RedisHook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []goredis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		duration := time.Since(start)

		// Record metrics per command in the pipeline.
		for _, cmd := range cmds {
			op := cmd.Name()
			success := err == nil
			h.metrics.RecordRedisOp(op, success, duration)
			if err != nil {
				h.logger.Error().
					Err(err).
					Str("command", op).
					Dur("duration", duration).
					Msg("redis pipeline operation failed")
			} else {
				// TEST LOGGING - log all successful Redis pipeline operations (remove after verification)
				h.logger.Info().
					Str("command", op).
					Dur("duration", duration).
					Msg("redis pipeline operation succeeded")
			}
		}

		return err
	}
}
