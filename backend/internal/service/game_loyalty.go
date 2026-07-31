package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	GameLedgerEntryCheckin      = "checkin"
	GameLedgerEntrySlotBet      = "slot_bet"
	GameLedgerEntrySlotPayout   = "slot_payout"
	GameLedgerEntryRewardClaim  = "reward_claim"
	GameLedgerEntryGiftSent     = "gift_sent"
	GameLedgerEntryGiftReceived = "gift_received"

	GameRewardCodePrefix = "GT"

	gameLoyaltyMaxRawSize      = 32
	gameLoyaltyMaxWhole        = 12
	gameLoyaltyMaxScale        = 8
	gameLoyaltyMaxCatalogJSON  = 64 * 1024
	gameLoyaltyMaxCatalogItems = 50
	gameLoyaltyMaxTitleLen     = 128
	gameLoyaltyMaxRewardIDLen  = 64
	gameLoyaltyMaxDailyClaims  = 1000
	gameLoyaltyMaxDailySpins   = 5000
	gameLoyaltyMaxFreeSpins    = 1000
)

var (
	ErrGameLoyaltyDisabled = infraerrors.Forbidden(
		"GAME_LOYALTY_DISABLED", "game loyalty feature is disabled",
	)
	ErrGameLoyaltyConfigInvalid = infraerrors.New(
		500, "GAME_LOYALTY_CONFIG_INVALID", "game loyalty configuration is invalid",
	)
	ErrGameLoyaltyNotConfigured = infraerrors.Forbidden(
		"GAME_LOYALTY_NOT_CONFIGURED", "game loyalty economics are not configured",
	)
	ErrGameLoyaltyCheckinAlreadyClaimed = infraerrors.Conflict(
		"GAME_LOYALTY_CHECKIN_ALREADY_CLAIMED", "daily check-in already claimed",
	)
	ErrGameLoyaltyInsufficientCredits = infraerrors.Conflict(
		"GAME_LOYALTY_INSUFFICIENT_CREDITS", "game wallet credits are insufficient",
	)
	ErrGameLoyaltyCreditsLimitExceeded = infraerrors.Conflict(
		"GAME_LOYALTY_CREDITS_LIMIT_EXCEEDED", "game wallet credits limit would be exceeded",
	)
	ErrGameLoyaltyRewardNotFound = infraerrors.NotFound(
		"GAME_LOYALTY_REWARD_NOT_FOUND", "reward not found or disabled",
	)
	ErrGameLoyaltyRewardCapExceeded = infraerrors.Conflict(
		"GAME_LOYALTY_REWARD_CAP_EXCEEDED", "daily reward claim limit would be exceeded",
	)
	ErrGameLoyaltyRewardStockExceeded = infraerrors.Conflict(
		"GAME_LOYALTY_REWARD_STOCK_EXCEEDED", "daily stock for this reward is exhausted",
	)
	ErrGameLoyaltyInvalidRewardID = infraerrors.BadRequest(
		"GAME_LOYALTY_REWARD_ID_INVALID", "reward_id is required",
	)
	ErrGameLoyaltyInvalidBet = infraerrors.BadRequest(
		"GAME_LOYALTY_BET_INVALID", "bet_credits is not a supported slot bet",
	)
	ErrGameLoyaltyRNGFailed = infraerrors.New(
		500, "GAME_LOYALTY_RNG_FAILED", "failed to generate a secure slot outcome",
	)
	ErrGameLoyaltySpinLimitExceeded = infraerrors.TooManyRequests(
		"GAME_LOYALTY_SPIN_LIMIT_EXCEEDED", "too many slot spins today",
	)
	ErrGameLoyaltyUserNotFound        = ErrGameWalletUserNotFound
	ErrGameCreditGiftRecipientInvalid = infraerrors.BadRequest(
		"GAME_CREDIT_GIFT_RECIPIENT_INVALID", "recipient_email must be a valid email address",
	)
	ErrGameCreditGiftRecipientNotFound = infraerrors.NotFound(
		"GAME_CREDIT_GIFT_RECIPIENT_NOT_FOUND", "recipient account was not found",
	)
	ErrGameCreditGiftSelfForbidden = infraerrors.BadRequest(
		"GAME_CREDIT_GIFT_SELF_FORBIDDEN", "game credits cannot be gifted to the same account",
	)
	ErrGameCreditGiftAmountInvalid = infraerrors.BadRequest(
		"GAME_CREDIT_GIFT_AMOUNT_INVALID", "gift credits must be a positive integer",
	)
	ErrGameCreditGiftUnavailable = infraerrors.New(
		500, "GAME_CREDIT_GIFT_UNAVAILABLE", "game credit gifting is unavailable",
	)
	// Reuse exchange idempotency errors so clients see one consistent contract.
	ErrGameLoyaltyIdempotencyKeyRequired = ErrGameWalletIdempotencyKeyRequired
	ErrGameLoyaltyIdempotencyKeyInvalid  = ErrGameWalletIdempotencyKeyInvalid
	ErrGameLoyaltyIdempotencyConflict    = ErrGameWalletIdempotencyConflict
)

// GameLoyaltyRewardItem is one catalog entry after validation.
type GameLoyaltyRewardItem struct {
	ID              string
	Title           string
	CreditCost      int64
	VoucherValue    string // exact decimal string
	DailyStock      int
	ExpiryDays      int // 0 = no expiry on generated code
	Enabled         bool
	LocalizedTitles map[string]string
}

// GameLoyaltyConfig is the fail-closed operator configuration.
type GameLoyaltyConfig struct {
	Enabled           bool
	CheckinCredits    int64
	SlotBetCredits    int64
	DailyRewardLimit  int
	Catalog           []GameLoyaltyRewardItem
	CheckinConfigured bool
	SlotConfigured    bool
	CatalogValid      bool
	DailyLimitValid   bool
}

func (c GameLoyaltyConfig) Available() bool {
	return c.Enabled &&
		c.CheckinConfigured && c.CheckinCredits > 0 &&
		c.SlotConfigured && c.SlotBetCredits > 0 &&
		c.CatalogValid &&
		c.DailyLimitValid
}

// GameLoyaltyWalletSnapshot is the locked/read wallet state.
type GameLoyaltyWalletSnapshot struct {
	UserID             int64
	AccountBalance     string
	Credits            int64
	FreeSpinsRemaining int
	WalletCreatedAt    time.Time
	WalletUpdatedAt    time.Time
	CheckedInToday     bool
	TodayCheckinAt     *time.Time
	LocalDate          string
}

// GameLoyaltyState is the user-facing wallet + feature status.
type GameLoyaltyState struct {
	UserID             int64
	AccountBalance     string
	Credits            int64
	FreeSpinsRemaining int
	LoyaltyEnabled     bool
	LoyaltyAvailable   bool
	CheckinCredits     string
	SlotBetCredits     string
	SlotBetOptions     []int64
	DailyRewardLimit   int
	CheckedInToday     bool
	TodayCheckinAt     *time.Time
	LocalDate          string
	WalletCreatedAt    time.Time
	WalletUpdatedAt    time.Time
	PendingSlotBonus   *SlotBonusRoundOffer
}

// GameLoyaltyCheckinResult is returned by check-in (including replays).
type GameLoyaltyCheckinResult struct {
	ID               int64
	UserID           int64
	LocalDate        string
	CreditsAwarded   int64
	CreditsBefore    int64
	CreditsAfter     int64
	IdempotencyKey   string
	IdempotentReplay bool
	CreatedAt        time.Time
}

// GameLoyaltySpinInput is the normalized spin request.
type GameLoyaltySpinInput struct {
	BetCredits     int64 // 0 means "use free spin if available, else configured bet"
	IdempotencyKey string
	RequestHash    string
	LocalDate      string
	BonusRound     *SlotBonusRoundOfferInput
}

// GameLoyaltySpinResult is the authoritative spin response payload.
type GameLoyaltySpinResult struct {
	ID               int64
	UserID           int64
	BetCredits       int64
	PayoutCredits    int64
	UsedFreeSpin     bool
	Grid             [SlotRowCount][SlotReelCount]string
	Stops            [SlotReelCount]int
	WinningLines     []SlotWinningLine
	ScatterCount     int
	BonusCount       int
	FreeSpinsAwarded int
	FreeSpinsBefore  int
	FreeSpinsAfter   int
	CreditsBefore    int64
	CreditsAfter     int64
	PaytableVersion  string
	ReelStripVersion string
	IdempotencyKey   string
	IdempotentReplay bool
	CreatedAt        time.Time
	BonusRound       *SlotBonusRoundOffer
}

// GameLoyaltyRewardClaimInput is a normalized claim request.
type GameLoyaltyRewardClaimInput struct {
	RewardID       string
	IdempotencyKey string
	RequestHash    string
}

// GameLoyaltyRewardClaimResult is returned after a successful claim.
type GameLoyaltyRewardClaimResult struct {
	ID               int64
	UserID           int64
	RewardID         string
	Title            string
	CreditCost       int64
	VoucherValue     string
	RedeemCode       string
	ClaimLocalDate   string
	CreditsBefore    int64
	CreditsAfter     int64
	ExpiresAt        *time.Time
	IdempotencyKey   string
	IdempotentReplay bool
	CreatedAt        time.Time
}

// GameLoyaltyLedgerEntry is an immutable credit movement.
type GameLoyaltyLedgerEntry struct {
	ID             int64
	UserID         int64
	EntryType      string
	Amount         int64
	CreditsBefore  int64
	CreditsAfter   int64
	ReferenceType  string
	ReferenceID    *int64
	IdempotencyKey string
	Metadata       json.RawMessage
	CreatedAt      time.Time
}

// GameCreditGiftInput is the normalized, idempotent transfer request.
type GameCreditGiftInput struct {
	RecipientEmail string
	Credits        int64
	IdempotencyKey string
	RequestHash    string
}

// GameCreditGiftResult is the immutable result returned to the sender.
type GameCreditGiftResult struct {
	ID                     int64
	SenderUserID           int64
	RecipientUserID        int64
	RecipientEmail         string
	Credits                int64
	SenderCreditsBefore    int64
	SenderCreditsAfter     int64
	RecipientCreditsBefore int64
	RecipientCreditsAfter  int64
	IdempotencyKey         string
	IdempotentReplay       bool
	CreatedAt              time.Time
}

// GameLoyaltyRewardView is a client-safe catalog row.
type GameLoyaltyRewardView struct {
	ID             string
	Title          string
	CreditCost     int64
	VoucherValue   string
	DailyStock     int
	ExpiryDays     int
	RemainingStock *int
	ClaimedToday   int
}

// GameLoyaltyRepository owns all loyalty SQL mutations.
type GameLoyaltyRepository interface {
	GetSnapshot(ctx context.Context, userID int64, localDate string) (*GameLoyaltyWalletSnapshot, error)
	CheckIn(ctx context.Context, userID int64, localDate string, credits int64, idempotencyKey, requestHash string) (*GameLoyaltyCheckinResult, error)
	Spin(ctx context.Context, userID int64, input GameLoyaltySpinInput, betCredits int64, outcome *SlotSpinResult) (*GameLoyaltySpinResult, error)
	ListRewardsStatus(ctx context.Context, userID int64, localDate string, catalog []GameLoyaltyRewardItem, dailyLimit int) ([]GameLoyaltyRewardView, int, error)
	ClaimReward(ctx context.Context, userID int64, localDate string, item GameLoyaltyRewardItem, dailyLimit int, redeemCode string, expiresAt *time.Time, input GameLoyaltyRewardClaimInput) (*GameLoyaltyRewardClaimResult, error)
	ListLedger(ctx context.Context, userID int64, limit int, beforeID int64) ([]GameLoyaltyLedgerEntry, error)
}

// GameCreditGiftRepository is optional so older test doubles remain source compatible.
// The production loyalty repository implements it.
type GameCreditGiftRepository interface {
	GiftCredits(ctx context.Context, senderUserID int64, input GameCreditGiftInput) (*GameCreditGiftResult, error)
}

// GameLoyaltyService is the server-authoritative loyalty API.
type GameLoyaltyService struct {
	repo                 GameLoyaltyRepository
	settingService       *SettingService
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCache         GameWalletBalanceCache
	slot                 *SlotEngine
	fruit                *FruitEngine
	codeRand             io.Reader
}

func NewGameLoyaltyService(
	repo GameLoyaltyRepository,
	settingService *SettingService,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	billingCache GameWalletBalanceCache,
) *GameLoyaltyService {
	return &GameLoyaltyService{
		repo:                 repo,
		settingService:       settingService,
		authCacheInvalidator: authCacheInvalidator,
		billingCache:         billingCache,
		slot:                 NewSlotEngine(nil),
		fruit:                NewFruitEngine(nil),
		codeRand:             defaultCryptoReader(),
	}
}

// WithSlotEngine replaces the slot engine (tests).
func (s *GameLoyaltyService) WithSlotEngine(engine *SlotEngine) *GameLoyaltyService {
	if engine != nil {
		s.slot = engine
	}
	return s
}

// WithCodeRand replaces the redeem-code entropy source (tests).
func (s *GameLoyaltyService) WithCodeRand(r io.Reader) *GameLoyaltyService {
	if r != nil {
		s.codeRand = r
	}
	return s
}

func (s *GameLoyaltyService) GetWallet(ctx context.Context, userID int64) (*GameLoyaltyState, error) {
	localDate := timezone.Today().Format("2006-01-02")
	snapshot, err := s.repo.GetSnapshot(ctx, userID, localDate)
	if err != nil {
		return nil, err
	}
	var pendingSlotBonus *SlotBonusRoundOffer
	if bonusRepo, ok := s.repo.(GameSlotBonusSnapshotRepository); ok {
		now := timezone.Now()
		pending, pendingErr := bonusRepo.GetPendingSlotBonus(ctx, userID, now)
		if pendingErr != nil {
			return nil, pendingErr
		}
		if pending != nil && pending.Status == "pending" && now.Before(pending.ExpiresAt) {
			pendingSlotBonus = &SlotBonusRoundOffer{
				RoundID:         pending.RoundID,
				Kind:            pending.Kind,
				Token:           deriveSlotBonusToken(userID, pending.SpinIdempotencyKey),
				ExpiresAt:       pending.ExpiresAt,
				Choices:         pending.Choices,
				Status:          pending.Status,
				BetCredits:      pending.BetCredits,
				BonusCount:      pending.BonusCount,
				BonusMultiplier: pending.BonusMultiplier,
				Options:         pending.Options,
			}
		}
	}
	cfg := s.getConfig(ctx)
	return &GameLoyaltyState{
		UserID:             snapshot.UserID,
		AccountBalance:     snapshot.AccountBalance,
		Credits:            snapshot.Credits,
		FreeSpinsRemaining: snapshot.FreeSpinsRemaining,
		LoyaltyEnabled:     cfg.Enabled,
		LoyaltyAvailable:   cfg.Available(),
		CheckinCredits:     strconv.FormatInt(cfg.CheckinCredits, 10),
		SlotBetCredits:     strconv.FormatInt(cfg.SlotBetCredits, 10),
		SlotBetOptions:     SlotBetOptions(cfg.SlotBetCredits),
		DailyRewardLimit:   cfg.DailyRewardLimit,
		CheckedInToday:     snapshot.CheckedInToday,
		TodayCheckinAt:     snapshot.TodayCheckinAt,
		LocalDate:          snapshot.LocalDate,
		WalletCreatedAt:    snapshot.WalletCreatedAt,
		WalletUpdatedAt:    snapshot.WalletUpdatedAt,
		PendingSlotBonus:   pendingSlotBonus,
	}, nil
}

func (s *GameLoyaltyService) CheckIn(ctx context.Context, userID int64, idempotencyKey string) (*GameLoyaltyCheckinResult, error) {
	key, err := normalizeGameWalletIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, err
	}
	cfg := s.getConfig(ctx)
	if !cfg.Enabled {
		return nil, ErrGameLoyaltyDisabled
	}
	if !cfg.Available() {
		if !cfg.CheckinConfigured || cfg.CheckinCredits <= 0 {
			return nil, ErrGameLoyaltyNotConfigured
		}
		return nil, ErrGameLoyaltyConfigInvalid
	}
	localDate := timezone.Today().Format("2006-01-02")
	hash := sha256.Sum256([]byte("checkin\n" + localDate + "\n" + strconv.FormatInt(cfg.CheckinCredits, 10)))
	return s.repo.CheckIn(ctx, userID, localDate, cfg.CheckinCredits, key, hex.EncodeToString(hash[:]))
}

func (s *GameLoyaltyService) Spin(ctx context.Context, userID int64, idempotencyKey string, requestedBet *int64) (*GameLoyaltySpinResult, error) {
	key, err := normalizeGameWalletIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, err
	}
	cfg := s.getConfig(ctx)
	if !cfg.Enabled {
		return nil, ErrGameLoyaltyDisabled
	}
	if !cfg.Available() {
		if !cfg.SlotConfigured || cfg.SlotBetCredits <= 0 {
			return nil, ErrGameLoyaltyNotConfigured
		}
		return nil, ErrGameLoyaltyConfigInvalid
	}

	var reqBet int64
	if requestedBet != nil {
		if *requestedBet <= 0 {
			return nil, ErrGameLoyaltyInvalidBet
		}
		if !IsSupportedSlotBet(*requestedBet, cfg.SlotBetCredits) {
			return nil, ErrGameLoyaltyInvalidBet
		}
		reqBet = *requestedBet
	}

	hashMaterial := "spin\n" + strconv.FormatInt(cfg.SlotBetCredits, 10)
	if requestedBet != nil {
		hashMaterial += "\n" + strconv.FormatInt(*requestedBet, 10)
	} else {
		hashMaterial += "\nauto"
	}
	hash := sha256.Sum256([]byte(hashMaterial))
	input := GameLoyaltySpinInput{
		BetCredits:     reqBet,
		IdempotencyKey: key,
		RequestHash:    hex.EncodeToString(hash[:]),
		LocalDate:      timezone.Today().Format("2006-01-02"),
	}

	effectiveBet := cfg.SlotBetCredits
	if reqBet > 0 {
		effectiveBet = reqBet
	}
	profile := s.gameOddsProfile(ctx, GameOddsPoolSlot)
	outcome, err := s.slot.SpinWithOddsProfile(effectiveBet, profile)
	if err != nil {
		slog.Error("game loyalty slot rng failed", "user_id", userID, "error", err)
		return nil, ErrGameLoyaltyRNGFailed
	}
	if outcome.BonusCount >= SlotBonusTriggerMin {
		offer, offerErr := s.newSlotBonusOffer(userID, key, input.LocalDate, effectiveBet, outcome.BonusCount)
		if offerErr != nil {
			slog.Error("game loyalty slot bonus rng failed", "user_id", userID, "error", offerErr)
			return nil, ErrGameLoyaltyRNGFailed
		}
		input.BonusRound = offer
	}
	// Never expose seed; outcome already excludes it.
	result, err := s.repo.Spin(ctx, userID, input, cfg.SlotBetCredits, outcome)
	if err != nil {
		return nil, err
	}
	if result.BonusRound != nil && result.BonusRound.Status == "pending" {
		result.BonusRound.Token = deriveSlotBonusToken(userID, key)
	}
	return result, nil
}

func (s *GameLoyaltyService) ListRewards(ctx context.Context, userID int64) ([]GameLoyaltyRewardView, int, *GameLoyaltyConfig, error) {
	cfg := s.getConfig(ctx)
	localDate := timezone.Today().Format("2006-01-02")
	if !cfg.Enabled || !cfg.CatalogValid {
		return []GameLoyaltyRewardView{}, 0, &cfg, nil
	}
	views, claimed, err := s.repo.ListRewardsStatus(ctx, userID, localDate, cfg.Catalog, cfg.DailyRewardLimit)
	if err != nil {
		return nil, 0, &cfg, err
	}
	return views, claimed, &cfg, nil
}

func (s *GameLoyaltyService) ClaimReward(ctx context.Context, userID int64, idempotencyKey, rewardID string) (*GameLoyaltyRewardClaimResult, error) {
	key, err := normalizeGameWalletIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, err
	}
	rewardID = strings.TrimSpace(rewardID)
	if rewardID == "" || utf8.RuneCountInString(rewardID) > gameLoyaltyMaxRewardIDLen {
		return nil, ErrGameLoyaltyInvalidRewardID
	}
	cfg := s.getConfig(ctx)
	if !cfg.Enabled {
		return nil, ErrGameLoyaltyDisabled
	}
	if !cfg.Available() {
		return nil, ErrGameLoyaltyNotConfigured
	}

	var item *GameLoyaltyRewardItem
	for i := range cfg.Catalog {
		if cfg.Catalog[i].ID == rewardID && cfg.Catalog[i].Enabled {
			item = &cfg.Catalog[i]
			break
		}
	}
	if item == nil {
		return nil, ErrGameLoyaltyRewardNotFound
	}

	hash := sha256.Sum256([]byte("reward_claim\n" + rewardID + "\n" + strconv.FormatInt(item.CreditCost, 10) + "\n" + item.VoucherValue))
	input := GameLoyaltyRewardClaimInput{
		RewardID:       rewardID,
		IdempotencyKey: key,
		RequestHash:    hex.EncodeToString(hash[:]),
	}

	code, err := s.generateGameTokenCode()
	if err != nil {
		return nil, fmt.Errorf("generate game token code: %w", err)
	}
	var expiresAt *time.Time
	if item.ExpiryDays > 0 {
		t := timezone.Now().Add(time.Duration(item.ExpiryDays) * 24 * time.Hour)
		expiresAt = &t
	}
	localDate := timezone.Today().Format("2006-01-02")
	result, err := s.repo.ClaimReward(ctx, userID, localDate, *item, cfg.DailyRewardLimit, code, expiresAt, input)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GiftCredits atomically moves game credits to the account identified by email.
func (s *GameLoyaltyService) GiftCredits(
	ctx context.Context,
	senderUserID int64,
	idempotencyKey, recipientEmail, rawCredits string,
) (*GameCreditGiftResult, error) {
	key, err := normalizeGameWalletIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, err
	}
	normalizedEmail, err := normalizeGameCreditGiftEmail(recipientEmail)
	if err != nil {
		return nil, err
	}
	credits, err := normalizeGameLoyaltyCredits(rawCredits, false)
	if err != nil {
		return nil, ErrGameCreditGiftAmountInvalid
	}
	if !s.getConfig(ctx).Enabled {
		return nil, ErrGameLoyaltyDisabled
	}
	repo, ok := s.repo.(GameCreditGiftRepository)
	if !ok {
		return nil, ErrGameCreditGiftUnavailable
	}
	hash := sha256.Sum256([]byte("gift\n" + normalizedEmail + "\n" + strconv.FormatInt(credits, 10)))
	return repo.GiftCredits(ctx, senderUserID, GameCreditGiftInput{
		RecipientEmail: normalizedEmail,
		Credits:        credits,
		IdempotencyKey: key,
		RequestHash:    hex.EncodeToString(hash[:]),
	})
}

func (s *GameLoyaltyService) ListTransactions(ctx context.Context, userID int64, limit int, beforeID int64) ([]GameLoyaltyLedgerEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if beforeID < 0 {
		return nil, infraerrors.BadRequest("GAME_LOYALTY_CURSOR_INVALID", "before_id must be positive")
	}
	return s.repo.ListLedger(ctx, userID, limit, beforeID)
}

func (s *GameLoyaltyService) generateGameTokenCode() (string, error) {
	// GT + 28 hex chars gives 112 bits of entropy and fits VARCHAR(32).
	var buf [14]byte
	if _, err := io.ReadFull(s.codeRand, buf[:]); err != nil {
		return "", err
	}
	return GameRewardCodePrefix + strings.ToUpper(hex.EncodeToString(buf[:])), nil
}

func (s *GameLoyaltyService) getConfig(ctx context.Context) GameLoyaltyConfig {
	if s.settingService == nil {
		return defaultGameLoyaltyConfig()
	}
	return s.settingService.GetGameLoyaltyConfig(ctx)
}

func (s *SettingService) GetGameLoyaltyConfig(ctx context.Context) GameLoyaltyConfig {
	cfg := defaultGameLoyaltyConfig()
	if s == nil || s.settingRepo == nil {
		return cfg
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyGameLoyaltyEnabled,
		SettingKeyGameLoyaltyCheckinCredits,
		SettingKeyGameLoyaltySlotBetCredits,
		SettingKeyGameLoyaltyDailyRewardLimit,
		SettingKeyGameLoyaltyRewardCatalog,
	})
	if err != nil {
		return cfg
	}
	cfg.Enabled = values[SettingKeyGameLoyaltyEnabled] == "true"

	if credits, normErr := normalizeGameLoyaltyCredits(values[SettingKeyGameLoyaltyCheckinCredits], true); normErr == nil {
		cfg.CheckinCredits = credits
		cfg.CheckinConfigured = true
	} else {
		cfg.CheckinConfigured = false
	}
	if bet, normErr := normalizeGameLoyaltySlotBet(values[SettingKeyGameLoyaltySlotBetCredits], true); normErr == nil {
		cfg.SlotBetCredits = bet
		cfg.SlotConfigured = true
	} else {
		cfg.SlotConfigured = false
	}
	if limit, normErr := normalizeGameLoyaltyNonNegInt(values[SettingKeyGameLoyaltyDailyRewardLimit]); normErr == nil {
		cfg.DailyRewardLimit = limit
		cfg.DailyLimitValid = true
	} else {
		cfg.DailyLimitValid = false
	}
	catalogRaw := values[SettingKeyGameLoyaltyRewardCatalog]
	if catalogRaw == "" {
		catalogRaw = "[]"
	}
	if items, catErr := ParseGameLoyaltyRewardCatalog(catalogRaw); catErr == nil {
		cfg.Catalog = items
		cfg.CatalogValid = true
	} else {
		cfg.CatalogValid = false
	}
	return cfg
}

func defaultGameLoyaltyConfig() GameLoyaltyConfig {
	return GameLoyaltyConfig{
		CheckinCredits:    0,
		SlotBetCredits:    0,
		DailyRewardLimit:  0,
		Catalog:           []GameLoyaltyRewardItem{},
		CheckinConfigured: true,
		SlotConfigured:    true,
		CatalogValid:      true,
		DailyLimitValid:   true,
	}
}

// NormalizeGameLoyaltySettingValues canonicalizes raw setting strings.
func NormalizeGameLoyaltySettingValues(checkin, bet, dailyLimit, catalog string) (string, string, string, string, error) {
	c, err := normalizeGameLoyaltyCredits(checkin, true)
	if err != nil {
		return "", "", "", "", infraerrors.BadRequest("GAME_LOYALTY_CHECKIN_INVALID", "game loyalty check-in credits must be a non-negative integer")
	}
	b, err := normalizeGameLoyaltySlotBet(bet, true)
	if err != nil {
		return "", "", "", "", infraerrors.BadRequest("GAME_LOYALTY_SLOT_BET_INVALID", "game loyalty slot bet credits must be an integer between 0 and 10000")
	}
	limit, err := normalizeGameLoyaltyNonNegInt(dailyLimit)
	if err != nil {
		return "", "", "", "", infraerrors.BadRequest("GAME_LOYALTY_DAILY_REWARD_LIMIT_INVALID", "game loyalty daily reward limit must be a non-negative integer")
	}
	items, err := ParseGameLoyaltyRewardCatalog(catalog)
	if err != nil {
		return "", "", "", "", err
	}
	canonicalCatalog, err := marshalGameLoyaltyRewardCatalog(items)
	if err != nil {
		return "", "", "", "", infraerrors.BadRequest("GAME_LOYALTY_CATALOG_INVALID", err.Error())
	}
	return strconv.FormatInt(c, 10), strconv.FormatInt(b, 10), strconv.Itoa(limit), canonicalCatalog, nil
}

// ValidateGameLoyaltySettingValues enforces fail-closed enable rules.
func ValidateGameLoyaltySettingValues(enabled bool, checkin, bet, dailyLimit, catalog string) (string, string, string, string, error) {
	c, b, limit, cat, err := NormalizeGameLoyaltySettingValues(checkin, bet, dailyLimit, catalog)
	if err != nil {
		return "", "", "", "", err
	}
	if enabled {
		ci, _ := strconv.ParseInt(c, 10, 64)
		bi, _ := strconv.ParseInt(b, 10, 64)
		if ci <= 0 {
			return "", "", "", "", infraerrors.BadRequest(
				"GAME_LOYALTY_CHECKIN_REQUIRED",
				"game loyalty check-in credits must be positive when loyalty is enabled",
			)
		}
		if bi <= 0 {
			return "", "", "", "", infraerrors.BadRequest(
				"GAME_LOYALTY_SLOT_BET_REQUIRED",
				"game loyalty slot bet credits must be positive when loyalty is enabled",
			)
		}
	}
	return c, b, limit, cat, nil
}

func normalizeGameLoyaltyCredits(raw string, allowZero bool) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" || len(value) > 19 {
		return 0, fmt.Errorf("invalid credits")
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid credits")
		}
	}
	credits, err := strconv.ParseInt(value, 10, 64)
	if err != nil || credits < 0 {
		return 0, fmt.Errorf("credits out of range")
	}
	if credits == 0 && !allowZero {
		return 0, fmt.Errorf("credits must be positive")
	}
	return credits, nil
}

func normalizeGameCreditGiftEmail(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" || len(value) > 320 || strings.ContainsAny(value, "\r\n\t ") {
		return "", ErrGameCreditGiftRecipientInvalid
	}
	address, err := mail.ParseAddress(value)
	if err != nil || !strings.EqualFold(address.Address, value) {
		return "", ErrGameCreditGiftRecipientInvalid
	}
	return value, nil
}

func normalizeGameLoyaltySlotBet(raw string, allowZero bool) (int64, error) {
	credits, err := normalizeGameLoyaltyCredits(raw, allowZero)
	if err != nil {
		return 0, err
	}
	if credits > SlotBetMaxCredits {
		return 0, fmt.Errorf("slot bet exceeds maximum")
	}
	return credits, nil
}

func normalizeGameLoyaltyNonNegInt(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" || len(value) > 10 {
		return 0, fmt.Errorf("invalid int")
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid int")
		}
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("int out of range")
	}
	return n, nil
}

// ParseGameLoyaltyRewardCatalog validates catalog JSON into typed items.
func ParseGameLoyaltyRewardCatalog(raw string) ([]GameLoyaltyRewardItem, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "[]"
	}
	if len(raw) > gameLoyaltyMaxCatalogJSON {
		return nil, infraerrors.BadRequest("GAME_LOYALTY_CATALOG_INVALID", "reward catalog is too large")
	}
	var rows []map[string]any
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&rows); err != nil {
		return nil, infraerrors.BadRequest("GAME_LOYALTY_CATALOG_INVALID", "reward catalog must be a JSON array")
	}
	if len(rows) > gameLoyaltyMaxCatalogItems {
		return nil, infraerrors.BadRequest("GAME_LOYALTY_CATALOG_INVALID", "reward catalog has too many items")
	}
	items := make([]GameLoyaltyRewardItem, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for i, row := range rows {
		item, err := parseGameLoyaltyRewardRow(row, i)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[item.ID]; exists {
			return nil, infraerrors.BadRequest("GAME_LOYALTY_CATALOG_INVALID", "duplicate reward id: "+item.ID)
		}
		seen[item.ID] = struct{}{}
		items = append(items, item)
	}
	return items, nil
}

func parseGameLoyaltyRewardRow(row map[string]any, index int) (GameLoyaltyRewardItem, error) {
	prefix := fmt.Sprintf("reward[%d]", index)
	id, err := requireCatalogString(row, "id", true)
	if err != nil {
		return GameLoyaltyRewardItem{}, infraerrors.BadRequest("GAME_LOYALTY_CATALOG_INVALID", prefix+": "+err.Error())
	}
	if utf8.RuneCountInString(id) > gameLoyaltyMaxRewardIDLen {
		return GameLoyaltyRewardItem{}, infraerrors.BadRequest("GAME_LOYALTY_CATALOG_INVALID", prefix+": id too long")
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return GameLoyaltyRewardItem{}, infraerrors.BadRequest("GAME_LOYALTY_CATALOG_INVALID", prefix+": id has invalid characters")
	}
	title, err := requireCatalogString(row, "title", true)
	if err != nil {
		return GameLoyaltyRewardItem{}, infraerrors.BadRequest("GAME_LOYALTY_CATALOG_INVALID", prefix+": "+err.Error())
	}
	if utf8.RuneCountInString(title) > gameLoyaltyMaxTitleLen {
		return GameLoyaltyRewardItem{}, infraerrors.BadRequest("GAME_LOYALTY_CATALOG_INVALID", prefix+": title too long")
	}
	creditCost, err := requireCatalogInt64(row, "credit_cost", false)
	if err != nil {
		return GameLoyaltyRewardItem{}, infraerrors.BadRequest("GAME_LOYALTY_CATALOG_INVALID", prefix+": "+err.Error())
	}
	voucherValue, err := requireCatalogDecimal(row, "voucher_value")
	if err != nil {
		return GameLoyaltyRewardItem{}, infraerrors.BadRequest("GAME_LOYALTY_CATALOG_INVALID", prefix+": "+err.Error())
	}
	dailyStock, err := requireCatalogInt(row, "daily_stock", false)
	if err != nil {
		return GameLoyaltyRewardItem{}, infraerrors.BadRequest("GAME_LOYALTY_CATALOG_INVALID", prefix+": "+err.Error())
	}
	expiryDays := 0
	if raw, ok := row["expiry_days"]; ok && raw != nil {
		expiryDays, err = requireCatalogInt(row, "expiry_days", true)
		if err != nil {
			return GameLoyaltyRewardItem{}, infraerrors.BadRequest("GAME_LOYALTY_CATALOG_INVALID", prefix+": "+err.Error())
		}
	}
	enabled := true
	if raw, ok := row["enabled"]; ok && raw != nil {
		b, ok := raw.(bool)
		if !ok {
			return GameLoyaltyRewardItem{}, infraerrors.BadRequest("GAME_LOYALTY_CATALOG_INVALID", prefix+": enabled must be boolean")
		}
		enabled = b
	}
	localized := map[string]string{}
	if raw, ok := row["titles"]; ok && raw != nil {
		m, ok := raw.(map[string]any)
		if !ok {
			return GameLoyaltyRewardItem{}, infraerrors.BadRequest("GAME_LOYALTY_CATALOG_INVALID", prefix+": titles must be an object")
		}
		for k, v := range m {
			s, ok := v.(string)
			if !ok {
				return GameLoyaltyRewardItem{}, infraerrors.BadRequest("GAME_LOYALTY_CATALOG_INVALID", prefix+": titles values must be strings")
			}
			localized[strings.TrimSpace(k)] = strings.TrimSpace(s)
		}
	}
	return GameLoyaltyRewardItem{
		ID:              id,
		Title:           title,
		CreditCost:      creditCost,
		VoucherValue:    voucherValue,
		DailyStock:      dailyStock,
		ExpiryDays:      expiryDays,
		Enabled:         enabled,
		LocalizedTitles: localized,
	}, nil
}

func requireCatalogString(row map[string]any, key string, nonEmpty bool) (string, error) {
	raw, ok := row[key]
	if !ok || raw == nil {
		return "", fmt.Errorf("%s is required", key)
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	s = strings.TrimSpace(s)
	if nonEmpty && s == "" {
		return "", fmt.Errorf("%s must not be empty", key)
	}
	return s, nil
}

func requireCatalogInt64(row map[string]any, key string, allowZero bool) (int64, error) {
	raw, ok := row[key]
	if !ok || raw == nil {
		return 0, fmt.Errorf("%s is required", key)
	}
	switch v := raw.(type) {
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("%s must be an integer", key)
		}
		if n < 0 || (!allowZero && n == 0) {
			return 0, fmt.Errorf("%s must be positive", key)
		}
		return n, nil
	case string:
		return normalizeGameLoyaltyCredits(v, allowZero)
	case float64:
		return 0, fmt.Errorf("%s must be an exact integer string or number", key)
	default:
		return 0, fmt.Errorf("%s must be an integer", key)
	}
}

func requireCatalogInt(row map[string]any, key string, allowZero bool) (int, error) {
	n, err := requireCatalogInt64(row, key, allowZero)
	if err != nil {
		return 0, err
	}
	if n > int64(^uint(0)>>1) {
		return 0, fmt.Errorf("%s out of range", key)
	}
	return int(n), nil
}

func requireCatalogDecimal(row map[string]any, key string) (string, error) {
	raw, ok := row[key]
	if !ok || raw == nil {
		return "", fmt.Errorf("%s is required", key)
	}
	var text string
	switch v := raw.(type) {
	case string:
		text = v
	case json.Number:
		text = v.String()
	default:
		return "", fmt.Errorf("%s must be an exact decimal string", key)
	}
	// Reuse the exchange decimal normalizer (no float path).
	return normalizeGameWalletDecimal(text, false)
}

func marshalGameLoyaltyRewardCatalog(items []GameLoyaltyRewardItem) (string, error) {
	type wireItem struct {
		ID           string            `json:"id"`
		Title        string            `json:"title"`
		CreditCost   string            `json:"credit_cost"`
		VoucherValue string            `json:"voucher_value"`
		DailyStock   int               `json:"daily_stock"`
		ExpiryDays   int               `json:"expiry_days,omitempty"`
		Enabled      bool              `json:"enabled"`
		Titles       map[string]string `json:"titles,omitempty"`
	}
	out := make([]wireItem, 0, len(items))
	for _, item := range items {
		row := wireItem{
			ID:           item.ID,
			Title:        item.Title,
			CreditCost:   strconv.FormatInt(item.CreditCost, 10),
			VoucherValue: item.VoucherValue,
			DailyStock:   item.DailyStock,
			ExpiryDays:   item.ExpiryDays,
			Enabled:      item.Enabled,
		}
		if len(item.LocalizedTitles) > 0 {
			row.Titles = item.LocalizedTitles
		}
		out = append(out, row)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
