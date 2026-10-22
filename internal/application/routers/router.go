package routers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"findJobs/internal/adapters/inbound"

	"github.com/gin-gonic/gin"
)

func InitRoutes(authHandler *inbound.AuthHandler, mastersHandler *inbound.MastersHandler, profileHandler *inbound.ProfileHandler) (http.Handler, error) {
	"findJobs/internal/adapters/outbound/fx"
	"findJobs/internal/adapters/outbound/iam"
	"findJobs/internal/adapters/outbound/postgres"
	redisstore "findJobs/internal/adapters/outbound/redis"
	"findJobs/internal/adapters/outbound/storage"
	appauth "findJobs/internal/application/auth"
	appcompanies "findJobs/internal/application/companies"
	appjobs "findJobs/internal/application/jobs"
	appreference "findJobs/internal/application/reference"
	appsettings "findJobs/internal/application/settings"
	"findJobs/internal/pkg/config"
	"findJobs/internal/pkg/logger"
	"findJobs/internal/pkg/tokencrypt"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

// Request body caps: JSON documents, and raw image uploads (the largest image
// kind, a 5 MB company banner; handlers enforce each kind's own limit).
const (
	maxJSONBody  = 1 << 20
	maxImageBody = 5 << 20
)

func InitRoutes(cfg *config.Config, profileHandler *inbound.Handler, db *pgxpool.Pool, rdb *goredis.Client) (http.Handler, error) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	// Gin trusts every proxy by default, which would let any client spoof
	// X-Forwarded-For and dodge the rate limiter. Trust only configured ones.
	if err := router.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		return nil, fmt.Errorf("invalid TRUSTED_PROXIES: %w", err)
	}
	if cfg.RateLimit.RequestsPerMinute <= 0 || cfg.RateLimit.Burst <= 0 {
		return nil, fmt.Errorf("RATE_LIMIT_RPM and RATE_LIMIT_BURST must be positive")
	}
	router.Use(inbound.RateLimit(cfg.RateLimit.RequestsPerMinute, cfg.RateLimit.Burst), inbound.JSONBody(maxJSONBody, maxImageBody))

	api := router.Group("/api/v1")

	// Auth routes
	authGroup := api.Group("/auth")
	inbound.RegisterAuthRoutes(authGroup, authHandler)

	// Masters routes
	masterGroup := api.Group("/admin")
	inbound.RegisterMastersRoutes(masterGroup, mastersHandler)

	// Profile routes
	profileGroup := api.Group("/profile")
	inbound.RegisterRoutes(profileGroup, profileHandler)

	// Auth routes
	authRepo := postgres.NewAuthRepository(db)
	tokenKeys, err := tokencrypt.Parse(cfg.Session.TokenEncKeys)
	if err != nil {
		return nil, fmt.Errorf("invalid TOKEN_ENC_KEYS: %w", err)
	}
	// Postgres is the durable session store; Redis caches session state in front of it.
	sessions := redisstore.NewSessionCache(postgres.NewSessionStore(db, tokenKeys), rdb)
	authService := appauth.NewAuthService(authRepo, sessions, iam.NewClient(cfg.IAM), cfg.Session.IdleTimeout)
	authHandler := inbound.NewAuthHandler(authService, cfg.Environment != "dev", cfg.PlatformAdminIDs)
	authGroup := api.Group("/auth")
	inbound.RegisterAuthRoutes(authGroup, authHandler)

	// Job routes
	jobsHandler := inbound.NewJobsHandler(appjobs.NewJobService(postgres.NewJobRepository(db)))
	inbound.RegisterJobRoutes(api.Group("/jobs"), jobsHandler, authHandler.OptionalUser())

	// Platform admin routes
	settingsHandler := inbound.NewSettingsHandler(appsettings.NewSettingsService(postgres.NewSettingsRepository(db)))
	inbound.RegisterSettingsRoutes(api.Group("/admin"), settingsHandler, authHandler.RequirePlatformAdmin())

	// Reference data: taxonomies, locations, exchange rates, published JSON files
	referenceService := appreference.NewReferenceService(postgres.NewTaxonomyRepository(db), postgres.NewLocationRepository(db),
		postgres.NewRatesRepository(db), postgres.NewAssetRepository(db), fx.NewClient(cfg.FXEndpoint))
	inbound.RegisterReferenceRoutes(api.Group("/assets"), api.Group("/admin"), inbound.NewReferenceHandler(referenceService), authHandler.RequirePlatformAdmin())
	go startReferenceData(referenceService)

	// Recruiting: posting and managing jobs, job pages, recruiter cards
	recruitingService := appjobs.NewRecruitingService(postgres.NewRecruitingRepository(db), referenceService,
		postgres.NewSettingsRepository(db))
	inbound.RegisterRecruitingRoutes(api, inbound.NewRecruitingHandler(recruitingService), authHandler.OptionalUser(), authHandler.RequireUser())

	// Companies (uploads: local disk until the Internxt S3 adapter is configured)
	store, err := storage.NewLocal(cfg.StorageDir)
	if err != nil {
		return nil, err
	}
	companyService := appcompanies.NewCompanyService(postgres.NewCompanyRepository(db), store)
	inbound.RegisterCompanyRoutes(api, inbound.NewCompanyHandler(companyService), authHandler.OptionalUser(), authHandler.RequireUser())

	return router, nil
}

// startReferenceData makes sure every asset kind has a published file, and
// fetches exchange rates once if none exist yet (the daily refresh is a
// scheduled job). Runs in the background so a slow provider can't delay startup.
func startReferenceData(service appreference.ReferenceService) {
	log := logger.Get()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := service.PublishAll(ctx); err != nil {
		log.Error().Err(err).Msg("publishing reference files failed")
		return
	}
	if manifest, err := service.Manifest(ctx); err == nil {
		if _, ok := manifest["rates"]; !ok {
			service.RefreshRatesAsync()
		}
	}
}
