-- R39 follow-up: accept guaranteed free-spin plans written with protocol v4.

ALTER TABLE IF EXISTS game_slot_bonus_rounds
    DROP CONSTRAINT IF EXISTS chk_game_slot_bonus_rewards_v3;
ALTER TABLE IF EXISTS game_slot_bonus_rounds
    DROP CONSTRAINT IF EXISTS chk_game_slot_bonus_rewards_v4;

ALTER TABLE IF EXISTS game_slot_bonus_rounds
    ADD CONSTRAINT chk_game_slot_bonus_rewards_v4 CHECK (
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
            rewards->>'protocol' IN ('slot-bonus-v3', 'slot-bonus-v4') AND
            jsonb_typeof(rewards->'bet_credits') = 'number' AND
            jsonb_typeof(rewards->'bonus_count') = 'number' AND
            jsonb_typeof(rewards->'bonus_multiplier') = 'number' AND
            jsonb_typeof(rewards->'options') = 'array' AND
            jsonb_array_length(rewards->'options') = 3
        )
    );

-- Keep one-round-per-spin and unique-token constraints unchanged.
