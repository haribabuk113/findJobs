-- +goose Up
-- Candidate premium subscriptions (admin-portal-design.md §4.4). Created and
-- extended by platform admins at the configurable daily rate; a daily cron
-- expires them and reverts the candidate tier.
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    duration_days INT NOT NULL CHECK (duration_days > 0),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'expired')),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_subscriptions_user ON subscriptions (user_id, status);
CREATE INDEX idx_subscriptions_expiry ON subscriptions (status, expires_at);

-- +goose Down
DROP TABLE IF EXISTS subscriptions;