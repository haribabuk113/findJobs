-- +goose Up
-- Legal records (system requirements §12–14, §21). Both tables are
-- append-only (a trigger refuses UPDATE and DELETE) and outlive the account:
-- user_id has no foreign key, and ecosystem_id (IAM's id) is kept, so a
-- deleted account's consents and history stay attributable.

-- +goose StatementBegin
CREATE FUNCTION refuse_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME;
END;
$$;
-- +goose StatementEnd

-- One row per decision on a legal document; the latest per document counts.
CREATE TABLE consents (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL,
    ecosystem_id TEXT NOT NULL,
    doc_id TEXT NOT NULL,
    doc_version TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('accepted', 'withdrawn')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_consents_user_doc ON consents (user_id, doc_id, id DESC);
CREATE TRIGGER consents_append_only BEFORE UPDATE OR DELETE ON consents
    FOR EACH ROW EXECUTE FUNCTION refuse_change();

-- Account creation and deletion.
CREATE TABLE account_events (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL,
    ecosystem_id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('created', 'deleted')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_account_events_user ON account_events (user_id);
CREATE TRIGGER account_events_append_only BEFORE UPDATE OR DELETE ON account_events
    FOR EACH ROW EXECUTE FUNCTION refuse_change();

-- Accounts that existed before this table get their creation record.
INSERT INTO account_events (user_id, ecosystem_id, kind, created_at)
SELECT id, iam_user_id, 'created', COALESCE(created_at, NOW()) FROM users;

-- +goose Down
DROP TABLE account_events;
DROP TABLE consents;
DROP FUNCTION refuse_change();
