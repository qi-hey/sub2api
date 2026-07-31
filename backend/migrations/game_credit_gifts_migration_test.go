package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGameCreditGiftsMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("199_game_credit_gifts.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS game_credit_transfers")
	require.Contains(t, sql, "UNIQUE (sender_user_id, idempotency_key)")
	require.Contains(t, sql, "sender_user_id <> recipient_user_id")
	require.Contains(t, sql, "sender_credits_after = sender_credits_before - credits_amount")
	require.Contains(t, sql, "recipient_credits_after = recipient_credits_before + credits_amount")
	require.Contains(t, sql, "'reward_claim', 'admin_grant'")
	require.Contains(t, sql, "'gift_sent', 'gift_received'")
	require.Contains(t, sql, "reject_game_credit_transfers_mutation")
}
