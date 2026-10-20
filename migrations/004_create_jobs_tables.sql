-- +goose Up
-- Job search (manual: "Search jobs", "Filters", "Salary", "Locations").
-- Search runs in Postgres behind ports.JobRepository; the manual's Meilisearch
-- hard-filter engine can replace that adapter once volume justifies it
-- (system requirements §30: no search cluster before it is measurably needed).

CREATE TABLE IF NOT EXISTS companies (
    id VARCHAR(100) PRIMARY KEY,           -- public slug, e.g. "forma"
    name VARCHAR(255) NOT NULL,
    sector VARCHAR(255) NOT NULL DEFAULT '',
    size VARCHAR(50) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS jobs (
    id VARCHAR(150) PRIMARY KEY,           -- public job id used in /jobs/[id]
    company_id VARCHAR(100) NOT NULL REFERENCES companies(id),
    title VARCHAR(255) NOT NULL,
    designation_id VARCHAR(150) NOT NULL,  -- master designation id (client IndexedDB taxonomy)
    category VARCHAR(100) NOT NULL DEFAULT '',
    level VARCHAR(50) NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',

    work_mode VARCHAR(20) NOT NULL CHECK (work_mode IN ('Remote', 'Hybrid', 'On-site')),
    work_type VARCHAR(50) NOT NULL,
    location_type VARCHAR(20) NOT NULL DEFAULT 'specific' CHECK (location_type IN ('specific', 'world_remote')),
    location_label VARCHAR(255) NOT NULL DEFAULT '',
    location_ids TEXT[] NOT NULL DEFAULT '{}',   -- leaf city ids from the location tree

    -- Dual storage: original amount/currency plus INR baseline for filtering/sorting.
    salary_min NUMERIC(14, 2),
    salary_max NUMERIC(14, 2),
    currency CHAR(3) NOT NULL DEFAULT 'INR',
    salary_min_inr NUMERIC(16, 2),
    normalized_inr_amount NUMERIC(16, 2),        -- upper bound in INR; recalculated by the daily rates cron
    hide_salary BOOLEAN NOT NULL DEFAULT FALSE,

    skills TEXT[] NOT NULL DEFAULT '{}',
    notice_periods TEXT[] NOT NULL DEFAULT '{}',
    benefits TEXT[] NOT NULL DEFAULT '{}',
    relocation VARCHAR(50) NOT NULL DEFAULT '',
    experience_years INT NOT NULL DEFAULT 0 CHECK (experience_years >= 0),
    interview_rounds INT NOT NULL DEFAULT 0 CHECK (interview_rounds >= 0),

    tier VARCHAR(20) NOT NULL CHECK (tier IN ('Free', 'Premium')),          -- Free = unlisted, never in public search
    application VARCHAR(20) NOT NULL CHECK (application IN ('Easy apply', 'External', 'Email')),

    views INT NOT NULL DEFAULT 0,
    recruiter_actions INT NOT NULL DEFAULT 0,
    is_visible BOOLEAN NOT NULL DEFAULT TRUE,    -- daily cron hides listings older than 30 days
    posted_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,  -- reset on repost
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CHECK (salary_min IS NULL OR salary_max IS NULL OR salary_min <= salary_max)
);

-- Public search always filters on visibility + freshness and sorts by posted_at.
CREATE INDEX IF NOT EXISTS idx_jobs_public_posted ON jobs (posted_at DESC) WHERE is_visible AND tier <> 'Free';
CREATE INDEX IF NOT EXISTS idx_jobs_salary ON jobs (normalized_inr_amount);
CREATE INDEX IF NOT EXISTS idx_jobs_location_ids ON jobs USING GIN (location_ids);
CREATE INDEX IF NOT EXISTS idx_jobs_skills ON jobs USING GIN (skills);
CREATE INDEX IF NOT EXISTS idx_jobs_notice_periods ON jobs USING GIN (notice_periods);
CREATE INDEX IF NOT EXISTS idx_jobs_designation ON jobs (designation_id);

-- Read by search for the "never show jobs I already applied to" rule.
-- The apply flow (credits, match score) will write here.
CREATE TABLE IF NOT EXISTS applications (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    job_id VARCHAR(150) NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    calculated_match_score_pct INT CHECK (calculated_match_score_pct BETWEEN 0 AND 100),
    submitted_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, job_id)
);
