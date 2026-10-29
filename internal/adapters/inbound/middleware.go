package inbound

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"findJobs/internal/adapters"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// imageTypes are the raw-body upload types. Like JSON, none of them is a
// CORS "simple" type, so a cross-site page can't send them without a preflight.
var imageTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}

// JSONBody rejects request bodies that aren't JSON (or a raw image upload)
// and caps their size. An HTML form can't send these types cross-site
// without a CORS preflight, so this is the backend's CSRF layer behind
// SameSite=Lax. Bodyless requests (GET, logout) pass; a form post always
// sends a type.
func JSONBody(maxBytes, maxImageBytes int64) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		hasBody := ctx.Request.ContentLength != 0 && ctx.Request.Body != http.NoBody
		if hasBody || ctx.GetHeader("Content-Type") != "" {
			limit := maxBytes
			switch contentType := ctx.ContentType(); {
			case contentType == "application/json":
			case imageTypes[contentType]:
				limit = maxImageBytes // handlers apply the per-image limits
			default:
				abort(ctx, http.StatusUnsupportedMediaType, "content type must be application/json")
				return
			}
			// Oversized bodies fail to read, which handlers answer with 400/413.
			ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, limit)
		}
		ctx.Header("X-Content-Type-Options", "nosniff")
		ctx.Next()
	}
}

// RateLimit allows each client IP rpm requests per minute with the given burst.
// The client IP comes from X-Forwarded-For only when the peer is listed in
// TRUSTED_PROXIES; otherwise it is the TCP peer, so the header can't be spoofed.
// ponytail: per-instance memory; move to Redis once there are several instances.
func RateLimit(rpm, burst int) gin.HandlerFunc {
	type client struct {
		limiter *rate.Limiter
		seen    time.Time
	}
	var (
		mu      sync.Mutex
		clients = map[string]*client{}
	)
	go func() {
		for range time.Tick(time.Minute) {
			mu.Lock()
			for ip, c := range clients {
				if time.Since(c.seen) > 10*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	every := rate.Every(time.Minute / time.Duration(rpm))
	return func(ctx *gin.Context) {
		ip := ctx.ClientIP()
		mu.Lock()
		c, ok := clients[ip]
		if !ok {
			c = &client{limiter: rate.NewLimiter(every, burst)}
			clients[ip] = c
		}
		c.seen = time.Now()
		allowed := c.limiter.Allow()
		mu.Unlock()

		if !allowed {
			ctx.Header("Retry-After", "60")
			abort(ctx, http.StatusTooManyRequests, "too many requests, try again shortly")
			return
		}
		ctx.Next()
	}
}

func abort(ctx *gin.Context, status int, message string) {
	ctx.AbortWithStatusJSON(status, adapters.BuildErrorResponse(fmt.Sprintf("%d %s", status, http.StatusText(status)), nil, fmt.Errorf("%s", message)))
}

// RequestTimeout gives every request a deadline, so the DB, Redis and IAM
// calls it makes stop when it runs out. Needs router.ContextWithFallback, so
// handlers passing *gin.Context as a context see this deadline. The server's
// WriteTimeout alone never cancels anything: it only cuts the connection.
func RequestTimeout(d time.Duration) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		c, cancel := context.WithTimeout(ctx.Request.Context(), d)
		defer cancel()
		ctx.Request = ctx.Request.WithContext(c)
		ctx.Next()
	}
}
