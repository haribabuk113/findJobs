-- +goose Up
-- Admin-configurable platform values (manual: "Search jobs", "Post a job",
-- "Salary"). One row, so every reader sees the same values; the CHECKs are
-- the last line of defence behind domain validation. Money is in paise.
CREATE TABLE IF NOT EXISTS settings (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    daily_free_applications INT NOT NULL DEFAULT 15 CHECK (daily_free_applications BETWEEN 0 AND 1000),
    apply_fee_paise BIGINT NOT NULL DEFAULT 500 CHECK (apply_fee_paise BETWEEN 0 AND 10000000),
    match_sort_fee_paise BIGINT NOT NULL DEFAULT 2000 CHECK (match_sort_fee_paise BETWEEN 0 AND 10000000),
    job_post_fee_paise BIGINT NOT NULL DEFAULT 0 CHECK (job_post_fee_paise BETWEEN 0 AND 10000000),
    -- HH:MM in IST; the daily exchange-rate job runs at this time.
    fx_refresh_time TEXT NOT NULL DEFAULT '05:00' CHECK (fx_refresh_time ~ '^([01][0-9]|2[0-3]):[0-5][0-9]$'),
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

INSERT INTO settings DEFAULT VALUES ON CONFLICT DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS settings;
