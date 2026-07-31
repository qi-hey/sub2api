package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGameWalletMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("191_game_wallets.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS game_wallets")
	require.Contains(t, sql, "credits BIGINT NOT NULL DEFAULT 0 CHECK (credits >= 0)")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS game_wallet_transactions")
	require.Contains(t, sql, "exchange_rate BIGINT NOT NULL CHECK (exchange_rate > 0)")
	require.Contains(t, sql, "balance_before NUMERIC(20, 8) NOT NULL CHECK (balance_before >= 0)")
	require.Contains(t, sql, "credits_before BIGINT NOT NULL CHECK (credits_before >= 0)")
	require.Contains(t, sql, "UNIQUE (user_id, idempotency_key)")
	require.Contains(t, sql, "BEFORE UPDATE OR DELETE ON game_wallet_transactions")
	require.Contains(t, sql, "idx_game_wallet_transactions_user_id_desc")
	require.Contains(t, sql, "game wallet transactions are immutable")
	require.Contains(t, sql, "('game_wallet_enabled', 'false', NOW())")
	require.Contains(t, sql, "('game_wallet_exchange_rate', '0', NOW())")
}
