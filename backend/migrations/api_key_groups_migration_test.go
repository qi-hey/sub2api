package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyGroupsMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("185_api_key_groups.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS api_key_groups")
	require.Contains(t, sql, "PRIMARY KEY (api_key_id, group_id)")
	require.Contains(t, sql, "REFERENCES api_keys(id) ON DELETE CASCADE")
	require.Contains(t, sql, "REFERENCES groups(id) ON DELETE CASCADE")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_api_key_groups_group_id")
	require.Contains(t, sql, "SELECT id, group_id FROM api_keys")
	require.Contains(t, sql, "WHERE group_id IS NOT NULL AND deleted_at IS NULL")
	require.Contains(t, sql, "ON CONFLICT (api_key_id, group_id) DO NOTHING")
}
