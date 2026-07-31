-- R21 loyalty casino: check-in credits, server slot, reward vouchers.
-- R20 game_wallets / game_wallet_transactions remain intact for rollback.

ALTER TABLE game_wallets
    ADD COLUMN IF NOT EXISTS free_spins_remaining INTEGER NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_game_wallets_free_spins_remaining_nonneg'
    ) THEN
        ALTER TABLE game_wallets
            ADD CONSTRAINT chk_game_wallets_free_spins_remaining_nonneg
            CHECK (free_spins_remaining >= 0);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS game_credit_ledger (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    entry_type       VARCHAR(32) NOT NULL CHECK (entry_type IN ('checkin', 'slot_bet', 'slot_payout', 'reward_claim')),
    amount           BIGINT NOT NULL CHECK (amount <> 0),
    credits_before   BIGINT NOT NULL CHECK (credits_before >= 0),
    credits_after    BIGINT NOT NULL CHECK (credits_after >= 0),
    reference_type   VARCHAR(32) NOT NULL DEFAULT '',
    reference_id     BIGINT,
    idempotency_key  VARCHAR(128) NOT NULL CHECK (idempotency_key <> ''),
    request_hash     CHAR(64) NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    metadata         JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_game_credit_ledger_user_idempotency UNIQUE (user_id, idempotency_key),
    CONSTRAINT chk_game_credit_ledger_before_after CHECK (
        (amount > 0 AND credits_after = credits_before + amount) OR
        (amount < 0 AND credits_after = credits_before + amount)
    )
);

CREATE INDEX IF NOT EXISTS idx_game_credit_ledger_user_created
    ON game_credit_ledger (user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_game_credit_ledger_user_id_desc
    ON game_credit_ledger (user_id, id DESC);

CREATE INDEX IF NOT EXISTS idx_game_credit_ledger_user_type_created
    ON game_credit_ledger (user_id, entry_type, created_at DESC);

CREATE OR REPLACE FUNCTION reject_game_credit_ledger_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'game credit ledger entries are immutable'
        USING ERRCODE = '55000';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_game_credit_ledger_immutable ON game_credit_ledger;
CREATE TRIGGER trg_game_credit_ledger_immutable
    BEFORE UPDATE OR DELETE ON game_credit_ledger
    FOR EACH ROW EXECUTE FUNCTION reject_game_credit_ledger_mutation();

CREATE TABLE IF NOT EXISTS game_daily_checkins (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    local_date       DATE NOT NULL,
    credits_awarded  BIGINT NOT NULL CHECK (credits_awarded > 0),
    credits_before   BIGINT NOT NULL CHECK (credits_before >= 0),
    credits_after    BIGINT NOT NULL CHECK (credits_after >= 0),
    ledger_id        BIGINT REFERENCES game_credit_ledger(id) ON DELETE RESTRICT,
    idempotency_key  VARCHAR(128) NOT NULL CHECK (idempotency_key <> ''),
    request_hash     CHAR(64) NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_game_daily_checkins_user_local_date UNIQUE (user_id, local_date),
    CONSTRAINT uq_game_daily_checkins_user_idempotency UNIQUE (user_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_game_daily_checkins_user_created
    ON game_daily_checkins (user_id, created_at DESC, id DESC);

CREATE OR REPLACE FUNCTION reject_game_daily_checkins_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'game daily checkins are immutable'
        USING ERRCODE = '55000';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_game_daily_checkins_immutable ON game_daily_checkins;
CREATE TRIGGER trg_game_daily_checkins_immutable
    BEFORE UPDATE OR DELETE ON game_daily_checkins
    FOR EACH ROW EXECUTE FUNCTION reject_game_daily_checkins_mutation();

CREATE TABLE IF NOT EXISTS game_slot_spins (
    id                    BIGSERIAL PRIMARY KEY,
    user_id               BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    bet_credits           BIGINT NOT NULL CHECK (bet_credits > 0),
    payout_credits        BIGINT NOT NULL CHECK (payout_credits >= 0),
    used_free_spin        BOOLEAN NOT NULL DEFAULT FALSE,
    grid                  JSONB NOT NULL,
    stops                 JSONB NOT NULL,
    winning_lines         JSONB NOT NULL DEFAULT '[]'::jsonb,
    scatter_count         INTEGER NOT NULL DEFAULT 0 CHECK (scatter_count >= 0),
    free_spins_awarded    INTEGER NOT NULL DEFAULT 0 CHECK (free_spins_awarded >= 0),
    free_spins_before     INTEGER NOT NULL CHECK (free_spins_before >= 0),
    free_spins_after      INTEGER NOT NULL CHECK (free_spins_after >= 0),
    credits_before        BIGINT NOT NULL CHECK (credits_before >= 0),
    credits_after         BIGINT NOT NULL CHECK (credits_after >= 0),
    paytable_version      VARCHAR(32) NOT NULL,
    reel_strip_version    VARCHAR(32) NOT NULL,
    bet_ledger_id         BIGINT REFERENCES game_credit_ledger(id) ON DELETE RESTRICT,
    payout_ledger_id      BIGINT REFERENCES game_credit_ledger(id) ON DELETE RESTRICT,
    idempotency_key       VARCHAR(128) NOT NULL CHECK (idempotency_key <> ''),
    request_hash          CHAR(64) NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_game_slot_spins_user_idempotency UNIQUE (user_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_game_slot_spins_user_created
    ON game_slot_spins (user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_game_slot_spins_user_id_desc
    ON game_slot_spins (user_id, id DESC);

CREATE OR REPLACE FUNCTION reject_game_slot_spins_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'game slot spins are immutable'
        USING ERRCODE = '55000';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_game_slot_spins_immutable ON game_slot_spins;
CREATE TRIGGER trg_game_slot_spins_immutable
    BEFORE UPDATE OR DELETE ON game_slot_spins
    FOR EACH ROW EXECUTE FUNCTION reject_game_slot_spins_mutation();

CREATE TABLE IF NOT EXISTS game_reward_claims (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    reward_id          VARCHAR(64) NOT NULL CHECK (reward_id <> ''),
    title              VARCHAR(128) NOT NULL,
    credit_cost        BIGINT NOT NULL CHECK (credit_cost > 0),
    voucher_value      NUMERIC(20, 8) NOT NULL CHECK (voucher_value > 0),
    voucher_value_text TEXT NOT NULL CHECK (voucher_value_text <> ''),
    redeem_code        VARCHAR(32) NOT NULL,
    redeem_code_id     BIGINT NOT NULL REFERENCES redeem_codes(id) ON DELETE RESTRICT,
    catalog_snapshot   JSONB NOT NULL,
    claim_local_date   DATE NOT NULL,
    credits_before     BIGINT NOT NULL CHECK (credits_before >= 0),
    credits_after      BIGINT NOT NULL CHECK (credits_after >= 0),
    ledger_id          BIGINT REFERENCES game_credit_ledger(id) ON DELETE RESTRICT,
    expires_at         TIMESTAMPTZ,
    idempotency_key    VARCHAR(128) NOT NULL CHECK (idempotency_key <> ''),
    request_hash       CHAR(64) NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_game_reward_claims_user_idempotency UNIQUE (user_id, idempotency_key),
    CONSTRAINT uq_game_reward_claims_redeem_code_id UNIQUE (redeem_code_id),
    CONSTRAINT uq_game_reward_claims_redeem_code UNIQUE (redeem_code)
);

CREATE INDEX IF NOT EXISTS idx_game_reward_claims_user_created
    ON game_reward_claims (user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_game_reward_claims_user_date_reward
    ON game_reward_claims (user_id, claim_local_date, reward_id);

CREATE INDEX IF NOT EXISTS idx_game_reward_claims_date_reward
    ON game_reward_claims (claim_local_date, reward_id);

CREATE OR REPLACE FUNCTION reject_game_reward_claims_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'game reward claims are immutable'
        USING ERRCODE = '55000';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_game_reward_claims_immutable ON game_reward_claims;
CREATE TRIGGER trg_game_reward_claims_immutable
    BEFORE UPDATE OR DELETE ON game_reward_claims
    FOR EACH ROW EXECUTE FUNCTION reject_game_reward_claims_mutation();

-- Fail-closed defaults: feature disabled until operators configure economics.
INSERT INTO settings (key, value, updated_at) VALUES
    ('game_loyalty_enabled', 'false', NOW()),
    ('game_loyalty_checkin_credits', '0', NOW()),
    ('game_loyalty_slot_bet_credits', '0', NOW()),
    ('game_loyalty_daily_reward_limit', '0', NOW()),
    ('game_loyalty_reward_catalog', '[]', NOW())
ON CONFLICT (key) DO NOTHING;
