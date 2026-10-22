-- +goose Up
-- IAM tokens are bearer credentials: store them AES-256-GCM encrypted by the
-- app (TOKEN_ENC_KEYS), bound to their session id, so database readers and
-- backups can't use them. Plaintext rows can't be converted in SQL, so
-- existing sessions end and those users sign in again.
DELETE FROM sessions;
ALTER TABLE sessions
    DROP COLUMN refresh_token,
    DROP COLUMN access_token,
    ADD COLUMN refresh_token_enc BYTEA NOT NULL,
    ADD COLUMN access_token_enc BYTEA NOT NULL,
    ADD COLUMN key_id TEXT NOT NULL,
    -- Idle timeout (SESSION_IDLE_TIMEOUT); written at most every 15 minutes.
    ADD COLUMN last_seen_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW();

-- +goose Down
DELETE FROM sessions;
ALTER TABLE sessions
    DROP COLUMN refresh_token_enc,
    DROP COLUMN access_token_enc,
    DROP COLUMN key_id,
    DROP COLUMN last_seen_at,
    ADD COLUMN refresh_token TEXT NOT NULL,
    ADD COLUMN access_token TEXT NOT NULL;
