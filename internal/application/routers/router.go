package routers

import (
	"findJobs/internal/application/profiles"
	"net/http"

	"github.com/gin-gonic/gin"
)

func InitRoutes(profileController *profiles.ProfileController) (http.Handler, error) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// router.Use(security.RequestIDMiddleware(10*time.Second), gin.Recovery()) // 10 Second Timeout
	// router.Use(security.RateLimiter())

	api := router.Group("/vol/v1")

	/* AUTH */
	// authGroup := api.Group("/auth")
	// auth.RegisterRoutes(authGroup, authController)

	/* USERS */
	profileGroup := api.Group("/profile")
	profiles.RegisterRoutes(profileGroup, profileController)

	return router, nil
}
