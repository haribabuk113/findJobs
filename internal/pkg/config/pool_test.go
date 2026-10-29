package config

import (
	"strings"
	"testing"
)

func TestPoolSizes(t *testing.T) {
	t.Setenv("ENVIRONMENT", "dev")
	for _, k := range []string{"DB_MAX_CONNS", "DB_MIN_CONNS"} {
		t.Setenv(k, "")
	}
	cfg, err := Load()
	if err != nil || cfg.Database.MaxConns != 25 || cfg.Database.MinConns != 2 {
		t.Fatalf("defaults: %+v %v", cfg, err)
	}
	for _, tc := range []struct{ max, min, want string }{
		{"5", "6", "DB_MIN_CONNS must be 0 to DB_MAX_CONNS (5)"},
		{"0", "0", "DB_MAX_CONNS must be at least 1"},
		{"5", "-1", "DB_MIN_CONNS must be 0 to DB_MAX_CONNS"},
	} {
		t.Setenv("DB_MAX_CONNS", tc.max)
		t.Setenv("DB_MIN_CONNS", tc.min)
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("max %s min %s: got %v, want %q", tc.max, tc.min, err, tc.want)
		}
	}
}
