package dto

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyFromService_MapsLastUsedAt(t *testing.T) {
	lastUsed := time.Now().UTC().Truncate(time.Second)
	lastUsedIP := "203.0.113.10"
	src := &service.APIKey{
		ID:                 1,
		UserID:             2,
		Key:                "sk-map-last-used",
		Name:               "Mapper",
		Status:             service.StatusActive,
		LastUsedAt:         &lastUsed,
		LastUsedIP:         &lastUsedIP,
		CurrentConcurrency: 3,
	}

	out := APIKeyFromService(src)
	require.NotNil(t, out)
	require.NotNil(t, out.LastUsedAt)
	require.WithinDuration(t, lastUsed, *out.LastUsedAt, time.Second)
	require.NotNil(t, out.LastUsedIP)
	require.Equal(t, lastUsedIP, *out.LastUsedIP)
	require.Equal(t, 3, out.CurrentConcurrency)
}

func TestAPIKeyFromService_MapsNilLastUsedAt(t *testing.T) {
	src := &service.APIKey{
		ID:     1,
		UserID: 2,
		Key:    "sk-map-last-used-nil",
		Name:   "MapperNil",
		Status: service.StatusActive,
	}

	out := APIKeyFromService(src)
	require.NotNil(t, out)
	require.Nil(t, out.LastUsedAt)
	require.Nil(t, out.LastUsedIP)
}

func TestAPIKeyFromServiceMapsSortedGroupBindings(t *testing.T) {
	defaultID := int64(2)
	src := &service.APIKey{
		ID:       1,
		UserID:   2,
		Key:      "sk-map-groups",
		Name:     "MapperGroups",
		Status:   service.StatusActive,
		GroupID:  &defaultID,
		Group:    &service.Group{ID: 2, Name: "Codex", Platform: service.PlatformOpenAI, Status: service.StatusActive},
		GroupIDs: []int64{12, 2, 11, 12},
		Groups: []service.Group{
			{ID: 12, Name: "Grok", Platform: service.PlatformGrok, Status: service.StatusActive},
			{ID: 2, Name: "Codex", Platform: service.PlatformOpenAI, Status: service.StatusActive},
			{ID: 11, Name: "Claude", Platform: service.PlatformAnthropic, Status: service.StatusActive},
		},
	}

	out := APIKeyFromService(src)

	require.Equal(t, []int64{2, 11, 12}, out.GroupIDs)
	require.Len(t, out.Groups, 3)
	require.Equal(t, []int64{2, 11, 12}, []int64{out.Groups[0].ID, out.Groups[1].ID, out.Groups[2].ID})
}
