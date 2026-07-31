package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUpdateSettingsRejectsEnabledGameWalletWithoutRate(t *testing.T) {
	h, _ := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyGameWalletEnabled:      "false",
		service.SettingKeyGameWalletExchangeRate: "0",
		service.SettingKeyGameWalletDailyLimit:   "0",
	})

	rec := doUpdateSettings(t, h, map[string]any{
		"game_wallet_enabled":       true,
		"game_wallet_exchange_rate": "0",
		"game_wallet_daily_limit":   "0",
	}, nil)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var body response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "GAME_WALLET_RATE_REQUIRED", body.Reason)
}

func TestUpdateSettingsAllowsDisabledGameWalletWithoutRate(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyGameWalletEnabled:      "false",
		service.SettingKeyGameWalletExchangeRate: "0",
		service.SettingKeyGameWalletDailyLimit:   "0",
	})

	rec := doUpdateSettings(t, h, map[string]any{
		"game_wallet_enabled":       false,
		"game_wallet_exchange_rate": "0",
		"game_wallet_daily_limit":   "0",
	}, nil)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "false", repo.lastUpdates[service.SettingKeyGameWalletEnabled])
	require.Equal(t, "0", repo.lastUpdates[service.SettingKeyGameWalletExchangeRate])
}
