-- R25 server-authoritative ring fruit machine.

-- Replace only the entry_type allow-list created by migrations 192/193.
-- Other CHECK constraints may also reference entry_type and must remain intact.
ALTER TABLE game_credit_ledger
    DROP CONSTRAINT IF EXISTS game_credit_ledger_entry_type_check;
ALTER TABLE game_credit_ledger
    DROP CONSTRAINT IF EXISTS chk_game_credit_ledger_entry_type;
ALTER TABLE game_credit_ledger
    ADD CONSTRAINT chk_game_credit_ledger_entry_type
    CHECK (entry_type IN (
        'checkin', 'slot_bet', 'slot_payout', 'slot_bonus',
        'fruit_bet', 'fruit_payout', 'reward_claim'
    ));

CREATE TABLE IF NOT EXISTS game_fruit_spins (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    bets               JSONB NOT NULL,
    total_bet          BIGINT NOT NULL CHECK (total_bet BETWEEN 1 AND 800),
    stop_index         INTEGER NOT NULL CHECK (stop_index BETWEEN 0 AND 23),
    outcome            JSONB NOT NULL,
    payout_credits     BIGINT NOT NULL CHECK (payout_credits >= 0),
    credits_before     BIGINT NOT NULL CHECK (credits_before >= 0),
    credits_after      BIGINT NOT NULL CHECK (credits_after >= 0),
    paytable_version   VARCHAR(32) NOT NULL,
    bet_ledger_id      BIGINT NOT NULL REFERENCES game_credit_ledger(id) ON DELETE RESTRICT,
    payout_ledger_id   BIGINT REFERENCES game_credit_ledger(id) ON DELETE RESTRICT,
    idempotency_key    VARCHAR(128) NOT NULL CHECK (idempotency_key <> ''),
    request_hash       CHAR(64) NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_game_fruit_spins_user_idempotency UNIQUE (user_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_game_fruit_spins_user_created
    ON game_fruit_spins (user_id, created_at DESC, id DESC);

CREATE OR REPLACE FUNCTION reject_game_fruit_spins_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'game fruit spins are immutable' USING ERRCODE = '55000';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_game_fruit_spins_immutable ON game_fruit_spins;
CREATE TRIGGER trg_game_fruit_spins_immutable
    BEFORE UPDATE OR DELETE ON game_fruit_spins
    FOR EACH ROW EXECUTE FUNCTION reject_game_fruit_spins_mutation();
