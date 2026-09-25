package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"findJobs/internal/adapters/inbound"
	"findJobs/internal/adapters/outbound/postgres"
	"findJobs/internal/application/profiles"
	"findJobs/internal/application/routers"
	"findJobs/internal/pkg/config"
	"findJobs/internal/pkg/logger"
	postgrefacade "findJobs/internal/pkg/postgres"
	"findJobs/internal/pkg/redis"
)

func main() {
	log := logger.Get()

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

	// Initialize Redis client
	if _, err := redis.New(cfg); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Redis client")
	}
	defer redis.Close()

	log.Info().Msg("PostgreSQL and Redis connection pools initialized successfully")

	/* USERS */
	profileRepo := postgres.NewProfileRepository(db)
	profileService := profiles.NewProfileService(profileRepo)
	profileHandler := inbound.NewHandler(profileService)

	routes, err := routers.InitRoutes(profileHandler)
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

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server stopped gracefully")
}
