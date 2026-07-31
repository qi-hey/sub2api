package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGameCenterMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("193_game_center_leaderboard_bonus.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS game_daily_scores")
	require.Contains(t, sql, "UNIQUE (user_id, game_id, local_date)")
	require.Contains(t, sql, "local_date, game_id, score DESC, achieved_at ASC, user_id ASC")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS game_slot_bonus_rounds")
	require.Contains(t, sql, "UNIQUE (user_id, local_date)")
	require.Contains(t, sql, "UNIQUE (token_hash)")
	require.Contains(t, sql, "status IN ('pending', 'claimed', 'expired')")
	require.Contains(t, sql, "jsonb_array_length(rewards) = 3")
	require.Contains(t, sql, "'slot_bonus'")
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS game_credit_ledger_entry_type_check")
	require.NotContains(t, strings.ToLower(sql), "drop trigger")
	require.NotContains(t, strings.ToLower(sql), "token varchar")
	require.NotContains(t, strings.ToLower(sql), "token text")
}

func TestGameSlotBonusV2MigrationContract(t *testing.T) {
	content, err := FS.ReadFile("194_game_slot_bonus_v2.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS game_slot_bonus_rounds_rewards_check")
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS chk_game_slot_bonus_rewards_v2")
	require.Contains(t, sql, "jsonb_typeof(rewards) = 'array'")
	require.Contains(t, sql, "jsonb_typeof(rewards) = 'object'")
	require.Contains(t, sql, "'pick_chest', 'lucky_wheel'")
	require.Contains(t, sql, "jsonb_array_length(rewards->'values') = 3")
}

func TestGameSlotBonusFreeSpinsMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("196_slot_bonus_free_spins.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS uq_game_slot_bonus_user_date")
	require.Contains(t, sql, "'free_spins'")
	require.Contains(t, sql, "'slot-bonus-v3'")
	require.Contains(t, sql, "jsonb_array_length(rewards->'options') = 3")
	require.NotContains(t, sql, "DROP CONSTRAINT IF EXISTS uq_game_slot_bonus_trigger_spin")
	require.NotContains(t, sql, "DROP CONSTRAINT IF EXISTS uq_game_slot_bonus_token_hash")
}

func TestGameAllTimeLeaderboardMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("197_game_all_time_leaderboard.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_game_daily_scores_all_time_leaderboard")
	require.Contains(t, sql, "game_id, user_id, score DESC, achieved_at ASC, local_date ASC")
}

func TestGameSlotBonusV4MigrationContract(t *testing.T) {
	content, err := FS.ReadFile("198_slot_bonus_v4.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS chk_game_slot_bonus_rewards_v3")
	require.Contains(t, sql, "ADD CONSTRAINT chk_game_slot_bonus_rewards_v4")
	require.Contains(t, sql, "'pick_chest', 'lucky_wheel'")
	require.Contains(t, sql, "'slot-bonus-v3', 'slot-bonus-v4'")
	require.Contains(t, sql, "jsonb_array_length(rewards->'options') = 3")
	require.NotContains(t, sql, "DROP CONSTRAINT IF EXISTS uq_game_slot_bonus_trigger_spin")
	require.NotContains(t, sql, "DROP CONSTRAINT IF EXISTS uq_game_slot_bonus_token_hash")
}
