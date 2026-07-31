package service

import (
	"context"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type loyaltyRepoStub struct {
	spinIn         GameLoyaltySpinInput
	spinBet        int64
	spinOut        *SlotSpinResult
	claimIn        GameLoyaltyRewardClaimInput
	claimCode      string
	claimItem      GameLoyaltyRewardItem
	checkKey       string
	submittedGame  string
	submittedScore int64
	leaderboard    []GameLeaderboardEntry
	bonusClaim     *SlotBonusClaimResult
	pendingBonus   *SlotBonusPendingSnapshot
	persistSpin    bool
	persistedSpin  *GameLoyaltySpinResult
	bonusKind      string
	bonusRewards   [slotBonusChoiceCount]int64
	bonusPlan      *SlotBonusPlan
}

type loyaltyGiftRepoStub struct {
	*loyaltyRepoStub
	giftSenderID int64
	giftInput    GameCreditGiftInput
	giftCalls    int
}

func (s *loyaltyGiftRepoStub) GiftCredits(_ context.Context, senderUserID int64, input GameCreditGiftInput) (*GameCreditGiftResult, error) {
	s.giftSenderID = senderUserID
	s.giftInput = input
	s.giftCalls++
	return &GameCreditGiftResult{
		ID: 12, SenderUserID: senderUserID, RecipientUserID: 8,
		RecipientEmail: input.RecipientEmail, Credits: input.Credits,
		SenderCreditsBefore: 100, SenderCreditsAfter: 75,
		RecipientCreditsBefore: 5, RecipientCreditsAfter: 30,
		IdempotencyKey: input.IdempotencyKey,
	}, nil
}

func (s *loyaltyRepoStub) GetSnapshot(context.Context, int64, string) (*GameLoyaltyWalletSnapshot, error) {
	return &GameLoyaltyWalletSnapshot{
		UserID:         7,
		AccountBalance: "1.00",
		Credits:        100,
		LocalDate:      "2026-07-28",
	}, nil
}

func (s *loyaltyRepoStub) CheckIn(_ context.Context, userID int64, localDate string, credits int64, key, _ string) (*GameLoyaltyCheckinResult, error) {
	s.checkKey = key
	return &GameLoyaltyCheckinResult{
		ID:             1,
		UserID:         userID,
		LocalDate:      localDate,
		CreditsAwarded: credits,
		CreditsAfter:   credits,
		IdempotencyKey: key,
	}, nil
}

func (s *loyaltyRepoStub) Spin(_ context.Context, userID int64, in GameLoyaltySpinInput, bet int64, out *SlotSpinResult) (*GameLoyaltySpinResult, error) {
	s.spinIn, s.spinBet, s.spinOut = in, bet, out
	if s.persistSpin && s.persistedSpin != nil {
		result := *s.persistedSpin
		if result.BonusRound != nil {
			bonus := *result.BonusRound
			bonus.Token = ""
			result.BonusRound = &bonus
		}
		result.IdempotentReplay = true
		return &result, nil
	}
	resultBet := bet
	if in.BetCredits > 0 {
		resultBet = in.BetCredits
	}
	result := &GameLoyaltySpinResult{
		ID:               2,
		UserID:           userID,
		BetCredits:       resultBet,
		PayoutCredits:    out.TotalPayout,
		Grid:             out.Grid,
		Stops:            out.Stops,
		WinningLines:     out.WinningLines,
		ScatterCount:     out.ScatterCount,
		BonusCount:       out.BonusCount,
		FreeSpinsAwarded: out.FreeSpinsAwarded,
		PaytableVersion:  out.PaytableVersion,
		ReelStripVersion: out.ReelStripVersion,
		IdempotencyKey:   in.IdempotencyKey,
	}
	if in.BonusRound != nil {
		s.bonusKind = in.BonusRound.Kind
		s.bonusRewards = in.BonusRound.Rewards
		s.bonusPlan = in.BonusRound.Plan
		result.BonusRound = &SlotBonusRoundOffer{
			RoundID: 9, Kind: in.BonusRound.Kind, Token: in.BonusRound.Token, ExpiresAt: in.BonusRound.ExpiresAt,
			Choices: 3, Status: "pending",
		}
		if in.BonusRound.Plan != nil {
			result.BonusRound.BetCredits = in.BonusRound.Plan.BetCredits
			result.BonusRound.BonusCount = in.BonusRound.Plan.BonusCount
			result.BonusRound.BonusMultiplier = in.BonusRound.Plan.BonusMultiplier
			for _, option := range in.BonusRound.Plan.Options {
				result.BonusRound.Options = append(result.BonusRound.Options, SlotBonusOptionView{
					Choice: option.Choice, Spins: option.Spins, Multiplier: option.Multiplier,
				})
			}
		}
	}
	if s.persistSpin {
		persisted := *result
		if result.BonusRound != nil {
			bonus := *result.BonusRound
			persisted.BonusRound = &bonus
		}
		s.persistedSpin = &persisted
	}
	return result, nil
}

func (s *loyaltyRepoStub) ListRewardsStatus(context.Context, int64, string, []GameLoyaltyRewardItem, int) ([]GameLoyaltyRewardView, int, error) {
	return []GameLoyaltyRewardView{{
		ID: "api-1", Title: "1 USD", CreditCost: 500, VoucherValue: "1",
	}}, 0, nil
}

func (s *loyaltyRepoStub) ClaimReward(_ context.Context, userID int64, localDate string, item GameLoyaltyRewardItem, _ int, code string, _ *time.Time, in GameLoyaltyRewardClaimInput) (*GameLoyaltyRewardClaimResult, error) {
	s.claimIn, s.claimCode, s.claimItem = in, code, item
	return &GameLoyaltyRewardClaimResult{
		ID:             5,
		UserID:         userID,
		RewardID:       item.ID,
		Title:          item.Title,
		CreditCost:     item.CreditCost,
		VoucherValue:   item.VoucherValue,
		RedeemCode:     code,
		ClaimLocalDate: localDate,
		IdempotencyKey: in.IdempotencyKey,
	}, nil
}

func (s *loyaltyRepoStub) ListLedger(context.Context, int64, int, int64) ([]GameLoyaltyLedgerEntry, error) {
	return nil, nil
}

func (s *loyaltyRepoStub) SubmitDailyScore(_ context.Context, _ int64, _, gameID string, score int64) error {
	s.submittedGame, s.submittedScore = gameID, score
	return nil
}

func (s *loyaltyRepoStub) GetAllTimeLeaderboard(context.Context, int64, string, int) ([]GameLeaderboardEntry, error) {
	return s.leaderboard, nil
}

func (s *loyaltyRepoStub) ClaimSlotBonus(context.Context, int64, SlotBonusClaimInput) (*SlotBonusClaimResult, error) {
	return s.bonusClaim, nil
}

func (s *loyaltyRepoStub) GetPendingSlotBonus(context.Context, int64, time.Time) (*SlotBonusPendingSnapshot, error) {
	return s.pendingBonus, nil
}

func TestGameLoyaltyWalletRestoresOnlyCurrentPendingSlotBonus(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	repo := &loyaltyRepoStub{pendingBonus: &SlotBonusPendingSnapshot{
		RoundID: 9, Kind: SlotBonusKindLuckyWheel, SpinIdempotencyKey: "spin-pending",
		ExpiresAt: expiresAt, Choices: 3, Status: "pending",
	}}
	state, err := NewGameLoyaltyService(repo, nil, nil, nil).GetWallet(context.Background(), 7)
	require.NoError(t, err)
	require.NotNil(t, state.PendingSlotBonus)
	require.Equal(t, int64(9), state.PendingSlotBonus.RoundID)
	require.Equal(t, SlotBonusKindLuckyWheel, state.PendingSlotBonus.Kind)
	require.Equal(t, deriveSlotBonusToken(7, "spin-pending"), state.PendingSlotBonus.Token)
	require.Equal(t, "pending", state.PendingSlotBonus.Status)

	for _, pending := range []*SlotBonusPendingSnapshot{
		{RoundID: 10, Kind: SlotBonusKindPickChest, SpinIdempotencyKey: "claimed", ExpiresAt: expiresAt, Choices: 3, Status: "claimed"},
		{RoundID: 11, Kind: SlotBonusKindPickChest, SpinIdempotencyKey: "expired", ExpiresAt: time.Now().Add(-time.Minute), Choices: 3, Status: "pending"},
	} {
		repo.pendingBonus = pending
		state, err = NewGameLoyaltyService(repo, nil, nil, nil).GetWallet(context.Background(), 7)
		require.NoError(t, err)
		require.Nil(t, state.PendingSlotBonus)
	}
}

func TestGameCreditGiftNormalizesAndDelegates(t *testing.T) {
	repo := &loyaltyGiftRepoStub{loyaltyRepoStub: &loyaltyRepoStub{}}
	settings := &SettingService{settingRepo: &gameWalletSettingRepoStub{values: map[string]string{
		SettingKeyGameLoyaltyEnabled: "true",
	}}}
	result, err := NewGameLoyaltyService(repo, settings, nil, nil).GiftCredits(
		context.Background(), 7, "gift-request-1", " Friend@Example.COM ", "25",
	)
	require.NoError(t, err)
	require.Equal(t, int64(7), repo.giftSenderID)
	require.Equal(t, "friend@example.com", repo.giftInput.RecipientEmail)
	require.Equal(t, int64(25), repo.giftInput.Credits)
	require.Equal(t, "gift-request-1", repo.giftInput.IdempotencyKey)
	require.Len(t, repo.giftInput.RequestHash, 64)
	require.Equal(t, "75", strconv.FormatInt(result.SenderCreditsAfter, 10))
}

func TestGameCreditGiftRejectsInvalidPayloadBeforeRepository(t *testing.T) {
	repo := &loyaltyGiftRepoStub{loyaltyRepoStub: &loyaltyRepoStub{}}
	settings := &SettingService{settingRepo: &gameWalletSettingRepoStub{values: map[string]string{
		SettingKeyGameLoyaltyEnabled: "true",
	}}}
	service := NewGameLoyaltyService(repo, settings, nil, nil)

	_, err := service.GiftCredits(context.Background(), 7, "gift-request-2", "not-an-email", "25")
	require.ErrorIs(t, err, ErrGameCreditGiftRecipientInvalid)
	_, err = service.GiftCredits(context.Background(), 7, "gift-request-3", "friend@example.com", "0")
	require.ErrorIs(t, err, ErrGameCreditGiftAmountInvalid)
	require.Zero(t, repo.giftCalls)
}

func TestNormalizeGameLoyaltySettings(t *testing.T) {
	c, b, lim, cat, err := NormalizeGameLoyaltySettingValues(
		"0100",
		"5",
		"3",
		`[{"id":"api-1","title":"1 USD","credit_cost":"500","voucher_value":"1.00","daily_stock":2,"enabled":true}]`,
	)
	require.NoError(t, err)
	require.Equal(t, "100", c)
	require.Equal(t, "5", b)
	require.Equal(t, "3", lim)
	require.Contains(t, cat, `"id":"api-1"`)

	_, b, _, _, err = NormalizeGameLoyaltySettingValues("100", "10000", "3", "[]")
	require.NoError(t, err)
	require.Equal(t, "10000", b)
	_, _, _, _, err = NormalizeGameLoyaltySettingValues("100", "10001", "3", "[]")
	require.Error(t, err)

	_, _, _, _, err = ValidateGameLoyaltySettingValues(true, "0", "5", "0", "[]")
	require.Error(t, err)

	c, b, lim, cat, err = ValidateGameLoyaltySettingValues(false, "0", "0", "0", "[]")
	require.NoError(t, err)
	require.Equal(t, "0", c)
	require.Equal(t, "0", b)
	require.Equal(t, "0", lim)
	require.Equal(t, "[]", cat)
}

func TestParseGameLoyaltyCatalogExactVoucher(t *testing.T) {
	items, err := ParseGameLoyaltyRewardCatalog(
		`[{"id":"x","title":"t","credit_cost":"10","voucher_value":"1.5","daily_stock":1}]`,
	)
	require.NoError(t, err)
	require.Equal(t, "1.5", items[0].VoucherValue)
	require.Equal(t, int64(10), items[0].CreditCost)
}

func TestParseGameLoyaltyCatalogRejectsDuplicateIDs(t *testing.T) {
	_, err := ParseGameLoyaltyRewardCatalog(`[
		{"id":"a","title":"A","credit_cost":"1","voucher_value":"0.1","daily_stock":1},
		{"id":"a","title":"B","credit_cost":"2","voucher_value":"0.2","daily_stock":1}
	]`)
	require.Error(t, err)
}

func TestGetGameLoyaltyConfigFailsClosed(t *testing.T) {
	settings := &SettingService{settingRepo: &gameWalletSettingRepoStub{values: map[string]string{
		SettingKeyGameLoyaltyEnabled:          "true",
		SettingKeyGameLoyaltyCheckinCredits:   "not-int",
		SettingKeyGameLoyaltySlotBetCredits:   "10",
		SettingKeyGameLoyaltyDailyRewardLimit: "1",
		SettingKeyGameLoyaltyRewardCatalog:    "[]",
	}}}
	cfg := settings.GetGameLoyaltyConfig(context.Background())
	require.True(t, cfg.Enabled)
	require.False(t, cfg.CheckinConfigured)
	require.False(t, cfg.Available())
}

func TestGameLoyaltyCheckInRequiresIdempotencyAndConfig(t *testing.T) {
	repo := &loyaltyRepoStub{}
	settings := &SettingService{settingRepo: &gameWalletSettingRepoStub{values: map[string]string{
		SettingKeyGameLoyaltyEnabled:          "true",
		SettingKeyGameLoyaltyCheckinCredits:   "50",
		SettingKeyGameLoyaltySlotBetCredits:   "10",
		SettingKeyGameLoyaltyDailyRewardLimit: "5",
		SettingKeyGameLoyaltyRewardCatalog:    "[]",
	}}}
	svc := NewGameLoyaltyService(repo, settings, nil, nil)

	_, err := svc.CheckIn(context.Background(), 7, "")
	require.ErrorIs(t, err, ErrGameLoyaltyIdempotencyKeyRequired)

	result, err := svc.CheckIn(context.Background(), 7, "check-1")
	require.NoError(t, err)
	require.Equal(t, int64(50), result.CreditsAwarded)
	require.Equal(t, "check-1", repo.checkKey)
}

func TestGameLoyaltyIdempotencyKeyKeepsExternal128ByteContract(t *testing.T) {
	key := strings.Repeat("k", 128)
	normalized, err := normalizeGameWalletIdempotencyKey(key)
	require.NoError(t, err)
	require.Equal(t, key, normalized)

	_, err = normalizeGameWalletIdempotencyKey(key + "k")
	require.ErrorIs(t, err, ErrGameLoyaltyIdempotencyKeyInvalid)
}

func TestGameLoyaltySpinUsesServerRNGAndNoSeed(t *testing.T) {
	repo := &loyaltyRepoStub{}
	settings := &SettingService{settingRepo: &gameWalletSettingRepoStub{values: map[string]string{
		SettingKeyGameLoyaltyEnabled:          "true",
		SettingKeyGameLoyaltyCheckinCredits:   "50",
		SettingKeyGameLoyaltySlotBetCredits:   "10",
		SettingKeyGameLoyaltyDailyRewardLimit: "5",
		SettingKeyGameLoyaltyRewardCatalog:    "[]",
	}}}
	svc := NewGameLoyaltyService(repo, settings, nil, nil).
		WithSlotEngine(NewSlotEngine(newFixedRandReader(0, 1, 2, 3, 4)))
	result, err := svc.Spin(context.Background(), 7, "spin-1", nil)
	require.NoError(t, err)
	require.Equal(t, int64(10), repo.spinBet)
	require.NotNil(t, repo.spinOut)
	require.Equal(t, SlotPaytableVersionV10, result.PaytableVersion)
	require.Equal(t, "spin-1", repo.spinIn.IdempotencyKey)
}

func TestGameLoyaltySpinScatterDoesNotCreateBonusOffer(t *testing.T) {
	repo := &loyaltyRepoStub{}
	settings := &SettingService{settingRepo: &gameWalletSettingRepoStub{values: map[string]string{
		SettingKeyGameLoyaltyEnabled:          "true",
		SettingKeyGameLoyaltyCheckinCredits:   "50",
		SettingKeyGameLoyaltySlotBetCredits:   "10",
		SettingKeyGameLoyaltyDailyRewardLimit: "0",
		SettingKeyGameLoyaltyRewardCatalog:    "[]",
	}}}
	svc := NewGameLoyaltyService(repo, settings, nil, nil).
		WithSlotEngine(NewSlotEngine(newFixedRandReader(
			1000,
			firstSlotAllowedPlainStopIndex(t, 0),
			firstSlotAllowedPlainStopIndex(t, 1),
			firstSlotStopShowingSymbol(t, 2, SlotSymbolScatter),
			firstSlotStopShowingSymbol(t, 3, SlotSymbolScatter),
			firstSlotStopShowingSymbol(t, 4, SlotSymbolScatter),
		))).
		WithCodeRand(newFixedRandReader(1, 2, 3, 4, 5, 6, 7, 8))

	result, err := svc.Spin(context.Background(), 7, "scatter-spin", nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, repo.spinOut.ScatterCount, SlotScatterFreeSpinsMin)
	require.Zero(t, repo.spinOut.BonusCount)
	require.Nil(t, repo.spinIn.BonusRound)
	require.Nil(t, result.BonusRound)
}

func firstSlotAllowedPlainStopIndex(t *testing.T, reel int) uint32 {
	t.Helper()
	for index, stop := range slotAllowedStopsForReel(reel) {
		if slotStopIsPlain(reel, stop) {
			return uint32(index)
		}
	}
	t.Fatalf("reel %d has no allowed plain stop", reel)
	return 0
}

func TestGameLoyaltySpinBonusCreatesServerOwnedFreeSpinOptions(t *testing.T) {
	settings := &SettingService{settingRepo: &gameWalletSettingRepoStub{values: map[string]string{
		SettingKeyGameLoyaltyEnabled:          "true",
		SettingKeyGameLoyaltyCheckinCredits:   "50",
		SettingKeyGameLoyaltySlotBetCredits:   "10",
		SettingKeyGameLoyaltyDailyRewardLimit: "0",
		SettingKeyGameLoyaltyRewardCatalog:    "[]",
	}}}
	repo := &loyaltyRepoStub{}
	svc := NewGameLoyaltyService(repo, settings, nil, nil).
		WithSlotEngine(NewSlotEngine(newFixedRandReader(0)))
	result, err := svc.Spin(context.Background(), 7, "bonus-free-spins", nil)
	require.NoError(t, err)
	require.Equal(t, SlotBonusTriggerMin, repo.spinOut.BonusCount)
	require.NotNil(t, result.BonusRound)
	require.Equal(t, SlotBonusKindFreeSpins, result.BonusRound.Kind)
	require.Len(t, result.BonusRound.Token, 43)
	require.NotNil(t, repo.bonusPlan)
	require.Equal(t, SlotBonusProtocolV4, repo.bonusPlan.Protocol)
	require.Equal(t, int64(10), repo.bonusPlan.BetCredits)
	require.Equal(t, 3, repo.bonusPlan.BonusCount)
	require.Equal(t, 1, repo.bonusPlan.BonusMultiplier)
	require.Len(t, repo.bonusPlan.Options, 3)
	require.Equal(t, []int{8, 12, 20}, []int{
		repo.bonusPlan.Options[0].Spins, repo.bonusPlan.Options[1].Spins, repo.bonusPlan.Options[2].Spins,
	})
	for _, option := range repo.bonusPlan.Options {
		require.Len(t, option.SpinResults, option.Spins)
		require.Positive(t, option.RewardCredits)
		minimumWins, minimumErr := SlotBonusMinimumWinningSpins(option.Spins)
		require.NoError(t, minimumErr)
		winningSpins := 0
		for _, spin := range option.SpinResults {
			require.Less(t, spin.BonusCount, SlotBonusTriggerMin)
			if spin.TotalPayout > 0 {
				winningSpins++
			}
		}
		require.GreaterOrEqual(t, winningSpins, minimumWins)
		floor, floorErr := SlotBonusMinimumRewardCredits(repo.bonusPlan.BetCredits, repo.bonusPlan.BonusMultiplier)
		require.NoError(t, floorErr)
		require.GreaterOrEqual(t, option.RewardCredits, floor)
	}
}

func TestSlotBonusV4RewardHasAWholeFeatureFloor(t *testing.T) {
	reward, err := SlotBonusFeatureRewardCredits(0, 10, 5, 4)
	require.NoError(t, err)
	require.Equal(t, int64(200), reward)
}

func TestGameLoyaltySpinReplayRestoresPendingBonusToken(t *testing.T) {
	repo := &loyaltyRepoStub{persistSpin: true}
	settings := &SettingService{settingRepo: &gameWalletSettingRepoStub{values: map[string]string{
		SettingKeyGameLoyaltyEnabled:          "true",
		SettingKeyGameLoyaltyCheckinCredits:   "50",
		SettingKeyGameLoyaltySlotBetCredits:   "10",
		SettingKeyGameLoyaltyDailyRewardLimit: "0",
		SettingKeyGameLoyaltyRewardCatalog:    "[]",
	}}}
	svc := NewGameLoyaltyService(repo, settings, nil, nil).
		WithSlotEngine(NewSlotEngine(newFixedRandReader(0)))

	first, err := svc.Spin(context.Background(), 7, " replay-bonus ", nil)
	require.NoError(t, err)
	require.NotNil(t, first.BonusRound)
	tokenBytes, err := base64.RawURLEncoding.DecodeString(first.BonusRound.Token)
	require.NoError(t, err)
	require.Len(t, tokenBytes, 32)
	firstPlan := repo.bonusPlan

	replay, err := svc.Spin(context.Background(), 7, "replay-bonus", nil)
	require.NoError(t, err)
	require.True(t, replay.IdempotentReplay)
	require.NotNil(t, replay.BonusRound)
	require.Equal(t, "pending", replay.BonusRound.Status)
	require.Equal(t, SlotBonusKindFreeSpins, replay.BonusRound.Kind)
	require.Equal(t, first.BonusRound.Token, replay.BonusRound.Token)
	require.Same(t, firstPlan, repo.bonusPlan)
}

func TestGameLeaderboardRejectsLuckyClientScore(t *testing.T) {
	repo := &loyaltyRepoStub{}
	svc := NewGameLoyaltyService(repo, nil, nil, nil)
	err := svc.SubmitLeaderboardScore(context.Background(), 7, GameIDLucky, 100)
	require.ErrorIs(t, err, ErrGameLeaderboardLuckyClientScore)
	require.Empty(t, repo.submittedGame)
}

func TestGameLeaderboardOrderingAndEmailMasking(t *testing.T) {
	now := time.Now()
	repo := &loyaltyRepoStub{leaderboard: []GameLeaderboardEntry{
		{Rank: 1, UserID: 8, Username: "winner", Email: "winner@example.com", Score: 900, AchievedAt: now},
		{Rank: 2, UserID: 7, Email: "alice@example.com", Score: 800, AchievedAt: now.Add(time.Second)},
		{Rank: 3, UserID: 9, Username: "visible@example.net", Email: "private@example.org", Score: 700, AchievedAt: now.Add(2 * time.Second)},
		{Rank: 4, UserID: 10, Username: "same@example.com", Email: "same@example.com", Score: 600, AchievedAt: now.Add(3 * time.Second)},
	}}
	svc := NewGameLoyaltyService(repo, nil, nil, nil)
	result, err := svc.GetLeaderboard(context.Background(), 7, GameIDSnake, 10)
	require.NoError(t, err)
	require.Equal(t, "winner", result.Items[0].UserDisplay)
	require.Equal(t, "a***@example.com", result.Items[1].UserDisplay)
	require.NotContains(t, result.Items[1].UserDisplay, "alice@")
	require.Equal(t, "v***@example.net", result.Items[2].UserDisplay)
	require.NotContains(t, result.Items[2].UserDisplay, "visible@example.net")
	require.Equal(t, "s***@example.com", result.Items[3].UserDisplay)
	require.NotContains(t, result.Items[3].UserDisplay, "same@example.com")
	require.True(t, result.Items[1].CurrentUser)
	require.Equal(t, int64(2), result.MyEntry.Rank)
	require.Empty(t, result.LocalDate)
}

func TestGameLoyaltySpinRejectsCustomBet(t *testing.T) {
	repo := &loyaltyRepoStub{}
	settings := &SettingService{settingRepo: &gameWalletSettingRepoStub{values: map[string]string{
		SettingKeyGameLoyaltyEnabled:          "true",
		SettingKeyGameLoyaltyCheckinCredits:   "50",
		SettingKeyGameLoyaltySlotBetCredits:   "10",
		SettingKeyGameLoyaltyDailyRewardLimit: "0",
		SettingKeyGameLoyaltyRewardCatalog:    "[]",
	}}}
	svc := NewGameLoyaltyService(repo, settings, nil, nil)
	bad := int64(99)
	_, err := svc.Spin(context.Background(), 7, "spin-2", &bad)
	require.ErrorIs(t, err, ErrGameLoyaltyInvalidBet)
}

func TestGameLoyaltySpinAcceptsMaximumVariableBet(t *testing.T) {
	repo := &loyaltyRepoStub{}
	settings := &SettingService{settingRepo: &gameWalletSettingRepoStub{values: map[string]string{
		SettingKeyGameLoyaltyEnabled:          "true",
		SettingKeyGameLoyaltyCheckinCredits:   "50",
		SettingKeyGameLoyaltySlotBetCredits:   "10",
		SettingKeyGameLoyaltyDailyRewardLimit: "0",
		SettingKeyGameLoyaltyRewardCatalog:    "[]",
	}}}
	svc := NewGameLoyaltyService(repo, settings, nil, nil).
		WithSlotEngine(NewSlotEngine(newFixedRandReader(1000, 0, 0, 0, 0, 0)))
	bet := SlotBetMaxCredits
	result, err := svc.Spin(context.Background(), 7, "spin-variable-10000", &bet)
	require.NoError(t, err)
	require.Equal(t, SlotBetMaxCredits, repo.spinIn.BetCredits)
	require.Equal(t, SlotBetMaxCredits, repo.spinOut.BetCredits)
	require.Equal(t, SlotBetMaxCredits, result.BetCredits)
}

func TestGameLoyaltyClaimGeneratesCode(t *testing.T) {
	repo := &loyaltyRepoStub{}
	catalog := `[{"id":"api-1","title":"One","credit_cost":"500","voucher_value":"1.25","daily_stock":3,"enabled":true}]`
	settings := &SettingService{settingRepo: &gameWalletSettingRepoStub{values: map[string]string{
		SettingKeyGameLoyaltyEnabled:          "true",
		SettingKeyGameLoyaltyCheckinCredits:   "50",
		SettingKeyGameLoyaltySlotBetCredits:   "10",
		SettingKeyGameLoyaltyDailyRewardLimit: "5",
		SettingKeyGameLoyaltyRewardCatalog:    catalog,
	}}}
	svc := NewGameLoyaltyService(repo, settings, nil, nil).
		WithCodeRand(newFixedRandReader(0x11223344, 0x55667788))
	result, err := svc.ClaimReward(context.Background(), 7, "claim-1", "api-1")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(result.RedeemCode, "GT"))
	require.Len(t, result.RedeemCode, 30)
	require.Equal(t, "1.25", result.VoucherValue)
	require.Equal(t, "api-1", repo.claimItem.ID)
	require.Equal(t, int64(500), repo.claimItem.CreditCost)
}

func TestGameLoyaltyDisabledFailsClosed(t *testing.T) {
	repo := &loyaltyRepoStub{}
	svc := NewGameLoyaltyService(repo, nil, nil, nil)
	_, err := svc.CheckIn(context.Background(), 1, "k")
	require.ErrorIs(t, err, ErrGameLoyaltyDisabled)
}
