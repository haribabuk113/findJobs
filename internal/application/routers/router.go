package routers

import (
	"net/http"

	"findJobs/internal/adapters/inbound"

	"github.com/gin-gonic/gin"
)

func InitRoutes(profileHandler *inbound.Handler) (http.Handler, error) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	api := router.Group("/vol/v1")

	profileGroup := api.Group("/profile")
	inbound.RegisterRoutes(profileGroup, profileHandler)

	return router, nil
}
