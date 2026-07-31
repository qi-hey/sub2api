-- Game wallets are intentionally separate from the account balance ledger.
CREATE TABLE IF NOT EXISTS game_wallets (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    credits     BIGINT NOT NULL DEFAULT 0 CHECK (credits >= 0),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_game_wallets_user_id UNIQUE (user_id)
);

CREATE TABLE IF NOT EXISTS game_wallet_transactions (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    direction         VARCHAR(32) NOT NULL CHECK (direction IN ('balance_to_credits', 'credits_to_balance')),
    balance_amount    NUMERIC(20, 8) NOT NULL CHECK (balance_amount > 0),
    credits_amount    BIGINT NOT NULL CHECK (credits_amount > 0),
    exchange_rate     BIGINT NOT NULL CHECK (exchange_rate > 0),
    balance_before    NUMERIC(20, 8) NOT NULL CHECK (balance_before >= 0),
    credits_before    BIGINT NOT NULL CHECK (credits_before >= 0),
    balance_after     NUMERIC(20, 8) NOT NULL CHECK (balance_after >= 0),
    credits_after     BIGINT NOT NULL CHECK (credits_after >= 0),
    idempotency_key   VARCHAR(128) NOT NULL CHECK (idempotency_key <> ''),
    request_hash      CHAR(64) NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_game_wallet_transactions_user_idempotency UNIQUE (user_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_game_wallet_transactions_user_created
    ON game_wallet_transactions (user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_game_wallet_transactions_user_id_desc
    ON game_wallet_transactions (user_id, id DESC);

CREATE OR REPLACE FUNCTION reject_game_wallet_transaction_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'game wallet transactions are immutable'
        USING ERRCODE = '55000';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_game_wallet_transactions_immutable ON game_wallet_transactions;
CREATE TRIGGER trg_game_wallet_transactions_immutable
    BEFORE UPDATE OR DELETE ON game_wallet_transactions
    FOR EACH ROW EXECUTE FUNCTION reject_game_wallet_transaction_mutation();

-- Existing installations may already have settings, so seed these independently
-- from the application's first-run settings initialization.
INSERT INTO settings (key, value, updated_at) VALUES
    ('game_wallet_enabled', 'false', NOW()),
    ('game_wallet_exchange_rate', '0', NOW()),
    ('game_wallet_daily_limit', '0', NOW())
ON CONFLICT (key) DO NOTHING;
