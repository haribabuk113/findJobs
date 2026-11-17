package config

import (
	"strings"
	"testing"
)

func TestIAMEndpointHTTPSInProd(t *testing.T) {
	for _, tc := range []struct {
		name      string
		endpoint  string
		wantError bool
	}{
		{"https is ok", "https://iam.example.com", false},
		{"http fails in prod", "http://iam.example.com", true},
		{"empty fails in prod", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Create a prod config with the test endpoint
			prodCfg := &Config{
				Environment: EnvProd,
				IAM:         IAMConfig{Endpoint: tc.endpoint, AppID: "test", Issuer: "https://auth", Audience: "app"},
				Session:     SessionConfig{TokenEncKeys: "kid1:base64key1", IdleTimeout: 168 * 24 * 60 * 60 * 1000000000},
				Database:    DatabaseConfig{Password: "realpassword", SSLMode: "require", MaxConns: 25, MinConns: 2},
			}
			errs := prodCfg.validate()
			hasError := false
			for _, e := range errs {
				if strings.Contains(e.Error(), "IAM_ENDPOINT") {
					hasError = true
					break
				}
			}
			if hasError != tc.wantError {
				t.Errorf("validate() IAM error mismatch: got %v, want %v (errs: %v)", hasError, tc.wantError, errs)
			}
		})
	}
}

func TestIAMEndpointHTTPAllowedInDev(t *testing.T) {
	cfg := &Config{
		Environment: EnvDev,
		IAM:         IAMConfig{Endpoint: "http://localhost:8080", AppID: "test", Issuer: "https://auth", Audience: "app"},
		Session:     SessionConfig{TokenEncKeys: "kid1:base64key1", IdleTimeout: 168 * 24 * 60 * 60 * 1000000000},
		Database:    DatabaseConfig{Password: devDBPassword, SSLMode: "disable", MaxConns: 25, MinConns: 2},
	}
	errs := cfg.validate()
	for _, e := range errs {
		if strings.Contains(e.Error(), "IAM_ENDPOINT") {
			t.Errorf("dev should allow HTTP IAM_ENDPOINT, got: %v", e)
		}
	}
}