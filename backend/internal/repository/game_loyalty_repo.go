package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	gameLoyaltyEnsureQuery = `
INSERT INTO game_wallets (user_id)
SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL
ON CONFLICT (user_id) DO NOTHING`

	gameLoyaltySnapshotQuery = `
SELECT u.id,
       u.balance::text,
       w.credits,
       w.free_spins_remaining,
       w.created_at,
       w.updated_at,
       c.id IS NOT NULL,
       c.created_at
FROM users u
JOIN game_wallets w ON w.user_id = u.id
LEFT JOIN game_daily_checkins c
  ON c.user_id = u.id AND c.local_date = $2::date
WHERE u.id = $1 AND u.deleted_at IS NULL`

	gameLoyaltyLockUserQuery = `
SELECT balance::text
FROM users
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE`

	gameLoyaltyLockWalletQuery = `
SELECT id, credits, free_spins_remaining
FROM game_wallets
WHERE user_id = $1
FOR UPDATE`

	gameLoyaltyAddCreditsQuery = `
UPDATE game_wallets
SET credits = credits + $2::bigint, updated_at = NOW()
WHERE user_id = $1
  AND credits <= 9223372036854775807::bigint - $2::bigint
RETURNING credits`

	gameLoyaltyDeductCreditsQuery = `
UPDATE game_wallets
SET credits = credits - $2::bigint, updated_at = NOW()
WHERE user_id = $1 AND credits >= $2::bigint
RETURNING credits`

	gameLoyaltySetFreeSpinsQuery = `
UPDATE game_wallets
SET free_spins_remaining = $2::int, updated_at = NOW()
WHERE user_id = $1 AND $2::int >= 0
RETURNING free_spins_remaining`

	gameLoyaltyInsertLedgerQuery = `
INSERT INTO game_credit_ledger (
    user_id, entry_type, amount, credits_before, credits_after,
    reference_type, reference_id, idempotency_key, request_hash, metadata
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, COALESCE($10::jsonb, '{}'::jsonb)
)
RETURNING id, created_at`

	gameLoyaltyFindLedgerIdempotencyQuery = `
SELECT id, user_id, entry_type, amount, credits_before, credits_after,
       reference_type, reference_id, idempotency_key, request_hash, metadata, created_at
FROM game_credit_ledger
WHERE user_id = $1 AND idempotency_key = $2`

	gameLoyaltyFindCheckinIdempotencyQuery = `
SELECT id, user_id, local_date::text, credits_awarded, credits_before, credits_after,
       idempotency_key, request_hash, created_at
FROM game_daily_checkins
WHERE user_id = $1 AND idempotency_key = $2`

	gameLoyaltyFindCheckinDateQuery = `
SELECT id, user_id, local_date::text, credits_awarded, credits_before, credits_after,
       idempotency_key, request_hash, created_at
FROM game_daily_checkins
WHERE user_id = $1 AND local_date = $2::date`

	gameLoyaltyInsertCheckinQuery = `
INSERT INTO game_daily_checkins (
    user_id, local_date, credits_awarded, credits_before, credits_after,
    ledger_id, idempotency_key, request_hash
) VALUES ($1, $2::date, $3, $4, $5, $6, $7, $8)
RETURNING id, created_at`

	gameLoyaltyFindSpinIdempotencyQuery = `
SELECT id, user_id, bet_credits, payout_credits, used_free_spin,
       grid, stops, winning_lines, scatter_count, free_spins_awarded,
       free_spins_before, free_spins_after, credits_before, credits_after,
       paytable_version, reel_strip_version, idempotency_key, request_hash, created_at
FROM game_slot_spins
WHERE user_id = $1 AND idempotency_key = $2`

	gameLoyaltyCountSpinsTodayQuery = `
	SELECT COUNT(*)
	FROM game_slot_spins
	WHERE user_id = $1
	  AND created_at >= $2
	  AND created_at < $3`

	gameLoyaltyInsertSpinQuery = `
INSERT INTO game_slot_spins (
    user_id, bet_credits, payout_credits, used_free_spin,
    grid, stops, winning_lines, scatter_count, free_spins_awarded,
    free_spins_before, free_spins_after, credits_before, credits_after,
    paytable_version, reel_strip_version, bet_ledger_id, payout_ledger_id,
    idempotency_key, request_hash
) VALUES (
    $1, $2, $3, $4,
    $5::jsonb, $6::jsonb, $7::jsonb, $8, $9,
    $10, $11, $12, $13,
    $14, $15, $16, $17,
    $18, $19
)
RETURNING id, created_at`

	gameLoyaltyFindClaimIdempotencyQuery = `
SELECT id, user_id, reward_id, title, credit_cost, voucher_value_text, redeem_code,
       claim_local_date::text, credits_before, credits_after, expires_at,
       idempotency_key, request_hash, created_at
FROM game_reward_claims
WHERE user_id = $1 AND idempotency_key = $2`

	gameLoyaltyCountUserClaimsTodayQuery = `
	SELECT COUNT(*)
	FROM game_reward_claims
	WHERE user_id = $1 AND claim_local_date = $2::date`

	gameLoyaltyLockRewardStockQuery = `
	SELECT pg_advisory_xact_lock(
	    hashtext('sub2api_game_reward_stock'),
	    hashtext($1 || E'\n' || $2)
	)`

	gameLoyaltyCountRewardClaimsTodayQuery = `
	SELECT COUNT(*)
	FROM game_reward_claims
	WHERE reward_id = $1 AND claim_local_date = $2::date`

	gameLoyaltyInsertClaimQuery = `
INSERT INTO game_reward_claims (
    user_id, reward_id, title, credit_cost, voucher_value, voucher_value_text,
    redeem_code, redeem_code_id, catalog_snapshot, claim_local_date, credits_before, credits_after,
    ledger_id, expires_at, idempotency_key, request_hash
) VALUES (
    $1, $2, $3, $4, $5::numeric, $5,
    $6, $7, $8::jsonb, $9::date, $10, $11,
    $12, $13, $14, $15
)
RETURNING id, created_at`

	gameLoyaltyInsertRedeemCodeQuery = `
INSERT INTO redeem_codes (
    code, type, value, status, notes, validity_days, expires_at, created_at
) VALUES (
    $1, $2, $3::numeric, 'unused', $4, $5, $6, NOW()
)
RETURNING id`

	gameLoyaltyListLedgerQuery = `
SELECT id, user_id, entry_type, amount, credits_before, credits_after,
       reference_type, reference_id, idempotency_key, metadata, created_at
FROM game_credit_ledger
WHERE user_id = $1 AND ($3::bigint = 0 OR id < $3::bigint)
ORDER BY id DESC
LIMIT $2`

	gameLoyaltyListClaimCountsQuery = `
	SELECT reward_id, COUNT(*)
	FROM game_reward_claims
	WHERE user_id = $1 AND claim_local_date = $2::date
	GROUP BY reward_id`

	gameLoyaltyListGlobalClaimCountsQuery = `
		SELECT reward_id, COUNT(*)
		FROM game_reward_claims
		WHERE claim_local_date = $1::date
		GROUP BY reward_id`

	gameCreditGiftFindRecipientQuery = `
	SELECT id, TRIM(email)
	FROM users
	WHERE LOWER(TRIM(email)) = $1
	  AND deleted_at IS NULL
	ORDER BY id
	LIMIT 1`

	gameCreditGiftLockUserQuery = `
	SELECT TRIM(email)
	FROM users
	WHERE id = $1 AND deleted_at IS NULL
	FOR UPDATE`

	gameCreditGiftFindTransferQuery = `
	SELECT id, sender_user_id, recipient_user_id, recipient_email,
	       credits_amount, sender_credits_before, sender_credits_after,
	       recipient_credits_before, recipient_credits_after,
	       idempotency_key, request_hash, created_at
	FROM game_credit_transfers
	WHERE sender_user_id = $1 AND idempotency_key = $2`

	gameCreditGiftInsertTransferQuery = `
	INSERT INTO game_credit_transfers (
	    sender_user_id, recipient_user_id, recipient_email, credits_amount,
	    sender_credits_before, sender_credits_after,
	    recipient_credits_before, recipient_credits_after,
	    idempotency_key, request_hash
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	RETURNING id, created_at`
)

const (
	gameLoyaltyMaxDailySpins  = 5000
	gameLoyaltyMaxDailyClaims = 1000
)

type gameLoyaltyRepository struct {
	db *sql.DB
}

var _ service.GameLoyaltyRepository = (*gameLoyaltyRepository)(nil)
var _ service.GameCreditGiftRepository = (*gameLoyaltyRepository)(nil)

func NewGameLoyaltyRepository(db *sql.DB) service.GameLoyaltyRepository {
	return &gameLoyaltyRepository{db: db}
}

func (r *gameLoyaltyRepository) GetSnapshot(ctx context.Context, userID int64, localDate string) (*service.GameLoyaltyWalletSnapshot, error) {
	if _, err := r.db.ExecContext(ctx, gameLoyaltyEnsureQuery, userID); err != nil {
		return nil, fmt.Errorf("ensure game wallet: %w", err)
	}
	snapshot := &service.GameLoyaltyWalletSnapshot{LocalDate: localDate}
	var checkinAt sql.NullTime
	err := r.db.QueryRowContext(ctx, gameLoyaltySnapshotQuery, userID, localDate).Scan(
		&snapshot.UserID,
		&snapshot.AccountBalance,
		&snapshot.Credits,
		&snapshot.FreeSpinsRemaining,
		&snapshot.WalletCreatedAt,
		&snapshot.WalletUpdatedAt,
		&snapshot.CheckedInToday,
		&checkinAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGameLoyaltyUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get game loyalty snapshot: %w", err)
	}
	if checkinAt.Valid {
		t := checkinAt.Time
		snapshot.TodayCheckinAt = &t
	}
	return snapshot, nil
}

func (r *gameLoyaltyRepository) CheckIn(
	ctx context.Context,
	userID int64,
	localDate string,
	credits int64,
	idempotencyKey, requestHash string,
) (*service.GameLoyaltyCheckinResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin check-in: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockGameLoyaltyUser(ctx, tx, userID); err != nil {
		return nil, err
	}

	if existing, existingHash, err := findCheckinIdempotency(ctx, tx, userID, idempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		if existingHash != requestHash {
			return nil, service.ErrGameLoyaltyIdempotencyConflict
		}
		existing.IdempotentReplay = true
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit check-in replay: %w", err)
		}
		return existing, nil
	}

	if existingDate, err := findCheckinByDate(ctx, tx, userID, localDate); err != nil {
		return nil, err
	} else if existingDate != nil {
		return nil, service.ErrGameLoyaltyCheckinAlreadyClaimed
	}

	if _, err := tx.ExecContext(ctx, gameLoyaltyEnsureQuery, userID); err != nil {
		return nil, fmt.Errorf("ensure game wallet: %w", err)
	}
	creditsBefore, freeSpins, err := lockGameLoyaltyWallet(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	_ = freeSpins

	var creditsAfter int64
	if err := tx.QueryRowContext(ctx, gameLoyaltyAddCreditsQuery, userID, credits).Scan(&creditsAfter); errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGameLoyaltyCreditsLimitExceeded
	} else if err != nil {
		return nil, fmt.Errorf("add check-in credits: %w", err)
	}

	meta, _ := json.Marshal(map[string]any{
		"local_date": localDate,
		"source":     "checkin",
	})
	ledgerID, _, err := insertLedger(ctx, tx, userID, service.GameLedgerEntryCheckin, credits, creditsBefore, creditsAfter, "checkin", nil, idempotencyKey, requestHash, string(meta))
	if err != nil {
		return nil, err
	}

	result := &service.GameLoyaltyCheckinResult{
		UserID:         userID,
		LocalDate:      localDate,
		CreditsAwarded: credits,
		CreditsBefore:  creditsBefore,
		CreditsAfter:   creditsAfter,
		IdempotencyKey: idempotencyKey,
	}
	if err := tx.QueryRowContext(
		ctx,
		gameLoyaltyInsertCheckinQuery,
		userID, localDate, credits, creditsBefore, creditsAfter,
		ledgerID, idempotencyKey, requestHash,
	).Scan(&result.ID, &result.CreatedAt); err != nil {
		if isGameLoyaltyUniqueViolation(err) {
			return nil, service.ErrGameLoyaltyCheckinAlreadyClaimed
		}
		return nil, fmt.Errorf("insert check-in: %w", err)
	}
	// Link ledger reference to check-in id is best-effort via metadata only;
	// ledger rows are immutable so reference_id stays optional at insert time.
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit check-in: %w", err)
	}
	return result, nil
}

func (r *gameLoyaltyRepository) Spin(
	ctx context.Context,
	userID int64,
	input service.GameLoyaltySpinInput,
	configuredBet int64,
	outcome *service.SlotSpinResult,
) (*service.GameLoyaltySpinResult, error) {
	if outcome == nil {
		return nil, fmt.Errorf("spin outcome is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin spin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockGameLoyaltyUser(ctx, tx, userID); err != nil {
		return nil, err
	}
	if existing, existingHash, err := findSpinIdempotency(ctx, tx, userID, input.IdempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		if existingHash != input.RequestHash {
			return nil, service.ErrGameLoyaltyIdempotencyConflict
		}
		existing.BonusRound, err = findSlotBonusBySpin(ctx, tx, existing.ID)
		if err != nil {
			return nil, err
		}
		existing.IdempotentReplay = true
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit spin replay: %w", err)
		}
		return existing, nil
	}

	if _, err := tx.ExecContext(ctx, gameLoyaltyEnsureQuery, userID); err != nil {
		return nil, fmt.Errorf("ensure game wallet: %w", err)
	}
	creditsBefore, freeBefore, err := lockGameLoyaltyWallet(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	dayStart := timezone.Today()
	dayEnd := dayStart.AddDate(0, 0, 1)
	var spinCount int64
	if err := tx.QueryRowContext(ctx, gameLoyaltyCountSpinsTodayQuery, userID, dayStart, dayEnd).Scan(&spinCount); err != nil {
		return nil, fmt.Errorf("count daily spins: %w", err)
	}
	if spinCount >= gameLoyaltyMaxDailySpins {
		return nil, service.ErrGameLoyaltySpinLimitExceeded
	}

	usedFree := freeBefore > 0
	betCredits := configuredBet
	if input.BetCredits > 0 {
		// Client requested an explicit paid bet; free spins stay server-owned and
		// are not spent when the client forces the configured paid bet path.
		usedFree = false
		betCredits = input.BetCredits
	}
	if usedFree {
		betCredits = configuredBet // free spin still evaluates at configured bet
	}

	creditsAfterBet := creditsBefore
	var betLedgerID sql.NullInt64
	if !usedFree {
		if err := tx.QueryRowContext(ctx, gameLoyaltyDeductCreditsQuery, userID, betCredits).Scan(&creditsAfterBet); errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrGameLoyaltyInsufficientCredits
		} else if err != nil {
			return nil, fmt.Errorf("deduct spin bet: %w", err)
		}
		meta, _ := json.Marshal(map[string]any{"bet": betCredits, "used_free_spin": false})
		id, _, err := insertLedger(ctx, tx, userID, service.GameLedgerEntrySlotBet, -betCredits, creditsBefore, creditsAfterBet, "spin", nil, gameLoyaltyLedgerIdempotencyKey("slot_bet", input.IdempotencyKey), input.RequestHash, string(meta))
		if err != nil {
			return nil, err
		}
		betLedgerID = sql.NullInt64{Int64: id, Valid: true}
	}

	freeAfter := freeBefore
	if usedFree {
		freeAfter = freeBefore - 1
	}
	if outcome.FreeSpinsAwarded > 0 {
		sum := freeAfter + outcome.FreeSpinsAwarded
		if sum < freeAfter || sum > service.SlotMaxFreeSpinsCap() {
			return nil, service.ErrGameLoyaltyCreditsLimitExceeded
		}
		freeAfter = sum
	}
	if _, err := tx.ExecContext(ctx, gameLoyaltySetFreeSpinsQuery, userID, freeAfter); err != nil {
		return nil, fmt.Errorf("update free spins: %w", err)
	}

	creditsAfter := creditsAfterBet
	var payoutLedgerID sql.NullInt64
	if outcome.TotalPayout > 0 {
		if err := tx.QueryRowContext(ctx, gameLoyaltyAddCreditsQuery, userID, outcome.TotalPayout).Scan(&creditsAfter); errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrGameLoyaltyCreditsLimitExceeded
		} else if err != nil {
			return nil, fmt.Errorf("add spin payout: %w", err)
		}
		meta, _ := json.Marshal(map[string]any{
			"payout":             outcome.TotalPayout,
			"scatter_count":      outcome.ScatterCount,
			"bonus_count":        outcome.BonusCount,
			"free_spins_awarded": outcome.FreeSpinsAwarded,
		})
		id, _, err := insertLedger(ctx, tx, userID, service.GameLedgerEntrySlotPayout, outcome.TotalPayout, creditsAfterBet, creditsAfter, "spin", nil, gameLoyaltyLedgerIdempotencyKey("slot_payout", input.IdempotencyKey), input.RequestHash, string(meta))
		if err != nil {
			return nil, err
		}
		payoutLedgerID = sql.NullInt64{Int64: id, Valid: true}
	}

	gridJSON, err := json.Marshal(outcome.Grid)
	if err != nil {
		return nil, fmt.Errorf("marshal grid: %w", err)
	}
	stopsJSON, err := json.Marshal(outcome.Stops)
	if err != nil {
		return nil, fmt.Errorf("marshal stops: %w", err)
	}
	lines := outcome.WinningLines
	if lines == nil {
		lines = []service.SlotWinningLine{}
	}
	linesJSON, err := json.Marshal(lines)
	if err != nil {
		return nil, fmt.Errorf("marshal lines: %w", err)
	}

	result := &service.GameLoyaltySpinResult{
		UserID:           userID,
		BetCredits:       betCredits,
		PayoutCredits:    outcome.TotalPayout,
		UsedFreeSpin:     usedFree,
		Grid:             outcome.Grid,
		Stops:            outcome.Stops,
		WinningLines:     lines,
		ScatterCount:     outcome.ScatterCount,
		BonusCount:       outcome.BonusCount,
		FreeSpinsAwarded: outcome.FreeSpinsAwarded,
		FreeSpinsBefore:  freeBefore,
		FreeSpinsAfter:   freeAfter,
		CreditsBefore:    creditsBefore,
		CreditsAfter:     creditsAfter,
		PaytableVersion:  outcome.PaytableVersion,
		ReelStripVersion: outcome.ReelStripVersion,
		IdempotencyKey:   input.IdempotencyKey,
	}

	var betLedgerArg any
	if betLedgerID.Valid {
		betLedgerArg = betLedgerID.Int64
	}
	var payoutLedgerArg any
	if payoutLedgerID.Valid {
		payoutLedgerArg = payoutLedgerID.Int64
	}

	if err := tx.QueryRowContext(
		ctx,
		gameLoyaltyInsertSpinQuery,
		userID,
		result.BetCredits,
		result.PayoutCredits,
		result.UsedFreeSpin,
		string(gridJSON),
		string(stopsJSON),
		string(linesJSON),
		result.ScatterCount,
		result.FreeSpinsAwarded,
		result.FreeSpinsBefore,
		result.FreeSpinsAfter,
		result.CreditsBefore,
		result.CreditsAfter,
		result.PaytableVersion,
		result.ReelStripVersion,
		betLedgerArg,
		payoutLedgerArg,
		input.IdempotencyKey,
		input.RequestHash,
	).Scan(&result.ID, &result.CreatedAt); err != nil {
		return nil, fmt.Errorf("insert spin: %w", err)
	}
	localDate := input.LocalDate
	if localDate == "" {
		localDate = timezone.Today().Format("2006-01-02")
	}
	if _, err := tx.ExecContext(
		ctx, gameDailyScoreUpsertQuery, userID, localDate,
		service.GameIDLucky, result.PayoutCredits, result.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("upsert lucky spin score: %w", err)
	}
	if result.BonusCount >= service.SlotBonusTriggerMin && input.BonusRound != nil {
		rewardsJSON, err := encodeSlotBonusOffer(input.BonusRound)
		if err != nil {
			return nil, fmt.Errorf("marshal slot bonus rewards: %w", err)
		}
		offer := &service.SlotBonusRoundOffer{
			Kind: input.BonusRound.Kind, Token: input.BonusRound.Token, Choices: 3,
		}
		if input.BonusRound.Plan != nil {
			offer.BetCredits = input.BonusRound.Plan.BetCredits
			offer.BonusCount = input.BonusRound.Plan.BonusCount
			offer.BonusMultiplier = input.BonusRound.Plan.BonusMultiplier
			offer.Options = slotBonusOptionViews(input.BonusRound.Plan.Options)
		}
		err = tx.QueryRowContext(
			ctx, gameSlotBonusInsertQuery, userID, result.ID, localDate,
			input.BonusRound.TokenHash, string(rewardsJSON), input.BonusRound.ExpiresAt,
		).Scan(&offer.RoundID, &offer.ExpiresAt, &offer.Status)
		if err == nil {
			result.BonusRound = offer
		} else if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("insert slot bonus round returned no row")
		} else {
			return nil, fmt.Errorf("insert slot bonus round: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit spin: %w", err)
	}
	return result, nil
}

func (r *gameLoyaltyRepository) ListRewardsStatus(
	ctx context.Context,
	userID int64,
	localDate string,
	catalog []service.GameLoyaltyRewardItem,
	dailyLimit int,
) ([]service.GameLoyaltyRewardView, int, error) {
	rows, err := r.db.QueryContext(ctx, gameLoyaltyListClaimCountsQuery, userID, localDate)
	if err != nil {
		return nil, 0, fmt.Errorf("list reward claim counts: %w", err)
	}
	defer rows.Close()

	userCounts := map[string]int{}
	total := 0
	for rows.Next() {
		var rewardID string
		var count int
		if err := rows.Scan(&rewardID, &count); err != nil {
			return nil, 0, fmt.Errorf("scan reward claim count: %w", err)
		}
		userCounts[rewardID] = count
		total += count
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	globalRows, err := r.db.QueryContext(ctx, gameLoyaltyListGlobalClaimCountsQuery, localDate)
	if err != nil {
		return nil, 0, fmt.Errorf("list global reward claim counts: %w", err)
	}
	defer globalRows.Close()

	globalCounts := map[string]int{}
	for globalRows.Next() {
		var rewardID string
		var count int
		if err := globalRows.Scan(&rewardID, &count); err != nil {
			return nil, 0, fmt.Errorf("scan global reward claim count: %w", err)
		}
		globalCounts[rewardID] = count
	}
	if err := globalRows.Err(); err != nil {
		return nil, 0, err
	}

	views := make([]service.GameLoyaltyRewardView, 0, len(catalog))
	for _, item := range catalog {
		if !item.Enabled {
			continue
		}
		claimed := userCounts[item.ID]
		view := service.GameLoyaltyRewardView{
			ID:           item.ID,
			Title:        item.Title,
			CreditCost:   item.CreditCost,
			VoucherValue: item.VoucherValue,
			DailyStock:   item.DailyStock,
			ExpiryDays:   item.ExpiryDays,
			ClaimedToday: claimed,
		}
		if item.DailyStock > 0 {
			remaining := item.DailyStock - globalCounts[item.ID]
			if remaining < 0 {
				remaining = 0
			}
			view.RemainingStock = &remaining
		}
		if dailyLimit > 0 {
			globalRemaining := dailyLimit - total
			if globalRemaining < 0 {
				globalRemaining = 0
			}
			if view.RemainingStock == nil || globalRemaining < *view.RemainingStock {
				// Surface the tighter cap without mutating stock semantics when stock is unlimited.
				if item.DailyStock > 0 {
					r := globalRemaining
					if *view.RemainingStock < r {
						r = *view.RemainingStock
					}
					view.RemainingStock = &r
				}
			}
		}
		views = append(views, view)
	}
	return views, total, nil
}

func (r *gameLoyaltyRepository) ClaimReward(
	ctx context.Context,
	userID int64,
	localDate string,
	item service.GameLoyaltyRewardItem,
	dailyLimit int,
	redeemCode string,
	expiresAt *time.Time,
	input service.GameLoyaltyRewardClaimInput,
) (*service.GameLoyaltyRewardClaimResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin reward claim: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockGameLoyaltyUser(ctx, tx, userID); err != nil {
		return nil, err
	}
	if existing, existingHash, err := findClaimIdempotency(ctx, tx, userID, input.IdempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		if existingHash != input.RequestHash {
			return nil, service.ErrGameLoyaltyIdempotencyConflict
		}
		existing.IdempotentReplay = true
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit claim replay: %w", err)
		}
		return existing, nil
	}

	if _, err := tx.ExecContext(ctx, gameLoyaltyEnsureQuery, userID); err != nil {
		return nil, fmt.Errorf("ensure game wallet: %w", err)
	}
	creditsBefore, _, err := lockGameLoyaltyWallet(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	var totalClaims int
	if err := tx.QueryRowContext(ctx, gameLoyaltyCountUserClaimsTodayQuery, userID, localDate).Scan(&totalClaims); err != nil {
		return nil, fmt.Errorf("count user reward claims: %w", err)
	}
	if totalClaims >= gameLoyaltyMaxDailyClaims {
		return nil, service.ErrGameLoyaltyRewardCapExceeded
	}
	if dailyLimit > 0 && totalClaims >= dailyLimit {
		return nil, service.ErrGameLoyaltyRewardCapExceeded
	}
	if item.DailyStock > 0 {
		var lockResult any
		if err := tx.QueryRowContext(ctx, gameLoyaltyLockRewardStockQuery, item.ID, localDate).Scan(&lockResult); err != nil {
			return nil, fmt.Errorf("lock reward stock: %w", err)
		}
		var itemClaims int
		if err := tx.QueryRowContext(ctx, gameLoyaltyCountRewardClaimsTodayQuery, item.ID, localDate).Scan(&itemClaims); err != nil {
			return nil, fmt.Errorf("count global reward claims: %w", err)
		}
		if itemClaims >= item.DailyStock {
			return nil, service.ErrGameLoyaltyRewardStockExceeded
		}
	}

	var creditsAfter int64
	if err := tx.QueryRowContext(ctx, gameLoyaltyDeductCreditsQuery, userID, item.CreditCost).Scan(&creditsAfter); errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGameLoyaltyInsufficientCredits
	} else if err != nil {
		return nil, fmt.Errorf("deduct reward credits: %w", err)
	}

	snapshot, err := json.Marshal(map[string]any{
		"id":            item.ID,
		"title":         item.Title,
		"credit_cost":   fmt.Sprintf("%d", item.CreditCost),
		"voucher_value": item.VoucherValue,
		"daily_stock":   item.DailyStock,
		"expiry_days":   item.ExpiryDays,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal catalog snapshot: %w", err)
	}

	meta, _ := json.Marshal(map[string]any{
		"reward_id":     item.ID,
		"voucher_value": item.VoucherValue,
		"redeem_code":   redeemCode,
	})
	ledgerID, _, err := insertLedger(
		ctx, tx, userID, service.GameLedgerEntryRewardClaim,
		-item.CreditCost, creditsBefore, creditsAfter,
		"reward_claim", nil, input.IdempotencyKey, input.RequestHash, string(meta),
	)
	if err != nil {
		return nil, err
	}

	validityDays := item.ExpiryDays
	if validityDays <= 0 {
		validityDays = 0
	}
	notes := "game loyalty reward: " + item.ID
	var expires any
	if expiresAt != nil {
		expires = *expiresAt
	}
	// Insert redeem code and claim in the same transaction.
	var redeemID int64
	if err := tx.QueryRowContext(
		ctx,
		gameLoyaltyInsertRedeemCodeQuery,
		redeemCode,
		service.RedeemTypeGameToken,
		item.VoucherValue, // exact decimal string bound as numeric
		notes,
		validityDays,
		expires,
	).Scan(&redeemID); err != nil {
		return nil, fmt.Errorf("insert game_token redeem code: %w", err)
	}
	result := &service.GameLoyaltyRewardClaimResult{
		UserID:         userID,
		RewardID:       item.ID,
		Title:          item.Title,
		CreditCost:     item.CreditCost,
		VoucherValue:   item.VoucherValue,
		RedeemCode:     redeemCode,
		ClaimLocalDate: localDate,
		CreditsBefore:  creditsBefore,
		CreditsAfter:   creditsAfter,
		ExpiresAt:      expiresAt,
		IdempotencyKey: input.IdempotencyKey,
	}
	if err := tx.QueryRowContext(
		ctx,
		gameLoyaltyInsertClaimQuery,
		userID,
		item.ID,
		item.Title,
		item.CreditCost,
		item.VoucherValue,
		redeemCode,
		redeemID,
		string(snapshot),
		localDate,
		creditsBefore,
		creditsAfter,
		ledgerID,
		expires,
		input.IdempotencyKey,
		input.RequestHash,
	).Scan(&result.ID, &result.CreatedAt); err != nil {
		return nil, fmt.Errorf("insert reward claim: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit reward claim: %w", err)
	}
	return result, nil
}

func (r *gameLoyaltyRepository) GiftCredits(
	ctx context.Context,
	senderUserID int64,
	input service.GameCreditGiftInput,
) (*service.GameCreditGiftResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin game credit gift: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if existing, existingHash, err := findGameCreditGift(ctx, tx, senderUserID, input.IdempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		if existingHash != input.RequestHash {
			return nil, service.ErrGameLoyaltyIdempotencyConflict
		}
		existing.IdempotentReplay = true
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit game credit gift replay: %w", err)
		}
		return existing, nil
	}

	var recipientUserID int64
	var recipientEmail string
	if err := tx.QueryRowContext(ctx, gameCreditGiftFindRecipientQuery, input.RecipientEmail).Scan(
		&recipientUserID, &recipientEmail,
	); errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGameCreditGiftRecipientNotFound
	} else if err != nil {
		return nil, fmt.Errorf("find game credit gift recipient: %w", err)
	}
	if recipientUserID == senderUserID {
		return nil, service.ErrGameCreditGiftSelfForbidden
	}

	firstUserID, secondUserID := senderUserID, recipientUserID
	if firstUserID > secondUserID {
		firstUserID, secondUserID = secondUserID, firstUserID
	}
	lockedEmails := make(map[int64]string, 2)
	for _, userID := range []int64{firstUserID, secondUserID} {
		var email string
		if err := tx.QueryRowContext(ctx, gameCreditGiftLockUserQuery, userID).Scan(&email); errors.Is(err, sql.ErrNoRows) {
			if userID == senderUserID {
				return nil, service.ErrGameLoyaltyUserNotFound
			}
			return nil, service.ErrGameCreditGiftRecipientNotFound
		} else if err != nil {
			return nil, fmt.Errorf("lock game credit gift user %d: %w", userID, err)
		}
		lockedEmails[userID] = email
	}
	if strings.ToLower(strings.TrimSpace(lockedEmails[recipientUserID])) != input.RecipientEmail {
		return nil, service.ErrGameCreditGiftRecipientNotFound
	}
	recipientEmail = lockedEmails[recipientUserID]

	// The sender lock serializes equal idempotency keys. Recheck after waiting.
	if existing, existingHash, err := findGameCreditGift(ctx, tx, senderUserID, input.IdempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		if existingHash != input.RequestHash {
			return nil, service.ErrGameLoyaltyIdempotencyConflict
		}
		existing.IdempotentReplay = true
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit game credit gift replay: %w", err)
		}
		return existing, nil
	}

	for _, userID := range []int64{firstUserID, secondUserID} {
		if _, err := tx.ExecContext(ctx, gameLoyaltyEnsureQuery, userID); err != nil {
			return nil, fmt.Errorf("ensure game credit gift wallet %d: %w", userID, err)
		}
	}
	walletCredits := make(map[int64]int64, 2)
	for _, userID := range []int64{firstUserID, secondUserID} {
		credits, _, err := lockGameLoyaltyWallet(ctx, tx, userID)
		if err != nil {
			return nil, err
		}
		walletCredits[userID] = credits
	}

	result := &service.GameCreditGiftResult{
		SenderUserID:           senderUserID,
		RecipientUserID:        recipientUserID,
		RecipientEmail:         recipientEmail,
		Credits:                input.Credits,
		SenderCreditsBefore:    walletCredits[senderUserID],
		RecipientCreditsBefore: walletCredits[recipientUserID],
		IdempotencyKey:         input.IdempotencyKey,
	}
	if err := tx.QueryRowContext(ctx, gameLoyaltyDeductCreditsQuery, senderUserID, input.Credits).Scan(
		&result.SenderCreditsAfter,
	); errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGameLoyaltyInsufficientCredits
	} else if err != nil {
		return nil, fmt.Errorf("deduct gifted game credits: %w", err)
	}
	if err := tx.QueryRowContext(ctx, gameLoyaltyAddCreditsQuery, recipientUserID, input.Credits).Scan(
		&result.RecipientCreditsAfter,
	); errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGameLoyaltyCreditsLimitExceeded
	} else if err != nil {
		return nil, fmt.Errorf("add gifted game credits: %w", err)
	}

	if err := tx.QueryRowContext(
		ctx, gameCreditGiftInsertTransferQuery,
		result.SenderUserID, result.RecipientUserID, result.RecipientEmail, result.Credits,
		result.SenderCreditsBefore, result.SenderCreditsAfter,
		result.RecipientCreditsBefore, result.RecipientCreditsAfter,
		input.IdempotencyKey, input.RequestHash,
	).Scan(&result.ID, &result.CreatedAt); err != nil {
		return nil, fmt.Errorf("insert game credit gift: %w", err)
	}

	sentMeta, _ := json.Marshal(map[string]any{
		"recipient_user_id": result.RecipientUserID,
		"recipient_email":   result.RecipientEmail,
	})
	receivedMeta, _ := json.Marshal(map[string]any{
		"sender_user_id": result.SenderUserID,
	})
	if _, _, err := insertLedger(
		ctx, tx, senderUserID, service.GameLedgerEntryGiftSent,
		-input.Credits, result.SenderCreditsBefore, result.SenderCreditsAfter,
		"credit_gift", &result.ID, input.IdempotencyKey, input.RequestHash, string(sentMeta),
	); err != nil {
		return nil, err
	}
	receivedKey := fmt.Sprintf("gift-received-%d", result.ID)
	if _, _, err := insertLedger(
		ctx, tx, recipientUserID, service.GameLedgerEntryGiftReceived,
		input.Credits, result.RecipientCreditsBefore, result.RecipientCreditsAfter,
		"credit_gift", &result.ID, receivedKey, input.RequestHash, string(receivedMeta),
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit game credit gift: %w", err)
	}
	return result, nil
}

func (r *gameLoyaltyRepository) ListLedger(ctx context.Context, userID int64, limit int, beforeID int64) ([]service.GameLoyaltyLedgerEntry, error) {
	rows, err := r.db.QueryContext(ctx, gameLoyaltyListLedgerQuery, userID, limit, beforeID)
	if err != nil {
		return nil, fmt.Errorf("list game credit ledger: %w", err)
	}
	defer rows.Close()

	items := make([]service.GameLoyaltyLedgerEntry, 0, limit)
	for rows.Next() {
		var item service.GameLoyaltyLedgerEntry
		var refID sql.NullInt64
		var meta []byte
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.EntryType,
			&item.Amount,
			&item.CreditsBefore,
			&item.CreditsAfter,
			&item.ReferenceType,
			&refID,
			&item.IdempotencyKey,
			&meta,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ledger entry: %w", err)
		}
		if refID.Valid {
			v := refID.Int64
			item.ReferenceID = &v
		}
		item.Metadata = json.RawMessage(meta)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func findGameCreditGift(
	ctx context.Context,
	tx *sql.Tx,
	senderUserID int64,
	idempotencyKey string,
) (*service.GameCreditGiftResult, string, error) {
	item := &service.GameCreditGiftResult{}
	var requestHash string
	err := tx.QueryRowContext(ctx, gameCreditGiftFindTransferQuery, senderUserID, idempotencyKey).Scan(
		&item.ID, &item.SenderUserID, &item.RecipientUserID, &item.RecipientEmail,
		&item.Credits, &item.SenderCreditsBefore, &item.SenderCreditsAfter,
		&item.RecipientCreditsBefore, &item.RecipientCreditsAfter,
		&item.IdempotencyKey, &requestHash, &item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("query game credit gift idempotency: %w", err)
	}
	return item, requestHash, nil
}

func lockGameLoyaltyUser(ctx context.Context, tx *sql.Tx, userID int64) error {
	var balance string
	if err := tx.QueryRowContext(ctx, gameLoyaltyLockUserQuery, userID).Scan(&balance); errors.Is(err, sql.ErrNoRows) {
		return service.ErrGameLoyaltyUserNotFound
	} else if err != nil {
		return fmt.Errorf("lock user: %w", err)
	}
	return nil
}

func lockGameLoyaltyWallet(ctx context.Context, tx *sql.Tx, userID int64) (credits int64, freeSpins int, err error) {
	var walletID int64
	if err := tx.QueryRowContext(ctx, gameLoyaltyLockWalletQuery, userID).Scan(&walletID, &credits, &freeSpins); err != nil {
		return 0, 0, fmt.Errorf("lock game wallet: %w", err)
	}
	return credits, freeSpins, nil
}

func insertLedger(
	ctx context.Context,
	tx *sql.Tx,
	userID int64,
	entryType string,
	amount, before, after int64,
	refType string,
	refID *int64,
	idempotencyKey, requestHash, metadata string,
) (int64, time.Time, error) {
	var id int64
	var created time.Time
	var ref any
	if refID != nil {
		ref = *refID
	}
	if metadata == "" {
		metadata = "{}"
	}
	if err := tx.QueryRowContext(
		ctx,
		gameLoyaltyInsertLedgerQuery,
		userID, entryType, amount, before, after,
		refType, ref, idempotencyKey, requestHash, metadata,
	).Scan(&id, &created); err != nil {
		return 0, time.Time{}, fmt.Errorf("insert ledger %s: %w", entryType, err)
	}
	return id, created, nil
}

func findCheckinIdempotency(ctx context.Context, tx *sql.Tx, userID int64, key string) (*service.GameLoyaltyCheckinResult, string, error) {
	item := &service.GameLoyaltyCheckinResult{}
	var requestHash string
	err := tx.QueryRowContext(ctx, gameLoyaltyFindCheckinIdempotencyQuery, userID, key).Scan(
		&item.ID, &item.UserID, &item.LocalDate, &item.CreditsAwarded,
		&item.CreditsBefore, &item.CreditsAfter, &item.IdempotencyKey, &requestHash, &item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("query check-in idempotency: %w", err)
	}
	return item, requestHash, nil
}

func findCheckinByDate(ctx context.Context, tx *sql.Tx, userID int64, localDate string) (*service.GameLoyaltyCheckinResult, error) {
	item := &service.GameLoyaltyCheckinResult{}
	var requestHash string
	err := tx.QueryRowContext(ctx, gameLoyaltyFindCheckinDateQuery, userID, localDate).Scan(
		&item.ID, &item.UserID, &item.LocalDate, &item.CreditsAwarded,
		&item.CreditsBefore, &item.CreditsAfter, &item.IdempotencyKey, &requestHash, &item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query check-in by date: %w", err)
	}
	return item, nil
}

func findSpinIdempotency(ctx context.Context, tx *sql.Tx, userID int64, key string) (*service.GameLoyaltySpinResult, string, error) {
	item := &service.GameLoyaltySpinResult{}
	var (
		gridJSON, stopsJSON, linesJSON []byte
		requestHash                    string
	)
	err := tx.QueryRowContext(ctx, gameLoyaltyFindSpinIdempotencyQuery, userID, key).Scan(
		&item.ID, &item.UserID, &item.BetCredits, &item.PayoutCredits, &item.UsedFreeSpin,
		&gridJSON, &stopsJSON, &linesJSON, &item.ScatterCount, &item.FreeSpinsAwarded,
		&item.FreeSpinsBefore, &item.FreeSpinsAfter, &item.CreditsBefore, &item.CreditsAfter,
		&item.PaytableVersion, &item.ReelStripVersion, &item.IdempotencyKey, &requestHash, &item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("query spin idempotency: %w", err)
	}
	if err := json.Unmarshal(gridJSON, &item.Grid); err != nil {
		return nil, "", fmt.Errorf("decode spin grid: %w", err)
	}
	for row := 0; row < service.SlotRowCount; row++ {
		for reel := 0; reel < service.SlotReelCount; reel++ {
			if item.Grid[row][reel] == service.SlotSymbolBonus {
				item.BonusCount++
			}
		}
	}
	if err := json.Unmarshal(stopsJSON, &item.Stops); err != nil {
		return nil, "", fmt.Errorf("decode spin stops: %w", err)
	}
	if err := json.Unmarshal(linesJSON, &item.WinningLines); err != nil {
		return nil, "", fmt.Errorf("decode spin lines: %w", err)
	}
	return item, requestHash, nil
}

func findClaimIdempotency(ctx context.Context, tx *sql.Tx, userID int64, key string) (*service.GameLoyaltyRewardClaimResult, string, error) {
	item := &service.GameLoyaltyRewardClaimResult{}
	var requestHash string
	var expires sql.NullTime
	err := tx.QueryRowContext(ctx, gameLoyaltyFindClaimIdempotencyQuery, userID, key).Scan(
		&item.ID, &item.UserID, &item.RewardID, &item.Title, &item.CreditCost, &item.VoucherValue,
		&item.RedeemCode, &item.ClaimLocalDate, &item.CreditsBefore, &item.CreditsAfter, &expires,
		&item.IdempotencyKey, &requestHash, &item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("query claim idempotency: %w", err)
	}
	if expires.Valid {
		t := expires.Time
		item.ExpiresAt = &t
	}
	return item, requestHash, nil
}

func isGameLoyaltyUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// lib/pq and pgx both include SQLSTATE 23505 in the error string/message.
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(strings.ToLower(msg), "unique constraint") || strings.Contains(strings.ToLower(msg), "duplicate key")
}
