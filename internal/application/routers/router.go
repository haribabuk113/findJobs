package routers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"findJobs/internal/adapters/inbound"

	"findJobs/internal/adapters/outbound/fx"
	"findJobs/internal/adapters/outbound/iam"
	"findJobs/internal/adapters/outbound/postgres"
	"findJobs/internal/adapters/outbound/razorpay"
	redisstore "findJobs/internal/adapters/outbound/redis"
	"findJobs/internal/adapters/outbound/storage"
	appaccount "findJobs/internal/application/account"
	appauth "findJobs/internal/application/auth"
	appcompanies "findJobs/internal/application/companies"
	appjobs "findJobs/internal/application/jobs"
	appprofiles "findJobs/internal/application/profiles"
	appapplications "findJobs/internal/application/applications"
	apppayments "findJobs/internal/application/payments"
	appreference "findJobs/internal/application/reference"
	appsettings "findJobs/internal/application/settings"
	domainaccount "findJobs/internal/domain/account"
	"findJobs/internal/pkg/background"
	"findJobs/internal/pkg/config"
	"findJobs/internal/pkg/logger"
	"findJobs/internal/pkg/scheduler"
	"findJobs/internal/pkg/tokencrypt"
	"findJobs/internal/ports"

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

// InitRoutes wires the app. Background work (rate refreshes, asset publishing)
// runs in bg, which main stops on shutdown before closing the pools.
func InitRoutes(cfg *config.Config, db *pgxpool.Pool, rdb *goredis.Client, bg *background.Group) (http.Handler, error) {
	// Debug mode only in dev: it prints route tables and warnings meant for a terminal.
	if cfg.Environment == config.EnvDev {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	// gin.New, not gin.Default: Default's text logger writes beside zerolog's
	// JSON and breaks log aggregation. RequestLogger replaces it (and records
	// ctx.Errors, the only place a 500's cause is kept); Recover runs inside it
	// so a panic is logged as that request's 500.
	router := gin.New()
	metrics := inbound.NewMetrics(db.Stat)
	router.Use(inbound.RequestLogger(logger.Get()), metrics.Middleware(), inbound.Recover())
	// Make *gin.Context forward Done/Err/Deadline to the request context, so a
	// client disconnect or the request deadline cancels the work in progress.
	router.ContextWithFallback = true
	// Gin trusts every proxy by default, which would let any client spoof
	// X-Forwarded-For and dodge the rate limiter. Trust only configured ones.
	if err := router.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		return nil, fmt.Errorf("invalid TRUSTED_PROXIES: %w", err)
	}
	if cfg.RateLimit.RequestsPerMinute <= 0 || cfg.RateLimit.Burst <= 0 {
		return nil, fmt.Errorf("RATE_LIMIT_RPM and RATE_LIMIT_BURST must be positive")
	}
	// Gin attaches middleware only to routes registered after Use, so the
	// probes registered here skip the rate limiter and the body guard.
	inbound.RegisterHealthRoutes(router, db.Ping)
	// Prometheus scrape target. Internal only, like the whole backend: it sits
	// behind the SvelteKit server, whose proxies never forward /metrics.
	router.GET("/metrics", metrics.Handler)
	router.Use(inbound.RequestTimeout(cfg.Server.WriteTimeout), inbound.RateLimit(cfg.RateLimit.RequestsPerMinute, cfg.RateLimit.Burst), inbound.JSONBody(maxJSONBody, maxImageBody))

	api := router.Group("/api/v1")

	// Auth routes
	authRepo := postgres.NewAuthRepository(db)
	tokenKeys, err := tokencrypt.Parse(cfg.Session.TokenEncKeys)
	if err != nil {
		return nil, fmt.Errorf("invalid TOKEN_ENC_KEYS: %w", err)
	}
	// Postgres is the durable session store; Redis caches session state in front of it.
	sessions := redisstore.NewSessionCache(postgres.NewSessionStore(db, tokenKeys), rdb)
	iamClient := iam.NewClient(cfg.IAM)
	authService := appauth.NewAuthService(authRepo, sessions, iamClient, cfg.Session.IdleTimeout)
	authHandler := inbound.NewAuthHandler(authService, cfg.Environment != "dev", cfg.PlatformAdminIDs)
	authGroup := api.Group("/auth")
	inbound.RegisterAuthRoutes(authGroup, authHandler)
	// Uploads (local disk until the Internxt S3 adapter is configured)
	store, err := storage.NewLocal(cfg.StorageDir)
	if err != nil {
		return nil, err
	}

	// Account and legal: consents and account deletion. Every other signed-in
	// route goes through the consent gate (requireUser below); these use the
	// plain session check, so a user can always accept the documents, or leave.
	legalDocs := make([]domainaccount.Document, len(cfg.LegalDocuments))
	for i, d := range cfg.LegalDocuments {
		legalDocs[i] = domainaccount.Document{ID: d.ID, Version: d.Version}
	}
	accountService := appaccount.NewAccountService(postgres.NewAccountRepository(db), sessions, iamClient, store, legalDocs)
	inbound.RegisterAccountRoutes(api, inbound.NewAccountHandler(accountService), authHandler.RequireUser())
	requireUser := authHandler.RequireConsentedUser(accountService.Gate)

	// Daily at 03:30 IST (and at startup): delete expired and idle sessions.
	scheduler.Daily(bg, db, "sessions-purge", 3, 30, postgres.PurgeSessions(db, cfg.Session.IdleTimeout))

	// Landing page counters: recomputed daily at 03:45 IST (and at startup), served by GET /stats.
	statsRepo := postgres.NewStatsRepository(db)
	scheduler.Daily(bg, db, "landing-stats", 3, 45, statsRepo.Refresh)
	inbound.RegisterStatsRoutes(api, statsRepo)

	// Settings: the admin's row, except the job-post fee, which comes from config for now.
	settingsRepo := appsettings.WithJobPostFee(postgres.NewSettingsRepository(db), cfg.JobPostFeePaise)

	// Job routes
	jobsHandler := inbound.NewJobsHandler(appjobs.NewJobService(postgres.NewJobRepository(db), settingsRepo))
	inbound.RegisterJobRoutes(api.Group("/jobs"), jobsHandler, authHandler.OptionalUser(), requireUser)

	// Platform admin routes
	settingsHandler := inbound.NewSettingsHandler(appsettings.NewSettingsService(settingsRepo))
	inbound.RegisterSettingsRoutes(api, api.Group("/admin"), settingsHandler, authHandler.RequirePlatformAdmin())

	// Reference data: taxonomies, locations, exchange rates, published JSON files
	referenceService := appreference.NewReferenceService(bg, postgres.NewTaxonomyRepository(db), postgres.NewLocationRepository(db),
		postgres.NewRatesRepository(db), postgres.NewAssetRepository(db), fx.NewClient(cfg.FXEndpoint))
	inbound.RegisterReferenceRoutes(api.Group("/assets"), api.Group("/admin"), inbound.NewReferenceHandler(referenceService), authHandler.RequirePlatformAdmin())
	bg.Go(func(ctx context.Context) { startReferenceData(ctx, referenceService) })
	// Daily FX refresh at the admin's fx_refresh_time (05:00 IST by default).
	// Polled every 15 minutes, so a changed time applies without a restart, an
	// empty rates table is filled at startup, and a failed refresh is retried.
	scheduler.Every(bg, db, "fx-refresh", 15*time.Minute, func(ctx context.Context) error {
		s, err := settingsRepo.Get(ctx)
		if err != nil {
			return err
		}
		return referenceService.RefreshRatesIfDue(ctx, s.FXRefreshTime, time.Now())
	})
	// Daily at 00:05 IST (and at startup): hide listings past the 30-day public window.
	scheduler.Daily(bg, db, "jobs-expiry", 0, 5, postgres.ExpireJobs(db))

	// Wallets and top-ups. Without Razorpay keys, wallets read as usual and top-ups return 503.
	var gateway ports.PaymentGateway
	if cfg.Razorpay.KeyID != "" {
		gateway = razorpay.New(cfg.Razorpay.KeyID, cfg.Razorpay.KeySecret, cfg.Razorpay.WebhookSecret)
	}
	paymentService := apppayments.NewPaymentService(postgres.NewWalletRepository(db), gateway)
	inbound.RegisterPaymentRoutes(api, inbound.NewPaymentsHandler(paymentService), requireUser)
	if gateway != nil {
		// Daily at 02:00 IST (and at startup): catch paid orders whose webhook never came, flag mismatches.
		scheduler.Daily(bg, db, "payments-reconcile", 2, 0, paymentService.Reconcile)
	}

	// Recruiting: posting and managing jobs (paid posts charge through payments), job pages, recruiter cards
	recruitingService := appjobs.NewRecruitingService(postgres.NewRecruitingRepository(db), referenceService,
		settingsRepo, paymentService)
	inbound.RegisterRecruitingRoutes(api, inbound.NewRecruitingHandler(recruitingService), authHandler.OptionalUser(), requireUser)

	// Companies
	companyService := appcompanies.NewCompanyService(postgres.NewCompanyRepository(db), store)
	inbound.RegisterCompanyRoutes(api, inbound.NewCompanyHandler(companyService), authHandler.OptionalUser(), requireUser)

	// Candidate profiles: shared taxonomy terms, company verification, photo uploads
	profileService := appprofiles.NewProfileService(postgres.NewProfileRepository(db), referenceService, companyService, store)
	// Profile analytics: each public profile load is counted in Redis (Postgres
	// when Redis is down), flushed to Postgres every 5 minutes, and rolled up
	// daily at 00:20 IST (and at startup, to catch up after downtime).
	analyticsService := appprofiles.NewAnalyticsService(redisstore.NewProfileViewCounter(postgres.NewProfileAnalyticsRepository(db), rdb))
	scheduler.Every(bg, db, "profile-views-flush", 5*time.Minute, analyticsService.Flush)
	scheduler.Daily(bg, db, "profile-analytics-rollup", 0, 20, analyticsService.Rollup)
	inbound.RegisterProfileRoutes(api, inbound.NewProfileHandler(profileService, analyticsService), authHandler.OptionalUser(), requireUser)

	// Applications: Easy Apply with the daily free quota, external confirmations, applicant pipeline
	applicationService := appapplications.NewApplicationService(postgres.NewApplicationRepository(db), settingsRepo)
	inbound.RegisterApplicationRoutes(api, inbound.NewApplicationsHandler(applicationService), requireUser)
	return router, nil
}

// startReferenceData makes sure every asset kind has a published file (rates
// come from the fx-refresh job). Runs in the background so startup isn't delayed.
func startReferenceData(ctx context.Context, service appreference.ReferenceService) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := service.PublishAll(ctx); err != nil {
		log := logger.Get()
		log.Error().Err(err).Msg("publishing reference files failed")
	}
}
