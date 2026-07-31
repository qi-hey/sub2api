package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gameLoyaltyHandlerRepoStub struct {
	checkin *service.GameLoyaltyCheckinResult
	spin    *service.GameLoyaltySpinResult
	claim   *service.GameLoyaltyRewardClaimResult
	gift    *service.GameCreditGiftResult
	giftIn  service.GameCreditGiftInput
	pending *service.SlotBonusPendingSnapshot
}

func (s *gameLoyaltyHandlerRepoStub) GetSnapshot(context.Context, int64, string) (*service.GameLoyaltyWalletSnapshot, error) {
	return &service.GameLoyaltyWalletSnapshot{
		UserID: 7, AccountBalance: "3.5", Credits: 120, FreeSpinsRemaining: 2,
		LocalDate: "2026-07-28", WalletCreatedAt: time.Unix(1, 0).UTC(), WalletUpdatedAt: time.Unix(2, 0).UTC(),
	}, nil
}
func (s *gameLoyaltyHandlerRepoStub) CheckIn(context.Context, int64, string, int64, string, string) (*service.GameLoyaltyCheckinResult, error) {
	if s.checkin == nil {
		return &service.GameLoyaltyCheckinResult{ID: 1, UserID: 7, LocalDate: "2026-07-28", CreditsAwarded: 50, CreditsAfter: 50}, nil
	}
	return s.checkin, nil
}
func (s *gameLoyaltyHandlerRepoStub) Spin(context.Context, int64, service.GameLoyaltySpinInput, int64, *service.SlotSpinResult) (*service.GameLoyaltySpinResult, error) {
	if s.spin == nil {
		return &service.GameLoyaltySpinResult{
			ID: 2, UserID: 7, BetCredits: 10, PayoutCredits: 0,
			PaytableVersion: service.SlotPaytableVersionV2, ReelStripVersion: service.SlotReelStripVersionV2,
		}, nil
	}
	return s.spin, nil
}
func (s *gameLoyaltyHandlerRepoStub) ListRewardsStatus(context.Context, int64, string, []service.GameLoyaltyRewardItem, int) ([]service.GameLoyaltyRewardView, int, error) {
	return []service.GameLoyaltyRewardView{}, 0, nil
}
func (s *gameLoyaltyHandlerRepoStub) ClaimReward(context.Context, int64, string, service.GameLoyaltyRewardItem, int, string, *time.Time, service.GameLoyaltyRewardClaimInput) (*service.GameLoyaltyRewardClaimResult, error) {
	return s.claim, nil
}
func (s *gameLoyaltyHandlerRepoStub) GiftCredits(_ context.Context, senderUserID int64, input service.GameCreditGiftInput) (*service.GameCreditGiftResult, error) {
	s.giftIn = input
	if s.gift != nil {
		return s.gift, nil
	}
	return &service.GameCreditGiftResult{
		ID: 4, SenderUserID: senderUserID, RecipientUserID: 8,
		RecipientEmail: input.RecipientEmail, Credits: input.Credits,
		SenderCreditsBefore: 120, SenderCreditsAfter: 95,
		RecipientCreditsBefore: 10, RecipientCreditsAfter: 35,
		IdempotencyKey: input.IdempotencyKey, CreatedAt: time.Unix(4, 0).UTC(),
	}, nil
}
func (s *gameLoyaltyHandlerRepoStub) ListLedger(context.Context, int64, int, int64) ([]service.GameLoyaltyLedgerEntry, error) {
	return []service.GameLoyaltyLedgerEntry{{
		ID: 3, UserID: 7, EntryType: service.GameLedgerEntryCheckin, Amount: 50,
		CreditsBefore: 0, CreditsAfter: 50, IdempotencyKey: "k", CreatedAt: time.Unix(3, 0).UTC(),
	}}, nil
}

func (s *gameLoyaltyHandlerRepoStub) GetPendingSlotBonus(context.Context, int64, time.Time) (*service.SlotBonusPendingSnapshot, error) {
	return s.pending, nil
}

type gameWalletHandlerSettingRepo struct {
	service.SettingRepository
	values map[string]string
}

func (s *gameWalletHandlerSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func TestGameWalletGetHandlerJSONContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &gameLoyaltyHandlerRepoStub{}
	h := NewGameWalletHandler(service.NewGameLoyaltyService(repo, nil, nil, nil))
	r := gin.New()
	r.GET("/wallet", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
		h.GetWallet(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/wallet", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]any)
	require.Equal(t, "120", data["credits"])
	require.Equal(t, float64(2), data["free_spins_remaining"])
	require.Equal(t, false, data["exchange_enabled"])
	require.Equal(t, false, data["loyalty_enabled"])
}

func TestGameWalletGetHandlerRestoresPendingSlotBonus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	expiresAt := time.Now().Add(time.Hour).UTC()
	repo := &gameLoyaltyHandlerRepoStub{pending: &service.SlotBonusPendingSnapshot{
		RoundID: 9, Kind: service.SlotBonusKindPickChest, SpinIdempotencyKey: "spin-pending",
		ExpiresAt: expiresAt, Choices: 3, Status: "pending",
	}}
	h := NewGameWalletHandler(service.NewGameLoyaltyService(repo, nil, nil, nil))
	r := gin.New()
	r.GET("/wallet", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
		h.GetWallet(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/wallet", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	pending := body["data"].(map[string]any)["pending_slot_bonus"].(map[string]any)
	require.Equal(t, "9", pending["round_id"])
	require.Equal(t, service.SlotBonusKindPickChest, pending["kind"])
	require.NotEmpty(t, pending["token"])
	require.Equal(t, "pending", pending["status"])
}

func TestGameWalletSpinV2JSONContract(t *testing.T) {
	payload := gameWalletSpinDTO(service.GameLoyaltySpinResult{
		ID: 2, UserID: 7, BetCredits: 10, BonusCount: 3,
		PaytableVersion:  service.SlotPaytableVersionV2,
		ReelStripVersion: service.SlotReelStripVersionV2,
		BonusRound: &service.SlotBonusRoundOffer{
			RoundID: 9, Kind: service.SlotBonusKindLuckyWheel, Choices: 3, Status: "pending",
		},
	})
	require.Equal(t, 3, payload.BonusCount)
	require.NotNil(t, payload.BonusRound)
	require.Equal(t, service.SlotBonusKindLuckyWheel, payload.BonusRound.Kind)

	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"bonus_count":3`)
	require.Contains(t, string(raw), `"kind":"lucky_wheel"`)
}

func TestGameWalletCheckInRequiresIdempotencyKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewGameWalletHandler(service.NewGameLoyaltyService(&gameLoyaltyHandlerRepoStub{}, nil, nil, nil))
	r := gin.New()
	r.POST("/check-in", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
		h.CheckIn(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/check-in", nil)
	r.ServeHTTP(w, req)
	require.NotEqual(t, http.StatusOK, w.Code)
}

func TestGameWalletGiftRequiresIdempotencyKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewGameWalletHandler(service.NewGameLoyaltyService(&gameLoyaltyHandlerRepoStub{}, nil, nil, nil))
	r := gin.New()
	r.POST("/gifts", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
		h.GiftCredits(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/gifts", strings.NewReader(`{"recipient_email":"friend@example.com","credits":"25"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.NotEqual(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "IDEMPOTENCY_KEY_REQUIRED")
}

func TestGameWalletGiftJSONContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &gameLoyaltyHandlerRepoStub{}
	settings := service.NewSettingService(&gameWalletHandlerSettingRepo{values: map[string]string{
		service.SettingKeyGameLoyaltyEnabled: "true",
	}}, nil)
	h := NewGameWalletHandler(service.NewGameLoyaltyService(repo, settings, nil, nil))
	r := gin.New()
	r.POST("/gifts", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
		h.GiftCredits(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/gifts", strings.NewReader(`{"recipient_email":" Friend@Example.COM ","credits":"25"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "gift-request-1")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]any)
	require.Equal(t, "4", data["id"])
	require.Equal(t, "friend@example.com", data["recipient_email"])
	require.Equal(t, "25", data["credits"])
	require.Equal(t, "95", data["sender_credits_after"])
	require.Equal(t, "35", data["recipient_credits_after"])
	require.Equal(t, "friend@example.com", repo.giftIn.RecipientEmail)
	require.Equal(t, int64(25), repo.giftIn.Credits)
	require.Equal(t, "gift-request-1", repo.giftIn.IdempotencyKey)
}

func TestGameWalletExchangeRemoved(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewGameWalletHandler(service.NewGameLoyaltyService(&gameLoyaltyHandlerRepoStub{}, nil, nil, nil))
	r := gin.New()
	r.POST("/exchange", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
		h.Exchange(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/exchange", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusGone, w.Code)
}
