-- R22 game center: daily entertainment leaderboards and slot bonus rounds.

CREATE TABLE IF NOT EXISTS game_daily_scores (
    id           BIGSERIAL PRIMARY KEY,
    local_date   DATE NOT NULL,
    game_id      VARCHAR(32) NOT NULL CHECK (game_id IN (
        'snake', 'tetris', 'breakout', 'merge2048', 'orbital', 'hoarder', 'lucky'
    )),
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    score        BIGINT NOT NULL CHECK (score >= 0),
    achieved_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_game_daily_scores_user_game_date UNIQUE (user_id, game_id, local_date)
);

CREATE INDEX IF NOT EXISTS idx_game_daily_scores_leaderboard
    ON game_daily_scores (local_date, game_id, score DESC, achieved_at ASC, user_id ASC);

CREATE TABLE IF NOT EXISTS game_slot_bonus_rounds (
    id                    BIGSERIAL PRIMARY KEY,
    user_id               BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    trigger_spin_id       BIGINT NOT NULL REFERENCES game_slot_spins(id) ON DELETE RESTRICT,
    local_date            DATE NOT NULL,
    token_hash            CHAR(64) NOT NULL CHECK (token_hash ~ '^[0-9a-f]{64}$'),
    rewards               JSONB NOT NULL CHECK (
        jsonb_typeof(rewards) = 'array' AND jsonb_array_length(rewards) = 3
    ),
    status                VARCHAR(16) NOT NULL DEFAULT 'pending'
                              CHECK (status IN ('pending', 'claimed', 'expired')),
    choice                INTEGER CHECK (choice BETWEEN 0 AND 2),
    reward_credits        BIGINT CHECK (reward_credits > 0),
    expires_at            TIMESTAMPTZ NOT NULL,
    claimed_at            TIMESTAMPTZ,
    claim_idempotency_key VARCHAR(128),
    claim_request_hash    CHAR(64) CHECK (
        claim_request_hash IS NULL OR claim_request_hash ~ '^[0-9a-f]{64}$'
    ),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_game_slot_bonus_user_date UNIQUE (user_id, local_date),
    CONSTRAINT uq_game_slot_bonus_trigger_spin UNIQUE (trigger_spin_id),
    CONSTRAINT uq_game_slot_bonus_token_hash UNIQUE (token_hash),
    CONSTRAINT uq_game_slot_bonus_user_claim_idempotency UNIQUE (user_id, claim_idempotency_key),
    CONSTRAINT chk_game_slot_bonus_claim_state CHECK (
        (status = 'pending' AND choice IS NULL AND reward_credits IS NULL AND claimed_at IS NULL) OR
        (status = 'claimed' AND choice IS NOT NULL AND reward_credits IS NOT NULL AND claimed_at IS NOT NULL) OR
        (status = 'expired' AND choice IS NULL AND reward_credits IS NULL AND claimed_at IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_game_slot_bonus_user_status_expires
    ON game_slot_bonus_rounds (user_id, status, expires_at);

-- Replace only the entry_type constraint. The immutable UPDATE/DELETE trigger
-- on game_credit_ledger remains installed and unchanged.
ALTER TABLE game_credit_ledger
    DROP CONSTRAINT IF EXISTS game_credit_ledger_entry_type_check;
ALTER TABLE game_credit_ledger
    DROP CONSTRAINT IF EXISTS chk_game_credit_ledger_entry_type;
ALTER TABLE game_credit_ledger
    ADD CONSTRAINT chk_game_credit_ledger_entry_type CHECK (entry_type IN (
        'checkin', 'slot_bet', 'slot_payout', 'reward_claim', 'slot_bonus'
    ));
