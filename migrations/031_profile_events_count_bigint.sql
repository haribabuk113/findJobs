-- +goose Up
-- Profile events count column is INT (32-bit signed) which overflows at ~2.1 billion.
-- Change to BIGINT so high-traffic profiles (>15 days retention) cannot silently
-- wrap around when counts exceed int32 max.
ALTER TABLE profile_events ALTER COLUMN count TYPE BIGINT USING (count::bigint);

-- Also widen the rolled-up daily stats columns for consistency, since they derive
-- from profile_events.count via SUM().
ALTER TABLE profile_daily_stats ALTER COLUMN views_search TYPE BIGINT USING (views_search::bigint);
ALTER TABLE profile_daily_stats ALTER COLUMN views_direct TYPE BIGINT USING (views_direct::bigint);
ALTER TABLE profile_daily_stats ALTER COLUMN resume_views TYPE BIGINT USING (resume_views::bigint);
ALTER TABLE profile_daily_stats ALTER COLUMN resume_downloads TYPE BIGINT USING (resume_downloads::bigint);

-- +goose Down
ALTER TABLE profile_events ALTER COLUMN count TYPE INT USING (count::int);
ALTER TABLE profile_daily_stats ALTER COLUMN views_search TYPE INT USING (views_search::int);
ALTER TABLE profile_daily_stats ALTER COLUMN views_direct TYPE INT USING (views_direct::int);
ALTER TABLE profile_daily_stats ALTER COLUMN resume_views TYPE INT USING (resume_views::int);
ALTER TABLE profile_daily_stats ALTER COLUMN resume_downloads TYPE INT USING (resume_downloads::int);
