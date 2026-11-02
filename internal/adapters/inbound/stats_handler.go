package inbound

import (
	"fmt"
	"net/http"

	"findJobs/internal/adapters"
	"findJobs/internal/ports"

	"github.com/gin-gonic/gin"
)

// RegisterStatsRoutes registers GET /stats on api (the /api/v1 group). The
// counters change once a day, so the handler reads the stored row directly.
func RegisterStatsRoutes(api *gin.RouterGroup, repo ports.StatsRepository) {
	api.GET("/stats", platformStats(repo))
}

// platformStats handles GET /stats: the landing page's six counters.
//
//	@Summary	Landing page platform counters (updated daily)
//	@ID			getPlatformStats
//	@Tags		stats
//	@Produce	json
//	@Success	200	{object}	adapters.Response{data=stats.Platform}
//	@Failure	500	{object}	adapters.Response
//	@Router		/api/v1/stats [get]
func platformStats(repo ports.StatsRepository) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		s, err := repo.Get(ctx)
		if err != nil {
			ctx.Error(err)
			ctx.JSON(http.StatusInternalServerError, adapters.BuildErrorResponse("500 Internal Server Error", nil, fmt.Errorf("internal error")))
			return
		}
		// The same for everyone and refreshed daily: safe for shared caches.
		ctx.Header("Cache-Control", "public, max-age=3600")
		ctx.JSON(http.StatusOK, adapters.BuildResponse("Platform stats retrieved successfully", s, nil))
	}
}
