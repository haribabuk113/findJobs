-- +goose Up
-- Login sessions moved to Redis (adapters/outbound/redis/session_store.go).
-- Anyone signed in when this runs signs in again; the IAM sessions behind the
-- dropped rows expire on their own (30-day absolute life).
DROP TABLE IF EXISTS sessions;

-- +goose Down
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    refresh_token TEXT NOT NULL DEFAULT '',
    access_token TEXT NOT NULL DEFAULT '',
    access_expires_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
