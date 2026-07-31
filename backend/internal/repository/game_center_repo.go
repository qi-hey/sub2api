package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	gameDailyScoreUpsertQuery = `
INSERT INTO game_daily_scores (local_date, game_id, user_id, score, achieved_at)
SELECT $2::date, $3, u.id, $4::bigint, $5
FROM users u
WHERE u.id = $1 AND u.deleted_at IS NULL
ON CONFLICT (user_id, game_id, local_date) DO UPDATE
SET score = EXCLUDED.score,
    achieved_at = EXCLUDED.achieved_at,
    updated_at = NOW()
WHERE game_daily_scores.score < EXCLUDED.score`

	gameAllTimeLeaderboardTopQuery = `
	WITH best_scores AS (
	    SELECT DISTINCT ON (s.user_id)
	           s.user_id, u.username, u.email, s.score, s.achieved_at
	    FROM game_daily_scores s
	    JOIN users u ON u.id = s.user_id AND u.deleted_at IS NULL
	    WHERE s.game_id = $1
	    ORDER BY s.user_id, s.score DESC, s.achieved_at ASC, s.local_date ASC
	)
	SELECT user_id, username, email, score, achieved_at
	FROM best_scores
	ORDER BY score DESC, achieved_at ASC, user_id ASC
	LIMIT $2`

	gameAllTimeLeaderboardUserRankQuery = `
	WITH best_scores AS (
	    SELECT DISTINCT ON (s.user_id)
	           s.user_id, u.username, u.email, s.score, s.achieved_at
	    FROM game_daily_scores s
	    JOIN users u ON u.id = s.user_id AND u.deleted_at IS NULL
	    WHERE s.game_id = $1
	    ORDER BY s.user_id, s.score DESC, s.achieved_at ASC, s.local_date ASC
	)
	SELECT (
	           SELECT COUNT(*) + 1
	           FROM best_scores ahead
	           WHERE ahead.score > current_score.score
	              OR (ahead.score = current_score.score AND ahead.achieved_at < current_score.achieved_at)
	              OR (ahead.score = current_score.score AND ahead.achieved_at = current_score.achieved_at AND ahead.user_id < current_score.user_id)
	       ) AS rank,
	       current_score.user_id,
	       current_score.username,
	       current_score.email,
	       current_score.score,
	       current_score.achieved_at
	FROM best_scores current_score
	WHERE current_score.user_id = $2`

	gameSlotBonusInsertQuery = `
INSERT INTO game_slot_bonus_rounds (
    user_id, trigger_spin_id, local_date, token_hash, rewards, expires_at
) VALUES ($1, $2, $3::date, $4, $5::jsonb, $6)
ON CONFLICT (trigger_spin_id) DO NOTHING
RETURNING id, expires_at, status`

	gameSlotBonusFindBySpinQuery = `
SELECT id, rewards, expires_at, status
FROM game_slot_bonus_rounds
WHERE trigger_spin_id = $1`

	gameSlotBonusFindPendingQuery = `
SELECT b.id, b.rewards, b.expires_at, b.status, s.idempotency_key
FROM game_slot_bonus_rounds b
JOIN game_slot_spins s ON s.id = b.trigger_spin_id AND s.user_id = b.user_id
WHERE b.user_id = $1
  AND b.status = 'pending'
  AND b.expires_at > $2
ORDER BY b.created_at ASC, b.id ASC
LIMIT 1`

	gameSlotBonusFindClaimReplayQuery = `
SELECT b.id, b.choice, b.reward_credits, l.credits_after, b.claimed_at, b.claim_request_hash, b.rewards
FROM game_slot_bonus_rounds b
JOIN game_credit_ledger l
  ON l.reference_type = 'slot_bonus' AND l.reference_id = b.id
WHERE b.user_id = $1 AND b.claim_idempotency_key = $2 AND b.status = 'claimed'`

	gameSlotBonusLockByTokenQuery = `
SELECT id, status, rewards, expires_at
FROM game_slot_bonus_rounds
WHERE user_id = $1 AND token_hash = $2
FOR UPDATE`

	gameSlotBonusExpireQuery = `
UPDATE game_slot_bonus_rounds
SET status = 'expired', updated_at = NOW()
WHERE id = $1 AND status = 'pending'`

	gameSlotBonusClaimQuery = `
UPDATE game_slot_bonus_rounds
SET status = 'claimed',
    choice = $2,
    reward_credits = $3,
    claimed_at = $4,
    claim_idempotency_key = $5,
    claim_request_hash = $6,
    updated_at = NOW()
WHERE id = $1 AND status = 'pending'
RETURNING claimed_at`
)

var _ service.GameCenterRepository = (*gameLoyaltyRepository)(nil)
var _ service.GameSlotBonusSnapshotRepository = (*gameLoyaltyRepository)(nil)

type slotBonusRewardsV2 struct {
	Kind   string   `json:"kind"`
	Values [3]int64 `json:"values"`
}

type slotBonusRewardsV3 struct {
	Kind            string                    `json:"kind"`
	Protocol        string                    `json:"protocol"`
	BetCredits      int64                     `json:"bet_credits"`
	BonusCount      int                       `json:"bonus_count"`
	BonusMultiplier int                       `json:"bonus_multiplier"`
	Options         []service.SlotBonusOption `json:"options"`
}

func encodeSlotBonusOffer(input *service.SlotBonusRoundOfferInput) ([]byte, error) {
	if input == nil {
		return nil, fmt.Errorf("slot bonus offer is required")
	}
	if input.Plan == nil {
		return encodeSlotBonusRewards(input.Kind, input.Rewards)
	}
	plan := slotBonusRewardsV3{
		Kind: input.Plan.Kind, Protocol: input.Plan.Protocol,
		BetCredits: input.Plan.BetCredits, BonusCount: input.Plan.BonusCount,
		BonusMultiplier: input.Plan.BonusMultiplier, Options: input.Plan.Options,
	}
	if err := validateSlotBonusPlan(plan); err != nil {
		return nil, err
	}
	return json.Marshal(plan)
}

func encodeSlotBonusRewards(kind string, values [3]int64) ([]byte, error) {
	if err := validateSlotBonusRewards(kind, values); err != nil {
		return nil, err
	}
	return json.Marshal(slotBonusRewardsV2{Kind: kind, Values: values})
}

func decodeSlotBonusRewards(raw []byte) (string, [3]int64, *slotBonusRewardsV3, error) {
	var plan slotBonusRewardsV3
	if err := json.Unmarshal(raw, &plan); err == nil && (plan.Protocol == service.SlotBonusProtocolV3 || plan.Protocol == service.SlotBonusProtocolV4) {
		if err := validateSlotBonusPlan(plan); err != nil {
			return "", [3]int64{}, nil, err
		}
		return plan.Kind, [3]int64{}, &plan, nil
	}

	var current slotBonusRewardsV2
	if err := json.Unmarshal(raw, &current); err == nil {
		if err := validateSlotBonusRewards(current.Kind, current.Values); err != nil {
			return "", [3]int64{}, nil, err
		}
		return current.Kind, current.Values, nil, nil
	}

	// R22 persisted a bare three-element array. Those rounds were all chest
	// picks and remain claimable after the v2 protocol rollout.
	var legacy []int64
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return "", [3]int64{}, nil, fmt.Errorf("decode slot bonus rewards: %w", err)
	}
	if len(legacy) != 3 {
		return "", [3]int64{}, nil, fmt.Errorf("decode slot bonus rewards: expected 3 choices")
	}
	values := [3]int64{legacy[0], legacy[1], legacy[2]}
	if err := validateSlotBonusRewards(service.SlotBonusKindPickChest, values); err != nil {
		return "", [3]int64{}, nil, err
	}
	return service.SlotBonusKindPickChest, values, nil, nil
}

func validateSlotBonusPlan(plan slotBonusRewardsV3) error {
	if plan.Kind != service.SlotBonusKindFreeSpins ||
		(plan.Protocol != service.SlotBonusProtocolV3 && plan.Protocol != service.SlotBonusProtocolV4) {
		return fmt.Errorf("invalid free-spin bonus protocol")
	}
	if plan.BetCredits <= 0 || plan.BonusCount < 3 || plan.BonusCount > 5 {
		return fmt.Errorf("invalid free-spin bonus trigger")
	}
	wantBonusMultiplier := map[int]int{3: 1, 4: 2, 5: 4}[plan.BonusCount]
	if plan.BonusMultiplier != wantBonusMultiplier || len(plan.Options) != 3 {
		return fmt.Errorf("invalid free-spin bonus multiplier or options")
	}
	wantSpins := [...]int{8, 12, 20}
	wantMultipliers := [...]int{5, 3, 2}
	for index, option := range plan.Options {
		if option.Choice != index || option.Spins != wantSpins[index] || option.Multiplier != wantMultipliers[index] {
			return fmt.Errorf("invalid free-spin bonus option")
		}
		if len(option.SpinResults) != option.Spins || option.BasePayout < 0 || option.RewardCredits <= 0 {
			return fmt.Errorf("invalid free-spin bonus outcome")
		}
		var base int64
		winningSpins := 0
		for _, spin := range option.SpinResults {
			if spin == nil || spin.BetCredits != plan.BetCredits || spin.BonusCount >= service.SlotBonusTriggerMin {
				return fmt.Errorf("invalid free-spin result")
			}
			if spin.TotalPayout > 0 && base > math.MaxInt64-spin.TotalPayout {
				return fmt.Errorf("free-spin payout overflow")
			}
			base += spin.TotalPayout
			if spin.TotalPayout > 0 {
				winningSpins++
			}
		}
		if base != option.BasePayout {
			return fmt.Errorf("free-spin base payout mismatch")
		}
		var expected int64
		if plan.Protocol == service.SlotBonusProtocolV4 {
			minimumWins, err := service.SlotBonusMinimumWinningSpins(option.Spins)
			if err != nil || winningSpins < minimumWins {
				return fmt.Errorf("free-spin bonus hit guarantee is not satisfied")
			}
			expected, err = service.SlotBonusFeatureRewardCredits(
				base, plan.BetCredits, option.Multiplier, plan.BonusMultiplier,
			)
			if err != nil {
				return fmt.Errorf("free-spin reward overflow")
			}
		} else {
			if base > math.MaxInt64/int64(option.Multiplier) ||
				base*int64(option.Multiplier) > math.MaxInt64/int64(plan.BonusMultiplier) {
				return fmt.Errorf("free-spin reward overflow")
			}
			expected = base * int64(option.Multiplier) * int64(plan.BonusMultiplier)
			if expected == 0 {
				expected = int64(plan.BonusMultiplier)
			}
		}
		if option.RewardCredits != expected {
			return fmt.Errorf("free-spin reward mismatch")
		}
	}
	return nil
}

func validateSlotBonusRewards(kind string, values [3]int64) error {
	allowed := map[int64]int{50: 0, 100: 0, 200: 0}
	for _, value := range values {
		if _, ok := allowed[value]; !ok {
			return fmt.Errorf("invalid slot bonus reward")
		}
		allowed[value]++
	}
	switch kind {
	case service.SlotBonusKindPickChest:
		if allowed[50] != 1 || allowed[100] != 1 || allowed[200] != 1 {
			return fmt.Errorf("invalid pick chest rewards")
		}
	case service.SlotBonusKindLuckyWheel:
		if values[0] != values[1] || values[1] != values[2] {
			return fmt.Errorf("lucky wheel reward must be server-fixed")
		}
	default:
		return fmt.Errorf("invalid slot bonus kind %q", kind)
	}
	return nil
}

func (r *gameLoyaltyRepository) GetPendingSlotBonus(ctx context.Context, userID int64, now time.Time) (*service.SlotBonusPendingSnapshot, error) {
	item := &service.SlotBonusPendingSnapshot{Choices: 3}
	var rewardsRaw []byte
	err := r.db.QueryRowContext(ctx, gameSlotBonusFindPendingQuery, userID, now).Scan(
		&item.RoundID,
		&rewardsRaw,
		&item.ExpiresAt,
		&item.Status,
		&item.SpinIdempotencyKey,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query pending slot bonus: %w", err)
	}
	var plan *slotBonusRewardsV3
	item.Kind, _, plan, err = decodeSlotBonusRewards(rewardsRaw)
	if err != nil {
		return nil, err
	}
	if plan != nil {
		item.BetCredits = plan.BetCredits
		item.BonusCount = plan.BonusCount
		item.BonusMultiplier = plan.BonusMultiplier
		item.Options = slotBonusOptionViews(plan.Options)
	}
	return item, nil
}

func (r *gameLoyaltyRepository) SubmitDailyScore(ctx context.Context, userID int64, localDate, gameID string, score int64) error {
	if _, err := r.db.ExecContext(ctx, gameDailyScoreUpsertQuery, userID, localDate, gameID, score, time.Now()); err != nil {
		return fmt.Errorf("upsert game daily score: %w", err)
	}
	return nil
}

func (r *gameLoyaltyRepository) GetAllTimeLeaderboard(ctx context.Context, userID int64, gameID string, limit int) ([]service.GameLeaderboardEntry, error) {
	rows, err := r.db.QueryContext(ctx, gameAllTimeLeaderboardTopQuery, gameID, limit)
	if err != nil {
		return nil, fmt.Errorf("query all-time game leaderboard: %w", err)
	}

	items := make([]service.GameLeaderboardEntry, 0, limit+1)
	currentUserInTop := false
	for rows.Next() {
		item := service.GameLeaderboardEntry{Rank: int64(len(items) + 1)}
		if err := rows.Scan(&item.UserID, &item.Username, &item.Email, &item.Score, &item.AchievedAt); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan all-time game leaderboard: %w", err)
		}
		currentUserInTop = currentUserInTop || item.UserID == userID
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate all-time game leaderboard: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close all-time game leaderboard: %w", err)
	}
	if currentUserInTop {
		return items, nil
	}

	var current service.GameLeaderboardEntry
	err = r.db.QueryRowContext(ctx, gameAllTimeLeaderboardUserRankQuery, gameID, userID).Scan(
		&current.Rank, &current.UserID, &current.Username, &current.Email, &current.Score, &current.AchievedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return items, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query current user all-time game leaderboard rank: %w", err)
	}
	items = append(items, current)
	return items, nil
}

func (r *gameLoyaltyRepository) ClaimSlotBonus(ctx context.Context, userID int64, input service.SlotBonusClaimInput) (*service.SlotBonusClaimResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin slot bonus claim: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockGameLoyaltyUser(ctx, tx, userID); err != nil {
		return nil, err
	}
	if existing, existingHash, err := findSlotBonusClaimReplay(ctx, tx, userID, input.IdempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		if existingHash != input.RequestHash {
			return nil, service.ErrGameLoyaltyIdempotencyConflict
		}
		existing.IdempotentReplay = true
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit slot bonus replay: %w", err)
		}
		return existing, nil
	}

	var (
		roundID    int64
		status     string
		rewardsRaw []byte
		expiresAt  time.Time
	)
	if err := tx.QueryRowContext(ctx, gameSlotBonusLockByTokenQuery, userID, input.TokenHash).Scan(
		&roundID, &status, &rewardsRaw, &expiresAt,
	); errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGameSlotBonusTokenInvalid
	} else if err != nil {
		return nil, fmt.Errorf("lock slot bonus round: %w", err)
	}
	if status == "claimed" {
		return nil, service.ErrGameSlotBonusAlreadyClaimed
	}
	if status == "expired" || !input.Now.Before(expiresAt) {
		if status == "pending" {
			if _, err := tx.ExecContext(ctx, gameSlotBonusExpireQuery, roundID); err != nil {
				return nil, fmt.Errorf("expire slot bonus round: %w", err)
			}
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit expired slot bonus round: %w", err)
		}
		return nil, service.ErrGameSlotBonusExpired
	}

	kind, rewards, plan, err := decodeSlotBonusRewards(rewardsRaw)
	if err != nil {
		return nil, err
	}
	if input.Choice < 0 || input.Choice >= len(rewards) {
		return nil, service.ErrGameSlotBonusChoiceInvalid
	}
	reward := rewards[input.Choice]
	var selected *service.SlotBonusOption
	if plan != nil {
		selected = &plan.Options[input.Choice]
		reward = selected.RewardCredits
	}
	if reward <= 0 {
		return nil, fmt.Errorf("invalid slot bonus reward")
	}
	if _, err := tx.ExecContext(ctx, gameLoyaltyEnsureQuery, userID); err != nil {
		return nil, fmt.Errorf("ensure slot bonus wallet: %w", err)
	}
	creditsBefore, _, err := lockGameLoyaltyWallet(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	var creditsAfter int64
	if err := tx.QueryRowContext(ctx, gameLoyaltyAddCreditsQuery, userID, reward).Scan(&creditsAfter); errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGameLoyaltyCreditsLimitExceeded
	} else if err != nil {
		return nil, fmt.Errorf("add slot bonus credits: %w", err)
	}
	metadata, _ := json.Marshal(map[string]any{"round_id": roundID, "kind": kind, "choice": input.Choice})
	if _, _, err := insertLedger(
		ctx, tx, userID, service.GameLedgerEntrySlotBonus,
		reward, creditsBefore, creditsAfter, "slot_bonus", &roundID,
		"slot_bonus:"+sha256String(input.IdempotencyKey), input.RequestHash, string(metadata),
	); err != nil {
		return nil, err
	}
	claimedAt := input.Now
	if err := tx.QueryRowContext(
		ctx, gameSlotBonusClaimQuery, roundID, input.Choice, reward, input.Now,
		input.IdempotencyKey, input.RequestHash,
	).Scan(&claimedAt); err != nil {
		return nil, fmt.Errorf("claim slot bonus round: %w", err)
	}
	if _, err := tx.ExecContext(ctx, gameDailyScoreUpsertQuery, userID, input.Now.Format("2006-01-02"), service.GameIDLucky, reward, claimedAt); err != nil {
		return nil, fmt.Errorf("upsert lucky bonus score: %w", err)
	}
	result := &service.SlotBonusClaimResult{
		RoundID: roundID, Kind: kind, Choice: input.Choice, RewardCredits: reward,
		CreditsAfter: creditsAfter, ClaimedAt: claimedAt,
	}
	if selected != nil {
		result.Spins = selected.Spins
		result.Multiplier = selected.Multiplier
		result.BonusMultiplier = plan.BonusMultiplier
		result.BasePayout = selected.BasePayout
		result.SpinResults = selected.SpinResults
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit slot bonus claim: %w", err)
	}
	return result, nil
}

func findSlotBonusClaimReplay(ctx context.Context, tx *sql.Tx, userID int64, key string) (*service.SlotBonusClaimResult, string, error) {
	item := &service.SlotBonusClaimResult{}
	var requestHash string
	var rewardsRaw []byte
	err := tx.QueryRowContext(ctx, gameSlotBonusFindClaimReplayQuery, userID, key).Scan(
		&item.RoundID, &item.Choice, &item.RewardCredits, &item.CreditsAfter, &item.ClaimedAt, &requestHash, &rewardsRaw,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("query slot bonus replay: %w", err)
	}
	var plan *slotBonusRewardsV3
	item.Kind, _, plan, err = decodeSlotBonusRewards(rewardsRaw)
	if err != nil {
		return nil, "", err
	}
	if plan != nil && item.Choice >= 0 && item.Choice < len(plan.Options) {
		selected := plan.Options[item.Choice]
		item.Spins = selected.Spins
		item.Multiplier = selected.Multiplier
		item.BonusMultiplier = plan.BonusMultiplier
		item.BasePayout = selected.BasePayout
		item.SpinResults = selected.SpinResults
	}
	return item, requestHash, nil
}

func findSlotBonusBySpin(ctx context.Context, tx *sql.Tx, spinID int64) (*service.SlotBonusRoundOffer, error) {
	item := &service.SlotBonusRoundOffer{Choices: 3}
	var rewardsRaw []byte
	err := tx.QueryRowContext(ctx, gameSlotBonusFindBySpinQuery, spinID).Scan(&item.RoundID, &rewardsRaw, &item.ExpiresAt, &item.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query spin bonus round: %w", err)
	}
	var plan *slotBonusRewardsV3
	item.Kind, _, plan, err = decodeSlotBonusRewards(rewardsRaw)
	if err != nil {
		return nil, err
	}
	if plan != nil {
		item.BetCredits = plan.BetCredits
		item.BonusCount = plan.BonusCount
		item.BonusMultiplier = plan.BonusMultiplier
		item.Options = slotBonusOptionViews(plan.Options)
	}
	if item.Status == "pending" && !time.Now().Before(item.ExpiresAt) {
		item.Status = "expired"
	}
	return item, nil
}

func slotBonusOptionViews(options []service.SlotBonusOption) []service.SlotBonusOptionView {
	views := make([]service.SlotBonusOptionView, 0, len(options))
	for _, option := range options {
		views = append(views, service.SlotBonusOptionView{
			Choice: option.Choice, Spins: option.Spins, Multiplier: option.Multiplier,
		})
	}
	return views
}

func sha256String(value string) string {
	digest := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", digest[:])
}

func gameLoyaltyLedgerIdempotencyKey(namespace, externalKey string) string {
	return "gl:" + namespace + ":" + sha256String(namespace+"\n"+externalKey)
}
