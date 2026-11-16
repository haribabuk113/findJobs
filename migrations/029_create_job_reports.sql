-- +goose Up
-- Job reports / moderation (admin-portal-design.md §4.5). A candidate or
-- submitter flags a job; an admin resolves it (approve keeps the job visible,
-- reject hides it).
CREATE TABLE job_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    reporter_id UUID REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'acknowledged', 'resolved')),
    moderation_note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);
CREATE INDEX idx_job_reports_status ON job_reports (status, created_at DESC);
CREATE INDEX idx_job_reports_job ON job_reports (job_id);

-- +goose Down
DROP TABLE IF EXISTS job_reports;