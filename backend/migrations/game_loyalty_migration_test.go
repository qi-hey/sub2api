package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGameLoyaltyMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("192_game_loyalty_rewards.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "free_spins_remaining")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS game_credit_ledger")
	require.Contains(t, sql, "entry_type VARCHAR(32) NOT NULL CHECK (entry_type IN ('checkin', 'slot_bet', 'slot_payout', 'reward_claim'))")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS game_daily_checkins")
	require.Contains(t, sql, "UNIQUE (user_id, local_date)")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS game_slot_spins")
	require.Contains(t, sql, "paytable_version")
	require.Contains(t, sql, "reel_strip_version")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS game_reward_claims")
	require.Contains(t, sql, "voucher_value NUMERIC(20, 8)")
	require.Contains(t, sql, "reject_game_credit_ledger_mutation")
	require.Contains(t, sql, "reject_game_daily_checkins_mutation")
	require.Contains(t, sql, "reject_game_slot_spins_mutation")
	require.Contains(t, sql, "reject_game_reward_claims_mutation")
	require.NotContains(t, sql, "ADD CONSTRAINT IF NOT EXISTS")
	require.Contains(t, sql, "('game_loyalty_enabled', 'false', NOW())")
	require.Contains(t, sql, "('game_loyalty_checkin_credits', '0', NOW())")
	require.Contains(t, sql, "('game_loyalty_slot_bet_credits', '0', NOW())")
	require.Contains(t, sql, "('game_loyalty_daily_reward_limit', '0', NOW())")
	require.Contains(t, sql, "('game_loyalty_reward_catalog', '[]', NOW())")
	// R20 exchange tables stay for rollback.
	require.NotContains(t, sql, "DROP TABLE game_wallets")
	require.NotContains(t, sql, "DROP TABLE game_wallet_transactions")
}
