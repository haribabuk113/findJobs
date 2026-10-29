package config

import (
	"errors"
	"fmt"
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

// Environments the code distinguishes: "dev" relaxes the Secure cookie flag
// and logs to the console; "prod" enables the safety checks in validate.
const (
	EnvDev  = "dev"
	EnvProd = "prod"
)

// Defaults that are only acceptable in dev.
const (
	devDBPassword = "dbpass"
	// devTokenKey is the dev-only key committed in docker-compose.yml.
	devTokenKey = "swISuuCHgTpq4kwV3Rc6UqWdjxuB2Ko4spxYswfM9wM="
)

// Load reads the configuration from the environment. It fails, listing every
// problem at once, when ENVIRONMENT is missing or unknown, when a value can't
// be parsed, or when prod is configured with dev-only settings.
func Load() (*Config, error) {
	l := &loader{}
	cfg := &Config{
		Environment: os.Getenv("ENVIRONMENT"),
		Server: ServerConfig{
			Port:            l.str("SERVER_PORT", "8080"),
			Host:            l.str("SERVER_HOST", "0.0.0.0"),
			ReadTimeout:     l.duration("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:    l.duration("SERVER_WRITE_TIMEOUT", 15*time.Second),
			ShutdownTimeout: l.duration("SERVER_SHUTDOWN_TIMEOUT", 10*time.Second),
			TrustedProxies:  getEnvList("TRUSTED_PROXIES"),
		},
		Database: DatabaseConfig{
			Host:            l.str("DB_HOST", "localhost"),
			Port:            l.int("DB_PORT", 5432),
			Username:        l.str("DB_USERNAME", "dbuser"),
			Password:        l.str("DB_PASSWORD", devDBPassword),
			DBName:          l.str("DB_NAME", "jportal"),
			SSLMode:         l.str("DB_SSL_MODE", "disable"),
			MaxOpenConns:    l.int("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    l.int("DB_MAX_IDLE_CONNS", 25),
			ConnMaxLifetime: l.duration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Host:         l.str("REDIS_HOST", "localhost"),
			Port:         l.int("REDIS_PORT", 6379),
			Password:     l.str("REDIS_PASSWORD", ""),
			DB:           l.int("REDIS_DB", 0),
			MaxRetries:   l.int("REDIS_MAX_RETRIES", 3),
			PoolSize:     l.int("REDIS_POOL_SIZE", 10),
			MinIdleConns: l.int("REDIS_MIN_IDLE_CONNS", 5),
			DialTimeout:  l.duration("REDIS_DIAL_TIMEOUT", 5*time.Second),
		},
		IAM: IAMConfig{
			Endpoint: l.str("IAM_ENDPOINT", "http://localhost:8080"),
			AppID:    l.str("IAM_APP_ID", "jportal"),
			Issuer:   l.str("IAM_ISSUER", "https://auth.company.com"),
			Audience: l.str("IAM_AUDIENCE", "app-registry"),
		},
		RateLimit: RateLimitConfig{
			RequestsPerMinute: l.int("RATE_LIMIT_RPM", 100),
			Burst:             l.int("RATE_LIMIT_BURST", 20),
		},
		PlatformAdminIDs: getEnvList("PLATFORM_ADMIN_IDS"),
		FXEndpoint:       l.str("FX_ENDPOINT", "https://open.er-api.com/v6/latest/INR"),
		StorageDir:       l.str("STORAGE_DIR", "./uploads"),
		Session: SessionConfig{
			TokenEncKeys: os.Getenv("TOKEN_ENC_KEYS"),
			IdleTimeout:  l.duration("SESSION_IDLE_TIMEOUT", 168*time.Hour),
		},
	}
	if err := errors.Join(append(l.errs, cfg.validate()...)...); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, nil
}

// validate checks the environment, and in prod rejects dev-only settings.
func (c *Config) validate() []error {
	switch c.Environment {
	case EnvDev:
		return nil
	case EnvProd:
	case "":
		return []error{errors.New("ENVIRONMENT is required (dev or prod)")}
	default:
		return []error{fmt.Errorf("ENVIRONMENT must be dev or prod, got %q", c.Environment)}
	}
	var errs []error
	if c.Database.Password == devDBPassword {
		errs = append(errs, errors.New("DB_PASSWORD is the dev default; set a real password in prod"))
	}
	if c.Database.SSLMode == "disable" {
		errs = append(errs, errors.New("DB_SSL_MODE=disable is not allowed in prod; use require or verify-full"))
	}
	if c.Session.TokenEncKeys == "" {
		errs = append(errs, errors.New("TOKEN_ENC_KEYS is required"))
	} else if strings.Contains(c.Session.TokenEncKeys, devTokenKey) {
		errs = append(errs, errors.New("TOKEN_ENC_KEYS contains the dev key from docker-compose.yml; use a key from the secret manager in prod"))
	}
	return errs
}

// loader reads environment variables, recording a value that fails to parse
// instead of silently using the default.
type loader struct {
	errs []error
}

func (l *loader) str(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (l *loader) int(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("%s=%q is not an integer", key, value))
	}
	return n
}

func (l *loader) duration(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		l.errs = append(l.errs, fmt.Errorf("%s=%q is not a duration (e.g. 15s, 5m, 168h)", key, value))
	}
	return d
}

// getEnvList splits a comma-separated variable, dropping blanks.
func getEnvList(key string) []string {
	var list []string
	for _, item := range strings.Split(os.Getenv(key), ",") {
		if item = strings.TrimSpace(item); item != "" {
			list = append(list, item)
		}
	}
	return list
}
