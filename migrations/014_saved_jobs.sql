-- +goose Up
-- Saved jobs (manual: "Saved Jobs Page"), newest first by saved_at. Removing
-- a job removes its saves.
CREATE TABLE saved_jobs (
    user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_id   VARCHAR(150) NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    saved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, job_id)
);
CREATE INDEX idx_saved_jobs_user ON saved_jobs (user_id, saved_at DESC);

-- +goose Down
DROP TABLE saved_jobs;
