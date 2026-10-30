package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"findJobs/internal/application/routers"
	"findJobs/internal/pkg/background"
	"findJobs/internal/pkg/config"
	"findJobs/internal/pkg/logger"
	postgrefacade "findJobs/internal/pkg/postgres"
	"findJobs/internal/pkg/redis"
	"findJobs/migrations"
)

//go:generate go run github.com/swaggo/swag/cmd/swag@v1.16.4 init -q -g main.go -d .,../internal/adapters/inbound,../internal/adapters,../internal/domain/applications,../internal/domain/auth,../internal/domain/companies,../internal/domain/jobs,../internal/domain/profiles,../internal/domain/reference,../internal/domain/settings --parseInternal --outputTypes yaml -o ../docs/swagger

// main wires and starts the API. The lines below are the swag API header.
//
//	@title						Referise Backend API
//	@version					1.1.0
//	@BasePath					/
//	@securityDefinitions.apikey	sessionCookie
//	@in							cookie
//	@name						jp_session
func main() {
	log := logger.Get()

	ist, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load IST timezone")
	}
	time.Local = ist

	// Load application configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Initialize PostgreSQL connection pool
	db, err := postgrefacade.New(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize PostgreSQL connection pool")
	}
	defer postgrefacade.Close()

	if err := postgrefacade.Migrate(context.Background(), db, migrations.FS); err != nil {
		log.Fatal().Err(err).Msg("Failed to apply database migrations")
	}

	rdb, err := redis.New(cfg)
	if err != nil {
		// Not fatal: session lookups fall through to Postgres until Redis is back.
		log.Warn().Err(err).Msg("Redis unavailable at startup; continuing without session cache")
	}
	defer redis.Close()

	log.Info().Msg("PostgreSQL and Redis connection pools initialized successfully")

	bg := background.New()
	routes, err := routers.InitRoutes(cfg, db, rdb, bg)

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize routes")
	}

	// Start server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      routes,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Info().Str("addr", server.Addr).Msg("Server starting")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")

	// Order matters: finish in-flight requests (they may start background
	// work), then stop the background work, and only then let the deferred
	// closes shut the Redis and Postgres pools. All within SERVER_SHUTDOWN_TIMEOUT.
	// No log.Fatal from here on: it would exit without running those closes.
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("HTTP server didn't drain in time; open requests were cut off")
	}
	if !bg.Stop(time.Until(deadline(ctx))) {
		log.Error().Msg("Background work still running at the shutdown deadline")
	}

	log.Info().Msg("Server stopped gracefully")
}

func deadline(ctx context.Context) time.Time {
	d, _ := ctx.Deadline()
	return d
}
