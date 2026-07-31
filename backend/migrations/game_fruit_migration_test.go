package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGameFruitMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("195_game_fruit_machine.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS game_fruit_spins")
	require.Contains(t, sql, "'fruit_bet', 'fruit_payout'")
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS game_credit_ledger_entry_type_check")
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS chk_game_credit_ledger_entry_type")
	require.NotContains(t, sql, "pg_get_constraintdef")
	require.NotContains(t, sql, "FOR constraint_name IN")
	require.Contains(t, sql, "UNIQUE (user_id, idempotency_key)")
	require.Contains(t, sql, "stop_index BETWEEN 0 AND 23")
	require.Contains(t, sql, "reject_game_fruit_spins_mutation")
	require.Contains(t, sql, "payout_ledger_id")
}
