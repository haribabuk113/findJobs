-- +goose Up
-- Profile shape from the newer requirements doc ("Profile page" §3, §6-§8):
--  * Current status: current company, work location (a location id; the
--    label is filled from the tree), work arrangement and work type.
--  * Career timeline: one employment record per company holding its roles
--    (designation, dates, promoted, responsibilities, achievements). The blue
--    tick stays on the employment record.
--  * Languages: Read / Write / Speak instead of a proficiency.
ALTER TABLE profiles
    ADD COLUMN current_company TEXT NOT NULL DEFAULT '',
    ADD COLUMN current_location_id TEXT,
    ADD COLUMN current_work_arrangement TEXT NOT NULL DEFAULT '',
    ADD COLUMN current_work_type TEXT NOT NULL DEFAULT '';

ALTER TABLE profile_experiences
    ADD COLUMN location_id TEXT,
    ADD COLUMN work_arrangement TEXT NOT NULL DEFAULT '',
    ADD COLUMN roles JSONB NOT NULL DEFAULT '[]';

-- Each flat entry becomes an employment record with one role.
UPDATE profile_experiences SET roles = jsonb_build_array(jsonb_build_object(
    'title', title, 'start', start_month, 'end', COALESCE(end_month, ''), 'promoted', promoted,
    'responsibilities', description, 'achievements', '[]'::jsonb));

ALTER TABLE profile_experiences DROP COLUMN title, DROP COLUMN promoted, DROP COLUMN description;

-- A language someone listed is one they know: read, write and speak.
UPDATE profiles SET details = jsonb_set(details, '{languages}', (
    SELECT COALESCE(jsonb_agg(jsonb_build_object('name', l->>'name', 'read', true, 'write', true, 'speak', true)), '[]'::jsonb)
    FROM jsonb_array_elements(details->'languages') l))
WHERE jsonb_typeof(details->'languages') = 'array';

-- +goose Down
UPDATE profiles SET details = jsonb_set(details, '{languages}', (
    SELECT COALESCE(jsonb_agg(jsonb_build_object('name', l->>'name', 'proficiency', '')), '[]'::jsonb)
    FROM jsonb_array_elements(details->'languages') l))
WHERE jsonb_typeof(details->'languages') = 'array';

ALTER TABLE profile_experiences
    ADD COLUMN title TEXT NOT NULL DEFAULT '',
    ADD COLUMN promoted BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN description TEXT NOT NULL DEFAULT '';

-- Keep the latest role of each record (the others are lost going down).
UPDATE profile_experiences e SET (title, promoted, description) = (
    SELECT COALESCE(r->>'title', ''), COALESCE((r->>'promoted')::boolean, FALSE), COALESCE(r->>'responsibilities', '')
    FROM jsonb_array_elements(e.roles) r ORDER BY r->>'start' DESC LIMIT 1)
WHERE jsonb_array_length(e.roles) > 0;

ALTER TABLE profile_experiences DROP COLUMN roles, DROP COLUMN work_arrangement, DROP COLUMN location_id;
ALTER TABLE profiles
    DROP COLUMN current_work_type, DROP COLUMN current_work_arrangement,
    DROP COLUMN current_location_id, DROP COLUMN current_company;
