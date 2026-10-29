-- +goose Up
-- Profile views are counted in Redis and flushed here in batches, so one
-- row can now stand for several events of the same kind on the same day.
ALTER TABLE profile_events ADD COLUMN count INT NOT NULL DEFAULT 1 CHECK (count > 0);

-- Flushed Redis batches, so a batch retried after a crash is applied once.
CREATE TABLE profile_event_batches (
    id         TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE profile_event_batches;
ALTER TABLE profile_events DROP COLUMN count;
