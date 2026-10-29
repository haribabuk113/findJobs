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
func RequestLogger(log zerolog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		id := ctx.GetHeader(requestIDHeader)
		if !validRequestID.MatchString(id) {
			id = newRequestID()
		}
		ctx.Header(requestIDHeader, id)

		ctx.Next()

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
}

// Recover turns a panic into a logged error (with its stack, server side
// only) and the usual generic 500 envelope. Use it inside RequestLogger so
// the request line records the 500 and the panic.
func Recover() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			p := recover()
			if p == nil {
				return
			}
			if p == http.ErrAbortHandler { // net/http's own "abort this response"
				panic(p)
			}
			ctx.Error(fmt.Errorf("panic: %v\n%s", p, debug.Stack()))
			if ctx.Writer.Written() {
				ctx.Abort()
				return
			}
			ctx.AbortWithStatusJSON(http.StatusInternalServerError,
				adapters.BuildErrorResponse("500 Internal Server Error", nil, errors.New("internal error")))
		}()
		ctx.Next()
	}
}

func newRequestID() string {
	b := make([]byte, 12)
	rand.Read(b) // never fails (crypto/rand, Go 1.24+)
	return hex.EncodeToString(b)
}
