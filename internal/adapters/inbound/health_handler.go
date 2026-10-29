package inbound

import (
	"context"
	"errors"
	"net/http"
	"time"

	"findJobs/internal/adapters"
	"findJobs/internal/pkg/logger"

	"github.com/gin-gonic/gin"
)

// readyTimeout bounds the readiness DB ping so a hung database fails the probe
// instead of hanging it.
const readyTimeout = 2 * time.Second

// RegisterHealthRoutes registers the probes for Docker and load balancers:
//   - GET /healthz: the process is up and serving HTTP.
//   - GET /readyz: the database answers a ping. Redis is left out because the
//     app serves sessions from Postgres when Redis is down.
//
// Register these before the rate limiter, so frequent probes never use up a
// client bucket or get throttled.
func RegisterHealthRoutes(router gin.IRoutes, pingDB func(context.Context) error) {
	router.GET("/healthz", func(ctx *gin.Context) {
		ctx.Header("Cache-Control", "no-store")
		ctx.JSON(http.StatusOK, adapters.BuildResponse("ok", nil, nil))
	})
	router.GET("/readyz", func(ctx *gin.Context) {
		ctx.Header("Cache-Control", "no-store")
		pingCtx, cancel := context.WithTimeout(ctx.Request.Context(), readyTimeout)
		defer cancel()
		if err := pingDB(pingCtx); err != nil {
			// Never show the DB error to callers (system requirements §20).
			l := logger.Get()
			l.Error().Err(err).Msg("readiness check: database ping failed")
			ctx.JSON(http.StatusServiceUnavailable,
				adapters.BuildErrorResponse("not ready", nil, errors.New("database unavailable")))
			return
		}
		ctx.JSON(http.StatusOK, adapters.BuildResponse("ready", nil, nil))
	})
}
