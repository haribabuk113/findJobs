-- +goose Up
-- Sessions back in Postgres (reverts 006). Postgres commits each refresh-token
-- rotation durably; Redis AOF (everysec) could lose one on a crash, and IAM
-- revokes a session when it sees a used refresh token. Redis stays available
-- for future non-critical uses (shared rate limiting, caches).
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    -- IAM tokens are bearer credentials: server-side only, never sent to the browser.
    refresh_token TEXT NOT NULL,
    access_token TEXT NOT NULL,
    access_expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

-- +goose Down
DROP TABLE IF EXISTS sessions;
