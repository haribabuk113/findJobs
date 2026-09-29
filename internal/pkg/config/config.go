package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Environment string
	Server      ServerConfig
	Database    DatabaseConfig
	Redis       RedisConfig
	Search      SearchConfig
	IAM         IAMConfig
	JWT         JWTConfig
	RateLimit   RateLimitConfig
}

type ServerConfig struct {
	Port            string
	Host            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	Host            string
	Port            int
	Username        string
	Password        string
	DBName          string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	Host         string
	Port         int
	Password     string
	DB           int
	MaxRetries   int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
}

type SearchConfig struct {
	Host      string
	APIKey    string
	IndexName string
}

type IAMConfig struct {
	Endpoint     string
	ClientID     string
	ClientSecret string
	TokenURL     string
	Audience     string
}

type JWTConfig struct {
	SecretKey          string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
	TokenType          string
}

type RateLimitConfig struct {
	RequestsPerMinute int
	Burst             int
}

func Load() (*Config, error) {
	// JPORTAL prefix for all environment variables (mimicking original viper behavior)
	return &Config{
		Environment: getEnvOrDefault("ENVIRONMENT", "dev"),
		Server: ServerConfig{
			Port:            getEnvOrDefault("SERVER_PORT", "8080"),
			Host:            getEnvOrDefault("SERVER_HOST", "0.0.0.0"),
			ReadTimeout:     getEnvDurationOrDefault("SERVER_READ_TIMEOUT", "15s"),
			WriteTimeout:    getEnvDurationOrDefault("SERVER_WRITE_TIMEOUT", "15s"),
			ShutdownTimeout: getEnvDurationOrDefault("SERVER_SHUTDOWN_TIMEOUT", "10s"),
		},
		Database: DatabaseConfig{
			Host:            getEnvOrDefault("DB_HOST", "localhost"),
			Port:            getEnvIntOrDefault("DB_PORT", 5432),
			Username:        getEnvOrDefault("DB_USERNAME", "dbuser"),
			Password:        getEnvOrDefault("DB_PASSWORD", "dbpass"),
			DBName:          getEnvOrDefault("DB_NAME", "jportal"),
			SSLMode:         getEnvOrDefault("DB_SSL_MODE", "disable"),
			MaxOpenConns:    getEnvIntOrDefault("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvIntOrDefault("DB_MAX_IDLE_CONNS", 25),
			ConnMaxLifetime: getEnvDurationOrDefault("DB_CONN_MAX_LIFETIME", "5m"),
		},
		Redis: RedisConfig{
			Host:         getEnvOrDefault("REDIS_HOST", "localhost"),
			Port:         getEnvIntOrDefault("REDIS_PORT", 6379),
			Password:     getEnvOrDefault("REDIS_PASSWORD", ""),
			DB:           getEnvIntOrDefault("REDIS_DB", 0),
			MaxRetries:   getEnvIntOrDefault("REDIS_MAX_RETRIES", 3),
			PoolSize:     getEnvIntOrDefault("REDIS_POOL_SIZE", 10),
			MinIdleConns: getEnvIntOrDefault("REDIS_MIN_IDLE_CONNS", 5),
			DialTimeout:  getEnvDurationOrDefault("REDIS_DIAL_TIMEOUT", "5s"),
		},
		Search: SearchConfig{
			Host:      getEnvOrDefault("SEARCH_HOST", "localhost"),
			APIKey:    getEnvOrDefault("SEARCH_API_KEY", ""),
			IndexName: "jobs",
		},
		IAM: IAMConfig{
			Endpoint:     getEnvOrDefault("IAM_ENDPOINT", "http://localhost:8081"),
			ClientID:     getEnvOrDefault("IAM_CLIENT_ID", "jportal"),
			ClientSecret: getEnvOrDefault("IAM_CLIENT_SECRET", "secret"),
			TokenURL:     getEnvOrDefault("IAM_TOKEN_URL", "http://localhost:8081/api/v1/token"),
			Audience:     getEnvOrDefault("IAM_AUDIENCE", "jportal"),
		},
		JWT: JWTConfig{
			SecretKey:          getEnvOrDefault("JWT_SECRET", "default-secret-key-change-in-production"),
			AccessTokenExpiry:  getEnvDurationOrDefault("JWT_ACCESS_EXPIRY", "15m"),
			RefreshTokenExpiry: getEnvDurationOrDefault("JWT_REFRESH_EXPIRY", "168h"),
			TokenType:          "Bearer",
		},
		RateLimit: RateLimitConfig{
			RequestsPerMinute: getEnvIntOrDefault("RATE_LIMIT_RPM", 100),
			Burst:             getEnvIntOrDefault("RATE_LIMIT_BURST", 20),
		},
	}, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvDurationOrDefault(key, defaultValue string) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	if duration, err := time.ParseDuration(defaultValue); err == nil {
		return duration
	}
	return 15 * time.Second
}
