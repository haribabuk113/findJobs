-- +goose Up
-- Company product cards get an optional image (manual "Section E": ≤1 MB).
ALTER TABLE company_products ADD COLUMN image_key TEXT;

-- Title search matches word stems too ("Product Manager" finds "Director of
-- Product Management"; newer doc §4). English stemming, title only.
CREATE INDEX idx_jobs_title_fts ON jobs USING GIN (to_tsvector('english', title));

-- Company search by city: HQ or any branch.
CREATE INDEX idx_companies_branch_locations ON companies USING GIN (branch_location_ids);

-- +goose Down
DROP INDEX idx_companies_branch_locations;
DROP INDEX idx_jobs_title_fts;
ALTER TABLE company_products DROP COLUMN image_key;
