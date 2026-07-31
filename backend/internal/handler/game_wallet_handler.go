package handler

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type GameWalletHandler struct {
	service *service.GameLoyaltyService
}

func NewGameWalletHandler(gameLoyaltyService *service.GameLoyaltyService) *GameWalletHandler {
	return &GameWalletHandler{service: gameLoyaltyService}
}

// GetWallet handles GET /api/v1/user/game-wallet.
func (h *GameWalletHandler) GetWallet(c *gin.Context) {
	userID, ok := authenticatedGameWalletUserID(c)
	if !ok {
		return
	}
	state, err := h.service.GetWallet(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	payload := dto.GameWalletState{
		UserID:             state.UserID,
		AccountBalance:     state.AccountBalance,
		Credits:            strconv.FormatInt(state.Credits, 10),
		FreeSpinsRemaining: state.FreeSpinsRemaining,
		LoyaltyEnabled:     state.LoyaltyEnabled,
		LoyaltyAvailable:   state.LoyaltyAvailable,
		CheckinCredits:     state.CheckinCredits,
		SlotBetCredits:     state.SlotBetCredits,
		SlotBetOptions:     int64Strings(state.SlotBetOptions),
		DailyRewardLimit:   state.DailyRewardLimit,
		CheckedInToday:     state.CheckedInToday,
		TodayCheckinAt:     state.TodayCheckinAt,
		LocalDate:          state.LocalDate,
		ExchangeEnabled:    false,
		ExchangeAvailable:  false,
		ExchangeRate:       "0",
		DailyLimit:         "0",
		DailyExchanged:     "0",
		WalletCreatedAt:    state.WalletCreatedAt,
		WalletUpdatedAt:    state.WalletUpdatedAt,
	}
	if state.PendingSlotBonus != nil {
		payload.PendingSlotBonus = gameWalletBonusRoundDTO(*state.PendingSlotBonus)
	}
	response.Success(c, payload)
}

// Exchange is removed in R21 (balance exchange replaced by loyalty check-in/rewards).
func (h *GameWalletHandler) Exchange(c *gin.Context) {
	response.ErrorFrom(c, infraerrors.New(
		410,
		"GAME_WALLET_EXCHANGE_REMOVED",
		"game wallet balance exchange has been replaced by the loyalty check-in and reward store",
	))
}

// CheckIn handles POST /api/v1/user/game-wallet/check-in.
func (h *GameWalletHandler) CheckIn(c *gin.Context) {
	userID, ok := authenticatedGameWalletUserID(c)
	if !ok {
		return
	}
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if strings.TrimSpace(idempotencyKey) == "" {
		response.ErrorFrom(c, service.ErrGameLoyaltyIdempotencyKeyRequired)
		return
	}
	result, err := h.service.CheckIn(c.Request.Context(), userID, idempotencyKey)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.GameWalletCheckinResult{
		ID:               strconv.FormatInt(result.ID, 10),
		UserID:           result.UserID,
		LocalDate:        result.LocalDate,
		CreditsAwarded:   strconv.FormatInt(result.CreditsAwarded, 10),
		CreditsBefore:    strconv.FormatInt(result.CreditsBefore, 10),
		CreditsAfter:     strconv.FormatInt(result.CreditsAfter, 10),
		IdempotencyKey:   result.IdempotencyKey,
		IdempotentReplay: result.IdempotentReplay,
		CreatedAt:        result.CreatedAt,
	})
}

// GiftCredits handles POST /api/v1/user/game-wallet/gifts.
func (h *GameWalletHandler) GiftCredits(c *gin.Context) {
	userID, ok := authenticatedGameWalletUserID(c)
	if !ok {
		return
	}
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if strings.TrimSpace(idempotencyKey) == "" {
		response.ErrorFrom(c, service.ErrGameLoyaltyIdempotencyKeyRequired)
		return
	}
	var req dto.GameCreditGiftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	rawCredits := ""
	if req.Credits != nil {
		rawCredits = string(*req.Credits)
	}
	result, err := h.service.GiftCredits(
		c.Request.Context(), userID, idempotencyKey, req.RecipientEmail, rawCredits,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.GameCreditGiftResult{
		ID:                     strconv.FormatInt(result.ID, 10),
		SenderUserID:           result.SenderUserID,
		RecipientUserID:        result.RecipientUserID,
		RecipientEmail:         result.RecipientEmail,
		Credits:                strconv.FormatInt(result.Credits, 10),
		SenderCreditsBefore:    strconv.FormatInt(result.SenderCreditsBefore, 10),
		SenderCreditsAfter:     strconv.FormatInt(result.SenderCreditsAfter, 10),
		RecipientCreditsBefore: strconv.FormatInt(result.RecipientCreditsBefore, 10),
		RecipientCreditsAfter:  strconv.FormatInt(result.RecipientCreditsAfter, 10),
		IdempotencyKey:         result.IdempotencyKey,
		IdempotentReplay:       result.IdempotentReplay,
		CreatedAt:              result.CreatedAt,
	})
}

// Spin handles POST /api/v1/user/game-wallet/slot/spin.
func (h *GameWalletHandler) Spin(c *gin.Context) {
	userID, ok := authenticatedGameWalletUserID(c)
	if !ok {
		return
	}
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if strings.TrimSpace(idempotencyKey) == "" {
		response.ErrorFrom(c, service.ErrGameLoyaltyIdempotencyKeyRequired)
		return
	}
	var req dto.GameWalletSpinRequest
	// Empty body is allowed (auto bet / free spin).
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Invalid request: "+err.Error())
			return
		}
	}
	var betPtr *int64
	if req.BetCredits != nil {
		raw := strings.TrimSpace(string(*req.BetCredits))
		if raw == "" {
			response.ErrorFrom(c, service.ErrGameLoyaltyInvalidBet)
			return
		}
		for _, r := range raw {
			if r < '0' || r > '9' {
				response.ErrorFrom(c, service.ErrGameLoyaltyInvalidBet)
				return
			}
		}
		bet, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || bet <= 0 {
			response.ErrorFrom(c, service.ErrGameLoyaltyInvalidBet)
			return
		}
		betPtr = &bet
	}
	result, err := h.service.Spin(c.Request.Context(), userID, idempotencyKey, betPtr)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gameWalletSpinDTO(*result))
}

// FruitSpin handles POST /api/v1/user/game-wallet/fruit/spin.
func (h *GameWalletHandler) FruitSpin(c *gin.Context) {
	userID, ok := authenticatedGameWalletUserID(c)
	if !ok {
		return
	}
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if strings.TrimSpace(idempotencyKey) == "" {
		response.ErrorFrom(c, service.ErrGameLoyaltyIdempotencyKeyRequired)
		return
	}
	var req dto.GameFruitSpinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	bets := make(service.FruitBetRequest, len(req.Bets))
	for door, value := range req.Bets {
		raw := strings.TrimSpace(string(value))
		if raw == "" || len(raw) > 19 {
			response.ErrorFrom(c, service.ErrGameFruitInvalidBets)
			return
		}
		for _, r := range raw {
			if r < '0' || r > '9' {
				response.ErrorFrom(c, service.ErrGameFruitInvalidBets)
				return
			}
		}
		bet, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			response.ErrorFrom(c, service.ErrGameFruitInvalidBets)
			return
		}
		bets[door] = bet
	}
	result, err := h.service.FruitSpin(c.Request.Context(), userID, idempotencyKey, bets)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gameFruitSpinDTO(*result))
}

// ClaimSlotBonus handles POST /api/v1/user/game-wallet/slot/bonus/claim.
func (h *GameWalletHandler) ClaimSlotBonus(c *gin.Context) {
	userID, ok := authenticatedGameWalletUserID(c)
	if !ok {
		return
	}
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if strings.TrimSpace(idempotencyKey) == "" {
		response.ErrorFrom(c, service.ErrGameLoyaltyIdempotencyKeyRequired)
		return
	}
	var req dto.GameWalletBonusClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.Choice == nil {
		response.ErrorFrom(c, service.ErrGameSlotBonusChoiceInvalid)
		return
	}
	result, err := h.service.ClaimSlotBonus(c.Request.Context(), userID, idempotencyKey, req.Token, *req.Choice)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	spinResults := make([]dto.GameWalletBonusSpinResult, 0, len(result.SpinResults))
	for _, spin := range result.SpinResults {
		if spin != nil {
			spinResults = append(spinResults, gameWalletBonusSpinDTO(*spin))
		}
	}
	response.Success(c, dto.GameWalletBonusClaimResult{
		RoundID:          strconv.FormatInt(result.RoundID, 10),
		Kind:             result.Kind,
		Choice:           result.Choice,
		RewardCredits:    strconv.FormatInt(result.RewardCredits, 10),
		CreditsAfter:     strconv.FormatInt(result.CreditsAfter, 10),
		ClaimedAt:        result.ClaimedAt,
		IdempotentReplay: result.IdempotentReplay,
		Spins:            result.Spins,
		Multiplier:       result.Multiplier,
		BonusMultiplier:  result.BonusMultiplier,
		TotalBasePayout:  strconv.FormatInt(result.BasePayout, 10),
		SpinResults:      spinResults,
	})
}

// GetLeaderboard handles GET /api/v1/user/game-wallet/leaderboard.
func (h *GameWalletHandler) GetLeaderboard(c *gin.Context) {
	userID, ok := authenticatedGameWalletUserID(c)
	if !ok {
		return
	}
	limit, err := parseOptionalPositiveInt(c.Query("limit"), 10)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("GAME_LEADERBOARD_LIMIT_INVALID", "limit must be a positive integer"))
		return
	}
	result, err := h.service.GetLeaderboard(c.Request.Context(), userID, c.Query("game_id"), limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	payload := dto.GameWalletLeaderboard{
		GameID: result.GameID, LocalDate: result.LocalDate,
		Items: make([]dto.GameWalletLeaderboardItem, 0, len(result.Items)),
	}
	for _, item := range result.Items {
		payload.Items = append(payload.Items, gameWalletLeaderboardDTO(item))
	}
	if result.MyEntry != nil {
		entry := gameWalletLeaderboardDTO(*result.MyEntry)
		payload.MyEntry = &entry
	}
	response.Success(c, payload)
}

// SubmitLeaderboardScore handles POST /api/v1/user/game-wallet/leaderboard/score.
func (h *GameWalletHandler) SubmitLeaderboardScore(c *gin.Context) {
	userID, ok := authenticatedGameWalletUserID(c)
	if !ok {
		return
	}
	var req dto.GameWalletScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	score, err := parseGameLeaderboardScore(req.Score)
	if err != nil {
		response.ErrorFrom(c, service.ErrGameLeaderboardScoreInvalid)
		return
	}
	if err := h.service.SubmitLeaderboardScore(c.Request.Context(), userID, req.GameID, score); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"game_id": strings.ToLower(strings.TrimSpace(req.GameID)), "score": strconv.FormatInt(score, 10)})
}

// ListRewards handles GET /api/v1/user/game-wallet/rewards.
func (h *GameWalletHandler) ListRewards(c *gin.Context) {
	userID, ok := authenticatedGameWalletUserID(c)
	if !ok {
		return
	}
	views, claimed, cfg, err := h.service.ListRewards(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]dto.GameWalletRewardItem, 0, len(views))
	for _, view := range views {
		items = append(items, dto.GameWalletRewardItem{
			ID:             view.ID,
			Title:          view.Title,
			CreditCost:     strconv.FormatInt(view.CreditCost, 10),
			VoucherValue:   view.VoucherValue,
			DailyStock:     view.DailyStock,
			ExpiryDays:     view.ExpiryDays,
			RemainingStock: view.RemainingStock,
			ClaimedToday:   view.ClaimedToday,
		})
	}
	payload := dto.GameWalletRewardList{
		Items:            items,
		DailyRewardLimit: 0,
		ClaimedToday:     claimed,
	}
	if cfg != nil {
		payload.DailyRewardLimit = cfg.DailyRewardLimit
		payload.LoyaltyEnabled = cfg.Enabled
		payload.LoyaltyAvailable = cfg.Available()
	}
	response.Success(c, payload)
}

// ClaimReward handles POST /api/v1/user/game-wallet/rewards/claim.
func (h *GameWalletHandler) ClaimReward(c *gin.Context) {
	userID, ok := authenticatedGameWalletUserID(c)
	if !ok {
		return
	}
	idempotencyKey := c.GetHeader("Idempotency-Key")
	if strings.TrimSpace(idempotencyKey) == "" {
		response.ErrorFrom(c, service.ErrGameLoyaltyIdempotencyKeyRequired)
		return
	}
	var req dto.GameWalletRewardClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.service.ClaimReward(c.Request.Context(), userID, idempotencyKey, req.RewardID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.GameWalletRewardClaimResult{
		ID:               strconv.FormatInt(result.ID, 10),
		UserID:           result.UserID,
		RewardID:         result.RewardID,
		Title:            result.Title,
		CreditCost:       strconv.FormatInt(result.CreditCost, 10),
		VoucherValue:     result.VoucherValue,
		RedeemCode:       result.RedeemCode,
		ClaimLocalDate:   result.ClaimLocalDate,
		CreditsBefore:    strconv.FormatInt(result.CreditsBefore, 10),
		CreditsAfter:     strconv.FormatInt(result.CreditsAfter, 10),
		ExpiresAt:        result.ExpiresAt,
		IdempotencyKey:   result.IdempotencyKey,
		IdempotentReplay: result.IdempotentReplay,
		CreatedAt:        result.CreatedAt,
	})
}

// ListTransactions handles GET /api/v1/user/game-wallet/transactions.
func (h *GameWalletHandler) ListTransactions(c *gin.Context) {
	userID, ok := authenticatedGameWalletUserID(c)
	if !ok {
		return
	}
	limit, err := parseOptionalPositiveInt(c.Query("limit"), 20)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("GAME_WALLET_LIMIT_INVALID", "limit must be a positive integer"))
		return
	}
	beforeID, err := parseOptionalPositiveInt64(c.Query("before_id"))
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("GAME_WALLET_CURSOR_INVALID", "before_id must be a positive integer"))
		return
	}
	items, err := h.service.ListTransactions(c.Request.Context(), userID, limit, beforeID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	payload := dto.GameWalletTransactionList{Items: make([]dto.GameWalletTransaction, 0, len(items))}
	for _, item := range items {
		payload.Items = append(payload.Items, gameWalletLedgerDTO(item))
	}
	if len(items) == limit && len(items) > 0 {
		next := strconv.FormatInt(items[len(items)-1].ID, 10)
		payload.NextBeforeID = &next
	}
	response.Success(c, payload)
}

func authenticatedGameWalletUserID(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	return subject.UserID, true
}

func gameWalletSpinDTO(item service.GameLoyaltySpinResult) dto.GameWalletSpinResult {
	grid := make([][]string, service.SlotRowCount)
	for row := 0; row < service.SlotRowCount; row++ {
		grid[row] = make([]string, service.SlotReelCount)
		for reel := 0; reel < service.SlotReelCount; reel++ {
			grid[row][reel] = item.Grid[row][reel]
		}
	}
	stops := make([]int, service.SlotReelCount)
	copy(stops, item.Stops[:])
	lines := make([]dto.GameWalletWinningLine, 0, len(item.WinningLines))
	for _, line := range item.WinningLines {
		lines = append(lines, dto.GameWalletWinningLine{
			LineIndex:  line.LineIndex,
			Symbol:     line.Symbol,
			Count:      line.Count,
			Multiplier: strconv.FormatInt(line.Mult, 10),
			Payout:     strconv.FormatInt(line.Payout, 10),
			Positions:  line.Positions,
		})
	}
	result := dto.GameWalletSpinResult{
		ID:                 strconv.FormatInt(item.ID, 10),
		UserID:             item.UserID,
		BetCredits:         strconv.FormatInt(item.BetCredits, 10),
		PayoutCredits:      strconv.FormatInt(item.PayoutCredits, 10),
		UsedFreeSpin:       item.UsedFreeSpin,
		Grid:               grid,
		Stops:              stops,
		WinningLines:       lines,
		ScatterCount:       item.ScatterCount,
		BonusCount:         item.BonusCount,
		FreeSpinsAwarded:   item.FreeSpinsAwarded,
		FreeSpinsRemaining: item.FreeSpinsAfter,
		CreditsBefore:      strconv.FormatInt(item.CreditsBefore, 10),
		CreditsAfter:       strconv.FormatInt(item.CreditsAfter, 10),
		PaytableVersion:    item.PaytableVersion,
		ReelStripVersion:   item.ReelStripVersion,
		IdempotencyKey:     item.IdempotencyKey,
		IdempotentReplay:   item.IdempotentReplay,
		CreatedAt:          item.CreatedAt,
	}
	if item.BonusRound != nil {
		result.BonusRound = gameWalletBonusRoundDTO(*item.BonusRound)
	}
	return result
}

func gameWalletBonusRoundDTO(item service.SlotBonusRoundOffer) *dto.GameWalletBonusRound {
	options := make([]dto.GameWalletBonusOption, 0, len(item.Options))
	for _, option := range item.Options {
		options = append(options, dto.GameWalletBonusOption{
			Choice: option.Choice, Spins: option.Spins, Multiplier: option.Multiplier,
		})
	}
	return &dto.GameWalletBonusRound{
		RoundID:         strconv.FormatInt(item.RoundID, 10),
		Kind:            item.Kind,
		Token:           item.Token,
		ExpiresAt:       item.ExpiresAt,
		Choices:         item.Choices,
		Status:          item.Status,
		BetCredits:      strconv.FormatInt(item.BetCredits, 10),
		BonusCount:      item.BonusCount,
		BonusMultiplier: item.BonusMultiplier,
		Options:         options,
	}
}

func gameWalletBonusSpinDTO(item service.SlotSpinResult) dto.GameWalletBonusSpinResult {
	grid := make([][]string, service.SlotRowCount)
	for row := 0; row < service.SlotRowCount; row++ {
		grid[row] = make([]string, service.SlotReelCount)
		copy(grid[row], item.Grid[row][:])
	}
	stops := make([]int, service.SlotReelCount)
	copy(stops, item.Stops[:])
	lines := make([]dto.GameWalletWinningLine, 0, len(item.WinningLines))
	for _, line := range item.WinningLines {
		lines = append(lines, dto.GameWalletWinningLine{
			LineIndex: line.LineIndex, Symbol: line.Symbol, Count: line.Count,
			Multiplier: strconv.FormatInt(line.Mult, 10), Payout: strconv.FormatInt(line.Payout, 10),
			Positions: line.Positions,
		})
	}
	return dto.GameWalletBonusSpinResult{
		BetCredits:    strconv.FormatInt(item.BetCredits, 10),
		PayoutCredits: strconv.FormatInt(item.TotalPayout, 10),
		Grid:          grid, Stops: stops, WinningLines: lines,
		ScatterCount: item.ScatterCount, BonusCount: item.BonusCount,
		FreeSpinsAwarded: item.FreeSpinsAwarded,
		PaytableVersion:  item.PaytableVersion, ReelStripVersion: item.ReelStripVersion,
	}
}

func int64Strings(values []int64) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, strconv.FormatInt(value, 10))
	}
	return result
}

func gameWalletLeaderboardDTO(item service.GameLeaderboardEntry) dto.GameWalletLeaderboardItem {
	return dto.GameWalletLeaderboardItem{
		Rank: item.Rank, UserDisplay: item.UserDisplay,
		Score: strconv.FormatInt(item.Score, 10), AchievedAt: item.AchievedAt,
		IsCurrentUser: item.CurrentUser,
	}
}

func gameFruitSpinDTO(item service.FruitSpinResult) dto.GameFruitSpinResult {
	bets := make(map[string]string, service.FruitDoorCount)
	for index, door := range service.FruitDoors {
		bets[door] = strconv.FormatInt(item.Bets[index], 10)
	}
	convertStops := func(items []service.FruitBoardCell) []dto.GameFruitRewardStop {
		out := make([]dto.GameFruitRewardStop, 0, len(items))
		for _, stop := range items {
			out = append(out, dto.GameFruitRewardStop{
				Index: stop.Index, Symbol: stop.Symbol, Size: stop.Size,
				Multiplier: strconv.FormatInt(stop.Multiplier, 10),
			})
		}
		return out
	}
	return dto.GameFruitSpinResult{
		ID: strconv.FormatInt(item.ID, 10), UserID: item.UserID, Bets: bets,
		TotalBet: strconv.FormatInt(item.TotalBet, 10), StopIndex: item.StopIndex,
		Outcome: dto.GameFruitOutcome{
			Kind: item.Outcome.Kind, CenterIndex: item.Outcome.CenterIndex,
			Symbol: item.Outcome.Symbol, Size: item.Outcome.Size,
			Multiplier:        strconv.FormatInt(item.Outcome.Multiplier, 10),
			ExtraStops:        convertStops(item.Outcome.ExtraStops),
			LittleMaryStops:   convertStops(item.Outcome.LittleMaryStops),
			JackpotTier:       item.Outcome.JackpotTier,
			JackpotMultiplier: strconv.FormatInt(item.Outcome.JackpotMultiplier, 10),
		},
		Payout:          strconv.FormatInt(item.Payout, 10),
		CreditsBefore:   strconv.FormatInt(item.CreditsBefore, 10),
		CreditsAfter:    strconv.FormatInt(item.CreditsAfter, 10),
		PaytableVersion: item.PaytableVersion, IdempotencyKey: item.IdempotencyKey,
		IdempotentReplay: item.IdempotentReplay, CreatedAt: item.CreatedAt,
	}
}

func parseGameLeaderboardScore(value *dto.GameWalletNumber) (int64, error) {
	if value == nil {
		return 0, strconv.ErrSyntax
	}
	raw := strings.TrimSpace(string(*value))
	if raw == "" || len(raw) > 10 {
		return 0, strconv.ErrSyntax
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, strconv.ErrSyntax
		}
	}
	score, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || score > 1_000_000_000 {
		return 0, strconv.ErrSyntax
	}
	return score, nil
}

func gameWalletLedgerDTO(item service.GameLoyaltyLedgerEntry) dto.GameWalletTransaction {
	out := dto.GameWalletTransaction{
		ID:             strconv.FormatInt(item.ID, 10),
		UserID:         item.UserID,
		EntryType:      item.EntryType,
		Amount:         strconv.FormatInt(item.Amount, 10),
		CreditsBefore:  strconv.FormatInt(item.CreditsBefore, 10),
		CreditsAfter:   strconv.FormatInt(item.CreditsAfter, 10),
		ReferenceType:  item.ReferenceType,
		IdempotencyKey: item.IdempotencyKey,
		Metadata:       item.Metadata,
		CreatedAt:      item.CreatedAt,
	}
	if item.ReferenceID != nil {
		ref := strconv.FormatInt(*item.ReferenceID, 10)
		out.ReferenceID = &ref
	}
	return out
}

func parseOptionalPositiveInt(raw string, defaultValue int) (int, error) {
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, strconv.ErrSyntax
	}
	return value, nil
}

func parseOptionalPositiveInt64(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, strconv.ErrSyntax
	}
	return value, nil
}
