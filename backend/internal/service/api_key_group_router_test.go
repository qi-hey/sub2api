package service

import (
	"net/http"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestResolveAPIKeyRequestGroupRoutesAliasesAndModelFamilies(t *testing.T) {
	key := testMultiGroupRoutingAPIKey()
	tests := []struct {
		name     string
		model    string
		wantID   int64
		platform string
	}{
		{name: "gpt alias uses grok", model: "gpt-5.4", wantID: 12, platform: PlatformGrok},
		{name: "claude alias is trimmed and case folded", model: "  CLAUDE-OPUS-4-8  ", wantID: 12, platform: PlatformGrok},
		{name: "grok family", model: "grok-4.5", wantID: 12, platform: PlatformGrok},
		{name: "gpt family", model: "gpt-5.5", wantID: 2, platform: PlatformOpenAI},
		{name: "gpt sol family", model: "gpt-5.6-sol", wantID: 2, platform: PlatformOpenAI},
		{name: "claude family", model: "claude-fable-5", wantID: 11, platform: PlatformAnthropic},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beforeDefault := *key.GroupID
			beforeGroup := *key.Group

			selected, err := ResolveAPIKeyRequestGroup(key, tt.model)

			require.NoError(t, err)
			require.NotSame(t, key, selected)
			require.Equal(t, tt.wantID, *selected.GroupID)
			require.NotNil(t, selected.Group)
			require.Equal(t, tt.platform, selected.Group.Platform)
			require.Equal(t, beforeDefault, *key.GroupID)
			require.Equal(t, beforeGroup, *key.Group)
		})
	}
}

func TestResolveAPIKeyRequestGroupFallsBackToActiveDefault(t *testing.T) {
	for _, model := range []string{"", "   ", "other-model"} {
		key := testMultiGroupRoutingAPIKey()
		selected, err := ResolveAPIKeyRequestGroup(key, model)
		require.NoError(t, err)
		require.Equal(t, int64(2), *selected.GroupID)
		require.Equal(t, PlatformOpenAI, selected.Group.Platform)
	}
}

func TestResolveAPIKeyRequestGroupSupportsLegacyDefaultHydration(t *testing.T) {
	defaultID := int64(2)
	key := &APIKey{
		GroupID: &defaultID,
		Group: &Group{
			ID:       defaultID,
			Name:     "Legacy Codex",
			Platform: PlatformOpenAI,
			Status:   StatusActive,
		},
	}

	selected, err := ResolveAPIKeyRequestGroup(key, "unknown")

	require.NoError(t, err)
	require.Equal(t, defaultID, *selected.GroupID)
	require.Equal(t, "Legacy Codex", selected.Group.Name)
}

func TestResolveAPIKeyRequestGroupRejectsMissingOrInactiveGroups(t *testing.T) {
	t.Run("nil api key", func(t *testing.T) {
		selected, err := ResolveAPIKeyRequestGroup(nil, "gpt-5.5")
		require.Nil(t, selected)
		require.ErrorIs(t, err, ErrAPIKeyGroupNotBound)
	})

	t.Run("requested platform is unbound", func(t *testing.T) {
		key := testMultiGroupRoutingAPIKey()
		key.Groups = key.Groups[:2]
		selected, err := ResolveAPIKeyRequestGroup(key, "gpt-5.4")
		require.Nil(t, selected)
		require.ErrorIs(t, err, ErrAPIKeyGroupNotBound)
	})

	t.Run("requested platform is inactive", func(t *testing.T) {
		key := testMultiGroupRoutingAPIKey()
		key.Groups[2].Status = StatusDisabled
		selected, err := ResolveAPIKeyRequestGroup(key, "gpt-5.4")
		require.Nil(t, selected)
		require.ErrorIs(t, err, ErrAPIKeyGroupNotBound)
	})

	t.Run("fallback default is inactive", func(t *testing.T) {
		key := testMultiGroupRoutingAPIKey()
		key.Group.Status = StatusDisabled
		key.Groups[0].Status = StatusDisabled
		selected, err := ResolveAPIKeyRequestGroup(key, "unknown")
		require.Nil(t, selected)
		require.ErrorIs(t, err, ErrAPIKeyGroupNotBound)
	})
}

func TestResolveAPIKeyRequestGroupHandlesMultipleGroupsForPlatform(t *testing.T) {
	t.Run("default group disambiguates matching platform", func(t *testing.T) {
		defaultID := int64(12)
		key := &APIKey{
			GroupID:  &defaultID,
			Group:    &Group{ID: 12, Platform: PlatformGrok, Status: StatusActive},
			GroupIDs: []int64{12, 13},
			Groups: []Group{
				{ID: 12, Platform: PlatformGrok, Status: StatusActive},
				{ID: 13, Platform: PlatformGrok, Status: StatusActive},
			},
		}

		selected, err := ResolveAPIKeyRequestGroup(key, "gpt-5.4")
		require.NoError(t, err)
		require.Equal(t, int64(12), *selected.GroupID)
	})

	t.Run("non matching default leaves configuration ambiguous", func(t *testing.T) {
		key := testMultiGroupRoutingAPIKey()
		key.GroupIDs = append(key.GroupIDs, 13)
		key.Groups = append(key.Groups, Group{ID: 13, Platform: PlatformGrok, Status: StatusActive})

		selected, err := ResolveAPIKeyRequestGroup(key, "gpt-5.4")
		require.Nil(t, selected)
		require.ErrorIs(t, err, ErrAPIKeyGroupAmbiguous)
		require.Equal(t, http.StatusInternalServerError, infraerrors.Code(err))
		require.Equal(t, "API_KEY_GROUP_AMBIGUOUS", infraerrors.Reason(err))
	})
}

func TestResolveAPIKeyRequestGroupUsesTheValidatedDuplicateGroup(t *testing.T) {
	defaultID := int64(2)
	key := &APIKey{
		GroupID: &defaultID,
		Group:   &Group{ID: 2, Platform: PlatformOpenAI, Status: StatusActive},
		Groups: []Group{
			{ID: 2, Platform: PlatformOpenAI, Status: StatusActive},
			{ID: 12, Name: "stale", Platform: PlatformGrok, Status: StatusDisabled},
			{ID: 12, Name: "active", Platform: PlatformGrok, Status: StatusActive},
		},
	}

	selected, err := ResolveAPIKeyRequestGroup(key, "gpt-5.4")

	require.NoError(t, err)
	require.Equal(t, int64(12), *selected.GroupID)
	require.Equal(t, "active", selected.Group.Name)
	require.Equal(t, StatusActive, selected.Group.Status)
}

func TestResolveAPIKeyRequestGroupRejectsConflictingActiveDuplicateID(t *testing.T) {
	defaultID := int64(2)
	key := &APIKey{
		GroupID: &defaultID,
		Group:   &Group{ID: 2, Platform: PlatformOpenAI, Status: StatusActive},
		Groups: []Group{
			{ID: 2, Platform: PlatformOpenAI, Status: StatusActive},
			{ID: 12, Platform: PlatformOpenAI, Status: StatusActive},
			{ID: 12, Platform: PlatformGrok, Status: StatusActive},
		},
	}

	selected, err := ResolveAPIKeyRequestGroup(key, "gpt-5.4")

	require.Nil(t, selected)
	require.ErrorIs(t, err, ErrAPIKeyGroupConflict)
	require.Equal(t, http.StatusInternalServerError, infraerrors.Code(err))
	require.Equal(t, "API_KEY_GROUP_CONFLICT", infraerrors.Reason(err))
}

func TestResolveAPIKeyRequestGroupUnboundErrorContract(t *testing.T) {
	selected, err := ResolveAPIKeyRequestGroup(&APIKey{}, "gpt-5.5")
	require.Nil(t, selected)
	require.ErrorIs(t, err, ErrAPIKeyGroupNotBound)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "API_KEY_GROUP_NOT_BOUND", infraerrors.Reason(err))
}

func testMultiGroupRoutingAPIKey() *APIKey {
	defaultID := int64(2)
	return &APIKey{
		GroupID: &defaultID,
		Group: &Group{
			ID:       2,
			Name:     "Codex",
			Platform: PlatformOpenAI,
			Status:   StatusActive,
		},
		GroupIDs: []int64{2, 11, 12},
		Groups: []Group{
			{ID: 2, Name: "Codex", Platform: PlatformOpenAI, Status: StatusActive},
			{ID: 11, Name: "Claude", Platform: PlatformAnthropic, Status: StatusActive},
			{ID: 12, Name: "Grok", Platform: PlatformGrok, Status: StatusActive},
		},
	}
}
