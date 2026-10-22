package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment string
	Server      ServerConfig
	Database    DatabaseConfig
	Redis       RedisConfig
	IAM         IAMConfig
	RateLimit   RateLimitConfig
	// PlatformAdminIDs are the IAM ecosystem IDs (JWT sub) allowed into the
	// findJobs /admin routes. Separate from IAM's own admin list.
	PlatformAdminIDs []string
	Session          SessionConfig
	// FXEndpoint returns exchange rates with INR as the base (open.er-api.com format).
	FXEndpoint string
	// StorageDir is where the local-disk ObjectStore keeps uploads.
	StorageDir string
}

type SessionConfig struct {
	// TokenEncKeys is "kid:base64key[,...]" (32-byte keys); the first one
	// encrypts IAM tokens at rest. Required: there is deliberately no default.
	TokenEncKeys string
	// IdleTimeout ends sessions unused this long (IAM's 30-day absolute limit still applies).
	IdleTimeout time.Duration
}

type ServerConfig struct {
	Port            string
	Host            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	// TrustedProxies are the CIDRs/IPs (SvelteKit server, Cloudflare) whose
	// X-Forwarded-For is believed. Empty: trust none, use the TCP peer.
	TrustedProxies []string
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

type IAMConfig struct {
	Endpoint string // IAM base URL, server-to-server
	AppID    string // this app's id registered in IAM_ALLOWED_APPS
	Issuer   string // must match IAM's JWT_ISSUER
	Audience string // IAM tokens always carry aud "app-registry"
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
			TrustedProxies:  getEnvListOrDefault("TRUSTED_PROXIES"),
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
		IAM: IAMConfig{
			Endpoint: getEnvOrDefault("IAM_ENDPOINT", "http://localhost:8080"),
			AppID:    getEnvOrDefault("IAM_APP_ID", "jportal"),
			Issuer:   getEnvOrDefault("IAM_ISSUER", "https://auth.company.com"),
			Audience: getEnvOrDefault("IAM_AUDIENCE", "app-registry"),
		},
		RateLimit: RateLimitConfig{
			RequestsPerMinute: getEnvIntOrDefault("RATE_LIMIT_RPM", 100),
			Burst:             getEnvIntOrDefault("RATE_LIMIT_BURST", 20),
		},
		PlatformAdminIDs: getEnvListOrDefault("PLATFORM_ADMIN_IDS"),
		FXEndpoint:       getEnvOrDefault("FX_ENDPOINT", "https://open.er-api.com/v6/latest/INR"),
		StorageDir:       getEnvOrDefault("STORAGE_DIR", "./uploads"),
		Session: SessionConfig{
			TokenEncKeys: os.Getenv("TOKEN_ENC_KEYS"),
			IdleTimeout:  getEnvDurationOrDefault("SESSION_IDLE_TIMEOUT", "168h"),
		},
	}, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvListOrDefault splits a comma-separated variable, dropping blanks.
func getEnvListOrDefault(key string) []string {
	var list []string
	for _, item := range strings.Split(os.Getenv(key), ",") {
		if item = strings.TrimSpace(item); item != "" {
			list = append(list, item)
		}
	}
	return list
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
