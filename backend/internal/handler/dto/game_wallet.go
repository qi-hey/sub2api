package dto

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// GameWalletNumber preserves the exact JSON token so both string and legacy
// numeric request values can be validated without passing through float64.
type GameWalletNumber string

func (n *GameWalletNumber) UnmarshalJSON(data []byte) error {
	raw := bytes.TrimSpace(data)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return fmt.Errorf("amount must not be null")
	}
	if raw[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		*n = GameWalletNumber(value)
		return nil
	}
	*n = GameWalletNumber(string(raw))
	return nil
}

func (n *GameWalletNumber) StringPointer() *string {
	if n == nil {
		return nil
	}
	value := string(*n)
	return &value
}

// Deprecated R20 exchange request retained only so old clients get a clean 410
// from the removed route rather than a bind error storm.
type GameWalletExchangeRequest struct {
	Direction     string            `json:"direction"`
	BalanceAmount *GameWalletNumber `json:"balance_amount"`
	CreditsAmount *GameWalletNumber `json:"credits_amount"`
}

type GameWalletState struct {
	UserID             int64      `json:"user_id"`
	AccountBalance     string     `json:"account_balance"`
	Credits            string     `json:"credits"`
	FreeSpinsRemaining int        `json:"free_spins_remaining"`
	LoyaltyEnabled     bool       `json:"loyalty_enabled"`
	LoyaltyAvailable   bool       `json:"loyalty_available"`
	CheckinCredits     string     `json:"checkin_credits"`
	SlotBetCredits     string     `json:"slot_bet_credits"`
	SlotBetOptions     []string   `json:"slot_bet_options"`
	DailyRewardLimit   int        `json:"daily_reward_limit"`
	CheckedInToday     bool       `json:"checked_in_today"`
	TodayCheckinAt     *time.Time `json:"today_checkin_at,omitempty"`
	LocalDate          string     `json:"local_date"`
	// Legacy exchange fields kept for older UIs; always false/zero in R21.
	ExchangeEnabled   bool                  `json:"exchange_enabled"`
	ExchangeAvailable bool                  `json:"exchange_available"`
	ExchangeRate      string                `json:"exchange_rate"`
	DailyLimit        string                `json:"daily_limit"`
	DailyExchanged    string                `json:"daily_exchanged"`
	DailyRemaining    *string               `json:"daily_remaining"`
	WalletCreatedAt   time.Time             `json:"wallet_created_at"`
	WalletUpdatedAt   time.Time             `json:"wallet_updated_at"`
	PendingSlotBonus  *GameWalletBonusRound `json:"pending_slot_bonus,omitempty"`
}

type GameWalletCheckinResult struct {
	ID               string    `json:"id"`
	UserID           int64     `json:"user_id"`
	LocalDate        string    `json:"local_date"`
	CreditsAwarded   string    `json:"credits_awarded"`
	CreditsBefore    string    `json:"credits_before"`
	CreditsAfter     string    `json:"credits_after"`
	IdempotencyKey   string    `json:"idempotency_key"`
	IdempotentReplay bool      `json:"idempotent_replay,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type GameWalletSpinRequest struct {
	// BetCredits is optional. When set it must equal the configured slot bet.
	// Free spins are server-owned and cannot be requested by the client.
	BetCredits *GameWalletNumber `json:"bet_credits"`
}

type GameFruitSpinRequest struct {
	Bets map[string]GameWalletNumber `json:"bets" binding:"required"`
}

type GameFruitRewardStop struct {
	Index      int    `json:"index"`
	Symbol     string `json:"symbol"`
	Size       string `json:"size"`
	Multiplier string `json:"multiplier"`
}

type GameFruitOutcome struct {
	Kind              string                `json:"kind"`
	CenterIndex       int                   `json:"center_index,omitempty"`
	Symbol            string                `json:"symbol,omitempty"`
	Size              string                `json:"size,omitempty"`
	Multiplier        string                `json:"multiplier,omitempty"`
	ExtraStops        []GameFruitRewardStop `json:"extra_stops,omitempty"`
	LittleMaryStops   []GameFruitRewardStop `json:"little_mary_stops,omitempty"`
	JackpotTier       string                `json:"jackpot_tier,omitempty"`
	JackpotMultiplier string                `json:"jackpot_multiplier,omitempty"`
}

type GameFruitSpinResult struct {
	ID               string            `json:"id"`
	UserID           int64             `json:"user_id"`
	Bets             map[string]string `json:"bets"`
	TotalBet         string            `json:"total_bet"`
	StopIndex        int               `json:"stop_index"`
	Outcome          GameFruitOutcome  `json:"outcome"`
	Payout           string            `json:"payout"`
	CreditsBefore    string            `json:"credits_before"`
	CreditsAfter     string            `json:"credits_after"`
	PaytableVersion  string            `json:"paytable_version"`
	IdempotencyKey   string            `json:"idempotency_key"`
	IdempotentReplay bool              `json:"idempotent_replay,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
}

type GameWalletWinningLine struct {
	LineIndex  int      `json:"line_index"`
	Symbol     string   `json:"symbol"`
	Count      int      `json:"count"`
	Multiplier string   `json:"multiplier"`
	Payout     string   `json:"payout"`
	Positions  [][2]int `json:"positions"`
}

type GameWalletSpinResult struct {
	ID                 string                  `json:"id"`
	UserID             int64                   `json:"user_id"`
	BetCredits         string                  `json:"bet_credits"`
	PayoutCredits      string                  `json:"payout_credits"`
	UsedFreeSpin       bool                    `json:"used_free_spin"`
	Grid               [][]string              `json:"grid"`
	Stops              []int                   `json:"stops"`
	WinningLines       []GameWalletWinningLine `json:"winning_lines"`
	ScatterCount       int                     `json:"scatter_count"`
	BonusCount         int                     `json:"bonus_count"`
	FreeSpinsAwarded   int                     `json:"free_spins_awarded"`
	FreeSpinsRemaining int                     `json:"free_spins_remaining"`
	CreditsBefore      string                  `json:"credits_before"`
	CreditsAfter       string                  `json:"credits_after"`
	PaytableVersion    string                  `json:"paytable_version"`
	ReelStripVersion   string                  `json:"reel_strip_version"`
	IdempotencyKey     string                  `json:"idempotency_key"`
	IdempotentReplay   bool                    `json:"idempotent_replay,omitempty"`
	CreatedAt          time.Time               `json:"created_at"`
	BonusRound         *GameWalletBonusRound   `json:"bonus_round,omitempty"`
}

type GameWalletBonusRound struct {
	RoundID         string                  `json:"round_id"`
	Kind            string                  `json:"kind"`
	Token           string                  `json:"token,omitempty"`
	ExpiresAt       time.Time               `json:"expires_at"`
	Choices         int                     `json:"choices"`
	Status          string                  `json:"status,omitempty"`
	BetCredits      string                  `json:"bet_credits,omitempty"`
	BonusCount      int                     `json:"bonus_count,omitempty"`
	BonusMultiplier int                     `json:"bonus_multiplier,omitempty"`
	Options         []GameWalletBonusOption `json:"options,omitempty"`
}

type GameWalletBonusOption struct {
	Choice     int `json:"choice"`
	Spins      int `json:"spins"`
	Multiplier int `json:"multiplier"`
}

type GameWalletBonusClaimRequest struct {
	Token  string `json:"token" binding:"required"`
	Choice *int   `json:"choice" binding:"required"`
}

type GameWalletBonusClaimResult struct {
	RoundID          string                      `json:"round_id"`
	Kind             string                      `json:"kind"`
	Choice           int                         `json:"choice"`
	RewardCredits    string                      `json:"reward_credits"`
	CreditsAfter     string                      `json:"credits_after"`
	ClaimedAt        time.Time                   `json:"claimed_at"`
	IdempotentReplay bool                        `json:"idempotent_replay"`
	Spins            int                         `json:"spins,omitempty"`
	Multiplier       int                         `json:"multiplier,omitempty"`
	BonusMultiplier  int                         `json:"bonus_multiplier,omitempty"`
	TotalBasePayout  string                      `json:"total_base_payout,omitempty"`
	SpinResults      []GameWalletBonusSpinResult `json:"spin_results,omitempty"`
}

type GameWalletBonusSpinResult struct {
	BetCredits       string                  `json:"bet_credits"`
	PayoutCredits    string                  `json:"payout_credits"`
	Grid             [][]string              `json:"grid"`
	Stops            []int                   `json:"stops"`
	WinningLines     []GameWalletWinningLine `json:"winning_lines"`
	ScatterCount     int                     `json:"scatter_count"`
	BonusCount       int                     `json:"bonus_count"`
	FreeSpinsAwarded int                     `json:"free_spins_awarded"`
	PaytableVersion  string                  `json:"paytable_version"`
	ReelStripVersion string                  `json:"reel_strip_version"`
}

type GameWalletScoreRequest struct {
	GameID string            `json:"game_id" binding:"required"`
	Score  *GameWalletNumber `json:"score" binding:"required"`
}

type GameWalletLeaderboardItem struct {
	Rank          int64     `json:"rank"`
	UserDisplay   string    `json:"user_display"`
	Score         string    `json:"score"`
	AchievedAt    time.Time `json:"achieved_at"`
	IsCurrentUser bool      `json:"is_current_user"`
}

type GameWalletLeaderboard struct {
	GameID    string                      `json:"game_id"`
	LocalDate string                      `json:"local_date"`
	Items     []GameWalletLeaderboardItem `json:"items"`
	MyEntry   *GameWalletLeaderboardItem  `json:"my_entry"`
}

type GameWalletRewardClaimRequest struct {
	RewardID string `json:"reward_id"`
}

type GameWalletRewardItem struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	CreditCost     string `json:"credit_cost"`
	VoucherValue   string `json:"voucher_value"`
	DailyStock     int    `json:"daily_stock"`
	ExpiryDays     int    `json:"expiry_days"`
	RemainingStock *int   `json:"remaining_stock"`
	ClaimedToday   int    `json:"claimed_today"`
}

type GameWalletRewardList struct {
	Items            []GameWalletRewardItem `json:"items"`
	DailyRewardLimit int                    `json:"daily_reward_limit"`
	ClaimedToday     int                    `json:"claimed_today"`
	LoyaltyEnabled   bool                   `json:"loyalty_enabled"`
	LoyaltyAvailable bool                   `json:"loyalty_available"`
}

type GameWalletRewardClaimResult struct {
	ID               string     `json:"id"`
	UserID           int64      `json:"user_id"`
	RewardID         string     `json:"reward_id"`
	Title            string     `json:"title"`
	CreditCost       string     `json:"credit_cost"`
	VoucherValue     string     `json:"voucher_value"`
	RedeemCode       string     `json:"redeem_code"`
	ClaimLocalDate   string     `json:"claim_local_date"`
	CreditsBefore    string     `json:"credits_before"`
	CreditsAfter     string     `json:"credits_after"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	IdempotencyKey   string     `json:"idempotency_key"`
	IdempotentReplay bool       `json:"idempotent_replay,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

type GameCreditGiftRequest struct {
	RecipientEmail string            `json:"recipient_email" binding:"required"`
	Credits        *GameWalletNumber `json:"credits" binding:"required"`
}

type GameCreditGiftResult struct {
	ID                     string    `json:"id"`
	SenderUserID           int64     `json:"sender_user_id"`
	RecipientUserID        int64     `json:"recipient_user_id"`
	RecipientEmail         string    `json:"recipient_email"`
	Credits                string    `json:"credits"`
	SenderCreditsBefore    string    `json:"sender_credits_before"`
	SenderCreditsAfter     string    `json:"sender_credits_after"`
	RecipientCreditsBefore string    `json:"recipient_credits_before"`
	RecipientCreditsAfter  string    `json:"recipient_credits_after"`
	IdempotencyKey         string    `json:"idempotency_key"`
	IdempotentReplay       bool      `json:"idempotent_replay,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
}

// GameWalletTransaction is now a credit-ledger entry (R21). Legacy exchange
// fields are omitted; clients should use entry_type + amount.
type GameWalletTransaction struct {
	ID             string          `json:"id"`
	UserID         int64           `json:"user_id"`
	EntryType      string          `json:"entry_type"`
	Amount         string          `json:"amount"`
	CreditsBefore  string          `json:"credits_before"`
	CreditsAfter   string          `json:"credits_after"`
	ReferenceType  string          `json:"reference_type,omitempty"`
	ReferenceID    *string         `json:"reference_id,omitempty"`
	IdempotencyKey string          `json:"idempotency_key"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

type GameWalletTransactionList struct {
	Items        []GameWalletTransaction `json:"items"`
	NextBeforeID *string                 `json:"next_before_id"`
}
