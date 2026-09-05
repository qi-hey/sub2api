package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyOpenAICreateDefaultsAddsAstraWithoutMutatingInput(t *testing.T) {
	for _, accountType := range []string{AccountTypeOAuth, AccountTypeSetupToken} {
		t.Run(accountType, func(t *testing.T) {
			credentials := map[string]any{
				"access_token": "token",
				"model_mapping": map[string]any{
					"gpt-5.6-sol": "gpt-5.6-sol",
				},
			}
			before, err := json.Marshal(credentials)
			require.NoError(t, err)

			got := ApplyOpenAICreateDefaults(PlatformOpenAI, accountType, credentials)

			after, err := json.Marshal(credentials)
			require.NoError(t, err)
			require.JSONEq(t, string(before), string(after))
			require.Equal(t, "gpt-6-astra", got["model_mapping"].(map[string]any)["gpt-6-astra"])
			require.Equal(t, "gpt-5.6-sol", got["model_mapping"].(map[string]any)["gpt-5.6-sol"])
		})
	}
}

func TestApplyOpenAICreateDefaultsPreservesExplicitAstraMapping(t *testing.T) {
	got := ApplyOpenAICreateDefaults(PlatformOpenAI, AccountTypeOAuth, map[string]any{
		"model_mapping": map[string]string{
			"gpt-6-astra": "private-astra-route",
		},
	})

	require.Equal(t, "private-astra-route", got["model_mapping"].(map[string]any)["gpt-6-astra"])
}

func TestApplyOpenAICreateDefaultsKeepsAllowAllMappingUnrestricted(t *testing.T) {
	got := ApplyOpenAICreateDefaults(PlatformOpenAI, AccountTypeOAuth, map[string]any{
		"access_token": "token",
	})
	account := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: got,
	}

	require.NotContains(t, got, "model_mapping")
	require.True(t, account.IsModelSupported("gpt-6-astra"))
	require.True(t, account.IsModelSupported("future-openai-model"))
}

func TestApplyOpenAICreateDefaultsLeavesNonOfficialAccountsUntouched(t *testing.T) {
	for _, tt := range []struct {
		name        string
		platform    string
		accountType string
	}{
		{name: "OpenAI API key", platform: PlatformOpenAI, accountType: AccountTypeAPIKey},
		{name: "Grok OAuth", platform: PlatformGrok, accountType: AccountTypeOAuth},
	} {
		t.Run(tt.name, func(t *testing.T) {
			credentials := map[string]any{"api_key": "sk-test"}
			got := ApplyOpenAICreateDefaults(tt.platform, tt.accountType, credentials)
			require.Equal(t, credentials, got)
			require.NotContains(t, got, "model_mapping")
		})
	}
}

func TestAccountCreateServicesApplyOpenAIAstraDefault(t *testing.T) {
	t.Run("admin service", func(t *testing.T) {
		repo := &grokDefaultsAccountRepo{}
		svc := &adminServiceImpl{accountRepo: repo}

		account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
			Name:     "OpenAI OAuth",
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Credentials: map[string]any{
				"access_token": "token",
				"model_mapping": map[string]any{
					"gpt-5.6-sol": "gpt-5.6-sol",
				},
			},
			SkipDefaultGroupBind: true,
		})

		require.NoError(t, err)
		require.Equal(t, "gpt-6-astra", account.GetModelMapping()["gpt-6-astra"])
	})

	t.Run("account service", func(t *testing.T) {
		repo := &grokDefaultsAccountRepo{}
		svc := NewAccountService(repo, nil)

		account, err := svc.Create(context.Background(), CreateAccountRequest{
			Name:     "OpenAI Setup Token",
			Platform: PlatformOpenAI,
			Type:     AccountTypeSetupToken,
			Credentials: map[string]any{
				"access_token": "token",
				"model_mapping": map[string]any{
					"gpt-5.6-sol": "gpt-5.6-sol",
				},
			},
		})

		require.NoError(t, err)
		require.Equal(t, "gpt-6-astra", account.GetModelMapping()["gpt-6-astra"])
	})
}
