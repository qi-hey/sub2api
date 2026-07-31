-- R43 global, per-game odds pools. Pools are isolated by game_id and never
-- contain a user identifier, so no account can receive personalized odds.

CREATE TABLE IF NOT EXISTS game_odds_pools (
    game_id                VARCHAR(64) PRIMARY KEY,
    algorithm_version      VARCHAR(32) NOT NULL DEFAULT 'global-pool-v1',
    current_profile        VARCHAR(16) NOT NULL DEFAULT 'standard'
                           CHECK (current_profile IN ('standard', 'cooling', 'tight')),
    total_bet_credits      BIGINT NOT NULL DEFAULT 0 CHECK (total_bet_credits >= 0),
    total_payout_credits   BIGINT NOT NULL DEFAULT 0 CHECK (total_payout_credits >= 0),
    reserve_credits        BIGINT NOT NULL DEFAULT 0,
    total_rounds           BIGINT NOT NULL DEFAULT 0 CHECK (total_rounds >= 0),
    period_bet_credits     BIGINT NOT NULL DEFAULT 0 CHECK (period_bet_credits >= 0),
    period_payout_credits  BIGINT NOT NULL DEFAULT 0 CHECK (period_payout_credits >= 0),
    period_rounds          BIGINT NOT NULL DEFAULT 0 CHECK (period_rounds >= 0),
    period_started_at      TIMESTAMPTZ NOT NULL,
    period_ends_at         TIMESTAMPTZ NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_game_odds_pool_id
        CHECK (game_id ~ '^[a-z0-9][a-z0-9._-]{0,63}$'),
    CONSTRAINT chk_game_odds_pool_period
        CHECK (period_ends_at > period_started_at)
);

CREATE TABLE IF NOT EXISTS game_odds_periods (
    id                     BIGSERIAL PRIMARY KEY,
    game_id                VARCHAR(64) NOT NULL REFERENCES game_odds_pools(game_id) ON DELETE RESTRICT,
    algorithm_version      VARCHAR(32) NOT NULL,
    profile                VARCHAR(16) NOT NULL CHECK (profile IN ('standard', 'cooling', 'tight')),
    next_profile           VARCHAR(16) NOT NULL CHECK (next_profile IN ('standard', 'cooling', 'tight')),
    period_started_at      TIMESTAMPTZ NOT NULL,
    period_ends_at         TIMESTAMPTZ NOT NULL,
    bet_credits            BIGINT NOT NULL CHECK (bet_credits >= 0),
    payout_credits         BIGINT NOT NULL CHECK (payout_credits >= 0),
    rounds                 BIGINT NOT NULL CHECK (rounds >= 0),
    total_bet_credits      BIGINT NOT NULL CHECK (total_bet_credits >= 0),
    total_payout_credits   BIGINT NOT NULL CHECK (total_payout_credits >= 0),
    closed_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_game_odds_period UNIQUE (game_id, period_started_at),
    CONSTRAINT chk_game_odds_period_range CHECK (period_ends_at > period_started_at)
);

CREATE INDEX IF NOT EXISTS idx_game_odds_periods_game_started
    ON game_odds_periods (game_id, period_started_at DESC);

-- Seed the two current paid games. New games are created automatically when
-- their immutable credit-ledger metadata supplies game_id and pool_direction.
INSERT INTO game_odds_pools (
    game_id, period_started_at, period_ends_at
)
VALUES
    ('fruit_machine', date_trunc('hour', NOW()) + floor(extract(minute FROM NOW()) / 15) * interval '15 minutes',
                      date_trunc('hour', NOW()) + (floor(extract(minute FROM NOW()) / 15) + 1) * interval '15 minutes'),
    ('five_reel_slot', date_trunc('hour', NOW()) + floor(extract(minute FROM NOW()) / 15) * interval '15 minutes',
                       date_trunc('hour', NOW()) + (floor(extract(minute FROM NOW()) / 15) + 1) * interval '15 minutes')
ON CONFLICT (game_id) DO NOTHING;

-- Preserve all historical economics without modifying immutable spin or ledger
-- rows. Slot BONUS payouts belong only to the five-reel slot pool.
WITH mapped AS (
    SELECT
        CASE
            WHEN entry_type IN ('fruit_bet', 'fruit_payout') THEN 'fruit_machine'
            WHEN entry_type IN ('slot_bet', 'slot_payout', 'slot_bonus') THEN 'five_reel_slot'
        END AS game_id,
        CASE WHEN entry_type IN ('fruit_bet', 'slot_bet') THEN -amount ELSE 0 END AS bet_credits,
        CASE WHEN entry_type IN ('fruit_payout', 'slot_payout', 'slot_bonus') THEN amount ELSE 0 END AS payout_credits,
        CASE WHEN entry_type IN ('fruit_bet', 'slot_bet') THEN 1 ELSE 0 END AS rounds
    FROM game_credit_ledger
    WHERE entry_type IN ('fruit_bet', 'fruit_payout', 'slot_bet', 'slot_payout', 'slot_bonus')
), totals AS (
    SELECT game_id,
           COALESCE(SUM(bet_credits), 0)::BIGINT AS bet_credits,
           COALESCE(SUM(payout_credits), 0)::BIGINT AS payout_credits,
           COALESCE(SUM(rounds), 0)::BIGINT AS rounds
    FROM mapped
    GROUP BY game_id
)
UPDATE game_odds_pools AS pool
SET total_bet_credits = totals.bet_credits,
    total_payout_credits = totals.payout_credits,
    reserve_credits = totals.bet_credits - totals.payout_credits,
    total_rounds = totals.rounds,
    updated_at = NOW()
FROM totals
WHERE pool.game_id = totals.game_id;

CREATE OR REPLACE FUNCTION update_game_odds_pool_from_ledger()
RETURNS TRIGGER AS $$
DECLARE
    resolved_game_id TEXT;
    resolved_direction TEXT;
    bet_delta BIGINT := 0;
    payout_delta BIGINT := 0;
    round_delta BIGINT := 0;
    window_start TIMESTAMPTZ;
BEGIN
    resolved_game_id := NULLIF(lower(btrim(NEW.metadata ->> 'game_id')), '');
    resolved_direction := NULLIF(lower(btrim(NEW.metadata ->> 'pool_direction')), '');

    IF resolved_game_id IS NULL THEN
        resolved_game_id := CASE
            WHEN NEW.entry_type IN ('fruit_bet', 'fruit_payout') THEN 'fruit_machine'
            WHEN NEW.entry_type IN ('slot_bet', 'slot_payout', 'slot_bonus') THEN 'five_reel_slot'
        END;
    END IF;
    IF resolved_direction IS NULL THEN
        resolved_direction := CASE
            WHEN NEW.entry_type IN ('fruit_bet', 'slot_bet') THEN 'bet'
            WHEN NEW.entry_type IN ('fruit_payout', 'slot_payout', 'slot_bonus') THEN 'payout'
        END;
    END IF;

    IF resolved_game_id IS NULL OR resolved_game_id !~ '^[a-z0-9][a-z0-9._-]{0,63}$' THEN
        RETURN NEW;
    END IF;
    IF resolved_direction = 'bet' AND NEW.amount < 0 THEN
        bet_delta := -NEW.amount;
        round_delta := 1;
    ELSIF resolved_direction = 'payout' AND NEW.amount > 0 THEN
        payout_delta := NEW.amount;
    ELSE
        RETURN NEW;
    END IF;

    window_start := date_trunc('hour', NEW.created_at)
        + floor(extract(minute FROM NEW.created_at) / 15) * interval '15 minutes';

    INSERT INTO game_odds_pools (
        game_id, total_bet_credits, total_payout_credits, reserve_credits, total_rounds,
        period_bet_credits, period_payout_credits, period_rounds,
        period_started_at, period_ends_at
    ) VALUES (
        resolved_game_id, bet_delta, payout_delta, bet_delta - payout_delta, round_delta,
        bet_delta, payout_delta, round_delta,
        window_start, window_start + interval '15 minutes'
    )
    ON CONFLICT (game_id) DO UPDATE SET
        total_bet_credits = game_odds_pools.total_bet_credits + EXCLUDED.total_bet_credits,
        total_payout_credits = game_odds_pools.total_payout_credits + EXCLUDED.total_payout_credits,
        reserve_credits = game_odds_pools.reserve_credits + EXCLUDED.reserve_credits,
        total_rounds = game_odds_pools.total_rounds + EXCLUDED.total_rounds,
        period_bet_credits = game_odds_pools.period_bet_credits + EXCLUDED.period_bet_credits,
        period_payout_credits = game_odds_pools.period_payout_credits + EXCLUDED.period_payout_credits,
        period_rounds = game_odds_pools.period_rounds + EXCLUDED.period_rounds,
        updated_at = NOW();

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_game_credit_ledger_odds_pool ON game_credit_ledger;
CREATE TRIGGER trg_game_credit_ledger_odds_pool
    AFTER INSERT ON game_credit_ledger
    FOR EACH ROW EXECUTE FUNCTION update_game_odds_pool_from_ledger();
