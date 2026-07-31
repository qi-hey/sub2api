package repository

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestFruitSpinIdempotentReplayReturnsPersistedOutcome(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC()
	storedHash := strings.Repeat("a", 64)
	requestHash := strings.Repeat("b", 64)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyLockUserQuery)).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow("0"))
	mock.ExpectQuery(regexp.QuoteMeta(gameFruitFindIdempotencyQuery)).WithArgs(int64(7), "fruit-replay").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "bets", "total_bet", "stop_index", "outcome", "payout_credits",
			"credits_before", "credits_after", "paytable_version", "idempotency_key", "request_hash", "created_at",
		}).AddRow(9, 7, `[1,0,0,0,0,0,0,0]`, 1, 0,
			`{"kind":"fruit","symbol":"bar","size":"big","multiplier":100}`,
			100, 500, 599, "fruit-v0", "fruit-replay", storedHash, now))
	mock.ExpectCommit()

	repo := &gameLoyaltyRepository{db: db}
	result, err := repo.FruitSpin(context.Background(), 7, service.FruitSpinInput{
		Bets: [service.FruitDoorCount]int64{1}, TotalBet: 1,
		IdempotencyKey: "fruit-replay", RequestHash: requestHash,
	}, &service.FruitSpinResult{Bets: [service.FruitDoorCount]int64{1}, TotalBet: 1})
	require.NoError(t, err)
	require.True(t, result.IdempotentReplay)
	require.Equal(t, int64(100), result.Payout)
	require.Equal(t, 0, result.StopIndex)
	require.Equal(t, "fruit-v0", result.PaytableVersion)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFindFruitSpinReturnsPersistedOutcomeWithoutStartingMutation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta(gameFruitFindIdempotencyQuery)).WithArgs(int64(7), "fruit-existing").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "bets", "total_bet", "stop_index", "outcome", "payout_credits",
			"credits_before", "credits_after", "paytable_version", "idempotency_key", "request_hash", "created_at",
		}).AddRow(10, 7, `[0,0,0,0,0,0,0,3]`, 3, 15,
			`{"kind":"fruit","symbol":"apple","size":"big","multiplier":5}`,
			15, 500, 512, "fruit-v0", "fruit-existing", strings.Repeat("c", 64), now))

	repo := &gameLoyaltyRepository{db: db}
	result, err := repo.FindFruitSpin(context.Background(), 7, "fruit-existing")
	require.NoError(t, err)
	require.Equal(t, int64(10), result.ID)
	require.Equal(t, int64(3), result.Bets[7])
	require.Equal(t, "fruit-v0", result.PaytableVersion)
	require.False(t, result.IdempotentReplay)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFruitSpinInsufficientCreditsCreatesNoLedger(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyLockUserQuery)).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow("0"))
	mock.ExpectQuery(regexp.QuoteMeta(gameFruitFindIdempotencyQuery)).WithArgs(int64(7), "fruit-low").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec(regexp.QuoteMeta(gameLoyaltyEnsureQuery)).WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyLockWalletQuery)).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "credits", "free_spins_remaining"}).AddRow(1, 1, 0))
	mock.ExpectQuery(regexp.QuoteMeta(gameFruitCountTodayQuery)).WithArgs(int64(7), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyDeductCreditsQuery)).WithArgs(int64(7), int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"credits"}))
	mock.ExpectRollback()

	bets := [service.FruitDoorCount]int64{2}
	repo := &gameLoyaltyRepository{db: db}
	_, err = repo.FruitSpin(context.Background(), 7, service.FruitSpinInput{
		Bets: bets, TotalBet: 2, IdempotencyKey: "fruit-low", RequestHash: strings.Repeat("b", 64),
	}, &service.FruitSpinResult{Bets: bets, TotalBet: 2, PaytableVersion: service.FruitPaytableVersion})
	require.ErrorIs(t, err, service.ErrGameLoyaltyInsufficientCredits)
	require.NoError(t, mock.ExpectationsWereMet())
}
