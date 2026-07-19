package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestUserGroupRateRepositoryGetRPMOverridesByUserAndGroupIDsUsesOneQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &userGroupRateRepository{sql: db}

	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT group_id, rpm_override
		FROM user_group_rate_multipliers
		WHERE user_id = $1 AND group_id = ANY($2) AND rpm_override IS NOT NULL
	`)).
		WithArgs(int64(7), pq.Array([]int64{2, 3})).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "rpm_override"}).
			AddRow(int64(2), 0).
			AddRow(int64(3), 10))

	got, err := repo.GetRPMOverridesByUserAndGroupIDs(context.Background(), 7, []int64{3, 0, -1, 2, 3})
	require.NoError(t, err)
	require.Equal(t, map[int64]int{2: 0, 3: 10}, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserGroupRateRepositoryGetRPMOverridesByUserAndGroupIDsSkipsEmptyNormalizedInput(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &userGroupRateRepository{sql: db}

	got, err := repo.GetRPMOverridesByUserAndGroupIDs(context.Background(), 7, []int64{0, -1})
	require.NoError(t, err)
	require.Empty(t, got)
	require.NoError(t, mock.ExpectationsWereMet())
}
