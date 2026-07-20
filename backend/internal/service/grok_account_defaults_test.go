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

func TestAdminServiceCreateAccountBindsOnlyActiveGrokGroupWhenGroupIDsMissing(t *testing.T) {
	repo := &grokDefaultsAccountRepo{}
	groupRepo := &grokDefaultsGroupRepo{
		groups: []Group{{ID: 12, Name: "Grok", Platform: PlatformGrok, Status: StatusActive}},
	}
	service := &adminServiceImpl{accountRepo: repo, groupRepo: groupRepo}

	created, err := service.CreateAccount(context.Background(), &CreateAccountInput{
		Name:        "grok oauth",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "token"},
	})

	require.NoError(t, err)
	require.NotNil(t, created)
	require.Equal(t, []int64{12}, repo.boundGroupIDs)
}

func TestAdminServiceCreateAccountPreservesExplicitGrokGroupIDs(t *testing.T) {
	repo := &grokDefaultsAccountRepo{}
	service := &adminServiceImpl{accountRepo: repo}

	_, err := service.CreateAccount(context.Background(), &CreateAccountInput{
		Name:        "grok oauth",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "token"},
		GroupIDs:    []int64{99},
	})

	require.NoError(t, err)
	require.Equal(t, []int64{99}, repo.boundGroupIDs)
}

func TestAdminServiceCreateAccountLeavesGrokUngroupedWhenMultipleGroupsAreAmbiguous(t *testing.T) {
	repo := &grokDefaultsAccountRepo{}
	groupRepo := &grokDefaultsGroupRepo{
		groups: []Group{{ID: 12, Name: "Grok"}, {ID: 13, Name: "Grok backup"}},
	}
	service := &adminServiceImpl{accountRepo: repo, groupRepo: groupRepo}

	_, err := service.CreateAccount(context.Background(), &CreateAccountInput{
		Name:        "grok oauth",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "token"},
	})

	require.NoError(t, err)
	require.Nil(t, repo.boundGroupIDs)
}

func TestAdminServiceCreateAccountKeepsNamedDefaultForOtherPlatforms(t *testing.T) {
	repo := &grokDefaultsAccountRepo{}
	groupRepo := &grokDefaultsGroupRepo{
		groups: []Group{{ID: 2, Name: "openai-default"}},
	}
	service := &adminServiceImpl{accountRepo: repo, groupRepo: groupRepo}

	_, err := service.CreateAccount(context.Background(), &CreateAccountInput{
		Name:        "openai api key",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test"},
	})

	require.NoError(t, err)
	require.Equal(t, []int64{2}, repo.boundGroupIDs)
}

func TestDefaultGroupIDsForCreate(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		groups   []Group
		want     []int64
	}{
		{
			name:     "named default wins",
			platform: PlatformGrok,
			groups:   []Group{{ID: 12, Name: "Grok"}, {ID: 13, Name: "grok-default"}},
			want:     []int64{13},
		},
		{
			name:     "unique Grok group",
			platform: PlatformGrok,
			groups:   []Group{{ID: 12, Name: "Grok"}},
			want:     []int64{12},
		},
		{
			name:     "ambiguous Grok groups",
			platform: PlatformGrok,
			groups:   []Group{{ID: 12, Name: "Grok"}, {ID: 13, Name: "Grok backup"}},
		},
		{
			name:     "other platform keeps existing behavior",
			platform: PlatformOpenAI,
			groups:   []Group{{ID: 2, Name: "Codex"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, defaultGroupIDsForCreate(tt.platform, tt.groups))
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
	created       *Account
	boundGroupIDs []int64
}

func (r *grokDefaultsAccountRepo) Create(_ context.Context, account *Account) error {
	clone := *account
	r.created = &clone
	account.ID = 1
	return nil
}

func (r *grokDefaultsAccountRepo) BindGroups(_ context.Context, _ int64, groupIDs []int64) error {
	r.boundGroupIDs = append([]int64(nil), groupIDs...)
	return nil
}

type grokDefaultsGroupRepo struct {
	GroupRepository
	groups []Group
}

func (r *grokDefaultsGroupRepo) ListActiveByPlatform(_ context.Context, _ string) ([]Group, error) {
	return append([]Group(nil), r.groups...), nil
}
