package handler

import (
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var (
	errOpenAIGPT54FallbackNotEligible     = errors.New("gpt-5.4 grok fallback is not eligible")
	errOpenAIGPT54FallbackOutputStarted   = errors.New("gpt-5.4 grok fallback cannot switch after output")
	errOpenAIGPT54FallbackAlreadySwitched = errors.New("gpt-5.4 grok fallback already switched")
)

const (
	openAIGPT54PrimaryRoute = iota
	openAIGPT54FallbackRoute
)

// openAIGPT54Route owns request-local state for the one-way Grok-to-OpenAI
// transition. The authenticated API key remains immutable.
type openAIGPT54Route struct {
	sourceAPIKey        *service.APIKey
	currentAPIKey       *service.APIKey
	currentSubscription *service.UserSubscription
	requestedModel      string
	eligible            bool
	currentRoute        int
	didSwitch           bool
	reason              string
	failedByRoute       [2]map[int64]struct{}
	retriesByRoute      [2]map[int64]int
}

func newOpenAIGPT54Route(apiKey *service.APIKey, subscription *service.UserSubscription, requestedModel string) *openAIGPT54Route {
	platform := ""
	if apiKey != nil && apiKey.Group != nil {
		platform = apiKey.Group.Platform
	}
	return &openAIGPT54Route{
		sourceAPIKey:        apiKey,
		currentAPIKey:       apiKey,
		currentSubscription: subscription,
		requestedModel:      strings.ToLower(strings.TrimSpace(requestedModel)),
		eligible:            strings.EqualFold(strings.TrimSpace(requestedModel), "gpt-5.4") && platform == service.PlatformGrok,
		failedByRoute: [2]map[int64]struct{}{
			openAIGPT54PrimaryRoute:  make(map[int64]struct{}),
			openAIGPT54FallbackRoute: make(map[int64]struct{}),
		},
		retriesByRoute: [2]map[int64]int{
			openAIGPT54PrimaryRoute:  make(map[int64]int),
			openAIGPT54FallbackRoute: make(map[int64]int),
		},
	}
}

func (r *openAIGPT54Route) canFallback() bool {
	return r != nil && r.eligible && !r.didSwitch
}

func (r *openAIGPT54Route) apiKey() *service.APIKey {
	if r == nil {
		return nil
	}
	return r.currentAPIKey
}

func (r *openAIGPT54Route) subscription() *service.UserSubscription {
	if r == nil {
		return nil
	}
	return r.currentSubscription
}

func (r *openAIGPT54Route) platform() string {
	if r == nil || r.currentAPIKey == nil || r.currentAPIKey.Group == nil {
		return service.PlatformOpenAI
	}
	return openAICompatibleRequestPlatform(r.currentAPIKey)
}

func (r *openAIGPT54Route) switched() bool {
	return r != nil && r.didSwitch
}

func (r *openAIGPT54Route) fallbackReason() string {
	if r == nil {
		return ""
	}
	return r.reason
}

func (r *openAIGPT54Route) failedAccountIDs() map[int64]struct{} {
	if r == nil {
		return nil
	}
	return r.failedByRoute[r.currentRoute]
}

func (r *openAIGPT54Route) sameAccountRetries() map[int64]int {
	if r == nil {
		return nil
	}
	return r.retriesByRoute[r.currentRoute]
}

func (r *openAIGPT54Route) primaryFailedAccountIDs() map[int64]struct{} {
	if r == nil {
		return nil
	}
	return r.failedByRoute[openAIGPT54PrimaryRoute]
}

func (r *openAIGPT54Route) primarySameAccountRetries() map[int64]int {
	if r == nil {
		return nil
	}
	return r.retriesByRoute[openAIGPT54PrimaryRoute]
}

func (r *openAIGPT54Route) switchToOpenAI(reason string, outputStarted bool) error {
	if r == nil || !r.eligible {
		return errOpenAIGPT54FallbackNotEligible
	}
	if r.didSwitch {
		return errOpenAIGPT54FallbackAlreadySwitched
	}
	if outputStarted {
		return errOpenAIGPT54FallbackOutputStarted
	}

	fallbackAPIKey, err := service.ResolveAPIKeyRequestPlatform(r.sourceAPIKey, service.PlatformOpenAI)
	if err != nil {
		return err
	}
	r.currentAPIKey = fallbackAPIKey
	r.currentSubscription = nil
	r.currentRoute = openAIGPT54FallbackRoute
	r.didSwitch = true
	r.reason = strings.TrimSpace(reason)
	return nil
}
