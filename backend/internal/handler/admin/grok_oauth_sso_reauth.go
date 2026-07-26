package admin

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GrokSSOReauthRequest previews or applies SSO-based credential replacement.
// Apply requests must opt in explicitly with confirmed=true.
type GrokSSOReauthRequest struct {
	SSOTokens          []string       `json:"sso_tokens"`
	SSOToken           string         `json:"sso_token"`
	ProxyID            *int64         `json:"proxy_id"`
	Preview            bool           `json:"preview"`
	Confirmed          bool           `json:"confirmed"`
	CreateIfMissing    bool           `json:"create_if_missing"`
	Name               string         `json:"name"`
	Notes              *string        `json:"notes"`
	GroupIDs           []int64        `json:"group_ids"`
	Credentials        map[string]any `json:"credentials"`
	Extra              map[string]any `json:"extra"`
	Concurrency        int            `json:"concurrency"`
	LoadFactor         *int           `json:"load_factor"`
	Priority           int            `json:"priority"`
	RateMultiplier     *float64       `json:"rate_multiplier"`
	ExpiresAt          *int64         `json:"expires_at"`
	AutoPauseOnExpired *bool          `json:"auto_pause_on_expired"`
}

type GrokSSOReauthPreviewItem struct {
	Index       int    `json:"index"`
	Email       string `json:"email,omitempty"`
	Subject     string `json:"sub,omitempty"`
	Action      string `json:"action"`
	AccountID   int64  `json:"account_id,omitempty"`
	AccountName string `json:"account_name,omitempty"`
	Error       string `json:"error,omitempty"`
}

type GrokSSOReauthResponse struct {
	Preview []GrokSSOReauthPreviewItem `json:"preview,omitempty"`
	Updated []GrokSSOToOAuthItemResult `json:"updated"`
	Created []GrokSSOToOAuthItemResult `json:"created"`
	Failed  []GrokSSOToOAuthItemResult `json:"failed"`
}

type grokSSOReauthPrepared struct {
	index     int
	token     string
	emailHint string
	tokenInfo *service.GrokTokenInfo
	matches   []*service.Account
	action    string
	err       string
}

// ReauthAccountsFromSSO previews email-prefixed imports without exchanging
// cookies, then validates the real SSO identity before applying any update.
func (h *GrokOAuthHandler) ReauthAccountsFromSSO(c *gin.Context) {
	var req GrokSSOReauthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	items := normalizeSSOImportItems(req.SSOTokens, req.SSOToken)
	if len(items) == 0 {
		response.BadRequest(c, "sso_tokens is required")
		return
	}
	if !req.Preview && !req.Confirmed {
		response.BadRequest(c, "confirmed=true is required after reviewing the SSO reauthentication preview")
		return
	}

	ctx := c.Request.Context()
	accounts, err := h.adminService.ListAccountsForSchedulerScoreFilter(ctx, service.PlatformGrok, service.AccountTypeOAuth, "", "", 0, "")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	byEmail, bySub := indexGrokAccountsForSSO(accounts)
	prepared := h.prepareGrokSSOReauth(ctx, req, items, byEmail, bySub)
	markGrokSSOReauthConflicts(prepared, req.CreateIfMissing)

	result := GrokSSOReauthResponse{
		Updated: make([]GrokSSOToOAuthItemResult, 0, len(items)),
		Created: make([]GrokSSOToOAuthItemResult, 0),
		Failed:  make([]GrokSSOToOAuthItemResult, 0),
	}
	if req.Preview {
		result.Preview = make([]GrokSSOReauthPreviewItem, 0, len(prepared))
		for _, item := range prepared {
			result.Preview = append(result.Preview, grokSSOReauthPreview(item))
		}
		response.Success(c, result)
		return
	}

	for _, item := range prepared {
		if item.err != "" {
			result.Failed = append(result.Failed, GrokSSOToOAuthItemResult{
				Index: item.index,
				Email: grokSSOReauthEmail(item),
				Error: item.err,
			})
			continue
		}
		switch item.action {
		case "create":
			created := h.createAccountFromSSOToken(ctx, grokSSOReauthCreateRequest(req), item.token, item.index, len(items))
			if created.item.Error != "" {
				result.Failed = append(result.Failed, created.item)
			} else {
				result.Created = append(result.Created, created.item)
			}
		case "update":
			updated := h.applyPreparedGrokSSOReauth(ctx, req, item)
			if updated.Error != "" {
				result.Failed = append(result.Failed, updated)
			} else {
				result.Updated = append(result.Updated, updated)
			}
		default:
			result.Failed = append(result.Failed, GrokSSOToOAuthItemResult{
				Index: item.index,
				Email: grokSSOReauthEmail(item),
				Error: "SSO item is not eligible for reauthentication",
			})
		}
	}
	response.Success(c, result)
}

func indexGrokAccountsForSSO(accounts []service.Account) (map[string][]*service.Account, map[string][]*service.Account) {
	byEmail := make(map[string][]*service.Account)
	bySub := make(map[string][]*service.Account)
	for i := range accounts {
		account := &accounts[i]
		if email := strings.ToLower(strings.TrimSpace(account.GetCredential("email"))); email != "" {
			byEmail[email] = append(byEmail[email], account)
		}
		if sub := strings.TrimSpace(account.GetCredential("sub")); sub != "" {
			bySub[sub] = append(bySub[sub], account)
		}
	}
	return byEmail, bySub
}

func (h *GrokOAuthHandler) prepareGrokSSOReauth(
	ctx context.Context,
	req GrokSSOReauthRequest,
	tokens []grokSSOImportToken,
	byEmail, bySub map[string][]*service.Account,
) []grokSSOReauthPrepared {
	workerCount := grokSSOImportConcurrency
	if workerCount > len(tokens) {
		workerCount = len(tokens)
	}
	jobs := make(chan int)
	items := make([]grokSSOReauthPrepared, len(tokens))
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for position := range jobs {
				items[position] = h.prepareOneGrokSSOReauth(ctx, req, tokens[position], byEmail, bySub)
			}
		}()
	}
	for i := range tokens {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return items
}

func (h *GrokOAuthHandler) prepareOneGrokSSOReauth(
	ctx context.Context,
	req GrokSSOReauthRequest,
	item grokSSOImportToken,
	byEmail, bySub map[string][]*service.Account,
) (result grokSSOReauthPrepared) {
	result = grokSSOReauthPrepared{index: item.index, token: item.token, emailHint: item.emailHint}
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error("grok_sso_reauth_worker_panic", "index", item.index, "recover", recovered)
			result.err = fmt.Sprintf("worker panic: %v", recovered)
		}
	}()

	if req.Preview && !req.CreateIfMissing && item.emailHint != "" {
		result.matches = append([]*service.Account(nil), byEmail[item.emailHint]...)
		setGrokSSOReauthAction(&result, req.CreateIfMissing)
		return result
	}

	tokenInfo, err := h.grokOAuthService.ConvertFromSSO(ctx, item.token, req.ProxyID)
	if err != nil {
		result.err = grokSSOReauthErrorMessage(err)
		return result
	}
	result.tokenInfo = tokenInfo
	if err := validateGrokSSOReauthEmailHint(item.emailHint, tokenInfo); err != nil {
		result.action = "conflict"
		result.err = err.Error()
		return result
	}
	result.matches = matchGrokAccountsForSSO(tokenInfo, byEmail, bySub)
	setGrokSSOReauthAction(&result, req.CreateIfMissing)
	return result
}

func setGrokSSOReauthAction(result *grokSSOReauthPrepared, createIfMissing bool) {
	if result == nil {
		return
	}
	switch len(result.matches) {
	case 0:
		if createIfMissing {
			result.action = "create"
		} else {
			result.action = "unmatched"
			result.err = "no matching Grok account found for SSO email/sub"
		}
	case 1:
		result.action = "update"
	default:
		result.action = "conflict"
		result.err = fmt.Sprintf("SSO identity matches %d Grok accounts; resolve duplicate email/sub before reauthentication", len(result.matches))
	}
}

func validateGrokSSOReauthEmailHint(emailHint string, tokenInfo *service.GrokTokenInfo) error {
	emailHint = strings.ToLower(strings.TrimSpace(emailHint))
	if emailHint == "" {
		return nil
	}
	actualEmail := ""
	if tokenInfo != nil {
		actualEmail = strings.ToLower(strings.TrimSpace(tokenInfo.Email))
	}
	if actualEmail == emailHint {
		return nil
	}
	return fmt.Errorf("SSO identity email does not match uploaded email prefix")
}

func markGrokSSOReauthConflicts(items []grokSSOReauthPrepared, createIfMissing bool) {
	byAccount := make(map[int64][]int)
	byIdentity := make(map[string][]int)
	for i := range items {
		item := &items[i]
		if item.err != "" {
			continue
		}
		if item.action == "update" && len(item.matches) == 1 {
			byAccount[item.matches[0].ID] = append(byAccount[item.matches[0].ID], i)
		}
		if createIfMissing && item.action == "create" {
			identity := grokSSOReauthIdentity(item.tokenInfo)
			if identity != "" {
				byIdentity[identity] = append(byIdentity[identity], i)
			}
		}
	}
	for accountID, indexes := range byAccount {
		if len(indexes) < 2 {
			continue
		}
		for _, index := range indexes {
			items[index].action = "conflict"
			items[index].err = fmt.Sprintf("multiple uploaded SSO tokens target account %d", accountID)
		}
	}
	for identity, indexes := range byIdentity {
		if len(indexes) < 2 {
			continue
		}
		for _, index := range indexes {
			items[index].action = "conflict"
			items[index].err = "multiple uploaded SSO tokens resolve to the same new identity: " + identity
		}
	}
}

func grokSSOReauthPreview(item grokSSOReauthPrepared) GrokSSOReauthPreviewItem {
	preview := GrokSSOReauthPreviewItem{
		Index:   item.index,
		Email:   grokSSOReauthEmail(item),
		Action:  item.action,
		Error:   item.err,
		Subject: "",
	}
	if item.tokenInfo != nil {
		preview.Subject = item.tokenInfo.Subject
	}
	if len(item.matches) == 1 {
		preview.AccountID = item.matches[0].ID
		preview.AccountName = item.matches[0].Name
	}
	return preview
}

func (h *GrokOAuthHandler) applyPreparedGrokSSOReauth(
	ctx context.Context,
	req GrokSSOReauthRequest,
	item grokSSOReauthPrepared,
) GrokSSOToOAuthItemResult {
	account := item.matches[0]
	credentials := grokSSOImportCredentials(h.grokOAuthService.BuildAccountCredentials(item.tokenInfo), req.Credentials)
	if normalizedSSO := xai.NormalizeSSOToken(item.token); normalizedSSO != "" {
		credentials["sso_token"] = normalizedSSO
	}
	merged := service.MergeCredentials(account.Credentials, credentials)
	updated, err := h.adminService.UpdateAccount(ctx, account.ID, &service.UpdateAccountInput{Credentials: merged})
	if err != nil {
		return GrokSSOToOAuthItemResult{Index: item.index, Name: account.Name, Email: grokSSOReauthEmail(item), Error: grokSSOReauthErrorMessage(err)}
	}
	if cleared, clearErr := h.adminService.ClearAccountError(ctx, account.ID); clearErr == nil && cleared != nil {
		updated = cleared
	}
	return GrokSSOToOAuthItemResult{
		Index:   item.index,
		Name:    updated.Name,
		Email:   grokSSOReauthEmail(item),
		Account: dto.AccountFromService(updated),
	}
}

func grokSSOReauthCreateRequest(req GrokSSOReauthRequest) GrokSSOToOAuthRequest {
	return GrokSSOToOAuthRequest{
		Name:               req.Name,
		Notes:              req.Notes,
		ProxyID:            req.ProxyID,
		GroupIDs:           append([]int64(nil), req.GroupIDs...),
		Credentials:        cloneGrokSSOMap(req.Credentials),
		Extra:              cloneGrokSSOMap(req.Extra),
		Concurrency:        req.Concurrency,
		LoadFactor:         req.LoadFactor,
		Priority:           req.Priority,
		RateMultiplier:     req.RateMultiplier,
		ExpiresAt:          req.ExpiresAt,
		AutoPauseOnExpired: req.AutoPauseOnExpired,
	}
}

func grokSSOReauthEmail(item grokSSOReauthPrepared) string {
	if item.tokenInfo != nil {
		if email := strings.TrimSpace(item.tokenInfo.Email); email != "" {
			return email
		}
	}
	return strings.TrimSpace(item.emailHint)
}

func grokSSOReauthIdentity(tokenInfo *service.GrokTokenInfo) string {
	if tokenInfo == nil {
		return ""
	}
	if email := strings.ToLower(strings.TrimSpace(tokenInfo.Email)); email != "" {
		return "email:" + email
	}
	if sub := strings.TrimSpace(tokenInfo.Subject); sub != "" {
		return "sub:" + sub
	}
	return ""
}

func grokSSOReauthErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if message := strings.TrimSpace(grokSSOImportErrorMessage(err)); message != "" {
		return message
	}
	return strings.TrimSpace(err.Error())
}

func matchGrokAccountsForSSO(
	tokenInfo *service.GrokTokenInfo,
	byEmail, bySub map[string][]*service.Account,
) []*service.Account {
	if tokenInfo == nil {
		return nil
	}
	seen := make(map[int64]struct{})
	out := make([]*service.Account, 0, 1)
	appendUnique := func(items []*service.Account) {
		for _, account := range items {
			if account == nil {
				continue
			}
			if _, ok := seen[account.ID]; ok {
				continue
			}
			seen[account.ID] = struct{}{}
			out = append(out, account)
		}
	}
	if email := strings.ToLower(strings.TrimSpace(tokenInfo.Email)); email != "" {
		appendUnique(byEmail[email])
	}
	if len(out) == 0 {
		if sub := strings.TrimSpace(tokenInfo.Subject); sub != "" {
			appendUnique(bySub[sub])
		}
	}
	return out
}
