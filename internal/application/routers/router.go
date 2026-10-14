package routers

import (
	"net/http"

	"findJobs/internal/adapters/inbound"
	"findJobs/internal/adapters/outbound/iam"
	"findJobs/internal/adapters/outbound/postgres"
	appauth "findJobs/internal/application/auth"
	"findJobs/internal/pkg/config"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func InitRoutes(cfg *config.Config, profileHandler *inbound.Handler, db *pgxpool.Pool) (http.Handler, error) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	api := router.Group("/vol/v1")

	// Profile routes
	profileGroup := api.Group("/profile")
	inbound.RegisterRoutes(profileGroup, profileHandler)

	// Auth routes
	authRepo := postgres.NewAuthRepository(db)
	authService := appauth.NewAuthService(authRepo, iam.NewClient(cfg.IAM))
	authHandler := inbound.NewAuthHandler(authService, cfg.Environment != "dev")
	authGroup := api.Group("/auth")
	inbound.RegisterAuthRoutes(authGroup, authHandler)

	return router, nil
}
