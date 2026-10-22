package routers

import (
	"net/http"

	"findJobs/internal/adapters/inbound"

	"github.com/gin-gonic/gin"
)

func InitRoutes(authHandler *inbound.AuthHandler, mastersHandler *inbound.MastersHandler, profileHandler *inbound.ProfileHandler) (http.Handler, error) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	api := router.Group("/vol/v1")

	// Auth routes
	authGroup := api.Group("/auth")
	inbound.RegisterAuthRoutes(authGroup, authHandler)

	// Masters routes
	masterGroup := api.Group("/admin")
	inbound.RegisterMastersRoutes(masterGroup, mastersHandler)

	// Profile routes
	profileGroup := api.Group("/profile")
	inbound.RegisterRoutes(profileGroup, profileHandler)

	return router, nil
}
