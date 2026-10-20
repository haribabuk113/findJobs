-- +goose Up
-- Reference data (manual: "Skills and Designations", "Locations", "Salary").

-- Global taxonomies. Custom terms go live immediately as 'pending' for site
-- admin review; a merged term stays as an alias (merged_into -> master) so the
-- same text typed again resolves to the master.
CREATE TABLE IF NOT EXISTS skills (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    name_norm TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('verified', 'pending', 'merged')),
    merged_into BIGINT REFERENCES skills(id) ON DELETE CASCADE,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK ((status = 'merged') = (merged_into IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS idx_skills_status ON skills (status);

CREATE TABLE IF NOT EXISTS designations (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    name_norm TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('verified', 'pending', 'merged')),
    merged_into BIGINT REFERENCES designations(id) ON DELETE CASCADE,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK ((status = 'merged') = (merged_into IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS idx_designations_status ON designations (status);

-- Location tree from GeoNames (CC-BY 4.0): country -> region (admin1) -> city.
-- ids are GeoNames ids as text, matching jobs.location_ids.
CREATE TABLE IF NOT EXISTS locations (
    id TEXT PRIMARY KEY,
    parent_id TEXT REFERENCES locations(id) ON DELETE CASCADE,
    level TEXT NOT NULL CHECK (level IN ('country', 'region', 'city')),
    name TEXT NOT NULL,
    country_code CHAR(2) NOT NULL,
    lat DOUBLE PRECISION,
    lng DOUBLE PRECISION
);
CREATE INDEX IF NOT EXISTS idx_locations_parent ON locations (parent_id);

-- INR per one unit of each currency (manual "Salary": INR is the baseline).
CREATE TABLE IF NOT EXISTS exchange_rates (
    currency CHAR(3) PRIMARY KEY CHECK (currency ~ '^[A-Z]{3}$'),
    rate_to_inr NUMERIC(24, 10) NOT NULL CHECK (rate_to_inr > 0),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Generated static JSON exports (skills-<UTC>.json, ...), served from
-- /api/v1/assets with immutable caching so Cloudflare serves them. Kept in
-- Postgres because they are small, regenerated artefacts: the app stays
-- stateless with no shared disk. Only the latest few per kind are kept.
CREATE TABLE IF NOT EXISTS published_assets (
    file TEXT PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('skills', 'designations', 'locations', 'rates')),
    content BYTEA NOT NULL,
    sha256 TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_published_assets_kind ON published_assets (kind, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS published_assets;
DROP TABLE IF EXISTS exchange_rates;
DROP TABLE IF EXISTS locations;
DROP TABLE IF EXISTS designations;
DROP TABLE IF EXISTS skills;
