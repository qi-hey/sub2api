-- R42 user-to-user game credit gifts by exact account email.

ALTER TABLE game_credit_ledger
    DROP CONSTRAINT IF EXISTS game_credit_ledger_entry_type_check;
ALTER TABLE game_credit_ledger
    DROP CONSTRAINT IF EXISTS chk_game_credit_ledger_entry_type;
ALTER TABLE game_credit_ledger
    ADD CONSTRAINT chk_game_credit_ledger_entry_type
    CHECK (entry_type IN (
        'checkin', 'slot_bet', 'slot_payout', 'slot_bonus',
        'fruit_bet', 'fruit_payout', 'reward_claim', 'admin_grant',
        'gift_sent', 'gift_received'
    ));

CREATE TABLE IF NOT EXISTS game_credit_transfers (
    id                       BIGSERIAL PRIMARY KEY,
    sender_user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    recipient_user_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    recipient_email          VARCHAR(320) NOT NULL CHECK (recipient_email <> ''),
    credits_amount           BIGINT NOT NULL CHECK (credits_amount > 0),
    sender_credits_before    BIGINT NOT NULL CHECK (sender_credits_before >= 0),
    sender_credits_after     BIGINT NOT NULL CHECK (sender_credits_after >= 0),
    recipient_credits_before BIGINT NOT NULL CHECK (recipient_credits_before >= 0),
    recipient_credits_after  BIGINT NOT NULL CHECK (recipient_credits_after >= 0),
    idempotency_key          VARCHAR(128) NOT NULL CHECK (idempotency_key <> ''),
    request_hash             CHAR(64) NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_game_credit_transfers_sender_idempotency
        UNIQUE (sender_user_id, idempotency_key),
    CONSTRAINT chk_game_credit_transfers_distinct_users
        CHECK (sender_user_id <> recipient_user_id),
    CONSTRAINT chk_game_credit_transfers_sender_math
        CHECK (sender_credits_after = sender_credits_before - credits_amount),
    CONSTRAINT chk_game_credit_transfers_recipient_math
        CHECK (recipient_credits_after = recipient_credits_before + credits_amount)
);

CREATE INDEX IF NOT EXISTS idx_game_credit_transfers_sender_created
    ON game_credit_transfers (sender_user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_game_credit_transfers_recipient_created
    ON game_credit_transfers (recipient_user_id, created_at DESC, id DESC);

CREATE OR REPLACE FUNCTION reject_game_credit_transfers_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'game credit transfers are immutable'
        USING ERRCODE = '55000';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_game_credit_transfers_immutable ON game_credit_transfers;
CREATE TRIGGER trg_game_credit_transfers_immutable
    BEFORE UPDATE OR DELETE ON game_credit_transfers
    FOR EACH ROW EXECUTE FUNCTION reject_game_credit_transfers_mutation();
