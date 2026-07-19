package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrokAccountDefaultsAddMissingModelMappingsWithoutMutatingInput(t *testing.T) {
	credentials := map[string]any{"api_key": "sk-test"}

	got := ApplyGrokCreateDefaults(credentials)

	require.NotContains(t, credentials, "model_mapping")
	require.Equal(t, map[string]any{
		"claude-opus-4-8": "grok-4.5",
		"gpt-5.4":         "grok-4.5",
		"grok-4.5":        "grok-4.5",
	}, got["model_mapping"])
	require.True(t, (&Account{Platform: PlatformGrok, Credentials: got}).IsModelSupported("grok-4.5"))
}

func TestGrokAccountDefaultsPreserveExplicitMapping(t *testing.T) {
	credentials := map[string]any{
		"api_key": "sk-test",
		"model_mapping": map[string]any{
			"custom-model": "grok-custom",
		},
	}
	before, err := json.Marshal(credentials)
	require.NoError(t, err)

	got := ApplyGrokCreateDefaults(credentials)
	after, err := json.Marshal(got)
	require.NoError(t, err)

	require.Equal(t, string(before), string(after))
}

func TestAdminServiceCreateAccountAppliesGrokDefaultsOnlyToGrok(t *testing.T) {
	tests := []struct {
		name        string
		platform    string
		wantMapping bool
	}{
		{name: "grok", platform: PlatformGrok, wantMapping: true},
		{name: "openai", platform: PlatformOpenAI, wantMapping: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &grokDefaultsAccountRepo{}
			service := &adminServiceImpl{accountRepo: repo}

			created, err := service.CreateAccount(context.Background(), &CreateAccountInput{
				Name:                 tt.name,
				Platform:             tt.platform,
				Type:                 AccountTypeAPIKey,
				Credentials:          map[string]any{"api_key": "sk-test"},
				SkipDefaultGroupBind: true,
			})

			require.NoError(t, err)
			require.NotNil(t, created)
			if tt.wantMapping {
				require.Equal(t, map[string]any{
					"claude-opus-4-8": "grok-4.5",
					"gpt-5.4":         "grok-4.5",
					"grok-4.5":        "grok-4.5",
				}, created.Credentials["model_mapping"])
			} else {
				require.NotContains(t, created.Credentials, "model_mapping")
			}
		})
	}
}

func TestAccountServiceCreateAppliesGrokDefaultsOnlyToGrok(t *testing.T) {
	tests := []struct {
		name        string
		platform    string
		credentials map[string]any
		wantMapping map[string]any
	}{
		{
			name:        "grok defaults",
			platform:    PlatformGrok,
			credentials: map[string]any{"api_key": "sk-test"},
			wantMapping: map[string]any{
				"claude-opus-4-8": "grok-4.5",
				"gpt-5.4":         "grok-4.5",
				"grok-4.5":        "grok-4.5",
			},
		},
		{
			name:     "grok explicit mapping",
			platform: PlatformGrok,
			credentials: map[string]any{
				"api_key": "sk-test",
				"model_mapping": map[string]any{
					"custom-model": "grok-custom",
				},
			},
			wantMapping: map[string]any{"custom-model": "grok-custom"},
		},
		{
			name:        "openai unchanged",
			platform:    PlatformOpenAI,
			credentials: map[string]any{"api_key": "sk-test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &grokDefaultsAccountRepo{}
			accountService := NewAccountService(repo, nil)

			created, err := accountService.Create(context.Background(), CreateAccountRequest{
				Name:        tt.name,
				Platform:    tt.platform,
				Type:        AccountTypeAPIKey,
				Credentials: tt.credentials,
			})

			require.NoError(t, err)
			if tt.wantMapping == nil {
				require.NotContains(t, created.Credentials, "model_mapping")
				return
			}
			require.Equal(t, tt.wantMapping, created.Credentials["model_mapping"])
		})
	}
}

type grokDefaultsAccountRepo struct {
	AccountRepository
	created *Account
}

func (r *grokDefaultsAccountRepo) Create(_ context.Context, account *Account) error {
	clone := *account
	r.created = &clone
	account.ID = 1
	return nil
}
