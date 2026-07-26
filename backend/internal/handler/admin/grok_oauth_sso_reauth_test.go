package admin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type grokSSOReauthOAuthClientStub struct {
	ssoResponse *xai.TokenResponse
}

func (s *grokSSOReauthOAuthClientStub) ExchangeCode(context.Context, string, string, string, string, string) (*xai.TokenResponse, error) {
	return &xai.TokenResponse{}, nil
}

func (s *grokSSOReauthOAuthClientStub) RefreshToken(context.Context, string, string, string) (*xai.TokenResponse, error) {
	return &xai.TokenResponse{}, nil
}

func (s *grokSSOReauthOAuthClientStub) ConvertSSOToBuild(context.Context, string, string) (*xai.TokenResponse, error) {
	return s.ssoResponse, nil
}

func TestNormalizeSSOImportItemsSupportsEmailPrefixAndCookieHeaders(t *testing.T) {
	items := normalizeSSOImportItems([]string{
		"User@example.com:header.payload.signature",
		"Cookie: other=value; sso=cookie-token; theme=dark",
		"raw-token",
	}, "")

	require.Len(t, items, 3)
	require.Equal(t, "user@example.com", items[0].emailHint)
	require.Equal(t, "header.payload.signature", items[0].token)
	require.Equal(t, "cookie-token", items[1].token)
	require.Empty(t, items[1].emailHint)
	require.Equal(t, "raw-token", items[2].token)
}

func TestNormalizeSSOImportItemsDeduplicatesNormalizedTokens(t *testing.T) {
	items := normalizeSSOImportItems([]string{
		"first@example.com:same-token",
		"second@example.com:same-token",
		"sso=another-token",
		"Cookie: sso=another-token",
	}, "")

	require.Len(t, items, 2)
	require.Equal(t, "first@example.com", items[0].emailHint)
	require.Equal(t, "same-token", items[0].token)
	require.Equal(t, "another-token", items[1].token)
}

func TestPrepareGrokSSOReauthUsesEmailHintForPreviewWithoutConversion(t *testing.T) {
	account := &service.Account{ID: 42, Name: "matched"}
	handler := &GrokOAuthHandler{}
	items := []grokSSOImportToken{{index: 1, token: "not-converted", emailHint: "user@example.com"}}

	prepared := handler.prepareGrokSSOReauth(
		context.Background(),
		GrokSSOReauthRequest{Preview: true},
		items,
		map[string][]*service.Account{"user@example.com": {account}},
		nil,
	)

	require.Len(t, prepared, 1)
	require.Equal(t, "update", prepared[0].action)
	require.Equal(t, []*service.Account{account}, prepared[0].matches)
	require.Nil(t, prepared[0].tokenInfo)
	require.Equal(t, "user@example.com", grokSSOReauthEmail(prepared[0]))
}

func TestValidateGrokSSOReauthEmailHintRequiresConvertedIdentityMatch(t *testing.T) {
	require.NoError(t, validateGrokSSOReauthEmailHint("User@example.com", &service.GrokTokenInfo{Email: "user@example.com"}))
	require.NoError(t, validateGrokSSOReauthEmailHint("", &service.GrokTokenInfo{Email: "other@example.com"}))
	require.Error(t, validateGrokSSOReauthEmailHint("user@example.com", &service.GrokTokenInfo{Email: "other@example.com"}))
	require.Error(t, validateGrokSSOReauthEmailHint("user@example.com", &service.GrokTokenInfo{}))
}

func TestPrepareGrokSSOReauthApplyRejectsMismatchedConvertedIdentity(t *testing.T) {
	oauthService := service.NewGrokOAuthService(nil, &grokSSOReauthOAuthClientStub{
		ssoResponse: &xai.TokenResponse{
			AccessToken: grokSSOReauthJWT(map[string]any{"sub": "other-sub"}),
			IDToken:     grokSSOReauthJWT(map[string]any{"email": "other@example.com"}),
			ExpiresIn:   3600,
		},
	})
	defer oauthService.Stop()
	handler := &GrokOAuthHandler{grokOAuthService: oauthService}

	prepared := handler.prepareOneGrokSSOReauth(
		context.Background(),
		GrokSSOReauthRequest{Confirmed: true},
		grokSSOImportToken{index: 1, token: "real-sso-token", emailHint: "claimed@example.com"},
		map[string][]*service.Account{"claimed@example.com": {{ID: 42, Name: "claimed"}}},
		nil,
	)

	require.Equal(t, "conflict", prepared.action)
	require.Contains(t, prepared.err, "does not match")
	require.Empty(t, prepared.matches)
}

func grokSSOReauthJWT(claims map[string]any) string {
	payload, _ := json.Marshal(claims)
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

func TestMatchGrokAccountsForSSOPrefersEmailThenSubject(t *testing.T) {
	emailAccount := &service.Account{ID: 1, Name: "email"}
	subAccount := &service.Account{ID: 2, Name: "sub"}
	byEmail := map[string][]*service.Account{"user@example.com": {emailAccount}}
	bySub := map[string][]*service.Account{"subject-1": {subAccount}}

	matches := matchGrokAccountsForSSO(&service.GrokTokenInfo{
		Email:   " USER@example.com ",
		Subject: "subject-1",
	}, byEmail, bySub)
	require.Equal(t, []*service.Account{emailAccount}, matches)

	matches = matchGrokAccountsForSSO(&service.GrokTokenInfo{Subject: "subject-1"}, byEmail, bySub)
	require.Equal(t, []*service.Account{subAccount}, matches)
}

func TestMarkGrokSSOReauthConflictsRejectsDuplicateAccountTargets(t *testing.T) {
	account := &service.Account{ID: 42, Name: "target"}
	items := []grokSSOReauthPrepared{
		{index: 1, action: "update", matches: []*service.Account{account}},
		{index: 2, action: "update", matches: []*service.Account{account}},
	}

	markGrokSSOReauthConflicts(items, false)
	require.Equal(t, "conflict", items[0].action)
	require.Equal(t, "conflict", items[1].action)
	require.Contains(t, items[0].err, "multiple uploaded SSO tokens")
}

func TestMarkGrokSSOReauthConflictsRejectsDuplicateCreates(t *testing.T) {
	items := []grokSSOReauthPrepared{
		{index: 1, action: "create", tokenInfo: &service.GrokTokenInfo{Email: "same@example.com"}},
		{index: 2, action: "create", tokenInfo: &service.GrokTokenInfo{Email: "SAME@example.com"}},
	}

	markGrokSSOReauthConflicts(items, true)
	require.Equal(t, "conflict", items[0].action)
	require.Equal(t, "conflict", items[1].action)
}
