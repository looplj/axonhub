// 职责: 查询 Claude OAuth usage 和重置权益，并执行经重新验证的手动重置。
// 关联文件:
//
//	internal/server/biz/provider_quota/claudecode_checker.go
//	internal/server/biz/provider_quota/types.go
//	internal/server/biz/provider_quota.go
//
// 注意事项:
//
//	usage 的 utilization 是百分数，兼容窗口数据使用 0–1 比例。
//	未明确允许使用的权益不能兑换；结果不明时复用原 request_id，不自动重试。
//	自定义 BaseURL 继续走响应头探测，不把对方凭证发给 api.anthropic.com。
//	待确认权益已从 usage 消失时先重放原 request_id。重放本身兑换成功就停止，避免同一次操作再花另一张权益。
//	usage 里该权益剩余次数已为 0 时直接结束待确认状态，不再主动补发请求。

package provider_quota

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/anthropic/claudecode"
)

// OAuth reset eligibility checks the CLI version; share the provider version.
const claudeOAuthUserAgent = claudecode.UserAgent

const (
	claudeUsagePath    = "/api/oauth/usage?cedar_ember=1&skip_spend=1"
	claudeOAuthAPIRoot = "https://api.anthropic.com"
)

// claudeOfficialUsageAPI reports whether this channel should use Anthropic's
// OAuth usage API. A custom base URL keeps the header probe: its credentials
// belong to that upstream, and a 401 from api.anthropic.com would wipe the
// last saved quota.
func claudeOfficialUsageAPI(baseURL string) bool {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return true
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.User != nil || !strings.EqualFold(parsed.Scheme, "https") || !strings.EqualFold(parsed.Hostname(), "api.anthropic.com") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	path := strings.TrimSuffix(parsed.EscapedPath(), "/")
	return path == "" || strings.EqualFold(path, "/v1")
}

var (
	claudeGrantID                  = regexp.MustCompile(`^[a-z0-9_-]{1,40}$`)
	claudeReasonCode               = regexp.MustCompile(`^[a-z0-9_]{1,40}$`)
	_                     Resetter = (*ClaudeCodeQuotaChecker)(nil)
	errClaudeProfileScope          = errors.New("Claude OAuth user:profile scope required")
)

type claudeUsageWindow struct {
	Utilization *float64   `json:"utilization"`
	ResetsAt    *time.Time `json:"resets_at"`
}
type claudeUsageResponse struct {
	FiveHour   *claudeUsageWindow `json:"five_hour"`
	SevenDay   *claudeUsageWindow `json:"seven_day"`
	CedarEmber json.RawMessage    `json:"cedar_ember"`
	// Limits is decoded leniently: a malformed entry must not hide the quota windows.
	Limits json.RawMessage `json:"limits"`
}
type claudeUsageLimit struct {
	Kind     string     `json:"kind"`
	Percent  *float64   `json:"percent"`
	ResetsAt *time.Time `json:"resets_at"`
	Scope    *struct {
		Model *struct {
			DisplayName string `json:"display_name"`
		} `json:"model"`
	} `json:"scope"`
}

// claudeModelWindow is a weekly limit scoped to one model, such as a newly launched model.
type claudeModelWindow struct {
	Model    string
	Percent  float64
	ResetsAt *time.Time
}
type claudeResetGrant struct {
	ID               string     `json:"id"`
	Label            string     `json:"label"`
	ResetsLeft       int        `json:"resets_left"`
	StartsAt         *time.Time `json:"starts_at"`
	EndsAt           *time.Time `json:"ends_at"`
	Clears           []string   `json:"clears"`
	Paused           bool       `json:"paused"`
	UsableNow        bool       `json:"usable_now"`
	UseRequiresLimit *bool      `json:"use_requires_limit"`
	Blocking         []string   `json:"blocking"`
	// UpstreamReason is upstream's own unavailable_reason; only short codes are kept.
	UpstreamReason *string         `json:"unavailable_reason"`
	PercentUsed    json.RawMessage `json:"percent_used"`
}
type claudeResetBlock struct {
	IneligibleReason string             `json:"ineligible_reason"`
	Eligible         bool               `json:"eligible"`
	AtLimit          bool               `json:"at_limit"`
	Grants           []claudeResetGrant `json:"grants"`
	NextGrantID      string             `json:"next_grant_id"`
	CooldownUntil    *time.Time         `json:"cooldown_until"`
}
type claudePendingReset struct{ Organization, GrantID, RequestID string }

func (c *ClaudeCodeQuotaChecker) oauthRequest(ctx context.Context, ch *ent.Channel, method, path string, body any) (*httpclient.Response, error) {
	creds, err := ch.Credentials.ResolveOAuthCredentials()
	if err != nil || creds == nil || creds.AccessToken == "" {
		return nil, fmt.Errorf("Claude OAuth credentials required")
	}
	// Imported credentials may omit scope metadata. Let upstream authorize those.
	if len(creds.Scopes) > 0 && !slices.Contains(creds.Scopes, "user:profile") {
		return nil, errClaudeProfileScope
	}
	req := httpclient.NewRequestBuilder().WithMethod(method).
		WithURL(claudeOAuthAPIRoot+path).WithBearerToken(creds.AccessToken).
		WithHeader("anthropic-beta", "oauth-2025-04-20").WithHeader("x-app", "cli").
		WithHeader("User-Agent", claudeOAuthUserAgent).WithHeader("Accept", "application/json").WithHeader("Content-Type", "application/json")
	if body != nil {
		req = req.WithBody(body)
	}
	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = hc.WithProxy(ch.Settings.Proxy)
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	resp, err := hc.WithRejectRedirects().Do(ctx, req.Build())
	if err != nil {
		if upstream, ok := errors.AsType[*httpclient.Error](err); ok {
			if upstream.StatusCode == http.StatusUnauthorized {
				return nil, errors.Join(ErrInvalidCredentials, &httpclient.Error{StatusCode: upstream.StatusCode})
			}
			return nil, &httpclient.Error{StatusCode: upstream.StatusCode}
		}
		return nil, fmt.Errorf("Claude OAuth request failed")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &httpclient.Error{StatusCode: resp.StatusCode}
	}
	return resp, nil
}

func (c *ClaudeCodeQuotaChecker) fetchUsage(ctx context.Context, ch *ent.Channel) (*claudeUsageResponse, error) {
	resp, err := c.oauthRequest(ctx, ch, http.MethodGet, claudeUsagePath, nil)
	if err != nil {
		return nil, err
	}
	var usage claudeUsageResponse
	if err := json.Unmarshal(resp.Body, &usage); err != nil {
		return nil, fmt.Errorf("invalid Claude usage response")
	}
	usage.logResetDiagnostics(ctx, ch.ID)
	return &usage, nil
}

func (c *ClaudeCodeQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	if !claudeOfficialUsageAPI(ch.BaseURL) {
		data, err := c.checkQuotaFromHeaders(ctx, ch)
		if err != nil {
			return data, err
		}
		data.Resets = &ResetList{Supported: true, Resets: []Reset{}, Error: "Claude reset status unavailable: custom upstream"}
		return data, nil
	}
	usage, err := c.fetchUsage(ctx, ch)
	if err != nil {
		upstream, isUpstream := errors.AsType[*httpclient.Error](err)
		fallback := errors.Is(err, errClaudeProfileScope) || (isUpstream && (upstream.StatusCode == http.StatusForbidden || upstream.StatusCode == http.StatusNotFound))
		if !fallback {
			return QuotaData{}, err
		}
		log.Warn(ctx, "Claude usage API falling back to response headers", log.Int("channel_id", ch.ID), log.Cause(err))
		data, probeErr := c.checkQuotaFromHeaders(ctx, ch)
		data.Resets = &ResetList{Supported: true, Resets: []Reset{}, Error: "Claude reset status unavailable: " + err.Error()}
		return data, probeErr
	}
	c.resolveSpentPending(ch.ID, usage)
	headers := http.Header{}
	headers.Set("Anthropic-Ratelimit-Unified-Status", "allowed")
	count := 0
	for _, item := range []struct {
		key, claim string
		window     *claudeUsageWindow
	}{{"5h", "five_hour", usage.FiveHour}, {"7d", "seven_day", usage.SevenDay}} {
		w := item.window
		if w == nil || w.Utilization == nil {
			continue
		}
		if *w.Utilization < 0 || *w.Utilization > 100 {
			return QuotaData{}, fmt.Errorf("invalid Claude usage percentage")
		}
		count++
		prefix := "Anthropic-Ratelimit-Unified-" + item.key
		headers.Set(prefix+"-Utilization", strconv.FormatFloat(*w.Utilization/100, 'f', -1, 64))
		headers.Set(prefix+"-Status", "allowed")
		if w.ResetsAt != nil {
			headers.Set(prefix+"-Reset", strconv.FormatInt(w.ResetsAt.Unix(), 10))
		}
		if *w.Utilization >= 100 {
			headers.Set(prefix+"-Status", "rejected")
			headers.Set("Anthropic-Ratelimit-Unified-Status", "rejected")
			headers.Set("Anthropic-Ratelimit-Unified-Representative-Claim", item.claim)
		}
	}
	if count == 0 {
		return QuotaData{}, fmt.Errorf("Claude usage response has no quota windows")
	}
	data, err := c.parseResponse(headers)
	if err != nil {
		return QuotaData{}, err
	}
	// Do not invent zero-usage windows when upstream omits them.
	windows, ok := data.RawData["windows"].(map[string]any)
	if !ok {
		return QuotaData{}, fmt.Errorf("Claude usage response has no quota windows")
	}
	for _, key := range []string{"5h", "7d", "overage"} {
		if headers.Get("Anthropic-Ratelimit-Unified-"+key+"-Utilization") == "" {
			delete(windows, key)
		}
	}
	data.RawData["source"] = "oauth_usage"
	if models := usage.modelWindows(); len(models) > 0 {
		rows := make([]map[string]any, 0, len(models))
		for _, m := range models {
			row := map[string]any{"model": m.Model, "utilization": m.Percent / 100}
			if m.ResetsAt != nil {
				row["reset"] = m.ResetsAt.Unix()
			}
			rows = append(rows, row)
		}
		data.RawData["model_windows"] = rows
	}
	resets, resetErr := usage.resetList(time.Now())
	if resetErr != nil {
		resets.Error = resetErr.Error()
	}
	data.Resets = &resets
	return data, err
}

func (u *claudeUsageResponse) resetBlock() (*claudeResetBlock, error) {
	if len(u.CedarEmber) == 0 || string(u.CedarEmber) == "null" {
		return nil, nil
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(u.CedarEmber, &fields) != nil {
		return nil, fmt.Errorf("invalid Claude reset status")
	}
	var eligible *bool
	if json.Unmarshal(fields["eligible"], &eligible) != nil || eligible == nil {
		return nil, fmt.Errorf("Claude reset eligibility missing or invalid")
	}
	var b claudeResetBlock
	if json.Unmarshal(u.CedarEmber, &b) != nil || b.Grants == nil {
		return nil, fmt.Errorf("invalid Claude reset status")
	}
	return &b, nil
}

func (u *claudeUsageResponse) modelWindows() []claudeModelWindow {
	var limits []json.RawMessage
	if len(u.Limits) == 0 || json.Unmarshal(u.Limits, &limits) != nil {
		return nil
	}
	var out []claudeModelWindow
	for _, raw := range limits {
		var l claudeUsageLimit
		if json.Unmarshal(raw, &l) != nil || l.Kind != "weekly_scoped" || l.Percent == nil || *l.Percent < 0 || *l.Percent > 100 ||
			l.Scope == nil || l.Scope.Model == nil {
			continue
		}
		name := strings.TrimSpace(l.Scope.Model.DisplayName)
		if name == "" || utf8.RuneCountInString(name) > 40 {
			continue
		}
		out = append(out, claudeModelWindow{Model: name, Percent: *l.Percent, ResetsAt: l.ResetsAt})
	}
	return out
}

// windowModel names the model behind a grant window. Upstream does not link grant windows
// to limits, so a window is matched only when exactly one model-scoped weekly limit
// reports the same usage the grant reports for that window.
func (g claudeResetGrant) windowModel(window string, models []claudeModelWindow) string {
	if window == "five_hour" || window == "seven_day" || len(models) == 0 {
		return ""
	}
	var used map[string]*float64
	if json.Unmarshal(g.PercentUsed, &used) != nil || used[window] == nil {
		return ""
	}
	match := ""
	for _, m := range models {
		if math.Round(m.Percent) != math.Round(*used[window]) {
			continue
		}
		if match != "" && match != m.Model {
			return ""
		}
		match = m.Model
	}
	return match
}

func (g claudeResetGrant) upstreamReason() string {
	if g.UpstreamReason == nil || *g.UpstreamReason == "" {
		return ""
	}
	if !claudeReasonCode.MatchString(*g.UpstreamReason) {
		return "unknown"
	}
	return *g.UpstreamReason
}

func (g claudeResetGrant) held(now time.Time) bool {
	return claudeGrantID.MatchString(g.ID) && len(g.Clears) > 0 && g.ResetsLeft > 0 && !g.Paused && (g.StartsAt == nil || !now.Before(*g.StartsAt)) && (g.EndsAt == nil || now.Before(*g.EndsAt))
}

// unavailableReason reports the first gate that stops a held grant from being redeemed.
// requires_limit is checked before usable_now because upstream also clears usable_now
// while the account is under its limits, and the limit is the actionable explanation.
func (b *claudeResetBlock) unavailableReason(g claudeResetGrant, now time.Time) string {
	switch {
	case !b.Eligible:
		return "ineligible"
	case b.CooldownUntil != nil && now.Before(*b.CooldownUntil):
		return "cooldown"
	case (g.UseRequiresLimit == nil || *g.UseRequiresLimit) && !b.AtLimit:
		return "requires_limit"
	case len(g.Blocking) > 0:
		return "blocked"
	case g.upstreamReason() != "":
		return "upstream"
	case !g.UsableNow:
		return "not_usable"
	}
	return ""
}

func (b *claudeResetBlock) usable(g claudeResetGrant, now time.Time) bool {
	return g.held(now) && b.unavailableReason(g, now) == ""
}

func (u *claudeUsageResponse) resetList(now time.Time) (ResetList, error) {
	list := ResetList{Supported: true, Resets: []Reset{}}
	b, err := u.resetBlock()
	if err != nil {
		return list, err
	}
	if b == nil {
		return list, fmt.Errorf("Claude reset status unavailable")
	}
	list.Eligible = lo.ToPtr(b.Eligible)
	list.IneligibleReason = claudeResetIneligibleReason(b.IneligibleReason)
	list.CooldownUntil = b.CooldownUntil
	models := u.modelWindows()
	for _, g := range b.Grants {
		if !g.held(now) {
			continue
		}
		reason := b.unavailableReason(g, now)
		list.Resets = append(list.Resets, Reset{Clears: g.Clears, Recommended: g.ID == b.NextGrantID, ID: g.ID, Title: g.Label, Status: "held", Remaining: lo.ToPtr(g.ResetsLeft), Redeemable: lo.ToPtr(reason == ""), UnavailableReason: reason, UpstreamReason: g.upstreamReason(), GrantedAt: g.StartsAt, ExpiresAt: g.EndsAt})
		for _, window := range g.Clears {
			model := g.windowModel(window, models)
			if model == "" {
				continue
			}
			if list.WindowModels == nil {
				list.WindowModels = map[string]string{}
			}
			if existing, ok := list.WindowModels[window]; ok && existing != model {
				list.WindowModels[window] = ""
				continue
			}
			list.WindowModels[window] = model
		}
	}
	for window, model := range list.WindowModels {
		if model == "" {
			delete(list.WindowModels, window)
		}
	}
	return list, nil
}

func (c *ClaudeCodeQuotaChecker) ListResets(ctx context.Context, ch *ent.Channel) (ResetList, error) {
	if !claudeOfficialUsageAPI(ch.BaseURL) {
		return ResetList{Supported: true, Resets: []Reset{}, Error: "Claude reset status unavailable: custom upstream"}, nil
	}
	u, err := c.fetchUsage(ctx, ch)
	if err != nil {
		return ResetList{Supported: true, Resets: []Reset{}}, err
	}
	return u.resetList(time.Now())
}

func (c *ClaudeCodeQuotaChecker) Reset(ctx context.Context, ch *ent.Channel) error {
	return c.ResetSelected(ctx, ch, "")
}

func (c *ClaudeCodeQuotaChecker) ResetSelected(ctx context.Context, ch *ent.Channel, resetID string) error {
	if !claudeOfficialUsageAPI(ch.BaseURL) {
		return fmt.Errorf("Claude reset credits require the official Anthropic API")
	}
	if resetID != "" && !claudeGrantID.MatchString(resetID) {
		return fmt.Errorf("invalid Claude reset ID")
	}
	if !c.resetMu.TryLock() {
		return fmt.Errorf("a Claude reset is already in progress")
	}
	defer c.resetMu.Unlock()
	if c.pending == nil {
		c.pending = make(map[string]claudePendingReset)
	}
	organization, err := c.claudeOrganization(ctx, ch)
	if err != nil {
		return err
	}
	accountKey := organization.String()
	pending, retry := c.pending[accountKey]
	if retry && resetID != "" && pending.GrantID != resetID {
		held, heldErr := c.claudeGrantHeld(ctx, ch, pending.GrantID)
		if heldErr != nil {
			return heldErr
		}
		if held {
			return fmt.Errorf("another Claude reset has an unconfirmed outcome; resolve it before selecting a different reset")
		}
		// The saved grant was spent or dropped, so its button is gone. Replaying
		// the saved request confirms that outcome without spending another credit.
		outcome, resetErr := c.finishClaudeReset(ctx, ch, accountKey, pending)
		switch outcome {
		case "":
			return resetErr
		case "reset":
			return nil
		case "already_used":
			// The earlier request already consumed that credit. Continue with the grant selected now.
		default:
			return resetErr
		}
		retry = false
	}
	if !retry {
		chosen, err := c.selectClaudeGrant(ctx, ch, resetID)
		if err != nil {
			return err
		}
		// Equivalent requests from channels sharing an organization use the same
		// upstream idempotency key until the selected grant count changes.
		pending = claudePendingReset{Organization: accountKey, GrantID: chosen.ID, RequestID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("claude-reset:%s:%s:%d:%v:%v", accountKey, chosen.ID, chosen.ResetsLeft, chosen.StartsAt, chosen.EndsAt))).String()}
		c.rememberPending(ch.ID, accountKey, pending)
	}
	_, err = c.finishClaudeReset(ctx, ch, accountKey, pending)
	return err
}

func (c *ClaudeCodeQuotaChecker) claudeOrganization(ctx context.Context, ch *ent.Channel) (uuid.UUID, error) {
	resp, err := c.oauthRequest(ctx, ch, http.MethodGet, "/api/oauth/profile", nil)
	if err != nil {
		return uuid.UUID{}, err
	}
	var profile struct {
		Organization struct {
			UUID string `json:"uuid"`
		} `json:"organization"`
	}
	if json.Unmarshal(resp.Body, &profile) != nil {
		return uuid.UUID{}, fmt.Errorf("invalid Claude profile")
	}
	organization, err := uuid.Parse(profile.Organization.UUID)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("invalid Claude organization")
	}
	return organization, nil
}

func (c *ClaudeCodeQuotaChecker) claudeGrantHeld(ctx context.Context, ch *ent.Channel, grantID string) (bool, error) {
	u, err := c.fetchUsage(ctx, ch)
	if err != nil {
		return false, err
	}
	b, err := u.resetBlock()
	if err != nil {
		return false, err
	}
	if b == nil {
		return false, nil
	}
	now := time.Now()
	for _, g := range b.Grants {
		if g.ID == grantID && g.held(now) {
			return true, nil
		}
	}
	return false, nil
}

func (c *ClaudeCodeQuotaChecker) selectClaudeGrant(ctx context.Context, ch *ent.Channel, resetID string) (claudeResetGrant, error) {
	u, err := c.fetchUsage(ctx, ch)
	if err != nil {
		return claudeResetGrant{}, err
	}
	b, err := u.resetBlock()
	if err != nil {
		return claudeResetGrant{}, err
	}
	if b == nil {
		return claudeResetGrant{}, fmt.Errorf("Claude reset unavailable")
	}
	var chosen *claudeResetGrant
	now := time.Now()
	for _, g := range b.Grants {
		if !b.usable(g, now) || (resetID != "" && g.ID != resetID) {
			continue
		}
		if chosen == nil || g.ID == b.NextGrantID {
			chosen = &g
		}
		if resetID != "" || g.ID == b.NextGrantID {
			break
		}
	}
	if chosen == nil {
		return claudeResetGrant{}, fmt.Errorf("requested Claude reset is unavailable or cannot be used now")
	}
	return *chosen, nil
}

// finishClaudeReset posts the saved request. An empty outcome means the result
// is still unknown and the pending request must be reused. "reset" and
// "already_used" clear it successfully. A refusal also clears it and returns
// the refusal error.
func (c *ClaudeCodeQuotaChecker) finishClaudeReset(ctx context.Context, ch *ent.Channel, accountKey string, pending claudePendingReset) (string, error) {
	resp, err := c.oauthRequest(ctx, ch, http.MethodPost, "/api/organizations/"+pending.Organization+"/reset_rate_limits", map[string]string{"program": "cedar_ember", "grant_id": pending.GrantID, "request_id": pending.RequestID})
	if err != nil {
		return "", fmt.Errorf("Claude reset outcome unconfirmed; refresh usage before retrying (retry reuses the same request): %w", err)
	}
	var result struct {
		Result string `json:"result"`
	}
	if json.Unmarshal(resp.Body, &result) != nil {
		return "", fmt.Errorf("Claude reset outcome unconfirmed")
	}
	switch result.Result {
	case "reset", "already_used":
		c.clearPending(accountKey)
		return result.Result, nil
	case "not_limited", "cooldown", "ineligible", "unavailable":
		c.clearPending(accountKey)
		return "refused", fmt.Errorf("Claude reset refused: %s", result.Result)
	default:
		return "", fmt.Errorf("Claude reset outcome unconfirmed")
	}
}

func (c *ClaudeCodeQuotaChecker) rememberPending(channelID int, accountKey string, pending claudePendingReset) {
	if c.pending == nil {
		c.pending = map[string]claudePendingReset{}
	}
	if c.pendingByChannel == nil {
		c.pendingByChannel = map[int]string{}
	}
	c.pending[accountKey] = pending
	c.pendingByChannel[channelID] = accountKey
}

func (c *ClaudeCodeQuotaChecker) clearPending(accountKey string) {
	delete(c.pending, accountKey)
	for channelID, key := range c.pendingByChannel {
		if key == accountKey {
			delete(c.pendingByChannel, channelID)
		}
	}
}

// resolveSpentPending drops an unconfirmed reset once fresh usage shows that
// grant has no uses left. The credit is already consumed, so a quota refresh
// must not keep blocking every other grant. Absence alone is not enough:
// a partial payload must not forget a request that may still be applied.
func (c *ClaudeCodeQuotaChecker) resolveSpentPending(channelID int, usage *claudeUsageResponse) {
	if !c.resetMu.TryLock() {
		return
	}
	defer c.resetMu.Unlock()
	accountKey, ok := c.pendingByChannel[channelID]
	if !ok {
		return
	}
	pending, ok := c.pending[accountKey]
	if !ok {
		delete(c.pendingByChannel, channelID)
		return
	}
	block, err := usage.resetBlock()
	if err != nil || block == nil {
		return
	}
	for _, grant := range block.Grants {
		if grant.ID != pending.GrantID {
			continue
		}
		if grant.ResetsLeft <= 0 {
			c.clearPending(accountKey)
		}
		return
	}
}

// logResetDiagnostics records only quota metadata; credentials, organization IDs,
// grant IDs and arbitrary upstream text must not enter application logs.
func (u *claudeUsageResponse) logResetDiagnostics(ctx context.Context, channelID int) {
	b, err := u.resetBlock()
	if err != nil || b == nil {
		log.Warn(ctx, "Claude reset status unavailable", log.Int("channel_id", channelID), log.Bool("reset_block_present", len(u.CedarEmber) > 0 && string(u.CedarEmber) != "null"), log.Cause(err))
		return
	}
	now := time.Now()
	reported, displayed := 0, 0
	for i, g := range b.Grants {
		reported += max(0, g.ResetsLeft)
		held := g.held(now)
		if held {
			displayed += g.ResetsLeft
		}
		log.Info(ctx, "Claude reset grant evaluated", log.Int("channel_id", channelID), log.Int("grant_index", i), log.Int("resets_left", g.ResetsLeft), log.Bool("included_in_remaining", held), log.Bool("valid_id", claudeGrantID.MatchString(g.ID)), log.Bool("has_windows", len(g.Clears) > 0), log.Bool("paused", g.Paused), log.Bool("not_started", g.StartsAt != nil && now.Before(*g.StartsAt)), log.Bool("expired", g.EndsAt != nil && !now.Before(*g.EndsAt)), log.Bool("usable_now", g.UsableNow), log.Bool("requires_limit", g.UseRequiresLimit == nil || *g.UseRequiresLimit), log.Bool("is_next_grant", g.ID == b.NextGrantID), log.Int("blocking_count", len(g.Blocking)), log.Bool("redeemable", b.usable(g, now)))
	}
	reason := claudeResetIneligibleReason(b.IneligibleReason)
	log.Info(ctx, "Claude reset status fetched", log.Int("channel_id", channelID), log.Bool("eligible", b.Eligible), log.String("ineligible_reason", reason), log.Bool("at_limit", b.AtLimit), log.Bool("cooling_down", b.CooldownUntil != nil && now.Before(*b.CooldownUntil)), log.Int("grant_count", len(b.Grants)), log.Int("reported_remaining", reported), log.Int("displayed_remaining", displayed), log.String("client_user_agent", claudeOAuthUserAgent))
}

func claudeResetIneligibleReason(reason string) string {
	switch reason {
	case "", "config_off", "tier", "seat", "mobile", "surface", "cli_version", "no_grant", "tenure", "other_experiment", "unavailable", "unknown":
	default:
		reason = "unknown"
	}
	return reason
}
