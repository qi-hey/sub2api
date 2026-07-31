package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	GameWalletDirectionBalanceToCredits = "balance_to_credits"
	GameWalletDirectionCreditsToBalance = "credits_to_balance"

	gameWalletMaxDecimal = "999999999999.99999999"
	gameWalletMaxScale   = 8
	gameWalletMaxWhole   = 12
	gameWalletMaxRawSize = 32
)

var (
	ErrGameWalletDisabled = infraerrors.Forbidden(
		"GAME_WALLET_DISABLED", "game wallet exchange is disabled",
	)
	ErrGameWalletRateNotConfigured = infraerrors.Forbidden(
		"GAME_WALLET_RATE_NOT_CONFIGURED", "game wallet exchange rate is not configured",
	)
	ErrGameWalletConfigInvalid = infraerrors.New(
		500, "GAME_WALLET_CONFIG_INVALID", "game wallet exchange configuration is invalid",
	)
	ErrGameWalletIdempotencyKeyRequired = infraerrors.BadRequest(
		"IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header is required",
	)
	ErrGameWalletIdempotencyKeyInvalid = infraerrors.BadRequest(
		"IDEMPOTENCY_KEY_INVALID", "Idempotency-Key must be 1-128 visible ASCII characters",
	)
	ErrGameWalletIdempotencyConflict = infraerrors.Conflict(
		"IDEMPOTENCY_KEY_CONFLICT", "Idempotency-Key was already used with a different payload",
	)
	ErrGameWalletInvalidDirection = infraerrors.BadRequest(
		"GAME_WALLET_DIRECTION_INVALID", "direction must be balance_to_credits or credits_to_balance",
	)
	ErrGameWalletInvalidAmount = infraerrors.BadRequest(
		"GAME_WALLET_AMOUNT_INVALID", "exchange amount must be positive and within the supported precision",
	)
	ErrGameWalletAmountNotConvertible = infraerrors.BadRequest(
		"GAME_WALLET_AMOUNT_NOT_CONVERTIBLE", "amount cannot be converted exactly at the configured exchange rate",
	)
	ErrGameWalletInsufficientBalance = infraerrors.Conflict(
		"GAME_WALLET_INSUFFICIENT_BALANCE", "account balance is insufficient",
	)
	ErrGameWalletAccountOverdrawn = infraerrors.Conflict(
		"GAME_WALLET_ACCOUNT_OVERDRAWN", "account balance must not be negative before exchange",
	)
	ErrGameWalletInsufficientCredits = infraerrors.Conflict(
		"GAME_WALLET_INSUFFICIENT_CREDITS", "game wallet credits are insufficient",
	)
	ErrGameWalletDailyLimitExceeded = infraerrors.Conflict(
		"GAME_WALLET_DAILY_LIMIT_EXCEEDED", "game wallet daily exchange limit would be exceeded",
	)
	ErrGameWalletDailyTransactionLimitExceeded = infraerrors.TooManyRequests(
		"GAME_WALLET_DAILY_TRANSACTION_LIMIT_EXCEEDED", "too many game wallet exchanges today",
	)
	ErrGameWalletBalanceLimitExceeded = infraerrors.Conflict(
		"GAME_WALLET_BALANCE_LIMIT_EXCEEDED", "account balance limit would be exceeded",
	)
	ErrGameWalletCreditsLimitExceeded = infraerrors.Conflict(
		"GAME_WALLET_CREDITS_LIMIT_EXCEEDED", "game wallet credits limit would be exceeded",
	)
	ErrGameWalletUserNotFound = infraerrors.NotFound(
		"GAME_WALLET_USER_NOT_FOUND", "user not found",
	)
)

type GameWalletExchangeConfig struct {
	Enabled         bool
	Rate            string
	DailyLimit      string
	RateConfigured  bool
	DailyLimitValid bool
}

type GameWalletSnapshot struct {
	UserID          int64
	AccountBalance  string
	Credits         int64
	DailyExchanged  string
	WalletCreatedAt time.Time
	WalletUpdatedAt time.Time
}

type GameWalletState struct {
	UserID            int64
	AccountBalance    string
	Credits           int64
	ExchangeEnabled   bool
	ExchangeAvailable bool
	ExchangeRate      string
	DailyLimit        string
	DailyExchanged    string
	DailyRemaining    *string
	WalletCreatedAt   time.Time
	WalletUpdatedAt   time.Time
}

type GameWalletExchangeRequest struct {
	Direction     string
	BalanceAmount *string
	CreditsAmount *string
}

type GameWalletExchangeInput struct {
	Direction      string
	BalanceAmount  string
	CreditsAmount  int64
	IdempotencyKey string
	RequestHash    string
}

type GameWalletTransaction struct {
	ID               int64
	UserID           int64
	Direction        string
	BalanceAmount    string
	CreditsAmount    int64
	ExchangeRate     string
	BalanceBefore    string
	CreditsBefore    int64
	BalanceAfter     string
	CreditsAfter     int64
	IdempotencyKey   string
	CreatedAt        time.Time
	IdempotentReplay bool
}

type GameWalletRepository interface {
	GetSnapshot(ctx context.Context, userID int64) (*GameWalletSnapshot, error)
	Exchange(ctx context.Context, userID int64, input GameWalletExchangeInput, cfg GameWalletExchangeConfig) (*GameWalletTransaction, error)
	ListTransactions(ctx context.Context, userID int64, limit int, beforeID int64) ([]GameWalletTransaction, error)
}

type GameWalletBalanceCache interface {
	InvalidateUserBalance(ctx context.Context, userID int64) error
}

type GameWalletService struct {
	repo                 GameWalletRepository
	settingService       *SettingService
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCache         GameWalletBalanceCache
}

func NewGameWalletService(
	repo GameWalletRepository,
	settingService *SettingService,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	billingCache GameWalletBalanceCache,
) *GameWalletService {
	return &GameWalletService{
		repo:                 repo,
		settingService:       settingService,
		authCacheInvalidator: authCacheInvalidator,
		billingCache:         billingCache,
	}
}

func (s *GameWalletService) GetWallet(ctx context.Context, userID int64) (*GameWalletState, error) {
	snapshot, err := s.repo.GetSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	cfg := s.getConfig(ctx)

	state := &GameWalletState{
		UserID:            snapshot.UserID,
		AccountBalance:    snapshot.AccountBalance,
		Credits:           snapshot.Credits,
		ExchangeEnabled:   cfg.Enabled,
		ExchangeAvailable: cfg.Enabled && cfg.RateConfigured && cfg.DailyLimitValid,
		ExchangeRate:      cfg.Rate,
		DailyLimit:        cfg.DailyLimit,
		DailyExchanged:    snapshot.DailyExchanged,
		WalletCreatedAt:   snapshot.WalletCreatedAt,
		WalletUpdatedAt:   snapshot.WalletUpdatedAt,
	}
	if cfg.DailyLimitValid && cfg.DailyLimit != "0" {
		limit, limitErr := decimal.NewFromString(cfg.DailyLimit)
		used, usedErr := decimal.NewFromString(snapshot.DailyExchanged)
		if limitErr == nil && usedErr == nil {
			remaining := limit.Sub(used)
			if remaining.IsNegative() {
				remaining = decimal.Zero
			}
			value := remaining.String()
			state.DailyRemaining = &value
		}
	}
	return state, nil
}

func (s *GameWalletService) Exchange(ctx context.Context, userID int64, idempotencyKey string, req GameWalletExchangeRequest) (*GameWalletTransaction, error) {
	key, err := normalizeGameWalletIdempotencyKey(idempotencyKey)
	if err != nil {
		return nil, err
	}

	input, err := normalizeGameWalletExchangeRequest(req)
	if err != nil {
		return nil, err
	}
	input.IdempotencyKey = key
	hash := sha256.Sum256([]byte(input.Direction + "\n" + inputPayloadAmount(input)))
	input.RequestHash = hex.EncodeToString(hash[:])

	result, err := s.repo.Exchange(ctx, userID, input, s.getConfig(ctx))
	if err != nil {
		return nil, err
	}
	s.invalidateBalanceCaches(userID)
	return result, nil
}

func (s *GameWalletService) invalidateBalanceCaches(userID int64) {
	cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(cacheCtx, userID)
	}
	if s.billingCache != nil {
		if err := s.billingCache.InvalidateUserBalance(cacheCtx, userID); err != nil {
			slog.Error("invalidate game wallet balance cache failed", "user_id", userID, "error", err)
		}
	}
}

func (s *GameWalletService) ListTransactions(ctx context.Context, userID int64, limit int, beforeID int64) ([]GameWalletTransaction, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if beforeID < 0 {
		return nil, infraerrors.BadRequest("GAME_WALLET_CURSOR_INVALID", "before_id must be positive")
	}
	return s.repo.ListTransactions(ctx, userID, limit, beforeID)
}

func (s *GameWalletService) getConfig(ctx context.Context) GameWalletExchangeConfig {
	if s.settingService == nil {
		return defaultGameWalletExchangeConfig()
	}
	return s.settingService.GetGameWalletExchangeConfig(ctx)
}

func (s *SettingService) GetGameWalletExchangeConfig(ctx context.Context) GameWalletExchangeConfig {
	cfg := defaultGameWalletExchangeConfig()
	if s == nil || s.settingRepo == nil {
		return cfg
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyGameWalletEnabled,
		SettingKeyGameWalletExchangeRate,
		SettingKeyGameWalletDailyLimit,
	})
	if err != nil {
		return cfg
	}
	cfg.Enabled = values[SettingKeyGameWalletEnabled] == "true"
	if rate, normalizeErr := normalizeGameWalletRate(values[SettingKeyGameWalletExchangeRate]); normalizeErr == nil {
		cfg.Rate = rate
		cfg.RateConfigured = rate != "0"
	}
	if rawLimit, exists := values[SettingKeyGameWalletDailyLimit]; exists {
		cfg.DailyLimitValid = false
		if limit, normalizeErr := normalizeGameWalletDecimal(rawLimit, true); normalizeErr == nil {
			cfg.DailyLimit = limit
			cfg.DailyLimitValid = true
		}
	}
	return cfg
}

func NormalizeGameWalletSettingValues(rate, dailyLimit string) (string, string, error) {
	normalizedRate, err := normalizeGameWalletRate(rate)
	if err != nil {
		return "", "", infraerrors.BadRequest("GAME_WALLET_RATE_INVALID", "game wallet exchange rate must be a non-negative integer")
	}
	normalizedLimit, err := normalizeGameWalletDecimal(dailyLimit, true)
	if err != nil {
		return "", "", infraerrors.BadRequest("GAME_WALLET_DAILY_LIMIT_INVALID", "game wallet daily limit must be a non-negative decimal with at most 8 fractional digits")
	}
	return normalizedRate, normalizedLimit, nil
}

func ValidateGameWalletSettingValues(enabled bool, rate, dailyLimit string) (string, string, error) {
	normalizedRate, normalizedLimit, err := NormalizeGameWalletSettingValues(rate, dailyLimit)
	if err != nil {
		return "", "", err
	}
	if enabled && normalizedRate == "0" {
		return "", "", infraerrors.BadRequest(
			"GAME_WALLET_RATE_REQUIRED",
			"game wallet exchange rate must be positive when game wallet exchange is enabled",
		)
	}
	return normalizedRate, normalizedLimit, nil
}

func defaultGameWalletExchangeConfig() GameWalletExchangeConfig {
	return GameWalletExchangeConfig{
		Rate:            "0",
		DailyLimit:      "0",
		DailyLimitValid: true,
	}
}

func normalizeGameWalletExchangeRequest(req GameWalletExchangeRequest) (GameWalletExchangeInput, error) {
	input := GameWalletExchangeInput{Direction: strings.TrimSpace(req.Direction)}
	switch input.Direction {
	case GameWalletDirectionBalanceToCredits:
		if req.BalanceAmount == nil || req.CreditsAmount != nil {
			return input, ErrGameWalletInvalidAmount
		}
		amount, err := normalizeGameWalletDecimal(*req.BalanceAmount, false)
		if err != nil {
			return input, ErrGameWalletInvalidAmount
		}
		input.BalanceAmount = amount
	case GameWalletDirectionCreditsToBalance:
		if req.CreditsAmount == nil || req.BalanceAmount != nil {
			return input, ErrGameWalletInvalidAmount
		}
		credits, err := normalizeGameWalletCredits(*req.CreditsAmount)
		if err != nil {
			return input, ErrGameWalletInvalidAmount
		}
		input.CreditsAmount = credits
	default:
		return input, ErrGameWalletInvalidDirection
	}
	return input, nil
}

func normalizeGameWalletIdempotencyKey(raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if key == "" {
		return "", ErrGameWalletIdempotencyKeyRequired
	}
	if len(key) > 128 {
		return "", ErrGameWalletIdempotencyKeyInvalid
	}
	for _, r := range key {
		if r < 0x21 || r > 0x7e {
			return "", ErrGameWalletIdempotencyKeyInvalid
		}
	}
	return key, nil
}

func normalizeGameWalletDecimal(raw string, allowZero bool) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || len(value) > gameWalletMaxRawSize {
		return "", fmt.Errorf("invalid decimal")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && parts[1] == "") {
		return "", fmt.Errorf("invalid decimal")
	}
	for _, part := range parts {
		for _, r := range part {
			if r < '0' || r > '9' {
				return "", fmt.Errorf("invalid decimal")
			}
		}
	}
	if len(parts) == 2 && len(parts[1]) > gameWalletMaxScale {
		return "", fmt.Errorf("decimal scale exceeds %d", gameWalletMaxScale)
	}
	whole := strings.TrimLeft(parts[0], "0")
	if whole == "" {
		whole = "0"
	}
	if len(whole) > gameWalletMaxWhole {
		return "", fmt.Errorf("decimal exceeds maximum")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = strings.TrimRight(parts[1], "0")
	}
	canonical := whole
	if fraction != "" {
		canonical += "." + fraction
	}
	if canonical == "0" && !allowZero {
		return "", fmt.Errorf("decimal must be positive")
	}
	if canonical == gameWalletMaxDecimal {
		return canonical, nil
	}
	return canonical, nil
}

func normalizeGameWalletCredits(raw string) (int64, error) {
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
	if err != nil || credits <= 0 {
		return 0, fmt.Errorf("credits are out of range")
	}
	return credits, nil
}

func normalizeGameWalletRate(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || len(value) > 19 {
		return "", fmt.Errorf("empty exchange rate")
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("exchange rate is not an integer")
		}
	}
	rate, err := strconv.ParseInt(value, 10, 64)
	if err != nil || rate < 0 {
		return "", fmt.Errorf("exchange rate is out of range")
	}
	return strconv.FormatInt(rate, 10), nil
}

func inputPayloadAmount(input GameWalletExchangeInput) string {
	if input.Direction == GameWalletDirectionBalanceToCredits {
		return input.BalanceAmount
	}
	return fmt.Sprintf("%d", input.CreditsAmount)
}
