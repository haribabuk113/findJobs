-- +goose Up
-- Applications (manual "Apply for jobs"). Easy Apply stores a dossier: a
-- privacy-filtered snapshot of the profile taken at apply time, so later
-- profile edits don't change what the recruiter received. External
-- applications record the candidate's "Yes, I applied".
ALTER TABLE applications
    ADD COLUMN id UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    ADD COLUMN apply_type TEXT NOT NULL DEFAULT 'platform' CHECK (apply_type IN ('platform', 'external')),
    ADD COLUMN matched_skill_ids BIGINT[] NOT NULL DEFAULT '{}',
    ADD COLUMN unmatched_skill_ids BIGINT[] NOT NULL DEFAULT '{}',
    -- Manual: recruiter actions are Shortlisted, Contacted, Rejected.
    ADD COLUMN stage TEXT NOT NULL DEFAULT 'new' CHECK (stage IN ('new', 'shortlisted', 'contacted', 'rejected')),
    ADD COLUMN stage_updated_at TIMESTAMPTZ,
    ADD COLUMN paid BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN dossier JSONB NOT NULL DEFAULT '{}';
CREATE INDEX idx_applications_job ON applications (job_id, submitted_at DESC);

-- Free Easy Apply applications used per candidate per IST day.
CREATE TABLE apply_quota (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    day_ist DATE NOT NULL,
    used    INT NOT NULL DEFAULT 0 CHECK (used >= 0),
    PRIMARY KEY (user_id, day_ist)
);

-- +goose Down
DROP TABLE apply_quota;
DROP INDEX idx_applications_job;
ALTER TABLE applications
    DROP COLUMN dossier, DROP COLUMN paid, DROP COLUMN stage_updated_at, DROP COLUMN stage,
    DROP COLUMN unmatched_skill_ids, DROP COLUMN matched_skill_ids, DROP COLUMN apply_type, DROP COLUMN id;
