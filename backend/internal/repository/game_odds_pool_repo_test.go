package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

var gameOddsPoolColumns = []string{
	"game_id", "algorithm_version", "current_profile",
	"period_started_at", "period_ends_at",
	"total_bet_credits", "total_payout_credits", "total_rounds",
	"period_bet_credits", "period_payout_credits", "period_rounds",
}

func TestResolveGameOddsPolicyKeepsCurrentGamePoolPeriod(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Date(2026, 7, 31, 10, 7, 0, 0, time.UTC)
	start := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
	end := start.Add(service.GameOddsPeriod)
	mock.ExpectExec(regexp.QuoteMeta(gameOddsPoolEnsureQuery)).WithArgs(
		service.GameOddsPoolFruit, service.GameOddsAlgorithmVersion,
		service.GameOddsProfileStandard, start, end,
	).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(gameOddsPoolReadQuery)).WithArgs(service.GameOddsPoolFruit).
		WillReturnRows(sqlmock.NewRows(gameOddsPoolColumns).AddRow(
			service.GameOddsPoolFruit, service.GameOddsAlgorithmVersion, service.GameOddsProfileCooling,
			start, end, 1_000_000, 970_000, 100, 100_000, 90_000, 30,
		))

	repo := &gameLoyaltyRepository{db: db}
	policy, err := repo.ResolveGameOddsPolicy(context.Background(), service.GameOddsPoolFruit, now)
	require.NoError(t, err)
	require.Equal(t, service.GameOddsPoolFruit, policy.GameID)
	require.Equal(t, service.GameOddsProfileCooling, policy.Profile)
	require.Equal(t, int64(100_000), policy.Metrics.PeriodBetCredits)
	require.Equal(t, int64(100), policy.Metrics.TotalRounds)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestResolveGameOddsPolicyRotatesOnlyThatGamePool(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Date(2026, 7, 31, 10, 16, 0, 0, time.UTC)
	oldStart := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
	oldEnd := oldStart.Add(service.GameOddsPeriod)
	newStart := oldEnd
	newEnd := newStart.Add(service.GameOddsPeriod)
	row := func() *sqlmock.Rows {
		return sqlmock.NewRows(gameOddsPoolColumns).AddRow(
			service.GameOddsPoolSlot, service.GameOddsAlgorithmVersion, service.GameOddsProfileStandard,
			oldStart, oldEnd, 1_000_000, 1_050_000, 100, 100_000, 120_000, 30,
		)
	}

	mock.ExpectExec(regexp.QuoteMeta(gameOddsPoolEnsureQuery)).WithArgs(
		service.GameOddsPoolSlot, service.GameOddsAlgorithmVersion,
		service.GameOddsProfileStandard, newStart, newEnd,
	).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(gameOddsPoolReadQuery)).WithArgs(service.GameOddsPoolSlot).WillReturnRows(row())
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(gameOddsPoolLockQuery)).WithArgs(service.GameOddsPoolSlot).WillReturnRows(row())
	mock.ExpectExec(regexp.QuoteMeta(gameOddsPeriodInsertQuery)).WithArgs(
		service.GameOddsPoolSlot, service.GameOddsAlgorithmVersion,
		service.GameOddsProfileStandard, service.GameOddsProfileCooling,
		oldStart, oldEnd, int64(100_000), int64(120_000), int64(30), int64(1_000_000), int64(1_050_000),
	).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(gameOddsPoolRotateQuery)).WithArgs(
		service.GameOddsPoolSlot, service.GameOddsAlgorithmVersion,
		service.GameOddsProfileCooling, newStart, newEnd,
	).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &gameLoyaltyRepository{db: db}
	policy, err := repo.ResolveGameOddsPolicy(context.Background(), service.GameOddsPoolSlot, now)
	require.NoError(t, err)
	require.Equal(t, service.GameOddsProfileCooling, policy.Profile)
	require.Zero(t, policy.Metrics.PeriodBetCredits)
	require.NoError(t, mock.ExpectationsWereMet())
}
