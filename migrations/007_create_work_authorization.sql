-- The response has a "work_authorization" block but no table exists for it in 003/004.
CREATE TABLE IF NOT EXISTS candidate_work_authorization (
    id                   BIGSERIAL PRIMARY KEY,
    candidate_id         BIGINT NOT NULL REFERENCES candidates(candidate_id) ON DELETE CASCADE,
    country              VARCHAR(100) NOT NULL,
    authorization_type   VARCHAR(100) NOT NULL,      -- e.g. 'H-1B Holder', 'Citizen'
    valid_from           DATE NULL,
    valid_until          DATE NULL,
    is_indefinite        BOOLEAN NOT NULL DEFAULT FALSE,
    CHECK (is_indefinite = FALSE OR valid_until IS NULL),
    CHECK (valid_from IS NULL OR valid_until IS NULL OR valid_until >= valid_from)
);

CREATE INDEX IF NOT EXISTS idx_work_auth_candidate ON candidate_work_authorization(candidate_id);
