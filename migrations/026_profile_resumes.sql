-- +goose Up
-- The candidate's résumé (PDF or DOCX) in private storage. Never served by
-- /files/img; streamed only through the gated résumé routes.
ALTER TABLE profiles ADD COLUMN resume_key TEXT, ADD COLUMN resume_uploaded_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE profiles DROP COLUMN resume_key, DROP COLUMN resume_uploaded_at;
