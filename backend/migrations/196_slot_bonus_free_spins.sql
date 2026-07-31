-- R27: every qualifying spin owns an independent, server-authored BONUS round.

ALTER TABLE IF EXISTS game_slot_bonus_rounds
    DROP CONSTRAINT IF EXISTS uq_game_slot_bonus_user_date;

ALTER TABLE IF EXISTS game_slot_bonus_rounds
    DROP CONSTRAINT IF EXISTS chk_game_slot_bonus_rewards_v2;

ALTER TABLE IF EXISTS game_slot_bonus_rounds
    ADD CONSTRAINT chk_game_slot_bonus_rewards_v3 CHECK (
        (jsonb_typeof(rewards) = 'array' AND jsonb_array_length(rewards) = 3) OR
        (
            jsonb_typeof(rewards) = 'object' AND
            rewards ? 'kind' AND
            rewards ? 'values' AND
            rewards->>'kind' IN ('pick_chest', 'lucky_wheel') AND
            jsonb_typeof(rewards->'values') = 'array' AND
            jsonb_array_length(rewards->'values') = 3
        ) OR
        (
            jsonb_typeof(rewards) = 'object' AND
            rewards->>'kind' = 'free_spins' AND
            rewards->>'protocol' = 'slot-bonus-v3' AND
            jsonb_typeof(rewards->'bet_credits') = 'number' AND
            jsonb_typeof(rewards->'bonus_count') = 'number' AND
            jsonb_typeof(rewards->'bonus_multiplier') = 'number' AND
            jsonb_typeof(rewards->'options') = 'array' AND
            jsonb_array_length(rewards->'options') = 3
        )
    );

-- These constraints intentionally remain in force:
--   uq_game_slot_bonus_trigger_spin (one round per spin)
--   uq_game_slot_bonus_token_hash   (one bearer token per round)
