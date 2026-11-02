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

func TestJobPostFee(t *testing.T) {
	t.Setenv("ENVIRONMENT", "dev")
	t.Setenv("JOB_POST_FEE_PAISE", "")
	if cfg, err := Load(); err != nil || cfg.JobPostFeePaise != 200 {
		t.Fatalf("default: %+v %v", cfg, err)
	}
	t.Setenv("JOB_POST_FEE_PAISE", "0")
	if cfg, err := Load(); err != nil || cfg.JobPostFeePaise != 0 {
		t.Fatalf("0 (free): %+v %v", cfg, err)
	}
	for _, v := range []string{"-1", "10000001", "2.5"} {
		t.Setenv("JOB_POST_FEE_PAISE", v)
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "JOB_POST_FEE_PAISE") {
			t.Errorf("%s: got %v", v, err)
		}
	}
}

func TestLegalDocuments(t *testing.T) {
	t.Setenv("ENVIRONMENT", "dev")
	t.Setenv("LEGAL_DOCUMENTS", "")
	if cfg, err := Load(); err != nil || len(cfg.LegalDocuments) != 0 {
		t.Fatalf("unset: %+v %v", cfg, err)
	}
	t.Setenv("LEGAL_DOCUMENTS", "terms:2026-11-02, privacy:v1.2")
	cfg, err := Load()
	if err != nil || len(cfg.LegalDocuments) != 2 || cfg.LegalDocuments[1] != (LegalDocument{"privacy", "v1.2"}) {
		t.Fatalf("parsed: %+v %v", cfg, err)
	}
	for _, v := range []string{"terms", "terms:", ":v1", "terms:v1,terms:v2", "Terms:v1", "terms:v 1"} {
		t.Setenv("LEGAL_DOCUMENTS", v)
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "LEGAL_DOCUMENTS") {
			t.Errorf("%q: got %v", v, err)
		}
	}
}
