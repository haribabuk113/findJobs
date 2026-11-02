-- +goose Up
-- The landing page's six counters (requirements doc "Platform Analytics":
-- backend-driven, updated once a day, no live data). One row, rewritten by
-- the daily landing-stats job and read by GET /api/v1/stats.
CREATE TABLE platform_stats (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    registered_users BIGINT NOT NULL DEFAULT 0,
    hr_professionals BIGINT NOT NULL DEFAULT 0,
    companies BIGINT NOT NULL DEFAULT 0,
    open_jobs BIGINT NOT NULL DEFAULT 0,
    countries_with_users BIGINT NOT NULL DEFAULT 0,
    countries_with_jobs_companies BIGINT NOT NULL DEFAULT 0,
    computed_at TIMESTAMPTZ
);
INSERT INTO platform_stats DEFAULT VALUES;

-- +goose Down
DROP TABLE platform_stats;
