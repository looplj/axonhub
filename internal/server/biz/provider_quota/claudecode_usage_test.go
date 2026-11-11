// 职责: 验证 Claude usage 查询、权限回退以及重置资格与请求重放。
// 关联文件:
//
//	internal/server/biz/provider_quota/claudecode_usage.go
//	internal/server/biz/provider_quota/claudecode_checker.go
//
// 注意事项: 使用模拟 HTTP 上游，不调用真实账号或消耗重置权益。

package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

func claudeTestChannel() *ent.Channel {
	return &ent.Channel{ID: 1, Credentials: objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{AccessToken: "test-token", Scopes: []string{"user:profile"}}}}
}

func claudeTestResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func claudeTestChecker(t *testing.T, f func(*http.Request) (*http.Response, error)) *ClaudeCodeQuotaChecker {
	t.Helper()
	return NewClaudeCodeQuotaChecker(httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripFunc(f)}))
}

const claudeTestGrant = `{"eligible":true,"at_limit":true,"next_grant_id":"credit_1","grants":[{"id":"credit_1","label":"Reset","resets_left":2,"clears":["five_hour"],"usable_now":true}]}`

func TestClaudeUsageAndResetCredits(t *testing.T) {
	calls := 0
	c := claudeTestChecker(t, func(r *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "GET", r.Method)
		require.Equal(t, "/api/oauth/usage", r.URL.Path)
		require.Equal(t, "1", r.URL.Query().Get("cedar_ember"))
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		require.Equal(t, "oauth-2025-04-20", r.Header.Get("Anthropic-Beta"))
		return claudeTestResponse(`{"five_hour":{"utilization":100,"resets_at":"2099-01-01T00:00:00Z"},"seven_day":{"utilization":25},"cedar_ember":` + claudeTestGrant + `}`), nil
	})
	q, err := c.CheckQuota(context.Background(), claudeTestChannel())
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.Equal(t, "exhausted", q.Status)
	require.False(t, q.Ready)
	require.Equal(t, 0.25, q.RawData["windows"].(map[string]any)["7d"].(map[string]any)["utilization"])
	require.NotContains(t, q.RawData["windows"], "overage")
	require.Len(t, q.Resets.Resets, 1)
	require.Equal(t, 2, *q.Resets.Resets[0].Remaining)
	require.True(t, *q.Resets.Resets[0].Redeemable)
}

func TestClaudeUsageResetFailureDoesNotLoseQuota(t *testing.T) {
	for _, block := range []string{`null`, `{"grants":"broken"}`, `{"grants":[]}`, `{"eligible":null,"grants":[]}`, `{"eligible":"false","grants":[]}`} {
		t.Run(block, func(t *testing.T) {
			c := claudeTestChecker(t, func(r *http.Request) (*http.Response, error) {
				return claudeTestResponse(`{"five_hour":{"utilization":0},"cedar_ember":` + block + `}`), nil
			})
			q, err := c.CheckQuota(context.Background(), claudeTestChannel())
			require.NoError(t, err)
			require.Equal(t, "available", q.Status)
			require.NotEmpty(t, q.Resets.Error)
			require.NotContains(t, q.RawData["windows"], "7d")
		})
	}
}

func TestClaudeUsageErrorsDoNotProbe(t *testing.T) {
	for _, status := range []int{401, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			c := claudeTestChecker(t, func(r *http.Request) (*http.Response, error) {
				calls++
				require.Equal(t, "GET", r.Method)
				resp := claudeTestResponse(`{}`)
				resp.StatusCode = status
				return resp, nil
			})
			_, err := c.CheckQuota(context.Background(), claudeTestChannel())
			require.Error(t, err)
			require.Equal(t, 1, calls)
		})
	}
}

func TestClaudeUsageScopeFallback(t *testing.T) {
	c := claudeTestChecker(t, func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "/v1/messages", r.URL.Path)
		resp := claudeTestResponse(`{}`)
		resp.StatusCode = 429
		resp.Header.Set("Anthropic-Ratelimit-Unified-Status", "rejected")
		resp.Header.Set("Anthropic-Ratelimit-Unified-5h-Utilization", "1")
		return resp, nil
	})
	ch := claudeTestChannel()
	ch.Credentials.OAuth.Scopes = []string{"user:inference"}
	q, err := c.CheckQuota(context.Background(), ch)
	require.NoError(t, err)
	require.Equal(t, "exhausted", q.Status)
	require.NotEmpty(t, q.Resets.Error)
}

func TestClaudeResetGates(t *testing.T) {
	now := time.Now()
	for _, test := range []struct {
		name   string
		change func(*claudeResetBlock)
	}{
		{"ineligible", func(b *claudeResetBlock) { b.Eligible = false }},
		{"not limited", func(b *claudeResetBlock) { b.AtLimit = false }},
		{"paused", func(b *claudeResetBlock) { b.Grants[0].Paused = true }},
		{"exhausted", func(b *claudeResetBlock) { b.Grants[0].ResetsLeft = 0 }},
		{"not usable", func(b *claudeResetBlock) { b.Grants[0].UsableNow = false }},
		{"blocked", func(b *claudeResetBlock) { b.Grants[0].Blocking = []string{"weekly"} }},
		{"expired", func(b *claudeResetBlock) { v := now.Add(-time.Minute); b.Grants[0].EndsAt = &v }},
		{"cooldown", func(b *claudeResetBlock) { v := now.Add(time.Hour); b.CooldownUntil = &v }},
	} {
		t.Run(test.name, func(t *testing.T) {
			var b claudeResetBlock
			require.NoError(t, json.Unmarshal([]byte(claudeTestGrant), &b))
			test.change(&b)
			require.False(t, b.usable(b.Grants[0], now))
			raw, err := json.Marshal(b)
			require.NoError(t, err)
			c := claudeTestChecker(t, func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/api/oauth/profile" {
					return claudeTestResponse(`{"organization":{"uuid":"12345678-1234-1234-1234-123456789012"}}`), nil
				}
				require.Equal(t, "/api/oauth/usage", r.URL.Path)
				return claudeTestResponse(`{"cedar_ember":` + string(raw) + `}`), nil
			})
			require.Error(t, c.Reset(context.Background(), claudeTestChannel()))
		})
	}
}

func TestClaudeResetUnknownOutcomeReusesRequest(t *testing.T) {
	var ids []string
	gets := 0
	c := claudeTestChecker(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/oauth/usage":
			gets++
			return claudeTestResponse(`{"cedar_ember":` + claudeTestGrant + `}`), nil
		case "/api/oauth/profile":
			return claudeTestResponse(`{"organization":{"uuid":"12345678-1234-1234-1234-123456789012"}}`), nil
		default:
			require.Equal(t, "/api/organizations/12345678-1234-1234-1234-123456789012/reset_rate_limits", r.URL.Path)
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "cedar_ember", body["program"])
			ids = append(ids, body["request_id"])
			if len(ids) == 1 {
				return nil, fmt.Errorf("connection lost")
			}
			return claudeTestResponse(`{"result":"already_used"}`), nil
		}
	})
	require.Error(t, c.Reset(context.Background(), claudeTestChannel()))
	require.NoError(t, c.Reset(context.Background(), claudeTestChannel()))
	require.Len(t, ids, 2)
	require.Equal(t, ids[0], ids[1])
	require.Equal(t, 1, gets)
}

func TestClaudeResetClientVersionReasonIsPreserved(t *testing.T) {
	c := claudeTestChecker(t, func(r *http.Request) (*http.Response, error) {
		return claudeTestResponse(`{"five_hour":{"utilization":95},"seven_day":{"utilization":40},"cedar_ember":{"eligible":false,"ineligible_reason":"cli_version","at_limit":false,"grants":[],"next_grant_id":null}}`), nil
	})
	quota, err := c.CheckQuota(context.Background(), claudeTestChannel())
	require.NoError(t, err)
	require.Equal(t, "warning", quota.Status)
	require.False(t, *quota.Resets.Eligible)
	require.Equal(t, "cli_version", quota.Resets.IneligibleReason)
	require.Empty(t, quota.Resets.Resets)
}

func TestClaudeResetSelectedGrant(t *testing.T) {
	for _, selected := range []string{"", "full", "missing", "blocked"} {
		t.Run(selected, func(t *testing.T) {
			posts := 0
			c := claudeTestChecker(t, func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case "/api/oauth/profile":
					return claudeTestResponse(`{"organization":{"uuid":"12345678-1234-1234-1234-123456789012"}}`), nil
				case "/api/oauth/usage":
					return claudeTestResponse(`{"cedar_ember":{"eligible":true,"at_limit":false,"next_grant_id":"fable","grants":[{"id":"full","resets_left":1,"clears":["five_hour","seven_day"],"usable_now":true,"use_requires_limit":false},{"id":"fable","resets_left":1,"clears":["five_hour","seven_day_overage_included"],"usable_now":true,"use_requires_limit":false},{"id":"blocked","resets_left":1,"clears":["five_hour"],"usable_now":false,"use_requires_limit":false}]}}`), nil
				default:
					posts++
					var body map[string]string
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					expected := selected
					if expected == "" {
						expected = "fable"
					}
					require.Equal(t, expected, body["grant_id"])
					return claudeTestResponse(`{"result":"reset"}`), nil
				}
			})
			err := c.ResetSelected(context.Background(), claudeTestChannel(), selected)
			if selected == "missing" || selected == "blocked" {
				require.Error(t, err)
				require.Zero(t, posts)
			} else {
				require.NoError(t, err)
				require.Equal(t, 1, posts)
			}
		})
	}
}

func TestClaudeResetCannotSwitchPendingGrant(t *testing.T) {
	posts := 0
	c := claudeTestChecker(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/oauth/profile":
			return claudeTestResponse(`{"organization":{"uuid":"12345678-1234-1234-1234-123456789012"}}`), nil
		case "/api/oauth/usage":
			return claudeTestResponse(`{"cedar_ember":` + claudeTestGrant + `}`), nil
		default:
			posts++
			return nil, fmt.Errorf("connection lost")
		}
	})
	require.Error(t, c.ResetSelected(context.Background(), claudeTestChannel(), "credit_1"))
	err := c.ResetSelected(context.Background(), claudeTestChannel(), "another")
	require.ErrorContains(t, err, "unconfirmed outcome")
	require.Equal(t, 1, posts)
}

func TestClaudeResetUnavailableReason(t *testing.T) {
	now := time.Now()
	for _, test := range []struct {
		name   string
		change func(*claudeResetBlock)
		reason string
	}{
		{"redeemable", func(b *claudeResetBlock) {}, ""},
		{"ineligible", func(b *claudeResetBlock) { b.Eligible = false }, "ineligible"},
		{"cooldown wins over limit", func(b *claudeResetBlock) {
			v := now.Add(time.Hour)
			b.CooldownUntil = &v
			b.AtLimit = false
		}, "cooldown"},
		{"requires limit wins over usable_now", func(b *claudeResetBlock) {
			b.AtLimit = false
			b.Grants[0].UsableNow = false
		}, "requires_limit"},
		{"limit not required", func(b *claudeResetBlock) {
			b.AtLimit = false
			b.Grants[0].UseRequiresLimit = new(bool)
		}, ""},
		{"blocked", func(b *claudeResetBlock) { b.Grants[0].Blocking = []string{"weekly"} }, "blocked"},
		{"not usable", func(b *claudeResetBlock) { b.Grants[0].UsableNow = false }, "not_usable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var b claudeResetBlock
			require.NoError(t, json.Unmarshal([]byte(claudeTestGrant), &b))
			test.change(&b)
			raw, err := json.Marshal(b)
			require.NoError(t, err)
			u := claudeUsageResponse{CedarEmber: raw}
			list, err := u.resetList(now)
			require.NoError(t, err)
			require.Len(t, list.Resets, 1)
			require.Equal(t, test.reason, list.Resets[0].UnavailableReason)
			require.Equal(t, test.reason == "", *list.Resets[0].Redeemable)
			require.Equal(t, test.reason == "", b.usable(b.Grants[0], now))
		})
	}
}

// claudeLiveUsage mirrors a real usage response: a launch grant that clears the
// five-hour window and a model-scoped weekly limit, plus a full reset grant.
func claudeLiveUsage(grantExtra, limits string) string {
	start := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	end := time.Now().Add(5 * 24 * time.Hour).UTC().Format(time.RFC3339)
	return `{"five_hour":{"utilization":95.0,"resets_at":"` + end + `"},"seven_day":{"utilization":40.0,"resets_at":"` + end + `"},` +
		`"cedar_ember":{"eligible":true,"ineligible_reason":null,"at_limit":false,"next_grant_id":"launch-2610-fable-week","cooldown_until":null,"grants":[` +
		`{"id":"launch-2610-fable-week","label":"New model launch","resets_left":1,"starts_at":"` + start + `","ends_at":"` + end + `","clears":["five_hour","seven_day_overage_included"],"paused":false,"usable_now":true,"use_requires_limit":false,"percent_used":{"five_hour":95,"seven_day_overage_included":78,"seven_day":40},"blocking":[],"type":"full","unavailable_reason":null` + grantExtra + `},` +
		`{"id":"full-reset-2611","label":"One full limit reset","resets_left":1,"starts_at":"` + start + `","ends_at":"` + end + `","clears":["five_hour","seven_day","seven_day_overage_included"],"paused":false,"usable_now":true,"use_requires_limit":false,"percent_used":{"five_hour":95,"seven_day":40,"seven_day_overage_included":78},"blocking":[],"type":"full","unavailable_reason":null}]},` +
		`"limits":` + limits + `}`
}

const claudeLiveLimits = `[{"kind":"session","percent":95,"scope":null},{"kind":"weekly_all","percent":40,"scope":null},` +
	`{"kind":"weekly_scoped","percent":78,"resets_at":"2099-01-01T00:00:00Z","scope":{"model":{"id":null,"display_name":"Fable"},"surface":null}}]`

func TestClaudeLiveUsageShapesModelWindowsAndGrants(t *testing.T) {
	c := claudeTestChecker(t, func(r *http.Request) (*http.Response, error) {
		return claudeTestResponse(claudeLiveUsage("", claudeLiveLimits)), nil
	})
	q, err := c.CheckQuota(context.Background(), claudeTestChannel())
	require.NoError(t, err)

	models, ok := q.RawData["model_windows"].([]map[string]any)
	require.True(t, ok)
	require.Equal(t, []map[string]any{{"model": "Fable", "utilization": 0.78, "reset": int64(4070908800)}}, models)

	require.Equal(t, map[string]string{"seven_day_overage_included": "Fable"}, q.Resets.WindowModels)
	require.Len(t, q.Resets.Resets, 2)
	for _, r := range q.Resets.Resets {
		require.True(t, *r.Redeemable, r.ID)
		require.Empty(t, r.UnavailableReason, r.ID)
		require.Empty(t, r.UpstreamReason, r.ID)
	}
	require.True(t, q.Resets.Resets[0].Recommended)
	require.Equal(t, []string{"five_hour", "seven_day", "seven_day_overage_included"}, q.Resets.Resets[1].Clears)
}

func TestClaudeUpstreamUnavailableReason(t *testing.T) {
	for _, test := range []struct{ raw, code string }{{`"not_at_limit"`, "not_at_limit"}, {`"Bad reason!"`, "unknown"}} {
		t.Run(test.code, func(t *testing.T) {
			u := claudeUsageResponse{}
			require.NoError(t, json.Unmarshal([]byte(claudeLiveUsage(`,"unavailable_reason":`+test.raw, claudeLiveLimits)), &u))
			list, err := u.resetList(time.Now())
			require.NoError(t, err)
			require.Equal(t, "upstream", list.Resets[0].UnavailableReason)
			require.Equal(t, test.code, list.Resets[0].UpstreamReason)
			require.False(t, *list.Resets[0].Redeemable)
			require.True(t, *list.Resets[1].Redeemable)
		})
	}
}

func TestClaudeOfficialUsageAPI(t *testing.T) {
	for _, baseURL := range []string{"", "https://api.anthropic.com", "https://api.anthropic.com/", "https://api.anthropic.com/v1", "https://API.anthropic.com/v1/", "https://api.anthropic.com:443/v1"} {
		require.True(t, claudeOfficialUsageAPI(baseURL), baseURL)
	}
	for _, baseURL := range []string{"https://gateway.example.com/v1", "http://api.anthropic.com", "https://api.anthropic.com/v1/messages", "https://api.anthropic.com.evil.com", "https://user:token@api.anthropic.com"} {
		require.False(t, claudeOfficialUsageAPI(baseURL), baseURL)
	}
}

func TestClaudeCustomUpstreamUsesHeaderProbe(t *testing.T) {
	calls := 0
	c := claudeTestChecker(t, func(r *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "https://gateway.example.com/v1/messages", r.URL.String())
		resp := claudeTestResponse(`{}`)
		resp.Header.Set("Anthropic-Ratelimit-Unified-Status", "allowed")
		resp.Header.Set("Anthropic-Ratelimit-Unified-5h-Status", "allowed")
		resp.Header.Set("Anthropic-Ratelimit-Unified-5h-Utilization", "0.5")
		return resp, nil
	})
	ch := claudeTestChannel()
	ch.BaseURL = "https://gateway.example.com/v1"
	quota, err := c.CheckQuota(context.Background(), ch)
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.NotContains(t, quota.RawData, "source")
	require.Contains(t, quota.Resets.Error, "custom upstream")
	resets, err := c.ListResets(context.Background(), ch)
	require.NoError(t, err)
	require.Contains(t, resets.Error, "custom upstream")
	require.ErrorContains(t, c.ResetSelected(context.Background(), ch, "credit_1"), "official Anthropic API")
	require.Equal(t, 1, calls)
}

func TestClaudeOfficialBaseURLUsesUsageAPI(t *testing.T) {
	for _, baseURL := range []string{"", "https://api.anthropic.com/v1"} {
		t.Run(baseURL, func(t *testing.T) {
			c := claudeTestChecker(t, func(r *http.Request) (*http.Response, error) {
				require.Equal(t, "api.anthropic.com", r.URL.Hostname())
				require.Equal(t, "/api/oauth/usage", r.URL.Path)
				return claudeTestResponse(`{"five_hour":{"utilization":0}}`), nil
			})
			ch := claudeTestChannel()
			ch.BaseURL = baseURL
			quota, err := c.CheckQuota(context.Background(), ch)
			require.NoError(t, err)
			require.Equal(t, "oauth_usage", quota.RawData["source"])
		})
	}
}

func TestClaudeLostResetReplaysSavedRequest(t *testing.T) {
	for _, replay := range []string{"already_used", "reset"} {
		t.Run(replay, func(t *testing.T) {
			usageCalls := 0
			var posts []string
			c := claudeTestChecker(t, func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case "/api/oauth/profile":
					return claudeTestResponse(`{"organization":{"uuid":"12345678-1234-1234-1234-123456789012"}}`), nil
				case "/api/oauth/usage":
					usageCalls++
					if usageCalls == 1 {
						return claudeTestResponse(`{"cedar_ember":{"eligible":true,"at_limit":true,"next_grant_id":"credit_1","grants":[{"id":"credit_1","resets_left":1,"clears":["five_hour"],"usable_now":true},{"id":"credit_2","resets_left":1,"clears":["seven_day"],"usable_now":true}]}}`), nil
					}
					return claudeTestResponse(`{"cedar_ember":{"eligible":true,"at_limit":true,"next_grant_id":"credit_2","grants":[{"id":"credit_2","resets_left":1,"clears":["seven_day"],"usable_now":true}]}}`), nil
				default:
					var body map[string]string
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					posts = append(posts, body["grant_id"]+" "+body["request_id"])
					if body["grant_id"] == "credit_1" && len(posts) == 1 {
						return nil, fmt.Errorf("connection lost")
					}
					if body["grant_id"] == "credit_1" {
						return claudeTestResponse(`{"result":"` + replay + `"}`), nil
					}
					require.Equal(t, "credit_2", body["grant_id"])
					return claudeTestResponse(`{"result":"reset"}`), nil
				}
			})
			ch := claudeTestChannel()
			require.Error(t, c.ResetSelected(context.Background(), ch, "credit_1"))
			require.NoError(t, c.ResetSelected(context.Background(), ch, "credit_2"))
			require.GreaterOrEqual(t, len(posts), 2)
			require.Equal(t, posts[0], posts[1])
			if replay == "reset" {
				require.Len(t, posts, 2)
				return
			}
			require.Len(t, posts, 3)
			require.True(t, strings.HasPrefix(posts[2], "credit_2 "))
		})
	}
}

func TestClaudeUsageRefreshClearsSpentPendingGrant(t *testing.T) {
	usageCalls := 0
	var posts []string
	c := claudeTestChecker(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/oauth/profile":
			return claudeTestResponse(`{"organization":{"uuid":"12345678-1234-1234-1234-123456789012"}}`), nil
		case "/api/oauth/usage":
			usageCalls++
			if usageCalls == 1 {
				return claudeTestResponse(`{"cedar_ember":{"eligible":true,"at_limit":true,"next_grant_id":"credit_1","grants":[{"id":"credit_1","resets_left":1,"clears":["five_hour"],"usable_now":true},{"id":"credit_2","resets_left":1,"clears":["seven_day"],"usable_now":true}]}}`), nil
			}
			return claudeTestResponse(`{"five_hour":{"utilization":10},"cedar_ember":{"eligible":true,"at_limit":true,"next_grant_id":"credit_2","grants":[{"id":"credit_1","resets_left":0,"clears":["five_hour"],"usable_now":false},{"id":"credit_2","resets_left":1,"clears":["seven_day"],"usable_now":true}]}}`), nil
		default:
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			posts = append(posts, body["grant_id"])
			if len(posts) == 1 {
				return nil, fmt.Errorf("connection lost")
			}
			return claudeTestResponse(`{"result":"reset"}`), nil
		}
	})
	ch := claudeTestChannel()
	require.Error(t, c.ResetSelected(context.Background(), ch, "credit_1"))
	quota, err := c.CheckQuota(context.Background(), ch)
	require.NoError(t, err)
	require.Len(t, quota.Resets.Resets, 1)
	require.Equal(t, "credit_2", quota.Resets.Resets[0].ID)
	require.NoError(t, c.ResetSelected(context.Background(), ch, "credit_2"))
	require.Equal(t, []string{"credit_1", "credit_2"}, posts)
}

func TestClaudeModelWindowMatching(t *testing.T) {
	for _, test := range []struct {
		name, limits string
		want         map[string]string
	}{
		{"no scoped limit", `[{"kind":"weekly_all","percent":40}]`, nil},
		{"percent mismatch", `[{"kind":"weekly_scoped","percent":50,"scope":{"model":{"display_name":"Fable"}}}]`, nil},
		{"ambiguous models", `[{"kind":"weekly_scoped","percent":78,"scope":{"model":{"display_name":"Fable"}}},{"kind":"weekly_scoped","percent":78,"scope":{"model":{"display_name":"Mythos"}}}]`, nil},
		{"pick by percent", `[{"kind":"weekly_scoped","percent":10,"scope":{"model":{"display_name":"Mythos"}}},{"kind":"weekly_scoped","percent":78,"scope":{"model":{"display_name":"Fable"}}}]`, map[string]string{"seven_day_overage_included": "Fable"}},
		{"surface scope ignored", `[{"kind":"weekly_scoped","percent":78,"scope":{"model":null,"surface":"cowork"}}]`, nil},
		{"malformed limits", `"oops"`, nil},
		{"malformed entry skipped", `[42,{"kind":"weekly_scoped","percent":78,"scope":{"model":{"display_name":"Fable"}}}]`, map[string]string{"seven_day_overage_included": "Fable"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := claudeTestChecker(t, func(r *http.Request) (*http.Response, error) {
				return claudeTestResponse(claudeLiveUsage("", test.limits)), nil
			})
			q, err := c.CheckQuota(context.Background(), claudeTestChannel())
			require.NoError(t, err)
			require.Equal(t, test.want, q.Resets.WindowModels)
			require.Len(t, q.Resets.Resets, 2)
		})
	}
}
