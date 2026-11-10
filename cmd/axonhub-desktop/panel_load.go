package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/samber/lo"
)

const (
	panelRequestLimit = 8
	statusPending     = "pending"
	statusProcessing  = "processing"
	statusCompleted   = "completed"
	statusFailed      = "failed"
	statusCanceled    = "canceled"
)

const panelQuery = `query DesktopPanel($first: Int!, $orderBy: RequestOrder!) {
  dashboardOverview {
    requestStats {
      requestsToday
    }
  }
  tokenStats {
    totalInputTokensToday
    totalOutputTokensToday
    totalCachedTokensToday
  }
  systemGeneralSettings {
    timezone
  }
  requests(first: $first, orderBy: $orderBy) {
    edges {
      node {
        id
        projectID
        modelID
        status
        userAgent
        metricsLatencyMs
        apiKey {
          name
        }
        executions(first: 10, orderBy: { field: CREATED_AT, direction: DESC }) {
          edges {
            node {
              createdAt
              channel {
                name
              }
            }
          }
        }
        usageLogs(first: 1) {
          edges {
            node {
              promptTokens
              completionTokens
              promptCachedTokens
              totalTokens
            }
          }
        }
      }
    }
  }
}`

// panelFailuresQuery counts failed and canceled requests in a createdAt
// window. dashboardOverview.failedRequests is the all-time failed count and
// does not include canceled requests, so the snapshot does not read it.
const panelFailuresQuery = `query DesktopPanelFailures($where: RequestWhereInput!) {
  requests(first: 1, where: $where) {
    totalCount
  }
}`

// panelSnapshot is the project data a later panel can render.
// RequestsToday, InputTokens, OutputTokens, and CachedTokens come from the
// dashboard aggregates. Failures is the today-scoped count of failed and
// canceled requests. Absent aggregate fields stay zero.
// DesktopKey is the stored secret for the project's Desktop API key. The
// panel copies it and does not draw it. Empty means it is not ready.
type panelSnapshot struct {
	RequestsToday int
	InputTokens   int
	OutputTokens  int
	CachedTokens  int
	Failures      int
	Requests      []panelRequest
	DesktopKey    string
}

// panelRequest is one row. PromptTokens, CompletionTokens, CachedTokens,
// TotalTokens, and LatencyMs stay nil when the JSON field is null.
// TokensPerSecond is completion tokens divided by latency in seconds,
// rounded to a whole number, and only for a completed request with a
// positive latency and a completion count. InProgress is true for pending
// and processing; a later routing card animates only while the selected
// row is in progress.
type panelRequest struct {
	ID               string
	ProjectID        string
	Caller           string
	Channel          string
	ModelID          string
	Status           string
	InProgress       bool
	PromptTokens     *int
	CompletionTokens *int
	CachedTokens     *int
	TotalTokens      *int
	LatencyMs        *int
	TokensPerSecond  *int
}

type panelQueryData struct {
	DashboardOverview struct {
		RequestStats struct {
			RequestsToday int `json:"requestsToday"`
		} `json:"requestStats"`
	} `json:"dashboardOverview"`
	TokenStats struct {
		TotalInputTokensToday  int `json:"totalInputTokensToday"`
		TotalOutputTokensToday int `json:"totalOutputTokensToday"`
		TotalCachedTokensToday int `json:"totalCachedTokensToday"`
	} `json:"tokenStats"`
	SystemGeneralSettings struct {
		Timezone string `json:"timezone"`
	} `json:"systemGeneralSettings"`
	Requests panelConnection[panelRequestNode] `json:"requests"`
}

type panelFailureData struct {
	Requests struct {
		TotalCount int `json:"totalCount"`
	} `json:"requests"`
}

type panelConnection[T any] struct {
	Edges []panelEdge[T] `json:"edges"`
}

type panelEdge[T any] struct {
	Node *T `json:"node"`
}

type panelName struct {
	Name string `json:"name"`
}

type panelRequestNode struct {
	ID               string                              `json:"id"`
	ProjectID        string                              `json:"projectID"`
	ModelID          string                              `json:"modelID"`
	Status           string                              `json:"status"`
	UserAgent        string                              `json:"userAgent"`
	MetricsLatencyMs *int                                `json:"metricsLatencyMs"`
	APIKey           *panelName                          `json:"apiKey"`
	Executions       panelConnection[panelExecutionNode] `json:"executions"`
	UsageLogs        panelConnection[panelUsageNode]     `json:"usageLogs"`
}

type panelExecutionNode struct {
	CreatedAt time.Time  `json:"createdAt"`
	Channel   *panelName `json:"channel"`
}

type panelUsageNode struct {
	PromptTokens       *int `json:"promptTokens"`
	CompletionTokens   *int `json:"completionTokens"`
	PromptCachedTokens *int `json:"promptCachedTokens"`
	TotalTokens        *int `json:"totalTokens"`
}

type graphQLRequest struct {
	Query         string `json:"query"`
	OperationName string `json:"operationName,omitempty"`
	Variables     any    `json:"variables,omitempty"`
}

type graphQLErrorItem struct {
	Message string `json:"message"`
}

// loadPanelSnapshot reads stats and recent requests across every project.
// The Desktop key always comes from the session's own project.
func loadPanelSnapshot(ctx context.Context, origin string, session desktopSession, client *http.Client, now time.Time) (panelSnapshot, error) {
	if session.Token == "" || session.ProjectID == "" {
		return panelSnapshot{}, errors.New("desktop session is missing a token or project")
	}
	origin = strings.TrimRight(origin, "/")
	if origin == "" {
		return panelSnapshot{}, errors.New("desktop server address is empty")
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	// No X-Project-ID: dashboard and token stats bypass project privacy, and
	// the request list is not filtered, so the panel stays global.
	global := desktopSession{Token: session.Token}
	data, err := postGraphQL[panelQueryData](ctx, client, origin, global, "DesktopPanel", panelQuery, panelVariables())
	if err != nil {
		return panelSnapshot{}, err
	}
	start, end := panelToday(now, panelTimeLocation(data.SystemGeneralSettings.Timezone))
	failures, err := postGraphQL[panelFailureData](ctx, client, origin, global, "DesktopPanelFailures", panelFailuresQuery, failureVariables(start, end))
	if err != nil {
		return panelSnapshot{}, err
	}
	// A failed ensure aborts this refresh. applySnapshot then leaves the
	// previous snapshot, including its key, in place.
	key, err := ensureDesktopKey(ctx, client, origin, session)
	if err != nil {
		return panelSnapshot{}, err
	}
	snapshot := mapPanelSnapshot(data, failures.Requests.TotalCount)
	snapshot.DesktopKey = key
	return snapshot, nil
}

func panelVariables() map[string]any {
	return map[string]any{
		"first": panelRequestLimit,
		"orderBy": map[string]string{
			"field":     "CREATED_AT",
			"direction": "DESC",
		},
	}
}

func failureVariables(start, end time.Time) map[string]any {
	return map[string]any{"where": map[string]any{
		"statusIn":     []string{statusFailed, statusCanceled},
		"createdAtGTE": start.Format(time.RFC3339Nano),
		"createdAtLT":  end.Format(time.RFC3339Nano),
	}}
}

// panelTimeLocation uses the server general-settings timezone. An empty or
// unknown name falls back to UTC, matching SystemService.TimeLocation.
func panelTimeLocation(name string) *time.Location {
	name = strings.TrimSpace(name)
	if name == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

// panelToday is the half-open local day [start, end) used by the dashboard.
func panelToday(now time.Time, loc *time.Location) (time.Time, time.Time) {
	local := now.In(loc)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	return start, start.AddDate(0, 0, 1)
}

func mapPanelSnapshot(data panelQueryData, failures int) panelSnapshot {
	nodes := connectionNodes(data.Requests)
	return panelSnapshot{
		RequestsToday: data.DashboardOverview.RequestStats.RequestsToday,
		InputTokens:   data.TokenStats.TotalInputTokensToday,
		OutputTokens:  data.TokenStats.TotalOutputTokensToday,
		CachedTokens:  data.TokenStats.TotalCachedTokensToday,
		Failures:      failures,
		Requests: lo.Map(nodes, func(node panelRequestNode, _ int) panelRequest {
			return mapPanelRequest(node)
		}),
	}
}

func mapPanelRequest(node panelRequestNode) panelRequest {
	var prompt, completion, cached, total *int
	if usage, ok := lo.First(connectionNodes(node.UsageLogs)); ok {
		prompt = usage.PromptTokens
		completion = usage.CompletionTokens
		cached = usage.PromptCachedTokens
		total = usage.TotalTokens
	}
	return panelRequest{
		ID:               node.ID,
		ProjectID:        node.ProjectID,
		Caller:           requestCaller(node.APIKey, node.UserAgent),
		Channel:          executionChannel(node.Executions),
		ModelID:          node.ModelID,
		Status:           node.Status,
		InProgress:       requestInProgress(node.Status),
		PromptTokens:     prompt,
		CompletionTokens: completion,
		CachedTokens:     cached,
		TotalTokens:      total,
		LatencyMs:        node.MetricsLatencyMs,
		TokensPerSecond:  completionRate(node.Status, completion, node.MetricsLatencyMs),
	}
}

func requestCaller(apiKey *panelName, userAgent string) string {
	name := ""
	if apiKey != nil {
		name = apiKey.Name
	}
	return lo.CoalesceOrEmpty(name, userAgent)
}

func executionChannel(conn panelConnection[panelExecutionNode]) string {
	nodes := connectionNodes(conn)
	if len(nodes) == 0 {
		return ""
	}
	newest := lo.LatestBy(nodes, func(item panelExecutionNode) time.Time {
		return item.CreatedAt
	})
	if newest.Channel == nil {
		return ""
	}
	return newest.Channel.Name
}

func requestInProgress(status string) bool {
	return lo.Contains([]string{statusPending, statusProcessing}, status)
}

func completionRate(status string, completion, latencyMs *int) *int {
	if status != statusCompleted || completion == nil || latencyMs == nil || *latencyMs <= 0 {
		return nil
	}
	perSecond := math.Round(float64(*completion) / (float64(*latencyMs) / 1000))
	return lo.ToPtr(int(perSecond))
}

func connectionNodes[T any](conn panelConnection[T]) []T {
	return lo.FilterMap(conn.Edges, func(edge panelEdge[T], _ int) (T, bool) {
		if edge.Node == nil {
			return lo.Empty[T](), false
		}
		return *edge.Node, true
	})
}

func postGraphQL[T any](ctx context.Context, client *http.Client, origin string, session desktopSession, operation, query string, variables any) (T, error) {
	var zero T
	body, err := json.Marshal(graphQLRequest{
		Query:         query,
		OperationName: operation,
		Variables:     variables,
	})
	if err != nil {
		return zero, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, origin+"/admin/graphql", bytes.NewReader(body))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+session.Token)
	// A session without a project reads across projects.
	if session.ProjectID != "" {
		req.Header.Set("X-Project-ID", session.ProjectID)
	}

	resp, err := client.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return zero, err
	}
	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("POST %s: %s: %s", req.URL.Path, resp.Status, redact(strings.TrimSpace(string(payload)), session.Token))
	}

	var decoded struct {
		Data   json.RawMessage    `json:"data"`
		Errors []graphQLErrorItem `json:"errors"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return zero, fmt.Errorf("decode %s: %w", operation, err)
	}
	if len(decoded.Errors) > 0 {
		messages := lo.Map(decoded.Errors, func(item graphQLErrorItem, _ int) string {
			return item.Message
		})
		return zero, fmt.Errorf("%s: %s", operation, redact(strings.Join(messages, "; "), session.Token))
	}
	if len(bytes.TrimSpace(decoded.Data)) == 0 || string(decoded.Data) == "null" {
		return zero, fmt.Errorf("%s: response did not include data", operation)
	}
	var data T
	if err := json.Unmarshal(decoded.Data, &data); err != nil {
		return zero, fmt.Errorf("decode %s: %w", operation, err)
	}
	return data, nil
}
