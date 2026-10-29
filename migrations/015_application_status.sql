-- +goose Up
-- Whether an application is still live. Deleting a company cancels the open
-- applications to its jobs (manual "Company page": deleting the company
-- "cancels open candidate applications"). Separate from stage, which is the
-- recruiter's decision; a rejected application isn't open and stays as it is.
ALTER TABLE applications
    ADD COLUMN status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'cancelled')),
    ADD COLUMN cancelled_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE applications DROP COLUMN cancelled_at, DROP COLUMN status;
