-- +goose Up
-- Jobs posted by recruiters (manual: "Post a job", "View jobs").
-- Free jobs have no company (unlisted, share link only); unlinked paid jobs
-- show company_name_text instead.
ALTER TABLE jobs ALTER COLUMN company_id DROP NOT NULL;
ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS recruiter_id UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS company_name_text TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'closed')),
    -- Random token for the direct link (/jobs/share/<hash>): the only way to
    -- reach a free job. Filled by the app (crypto/rand) for new jobs.
    ADD COLUMN IF NOT EXISTS share_hash TEXT,
    ADD COLUMN IF NOT EXISTS external_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS description_md TEXT NOT NULL DEFAULT '',
    -- interview_rounds is derived: the count of non-empty stages.
    ADD COLUMN IF NOT EXISTS interview_stages TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS work_modes TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS work_types TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS skill_ids BIGINT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS designation_ref BIGINT REFERENCES designations(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS outbound_clicks INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW();

-- Backfill existing (sample) jobs: multi-select arrays, taxonomy ids, share links.
UPDATE jobs SET work_modes = ARRAY[work_mode], work_types = ARRAY[work_type];
UPDATE jobs j SET designation_ref = d.id FROM designations d WHERE d.name_norm = lower(j.title);
UPDATE jobs j SET skill_ids = ARRAY(
    SELECT s.id FROM unnest(j.skills) WITH ORDINALITY AS n(name, ord)
    JOIN skills s ON s.name_norm = lower(n.name) ORDER BY n.ord);
UPDATE jobs SET share_hash = md5(id || random()::text || clock_timestamp()::text) WHERE share_hash IS NULL;

ALTER TABLE jobs ALTER COLUMN share_hash SET NOT NULL;
ALTER TABLE jobs DROP COLUMN work_mode, DROP COLUMN work_type, DROP COLUMN designation_id, DROP COLUMN skills;
ALTER TABLE jobs RENAME COLUMN designation_ref TO designation_id;
ALTER TABLE jobs ADD CONSTRAINT jobs_work_modes_valid CHECK (work_modes <@ ARRAY['Remote', 'Hybrid', 'On-site']::text[]);

CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_share_hash ON jobs (share_hash);
CREATE INDEX IF NOT EXISTS idx_jobs_work_modes ON jobs USING GIN (work_modes);
CREATE INDEX IF NOT EXISTS idx_jobs_work_types ON jobs USING GIN (work_types);
CREATE INDEX IF NOT EXISTS idx_jobs_skill_ids ON jobs USING GIN (skill_ids);
CREATE INDEX IF NOT EXISTS idx_jobs_designation ON jobs (designation_id);
CREATE INDEX IF NOT EXISTS idx_jobs_recruiter ON jobs (recruiter_id, posted_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_company ON jobs (company_id, posted_at DESC);

-- Saved, unvalidated post-a-job forms (manual: "Save as Draft").
CREATE TABLE IF NOT EXISTS job_drafts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    form JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_job_drafts_user ON job_drafts (user_id, updated_at DESC);

-- +goose Down
DROP TABLE IF EXISTS job_drafts;
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS jobs_work_modes_valid;
ALTER TABLE jobs RENAME COLUMN designation_id TO designation_ref;
ALTER TABLE jobs
    ADD COLUMN work_mode VARCHAR(20) NOT NULL DEFAULT 'On-site',
    ADD COLUMN work_type VARCHAR(50) NOT NULL DEFAULT 'Full-time',
    ADD COLUMN designation_id VARCHAR(150) NOT NULL DEFAULT '',
    ADD COLUMN skills TEXT[] NOT NULL DEFAULT '{}';
UPDATE jobs SET work_mode = COALESCE(work_modes[1], 'On-site'), work_type = COALESCE(work_types[1], 'Full-time'),
    skills = ARRAY(SELECT s.name FROM skills s WHERE s.id = ANY(skill_ids));
ALTER TABLE jobs
    DROP COLUMN recruiter_id, DROP COLUMN company_name_text, DROP COLUMN status, DROP COLUMN share_hash,
    DROP COLUMN external_url, DROP COLUMN description_md, DROP COLUMN interview_stages, DROP COLUMN work_modes,
    DROP COLUMN work_types, DROP COLUMN skill_ids, DROP COLUMN designation_ref, DROP COLUMN outbound_clicks,
    DROP COLUMN updated_at;
