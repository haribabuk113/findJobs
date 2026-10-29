-- +goose Up
-- Profile analytics (requirements doc "Profile Analytics"). Raw events are
-- kept 15 days: enough to recompute the 14 days the analytics compare.
-- Counts are non-unique by design; bots and the owner's own views are never
-- stored.
CREATE TABLE profile_events (
    user_id UUID NOT NULL REFERENCES profiles(user_id) ON DELETE CASCADE,
    kind    TEXT NOT NULL CHECK (kind IN ('view', 'resume_view', 'resume_download')),
    source  TEXT NOT NULL CHECK (source IN ('search', 'direct')),
    at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_profile_events_at ON profile_events (at);

-- Daily totals per profile and IST day, rebuilt from profile_events by the
-- daily rollup ("updated once per day"). The analytics endpoint reads only this.
CREATE TABLE profile_daily_stats (
    user_id          UUID NOT NULL REFERENCES profiles(user_id) ON DELETE CASCADE,
    day              DATE NOT NULL,
    views_search     INT NOT NULL DEFAULT 0,
    views_direct     INT NOT NULL DEFAULT 0,
    resume_views     INT NOT NULL DEFAULT 0,
    resume_downloads INT NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, day)
);

-- +goose Down
DROP TABLE profile_daily_stats;
DROP TABLE profile_events;
