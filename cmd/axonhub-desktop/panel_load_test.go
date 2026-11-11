package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/samber/lo"
)

func TestNextPageCursorStopsWhenThePageEnds(t *testing.T) {
	cursor := "c1"
	conn := panelConnection[string]{PageInfo: panelPageInfo{HasNextPage: true, EndCursor: &cursor}}
	if got := nextPageCursor(conn, ""); got != "c1" {
		t.Fatalf("cursor = %q", got)
	}
	if got := nextPageCursor(conn, "c1"); got != "" {
		t.Fatalf("repeated cursor = %q", got)
	}
	conn.PageInfo.HasNextPage = false
	if got := nextPageCursor(conn, ""); got != "" {
		t.Fatalf("last page cursor = %q", got)
	}
	blank := "  "
	conn.PageInfo = panelPageInfo{HasNextPage: true, EndCursor: &blank}
	if got := nextPageCursor(conn, ""); got != "" {
		t.Fatalf("blank cursor = %q", got)
	}
}

func TestMapPanelSnapshot(t *testing.T) {
	var data panelQueryData
	if err := json.Unmarshal([]byte(panelSnapshotFixture), &data); err != nil {
		t.Fatal(err)
	}
	got := withoutAttempts(mapPanelSnapshot(data, 3))
	want := panelSnapshot{
		RequestsToday: 12,
		InputTokens:   1000,
		OutputTokens:  250,
		CachedTokens:  80,
		Failures:      3,
		Requests: []panelRequest{
			{
				ID:               "key",
				ProjectID:        "gid://axonhub/Project/4",
				Caller:           "desktop",
				Channel:          "newest",
				ModelID:          "gpt-4.1",
				Status:           statusCompleted,
				PromptTokens:     lo.ToPtr(11),
				CompletionTokens: lo.ToPtr(100),
				CachedTokens:     lo.ToPtr(7),
				TotalTokens:      lo.ToPtr(118),
				LatencyMs:        lo.ToPtr(1500),
				TokensPerSecond:  lo.ToPtr(67),
			},
			{
				ID:               "agent",
				Caller:           "Codex",
				ModelID:          "codex",
				Status:           statusCompleted,
				PromptTokens:     lo.ToPtr(2),
				CompletionTokens: lo.ToPtr(1),
				TotalTokens:      lo.ToPtr(3),
				LatencyMs:        lo.ToPtr(2000),
				TokensPerSecond:  lo.ToPtr(1),
			},
			{
				ID:               "blank",
				ModelID:          "blank-model",
				Status:           statusCompleted,
				PromptTokens:     lo.ToPtr(5),
				CompletionTokens: lo.ToPtr(6),
				CachedTokens:     lo.ToPtr(0),
				TotalTokens:      lo.ToPtr(11),
			},
			{
				ID:               "zero-latency",
				Caller:           "zero",
				ModelID:          "z",
				Status:           statusCompleted,
				PromptTokens:     lo.ToPtr(1),
				CompletionTokens: lo.ToPtr(10),
				CachedTokens:     lo.ToPtr(0),
				TotalTokens:      lo.ToPtr(11),
				LatencyMs:        lo.ToPtr(0),
			},
			{
				ID:           "no-completion",
				Caller:       "partial",
				ModelID:      "n",
				Status:       statusCompleted,
				PromptTokens: lo.ToPtr(4),
				LatencyMs:    lo.ToPtr(1000),
			},
			{
				ID:               "failed",
				Caller:           "broken",
				Channel:          "fail-channel",
				ModelID:          "f",
				Status:           statusFailed,
				PromptTokens:     lo.ToPtr(9),
				CompletionTokens: lo.ToPtr(50),
				CachedTokens:     lo.ToPtr(1),
				TotalTokens:      lo.ToPtr(60),
				LatencyMs:        lo.ToPtr(500),
			},
			{
				ID:               "canceled",
				Caller:           "ua",
				ModelID:          "c",
				Status:           statusCanceled,
				PromptTokens:     lo.ToPtr(3),
				CompletionTokens: lo.ToPtr(4),
				TotalTokens:      lo.ToPtr(7),
				LatencyMs:        lo.ToPtr(800),
			},
			{
				ID:         "pending",
				Caller:     "pending-client",
				Channel:    "wait",
				ModelID:    "p",
				Status:     statusPending,
				InProgress: true,
			},
			{
				ID:               "processing",
				Caller:           "live",
				ModelID:          "proc",
				Status:           statusProcessing,
				InProgress:       true,
				PromptTokens:     lo.ToPtr(8),
				CompletionTokens: lo.ToPtr(8),
				TotalTokens:      lo.ToPtr(16),
				LatencyMs:        lo.ToPtr(100),
			},
			{
				ID:               "null-channel",
				Caller:           "named",
				ModelID:          "nc",
				Status:           statusCompleted,
				PromptTokens:     lo.ToPtr(1),
				CompletionTokens: lo.ToPtr(2),
				TotalTokens:      lo.ToPtr(3),
				LatencyMs:        lo.ToPtr(1000),
				TokensPerSecond:  lo.ToPtr(2),
			},
			{
				ID:        "null-usage",
				Caller:    "nu",
				ModelID:   "nu",
				Status:    statusCompleted,
				LatencyMs: lo.ToPtr(1000),
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot:\n%s\nwant:\n%s", mustSnapshotJSON(t, got), mustSnapshotJSON(t, want))
	}
}

func TestMapPanelSnapshotLeavesAbsentFieldsZero(t *testing.T) {
	got := mapPanelSnapshot(panelQueryData{}, 0)
	want := panelSnapshot{Requests: []panelRequest{}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot:\n%s\nwant:\n%s", mustSnapshotJSON(t, got), mustSnapshotJSON(t, want))
	}
}

func TestPanelTodayFollowsServerTimezone(t *testing.T) {
	now := time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC)
	shanghai := panelTimeLocation("Asia/Shanghai")
	start, end := panelToday(now, shanghai)
	wantStart := time.Date(2026, 11, 11, 0, 0, 0, 0, shanghai)
	if !start.Equal(wantStart) || !end.Equal(wantStart.AddDate(0, 0, 1)) {
		t.Fatalf("Shanghai day = %s .. %s, want %s .. %s", start, end, wantStart, wantStart.AddDate(0, 0, 1))
	}

	start, end = panelToday(now, panelTimeLocation(""))
	wantStart = time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) || !end.Equal(wantStart.AddDate(0, 0, 1)) {
		t.Fatalf("UTC day = %s .. %s", start, end)
	}
	start, end = panelToday(now, panelTimeLocation("Not/AZone"))
	if !start.Equal(wantStart) || !end.Equal(wantStart.AddDate(0, 0, 1)) {
		t.Fatalf("unknown timezone day = %s .. %s, want UTC", start, end)
	}
}

func TestLoadPanelReadsProjectGraphQL(t *testing.T) {
	const (
		token     = "desktop-token"
		projectID = "gid://axonhub/Project/7"
	)
	now := time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC)
	server, calls := servePanel(t, panelHTTPBody("Asia/Shanghai"), `{"data":{"requests":{"totalCount":4}}}`)
	session := desktopSession{Token: token, ProjectID: projectID}

	got, err := loadPanelSnapshot(t.Context(), server.URL, session, server.Client(), now)
	if err != nil {
		t.Fatal(err)
	}
	got = withoutAttempts(got)
	want := panelSnapshot{
		RequestsToday: 6,
		InputTokens:   10,
		OutputTokens:  20,
		CachedTokens:  3,
		Failures:      4,
		DesktopKey:    panelDesktopKey,
		Requests: []panelRequest{{
			ID:               "gid://axonhub/Request/1",
			ProjectID:        "gid://axonhub/Project/9",
			Caller:           "key",
			Channel:          "openai",
			ModelID:          "m",
			Status:           statusCompleted,
			PromptTokens:     lo.ToPtr(1),
			CompletionTokens: lo.ToPtr(2),
			TotalTokens:      lo.ToPtr(3),
			LatencyMs:        lo.ToPtr(1000),
			TokensPerSecond:  lo.ToPtr(2),
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot:\n%s\nwant:\n%s", mustSnapshotJSON(t, got), mustSnapshotJSON(t, want))
	}
	if len(*calls) != 3 {
		t.Fatalf("calls = %d, want 3", len(*calls))
	}
	assertPanelQuery(t, (*calls)[0], token)
	assertFailureQuery(t, (*calls)[1], token, time.Date(2026, 11, 11, 0, 0, 0, 0, panelTimeLocation("Asia/Shanghai")))
	assertDesktopKeyQuery(t, (*calls)[2], token, projectID)
}

func TestShellLoadPanelUsesStoredSession(t *testing.T) {
	const (
		token     = "stored-token"
		projectID = "gid://axonhub/Project/3"
	)
	server, calls := servePanel(t, panelHTTPBody("UTC"), `{"data":{"requests":{"totalCount":1}}}`)
	shell := newShell(server.URL+"/", desktopSession{Token: token, ProjectID: projectID})
	if shell.origin != server.URL || shell.session.Token != token || shell.session.ProjectID != projectID {
		t.Fatalf("shell origin = %q session = %+v", shell.origin, shell.session)
	}

	before := time.Now()
	got, err := shell.loadPanel(t.Context())
	after := time.Now()
	if err != nil {
		t.Fatal(err)
	}
	if got.Failures != 1 || got.RequestsToday != 6 || got.DesktopKey != panelDesktopKey {
		t.Fatalf("failures = %d requests = %d", got.Failures, got.RequestsToday)
	}
	if len(*calls) != 3 || (*calls)[0].Path != "/admin/graphql" {
		t.Fatalf("calls = %+v", *calls)
	}
	assertPanelQuery(t, (*calls)[0], token)
	start := utcMidnight(before)
	if !graphqlTime(t, whereOf(t, (*calls)[1]), "createdAtGTE").Equal(start) {
		start = utcMidnight(after)
	}
	assertFailureQuery(t, (*calls)[1], token, start)
	assertDesktopKeyQuery(t, (*calls)[2], token, projectID)
}

func TestLoadPanelUnknownTimezoneUsesUTC(t *testing.T) {
	now := time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC)
	server, calls := servePanel(t, panelHTTPBody("Not/AZone"), `{"data":{"requests":{}}}`)
	_, err := loadPanelSnapshot(t.Context(), server.URL, desktopSession{Token: "desktop-token", ProjectID: "gid://axonhub/Project/7"}, server.Client(), now)
	if err != nil {
		t.Fatal(err)
	}
	assertFailureQuery(t, (*calls)[1], "desktop-token", time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC))
}

func TestLoadPanelReportsGraphQLAndHTTPErrors(t *testing.T) {
	const token = "desktop-token"
	session := desktopSession{Token: token, ProjectID: "gid://axonhub/Project/7"}

	errorServer, errorCalls := servePanel(t, `{"errors":[{"message":"nope"}]}`, `{"data":{"requests":{"totalCount":1}}}`)
	if _, err := loadPanelSnapshot(t.Context(), errorServer.URL, session, errorServer.Client(), time.Now()); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatal(err)
	}
	if len(*errorCalls) != 1 {
		t.Fatalf("failure query ran after a GraphQL error: %d calls", len(*errorCalls))
	}

	nullServer, _ := servePanel(t, `{"data":null}`, `{"data":{"requests":{"totalCount":1}}}`)
	if _, err := loadPanelSnapshot(t.Context(), nullServer.URL, session, nullServer.Client(), time.Now()); err == nil || !strings.Contains(err.Error(), "did not include data") {
		t.Fatal(err)
	}

	denied := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, token)
	}))
	t.Cleanup(denied.Close)
	_, err := loadPanelSnapshot(t.Context(), denied.URL, session, denied.Client(), time.Now())
	if err == nil || !strings.Contains(err.Error(), "500") || strings.Contains(err.Error(), token) {
		t.Fatalf("error = %v, want a redacted 500", err)
	}

	if _, err := loadPanelSnapshot(t.Context(), "http://127.0.0.1", desktopSession{}, nil, time.Now()); err == nil {
		t.Fatal("missing session was accepted")
	}
	if _, err := loadPanelSnapshot(t.Context(), "///", desktopSession{Token: token, ProjectID: session.ProjectID}, nil, time.Now()); err == nil {
		t.Fatal("blank origin was accepted")
	}
}

func utcMidnight(at time.Time) time.Time {
	at = at.UTC()
	return time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
}

func TestLoadPanelAbsentFailureCountStaysZero(t *testing.T) {
	server, _ := servePanel(t, `{"data":{"dashboardOverview":{"requestStats":{}},"tokenStats":{},"systemGeneralSettings":{"timezone":"UTC"},"requests":{"edges":[]}}}`, `{"data":{"requests":{}}}`)
	got, err := loadPanelSnapshot(t.Context(), server.URL, desktopSession{Token: "desktop-token", ProjectID: "gid://axonhub/Project/7"}, server.Client(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	want := panelSnapshot{DesktopKey: panelDesktopKey, Requests: []panelRequest{}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot:\n%s\nwant:\n%s", mustSnapshotJSON(t, got), mustSnapshotJSON(t, want))
	}
}

type postedGraphQL struct {
	Query         string
	OperationName string
	Variables     map[string]any
	Header        http.Header
	Path          string
	Method        string
}

func servePanel(t *testing.T, panelBody, failureBody string) (*httptest.Server, *[]postedGraphQL) {
	t.Helper()
	calls := &[]postedGraphQL{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call, err := readPostedGraphQL(r)
		if err != nil {
			t.Errorf("read graphql: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		*calls = append(*calls, call)
		body := panelBody
		switch call.OperationName {
		case "DesktopPanelFailures":
			body = failureBody
		case "DesktopPanelKey":
			body = desktopKeyHTTPBody()
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, calls
}

func readPostedGraphQL(r *http.Request) (postedGraphQL, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return postedGraphQL{}, err
	}
	var decoded struct {
		Query         string         `json:"query"`
		OperationName string         `json:"operationName"`
		Variables     map[string]any `json:"variables"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return postedGraphQL{}, err
	}
	return postedGraphQL{
		Query:         decoded.Query,
		OperationName: decoded.OperationName,
		Variables:     decoded.Variables,
		Header:        r.Header.Clone(),
		Path:          r.URL.Path,
		Method:        r.Method,
	}, nil
}

// assertPanelQuery checks the global snapshot query: no project header and
// no project filter. Each request still carries its own projectID.
func assertPanelQuery(t *testing.T, call postedGraphQL, token string) {
	t.Helper()
	assertGraphQLAuth(t, call, token, "")
	if call.OperationName != "DesktopPanel" {
		t.Fatalf("operation = %q", call.OperationName)
	}
	for _, field := range []string{
		"projectID",
		"requestsToday",
		"totalInputTokensToday",
		"totalOutputTokensToday",
		"totalCachedTokensToday",
		"metricsLatencyMs",
		"promptTokens",
		"completionTokens",
		"promptCachedTokens",
		"totalTokens",
		"userAgent",
		"modelID",
		"CREATED_AT",
		"DESC",
	} {
		if !strings.Contains(call.Query, field) {
			t.Fatalf("query missing %s", field)
		}
	}
	if strings.Contains(call.Query, "failedRequests") {
		t.Fatal("query reads dashboardOverview.failedRequests")
	}
	if call.Variables["first"] != float64(panelRequestLimit) {
		t.Fatalf("first = %#v", call.Variables["first"])
	}
	orderBy, _ := call.Variables["orderBy"].(map[string]any)
	if orderBy["field"] != "CREATED_AT" || orderBy["direction"] != "DESC" {
		t.Fatalf("orderBy = %#v", orderBy)
	}
	if where, ok := call.Variables["where"]; ok {
		t.Fatalf("recent requests are global, where = %#v", where)
	}
}

func assertDesktopKeyQuery(t *testing.T, call postedGraphQL, token, projectID string) {
	t.Helper()
	assertGraphQLAuth(t, call, token, projectID)
	if call.OperationName != "DesktopPanelKey" {
		t.Fatalf("operation = %q", call.OperationName)
	}
	for _, field := range []string{"id", "name", "status", "key"} {
		if !strings.Contains(call.Query, field) {
			t.Fatalf("query missing %s", field)
		}
	}
	for _, field := range []string{"profiles", "scopes", "allowedIps"} {
		if strings.Contains(call.Query, field) {
			t.Fatalf("query requests %s", field)
		}
	}
	if call.Variables["first"] != float64(desktopKeyLimit) {
		t.Fatalf("first = %#v", call.Variables["first"])
	}
	where := whereOf(t, call)
	if where["projectID"] != projectID || where["name"] != desktopKeyName {
		t.Fatalf("where = %#v", where)
	}
	got := stringList(t, where["statusIn"])
	if !reflect.DeepEqual(got, []string{statusKeyEnabled, statusKeyDisabled, statusKeyArchived}) {
		t.Fatalf("statusIn = %#v", got)
	}
}

func assertFailureQuery(t *testing.T, call postedGraphQL, token string, start time.Time) {
	t.Helper()
	assertGraphQLAuth(t, call, token, "")
	if call.OperationName != "DesktopPanelFailures" || !strings.Contains(call.Query, "totalCount") {
		t.Fatalf("failure query = %s %s", call.OperationName, call.Query)
	}
	where := whereOf(t, call)
	if _, ok := where["projectID"]; ok {
		t.Fatalf("failure query is global, where = %#v", where)
	}
	got := stringList(t, where["statusIn"])
	if !reflect.DeepEqual(got, []string{statusFailed, statusCanceled}) {
		t.Fatalf("statusIn = %#v", got)
	}
	gotStart := graphqlTime(t, where, "createdAtGTE")
	gotEnd := graphqlTime(t, where, "createdAtLT")
	if !gotStart.Equal(start) || !gotEnd.Equal(start.AddDate(0, 0, 1)) {
		t.Fatalf("failure window = %s .. %s, want %s .. %s", gotStart, gotEnd, start, start.AddDate(0, 0, 1))
	}
}

func assertGraphQLAuth(t *testing.T, call postedGraphQL, token, projectID string) {
	t.Helper()
	if call.Method != http.MethodPost || call.Path != "/admin/graphql" {
		t.Fatalf("request = %s %s", call.Method, call.Path)
	}
	if call.Header.Get("Authorization") != "Bearer "+token {
		t.Fatalf("authorization = %q", call.Header.Get("Authorization"))
	}
	if call.Header.Get("X-Project-ID") != projectID {
		t.Fatalf("project header = %q", call.Header.Get("X-Project-ID"))
	}
	if call.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("content type = %q", call.Header.Get("Content-Type"))
	}
}

func whereOf(t *testing.T, call postedGraphQL) map[string]any {
	t.Helper()
	where, ok := call.Variables["where"].(map[string]any)
	if !ok {
		t.Fatalf("where = %#v", call.Variables["where"])
	}
	return where
}

func stringList(t *testing.T, value any) []string {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("list = %#v", value)
	}
	return lo.Map(items, func(item any, _ int) string {
		text, ok := item.(string)
		if !ok {
			t.Fatalf("list item = %#v", item)
		}
		return text
	})
}

func graphqlTime(t *testing.T, where map[string]any, key string) time.Time {
	t.Helper()
	raw, _ := where[key].(string)
	got, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		t.Fatalf("parse %s %q: %v", key, raw, err)
	}
	return got
}

func mustSnapshotJSON(t *testing.T, snapshot panelSnapshot) string {
	t.Helper()
	body, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

const panelDesktopKey = "desktop-panel-key"

func desktopKeyHTTPBody() string {
	return `{
	  "data": {
	    "apiKeys": {
	      "edges": [
	        {
	          "node": {
	            "id": "gid://axonhub/APIKey/1",
	            "name": "AxonHub Desktop",
	            "status": "enabled",
	            "key": "` + panelDesktopKey + `"
	          }
	        }
	      ]
	    }
	  }
	}`
}

func panelHTTPBody(timezone string) string {
	return `{
	  "data": {
	    "dashboardOverview": {
	      "failedRequests": 99,
	      "requestStats": { "requestsToday": 6 }
	    },
	    "tokenStats": {
	      "totalInputTokensToday": 10,
	      "totalOutputTokensToday": 20,
	      "totalCachedTokensToday": 3
	    },
	    "systemGeneralSettings": { "timezone": "` + timezone + `" },
	    "requests": {
	      "edges": [
	        {
	          "node": {
	            "id": "gid://axonhub/Request/1",
	            "projectID": "gid://axonhub/Project/9",
	            "modelID": "m",
	            "status": "completed",
	            "userAgent": "ua",
	            "metricsLatencyMs": 1000,
	            "apiKey": { "name": "key" },
	            "executions": { "edges": [{ "node": { "createdAt": "2026-11-10T00:00:00Z", "channel": { "name": "openai" } } }] },
	            "usageLogs": { "edges": [{ "node": { "promptTokens": 1, "completionTokens": 2, "promptCachedTokens": null, "totalTokens": 3 } }] }
	          }
	        }
	      ]
	    }
	  }
	}`
}

const panelSnapshotFixture = `{
  "dashboardOverview": {
    "failedRequests": 99,
    "requestStats": { "requestsToday": 12 }
  },
  "tokenStats": {
    "totalInputTokensToday": 1000,
    "totalOutputTokensToday": 250,
    "totalCachedTokensToday": 80
  },
  "requests": {
    "edges": [
      {
        "node": {
	          "id": "key",
	          "projectID": "gid://axonhub/Project/4",
	          "modelID": "gpt-4.1",
          "status": "completed",
          "userAgent": "curl/8.0",
          "metricsLatencyMs": 1500,
          "apiKey": { "name": "desktop" },
          "executions": {
            "edges": [
              { "node": { "createdAt": "2026-11-10T01:00:00Z", "channel": { "name": "older" } } },
              { "node": null },
              { "node": { "createdAt": "2026-11-10T03:00:00Z", "channel": { "name": "newest" } } },
              { "node": { "createdAt": "2026-11-10T02:00:00Z", "channel": { "name": "middle" } } }
            ]
          },
          "usageLogs": {
            "edges": [
              { "node": { "promptTokens": 11, "completionTokens": 100, "promptCachedTokens": 7, "totalTokens": 118 } }
            ]
          }
        }
      },
      {
        "node": {
          "id": "agent",
          "modelID": "codex",
          "status": "completed",
          "userAgent": "Codex",
          "metricsLatencyMs": 2000,
          "apiKey": { "name": "" },
          "executions": { "edges": [] },
          "usageLogs": {
            "edges": [
              { "node": { "promptTokens": 2, "completionTokens": 1, "promptCachedTokens": null, "totalTokens": 3 } }
            ]
          }
        }
      },
      {
        "node": {
          "id": "blank",
          "modelID": "blank-model",
          "status": "completed",
          "userAgent": "",
          "metricsLatencyMs": null,
          "apiKey": null,
          "executions": { "edges": [] },
          "usageLogs": {
            "edges": [
              { "node": { "promptTokens": 5, "completionTokens": 6, "promptCachedTokens": 0, "totalTokens": 11 } }
            ]
          }
        }
      },
      {
        "node": {
          "id": "zero-latency",
          "modelID": "z",
          "status": "completed",
          "userAgent": "ua",
          "metricsLatencyMs": 0,
          "apiKey": { "name": "zero" },
          "usageLogs": {
            "edges": [
              { "node": { "promptTokens": 1, "completionTokens": 10, "promptCachedTokens": 0, "totalTokens": 11 } }
            ]
          }
        }
      },
      {
        "node": {
          "id": "no-completion",
          "modelID": "n",
          "status": "completed",
          "userAgent": "ua",
          "metricsLatencyMs": 1000,
          "apiKey": { "name": "partial" },
          "usageLogs": {
            "edges": [
              { "node": { "promptTokens": 4, "completionTokens": null, "promptCachedTokens": null, "totalTokens": null } }
            ]
          }
        }
      },
      {
        "node": {
          "id": "failed",
          "modelID": "f",
          "status": "failed",
          "userAgent": "ua",
          "metricsLatencyMs": 500,
          "apiKey": { "name": "broken" },
          "executions": {
            "edges": [
              { "node": { "createdAt": "2026-11-10T04:00:00Z", "channel": { "name": "fail-channel" } } }
            ]
          },
          "usageLogs": {
            "edges": [
              { "node": { "promptTokens": 9, "completionTokens": 50, "promptCachedTokens": 1, "totalTokens": 60 } }
            ]
          }
        }
      },
      {
        "node": {
          "id": "canceled",
          "modelID": "c",
          "status": "canceled",
          "userAgent": "ua",
          "metricsLatencyMs": 800,
          "apiKey": null,
          "usageLogs": {
            "edges": [
              { "node": { "promptTokens": 3, "completionTokens": 4, "promptCachedTokens": null, "totalTokens": 7 } }
            ]
          }
        }
      },
      {
        "node": {
          "id": "pending",
          "modelID": "p",
          "status": "pending",
          "userAgent": "pending-client",
          "apiKey": null,
          "executions": {
            "edges": [
              { "node": { "createdAt": "2026-11-10T05:00:00Z", "channel": { "name": "wait" } } }
            ]
          },
          "usageLogs": { "edges": [] }
        }
      },
      {
        "node": {
          "id": "processing",
          "modelID": "proc",
          "status": "processing",
          "userAgent": "",
          "metricsLatencyMs": 100,
          "apiKey": { "name": "live" },
          "usageLogs": {
            "edges": [
              { "node": { "promptTokens": 8, "completionTokens": 8, "promptCachedTokens": null, "totalTokens": 16 } }
            ]
          }
        }
      },
      {
        "node": {
          "id": "null-channel",
          "modelID": "nc",
          "status": "completed",
          "userAgent": "ua",
          "metricsLatencyMs": 1000,
          "apiKey": { "name": "named" },
          "executions": {
            "edges": [
              { "node": { "createdAt": "2026-11-10T01:00:00Z", "channel": { "name": "stale" } } },
              { "node": { "createdAt": "2026-11-10T06:00:00Z", "channel": null } }
            ]
          },
          "usageLogs": {
            "edges": [
              { "node": null },
              { "node": { "promptTokens": 1, "completionTokens": 2, "promptCachedTokens": null, "totalTokens": 3 } }
            ]
          }
        }
      },
      {
        "node": {
          "id": "null-usage",
          "modelID": "nu",
          "status": "completed",
          "userAgent": "ua",
          "metricsLatencyMs": 1000,
          "apiKey": { "name": "nu" },
          "usageLogs": { "edges": [{ "node": null }] }
        }
      },
      { "node": null }
    ]
  }
}`

// withoutAttempts drops the attempts, which TestRequestAttemptsReadOldestFirst
// covers, so a snapshot comparison reads only the row fields.
func withoutAttempts(snapshot panelSnapshot) panelSnapshot {
	for i := range snapshot.Requests {
		snapshot.Requests[i].Attempts = nil
	}
	return snapshot
}

func TestRequestAttemptsReadOldestFirst(t *testing.T) {
	var node panelRequestNode
	if err := json.Unmarshal([]byte(`{
	  "id": "r", "status": "completed",
	  "executions": { "edges": [
	    { "node": { "createdAt": "2026-11-10T00:00:02Z", "status": "completed", "responseStatusCode": 200, "metricsLatencyMs": 1200, "metricsFirstTokenLatencyMs": 300, "channel": { "name": "backup" } } },
	    { "node": null },
	    { "node": { "createdAt": "2026-11-10T00:00:01Z", "status": "failed", "responseStatusCode": 429, "errorMessage": " rate limited ", "channel": null } }
	  ] }
	}`), &node); err != nil {
		t.Fatal(err)
	}
	got := mapPanelRequest(node).Attempts
	want := []panelAttempt{
		{Status: statusFailed, StatusCode: lo.ToPtr(429), Error: "rate limited"},
		{Channel: "backup", Status: statusCompleted, StatusCode: lo.ToPtr(200), LatencyMs: lo.ToPtr(1200), FirstTokenMs: lo.ToPtr(300)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("attempts = %#v", got)
	}
}
