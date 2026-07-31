-- R23 slot bonus v2: persist the round kind while retaining R22 arrays.

ALTER TABLE IF EXISTS game_slot_bonus_rounds
    DROP CONSTRAINT IF EXISTS game_slot_bonus_rounds_rewards_check;
ALTER TABLE IF EXISTS game_slot_bonus_rounds
    DROP CONSTRAINT IF EXISTS chk_game_slot_bonus_rewards_v2;

ALTER TABLE IF EXISTS game_slot_bonus_rounds
    ADD CONSTRAINT chk_game_slot_bonus_rewards_v2 CHECK (
        (jsonb_typeof(rewards) = 'array' AND jsonb_array_length(rewards) = 3) OR
        (
            jsonb_typeof(rewards) = 'object' AND
            rewards ? 'kind' AND
            rewards ? 'values' AND
            rewards->>'kind' IN ('pick_chest', 'lucky_wheel') AND
            jsonb_typeof(rewards->'values') = 'array' AND
            jsonb_array_length(rewards->'values') = 3
        )
    );
