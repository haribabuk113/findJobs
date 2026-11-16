-- +goose Up
-- Tier configuration + candidate daily rate (from admin-portal-design.md §4.1)
-- Adds configurable tier parameters: applicant visibility windows, grace periods, fees
-- and candidate subscription daily rate + durations.

ALTER TABLE settings ADD COLUMN IF NOT EXISTS free_applicant_visibility_days INT NOT NULL DEFAULT 30 CHECK (free_applicant_visibility_days > 0);
ALTER TABLE settings ADD COLUMN IF NOT EXISTS free_application_review_days INT NOT NULL DEFAULT 30 CHECK (free_application_review_days > 0);
ALTER TABLE settings ADD COLUMN IF NOT EXISTS free_grace_purge_days INT NOT NULL DEFAULT 10 CHECK (free_grace_purge_days >= 0);
ALTER TABLE settings ADD COLUMN IF NOT EXISTS free_repost_fee_paise BIGINT NOT NULL DEFAULT 2000 CHECK (free_repost_fee_paise >= 0);
ALTER TABLE settings ADD COLUMN IF NOT EXISTS free_review_extend_fee_paise BIGINT NOT NULL DEFAULT 2000 CHECK (free_review_extend_fee_paise >= 0);
ALTER TABLE settings ADD COLUMN IF NOT EXISTS premium_applicant_visibility_days INT NOT NULL DEFAULT 30 CHECK (premium_applicant_visibility_days > 0);
ALTER TABLE settings ADD COLUMN IF NOT EXISTS premium_application_review_days INT NOT NULL DEFAULT 30 CHECK (premium_application_review_days > 0);
ALTER TABLE settings ADD COLUMN IF NOT EXISTS premium_grace_purge_days INT NOT NULL DEFAULT 10 CHECK (premium_grace_purge_days >= 0);
ALTER TABLE settings ADD COLUMN IF NOT EXISTS premium_repost_fee_paise BIGINT NOT NULL DEFAULT 2000 CHECK (premium_repost_fee_paise >= 0);
ALTER TABLE settings ADD COLUMN IF NOT EXISTS premium_review_extend_fee_paise BIGINT NOT NULL DEFAULT 2000 CHECK (premium_review_extend_fee_paise >= 0);
ALTER TABLE settings ADD COLUMN IF NOT EXISTS candidate_daily_rate_paise BIGINT NOT NULL DEFAULT 10000 CHECK (candidate_daily_rate_paise >= 0);
ALTER TABLE settings ADD COLUMN IF NOT EXISTS candidate_subscription_duration_days INT[] DEFAULT '{{7,15,30}}';

-- Update the column list used by repository (below)
-- +goose Down
ALTER TABLE settings DROP COLUMN IF EXISTS candidate_subscription_duration_days;
ALTER TABLE settings DROP COLUMN IF EXISTS candidate_daily_rate_paise;
ALTER TABLE settings DROP COLUMN IF EXISTS premium_review_extend_fee_paise;
ALTER TABLE settings DROP COLUMN IF EXISTS premium_repost_fee_paise;
ALTER TABLE settings DROP COLUMN IF EXISTS premium_grace_purge_days;
ALTER TABLE settings DROP COLUMN IF EXISTS premium_application_review_days;
ALTER TABLE settings DROP COLUMN IF EXISTS premium_applicant_visibility_days;
ALTER TABLE settings DROP COLUMN IF EXISTS free_review_extend_fee_paise;
ALTER TABLE settings DROP COLUMN IF EXISTS free_repost_fee_paise;
ALTER TABLE settings DROP COLUMN IF EXISTS free_grace_purge_days;
ALTER TABLE settings DROP COLUMN IF EXISTS free_application_review_days;
ALTER TABLE settings DROP COLUMN IF EXISTS free_applicant_visibility_days;