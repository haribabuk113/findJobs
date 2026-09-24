package redis

import (
	"context"
	"fmt"
	"time"

	"findJobs/internal/pkg/config"

	"github.com/redis/go-redis/v9"
)

var client *redis.Client

// New creates a new Redis client using the provided configuration.
// It validates the connection by pinging the Redis server before returning.
// Returns the client on success, or an error if connection fails.
func New(cfg *config.Config) (*redis.Client, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port)

	redisClient := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		MaxRetries:   cfg.Redis.MaxRetries,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
		DialTimeout:  cfg.Redis.DialTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Verify the connection works
	if err := redisClient.Ping(ctx).Err(); err != nil {
		_ = redisClient.Close()
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	// Store the client in the package-level variable for the Get() function
	client = redisClient

	return redisClient, nil
}

// Get returns the global Redis client.
// It panics if the client has not been initialized via New().
func Get() *redis.Client {
	if client == nil {
		panic("redis client not initialized - call redis.New(cfg) first")
	}
	return client
}

// Close closes the Redis client and releases all resources.
func Close() {
	if client != nil {
		_ = client.Close()
		client = nil
	}
}
