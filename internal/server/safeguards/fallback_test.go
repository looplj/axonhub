package safeguards

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fallbackEvaluator(t *testing.T, config Config, call ClassifierFunc) *Evaluator {
	t.Helper()
	e, err := New([]byte(requestBody), config, call)
	if err != nil || e == nil {
		t.Fatalf("New: %v, %v", e, err)
	}
	return e
}

func proposedIDs(t *testing.T, request ClassifierRequest) []string {
	t.Helper()
	_, encoded, found := strings.Cut(request.User, "\n\nProposed tool calls:\n")
	if !found {
		t.Fatal("missing proposed tools")
	}
	var proposed []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(encoded), &proposed); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(proposed))
	for i, tool := range proposed {
		ids[i] = tool.ID
	}
	return ids
}

func TestFallbackRecoversFailures(t *testing.T) {
	for _, failure := range []string{"error", "malformed", "timeout", "late_success"} {
		t.Run(failure, func(t *testing.T) {
			var called []string
			e := fallbackEvaluator(t, Config{ClassifierModel: "primary", FallbackModels: []string{"backup"}, Timeout: time.Second, AttemptTimeout: 10 * time.Millisecond}, func(ctx context.Context, r ClassifierRequest) (string, error) {
				called = append(called, r.Model)
				if r.Model == "primary" {
					switch failure {
					case "error":
						return "", errors.New("upstream failed")
					case "malformed":
						return `{"verdicts":`, nil
					case "timeout":
						<-ctx.Done()
						return "", ctx.Err()
					case "late_success":
						<-ctx.Done()
						return `{"verdicts":{"safe":{"flagged":true},"danger":{"flagged":false}}}`, nil
					}
				}
				if ctx.Err() != nil {
					t.Fatal("backup inherits attempt cancellation")
				}
				return `{"verdicts":{"safe":{"flagged":false},"danger":{"flagged":true}}}`, nil
			})
			got := e.Evaluate(t.Context(), tools())[0].Status.ToolUses
			if !reflect.DeepEqual(called, []string{"primary", "backup"}) || got["safe"].Outcome != "not_flagged" || got["danger"].Outcome != "flagged" {
				t.Fatalf("calls %v, verdicts %+v", called, got)
			}
		})
	}
}

func TestFallbackOnlyReviewsUnresolvedTools(t *testing.T) {
	for _, flagged := range []bool{false, true} {
		t.Run(map[bool]string{false: "allowed", true: "flagged"}[flagged], func(t *testing.T) {
			var called []string
			e := fallbackEvaluator(t, Config{ClassifierModel: " primary ", FallbackModels: []string{"", "primary", " backup ", "backup", "unused"}}, func(_ context.Context, r ClassifierRequest) (string, error) {
				called = append(called, r.Model)
				if r.Model == "primary" {
					if !reflect.DeepEqual(proposedIDs(t, r), []string{"safe", "danger"}) {
						t.Fatal("missing primary tools")
					}
					encoded, _ := json.Marshal(map[string]any{"verdicts": map[string]any{"safe": map[string]any{"flagged": flagged}, "danger": map[string]any{"flagged": "false"}}})
					return string(encoded), nil
				}
				if !reflect.DeepEqual(proposedIDs(t, r), []string{"danger"}) {
					t.Fatal("backup re-reviews an evaluated tool")
				}
				// An unsolicited contradictory verdict must not overwrite the primary's verdict.
				encoded, _ := json.Marshal(map[string]any{"verdicts": map[string]any{"safe": map[string]any{"flagged": !flagged}, "danger": map[string]any{"flagged": true}}})
				return string(encoded), nil
			})
			got := e.Evaluate(t.Context(), tools())[0].Status.ToolUses
			want := "not_flagged"
			if flagged {
				want = "flagged"
			}
			if !reflect.DeepEqual(called, []string{"primary", "backup"}) || got["safe"].Outcome != want || got["danger"].Outcome != "flagged" {
				t.Fatalf("calls %v, verdicts %+v", called, got)
			}
		})
	}
}

func TestFallbackDoesNotRetryCompleteVerdicts(t *testing.T) {
	calls := 0
	e := fallbackEvaluator(t, Config{ClassifierModel: "primary", FallbackModels: []string{"backup"}}, func(context.Context, ClassifierRequest) (string, error) {
		calls++
		return `{"verdicts":{"safe":{"flagged":false},"danger":{"flagged":true}}}`, nil
	})
	e.Evaluate(t.Context(), tools())
	if calls != 1 {
		t.Fatalf("valid verdicts caused %d attempts", calls)
	}
}

func TestFallbackExhaustionPreservesEvaluatedVerdicts(t *testing.T) {
	calls := 0
	e := fallbackEvaluator(t, Config{ClassifierModel: "primary", FallbackModels: []string{"backup", "last"}}, func(_ context.Context, r ClassifierRequest) (string, error) {
		calls++
		if r.Model == "primary" {
			return `{"verdicts":{"safe":{"flagged":true}}}`, nil
		}
		return "", errors.New("unavailable")
	})
	got := e.Evaluate(t.Context(), tools())[0].Status.ToolUses
	if calls != 3 || got["safe"].Outcome != "flagged" || got["danger"].Type != "unavailable" || got["danger"].Reason != "error" {
		t.Fatalf("calls %d, verdicts %+v", calls, got)
	}
}

func TestFallbackCancellationStopsAttempts(t *testing.T) {
	for _, preCanceled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(t.Context())
		if preCanceled {
			cancel()
		}
		calls := 0
		e := fallbackEvaluator(t, Config{ClassifierModel: "primary", FallbackModels: []string{"backup"}}, func(ctx context.Context, _ ClassifierRequest) (string, error) {
			calls++
			cancel()
			return "", ctx.Err()
		})
		got := e.Evaluate(ctx, tools())[0].Status.ToolUses
		cancel()
		wantCalls := 1
		if preCanceled {
			wantCalls = 0
		}
		if calls != wantCalls || got["safe"].Type != "unavailable" || got["safe"].Reason != "error" {
			t.Fatalf("preCanceled %v, calls %d, verdicts %+v", preCanceled, calls, got)
		}
	}
}

func TestFallbackOverallBudget(t *testing.T) {
	calls := 0
	var overallDeadline time.Time
	e := fallbackEvaluator(t, Config{ClassifierModel: "primary", FallbackModels: []string{"backup", "last"}, Timeout: 40 * time.Millisecond, AttemptTimeout: time.Second}, func(ctx context.Context, r ClassifierRequest) (string, error) {
		calls++
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("no deadline")
		}
		if r.Model == "primary" {
			overallDeadline = deadline
			return "", errors.New("upstream failed")
		}
		if !deadline.Equal(overallDeadline) {
			t.Fatal("fallback extended overall deadline")
		}
		<-ctx.Done()
		return "", ctx.Err()
	})
	got := e.Evaluate(t.Context(), tools())[0].Status.ToolUses
	if calls != 2 || got["safe"].Reason != "timeout" {
		t.Fatalf("calls %d, verdicts %+v", calls, got)
	}
}

func TestFallbackSkipsInvalidTools(t *testing.T) {
	calls := 0
	e := fallbackEvaluator(t, Config{ClassifierModel: "primary", FallbackModels: []string{"backup"}}, func(_ context.Context, r ClassifierRequest) (string, error) {
		calls++
		if !reflect.DeepEqual(proposedIDs(t, r), []string{"valid"}) {
			t.Fatal("reviewed invalid tools")
		}
		return `{"verdicts":{}}`, nil
	})
	uses := []ToolUse{
		{ID: "valid", Name: "Bash", Input: json.RawMessage(`{}`)},
		{ID: "truncated", Name: "Bash", Input: json.RawMessage(`{}`), Truncated: true},
		{ID: "invalid", Name: "Bash", Input: json.RawMessage(`null`)},
	}
	got := e.Evaluate(t.Context(), uses)[0].Status.ToolUses
	if calls != 2 || got["truncated"].Reason != "truncated" || got["invalid"].Type != "unavailable" {
		t.Fatalf("calls %d, verdicts %+v", calls, got)
	}
}

func TestDenialEscalation(t *testing.T) {
	for _, scenario := range []string{"disabled", "approve", "deny", "error", "malformed", "timeout", "cancel", "overall_timeout"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			calls := 0
			cfg := Config{ClassifierModel: "primary", FallbackModels: []string{"backup", "last"}, EscalateOnDenial: scenario != "disabled", Timeout: time.Second, AttemptTimeout: 10 * time.Millisecond}
			if scenario == "overall_timeout" {
				cfg.Timeout = 10 * time.Millisecond
				cfg.AttemptTimeout = time.Second
			}
			e := fallbackEvaluator(t, cfg, func(ctx context.Context, r ClassifierRequest) (string, error) {
				calls++
				if r.Model == "primary" {
					return `{"verdicts":{"safe":{"flagged":false},"danger":{"flagged":true,"explanation":"original denial"}}}`, nil
				}
				if !reflect.DeepEqual(proposedIDs(t, r), []string{"danger"}) {
					t.Fatal("approval was re-reviewed")
				}
				if strings.Contains(r.User, "original denial") {
					t.Fatal("review must be independent")
				}
				switch scenario {
				case "approve":
					return `{"verdicts":{"danger":{"flagged":false},"safe":{"flagged":true}}}`, nil
				case "deny":
					return `{"verdicts":{"danger":{"flagged":true,"explanation":"confirmed"}}}`, nil
				case "error":
					return "", errors.New("failed")
				case "malformed":
					return `{"verdicts":{}}`, nil
				case "timeout", "overall_timeout":
					<-ctx.Done()
					return "", ctx.Err()
				case "cancel":
					cancel()
					return "", ctx.Err()
				}
				t.Fatal("unexpected fallback")
				return "", nil
			})
			got := e.Evaluate(ctx, tools())[0].Status.ToolUses
			expected := "flagged"
			if scenario == "approve" {
				expected = "not_flagged"
			}
			if got["danger"].Outcome != expected || got["safe"].Outcome != "not_flagged" {
				t.Fatalf("verdicts: %+v", got)
			}
			wantCalls := 3
			switch scenario {
			case "disabled":
				wantCalls = 1
			case "approve", "deny", "cancel", "overall_timeout":
				wantCalls = 2
			}
			if calls != wantCalls {
				t.Fatalf("calls=%d want=%d", calls, wantCalls)
			}
			if scenario != "approve" && scenario != "deny" && got["danger"].Explanation != "original denial" {
				t.Fatal("lost original denial")
			}
		})
	}
}
