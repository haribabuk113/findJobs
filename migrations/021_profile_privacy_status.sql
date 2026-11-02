-- +goose Up
-- Profile rules from the newer requirements doc ("Profile page"):
--  * Privacy: each restricted field is Visible to All, Applied Jobs Only, or
--    (current salary only) Hidden from All, replacing the hide_* booleans.
--  * Salaries carry a compensation period; availability, a notice period as
--    a number + unit, and a travel preference are new.
--  * Blue ticks are per experience entry: editing a verified entry sends it
--    back for review without touching the person's company membership.
ALTER TABLE profiles
    ADD COLUMN current_salary_visibility TEXT NOT NULL DEFAULT 'hidden'
        CHECK (current_salary_visibility IN ('all', 'applied', 'hidden')),
    ADD COLUMN expected_salary_visibility TEXT NOT NULL DEFAULT 'applied'
        CHECK (expected_salary_visibility IN ('all', 'applied')),
    ADD COLUMN hourly_rate_visibility TEXT NOT NULL DEFAULT 'applied'
        CHECK (hourly_rate_visibility IN ('all', 'applied')),
    ADD COLUMN resume_visibility TEXT NOT NULL DEFAULT 'applied'
        CHECK (resume_visibility IN ('all', 'applied')),
    ADD COLUMN contact_visibility TEXT NOT NULL DEFAULT 'applied'
        CHECK (contact_visibility IN ('all', 'applied')),
    ADD COLUMN current_salary_period TEXT NOT NULL DEFAULT 'year'
        CHECK (current_salary_period IN ('year', 'month', 'hour')),
    ADD COLUMN expected_salary_period TEXT NOT NULL DEFAULT 'year'
        CHECK (expected_salary_period IN ('year', 'month', 'hour')),
    ADD COLUMN availability TEXT NOT NULL DEFAULT '',
    ADD COLUMN available_from DATE,
    ADD COLUMN notice_value INTEGER CHECK (notice_value BETWEEN 0 AND 365),
    ADD COLUMN notice_unit TEXT NOT NULL DEFAULT '' CHECK (notice_unit IN ('', 'days', 'weeks', 'months')),
    ADD COLUMN travel TEXT NOT NULL DEFAULT '';

-- What visitors and recruiters saw before stays the same: a hidden current
-- salary never reached recruiters; the other flags only hid fields publicly.
UPDATE profiles SET
    current_salary_visibility = CASE WHEN hide_current_salary THEN 'hidden' ELSE 'all' END,
    expected_salary_visibility = CASE WHEN hide_expected_salary THEN 'applied' ELSE 'all' END,
    hourly_rate_visibility = CASE WHEN hide_hourly_rate THEN 'applied' ELSE 'all' END,
    resume_visibility = CASE WHEN resume_applications_only THEN 'applied' ELSE 'all' END,
    contact_visibility = CASE WHEN hide_contact THEN 'applied' ELSE 'all' END,
    availability = CASE WHEN serving_notice THEN 'Serving notice' ELSE '' END,
    notice_value = CASE notice_period WHEN 'Immediate' THEN 0 WHEN '15 days' THEN 15 WHEN '30 days' THEN 30
        WHEN '45 days' THEN 45 WHEN '60 days' THEN 60 WHEN '90 days' THEN 90 END,
    notice_unit = CASE WHEN notice_period IN ('Immediate', '15 days', '30 days', '45 days', '60 days', '90 days')
        THEN 'days' ELSE '' END;

ALTER TABLE profiles
    DROP COLUMN hide_current_salary, DROP COLUMN hide_expected_salary, DROP COLUMN hide_hourly_rate,
    DROP COLUMN resume_applications_only, DROP COLUMN hide_contact, DROP COLUMN serving_notice;

ALTER TABLE profile_experiences
    ADD COLUMN verification TEXT NOT NULL DEFAULT 'unverified'
        CHECK (verification IN ('unverified', 'pending', 'verified', 'rejected'));

-- Until now an entry's tick was its company membership's status.
UPDATE profile_experiences e SET verification = CASE m.status
        WHEN 'current' THEN 'verified' WHEN 'past' THEN 'verified'
        WHEN 'pending' THEN 'pending' WHEN 'rejected' THEN 'rejected' END
FROM company_members m
WHERE m.company_id = e.company_id AND m.user_id = e.user_id;

-- +goose Down
ALTER TABLE profile_experiences DROP COLUMN verification;

ALTER TABLE profiles
    ADD COLUMN hide_current_salary BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN hide_expected_salary BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN hide_hourly_rate BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN resume_applications_only BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN hide_contact BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN serving_notice BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE profiles SET
    hide_current_salary = current_salary_visibility <> 'all',
    hide_expected_salary = expected_salary_visibility <> 'all',
    hide_hourly_rate = hourly_rate_visibility <> 'all',
    resume_applications_only = resume_visibility <> 'all',
    hide_contact = contact_visibility <> 'all',
    serving_notice = availability = 'Serving notice';

ALTER TABLE profiles
    DROP COLUMN current_salary_visibility, DROP COLUMN expected_salary_visibility, DROP COLUMN hourly_rate_visibility,
    DROP COLUMN resume_visibility, DROP COLUMN contact_visibility, DROP COLUMN current_salary_period,
    DROP COLUMN expected_salary_period, DROP COLUMN availability, DROP COLUMN available_from,
    DROP COLUMN notice_value, DROP COLUMN notice_unit, DROP COLUMN travel;
