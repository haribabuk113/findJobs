-- +goose Up
-- Match-score sort unlocks (manual "Search jobs": sorting by match is a paid
-- unlock, locked again when the search criteria change). One row per user and
-- set of filters, so unlocking the same filters again charges nothing.
CREATE TABLE search_unlocks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    criteria_hash TEXT NOT NULL,
    fee_paise BIGINT NOT NULL CHECK (fee_paise >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, criteria_hash)
);

-- +goose Down
DROP TABLE search_unlocks;
