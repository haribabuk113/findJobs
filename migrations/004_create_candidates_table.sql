CREATE TABLE IF NOT EXISTS candidates (
    candidate_id         BIGSERIAL PRIMARY KEY,
    iam_user_id          VARCHAR(255) NOT NULL UNIQUE,
    full_name            VARCHAR(100) NOT NULL CHECK (char_length(full_name) >= 2),
    profile_slug         VARCHAR(50) NOT NULL UNIQUE CHECK (profile_slug ~ '^[a-z0-9-]+$'),
    profile_photo_url    TEXT NULL,                                    -- NULL => frontend renders initials avatar
    gender               VARCHAR(20) NULL CHECK (gender IN ('MALE','FEMALE','NON_BINARY','PREFER_NOT_TO_SAY')),
    headline_text        VARCHAR(200) NULL,                            -- free-typed headline
    headline_designation_id BIGINT NULL REFERENCES master_designations(designation_id),
    about_summary        VARCHAR(2000) NULL,
    email                VARCHAR(255) NOT NULL,
    status               VARCHAR(50) NOT NULL DEFAULT 'active',
    about_summary_updated_at TIMESTAMPTZ NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE master_designations
    ADD CONSTRAINT fk_designation_creator
    FOREIGN KEY (created_by_candidate_id) REFERENCES candidates(candidate_id);

ALTER TABLE master_skills
    ADD CONSTRAINT fk_skill_creator
    FOREIGN KEY (created_by_candidate_id) REFERENCES candidates(candidate_id);



-- Pronouns: multi-select, so a child table rather than a column
CREATE TABLE IF NOT EXISTS candidate_pronouns (
    candidate_id         BIGINT NOT NULL REFERENCES candidates(candidate_id) ON DELETE CASCADE,
    pronoun              VARCHAR(30) NOT NULL,   -- 'HE_HIM', 'SHE_HER', 'THEY_THEM', or custom text
    PRIMARY KEY (candidate_id, pronoun)
);

-- Social handles (LinkedIn, GitHub, Behance, Dribbble, Portfolio, ...)
CREATE TABLE IF NOT EXISTS candidate_social_handles (
    id                   BIGSERIAL PRIMARY KEY,
    candidate_id         BIGINT NOT NULL REFERENCES candidates(candidate_id) ON DELETE CASCADE,
    platform             VARCHAR(50) NOT NULL,   -- 'LINKEDIN','GITHUB','BEHANCE','DRIBBBLE','PORTFOLIO','OTHER'
    url                  TEXT NOT NULL,
    click_count          BIGINT NOT NULL DEFAULT 0,   -- outgoing click telemetry
    UNIQUE (candidate_id, platform)
);

CREATE TABLE IF NOT EXISTS candidate_salary (
    candidate_id         BIGINT PRIMARY KEY REFERENCES candidates(candidate_id) ON DELETE CASCADE,
    -- current base salary (privacy-configurable)
    current_amount       NUMERIC(18,2) NULL,
    current_currency     CHAR(3) NULL,
    current_amount_inr   NUMERIC(18,2) NULL,        -- normalized, always computed even if hidden
    current_is_hidden    BOOLEAN NOT NULL DEFAULT FALSE,
    -- target / expected salary (always public, no hide option)
    expected_amount      NUMERIC(18,2) NULL,
    expected_currency    CHAR(3) NULL,
    expected_amount_inr  NUMERIC(18,2) NULL,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS candidate_target_roles (
    candidate_id         BIGINT NOT NULL REFERENCES candidates(candidate_id) ON DELETE CASCADE,
    designation_id       BIGINT NOT NULL REFERENCES master_designations(designation_id),
    PRIMARY KEY (candidate_id, designation_id)
);

CREATE TABLE IF NOT EXISTS candidate_target_locations (
    candidate_id         BIGINT NOT NULL REFERENCES candidates(candidate_id) ON DELETE CASCADE,
    location_id          BIGINT NOT NULL REFERENCES master_locations(location_id),
    PRIMARY KEY (candidate_id, location_id)
);

CREATE TABLE IF NOT EXISTS candidate_preferences (
    candidate_id         BIGINT PRIMARY KEY REFERENCES candidates(candidate_id) ON DELETE CASCADE,
    open_to_remote       BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS candidate_work_types (
    candidate_id         BIGINT NOT NULL REFERENCES candidates(candidate_id) ON DELETE CASCADE,
    work_type            VARCHAR(20) NOT NULL CHECK (work_type IN
                            ('FULL_TIME','CONTRACT','PART_TIME','GIG','ADVISORY','VOLUNTEERING')),
    PRIMARY KEY (candidate_id, work_type)
);

CREATE TABLE IF NOT EXISTS candidate_timeline (
    timeline_id          BIGSERIAL PRIMARY KEY,
    candidate_id         BIGINT NOT NULL REFERENCES candidates(candidate_id) ON DELETE CASCADE,
    company_id           BIGINT NULL REFERENCES master_companies(company_id),  -- NULL if unregistered co.
    company_name_raw     VARCHAR(200) NULL,   -- freeform text if company not registered/linked
    designation_title    VARCHAR(150) NOT NULL,
    start_date           DATE NOT NULL,
    end_date             DATE NULL,
    is_current            BOOLEAN NOT NULL DEFAULT FALSE,
    description          TEXT NULL,
    verification_status  VARCHAR(20) NOT NULL DEFAULT 'UNVERIFIED'
                            CHECK (verification_status IN
                              ('UNVERIFIED','PENDING_VERIFICATION','VERIFIED','REJECTED')),
    verified_at           TIMESTAMPTZ NULL,
    verified_by_user_id   BIGINT NULL,        -- Company Admin user id
    rejected_at           TIMESTAMPTZ NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);


CREATE TABLE IF NOT EXISTS candidate_skill_groups (
    group_id             BIGSERIAL PRIMARY KEY,
    candidate_id         BIGINT NOT NULL REFERENCES candidates(candidate_id) ON DELETE CASCADE,
    group_name           VARCHAR(100) NOT NULL,
    display_order        INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS candidate_skill_group_items (
    id                   BIGSERIAL PRIMARY KEY,
    group_id             BIGINT NOT NULL REFERENCES candidate_skill_groups(group_id) ON DELETE CASCADE,
    skill_id             BIGINT NOT NULL REFERENCES master_skills(skill_id),
    display_order        INTEGER NOT NULL DEFAULT 0,   -- intra-group chip priority
    UNIQUE (group_id, skill_id)
);


CREATE TABLE IF NOT EXISTS candidate_contact (
    candidate_id         BIGINT PRIMARY KEY REFERENCES candidates(candidate_id) ON DELETE CASCADE,
    email                VARCHAR(255) NOT NULL,
    phone_number         VARCHAR(30) NULL,
    primary_location_id  BIGINT NULL REFERENCES master_locations(location_id)
);

CREATE TABLE IF NOT EXISTS candidate_resume (
    candidate_id         BIGINT PRIMARY KEY REFERENCES candidates(candidate_id) ON DELETE CASCADE,
    file_url             TEXT NOT NULL,
    file_size_bytes      BIGINT NOT NULL CHECK (file_size_bytes <= 5 * 1024 * 1024),
    access_control        VARCHAR(30) NOT NULL DEFAULT 'PUBLICLY_AVAILABLE'
                            CHECK (access_control IN ('PUBLICLY_AVAILABLE','APPLICATION_GRID_ONLY')),
    public_short_link     VARCHAR(50) NULL UNIQUE,   -- only relevant when PUBLICLY_AVAILABLE
    uploaded_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Grants created when a candidate applies to a job and the resume is
-- "Application Grid Only" -- this is what actually unlocks recruiter access.
CREATE TABLE IF NOT EXISTS resume_access_grants (
    id                    BIGSERIAL PRIMARY KEY,
    candidate_id          BIGINT NOT NULL REFERENCES candidates(candidate_id) ON DELETE CASCADE,
    recruiter_id          BIGINT NOT NULL,        -- FK to recruiters table (external to this doc's scope)
    job_posting_id        BIGINT NOT NULL,        -- FK to job_postings table (external to this doc's scope)
    granted_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (candidate_id, recruiter_id, job_posting_id)
);

CREATE INDEX idx_timeline_candidate ON candidate_timeline(candidate_id);
CREATE INDEX idx_timeline_company ON candidate_timeline(company_id);
CREATE INDEX idx_skill_groups_candidate ON candidate_skill_groups(candidate_id);
CREATE INDEX idx_skill_items_group ON candidate_skill_group_items(group_id);
CREATE INDEX idx_locations_parent ON master_locations(parent_location_id);
CREATE INDEX idx_resume_grants_candidate ON resume_access_grants(candidate_id);
