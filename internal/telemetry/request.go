package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// requestIDHeader is the header name used to carry request IDs across service boundaries.
const requestIDHeader = "X-Request-ID"

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// RequestLogger gives each request an id (kept from X-Request-ID when valid,
// echoed in the response) and writes one structured line when it finishes:
// method, path (never the query string, which can carry user input), status,
// latency, client IP and the errors handlers attached with ctx.Error.
func RequestLogger(log zerolog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		id := ctx.GetHeader(requestIDHeader)
		if !validRequestID.MatchString(id) {
			id = newRequestID()
		}
		ctx.Header(requestIDHeader, id)

		// Put the request ID in context so downstream outbound calls can access it.
		ctx.Request = ctx.Request.WithContext(WithRequestID(ctx.Request.Context(), id))

		// The request line must be written even when a handler panics.
		// Deferred functions run LIFO, so registering the logger first makes
		// it run last: after recoverPanic has turned the panic into a 500
		// and attached the cause to ctx.Errors.
		defer writeRequestLog(log, ctx, id, start)
		defer recoverPanic(ctx)

		ctx.Next()
	}
}

// writeRequestLog emits the single structured line for a finished request.
func writeRequestLog(log zerolog.Logger, ctx *gin.Context, id string, start time.Time) {
	status := ctx.Writer.Status()
	path := ctx.Request.URL.Path
	if status < 400 && (path == "/healthz" || path == "/readyz") {
		return
	}
	event := log.Info()
	if status >= 500 {
		event = log.Error()
	}
	if len(ctx.Errors) > 0 {
		event = event.Str("errors", ctx.Errors.String())
	}
	event.Str("request_id", id).
		Str("method", ctx.Request.Method).
		Str("path", path).
		Int("status", status).
		Dur("latency", time.Since(start)).
		Str("client_ip", ctx.ClientIP()).
		Msg("request")
}

// recoverPanic turns a panic into a logged error and the generic 500 envelope.
func recoverPanic(ctx *gin.Context) {
	if p := recover(); p != nil {
		// Gin's abort handler is not an error.
		if p == http.ErrAbortHandler {
			panic(p)
		}
		var err error
		if e, ok := p.(error); ok {
			err = fmt.Errorf("panic: %w", e)
		} else {
			err = fmt.Errorf("panic: %v", p)
		}
		ctx.Error(err)
		if !ctx.Writer.Written() {
			ctx.AbortWithStatus(http.StatusInternalServerError)
		} else {
			ctx.Abort()
		}
	}
}

func newRequestID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
