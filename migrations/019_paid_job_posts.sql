-- +goose Up
-- Paid job posts (manual "Post a job"): from company credits, or out of pocket
-- through the gateway.

-- HR spending allowances (manual "Company dashboard": recruiter spending
-- limits). Admins spend without a cap; employees can't spend company credits.
-- A monthly cap counts the IST month in spent_month; spending in a new month
-- starts from 0. New staff start at ₹0; offboarding sets the limit back to 0.
ALTER TABLE company_members
    ADD COLUMN cap_type TEXT NOT NULL DEFAULT 'monthly' CHECK (cap_type IN ('monthly', 'total')),
    ADD COLUMN limit_paise BIGINT NOT NULL DEFAULT 0 CHECK (limit_paise >= 0),
    ADD COLUMN spent_paise BIGINT NOT NULL DEFAULT 0 CHECK (spent_paise >= 0),
    ADD COLUMN spent_month DATE;

-- An out-of-pocket post waits in pending_payment (hidden, not open for
-- applications) until the gateway's webhook publishes it. post_idempotency_key
-- makes a retried charged post return the first job instead of a second one.
ALTER TABLE jobs DROP CONSTRAINT jobs_status_check,
    ADD CONSTRAINT jobs_status_check CHECK (status IN ('active', 'closed', 'pending_payment')),
    ADD COLUMN post_idempotency_key TEXT,
    ADD CONSTRAINT jobs_post_idempotency_key UNIQUE (recruiter_id, post_idempotency_key);

-- A payment tops up a wallet or pays for one job post.
ALTER TABLE payments DROP CONSTRAINT payments_purpose_check,
    ADD CONSTRAINT payments_purpose_check CHECK (purpose IN ('topup', 'job_post')),
    ALTER COLUMN wallet_id DROP NOT NULL,
    ADD COLUMN job_id TEXT REFERENCES jobs(id) ON DELETE SET NULL,
    ADD CONSTRAINT payments_target CHECK ((purpose = 'topup') = (wallet_id IS NOT NULL));

-- +goose Down
ALTER TABLE payments DROP CONSTRAINT payments_target, DROP COLUMN job_id, ALTER COLUMN wallet_id SET NOT NULL,
    DROP CONSTRAINT payments_purpose_check, ADD CONSTRAINT payments_purpose_check CHECK (purpose IN ('topup'));
ALTER TABLE jobs DROP CONSTRAINT jobs_post_idempotency_key, DROP COLUMN post_idempotency_key,
    DROP CONSTRAINT jobs_status_check, ADD CONSTRAINT jobs_status_check CHECK (status IN ('active', 'closed'));
ALTER TABLE company_members DROP COLUMN spent_month, DROP COLUMN spent_paise, DROP COLUMN limit_paise, DROP COLUMN cap_type;
