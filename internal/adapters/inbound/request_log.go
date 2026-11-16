package inbound

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"runtime/debug"
	"time"

	"findJobs/internal/adapters"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const requestIDHeader = "X-Request-ID"

// validRequestID accepts an incoming id (e.g. one the SvelteKit server
// forwards) only when it's short and plain, so it can't forge log lines.
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// RequestLogger gives each request an id (kept from X-Request-ID when valid,
// echoed in the response) and writes one structured line when it finishes:
// method, path (never the query string, which can carry user input), status,
// latency, client IP and the errors handlers attached with ctx.Error. That
// last part matters: a 500's real cause is only in ctx.Errors, since the
// client sees a generic message (system requirements §20).
// Successful health probes aren't logged; Docker calls them every 30 s.
// It also recovers panics from itself and everything downstream (see
// recoverPanic), so the request line still records the resulting 500.
func RequestLogger(log zerolog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		id := ctx.GetHeader(requestIDHeader)
		if !validRequestID.MatchString(id) {
			id = newRequestID()
		}
		ctx.Header(requestIDHeader, id)

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

// recoverPanic is meant to be used as a deferred call (e.g. defer
// recoverPanic(ctx)). It turns a panic into a logged error (with its stack,
// server side only) and the usual generic 500 envelope. When the panic value
// is itself an error it is wrapped with %w, so callers of ctx.Errors can still
// match it with errors.Is/As against the original cause.
func recoverPanic(ctx *gin.Context) {
	p := recover()
	if p == nil {
		return
	}
	if p == http.ErrAbortHandler { // net/http's own "abort this response"
		panic(p)
	}
	if err, ok := p.(error); ok {
		ctx.Error(fmt.Errorf("panic: %w\n%s", err, debug.Stack()))
	} else {
		ctx.Error(fmt.Errorf("panic: %v\n%s", p, debug.Stack()))
	}
	if ctx.Writer.Written() {
		ctx.Abort()
		return
	}
	ctx.AbortWithStatusJSON(http.StatusInternalServerError,
		adapters.BuildErrorResponse("500 Internal Server Error", nil, errors.New("internal error")))
}

// Recover turns a panic into a logged error and the generic 500 envelope.
// RequestLogger already runs the same recovery inline, so this standalone
// handler is only needed for routers that want panic recovery without
// request logging.
func Recover() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer recoverPanic(ctx)
		ctx.Next()
	}
}

func newRequestID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read never fails on Go 1.24+, but never emit an empty
		// id if it somehow does: fall back to a hex timestamp.
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
