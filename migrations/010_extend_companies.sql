-- +goose Up
-- Company module (manual: "Company page", "Company dashboard", "Company employees").
-- companies.id stays the stable key jobs reference; the public URL uses slug,
-- so a company can change its link without touching jobs.
ALTER TABLE companies
    ADD COLUMN IF NOT EXISTS slug TEXT,
    ADD COLUMN IF NOT EXISTS tagline TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS industries TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS about TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS website TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS social_links JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS founded_year INT CHECK (founded_year BETWEEN 1800 AND 2100),
    -- No FK to locations: the GeoNames import replaces that table wholesale.
    -- The app checks the ids exist on save.
    ADD COLUMN IF NOT EXISTS hq_location_id TEXT,
    ADD COLUMN IF NOT EXISTS hq_country_code CHAR(2),
    ADD COLUMN IF NOT EXISTS hq_address TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS branch_location_ids TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS logo_key TEXT,
    ADD COLUMN IF NOT EXISTS banner_key TEXT,
    ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;

UPDATE companies SET slug = id WHERE slug IS NULL;
UPDATE companies SET industries = ARRAY[sector] WHERE industries = '{}' AND sector <> '';
-- One spelling per size bracket (job search filters on this text).
UPDATE companies SET size = CASE size
    WHEN '501–1000 people' THEN '501–1,000 people'
    WHEN '1000+ people' THEN '1,001–5,000 people'
    ELSE size END;
ALTER TABLE companies ALTER COLUMN slug SET NOT NULL;
-- Unique across deleted companies too, so a deleted brand's link can't be reused.
CREATE UNIQUE INDEX IF NOT EXISTS idx_companies_slug ON companies (slug);
CREATE INDEX IF NOT EXISTS idx_companies_country ON companies (hq_country_code) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS company_products (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id VARCHAR(100) NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    position INT NOT NULL,
    name TEXT NOT NULL,
    about TEXT NOT NULL,
    url TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_company_products_company ON company_products (company_id, position);

CREATE TABLE IF NOT EXISTS company_benefits (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    company_id VARCHAR(100) NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    position INT NOT NULL,
    title TEXT NOT NULL,
    details TEXT NOT NULL DEFAULT '',
    icon TEXT NOT NULL CHECK (icon IN ('heart', 'laptop', 'coffee', 'spark', 'money'))
);
CREATE INDEX IF NOT EXISTS idx_company_benefits_company ON company_benefits (company_id, position);

-- Workplace linkage (manual "Company employees"): requests come from
-- candidates' profiles; admins approve, assign roles and offboard.
-- Spending caps arrive with the payments section.
CREATE TABLE IF NOT EXISTS company_members (
    company_id VARCHAR(100) NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'employee' CHECK (role IN ('admin', 'hr', 'employee')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'current', 'past', 'rejected')),
    title TEXT NOT NULL DEFAULT '',
    started_on DATE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (company_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_company_members_user ON company_members (user_id);
CREATE INDEX IF NOT EXISTS idx_company_members_status ON company_members (company_id, status);

-- +goose Down
DROP TABLE IF EXISTS company_members;
DROP TABLE IF EXISTS company_benefits;
DROP TABLE IF EXISTS company_products;
DROP INDEX IF EXISTS idx_companies_country;
DROP INDEX IF EXISTS idx_companies_slug;
ALTER TABLE companies
    DROP COLUMN IF EXISTS slug, DROP COLUMN IF EXISTS tagline, DROP COLUMN IF EXISTS industries,
    DROP COLUMN IF EXISTS about, DROP COLUMN IF EXISTS website, DROP COLUMN IF EXISTS social_links,
    DROP COLUMN IF EXISTS founded_year, DROP COLUMN IF EXISTS hq_location_id, DROP COLUMN IF EXISTS hq_country_code,
    DROP COLUMN IF EXISTS hq_address, DROP COLUMN IF EXISTS branch_location_ids, DROP COLUMN IF EXISTS logo_key,
    DROP COLUMN IF EXISTS banner_key, DROP COLUMN IF EXISTS created_by, DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS deleted_at;
