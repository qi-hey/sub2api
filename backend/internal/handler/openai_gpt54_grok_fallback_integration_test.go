//go:build unit

package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGPT54GrokFirstResponsesFallsBackWhenGrokHasNoSchedulableAccount(t *testing.T) {
	t.Parallel()
	testGPT54GrokFirstHTTPNoAccountFallback(t, "/openai/v1/responses", `{"model":"gpt-5.4","input":"hello","stream":false}`)
}

func TestGPT54GrokFirstChatCompletionsFallsBackWhenGrokHasNoSchedulableAccount(t *testing.T) {
	t.Parallel()
	testGPT54GrokFirstHTTPNoAccountFallback(t, "/openai/v1/chat/completions", `{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
}

func TestGPT54GrokFirstMessagesFallsBackWhenGrokHasNoSchedulableAccount(t *testing.T) {
	t.Parallel()
	testGPT54GrokFirstHTTPNoAccountFallback(t, "/openai/v1/messages", `{"model":"gpt-5.4","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
}

func TestGPT54GrokFirstResponsesUsesHealthyGrokBeforeOpenAI(t *testing.T) {
	t.Parallel()

	grok := gpt54GrokFirstTestAccount(801, service.PlatformGrok)
	openAI := gpt54GrokFirstTestAccount(900, service.PlatformOpenAI)
	upstream := &grokCredentialHandlerUpstream{}
	router := newGPT54GrokFirstHTTPTestRouter(t, []service.Account{grok, openAI}, upstream)

	recorder := performGPT54GrokFirstRequest(router, "/openai/v1/responses", `{"model":"gpt-5.4","input":"hello","stream":false}`)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, []int64{grok.ID}, upstream.accountHits())
}

func TestGPT54GrokFirstResponsesFallsBackAfterRetryableGrokExhaustion(t *testing.T) {
	t.Parallel()

	grok := gpt54GrokFirstTestAccount(801, service.PlatformGrok)
	openAI := gpt54GrokFirstTestAccount(900, service.PlatformOpenAI)
	upstream := &grokCredentialHandlerUpstream{failureStatus: map[int64]int{grok.ID: http.StatusInternalServerError}}
	router := newGPT54GrokFirstHTTPTestRouter(t, []service.Account{grok, openAI}, upstream)

	recorder := performGPT54GrokFirstRequest(router, "/openai/v1/responses", `{"model":"gpt-5.4","input":"hello","stream":false}`)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, []int64{grok.ID, openAI.ID}, upstream.accountHits())
}

func TestGPT54GrokFirstResponsesFallsBackWhenGrokSwitchBudgetIsExhausted(t *testing.T) {
	t.Parallel()

	grok := gpt54GrokFirstTestAccount(801, service.PlatformGrok)
	openAI := gpt54GrokFirstTestAccount(900, service.PlatformOpenAI)
	upstream := &grokCredentialHandlerUpstream{failureStatus: map[int64]int{grok.ID: http.StatusInternalServerError}}
	router := newGPT54GrokFirstHTTPTestRouterWithMaxSwitches(t, []service.Account{grok, openAI}, upstream, 0)

	recorder := performGPT54GrokFirstRequest(router, "/openai/v1/responses", `{"model":"gpt-5.4","input":"hello","stream":false}`)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, []int64{grok.ID, openAI.ID}, upstream.accountHits())
}

func TestGPT54GrokFirstResponsesKeepsOpenAIStickyAfterSuccessfulFallback(t *testing.T) {
	t.Parallel()

	grok := gpt54GrokFirstTestAccount(801, service.PlatformGrok)
	grok.Schedulable = false
	openAI := gpt54GrokFirstTestAccount(900, service.PlatformOpenAI)
	upstream := &grokCredentialHandlerUpstream{}
	router, repo := newGPT54GrokFirstHTTPTestRouterWithRepo(t, []service.Account{grok, openAI}, upstream, 3)

	first := performGPT54GrokFirstRequestWithSession(router, "/openai/v1/responses", `{"model":"gpt-5.4","input":"first","stream":false}`, "sticky-fallback")
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())

	repo.mu.Lock()
	for i := range repo.accounts {
		if repo.accounts[i].ID == grok.ID {
			repo.accounts[i].Schedulable = true
		}
	}
	repo.mu.Unlock()

	second := performGPT54GrokFirstRequestWithSession(router, "/openai/v1/responses", `{"model":"gpt-5.4","input":"second","stream":false}`, "sticky-fallback")
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	require.Equal(t, []int64{openAI.ID, openAI.ID}, upstream.accountHits())
}

func TestGPT54GrokFirstResponsesDoesNotFallbackOnInvalidGrokRequest(t *testing.T) {
	t.Parallel()

	grok := gpt54GrokFirstTestAccount(801, service.PlatformGrok)
	openAI := gpt54GrokFirstTestAccount(900, service.PlatformOpenAI)
	upstream := &grokCredentialHandlerUpstream{failureStatus: map[int64]int{grok.ID: http.StatusBadRequest}}
	router := newGPT54GrokFirstHTTPTestRouter(t, []service.Account{grok, openAI}, upstream)

	recorder := performGPT54GrokFirstRequest(router, "/openai/v1/responses", `{"model":"gpt-5.4","input":"hello","stream":false}`)

	require.NotEqual(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, []int64{grok.ID}, upstream.accountHits())
}

func TestGPT54GrokFirstResponsesWebSocketFallsBackBeforeFirstFrame(t *testing.T) {
	t.Parallel()

	upstreamHit := make(chan []byte, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, &coderws.AcceptOptions{CompressionMode: coderws.CompressionContextTakeover})
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		readCtx, cancelRead := context.WithTimeout(r.Context(), 3*time.Second)
		_, payload, readErr := conn.Read(readCtx)
		cancelRead()
		if readErr != nil {
			return
		}
		upstreamHit <- payload
		writeCtx, cancelWrite := context.WithTimeout(r.Context(), 3*time.Second)
		_ = conn.Write(writeCtx, coderws.MessageText, []byte(`{"type":"response.completed","response":{"id":"resp_gpt54_fallback","model":"gpt-5.5","usage":{"input_tokens":1,"output_tokens":1}}}`))
		cancelWrite()
		_ = conn.Close(coderws.StatusNormalClosure, "done")
	}))
	t.Cleanup(upstream.Close)

	account := gpt54GrokFirstTestAccount(9900, service.PlatformOpenAI)
	account.Credentials["base_url"] = upstream.URL
	account.Extra = map[string]any{
		"openai_apikey_responses_websockets_v2_enabled": true,
		"openai_apikey_responses_websockets_v2_mode":    service.OpenAIWSIngressModePassthrough,
	}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.MaxAccountSwitches = 3
	repo := &openAIWSFailoverHandlerAccountRepoStub{accounts: []service.Account{account}}
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	gateway := service.NewOpenAIGatewayService(
		repo, nil, nil, nil, nil, nil, nil, cfg, nil, nil,
		service.NewBillingService(cfg, nil), nil, billing, nil,
		&service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil,
	)
	cache := &concurrencyCacheMock{
		acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
		acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
	}
	h := &OpenAIGatewayHandler{
		gatewayService:      gateway,
		billingCacheService: billing,
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second),
		maxAccountSwitches:  3,
		cfg:                 cfg,
	}
	apiKey := gpt54RouteTestAPIKey(service.PlatformGrok, false)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), apiKey)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.User.ID, Concurrency: 1})
		c.Next()
	})
	router.GET("/openai/v1/responses", h.ResponsesWebSocket)
	handlerServer := httptest.NewServer(router)
	t.Cleanup(handlerServer.Close)

	dialCtx, cancelDial := context.WithTimeout(context.Background(), 3*time.Second)
	client, _, err := coderws.Dial(dialCtx, "ws"+strings.TrimPrefix(handlerServer.URL, "http")+"/openai/v1/responses", nil)
	cancelDial()
	require.NoError(t, err)
	defer func() { _ = client.CloseNow() }()
	writeCtx, cancelWrite := context.WithTimeout(context.Background(), 3*time.Second)
	err = client.Write(writeCtx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.4","stream":false}`))
	cancelWrite()
	require.NoError(t, err)
	readCtx, cancelRead := context.WithTimeout(context.Background(), 5*time.Second)
	_, event, err := client.Read(readCtx)
	cancelRead()
	require.NoError(t, err)
	require.Equal(t, "response.completed", gjson.GetBytes(event, "type").String())
	require.Equal(t, "resp_gpt54_fallback", gjson.GetBytes(event, "response.id").String())
	select {
	case <-upstreamHit:
	case <-time.After(3 * time.Second):
		t.Fatal("OpenAI fallback upstream did not receive the replayed first frame")
	}
}

func testGPT54GrokFirstHTTPNoAccountFallback(t *testing.T, path, body string) {
	t.Helper()

	account := gpt54GrokFirstTestAccount(900, service.PlatformOpenAI)
	upstream := &grokCredentialHandlerUpstream{}
	router := newGPT54GrokFirstHTTPTestRouter(t, []service.Account{account}, upstream)
	recorder := performGPT54GrokFirstRequest(router, path, body)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, []int64{account.ID}, upstream.accountHits())
}

func gpt54GrokFirstTestAccount(id int64, platform string) service.Account {
	mappedModel := "gpt-5.5"
	name := "openai-fallback"
	if platform == service.PlatformGrok {
		mappedModel = "grok-4.5"
		name = "grok-primary"
	}
	return service.Account{
		ID:          id,
		Name:        name,
		Platform:    platform,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    1,
		Credentials: map[string]any{
			"api_key":  "sk-test-account",
			"base_url": "https://api.example.test",
			"model_mapping": map[string]any{
				"gpt-5.4": mappedModel,
			},
		},
		Extra: map[string]any{"openai_passthrough": true},
	}
}

func newGPT54GrokFirstHTTPTestRouter(t *testing.T, accounts []service.Account, upstream *grokCredentialHandlerUpstream) *gin.Engine {
	return newGPT54GrokFirstHTTPTestRouterWithMaxSwitches(t, accounts, upstream, 3)
}

func newGPT54GrokFirstHTTPTestRouterWithMaxSwitches(t *testing.T, accounts []service.Account, upstream *grokCredentialHandlerUpstream, maxSwitches int) *gin.Engine {
	router, _ := newGPT54GrokFirstHTTPTestRouterWithRepo(t, accounts, upstream, maxSwitches)
	return router
}

func newGPT54GrokFirstHTTPTestRouterWithRepo(t *testing.T, accounts []service.Account, upstream *grokCredentialHandlerUpstream, maxSwitches int) (*gin.Engine, *grokCredentialHandlerRepo) {
	t.Helper()

	repo := &grokCredentialHandlerRepo{accounts: accounts, missingOnGet: map[int64]bool{}}
	stickyCache := &gpt54GrokFirstGatewayCache{bindings: make(map[gpt54GrokFirstGatewayCacheKey]int64)}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Gateway.MaxAccountSwitches = maxSwitches
	billing := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	gateway := service.NewOpenAIGatewayService(
		repo, nil, nil, nil, nil, nil, stickyCache, cfg, nil, nil,
		service.NewBillingService(cfg, nil), nil, billing, upstream,
		&service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil,
	)
	concurrency := &concurrencyCacheMock{
		acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
		acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
	}
	h := NewOpenAIGatewayHandler(
		gateway,
		service.NewConcurrencyService(concurrency),
		billing,
		&service.APIKeyService{},
		nil,
		nil,
		nil,
		nil,
		cfg,
	)
	h.maxAccountSwitches = maxSwitches
	apiKey := gpt54RouteTestAPIKey(service.PlatformGrok, false)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), apiKey)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.User.ID, Concurrency: 1})
		c.Next()
	})
	router.POST("/openai/v1/responses", h.Responses)
	router.POST("/openai/v1/chat/completions", h.ChatCompletions)
	router.POST("/openai/v1/messages", h.Messages)
	return router, repo
}

func performGPT54GrokFirstRequest(router *gin.Engine, path, body string) *httptest.ResponseRecorder {
	return performGPT54GrokFirstRequestWithSession(router, path, body, "")
}

func performGPT54GrokFirstRequestWithSession(router *gin.Engine, path, body, sessionID string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if sessionID != "" {
		request.Header.Set("session_id", sessionID)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

type gpt54GrokFirstGatewayCacheKey struct {
	groupID int64
	session string
}

type gpt54GrokFirstGatewayCache struct {
	mu       sync.Mutex
	bindings map[gpt54GrokFirstGatewayCacheKey]int64
}

func (c *gpt54GrokFirstGatewayCache) GetSessionAccountID(_ context.Context, groupID int64, session string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bindings[gpt54GrokFirstGatewayCacheKey{groupID: groupID, session: session}], nil
}

func (c *gpt54GrokFirstGatewayCache) SetSessionAccountID(_ context.Context, groupID int64, session string, accountID int64, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bindings[gpt54GrokFirstGatewayCacheKey{groupID: groupID, session: session}] = accountID
	return nil
}

func (c *gpt54GrokFirstGatewayCache) RefreshSessionTTL(context.Context, int64, string, time.Duration) error {
	return nil
}

func (c *gpt54GrokFirstGatewayCache) DeleteSessionAccountID(_ context.Context, groupID int64, session string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.bindings, gpt54GrokFirstGatewayCacheKey{groupID: groupID, session: session})
	return nil
}
