package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGameOddsPoolMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("200_game_odds_pools.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS game_odds_pools")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS game_odds_periods")
	require.Contains(t, sql, "PRIMARY KEY")
	require.Contains(t, sql, "UNIQUE (game_id, period_started_at)")
	require.Contains(t, sql, "'fruit_machine'")
	require.Contains(t, sql, "'five_reel_slot'")
	require.Contains(t, sql, "metadata ->> 'game_id'")
	require.Contains(t, sql, "metadata ->> 'pool_direction'")
	require.Contains(t, sql, "AFTER INSERT ON game_credit_ledger")
	require.Contains(t, sql, "entry_type IN ('slot_bet', 'slot_payout', 'slot_bonus')")
	require.NotContains(t, sql, "user_id")
}
