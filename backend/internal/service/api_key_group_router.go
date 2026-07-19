package service

import (
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrAPIKeyGroupAmbiguous = infraerrors.InternalServer(
	"API_KEY_GROUP_AMBIGUOUS",
	"multiple active API key groups match the requested model platform",
)

var ErrAPIKeyGroupConflict = infraerrors.InternalServer(
	"API_KEY_GROUP_CONFLICT",
	"conflicting active API key groups share the same ID",
)

// ResolveAPIKeyRequestGroup selects an active bound group for one request.
// The returned API key is request-local; the authenticated source is unchanged.
func ResolveAPIKeyRequestGroup(apiKey *APIKey, model string) (*APIKey, error) {
	return resolveAPIKeyRequestPlatform(apiKey, APIKeyRequestPlatformForModel(model))
}

// APIKeyRequestPlatformForModel returns the routing platform implied by a model.
// An empty result means the API key's default group should be used.
func APIKeyRequestPlatformForModel(model string) string {
	return requestedAPIKeyPlatform(model)
}

// ResolveAPIKeyRequestPlatform selects an active bound group for an endpoint
// whose platform is authoritative regardless of a request model.
func ResolveAPIKeyRequestPlatform(apiKey *APIKey, platform string) (*APIKey, error) {
	return resolveAPIKeyRequestPlatform(apiKey, strings.ToLower(strings.TrimSpace(platform)))
}

func resolveAPIKeyRequestPlatform(apiKey *APIKey, platform string) (*APIKey, error) {
	if apiKey == nil {
		return nil, ErrAPIKeyGroupNotBound
	}

	groups, err := activeAPIKeyRoutingGroups(apiKey)
	if err != nil {
		return nil, err
	}
	if platform == "" {
		return selectDefaultAPIKeyGroup(apiKey, groups)
	}

	matches := make([]Group, 0, 1)
	for _, group := range groups {
		if group.Platform == platform {
			matches = append(matches, group)
		}
	}
	if len(matches) == 0 {
		return nil, ErrAPIKeyGroupNotBound
	}
	if len(matches) == 1 {
		return selectResolvedAPIKeyGroup(apiKey, matches[0])
	}
	if apiKey.GroupID != nil {
		for i := range matches {
			if matches[i].ID == *apiKey.GroupID {
				return selectResolvedAPIKeyGroup(apiKey, matches[i])
			}
		}
	}
	return nil, ErrAPIKeyGroupAmbiguous
}

func requestedAPIKeyPlatform(model string) string {
	normalized := strings.ToLower(strings.TrimSpace(model))
	switch normalized {
	case "gpt-5.4", "claude-opus-4-8":
		return PlatformGrok
	}
	switch {
	case strings.HasPrefix(normalized, "grok-"):
		return PlatformGrok
	case strings.HasPrefix(normalized, "gpt-"):
		return PlatformOpenAI
	case strings.HasPrefix(normalized, "claude-"):
		return PlatformAnthropic
	default:
		return ""
	}
}

func activeAPIKeyRoutingGroups(apiKey *APIKey) (map[int64]Group, error) {
	groups := make(map[int64]Group, len(apiKey.Groups)+1)
	for i := range apiKey.Groups {
		group := apiKey.Groups[i]
		if group.ID <= 0 || group.Status != StatusActive {
			continue
		}
		if existing, exists := groups[group.ID]; exists {
			if existing.Platform != group.Platform {
				return nil, ErrAPIKeyGroupConflict
			}
			continue
		}
		groups[group.ID] = group
	}
	if apiKey.GroupID != nil && apiKey.Group != nil &&
		apiKey.Group.ID == *apiKey.GroupID && apiKey.Group.ID > 0 &&
		apiKey.Group.Status == StatusActive {
		if existing, exists := groups[apiKey.Group.ID]; exists {
			if existing.Platform != apiKey.Group.Platform {
				return nil, ErrAPIKeyGroupConflict
			}
		} else {
			groups[apiKey.Group.ID] = *apiKey.Group
		}
	}
	return groups, nil
}

func selectDefaultAPIKeyGroup(apiKey *APIKey, groups map[int64]Group) (*APIKey, error) {
	if apiKey.GroupID == nil {
		return nil, ErrAPIKeyGroupNotBound
	}
	group, ok := groups[*apiKey.GroupID]
	if !ok {
		return nil, ErrAPIKeyGroupNotBound
	}
	return selectResolvedAPIKeyGroup(apiKey, group)
}

func selectResolvedAPIKeyGroup(apiKey *APIKey, group Group) (*APIKey, error) {
	selected, err := apiKey.WithSelectedGroup(group.ID)
	if err != nil {
		return nil, err
	}
	selectedID := group.ID
	selected.GroupID = &selectedID
	selected.Group = cloneAPIKeyGroup(&group)
	return selected, nil
}
