-- +goose Up
-- Candidate profiles, one per user. Skills, target roles, locations and
-- companies point at the shared reference tables (skills, designations,
-- locations, companies), never at copies of them.
CREATE TABLE profiles (
    user_id            UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    slug               TEXT NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9-]{3,50}$'),
    name               TEXT NOT NULL,
    headline           TEXT NOT NULL DEFAULT '',
    location_label     TEXT NOT NULL DEFAULT '',
    website            TEXT NOT NULL DEFAULT '',        -- portfolio display name
    about              TEXT NOT NULL DEFAULT '',
    about_updated_at   TIMESTAMPTZ,
    photo_key          TEXT,
    gender             TEXT NOT NULL DEFAULT '',
    pronouns           TEXT NOT NULL DEFAULT '',
    contact_email      TEXT NOT NULL DEFAULT '',
    phone              TEXT NOT NULL DEFAULT '',
    experience_years   INTEGER NOT NULL DEFAULT 0 CHECK (experience_years BETWEEN 0 AND 99),
    notice_period      TEXT NOT NULL DEFAULT '',
    serving_notice     BOOLEAN NOT NULL DEFAULT FALSE,
    last_working_date  DATE,
    salary_currency    CHAR(3) NOT NULL DEFAULT 'INR',
    current_salary     NUMERIC(16,2) NOT NULL DEFAULT 0 CHECK (current_salary >= 0),
    expected_salary    NUMERIC(16,2) NOT NULL DEFAULT 0 CHECK (expected_salary >= 0),
    -- INR baseline of expected_salary, for recruiter-side filtering.
    expected_salary_inr NUMERIC(18,2),
    hourly_rate        NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (hourly_rate >= 0),
    work_types         TEXT[] NOT NULL DEFAULT '{}',
    location_ids       TEXT[] NOT NULL DEFAULT '{}',   -- preferred cities (GeoNames ids)
    designation_ids    BIGINT[] NOT NULL DEFAULT '{}', -- target roles, in the candidate's order
    open_to_remote     BOOLEAN NOT NULL DEFAULT FALSE,
    work_mode          TEXT NOT NULL DEFAULT '',
    relocation         TEXT NOT NULL DEFAULT '',
    -- Privacy (what public visitors see).
    is_public          BOOLEAN NOT NULL DEFAULT TRUE,
    open_to_work       BOOLEAN NOT NULL DEFAULT TRUE,
    hide_current_salary BOOLEAN NOT NULL DEFAULT TRUE,
    hide_hourly_rate   BOOLEAN NOT NULL DEFAULT TRUE,
    hide_contact       BOOLEAN NOT NULL DEFAULT TRUE,
    hide_expected_salary BOOLEAN NOT NULL DEFAULT TRUE,
    resume_applications_only BOOLEAN NOT NULL DEFAULT TRUE,
    -- Small owner-edited lists validated in Go: social_links, languages,
    -- education, authorizations.
    details            JSONB NOT NULL DEFAULT '{}',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_profiles_designation_ids ON profiles USING GIN (designation_ids);
CREATE INDEX idx_profiles_location_ids ON profiles USING GIN (location_ids);

-- Skills in display order; each skill sits in one named group.
CREATE TABLE profile_skills (
    user_id    UUID NOT NULL REFERENCES profiles(user_id) ON DELETE CASCADE,
    skill_id   BIGINT NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    group_name TEXT NOT NULL,
    position   INTEGER NOT NULL,
    PRIMARY KEY (user_id, skill_id)
);
CREATE INDEX idx_profile_skills_skill ON profile_skills (skill_id);

-- Work history. company_id links a registered company (matched by name);
-- verification comes from company_members, so it is never stored here.
CREATE TABLE profile_experiences (
    user_id      UUID NOT NULL REFERENCES profiles(user_id) ON DELETE CASCADE,
    position     INTEGER NOT NULL,
    company_id   VARCHAR(100) REFERENCES companies(id) ON DELETE SET NULL,
    company_name TEXT NOT NULL,
    title        TEXT NOT NULL,
    work_type    TEXT NOT NULL,
    start_month  TEXT NOT NULL CHECK (start_month ~ '^\d{4}-\d{2}$'),
    end_month    TEXT CHECK (end_month ~ '^\d{4}-\d{2}$'),
    is_current   BOOLEAN NOT NULL DEFAULT FALSE,
    promoted     BOOLEAN NOT NULL DEFAULT FALSE,
    description  TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (user_id, position)
);
CREATE INDEX idx_profile_experiences_company ON profile_experiences (company_id);

-- +goose Down
DROP TABLE profile_experiences;
DROP TABLE profile_skills;
DROP TABLE profiles;
