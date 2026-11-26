package telemetry

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// PostgresTracer implements pgx.QueryTracer to record query latency and errors.
type PostgresTracer struct {
	metrics *Metrics
	logger  zerolog.Logger
}

// NewPostgresTracer creates a tracer that records query metrics and logs slow/failed queries.
func NewPostgresTracer(metrics *Metrics, logger zerolog.Logger) *PostgresTracer {
	return &PostgresTracer{metrics: metrics, logger: logger}
}

// TraceQueryStart is called at the beginning of Query, QueryRow, and Exec calls.
// The returned context is used for the rest of the call and will be passed to TraceQueryEnd.
func (t *PostgresTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	// Store timer and query SQL in context so TraceQueryEnd can access them.
	start := time.Now()
	ctx = context.WithValue(ctx, timerKey{}, start)
	ctx = context.WithValue(ctx, "query_sql", data.SQL)
	return ctx
}

type timerKey struct{}

// TraceQueryEnd is called after Query, QueryRow, or Exec completes.
func (t *PostgresTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	timerVal := ctx.Value(timerKey{})
	var timer Timer
	if t, ok := timerVal.(Timer); ok {
		timer = t
	}
	sqlStr := ""
	if qs, ok := ctx.Value("query_sql").(string); ok {
		sqlStr = qs
	}
	duration := timer.Since()

	queryType := classifyQuery(sqlStr)
	success := data.Err == nil

	t.metrics.RecordPGQuery(queryType, success, duration)

	if data.Err != nil {
		t.logger.Error().
			Err(data.Err).
			Str("query_type", queryType).
			Dur("duration", duration).
			Str("sql", sanitizeSQL(sqlStr)).
			Msg("postgres query failed")
	} else if duration > 500*time.Millisecond {
		t.logger.Warn().
			Str("query_type", queryType).
			Dur("duration", duration).
			Str("sql", sanitizeSQL(sqlStr)).
			Msg("slow postgres query")
	} else {
		// TEST LOGGING - log all successful queries (remove after verification)
		t.logger.Info().
			Str("query_type", queryType).
			Dur("duration", duration).
			Str("sql", sanitizeSQL(sqlStr)).
			Msg("postgres query succeeded")
	}
}

// classifyQuery extracts a query type label from the SQL text.
// Returns the first word (SELECT, INSERT, UPDATE, DELETE, etc.) or "other".
func classifyQuery(sql string) string {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return "unknown"
	}
	// Skip comments
	for strings.HasPrefix(sql, "--") || strings.HasPrefix(sql, "/*") {
		if idx := strings.Index(sql, "\n"); idx >= 0 {
			sql = strings.TrimSpace(sql[idx+1:])
		} else {
			return "comment"
		}
	}
	// Get first word
	if idx := strings.IndexAny(sql, " \t\n\r"); idx > 0 {
		return strings.ToUpper(sql[:idx])
	}
	return strings.ToUpper(sql)
}

// sanitizeSQL removes string literals and limits length for safe logging.
func sanitizeSQL(sql string) string {
	if len(sql) > 500 {
		sql = sql[:500] + "..."
	}
	// Replace string literals with ?
	inQuote := false
	quoteChar := byte(0)
	result := make([]byte, 0, len(sql))
	for i := 0; i < len(sql); i++ {
		c := sql[i]
		if !inQuote && (c == '\'' || c == '"' || c == '`') {
			inQuote = true
			quoteChar = c
			result = append(result, '?')
			continue
		}
		if inQuote && c == quoteChar {
			// Check for escaped quote
			if i+1 < len(sql) && sql[i+1] == quoteChar {
				i++ // skip escaped quote
				continue
			}
			inQuote = false
			continue
		}
		if !inQuote {
			result = append(result, c)
		}
	}
	return string(result)
}

// ConfigureTracer sets up the pgxpool.Config with the tracing QueryTracer.
// Call this after pgxpool.ParseConfig but before pgxpool.NewWithConfig.
func ConfigureTracer(poolConfig *pgxpool.Config, metrics *Metrics, logger zerolog.Logger) {
	if poolConfig.ConnConfig == nil {
		return
	}
	tracer := NewPostgresTracer(metrics, logger)
	poolConfig.ConnConfig.Tracer = tracer
}
