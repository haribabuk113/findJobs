-- +goose Up
-- Wallets (manual: an INR wallet, non-refundable deposits). One per user and
-- one per company, created on first top-up. Money is BIGINT paise. A balance
-- moves only through an UPDATE in the same transaction as its ledger row; the
-- CHECK is the last line of defence against an overdraft.
CREATE TABLE wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID UNIQUE REFERENCES users(id),
    company_id VARCHAR(100) UNIQUE REFERENCES companies(id),
    balance_paise BIGINT NOT NULL DEFAULT 0 CHECK (balance_paise >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((user_id IS NULL) <> (company_id IS NULL))
);

-- Every balance change. Append-only: a trigger refuses UPDATE and DELETE.
-- amount_paise is signed (credits positive, debits negative). The unique
-- idempotency_key makes a replayed credit or debit fail instead of repeating.
CREATE TABLE wallet_ledger (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    wallet_id UUID NOT NULL REFERENCES wallets(id),
    kind TEXT NOT NULL CHECK (kind IN ('topup', 'apply', 'job_post', 'match_unlock', 'refund')),
    amount_paise BIGINT NOT NULL CHECK (amount_paise <> 0),
    balance_after_paise BIGINT NOT NULL CHECK (balance_after_paise >= 0),
    actor_user_id UUID REFERENCES users(id),
    ref_type TEXT,
    ref_id TEXT,
    idempotency_key TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_wallet_ledger_wallet ON wallet_ledger (wallet_id, id DESC);

-- +goose StatementBegin
CREATE FUNCTION wallet_ledger_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'wallet_ledger is append-only';
END
$$;
-- +goose StatementEnd
CREATE TRIGGER wallet_ledger_append_only BEFORE UPDATE OR DELETE ON wallet_ledger
    FOR EACH ROW EXECUTE FUNCTION wallet_ledger_append_only();

-- Gateway orders (Razorpay). A top-up credits its wallet once, when the
-- gateway's webhook reports the order paid; the row lock on the order makes
-- a replayed or concurrent webhook a no-op.
CREATE TABLE payments (
    id UUID PRIMARY KEY,
    gateway_order_id TEXT NOT NULL UNIQUE,
    gateway_payment_id TEXT,
    purpose TEXT NOT NULL CHECK (purpose IN ('topup')),
    wallet_id UUID NOT NULL REFERENCES wallets(id),
    user_id UUID NOT NULL REFERENCES users(id),
    amount_paise BIGINT NOT NULL CHECK (amount_paise > 0),
    status TEXT NOT NULL DEFAULT 'created' CHECK (status IN ('created', 'paid')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at TIMESTAMPTZ
);

-- +goose Down
DROP TABLE payments;
DROP TABLE wallet_ledger;
DROP FUNCTION wallet_ledger_append_only();
DROP TABLE wallets;
