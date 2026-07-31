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

func TestGameTokenBalanceCreditSQLOmitsTotalRecharged(t *testing.T) {
	// Keep the R21 voucher credit path free of recharge accumulation.
	const updateSQL = `
		UPDATE users
		SET balance = balance + $1::numeric, updated_at = NOW()
		WHERE id = $2
		  AND deleted_at IS NULL
		  AND $1::numeric > 0
		  AND balance <= 999999999999.99999999 - $1::numeric
	`
	require.NotContains(t, strings.ToLower(updateSQL), "total_recharged")
	require.Contains(t, updateSQL, "$1::numeric")
}

func TestGameCenterAllTimeLeaderboardAndBonusConstraints(t *testing.T) {
	require.Contains(t, gameDailyScoreUpsertQuery, "game_daily_scores.score < EXCLUDED.score")
	require.Contains(t, gameAllTimeLeaderboardTopQuery, "SELECT DISTINCT ON (s.user_id)")
	require.Contains(t, gameAllTimeLeaderboardTopQuery, "ORDER BY score DESC, achieved_at ASC, user_id ASC")
	require.Contains(t, gameAllTimeLeaderboardTopQuery, "LIMIT $2")
	require.NotContains(t, strings.ToUpper(gameAllTimeLeaderboardTopQuery), "ROW_NUMBER")
	require.Contains(t, gameAllTimeLeaderboardUserRankQuery, "SELECT COUNT(*) + 1")
	require.NotContains(t, strings.ToUpper(gameAllTimeLeaderboardUserRankQuery), "ROW_NUMBER")
	require.Contains(t, gameSlotBonusInsertQuery, "ON CONFLICT (trigger_spin_id) DO NOTHING")
	require.NotContains(t, gameSlotBonusInsertQuery, "ON CONFLICT (user_id, local_date)")
	require.NotContains(t, gameSlotBonusInsertQuery, "Token,")
	require.Contains(t, gameSlotBonusInsertQuery, "token_hash")
}

func TestSlotBonusRewardsV3FreeSpinPlanRoundTrip(t *testing.T) {
	plan := &service.SlotBonusPlan{
		Kind: service.SlotBonusKindFreeSpins, Protocol: service.SlotBonusProtocolV3,
		BetCredits: 10, BonusCount: 3, BonusMultiplier: 1,
	}
	for choice, config := range []struct{ spins, multiplier int }{{8, 5}, {12, 3}, {20, 2}} {
		results := make([]*service.SlotSpinResult, config.spins)
		for index := range results {
			results[index] = &service.SlotSpinResult{BetCredits: 10}
		}
		plan.Options = append(plan.Options, service.SlotBonusOption{
			Choice: choice, Spins: config.spins, Multiplier: config.multiplier,
			RewardCredits: 1, SpinResults: results,
		})
	}
	raw, err := encodeSlotBonusOffer(&service.SlotBonusRoundOfferInput{
		Kind: service.SlotBonusKindFreeSpins, Plan: plan,
	})
	require.NoError(t, err)
	kind, _, decoded, err := decodeSlotBonusRewards(raw)
	require.NoError(t, err)
	require.Equal(t, service.SlotBonusKindFreeSpins, kind)
	require.NotNil(t, decoded)
	require.Equal(t, 3, decoded.BonusCount)
	require.Equal(t, []int{8, 12, 20}, []int{
		decoded.Options[0].Spins, decoded.Options[1].Spins, decoded.Options[2].Spins,
	})
}

func TestSlotBonusRewardsV4FreeSpinPlanRoundTripKeepsFeatureGuarantees(t *testing.T) {
	plan := &service.SlotBonusPlan{
		Kind: service.SlotBonusKindFreeSpins, Protocol: service.SlotBonusProtocolV4,
		BetCredits: 10, BonusCount: 4, BonusMultiplier: 2,
	}
	for choice, config := range []struct{ spins, multiplier int }{{8, 5}, {12, 3}, {20, 2}} {
		minimumWins, err := service.SlotBonusMinimumWinningSpins(config.spins)
		require.NoError(t, err)
		results := make([]*service.SlotSpinResult, config.spins)
		for index := range results {
			payout := int64(0)
			if index < minimumWins {
				payout = 10
			}
			results[index] = &service.SlotSpinResult{BetCredits: 10, TotalPayout: payout}
		}
		base := int64(minimumWins * 10)
		reward, err := service.SlotBonusFeatureRewardCredits(base, 10, config.multiplier, 2)
		require.NoError(t, err)
		plan.Options = append(plan.Options, service.SlotBonusOption{
			Choice: choice, Spins: config.spins, Multiplier: config.multiplier,
			BasePayout: base, RewardCredits: reward, SpinResults: results,
		})
	}
	raw, err := encodeSlotBonusOffer(&service.SlotBonusRoundOfferInput{
		Kind: service.SlotBonusKindFreeSpins, Plan: plan,
	})
	require.NoError(t, err)
	kind, _, decoded, err := decodeSlotBonusRewards(raw)
	require.NoError(t, err)
	require.Equal(t, service.SlotBonusKindFreeSpins, kind)
	require.NotNil(t, decoded)
	require.Equal(t, service.SlotBonusProtocolV4, decoded.Protocol)
	require.Equal(t, int64(300), decoded.Options[0].RewardCredits)
}

func TestSlotBonusRewardsV2AndLegacyArrayCompatibility(t *testing.T) {
	legacyKind, legacyValues, legacyPlan, err := decodeSlotBonusRewards([]byte(`[100,200,50]`))
	require.NoError(t, err)
	require.Equal(t, service.SlotBonusKindPickChest, legacyKind)
	require.Equal(t, [3]int64{100, 200, 50}, legacyValues)
	require.Nil(t, legacyPlan)

	wheelJSON, err := encodeSlotBonusRewards(service.SlotBonusKindLuckyWheel, [3]int64{100, 100, 100})
	require.NoError(t, err)
	require.JSONEq(t, `{"kind":"lucky_wheel","values":[100,100,100]}`, string(wheelJSON))
	wheelKind, wheelValues, wheelPlan, err := decodeSlotBonusRewards(wheelJSON)
	require.NoError(t, err)
	require.Equal(t, service.SlotBonusKindLuckyWheel, wheelKind)
	require.Equal(t, [3]int64{100, 100, 100}, wheelValues)
	require.Nil(t, wheelPlan)

	_, err = encodeSlotBonusRewards(service.SlotBonusKindLuckyWheel, [3]int64{50, 100, 200})
	require.Error(t, err)
}

func TestFindSlotBonusBySpinRestoresKind(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	expiresAt := time.Now().Add(time.Minute)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(gameSlotBonusFindBySpinQuery)).WithArgs(int64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "rewards", "expires_at", "status"}).
			AddRow(9, `{"kind":"lucky_wheel","values":[200,200,200]}`, expiresAt, "pending"))
	mock.ExpectRollback()

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	offer, err := findSlotBonusBySpin(context.Background(), tx, 17)
	require.NoError(t, err)
	require.Equal(t, service.SlotBonusKindLuckyWheel, offer.Kind)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPendingSlotBonusFiltersAndRestoresPersistedOffer(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC()
	expiresAt := now.Add(time.Minute)
	require.Contains(t, gameSlotBonusFindPendingQuery, "b.status = 'pending'")
	require.Contains(t, gameSlotBonusFindPendingQuery, "b.expires_at > $2")
	mock.ExpectQuery(regexp.QuoteMeta(gameSlotBonusFindPendingQuery)).WithArgs(int64(7), now).
		WillReturnRows(sqlmock.NewRows([]string{"id", "rewards", "expires_at", "status", "idempotency_key"}).
			AddRow(9, `{"kind":"lucky_wheel","values":[100,100,100]}`, expiresAt, "pending", "spin-pending"))

	repo := &gameLoyaltyRepository{db: db}
	pending, err := repo.GetPendingSlotBonus(context.Background(), 7, now)
	require.NoError(t, err)
	require.NotNil(t, pending)
	require.Equal(t, int64(9), pending.RoundID)
	require.Equal(t, service.SlotBonusKindLuckyWheel, pending.Kind)
	require.Equal(t, "spin-pending", pending.SpinIdempotencyKey)
	require.Equal(t, expiresAt, pending.ExpiresAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPendingSlotBonusReturnsNilWhenNoCurrentOfferExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta(gameSlotBonusFindPendingQuery)).WithArgs(int64(7), now).
		WillReturnRows(sqlmock.NewRows([]string{"id", "rewards", "expires_at", "status", "idempotency_key"}))

	repo := &gameLoyaltyRepository{db: db}
	pending, err := repo.GetPendingSlotBonus(context.Background(), 7, now)
	require.NoError(t, err)
	require.Nil(t, pending)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAllTimeLeaderboardQueriesTopThenCurrentUserRank(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta(gameAllTimeLeaderboardTopQuery)).
		WithArgs(service.GameIDSnake, 2).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "username", "email", "score", "achieved_at"}).
			AddRow(8, "first", "first@example.com", 900, now).
			AddRow(9, "second", "second@example.com", 800, now.Add(time.Second)))
	mock.ExpectQuery(regexp.QuoteMeta(gameAllTimeLeaderboardUserRankQuery)).
		WithArgs(service.GameIDSnake, int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"rank", "user_id", "username", "email", "score", "achieved_at"}).
			AddRow(42, 7, "me", "me@example.com", 100, now.Add(time.Minute)))

	repo := &gameLoyaltyRepository{db: db}
	items, err := repo.GetAllTimeLeaderboard(context.Background(), 7, service.GameIDSnake, 2)
	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, []int64{1, 2, 42}, []int64{items[0].Rank, items[1].Rank, items[2].Rank})
	require.Equal(t, int64(7), items[2].UserID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetAllTimeLeaderboardDoesNotQueryRankWhenCurrentUserIsInTop(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta(gameAllTimeLeaderboardTopQuery)).
		WithArgs(service.GameIDSnake, 2).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "username", "email", "score", "achieved_at"}).
			AddRow(8, "first", "first@example.com", 900, now).
			AddRow(7, "me", "me@example.com", 800, now.Add(time.Second)))

	repo := &gameLoyaltyRepository{db: db}
	items, err := repo.GetAllTimeLeaderboard(context.Background(), 7, service.GameIDSnake, 2)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, int64(2), items[1].Rank)
	require.Equal(t, int64(7), items[1].UserID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSpinLedgerIdempotencyKeysFitMaxExternalKey(t *testing.T) {
	externalKey := strings.Repeat("x", 128)
	betKey := gameLoyaltyLedgerIdempotencyKey("slot_bet", externalKey)
	payoutKey := gameLoyaltyLedgerIdempotencyKey("slot_payout", externalKey)

	require.LessOrEqual(t, len(betKey), 128)
	require.LessOrEqual(t, len(payoutKey), 128)
	require.Equal(t, betKey, gameLoyaltyLedgerIdempotencyKey("slot_bet", externalKey))
	require.NotEqual(t, betKey, payoutKey)
	require.NotContains(t, betKey, externalKey)
	require.NotContains(t, payoutKey, externalKey)
}

func TestGameLoyaltySpinInsufficientCreditsUnchanged(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyLockUserQuery)).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow("0"))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyFindSpinIdempotencyQuery)).WithArgs(int64(7), "spin-low").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec(regexp.QuoteMeta(gameLoyaltyEnsureQuery)).WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyLockWalletQuery)).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "credits", "free_spins_remaining"}).AddRow(1, 5, 0))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyCountSpinsTodayQuery)).WithArgs(int64(7), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyDeductCreditsQuery)).WithArgs(int64(7), int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"credits"}))
	mock.ExpectRollback()

	repo := &gameLoyaltyRepository{db: db}
	_, err = repo.Spin(context.Background(), 7, service.GameLoyaltySpinInput{
		IdempotencyKey: "spin-low", RequestHash: strings.Repeat("a", 64), LocalDate: "2026-07-28",
	}, 10, &service.SlotSpinResult{})
	require.ErrorIs(t, err, service.ErrGameLoyaltyInsufficientCredits)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGameCreditGiftMovesCreditsAndWritesPairedLedgerEntries(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	createdAt := time.Date(2026, 7, 31, 2, 0, 0, 0, time.UTC)
	requestHash := strings.Repeat("a", 64)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(gameCreditGiftFindTransferQuery)).
		WithArgs(int64(7), "gift-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(regexp.QuoteMeta(gameCreditGiftFindRecipientQuery)).
		WithArgs("friend@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).AddRow(8, "friend@example.com"))
	mock.ExpectQuery(regexp.QuoteMeta(gameCreditGiftLockUserQuery)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow("sender@example.com"))
	mock.ExpectQuery(regexp.QuoteMeta(gameCreditGiftLockUserQuery)).
		WithArgs(int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow("friend@example.com"))
	mock.ExpectQuery(regexp.QuoteMeta(gameCreditGiftFindTransferQuery)).
		WithArgs(int64(7), "gift-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec(regexp.QuoteMeta(gameLoyaltyEnsureQuery)).
		WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(gameLoyaltyEnsureQuery)).
		WithArgs(int64(8)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyLockWalletQuery)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "credits", "free_spins_remaining"}).AddRow(1, 100, 0))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyLockWalletQuery)).
		WithArgs(int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "credits", "free_spins_remaining"}).AddRow(2, 10, 0))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyDeductCreditsQuery)).
		WithArgs(int64(7), int64(25)).
		WillReturnRows(sqlmock.NewRows([]string{"credits"}).AddRow(75))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyAddCreditsQuery)).
		WithArgs(int64(8), int64(25)).
		WillReturnRows(sqlmock.NewRows([]string{"credits"}).AddRow(35))
	mock.ExpectQuery(regexp.QuoteMeta(gameCreditGiftInsertTransferQuery)).WithArgs(
		int64(7), int64(8), "friend@example.com", int64(25),
		int64(100), int64(75), int64(10), int64(35), "gift-1", requestHash,
	).WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(12, createdAt))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyInsertLedgerQuery)).WithArgs(
		int64(7), service.GameLedgerEntryGiftSent, int64(-25), int64(100), int64(75),
		"credit_gift", int64(12), "gift-1", requestHash,
		`{"recipient_email":"friend@example.com","recipient_user_id":8}`,
	).WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(21, createdAt))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyInsertLedgerQuery)).WithArgs(
		int64(8), service.GameLedgerEntryGiftReceived, int64(25), int64(10), int64(35),
		"credit_gift", int64(12), "gift-received-12", requestHash,
		`{"sender_user_id":7}`,
	).WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(22, createdAt))
	mock.ExpectCommit()

	repo := &gameLoyaltyRepository{db: db}
	result, err := repo.GiftCredits(context.Background(), 7, service.GameCreditGiftInput{
		RecipientEmail: "friend@example.com", Credits: 25,
		IdempotencyKey: "gift-1", RequestHash: requestHash,
	})
	require.NoError(t, err)
	require.Equal(t, int64(12), result.ID)
	require.Equal(t, int64(75), result.SenderCreditsAfter)
	require.Equal(t, int64(35), result.RecipientCreditsAfter)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGameCreditGiftIdempotentReplayDoesNotMoveCreditsAgain(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	createdAt := time.Date(2026, 7, 31, 2, 0, 0, 0, time.UTC)
	requestHash := strings.Repeat("b", 64)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(gameCreditGiftFindTransferQuery)).
		WithArgs(int64(7), "gift-replay").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "sender_user_id", "recipient_user_id", "recipient_email",
			"credits_amount", "sender_credits_before", "sender_credits_after",
			"recipient_credits_before", "recipient_credits_after",
			"idempotency_key", "request_hash", "created_at",
		}).AddRow(12, 7, 8, "friend@example.com", 25, 100, 75, 10, 35, "gift-replay", requestHash, createdAt))
	mock.ExpectCommit()

	repo := &gameLoyaltyRepository{db: db}
	result, err := repo.GiftCredits(context.Background(), 7, service.GameCreditGiftInput{
		RecipientEmail: "friend@example.com", Credits: 25,
		IdempotencyKey: "gift-replay", RequestHash: requestHash,
	})
	require.NoError(t, err)
	require.True(t, result.IdempotentReplay)
	require.Equal(t, int64(75), result.SenderCreditsAfter)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGameCreditGiftInsufficientSenderCreditsRollsBack(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	requestHash := strings.Repeat("c", 64)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(gameCreditGiftFindTransferQuery)).
		WithArgs(int64(7), "gift-low").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(regexp.QuoteMeta(gameCreditGiftFindRecipientQuery)).
		WithArgs("friend@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).AddRow(8, "friend@example.com"))
	mock.ExpectQuery(regexp.QuoteMeta(gameCreditGiftLockUserQuery)).
		WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow("sender@example.com"))
	mock.ExpectQuery(regexp.QuoteMeta(gameCreditGiftLockUserQuery)).
		WithArgs(int64(8)).WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow("friend@example.com"))
	mock.ExpectQuery(regexp.QuoteMeta(gameCreditGiftFindTransferQuery)).
		WithArgs(int64(7), "gift-low").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec(regexp.QuoteMeta(gameLoyaltyEnsureQuery)).
		WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(gameLoyaltyEnsureQuery)).
		WithArgs(int64(8)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyLockWalletQuery)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "credits", "free_spins_remaining"}).AddRow(1, 5, 0))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyLockWalletQuery)).
		WithArgs(int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "credits", "free_spins_remaining"}).AddRow(2, 10, 0))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyDeductCreditsQuery)).
		WithArgs(int64(7), int64(25)).WillReturnRows(sqlmock.NewRows([]string{"credits"}))
	mock.ExpectRollback()

	repo := &gameLoyaltyRepository{db: db}
	_, err = repo.GiftCredits(context.Background(), 7, service.GameCreditGiftInput{
		RecipientEmail: "friend@example.com", Credits: 25,
		IdempotencyKey: "gift-low", RequestHash: requestHash,
	})
	require.ErrorIs(t, err, service.ErrGameLoyaltyInsufficientCredits)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimSlotBonusExpired(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	now := time.Now().UTC()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyLockUserQuery)).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow("0"))
	mock.ExpectQuery(regexp.QuoteMeta(gameSlotBonusFindClaimReplayQuery)).WithArgs(int64(7), "claim-expired").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(regexp.QuoteMeta(gameSlotBonusLockByTokenQuery)).WithArgs(int64(7), strings.Repeat("b", 64)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "rewards", "expires_at"}).
			AddRow(9, "pending", `[50,100,200]`, now.Add(-time.Second)))
	mock.ExpectExec(regexp.QuoteMeta(gameSlotBonusExpireQuery)).WithArgs(int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &gameLoyaltyRepository{db: db}
	_, err = repo.ClaimSlotBonus(context.Background(), 7, service.SlotBonusClaimInput{
		TokenHash: strings.Repeat("b", 64), Choice: 0, IdempotencyKey: "claim-expired",
		RequestHash: strings.Repeat("c", 64), Now: now,
	})
	require.ErrorIs(t, err, service.ErrGameSlotBonusExpired)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimSlotBonusLegacyPendingArray(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	tokenHash := strings.Repeat("a", 64)
	requestHash := strings.Repeat("b", 64)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyLockUserQuery)).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow("0"))
	mock.ExpectQuery(regexp.QuoteMeta(gameSlotBonusFindClaimReplayQuery)).WithArgs(int64(7), "claim-old").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(regexp.QuoteMeta(gameSlotBonusLockByTokenQuery)).WithArgs(int64(7), tokenHash).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "rewards", "expires_at"}).
			AddRow(9, "pending", `[50,100,200]`, now.Add(time.Minute)))
	mock.ExpectExec(regexp.QuoteMeta(gameLoyaltyEnsureQuery)).WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyLockWalletQuery)).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "credits", "free_spins_remaining"}).AddRow(1, 1000, 0))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyAddCreditsQuery)).WithArgs(int64(7), int64(50)).
		WillReturnRows(sqlmock.NewRows([]string{"credits"}).AddRow(1050))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyInsertLedgerQuery)).WithArgs(
		int64(7), service.GameLedgerEntrySlotBonus, int64(50), int64(1000), int64(1050),
		"slot_bonus", int64(9), "slot_bonus:"+sha256String("claim-old"), requestHash,
		`{"choice":0,"kind":"pick_chest","round_id":9}`,
	).WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(21, now))
	mock.ExpectQuery(regexp.QuoteMeta(gameSlotBonusClaimQuery)).WithArgs(
		int64(9), 0, int64(50), now, "claim-old", requestHash,
	).WillReturnRows(sqlmock.NewRows([]string{"claimed_at"}).AddRow(now))
	mock.ExpectExec(regexp.QuoteMeta(gameDailyScoreUpsertQuery)).WithArgs(
		int64(7), "2026-07-29", service.GameIDLucky, int64(50), now,
	).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &gameLoyaltyRepository{db: db}
	result, err := repo.ClaimSlotBonus(context.Background(), 7, service.SlotBonusClaimInput{
		TokenHash: tokenHash, Choice: 0, IdempotencyKey: "claim-old", RequestHash: requestHash, Now: now,
	})
	require.NoError(t, err)
	require.Equal(t, service.SlotBonusKindPickChest, result.Kind)
	require.Equal(t, int64(50), result.RewardCredits)
	require.Equal(t, int64(1050), result.CreditsAfter)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimSlotBonusAlreadyClaimed(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyLockUserQuery)).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow("0"))
	mock.ExpectQuery(regexp.QuoteMeta(gameSlotBonusFindClaimReplayQuery)).WithArgs(int64(7), "claim-new-key").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(regexp.QuoteMeta(gameSlotBonusLockByTokenQuery)).WithArgs(int64(7), strings.Repeat("d", 64)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "rewards", "expires_at"}).
			AddRow(9, "claimed", `[50,100,200]`, time.Now().Add(time.Minute)))
	mock.ExpectRollback()

	repo := &gameLoyaltyRepository{db: db}
	_, err = repo.ClaimSlotBonus(context.Background(), 7, service.SlotBonusClaimInput{
		TokenHash: strings.Repeat("d", 64), Choice: 1, IdempotencyKey: "claim-new-key",
		RequestHash: strings.Repeat("e", 64), Now: time.Now(),
	})
	require.ErrorIs(t, err, service.ErrGameSlotBonusAlreadyClaimed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimSlotBonusIdempotentReplay(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	claimedAt := time.Now().UTC()
	hash := strings.Repeat("f", 64)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyLockUserQuery)).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow("0"))
	mock.ExpectQuery(regexp.QuoteMeta(gameSlotBonusFindClaimReplayQuery)).WithArgs(int64(7), "claim-replay").
		WillReturnRows(sqlmock.NewRows([]string{"id", "choice", "reward_credits", "credits_after", "claimed_at", "claim_request_hash", "rewards"}).
			AddRow(9, 2, 200, 1200, claimedAt, hash, `[50,100,200]`))
	mock.ExpectCommit()

	repo := &gameLoyaltyRepository{db: db}
	result, err := repo.ClaimSlotBonus(context.Background(), 7, service.SlotBonusClaimInput{
		IdempotencyKey: "claim-replay", RequestHash: hash,
	})
	require.NoError(t, err)
	require.True(t, result.IdempotentReplay)
	require.Equal(t, service.SlotBonusKindPickChest, result.Kind)
	require.Equal(t, int64(1200), result.CreditsAfter)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGameLoyaltyRedeemCodeInsertUsesGameTokenType(t *testing.T) {
	require.Contains(t, gameLoyaltyInsertRedeemCodeQuery, "$2")
	require.Contains(t, strings.ToLower(gameLoyaltyInsertRedeemCodeQuery), "redeem_codes")
	require.Contains(t, gameLoyaltyInsertClaimQuery, "voucher_value")
	require.Contains(t, gameLoyaltyInsertClaimQuery, "redeem_code_id")
	// Claim + redeem code share one TX in ClaimReward; both inserts exist.
	require.NotContains(t, gameLoyaltyAddCreditsQuery, "total_recharged")
	require.NotContains(t, gameLoyaltyDeductCreditsQuery, "total_recharged")
}

func TestGameLoyaltySpinBetLedgerUsesSignedAmount(t *testing.T) {
	require.Contains(t, gameLoyaltyInsertLedgerQuery, "entry_type")
	require.Contains(t, gameLoyaltyInsertLedgerQuery, "idempotency_key")
	require.Contains(t, gameLoyaltyInsertLedgerQuery, "$10::jsonb")
	require.NotContains(t, gameLoyaltyInsertLedgerQuery, "$10::text")
	require.Contains(t, gameLoyaltyInsertSpinQuery, "paytable_version")
	require.Contains(t, gameLoyaltyInsertSpinQuery, "reel_strip_version")
}

func TestGameLoyaltyRewardStockQueriesAreGlobalAndLocked(t *testing.T) {
	lowerCount := strings.ToLower(gameLoyaltyCountRewardClaimsTodayQuery)
	require.Contains(t, lowerCount, "reward_id = $1")
	require.Contains(t, lowerCount, "claim_local_date = $2")
	require.NotContains(t, lowerCount, "user_id")
	require.Contains(t, strings.ToLower(gameLoyaltyLockRewardStockQuery), "pg_advisory_xact_lock")
	require.NotContains(t, strings.ToLower(gameLoyaltyListGlobalClaimCountsQuery), "user_id")
}

func TestListRewardsStatusSeparatesUserClaimsFromGlobalStock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyListClaimCountsQuery)).
		WithArgs(int64(7), "2026-07-28").
		WillReturnRows(sqlmock.NewRows([]string{"reward_id", "count"}).AddRow("api-1", 1))
	mock.ExpectQuery(regexp.QuoteMeta(gameLoyaltyListGlobalClaimCountsQuery)).
		WithArgs("2026-07-28").
		WillReturnRows(sqlmock.NewRows([]string{"reward_id", "count"}).AddRow("api-1", 4))

	repo := &gameLoyaltyRepository{db: db}
	items, claimed, err := repo.ListRewardsStatus(context.Background(), 7, "2026-07-28", []service.GameLoyaltyRewardItem{{
		ID: "api-1", Title: "API 额度兑换券", CreditCost: 100, VoucherValue: "1", DailyStock: 5, Enabled: true,
	}}, 0)
	require.NoError(t, err)
	require.Equal(t, 1, claimed)
	require.Len(t, items, 1)
	require.Equal(t, 1, items[0].ClaimedToday)
	require.NotNil(t, items[0].RemainingStock)
	require.Equal(t, 1, *items[0].RemainingStock)
	require.NoError(t, mock.ExpectationsWereMet())
}
