package service

import (
	"context"
	"strings"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type gameWalletServiceRepoStub struct {
	exchangeInput  GameWalletExchangeInput
	exchangeConfig GameWalletExchangeConfig
	snapshot       *GameWalletSnapshot
}

type gameWalletAuthCacheStub struct {
	invalidatedUserIDs []int64
}

func (s *gameWalletAuthCacheStub) InvalidateAuthCacheByKey(context.Context, string)    {}
func (s *gameWalletAuthCacheStub) InvalidateAuthCacheByGroupID(context.Context, int64) {}
func (s *gameWalletAuthCacheStub) InvalidateAuthCacheByUserID(_ context.Context, userID int64) {
	s.invalidatedUserIDs = append(s.invalidatedUserIDs, userID)
}

type gameWalletBalanceCacheStub struct {
	invalidatedUserIDs []int64
}

func (s *gameWalletBalanceCacheStub) InvalidateUserBalance(_ context.Context, userID int64) error {
	s.invalidatedUserIDs = append(s.invalidatedUserIDs, userID)
	return nil
}

func (s *gameWalletServiceRepoStub) GetSnapshot(context.Context, int64) (*GameWalletSnapshot, error) {
	return s.snapshot, nil
}

func (s *gameWalletServiceRepoStub) Exchange(_ context.Context, userID int64, input GameWalletExchangeInput, cfg GameWalletExchangeConfig) (*GameWalletTransaction, error) {
	s.exchangeInput = input
	s.exchangeConfig = cfg
	return &GameWalletTransaction{ID: 1, UserID: userID}, nil
}

func (s *gameWalletServiceRepoStub) ListTransactions(context.Context, int64, int, int64) ([]GameWalletTransaction, error) {
	return nil, nil
}

type gameWalletSettingRepoStub struct {
	values map[string]string
	err    error
}

func (s *gameWalletSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (s *gameWalletSettingRepoStub) GetValue(context.Context, string) (string, error) {
	return "", ErrSettingNotFound
}
func (s *gameWalletSettingRepoStub) Set(context.Context, string, string) error { return nil }
func (s *gameWalletSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return s.values, s.err
}
func (s *gameWalletSettingRepoStub) SetMultiple(context.Context, map[string]string) error { return nil }
func (s *gameWalletSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return s.values, s.err
}
func (s *gameWalletSettingRepoStub) Delete(context.Context, string) error { return nil }

func TestGameWalletSettingsRequireIntegerRate(t *testing.T) {
	rate, limit, err := NormalizeGameWalletSettingValues("00100", "25.50000000")
	require.NoError(t, err)
	require.Equal(t, "100", rate)
	require.Equal(t, "25.5", limit)

	_, _, err = NormalizeGameWalletSettingValues("1.5", "25")
	require.Error(t, err)
	require.Equal(t, "GAME_WALLET_RATE_INVALID", infraerrors.Reason(err))

	_, _, err = ValidateGameWalletSettingValues(true, "0", "0")
	require.Error(t, err)
	require.Equal(t, "GAME_WALLET_RATE_REQUIRED", infraerrors.Reason(err))

	rate, limit, err = ValidateGameWalletSettingValues(false, "0", "0")
	require.NoError(t, err)
	require.Equal(t, "0", rate)
	require.Equal(t, "0", limit)
}

func TestGameWalletExchangeNormalizesPayloadBeforeHashing(t *testing.T) {
	repo := &gameWalletServiceRepoStub{}
	svc := NewGameWalletService(repo, nil, nil, nil)
	amount := "01.25000000"

	_, err := svc.Exchange(context.Background(), 7, "wallet-request-1", GameWalletExchangeRequest{
		Direction:     GameWalletDirectionBalanceToCredits,
		BalanceAmount: &amount,
	})
	require.NoError(t, err)
	require.Equal(t, "1.25", repo.exchangeInput.BalanceAmount)
	require.Len(t, repo.exchangeInput.RequestHash, 64)
	require.False(t, repo.exchangeConfig.Enabled, "repository must receive disabled config so it can still replay an existing idempotent result")
	require.Equal(t, "0", repo.exchangeConfig.Rate)
}

func TestGameWalletExchangeRejectsAmbiguousOrOverPreciseAmounts(t *testing.T) {
	repo := &gameWalletServiceRepoStub{}
	svc := NewGameWalletService(repo, nil, nil, nil)
	balance := "1.000000001"
	credits := "1"

	_, err := svc.Exchange(context.Background(), 7, "wallet-request-2", GameWalletExchangeRequest{
		Direction:     GameWalletDirectionBalanceToCredits,
		BalanceAmount: &balance,
	})
	require.ErrorIs(t, err, ErrGameWalletInvalidAmount)

	balance = "1"
	_, err = svc.Exchange(context.Background(), 7, "wallet-request-3", GameWalletExchangeRequest{
		Direction:     GameWalletDirectionCreditsToBalance,
		BalanceAmount: &balance,
		CreditsAmount: &credits,
	})
	require.ErrorIs(t, err, ErrGameWalletInvalidAmount)
}

func TestGameWalletExchangeRejectsExponentAmountsBeforeDecimalParsing(t *testing.T) {
	repo := &gameWalletServiceRepoStub{}
	svc := NewGameWalletService(repo, nil, nil, nil)

	for _, amount := range []string{"1e-2147483647", "1E9", "+1", "-1", ".5", "1.", strings.Repeat("0", 33)} {
		_, err := svc.Exchange(context.Background(), 7, "wallet-request-exponent", GameWalletExchangeRequest{
			Direction:     GameWalletDirectionBalanceToCredits,
			BalanceAmount: &amount,
		})
		require.ErrorIs(t, err, ErrGameWalletInvalidAmount, "amount=%q", amount)
	}
}

func TestGameWalletExchangeInvalidatesBalanceCachesAfterCommit(t *testing.T) {
	repo := &gameWalletServiceRepoStub{}
	authCache := &gameWalletAuthCacheStub{}
	billingCache := &gameWalletBalanceCacheStub{}
	svc := NewGameWalletService(repo, nil, authCache, billingCache)
	amount := "1"

	_, err := svc.Exchange(context.Background(), 42, "wallet-request-cache", GameWalletExchangeRequest{
		Direction:     GameWalletDirectionBalanceToCredits,
		BalanceAmount: &amount,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{42}, authCache.invalidatedUserIDs)
	require.Equal(t, []int64{42}, billingCache.invalidatedUserIDs)
}

func TestGetGameWalletExchangeConfigFailsClosed(t *testing.T) {
	settings := NewSettingService(&gameWalletSettingRepoStub{values: map[string]string{
		SettingKeyGameWalletEnabled:      "true",
		SettingKeyGameWalletExchangeRate: "1.5",
		SettingKeyGameWalletDailyLimit:   "20",
	}}, nil)

	cfg := settings.GetGameWalletExchangeConfig(context.Background())
	require.True(t, cfg.Enabled)
	require.False(t, cfg.RateConfigured)
	require.Equal(t, "0", cfg.Rate)
	require.True(t, cfg.DailyLimitValid)

	settings = NewSettingService(&gameWalletSettingRepoStub{values: map[string]string{
		SettingKeyGameWalletEnabled:      "true",
		SettingKeyGameWalletExchangeRate: "100",
		SettingKeyGameWalletDailyLimit:   "not-a-number",
	}}, nil)
	cfg = settings.GetGameWalletExchangeConfig(context.Background())
	require.True(t, cfg.RateConfigured)
	require.False(t, cfg.DailyLimitValid)
}

func TestGameWalletStateUsesDecimalDailyRemaining(t *testing.T) {
	now := time.Now().UTC()
	repo := &gameWalletServiceRepoStub{snapshot: &GameWalletSnapshot{
		UserID:          7,
		AccountBalance:  "10.00000000",
		Credits:         300,
		DailyExchanged:  "1.25000000",
		WalletCreatedAt: now,
		WalletUpdatedAt: now,
	}}
	settings := NewSettingService(&gameWalletSettingRepoStub{values: map[string]string{
		SettingKeyGameWalletEnabled:      "true",
		SettingKeyGameWalletExchangeRate: "100",
		SettingKeyGameWalletDailyLimit:   "2.5",
	}}, nil)

	state, err := NewGameWalletService(repo, settings, nil, nil).GetWallet(context.Background(), 7)
	require.NoError(t, err)
	require.True(t, state.ExchangeAvailable)
	require.NotNil(t, state.DailyRemaining)
	require.Equal(t, "1.25", *state.DailyRemaining)
}
