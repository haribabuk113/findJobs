package postgres

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"findJobs/internal/pkg/config"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

var pool *pgxpool.Pool

// New creates a new PostgreSQL connection pool using the provided configuration.
// It validates the connection by pinging the database before returning.
// Returns the pool on success, or an error if connection fails.
func New(cfg *config.Config) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.SSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres config: %w", err)
	}

	// Apply pool settings from config
	poolConfig.MaxConns = int32(cfg.Database.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.Database.MaxIdleConns)
	poolConfig.MaxConnLifetime = cfg.Database.ConnMaxLifetime

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	newPool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres pool: %w", err)
	}

	// Verify the connection works
	if err := newPool.Ping(ctx); err != nil {
		newPool.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	// Store the pool in the package-level variable for the Get() function
	pool = newPool

	return newPool, nil
}

// Migrate applies pending migrations from fsys. A Postgres advisory lock
// serialises concurrent starts, so only one instance migrates at a time.
func Migrate(ctx context.Context, db *pgxpool.Pool, fsys fs.FS) error {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("failed to create migration lock: %w", err)
	}
	sqlDB := stdlib.OpenDBFromPool(db)
	defer sqlDB.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, fsys, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("failed to create migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	return nil
}

// Get returns the global PostgreSQL connection pool.
// It panics if the pool has not been initialized via New().
func Get() *pgxpool.Pool {
	if pool == nil {
		panic("postgres pool not initialized - call postgres.New(cfg) first")
	}
	return pool
}

// Close closes the PostgreSQL connection pool and releases all resources.
func Close() {
	if pool != nil {
		pool.Close()
		pool = nil
	}
}
