CREATE TABLE IF NOT EXISTS master_designations (
    designation_id      BIGSERIAL PRIMARY KEY,
    name                 VARCHAR(150) NOT NULL UNIQUE,
    is_admin_approved    BOOLEAN NOT NULL DEFAULT TRUE,   -- FALSE = pending review (custom entry)
    created_by_candidate_id BIGINT NULL,                  -- who first typed this custom value
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS master_skills (
    skill_id             BIGSERIAL PRIMARY KEY,
    name                 VARCHAR(150) NOT NULL UNIQUE,
    is_admin_approved    BOOLEAN NOT NULL DEFAULT TRUE,
    created_by_candidate_id BIGINT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS master_companies (
    company_id           BIGSERIAL PRIMARY KEY,
    name                 VARCHAR(200) NOT NULL,
    is_registered        BOOLEAN NOT NULL DEFAULT FALSE,  -- has an active Company Dashboard account
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Hierarchical location tree: Country -> State -> City
CREATE TABLE IF NOT EXISTS master_locations (
    location_id          BIGSERIAL PRIMARY KEY,
    parent_location_id   BIGINT NULL REFERENCES master_locations(location_id),
    level                VARCHAR(10) NOT NULL CHECK (level IN ('COUNTRY','STATE','CITY')),
    name                 VARCHAR(150) NOT NULL
);

CREATE TABLE IF NOT EXISTS fx_rates_snapshot (            -- static daily rates.json artifact, persisted
    rate_date            DATE NOT NULL,
    currency_code        CHAR(3) NOT NULL,
    inr_conversion_rate  NUMERIC(18,6) NOT NULL,
    PRIMARY KEY (rate_date, currency_code)
);
